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
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/oracle/oci-go-sdk/v65/core"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOCIMachinePoolAdoptsOnlyOwnedInstancePool(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fixture := createInstancePoolFixture(t, false, 2)
	ownedID := "ocid1.instancepool.oc1..adopted-owned"
	foreignID := "ocid1.instancepool.oc1..adopted-foreign"
	base := core.InstancePool{
		CompartmentId:           common.String(fixture.ociCluster.Spec.CompartmentId),
		InstanceConfigurationId: common.String("ocid1.instanceconfiguration.oc1..adopted-old"),
		LifecycleState:          core.InstancePoolLifecycleStateRunning,
		Size:                    common.Int(2),
		DisplayName:             common.String(fixture.ociMachinePool.Name),
	}
	foreign := base
	foreign.Id = common.String(foreignID)
	foreign.FreeformTags = map[string]string{"owner": "another-cluster"}
	fakeOCI.computeManagement.seedInstancePool(foreign)
	owned := base
	owned.Id = common.String(ownedID)
	owned.LifecycleState = core.InstancePoolLifecycleStateProvisioning
	owned.FreeformTags = ociutil.BuildClusterTags(fixture.ociCluster.Spec.OCIResourceIdentifier)
	fakeOCI.computeManagement.seedInstancePool(owned)

	unpauseCluster(t, fixture.cluster)
	key := client.ObjectKeyFromObject(fixture.ociMachinePool)
	g.Eventually(func(g Gomega) {
		stored := &infrav2exp.OCIMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(stored.Spec.OCID).To(Equal(common.String(ownedID)))
		g.Expect(stored.Status.Ready).To(BeFalse())
		condition := v1beta1conditions.Get(stored, infrav2exp.InstancePoolReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(infrav2exp.InstancePoolNotReadyReason))
		_, pools, configCreates, _, poolCreates, _, _, _ := fakeOCI.computeManagement.counts()
		g.Expect(pools).To(Equal(2))
		g.Expect(configCreates).To(Equal(1))
		g.Expect(poolCreates).To(BeZero())
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	fakeOCI.computeManagement.setInstancePoolState(ownedID, core.InstancePoolLifecycleStateRunning)
	triggerObjectReconcile(t, fixture.ociMachinePool)
	g.Eventually(func(g Gomega) {
		stored := &infrav2exp.OCIMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(stored.Spec.ProviderIDList).To(HaveLen(2))
		g.Expect(stored.Status.Replicas).To(Equal(int32(2)))
		g.Expect(stored.Status.Ready).To(BeTrue())
		_, _, _, _, poolCreates, poolUpdates, _, _ := fakeOCI.computeManagement.counts()
		g.Expect(poolCreates).To(BeZero())
		g.Expect(poolUpdates).To(Equal(1))
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Expect(testEnvironment.Delete(testContext, fixture.ociMachinePool)).To(Succeed())
	waitForInstancePoolTerminateCount(t, 1)
	triggerObjectReconcile(t, fixture.ociMachinePool)
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, &infrav2exp.OCIMachinePool{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
	_, pools, _, _, creates, _, terminates, _ := fakeOCI.computeManagement.counts()
	g.Expect(pools).To(Equal(1), "foreign instance pool must not be deleted")
	g.Expect(creates).To(BeZero())
	g.Expect(terminates).To(Equal(1))
}

func TestOCIManagedMachinePoolAdoptsOnlyOwnedNodePool(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fixture := createManagedMachinePoolFixture(t, false, 2)
	prepareManagedMachinePoolForAdoption(t, fixture)
	ownedID := "ocid1.nodepool.oc1..adopted-owned"
	foreignID := "ocid1.nodepool.oc1..adopted-foreign"
	owned := adoptedManagedNodePool(fixture, ownedID, ociutil.BuildClusterTags(fixture.managedCluster.Spec.OCIResourceIdentifier), 2)
	foreign := owned
	foreign.Id = common.String(foreignID)
	foreign.FreeformTags = map[string]string{"owner": "another-cluster"}
	fakeOCI.oke.seedNodePool(foreign)
	fakeOCI.oke.seedNodePool(owned)

	unpauseCluster(t, fixture.cluster)
	key := client.ObjectKeyFromObject(fixture.managedMachinePool)
	g.Eventually(func(g Gomega) {
		stored := &infrav2exp.OCIManagedMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(stored.Spec.ID).To(Equal(common.String(ownedID)))
		g.Expect(stored.Spec.ProviderIDList).To(HaveLen(2))
		g.Expect(stored.Status.Replicas).To(Equal(int32(2)))
		g.Expect(stored.Status.Ready).To(BeTrue())
		active, creates, _, _, _ := fakeOCI.oke.nodePoolCounts()
		g.Expect(active).To(Equal(2))
		g.Expect(creates).To(BeZero())
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Expect(testEnvironment.Delete(testContext, fixture.managedMachinePool)).To(Succeed())
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, &infrav2exp.OCIManagedMachinePool{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
	active, creates, _, deletes, _ := fakeOCI.oke.nodePoolCounts()
	g.Expect(active).To(Equal(1), "foreign managed node pool must not be deleted")
	g.Expect(creates).To(BeZero())
	g.Expect(deletes).To(Equal(1))
}

func TestOCIVirtualMachinePoolAdoptsOnlyOwnedVirtualNodePool(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fixture := createVirtualMachinePoolFixture(t, false, 2)
	prepareVirtualMachinePoolForAdoption(t, fixture)
	ownedID := "ocid1.virtualnodepool.oc1..adopted-owned"
	foreignID := "ocid1.virtualnodepool.oc1..adopted-foreign"
	owned := adoptedVirtualNodePool(fixture, ownedID, ociutil.BuildClusterTags(fixture.managedCluster.Spec.OCIResourceIdentifier), 2)
	foreign := owned
	foreign.Id = common.String(foreignID)
	foreign.FreeformTags = map[string]string{"owner": "another-cluster"}
	fakeOCI.oke.seedVirtualNodePool(foreign)
	fakeOCI.oke.seedVirtualNodePool(owned)

	unpauseCluster(t, fixture.cluster)
	key := client.ObjectKeyFromObject(fixture.virtualMachinePool)
	g.Eventually(func(g Gomega) {
		stored := &infrav2exp.OCIVirtualMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(stored.Spec.ID).To(Equal(common.String(ownedID)))
		g.Expect(stored.Spec.ProviderIDList).To(HaveLen(2))
		g.Expect(stored.Status.Replicas).To(Equal(int32(2)))
		g.Expect(stored.Status.Ready).To(BeTrue())
		active, creates, _, _, _ := fakeOCI.oke.virtualNodePoolCounts()
		g.Expect(active).To(Equal(2))
		g.Expect(creates).To(BeZero())
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Expect(testEnvironment.Delete(testContext, fixture.virtualMachinePool)).To(Succeed())
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, &infrav2exp.OCIVirtualMachinePool{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
	active, creates, _, deletes, _ := fakeOCI.oke.virtualNodePoolCounts()
	g.Expect(active).To(Equal(1), "foreign virtual node pool must not be deleted")
	g.Expect(creates).To(BeZero())
	g.Expect(deletes).To(Equal(1))
}

type instancePoolFixture struct {
	namespace      *corev1.Namespace
	cluster        *clusterv1.Cluster
	ociCluster     *infrastructurev1beta2.OCICluster
	bootstrap      *corev1.Secret
	machinePool    *clusterv1.MachinePool
	ociMachinePool *infrav2exp.OCIMachinePool
}

func createInstancePoolFixture(t *testing.T, externallyManaged bool, replicas int32) *instancePoolFixture {
	t.Helper()
	g := NewWithT(t)
	fixture := &instancePoolFixture{}
	fixture.namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "instance-pool-advanced-"}}
	g.Expect(testEnvironment.Create(testContext, fixture.namespace)).To(Succeed())
	fixture.cluster = instancePoolCAPICluster(fixture.namespace.Name)
	g.Expect(testEnvironment.Create(testContext, fixture.cluster)).To(Succeed())
	storedCluster := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.cluster), storedCluster)).To(Succeed())
	storedCluster.Status.Initialization.InfrastructureProvisioned = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedCluster)).To(Succeed())
	fixture.ociCluster = instancePoolOCICluster(fixture.cluster)
	g.Expect(testEnvironment.Create(testContext, fixture.ociCluster)).To(Succeed())
	storedOCICluster := &infrastructurev1beta2.OCICluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.ociCluster), storedOCICluster)).To(Succeed())
	storedOCICluster.Status.Ready = true
	g.Expect(testEnvironment.Status().Update(testContext, storedOCICluster)).To(Succeed())
	fixture.bootstrap = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-pool-advanced-bootstrap", Namespace: fixture.namespace.Name},
		Data:       map[string][]byte{"value": []byte("#!/bin/sh\necho instance-pool-advanced")},
	}
	g.Expect(testEnvironment.Create(testContext, fixture.bootstrap)).To(Succeed())
	fixture.machinePool = instanceCAPIMachinePool(fixture.cluster, fixture.bootstrap.Name, replicas)
	if externallyManaged {
		fixture.machinePool.Annotations = map[string]string{clusterv1.ReplicasManagedByAnnotation: "integration-test"}
	}
	g.Expect(testEnvironment.Create(testContext, fixture.machinePool)).To(Succeed())
	fixture.ociMachinePool = instanceInfrastructureMachinePool(fixture.machinePool, fixture.cluster.Name)
	g.Expect(testEnvironment.Create(testContext, fixture.ociMachinePool)).To(Succeed())
	t.Cleanup(func() {
		machines := &infrav2exp.OCIMachinePoolMachineList{}
		if err := testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(fixture.namespace.Name)); err == nil {
			for i := range machines.Items {
				deleteAndIgnoreNotFound(t, &machines.Items[i])
			}
		}
		for _, object := range []client.Object{fixture.ociMachinePool, fixture.machinePool, fixture.bootstrap, fixture.ociCluster, fixture.cluster, fixture.namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})
	return fixture
}

func prepareManagedMachinePoolForAdoption(t *testing.T, fixture *managedMachinePoolFixture) {
	t.Helper()
	g := NewWithT(t)
	stored := &infrav2exp.OCIManagedMachinePool{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.managedMachinePool), stored)).To(Succeed())
	stored.Spec.NodePoolNodeConfig = &infrav2exp.NodePoolNodeConfig{
		NsgNames: []string{"worker"},
		PlacementConfigs: []infrav2exp.PlacementConfig{{
			AvailabilityDomain: common.String("integration-ad-1"),
			FaultDomains:       []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
			SubnetName:         common.String("worker"),
		}},
		NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{CniType: infrastructurev1beta2.FlannelCNI},
	}
	g.Expect(testEnvironment.Update(testContext, stored)).To(Succeed())
}

func adoptedManagedNodePool(fixture *managedMachinePoolFixture, id string, tags map[string]string, size int) oke.NodePool {
	workerSubnetID, workerNSGID := managedNetworkIDs(fixture.managedCluster, infrastructurev1beta2.WorkerRole)
	return oke.NodePool{
		Id:                common.String(id),
		LifecycleState:    oke.NodePoolLifecycleStateActive,
		CompartmentId:     common.String(fixture.managedCluster.Spec.CompartmentId),
		ClusterId:         fixture.controlPlane.Spec.ID,
		Name:              common.String(fixture.managedMachinePool.Name),
		KubernetesVersion: common.String(fixture.version),
		NodeShape:         common.String("VM.Standard.E4.Flex"),
		NodeShapeConfig:   &oke.NodeShapeConfig{Ocpus: common.Float32(1), MemoryInGBs: common.Float32(16)},
		NodeSourceDetails: oke.NodeSourceViaImageDetails{ImageId: common.String("ocid1.image.oc1..managed-pool-v1"), BootVolumeSizeInGBs: common.Int64(50)},
		SshPublicKey:      common.String(""),
		NodeConfigDetails: &oke.NodePoolNodeConfigDetails{
			Size:                            common.Int(size),
			NsgIds:                          []string{workerNSGID},
			PlacementConfigs:                []oke.NodePoolPlacementConfigDetails{{AvailabilityDomain: common.String("integration-ad-1"), SubnetId: common.String(workerSubnetID), FaultDomains: []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"}}},
			NodePoolPodNetworkOptionDetails: oke.FlannelOverlayNodePoolPodNetworkOptionDetails{},
		},
		FreeformTags: cloneStringMap(tags),
		DefinedTags:  map[string]map[string]interface{}{},
	}
}

func prepareVirtualMachinePoolForAdoption(t *testing.T, fixture *virtualMachinePoolFixture) {
	t.Helper()
	g := NewWithT(t)
	stored := &infrav2exp.OCIVirtualMachinePool{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.virtualMachinePool), stored)).To(Succeed())
	stored.Spec.PlacementConfigs = []infrav2exp.VirtualNodepoolPlacementConfig{{
		AvailabilityDomain: common.String("integration-ad-1"),
		FaultDomains:       []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
		SubnetName:         common.String("worker"),
	}}
	stored.Spec.NsgNames = []string{"worker"}
	stored.Spec.PodConfiguration = infrav2exp.PodConfig{SubnetName: common.String("pod"), NsgNames: []string{"pod"}}
	g.Expect(testEnvironment.Update(testContext, stored)).To(Succeed())
}

func adoptedVirtualNodePool(fixture *virtualMachinePoolFixture, id string, tags map[string]string, size int) oke.VirtualNodePool {
	workerSubnetID, workerNSGID := managedNetworkIDs(fixture.managedCluster, infrastructurev1beta2.WorkerRole)
	podSubnetID, podNSGID := managedNetworkIDs(fixture.managedCluster, infrastructurev1beta2.PodRole)
	return oke.VirtualNodePool{
		Id:                common.String(id),
		LifecycleState:    oke.VirtualNodePoolLifecycleStateActive,
		CompartmentId:     common.String(fixture.managedCluster.Spec.CompartmentId),
		ClusterId:         fixture.controlPlane.Spec.ID,
		DisplayName:       common.String(fixture.virtualMachinePool.Name),
		KubernetesVersion: common.String(fixture.version),
		Size:              common.Int(size),
		PlacementConfigurations: []oke.PlacementConfiguration{{
			AvailabilityDomain: common.String("integration-ad-1"), SubnetId: common.String(workerSubnetID), FaultDomain: []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
		}},
		NsgIds:           []string{workerNSGID},
		PodConfiguration: &oke.PodConfiguration{SubnetId: common.String(podSubnetID), NsgIds: []string{podNSGID}},
		FreeformTags:     cloneStringMap(tags),
		DefinedTags:      map[string]map[string]interface{}{},
	}
}

func managedNetworkIDs(cluster *infrastructurev1beta2.OCIManagedCluster, role infrastructurev1beta2.Role) (string, string) {
	var subnetID, nsgID string
	for _, subnet := range cluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == role {
			subnetID = stringValue(subnet.ID)
			break
		}
	}
	for _, nsg := range cluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
		if nsg.Role == role {
			nsgID = stringValue(nsg.ID)
			break
		}
	}
	return subnetID, nsgID
}
