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

func TestOCIMachinePoolLifecycle(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "instance-pool-integration-"}}
	g.Expect(testEnvironment.Create(testContext, namespace)).To(Succeed())

	cluster := instancePoolCAPICluster(namespace.Name)
	g.Expect(testEnvironment.Create(testContext, cluster)).To(Succeed())
	storedCluster := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Status.Initialization.InfrastructureProvisioned = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedCluster)).To(Succeed())

	ociCluster := instancePoolOCICluster(cluster)
	g.Expect(testEnvironment.Create(testContext, ociCluster)).To(Succeed())
	storedOCICluster := &infrastructurev1beta2.OCICluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(ociCluster), storedOCICluster)).To(Succeed())
	storedOCICluster.Status.Ready = true
	g.Expect(testEnvironment.Status().Update(testContext, storedOCICluster)).To(Succeed())

	bootstrapSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-pool-bootstrap", Namespace: namespace.Name},
		Data:       map[string][]byte{"value": []byte("#!/bin/sh\necho instance-pool")},
	}
	g.Expect(testEnvironment.Create(testContext, bootstrapSecret)).To(Succeed())

	machinePool := instanceCAPIMachinePool(cluster, bootstrapSecret.Name, 2)
	g.Expect(testEnvironment.Create(testContext, machinePool)).To(Succeed())
	ociMachinePool := instanceInfrastructureMachinePool(machinePool, cluster.Name)
	g.Expect(testEnvironment.Create(testContext, ociMachinePool)).To(Succeed())

	t.Cleanup(func() {
		machines := &infrav2exp.OCIMachinePoolMachineList{}
		if err := testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(namespace.Name)); err == nil {
			for i := range machines.Items {
				deleteAndIgnoreNotFound(t, &machines.Items[i])
			}
		}
		for _, object := range []client.Object{ociMachinePool, machinePool, bootstrapSecret, ociCluster, cluster, namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})

	g.Consistently(func() int {
		_, _, _, _, creates, _, _, _ := fakeOCI.computeManagement.counts()
		return creates
	}).WithTimeout(time.Second).WithPolling(100 * time.Millisecond).Should(BeZero())

	storedCluster = &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())

	ociMachinePoolKey := client.ObjectKeyFromObject(ociMachinePool)
	waitForInstanceMachinePool(t, ociMachinePoolKey, 2, 2, 2, 1, 1, 0)

	updateMachinePoolReplicas(t, machinePool, 3)
	waitForInstancePoolUpdateCount(t, 1)
	triggerInstanceMachinePoolReconcile(t, ociMachinePool)
	waitForInstanceMachinePool(t, ociMachinePoolKey, 3, 3, 3, 1, 1, 1)

	g.Eventually(func() error {
		stored := &infrav2exp.OCIMachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, ociMachinePoolKey, stored); err != nil {
			return err
		}
		stored.Spec.InstanceConfiguration.ShapeConfig.MemoryInGBs = common.String("32")
		stored.Spec.InstanceConfiguration.InstanceSourceViaImageDetails.ImageId = common.String("ocid1.image.oc1..instance-pool-v2")
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForInstanceConfigurationCreateCount(t, 2)
	waitForInstancePoolUpdateCount(t, 2)
	triggerInstanceMachinePoolReconcile(t, ociMachinePool)
	waitForInstanceMachinePool(t, ociMachinePoolKey, 3, 3, 3, 1, 2, 2)

	_, _, configCreates, _, poolCreates, poolUpdates, _, _ := fakeOCI.computeManagement.counts()
	g.Expect(configCreates).To(Equal(2))
	g.Expect(poolCreates).To(Equal(1))
	g.Expect(poolUpdates).To(Equal(2))
	inspectionsBefore := waitForInstancePoolInspectionsToSettle(t)

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
		_, _, _, _, _, _, _, inspections := fakeOCI.computeManagement.counts()
		return inspections
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).
		Should(BeNumerically(">", inspectionsBefore))
	_, _, configCreates, _, poolCreates, poolUpdates, _, _ = fakeOCI.computeManagement.counts()
	g.Expect(configCreates).To(Equal(2))
	g.Expect(poolCreates).To(Equal(1))
	g.Expect(poolUpdates).To(Equal(2))

	updateMachinePoolReplicas(t, machinePool, 1)
	waitForInstancePoolUpdateCount(t, 3)
	waitForInstancePoolInspectionsToSettle(t)
	triggerInstanceMachinePoolReconcile(t, ociMachinePool)
	waitForInstanceMachinePool(t, ociMachinePoolKey, 1, 3, 3, 1, 2, 3)

	g.Expect(testEnvironment.Delete(testContext, ociMachinePool)).To(Succeed())
	waitForInstancePoolTerminateCount(t, 1)
	triggerInstanceMachinePoolReconcile(t, ociMachinePool)
	g.Eventually(func(g Gomega) {
		configurations, pools, _, configDeletes, _, _, poolTerminates, _ := fakeOCI.computeManagement.counts()
		g.Expect(configurations).To(BeZero())
		g.Expect(pools).To(BeZero())
		g.Expect(configDeletes).To(Equal(2))
		g.Expect(poolTerminates).To(Equal(1))
		err := testEnvironment.GetAPIReader().Get(testContext, ociMachinePoolKey, &infrav2exp.OCIMachinePool{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected OCI machine-pool deletion, got %v", err)
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func instancePoolCAPICluster(namespace string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-pool", Namespace: namespace},
		Spec: clusterv1.ClusterSpec{
			Paused: common.Bool(true),
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     scope.OCIClusterKind,
				Name:     "instance-pool",
			},
		},
	}
}

func instancePoolOCICluster(cluster *clusterv1.Cluster) *infrastructurev1beta2.OCICluster {
	network := infrastructurev1beta2.NetworkSpec{
		Vcn: infrastructurev1beta2.VCN{
			ID:   common.String("ocid1.vcn.oc1..instance-pool"),
			CIDR: "10.0.0.0/16",
			Subnets: []*infrastructurev1beta2.Subnet{
				{Role: infrastructurev1beta2.ControlPlaneEndpointRole, Name: "control-plane-endpoint", ID: common.String("ocid1.subnet.oc1..instance-pool-1"), CIDR: "10.0.0.0/28", Type: infrastructurev1beta2.Public},
				{Role: infrastructurev1beta2.ControlPlaneRole, Name: "control-plane", ID: common.String("ocid1.subnet.oc1..instance-pool-2"), CIDR: "10.0.1.0/24", Type: infrastructurev1beta2.Private},
				{Role: infrastructurev1beta2.ServiceLoadBalancerRole, Name: "service-lb", ID: common.String("ocid1.subnet.oc1..instance-pool-3"), CIDR: "10.0.2.0/24", Type: infrastructurev1beta2.Public},
				{Role: infrastructurev1beta2.WorkerRole, Name: "worker", ID: common.String("ocid1.subnet.oc1..instance-pool-4"), CIDR: "10.0.3.0/24", Type: infrastructurev1beta2.Private},
			},
			NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
				List: []*infrastructurev1beta2.NSG{
					{Role: infrastructurev1beta2.ControlPlaneEndpointRole, Name: "control-plane-endpoint", ID: common.String("ocid1.networksecuritygroup.oc1..instance-pool-1")},
					{Role: infrastructurev1beta2.ControlPlaneRole, Name: "control-plane", ID: common.String("ocid1.networksecuritygroup.oc1..instance-pool-2")},
					{Role: infrastructurev1beta2.ServiceLoadBalancerRole, Name: "service-lb", ID: common.String("ocid1.networksecuritygroup.oc1..instance-pool-3")},
					{Role: infrastructurev1beta2.WorkerRole, Name: "worker", ID: common.String("ocid1.networksecuritygroup.oc1..instance-pool-4")},
				},
			},
		},
	}
	return &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Annotations: map[string]string{
				// Keep the infrastructure controller from reconciling networking;
				// this test is scoped to the self-managed machine-pool controller.
				clusterv1.ManagedByAnnotation: "integration-test",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			OCIResourceIdentifier: "instance-pool-integration",
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

func instanceCAPIMachinePool(cluster *clusterv1.Cluster, bootstrapSecret string, replicas int32) *clusterv1.MachinePool {
	return &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-pool-workers",
			Namespace: cluster.Namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
		},
		Spec: clusterv1.MachinePoolSpec{
			ClusterName: cluster.Name,
			Replicas:    &replicas,
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName: cluster.Name,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: common.String(bootstrapSecret),
					},
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: infrastructurev1beta2.GroupVersion.Group,
						Kind:     scope.OCIMachinePoolKind,
						Name:     "instance-pool-workers",
					},
				},
			},
		},
	}
}

func instanceInfrastructureMachinePool(machinePool *clusterv1.MachinePool, clusterName string) *infrav2exp.OCIMachinePool {
	return &infrav2exp.OCIMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machinePool.Name,
			Namespace: machinePool.Namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(machinePool, clusterv1.GroupVersion.WithKind("MachinePool")),
			},
		},
		Spec: infrav2exp.OCIMachinePoolSpec{
			InstanceConfiguration: infrav2exp.InstanceConfiguration{
				Shape: common.String("VM.Standard.E4.Flex"),
				ShapeConfig: &infrav2exp.ShapeConfig{
					Ocpus:       common.String("1"),
					MemoryInGBs: common.String("16"),
				},
				InstanceSourceViaImageDetails: &infrav2exp.InstanceSourceViaImageConfig{
					ImageId:             common.String("ocid1.image.oc1..instance-pool-v1"),
					BootVolumeSizeInGBs: common.Int64(50),
				},
			},
		},
	}
}

func waitForInstanceMachinePool(t *testing.T, key client.ObjectKey, replicas, machineObjects, statusReplicas, configurations, configCreates, poolUpdates int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		activeConfigurations, pools, createCalls, _, poolCreates, updateCalls, _, _ := fakeOCI.computeManagement.counts()
		g.Expect(activeConfigurations).To(Equal(configurations))
		g.Expect(pools).To(Equal(1))
		g.Expect(createCalls).To(Equal(configCreates))
		g.Expect(poolCreates).To(Equal(1))
		g.Expect(updateCalls).To(Equal(poolUpdates))

		poolSize, configurationID := fakeOCI.computeManagement.instancePoolState()
		g.Expect(poolSize).To(Equal(replicas))
		g.Expect(configurationID).NotTo(BeEmpty())

		stored := &infrav2exp.OCIMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrav2exp.MachinePoolFinalizer)).To(BeTrue())
		g.Expect(stored.Spec.OCID).NotTo(BeNil())
		g.Expect(stored.Spec.ProviderID).NotTo(BeNil())
		g.Expect(stored.Spec.InstanceConfiguration.InstanceConfigurationId).NotTo(BeNil())
		g.Expect(*stored.Spec.InstanceConfiguration.InstanceConfigurationId).To(Equal(configurationID))
		g.Expect(stored.Spec.ProviderIDList).To(HaveLen(replicas))
		g.Expect(stored.Annotations[scope.InstanceConfigurationHashAnnotation]).NotTo(BeEmpty())
		g.Expect(stored.Annotations[scope.BootstrapDataHashAnnotation]).NotTo(BeEmpty())
		g.Expect(stored.Status.Ready).To(BeTrue())
		g.Expect(stored.Status.Replicas).To(Equal(int32(statusReplicas)))
		g.Expect(stored.Status.InfrastructureMachineKind).To(Equal("OCIMachinePoolMachine"))
		g.Expect(v1beta1conditions.IsTrue(stored, infrav2exp.InstancePoolReadyCondition)).To(BeTrue())

		machines := &infrav2exp.OCIMachinePoolMachineList{}
		g.Expect(testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(key.Namespace))).To(Succeed())
		// Scale-down cleanup is completed by the core CAPI MachinePool controller
		// and Kubernetes garbage collector, neither of which runs in envtest. The
		// self-managed reconciler returns before refreshing Status.Replicas while
		// those stale children have no owner Machines.
		g.Expect(machines.Items).To(HaveLen(machineObjects))
		for i := range machines.Items {
			g.Expect(machines.Items[i].Status.Ready).To(BeTrue())
			g.Expect(machines.Items[i].Spec.MachineType).To(Equal(infrav2exp.SelfManaged))
		}
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func waitForInstancePoolUpdateCount(t *testing.T, expected int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() int {
		_, _, _, _, _, updates, _, _ := fakeOCI.computeManagement.counts()
		return updates
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(expected))
}

func waitForInstanceConfigurationCreateCount(t *testing.T, expected int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() int {
		_, _, creates, _, _, _, _, _ := fakeOCI.computeManagement.counts()
		return creates
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(expected))
}

func waitForInstancePoolInspectionsToSettle(t *testing.T) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	stableSince := time.Now()
	_, _, _, _, _, _, _, previous := fakeOCI.computeManagement.counts()
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		_, _, _, _, _, _, _, current := fakeOCI.computeManagement.counts()
		if current != previous {
			previous = current
			stableSince = time.Now()
			continue
		}
		if time.Since(stableSince) >= 300*time.Millisecond {
			return current
		}
	}
	t.Fatalf("instance-pool inspections did not settle")
	return 0
}

func triggerInstanceMachinePoolReconcile(t *testing.T, machinePool *infrav2exp.OCIMachinePool) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() error {
		stored := &infrav2exp.OCIMachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(machinePool), stored); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if stored.Annotations == nil {
			stored.Annotations = map[string]string{}
		}
		stored.Annotations["integration-test/reconcile"] = time.Now().UTC().Format(time.RFC3339Nano)
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func waitForInstancePoolTerminateCount(t *testing.T, expected int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() int {
		_, _, _, _, _, _, terminates, _ := fakeOCI.computeManagement.counts()
		return terminates
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(expected))
}
