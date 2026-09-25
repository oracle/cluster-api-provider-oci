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
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestOCIVirtualMachinePoolLifecycle(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "virtual-pool-integration-"}}
	g.Expect(testEnvironment.Create(testContext, namespace)).To(Succeed())

	cluster := managedMachinePoolCAPICluster(namespace.Name)
	g.Expect(testEnvironment.Create(testContext, cluster)).To(Succeed())
	storedCluster := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Status.Initialization.InfrastructureProvisioned = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedCluster)).To(Succeed())

	managedCluster := managedMachinePoolCluster(cluster)
	g.Expect(testEnvironment.Create(testContext, managedCluster)).To(Succeed())
	storedManagedCluster := &infrastructurev1beta2.OCIManagedCluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(managedCluster), storedManagedCluster)).To(Succeed())
	storedManagedCluster.Status.Ready = true
	g.Expect(testEnvironment.Status().Update(testContext, storedManagedCluster)).To(Succeed())

	version := "v1.34.1"
	controlPlane := managedMachinePoolControlPlane(cluster, version)
	g.Expect(testEnvironment.Create(testContext, controlPlane)).To(Succeed())
	storedControlPlane := &infrastructurev1beta2.OCIManagedControlPlane{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(controlPlane), storedControlPlane)).To(Succeed())
	storedControlPlane.Status.Ready = true
	g.Expect(testEnvironment.Status().Update(testContext, storedControlPlane)).To(Succeed())

	machinePool := virtualCAPIMachinePool(cluster, version, 2)
	g.Expect(testEnvironment.Create(testContext, machinePool)).To(Succeed())
	virtualMachinePool := virtualInfrastructureMachinePool(machinePool, cluster.Name)
	g.Expect(testEnvironment.Create(testContext, virtualMachinePool)).To(Succeed())

	t.Cleanup(func() {
		machines := &infrav2exp.OCIMachinePoolMachineList{}
		if err := testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(namespace.Name)); err == nil {
			for i := range machines.Items {
				deleteAndIgnoreNotFound(t, &machines.Items[i])
			}
		}
		for _, object := range []client.Object{virtualMachinePool, machinePool, controlPlane, managedCluster, cluster, namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})

	g.Consistently(func() int {
		_, creates, _, _, _ := fakeOCI.oke.virtualNodePoolCounts()
		return creates
	}).WithTimeout(time.Second).WithPolling(100 * time.Millisecond).Should(BeZero())

	storedCluster = &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())

	virtualMachinePoolKey := client.ObjectKeyFromObject(virtualMachinePool)
	waitForVirtualMachinePool(t, virtualMachinePoolKey, 2, 2, 1, 0)

	updateMachinePoolReplicas(t, machinePool, 3)
	waitForVirtualNodePoolUpdateCount(t, 1)
	waitForVirtualNodePoolInspectionsToSettle(t)
	triggerVirtualMachinePoolReconcile(t, virtualMachinePool)
	waitForVirtualMachinePool(t, virtualMachinePoolKey, 3, 3, 1, 1)

	g.Eventually(func() error {
		stored := &infrav2exp.OCIVirtualMachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, virtualMachinePoolKey, stored); err != nil {
			return err
		}
		stored.Spec.PodConfiguration.Shape = common.String("Pod.Standard.A1.Flex")
		stored.Spec.InitialVirtualNodeLabels = []infrav2exp.KeyValue{{
			Key: common.String("pool"), Value: common.String("virtual"),
		}}
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	triggerVirtualMachinePoolReconcile(t, virtualMachinePool)
	waitForVirtualNodePoolUpdateCount(t, 2)
	waitForVirtualNodePoolInspectionsToSettle(t)
	triggerVirtualMachinePoolReconcile(t, virtualMachinePool)
	waitForVirtualMachinePool(t, virtualMachinePoolKey, 3, 3, 1, 2)

	_, creates, updates, _, _ := fakeOCI.oke.virtualNodePoolCounts()
	g.Expect(creates).To(Equal(1))
	g.Expect(updates).To(Equal(2))
	inspectionsBefore := waitForVirtualNodePoolInspectionsToSettle(t)

	storedCluster = &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(true)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())
	g.Eventually(func(g Gomega) {
		paused := &clusterv1.Cluster{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), paused)).To(Succeed())
		g.Expect(paused.Spec.Paused).NotTo(BeNil())
		g.Expect(*paused.Spec.Paused).To(BeTrue())
	}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	storedCluster = &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())
	g.Eventually(func() int {
		_, _, _, _, inspections := fakeOCI.oke.virtualNodePoolCounts()
		return inspections
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).
		Should(BeNumerically(">", inspectionsBefore))
	_, creates, updates, _, _ = fakeOCI.oke.virtualNodePoolCounts()
	g.Expect(creates).To(Equal(1))
	g.Expect(updates).To(Equal(2))

	updateMachinePoolReplicas(t, machinePool, 1)
	waitForVirtualNodePoolUpdateCount(t, 3)
	waitForVirtualNodePoolInspectionsToSettle(t)
	triggerVirtualMachinePoolReconcile(t, virtualMachinePool)
	waitForVirtualMachinePool(t, virtualMachinePoolKey, 1, 3, 1, 3)

	g.Expect(testEnvironment.Delete(testContext, virtualMachinePool)).To(Succeed())
	g.Eventually(func(g Gomega) {
		active, _, _, deletes, _ := fakeOCI.oke.virtualNodePoolCounts()
		g.Expect(active).To(BeZero())
		g.Expect(deletes).To(Equal(1))
		err := testEnvironment.GetAPIReader().Get(testContext, virtualMachinePoolKey, &infrav2exp.OCIVirtualMachinePool{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected virtual machine-pool deletion, got %v", err)
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func virtualCAPIMachinePool(cluster *clusterv1.Cluster, version string, replicas int32) *clusterv1.MachinePool {
	return &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "virtual-pool-workers",
			Namespace: cluster.Namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
		},
		Spec: clusterv1.MachinePoolSpec{
			ClusterName: cluster.Name,
			Replicas:    &replicas,
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName: cluster.Name,
					Bootstrap:   clusterv1.Bootstrap{DataSecretName: common.String("")},
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: infrastructurev1beta2.GroupVersion.Group,
						Kind:     scope.OCIVirtualMachinePoolKind,
						Name:     "virtual-pool-workers",
					},
					Version: version,
				},
			},
		},
	}
}

func virtualInfrastructureMachinePool(machinePool *clusterv1.MachinePool, clusterName string) *infrav2exp.OCIVirtualMachinePool {
	return &infrav2exp.OCIVirtualMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machinePool.Name,
			Namespace: machinePool.Namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(machinePool, clusterv1.GroupVersion.WithKind("MachinePool")),
			},
		},
	}
}

func waitForVirtualMachinePool(t *testing.T, key client.ObjectKey, replicas, machineObjects, creates, updates int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		active, createCalls, updateCalls, _, _ := fakeOCI.oke.virtualNodePoolCounts()
		g.Expect(active).To(Equal(1))
		g.Expect(createCalls).To(Equal(creates))
		g.Expect(updateCalls).To(Equal(updates))
		g.Expect(fakeOCI.oke.virtualNodePoolSize()).To(Equal(replicas))

		stored := &infrav2exp.OCIVirtualMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrav2exp.VirtualMachinePoolFinalizer)).To(BeTrue())
		g.Expect(stored.Spec.ID).NotTo(BeNil())
		g.Expect(stored.Spec.ProviderID).NotTo(BeNil())
		fakeSize, listCalls, lastListSize := fakeOCI.oke.virtualNodePoolDebug()
		g.Expect(stored.Spec.ProviderIDList).To(HaveLen(replicas), "fake size %d after %d virtual-node lists; last list returned %d", fakeSize, listCalls, lastListSize)
		g.Expect(stored.Status.Ready).To(BeTrue())
		g.Expect(stored.Status.Replicas).To(Equal(int32(replicas)))
		g.Expect(stored.Status.InfrastructureMachineKind).To(Equal("OCIMachinePoolMachine"))
		g.Expect(v1beta1conditions.IsTrue(stored, infrav2exp.VirtualNodePoolReadyCondition)).To(BeTrue())

		machines := &infrav2exp.OCIMachinePoolMachineList{}
		g.Expect(testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(key.Namespace))).To(Succeed())
		// Core CAPI and garbage collection, which are not part of envtest,
		// finish removing stale child infrastructure machines after scale-down.
		g.Expect(machines.Items).To(HaveLen(machineObjects))
		for i := range machines.Items {
			g.Expect(machines.Items[i].Status.Ready).To(BeTrue())
		}
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func waitForVirtualNodePoolUpdateCount(t *testing.T, expected int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() int {
		_, _, updates, _, _ := fakeOCI.oke.virtualNodePoolCounts()
		return updates
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(expected))
}

func waitForVirtualNodePoolInspectionsToSettle(t *testing.T) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	stableSince := time.Now()
	_, _, _, _, previous := fakeOCI.oke.virtualNodePoolCounts()
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		_, _, _, _, current := fakeOCI.oke.virtualNodePoolCounts()
		if current != previous {
			previous = current
			stableSince = time.Now()
			continue
		}
		if time.Since(stableSince) >= 300*time.Millisecond {
			return current
		}
	}
	t.Fatalf("virtual node-pool inspections did not settle")
	return 0
}

func triggerVirtualMachinePoolReconcile(t *testing.T, machinePool *infrav2exp.OCIVirtualMachinePool) {
	t.Helper()
	g := NewWithT(t)
	_, _, _, _, inspectionsBefore := fakeOCI.oke.virtualNodePoolCounts()
	g.Eventually(func() error {
		stored := &infrav2exp.OCIVirtualMachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(machinePool), stored); err != nil {
			return err
		}
		if stored.Annotations == nil {
			stored.Annotations = map[string]string{}
		}
		stored.Annotations["integration-test/reconcile"] = time.Now().UTC().Format(time.RFC3339Nano)
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	g.Eventually(func() int {
		_, _, _, _, inspections := fakeOCI.oke.virtualNodePoolCounts()
		return inspections
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).
		Should(BeNumerically(">", inspectionsBefore))
}
