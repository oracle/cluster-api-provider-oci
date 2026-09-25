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
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestOCIManagedMachinePoolRecoversFromCloudFailuresAndAsyncStates(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fakeOCI.oke.setFailure(createNodePoolOperation, errors.New("injected OKE node-pool create failure"))
	fakeOCI.oke.setNodePoolCreateState(oke.NodePoolLifecycleStateCreating)
	fixture := createManagedMachinePoolFixture(t, false, 1)
	t.Cleanup(func() {
		for _, operation := range []fakeOKEOperation{createNodePoolOperation, updateNodePoolOperation, deleteNodePoolOperation} {
			fakeOCI.oke.clearFailure(operation)
		}
	})
	unpauseCluster(t, fixture.cluster)
	triggerObjectReconcile(t, fixture.managedMachinePool)

	waitForOKEAttempt(t, createNodePoolOperation, 0)
	key := client.ObjectKeyFromObject(fixture.managedMachinePool)
	g.Eventually(func(g Gomega) {
		active, creates, _, _, _ := fakeOCI.oke.nodePoolCounts()
		g.Expect(active).To(BeZero())
		g.Expect(creates).To(BeZero())
		stored := &infrav2exp.OCIManagedMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrav2exp.ManagedMachinePoolFinalizer)).To(BeTrue())
		condition := v1beta1conditions.Get(stored, infrav2exp.NodePoolReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(condition.Reason).To(Equal(infrav2exp.NodePoolProvisionFailedReason))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForWarningEvent(t, fixture.namespace.Name, fixture.managedMachinePool.Name, "ReconcileError", "injected OKE node-pool create failure")

	fakeOCI.oke.clearFailure(createNodePoolOperation)
	triggerObjectReconcile(t, fixture.managedMachinePool)
	var nodePoolID string
	g.Eventually(func(g Gomega) {
		active, creates, _, _, _ := fakeOCI.oke.nodePoolCounts()
		g.Expect(active).To(Equal(1))
		g.Expect(creates).To(Equal(1))
		stored := &infrav2exp.OCIManagedMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(stored.Spec.ID).NotTo(BeNil())
		nodePoolID = *stored.Spec.ID
		g.Expect(stored.Status.Ready).To(BeFalse())
		g.Expect(stored.Status.NodepoolLifecycleState).To(Equal(string(oke.NodePoolLifecycleStateCreating)))
		condition := v1beta1conditions.Get(stored, infrav2exp.NodePoolReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(infrav2exp.NodePoolNotReadyReason))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	fakeOCI.oke.setNodePoolState(nodePoolID, oke.NodePoolLifecycleStateActive)
	triggerObjectReconcile(t, fixture.managedMachinePool)
	waitForManagedMachinePool(t, key, 1, 1, 1, 0)

	updateAttempts := fakeOCI.oke.operationAttemptCount(updateNodePoolOperation)
	fakeOCI.oke.setFailure(updateNodePoolOperation, errors.New("injected OKE node-pool update failure"))
	fakeOCI.oke.setNodePoolUpdateState(oke.NodePoolLifecycleStateUpdating)
	updatedVersion := "v1.34.2"
	g.Eventually(func() error {
		stored := &infrav2exp.OCIManagedMachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, key, stored); err != nil {
			return err
		}
		stored.Spec.Version = common.String(updatedVersion)
		stored.Spec.NodeSourceViaImage.ImageId = common.String("ocid1.image.oc1..managed-recovery-v2")
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForOKEAttempt(t, updateNodePoolOperation, updateAttempts)
	g.Eventually(func(g Gomega) {
		pool, ok := fakeOCI.oke.nodePool(nodePoolID)
		g.Expect(ok).To(BeTrue())
		g.Expect(pool.LifecycleState).To(Equal(oke.NodePoolLifecycleStateActive))
		_, creates, updates, _, _ := fakeOCI.oke.nodePoolCounts()
		g.Expect(creates).To(Equal(1))
		g.Expect(updates).To(BeZero())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	fakeOCI.oke.clearFailure(updateNodePoolOperation)
	triggerObjectReconcile(t, fixture.managedMachinePool)
	g.Eventually(func(g Gomega) {
		pool, ok := fakeOCI.oke.nodePool(nodePoolID)
		g.Expect(ok).To(BeTrue())
		g.Expect(pool.LifecycleState).To(Equal(oke.NodePoolLifecycleStateUpdating))
		g.Expect(pool.KubernetesVersion).To(Equal(common.String(updatedVersion)))
		_, _, updates, _, _ := fakeOCI.oke.nodePoolCounts()
		g.Expect(updates).To(Equal(1))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	fakeOCI.oke.setNodePoolState(nodePoolID, oke.NodePoolLifecycleStateActive)
	triggerObjectReconcile(t, fixture.managedMachinePool)
	waitForManagedMachinePool(t, key, 1, 1, 1, 1)

	deleteAttempts := fakeOCI.oke.operationAttemptCount(deleteNodePoolOperation)
	fakeOCI.oke.setFailure(deleteNodePoolOperation, errors.New("injected OKE node-pool delete failure"))
	g.Expect(testEnvironment.Delete(testContext, fixture.managedMachinePool)).To(Succeed())
	waitForOKEAttempt(t, deleteNodePoolOperation, deleteAttempts)
	g.Eventually(func(g Gomega) {
		stored := &infrav2exp.OCIManagedMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(stored.DeletionTimestamp.IsZero()).To(BeFalse())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrav2exp.ManagedMachinePoolFinalizer)).To(BeTrue())
		active, _, _, deletes, _ := fakeOCI.oke.nodePoolCounts()
		g.Expect(active).To(Equal(1))
		g.Expect(deletes).To(BeZero())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	fakeOCI.oke.clearFailure(deleteNodePoolOperation)
	triggerObjectReconcile(t, fixture.managedMachinePool)
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, &infrav2exp.OCIManagedMachinePool{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
	active, creates, updates, deletes, _ := fakeOCI.oke.nodePoolCounts()
	g.Expect(active).To(BeZero())
	g.Expect(creates).To(Equal(1))
	g.Expect(updates).To(Equal(1))
	g.Expect(deletes).To(Equal(1))
}

func TestOCIVirtualMachinePoolRecoversFromCloudFailuresAndAsyncStates(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fakeOCI.oke.setFailure(createVirtualNodePoolOperation, errors.New("injected OKE virtual node-pool create failure"))
	fakeOCI.oke.setVirtualNodePoolCreateState(oke.VirtualNodePoolLifecycleStateCreating)
	fixture := createVirtualMachinePoolFixture(t, false, 1)
	t.Cleanup(func() {
		for _, operation := range []fakeOKEOperation{createVirtualNodePoolOperation, updateVirtualNodePoolOperation, deleteVirtualNodePoolOperation} {
			fakeOCI.oke.clearFailure(operation)
		}
	})
	unpauseCluster(t, fixture.cluster)
	triggerObjectReconcile(t, fixture.virtualMachinePool)

	waitForOKEAttempt(t, createVirtualNodePoolOperation, 0)
	key := client.ObjectKeyFromObject(fixture.virtualMachinePool)
	g.Eventually(func(g Gomega) {
		active, creates, _, _, _ := fakeOCI.oke.virtualNodePoolCounts()
		g.Expect(active).To(BeZero())
		g.Expect(creates).To(BeZero())
		stored := &infrav2exp.OCIVirtualMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrav2exp.VirtualMachinePoolFinalizer)).To(BeTrue())
		condition := v1beta1conditions.Get(stored, infrav2exp.VirtualNodePoolReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(condition.Reason).To(Equal(infrav2exp.VirtualNodePoolProvisionFailedReason))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForWarningEvent(t, fixture.namespace.Name, fixture.virtualMachinePool.Name, "ReconcileError", "injected OKE virtual node-pool create failure")

	fakeOCI.oke.clearFailure(createVirtualNodePoolOperation)
	triggerObjectReconcile(t, fixture.virtualMachinePool)
	var poolID string
	g.Eventually(func(g Gomega) {
		active, creates, _, _, _ := fakeOCI.oke.virtualNodePoolCounts()
		g.Expect(active).To(Equal(1))
		g.Expect(creates).To(Equal(1))
		stored := &infrav2exp.OCIVirtualMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(stored.Spec.ID).NotTo(BeNil())
		poolID = *stored.Spec.ID
		g.Expect(stored.Status.Ready).To(BeFalse())
		g.Expect(stored.Status.NodepoolLifecycleState).To(Equal(string(oke.VirtualNodePoolLifecycleStateCreating)))
		condition := v1beta1conditions.Get(stored, infrav2exp.VirtualNodePoolReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(infrav2exp.VirtualNodePoolNotReadyReason))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	fakeOCI.oke.setVirtualNodePoolState(poolID, oke.VirtualNodePoolLifecycleStateActive)
	triggerObjectReconcile(t, fixture.virtualMachinePool)
	waitForVirtualMachinePool(t, key, 1, 1, 1, 0)

	updateAttempts := fakeOCI.oke.operationAttemptCount(updateVirtualNodePoolOperation)
	fakeOCI.oke.setFailure(updateVirtualNodePoolOperation, errors.New("injected OKE virtual node-pool update failure"))
	fakeOCI.oke.setVirtualNodePoolUpdateState(oke.VirtualNodePoolLifecycleStateUpdating)
	g.Eventually(func() error {
		stored := &infrav2exp.OCIVirtualMachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, key, stored); err != nil {
			return err
		}
		stored.Spec.PodConfiguration.Shape = common.String("Pod.Standard.A1.Flex")
		stored.Spec.InitialVirtualNodeLabels = []infrav2exp.KeyValue{{Key: common.String("recovery"), Value: common.String("true")}}
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForOKEAttempt(t, updateVirtualNodePoolOperation, updateAttempts)
	g.Eventually(func(g Gomega) {
		pool, ok := fakeOCI.oke.virtualNodePool(poolID)
		g.Expect(ok).To(BeTrue())
		g.Expect(pool.LifecycleState).To(Equal(oke.VirtualNodePoolLifecycleStateActive))
		_, creates, updates, _, _ := fakeOCI.oke.virtualNodePoolCounts()
		g.Expect(creates).To(Equal(1))
		g.Expect(updates).To(BeZero())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	fakeOCI.oke.clearFailure(updateVirtualNodePoolOperation)
	triggerObjectReconcile(t, fixture.virtualMachinePool)
	g.Eventually(func(g Gomega) {
		pool, ok := fakeOCI.oke.virtualNodePool(poolID)
		g.Expect(ok).To(BeTrue())
		g.Expect(pool.LifecycleState).To(Equal(oke.VirtualNodePoolLifecycleStateUpdating))
		_, _, updates, _, _ := fakeOCI.oke.virtualNodePoolCounts()
		g.Expect(updates).To(Equal(1))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	fakeOCI.oke.setVirtualNodePoolState(poolID, oke.VirtualNodePoolLifecycleStateActive)
	triggerObjectReconcile(t, fixture.virtualMachinePool)
	waitForVirtualMachinePool(t, key, 1, 1, 1, 1)

	deleteAttempts := fakeOCI.oke.operationAttemptCount(deleteVirtualNodePoolOperation)
	fakeOCI.oke.setFailure(deleteVirtualNodePoolOperation, errors.New("injected OKE virtual node-pool delete failure"))
	g.Expect(testEnvironment.Delete(testContext, fixture.virtualMachinePool)).To(Succeed())
	waitForOKEAttempt(t, deleteVirtualNodePoolOperation, deleteAttempts)
	g.Eventually(func(g Gomega) {
		stored := &infrav2exp.OCIVirtualMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(stored.DeletionTimestamp.IsZero()).To(BeFalse())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrav2exp.VirtualMachinePoolFinalizer)).To(BeTrue())
		active, _, _, deletes, _ := fakeOCI.oke.virtualNodePoolCounts()
		g.Expect(active).To(Equal(1))
		g.Expect(deletes).To(BeZero())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	fakeOCI.oke.clearFailure(deleteVirtualNodePoolOperation)
	triggerObjectReconcile(t, fixture.virtualMachinePool)
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, &infrav2exp.OCIVirtualMachinePool{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
	active, creates, updates, deletes, _ := fakeOCI.oke.virtualNodePoolCounts()
	g.Expect(active).To(BeZero())
	g.Expect(creates).To(Equal(1))
	g.Expect(updates).To(Equal(1))
	g.Expect(deletes).To(Equal(1))
}
