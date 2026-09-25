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
	"strconv"
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

func TestOCIManagedMachinePoolLifecycle(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "managed-pool-integration-"}}
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

	machinePool := managedCAPIMachinePool(cluster, version, 2)
	g.Expect(testEnvironment.Create(testContext, machinePool)).To(Succeed())
	managedMachinePool := managedInfrastructureMachinePool(machinePool, cluster.Name, version)
	g.Expect(testEnvironment.Create(testContext, managedMachinePool)).To(Succeed())

	t.Cleanup(func() {
		machinePoolMachines := &infrav2exp.OCIMachinePoolMachineList{}
		if err := testEnvironment.GetAPIReader().List(testContext, machinePoolMachines, client.InNamespace(namespace.Name)); err == nil {
			for i := range machinePoolMachines.Items {
				deleteAndIgnoreNotFound(t, &machinePoolMachines.Items[i])
			}
		}
		for _, object := range []client.Object{managedMachinePool, machinePool, controlPlane, managedCluster, cluster, namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})

	g.Consistently(func() int {
		_, creates, _, _, _ := fakeOCI.oke.nodePoolCounts()
		return creates
	}).WithTimeout(time.Second).WithPolling(100 * time.Millisecond).Should(BeZero())

	storedCluster = &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())

	managedMachinePoolKey := client.ObjectKeyFromObject(managedMachinePool)
	waitForManagedMachinePool(t, managedMachinePoolKey, 2, 2, 1, 0)

	updateMachinePoolReplicas(t, machinePool, 3)
	waitForNodePoolUpdateCount(t, 1)
	triggerMachinePoolReconcile(t, machinePool)
	waitForManagedMachinePool(t, managedMachinePoolKey, 3, 3, 1, 1)

	updateMachinePoolReplicas(t, machinePool, 1)
	waitForNodePoolUpdateCount(t, 2)
	triggerMachinePoolReconcile(t, machinePool)
	waitForManagedMachinePool(t, managedMachinePoolKey, 1, 3, 1, 2)

	updatedVersion := "v1.34.2"
	g.Eventually(func() error {
		stored := &infrav2exp.OCIManagedMachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, managedMachinePoolKey, stored); err != nil {
			return err
		}
		stored.Spec.Version = common.String(updatedVersion)
		stored.Spec.NodeSourceViaImage.ImageId = common.String("ocid1.image.oc1..managed-pool-v2")
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForNodePoolUpdateCount(t, 3)
	triggerMachinePoolReconcile(t, machinePool)
	waitForManagedMachinePool(t, managedMachinePoolKey, 1, 3, 1, 3)

	_, creates, updates, _, _ := fakeOCI.oke.nodePoolCounts()
	g.Expect(creates).To(Equal(1))
	g.Expect(updates).To(Equal(3))
	inspectionsBefore := waitForNodePoolInspectionsToSettle(t)

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
		_, _, _, _, inspections := fakeOCI.oke.nodePoolCounts()
		return inspections
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).
		Should(BeNumerically(">", inspectionsBefore))
	_, creates, updates, _, _ = fakeOCI.oke.nodePoolCounts()
	g.Expect(creates).To(Equal(1))
	g.Expect(updates).To(Equal(3))

	g.Expect(testEnvironment.Delete(testContext, managedMachinePool)).To(Succeed())
	g.Eventually(func(g Gomega) {
		active, _, _, deletes, _ := fakeOCI.oke.nodePoolCounts()
		g.Expect(active).To(BeZero())
		g.Expect(deletes).To(Equal(1))
		err := testEnvironment.GetAPIReader().Get(testContext, managedMachinePoolKey, &infrav2exp.OCIManagedMachinePool{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected managed machine-pool deletion, got %v", err)
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func managedMachinePoolCAPICluster(namespace string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "managed-pool", Namespace: namespace},
		Spec: clusterv1.ClusterSpec{
			Paused: common.Bool(true),
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     scope.OCIManagedClusterKind,
				Name:     "managed-pool",
			},
			ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     "OCIManagedControlPlane",
				Name:     "managed-pool",
			},
		},
	}
}

func managedMachinePoolCluster(cluster *clusterv1.Cluster) *infrastructurev1beta2.OCIManagedCluster {
	network := managedIntegrationNetworkSpec()
	network.Vcn.ID = common.String("ocid1.vcn.oc1..managed-pool")
	for i, subnet := range network.Vcn.Subnets {
		subnet.ID = common.String("ocid1.subnet.oc1..managed-pool-" + strconv.Itoa(i+1))
	}
	for i, nsg := range network.Vcn.NetworkSecurityGroup.List {
		nsg.ID = common.String("ocid1.networksecuritygroup.oc1..managed-pool-" + strconv.Itoa(i+1))
	}
	return &infrastructurev1beta2.OCIManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Annotations: map[string]string{
				// Keep the managed infrastructure and control-plane reconcilers
				// paused while exercising only the managed machine-pool controller.
				clusterv1.PausedAnnotation: "",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: infrastructurev1beta2.OCIManagedClusterSpec{
			OCIResourceIdentifier: "managed-pool-integration",
			CompartmentId:         "ocid1.compartment.oc1..integration",
			Region:                scope.MockTestRegion,
			NetworkSpec:           network,
			AvailabilityDomains: map[string]infrastructurev1beta2.OCIAvailabilityDomain{
				"integration-ad-1": {
					Name:         "integration-ad-1",
					FaultDomains: []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
				},
			},
		},
	}
}

func managedMachinePoolControlPlane(cluster *clusterv1.Cluster, version string) *infrastructurev1beta2.OCIManagedControlPlane {
	return &infrastructurev1beta2.OCIManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: infrastructurev1beta2.OCIManagedControlPlaneSpec{
			ID:      common.String("ocid1.cluster.oc1..managed-pool"),
			Version: common.String(version),
		},
	}
}

func managedCAPIMachinePool(cluster *clusterv1.Cluster, version string, replicas int32) *clusterv1.MachinePool {
	return &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-pool-workers",
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
						Kind:     scope.OCIManagedMachinePoolKind,
						Name:     "managed-pool-workers",
					},
					Version: version,
				},
			},
		},
	}
}

func managedInfrastructureMachinePool(machinePool *clusterv1.MachinePool, clusterName, version string) *infrav2exp.OCIManagedMachinePool {
	return &infrav2exp.OCIManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machinePool.Name,
			Namespace: machinePool.Namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(machinePool, clusterv1.GroupVersion.WithKind("MachinePool")),
			},
		},
		Spec: infrav2exp.OCIManagedMachinePoolSpec{
			Version:   common.String(version),
			NodeShape: "VM.Standard.E4.Flex",
			NodeShapeConfig: &infrav2exp.NodeShapeConfig{
				Ocpus:       common.String("1"),
				MemoryInGBs: common.String("16"),
			},
			NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
				ImageId:             common.String("ocid1.image.oc1..managed-pool-v1"),
				BootVolumeSizeInGBs: common.Int64(50),
			},
		},
	}
}

func waitForManagedMachinePool(t *testing.T, key client.ObjectKey, replicas, machineObjects, creates, updates int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		active, createCalls, updateCalls, _, _ := fakeOCI.oke.nodePoolCounts()
		g.Expect(active).To(Equal(1))
		g.Expect(createCalls).To(Equal(creates))
		g.Expect(updateCalls).To(Equal(updates))

		stored := &infrav2exp.OCIManagedMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrav2exp.ManagedMachinePoolFinalizer)).To(BeTrue())
		g.Expect(stored.Spec.ID).NotTo(BeNil())
		g.Expect(stored.Spec.ProviderID).NotTo(BeNil())
		g.Expect(stored.Spec.ProviderIDList).To(HaveLen(replicas))
		g.Expect(stored.Status.Ready).To(BeTrue())
		g.Expect(stored.Status.Replicas).To(Equal(int32(replicas)))
		g.Expect(stored.Status.InfrastructureMachineKind).To(Equal("OCIMachinePoolMachine"))
		g.Expect(v1beta1conditions.IsTrue(stored, infrav2exp.NodePoolReadyCondition)).To(BeTrue())

		machines := &infrav2exp.OCIMachinePoolMachineList{}
		g.Expect(testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(key.Namespace))).To(Succeed())
		// Scale-down cleanup is completed by the core CAPI MachinePool controller
		// and Kubernetes garbage collector, neither of which runs in envtest.
		g.Expect(machines.Items).To(HaveLen(machineObjects))
		for i := range machines.Items {
			g.Expect(machines.Items[i].Status.Ready).To(BeTrue())
		}
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func updateMachinePoolReplicas(t *testing.T, machinePool *clusterv1.MachinePool, replicas int32) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() error {
		stored := &clusterv1.MachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(machinePool), stored); err != nil {
			return err
		}
		stored.Spec.Replicas = &replicas
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func waitForNodePoolUpdateCount(t *testing.T, expected int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() int {
		_, _, updates, _, _ := fakeOCI.oke.nodePoolCounts()
		return updates
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(expected))
}

func waitForNodePoolInspectionsToSettle(t *testing.T) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	stableSince := time.Now()
	_, _, _, _, previous := fakeOCI.oke.nodePoolCounts()
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		_, _, _, _, current := fakeOCI.oke.nodePoolCounts()
		if current != previous {
			previous = current
			stableSince = time.Now()
			continue
		}
		if time.Since(stableSince) >= 300*time.Millisecond {
			return current
		}
	}
	t.Fatalf("node-pool inspections did not settle")
	return 0
}

func triggerMachinePoolReconcile(t *testing.T, machinePool *clusterv1.MachinePool) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() error {
		stored := &clusterv1.MachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(machinePool), stored); err != nil {
			return err
		}
		if stored.Annotations == nil {
			stored.Annotations = map[string]string{}
		}
		stored.Annotations["integration-test/reconcile"] = time.Now().UTC().Format(time.RFC3339Nano)
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}
