//go:build integration

/*
Copyright (c) 2026 Oracle and/or its affiliates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration_test

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	infrav3exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta3"
	"github.com/oracle/oci-go-sdk/v65/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOCIMachinePoolHonorsExternalAutoscaler(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fixture := createInstancePoolFixture(t, true, 1)
	unpauseCluster(t, fixture.cluster)
	triggerObjectReconcile(t, fixture.ociMachinePool)
	key := client.ObjectKeyFromObject(fixture.ociMachinePool)
	waitForInstanceMachinePool(t, key, 1, 1, 1, 1, 1, 0)
	stored := &infrav3exp.OCIMachinePool{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
	poolID := *stored.Spec.OCID

	fakeOCI.computeManagement.resizeInstancePool(poolID, 3)
	triggerObjectReconcile(t, fixture.ociMachinePool)
	g.Eventually(func(g Gomega) {
		storedPool := &infrav3exp.OCIMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, storedPool)).To(Succeed())
		g.Expect(storedPool.Spec.ProviderIDList).To(HaveLen(3))
		g.Expect(storedPool.Status.Replicas).To(Equal(int32(3)))
		g.Expect(storedPool.Status.Initialization.Provisioned).To(Equal(common.Bool(true)))
		g.Expect(conditions.IsTrue(storedPool, infrav3exp.InstancePoolReadyCondition)).To(BeTrue())
		machinePool := &clusterv1.MachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.machinePool), machinePool)).To(Succeed())
		g.Expect(machinePool.Spec.Replicas).NotTo(BeNil())
		g.Expect(*machinePool.Spec.Replicas).To(Equal(int32(3)))
		poolSize, _ := fakeOCI.computeManagement.instancePoolState()
		g.Expect(poolSize).To(Equal(3))
		_, _, _, _, creates, updates, _, _ := fakeOCI.computeManagement.counts()
		g.Expect(creates).To(Equal(1))
		g.Expect(updates).To(BeZero())
		machines := &infrav3exp.OCIMachinePoolMachineList{}
		g.Expect(testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(fixture.namespace.Name))).To(Succeed())
		g.Expect(machines.Items).To(HaveLen(3))
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Expect(testEnvironment.Delete(testContext, fixture.ociMachinePool)).To(Succeed())
	waitForInstancePoolTerminateCount(t, 1)
	triggerObjectReconcile(t, fixture.ociMachinePool)
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, &infrav3exp.OCIMachinePool{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
}

func TestOCIManagedMachinePoolHonorsExternalAutoscaler(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fixture := createManagedMachinePoolFixture(t, true, 1)
	unpauseCluster(t, fixture.cluster)
	triggerObjectReconcile(t, fixture.managedMachinePool)
	key := client.ObjectKeyFromObject(fixture.managedMachinePool)
	waitForManagedMachinePool(t, key, 1, 1, 1, 0)
	stored := &infrav3exp.OCIManagedMachinePool{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
	poolID := *stored.Spec.ID

	fakeOCI.oke.resizeNodePool(poolID, 3)
	triggerObjectReconcile(t, fixture.managedMachinePool)
	g.Eventually(func(g Gomega) {
		storedPool := &infrav3exp.OCIManagedMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, storedPool)).To(Succeed())
		g.Expect(storedPool.Spec.ProviderIDList).To(HaveLen(3))
		g.Expect(storedPool.Status.Replicas).To(Equal(int32(3)))
		g.Expect(storedPool.Status.Initialization.Provisioned).To(Equal(common.Bool(true)))
		machinePool := &clusterv1.MachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.machinePool), machinePool)).To(Succeed())
		g.Expect(machinePool.Spec.Replicas).NotTo(BeNil())
		g.Expect(*machinePool.Spec.Replicas).To(Equal(int32(1)))
		pool, ok := fakeOCI.oke.nodePool(poolID)
		g.Expect(ok).To(BeTrue())
		g.Expect(pool.NodeConfigDetails.Size).To(Equal(common.Int(3)))
		_, creates, updates, _, _ := fakeOCI.oke.nodePoolCounts()
		g.Expect(creates).To(Equal(1))
		g.Expect(updates).To(BeZero())
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Expect(testEnvironment.Delete(testContext, fixture.managedMachinePool)).To(Succeed())
	triggerObjectReconcile(t, fixture.managedMachinePool)
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, &infrav3exp.OCIManagedMachinePool{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
}

func TestOCIVirtualMachinePoolHonorsExternalAutoscaler(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fixture := createVirtualMachinePoolFixture(t, true, 1)
	unpauseCluster(t, fixture.cluster)
	triggerObjectReconcile(t, fixture.virtualMachinePool)
	key := client.ObjectKeyFromObject(fixture.virtualMachinePool)
	waitForVirtualMachinePool(t, key, 1, 1, 1, 0)
	stored := &infrav3exp.OCIVirtualMachinePool{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
	poolID := *stored.Spec.ID

	fakeOCI.oke.resizeVirtualNodePool(poolID, 3)
	triggerObjectReconcile(t, fixture.virtualMachinePool)
	g.Eventually(func(g Gomega) {
		storedPool := &infrav3exp.OCIVirtualMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, storedPool)).To(Succeed())
		g.Expect(storedPool.Spec.ProviderIDList).To(HaveLen(3))
		g.Expect(storedPool.Status.Replicas).To(Equal(int32(3)))
		g.Expect(storedPool.Status.Initialization.Provisioned).To(Equal(common.Bool(true)))
		machinePool := &clusterv1.MachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.machinePool), machinePool)).To(Succeed())
		g.Expect(machinePool.Spec.Replicas).NotTo(BeNil())
		g.Expect(*machinePool.Spec.Replicas).To(Equal(int32(1)))
		pool, ok := fakeOCI.oke.virtualNodePool(poolID)
		g.Expect(ok).To(BeTrue())
		g.Expect(pool.Size).To(Equal(common.Int(3)))
		_, creates, updates, _, _ := fakeOCI.oke.virtualNodePoolCounts()
		g.Expect(creates).To(Equal(1))
		g.Expect(updates).To(BeZero())
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Expect(testEnvironment.Delete(testContext, fixture.virtualMachinePool)).To(Succeed())
	triggerObjectReconcile(t, fixture.virtualMachinePool)
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, &infrav3exp.OCIVirtualMachinePool{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
}
