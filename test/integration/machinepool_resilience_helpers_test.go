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

	. "github.com/onsi/gomega"
	infrastructurev1beta3 "github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	infrav3exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta3"
	"github.com/oracle/oci-go-sdk/v65/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type managedMachinePoolFixture struct {
	namespace          *corev1.Namespace
	cluster            *clusterv1.Cluster
	managedCluster     *infrastructurev1beta3.OCIManagedCluster
	controlPlane       *infrastructurev1beta3.OCIManagedControlPlane
	machinePool        *clusterv1.MachinePool
	managedMachinePool *infrav3exp.OCIManagedMachinePool
	version            string
}

func createManagedMachinePoolFixture(t *testing.T, externallyManaged bool, replicas int32) *managedMachinePoolFixture {
	t.Helper()
	g := NewWithT(t)
	fixture := &managedMachinePoolFixture{version: "v1.34.1"}
	fixture.namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "managed-pool-resilience-"}}
	g.Expect(testEnvironment.Create(testContext, fixture.namespace)).To(Succeed())
	fixture.cluster = managedMachinePoolCAPICluster(fixture.namespace.Name)
	g.Expect(testEnvironment.Create(testContext, fixture.cluster)).To(Succeed())
	storedCluster := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.cluster), storedCluster)).To(Succeed())
	storedCluster.Status.Initialization.InfrastructureProvisioned = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedCluster)).To(Succeed())
	fixture.managedCluster = managedMachinePoolCluster(fixture.cluster)
	g.Expect(testEnvironment.Create(testContext, fixture.managedCluster)).To(Succeed())
	storedManagedCluster := &infrastructurev1beta3.OCIManagedCluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.managedCluster), storedManagedCluster)).To(Succeed())
	storedManagedCluster.Status.Initialization.Provisioned = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedManagedCluster)).To(Succeed())
	fixture.controlPlane = managedMachinePoolControlPlane(fixture.cluster, fixture.version)
	g.Expect(testEnvironment.Create(testContext, fixture.controlPlane)).To(Succeed())
	storedControlPlane := &infrastructurev1beta3.OCIManagedControlPlane{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.controlPlane), storedControlPlane)).To(Succeed())
	storedControlPlane.Status.Initialization.ControlPlaneInitialized = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedControlPlane)).To(Succeed())
	fixture.machinePool = managedCAPIMachinePool(fixture.cluster, fixture.version, replicas)
	if externallyManaged {
		fixture.machinePool.Annotations = map[string]string{clusterv1.ReplicasManagedByAnnotation: "integration-test"}
	}
	g.Expect(testEnvironment.Create(testContext, fixture.machinePool)).To(Succeed())
	fixture.managedMachinePool = managedInfrastructureMachinePool(fixture.machinePool, fixture.cluster.Name, fixture.version)
	g.Expect(testEnvironment.Create(testContext, fixture.managedMachinePool)).To(Succeed())
	t.Cleanup(func() {
		machines := &infrav3exp.OCIMachinePoolMachineList{}
		if err := testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(fixture.namespace.Name)); err == nil {
			for i := range machines.Items {
				deleteAndIgnoreNotFound(t, &machines.Items[i])
			}
		}
		for _, object := range []client.Object{fixture.managedMachinePool, fixture.machinePool, fixture.controlPlane, fixture.managedCluster, fixture.cluster, fixture.namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})
	return fixture
}

type virtualMachinePoolFixture struct {
	namespace          *corev1.Namespace
	cluster            *clusterv1.Cluster
	managedCluster     *infrastructurev1beta3.OCIManagedCluster
	controlPlane       *infrastructurev1beta3.OCIManagedControlPlane
	machinePool        *clusterv1.MachinePool
	virtualMachinePool *infrav3exp.OCIVirtualMachinePool
	version            string
}

func createVirtualMachinePoolFixture(t *testing.T, externallyManaged bool, replicas int32) *virtualMachinePoolFixture {
	t.Helper()
	g := NewWithT(t)
	fixture := &virtualMachinePoolFixture{version: "v1.34.1"}
	fixture.namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "virtual-pool-resilience-"}}
	g.Expect(testEnvironment.Create(testContext, fixture.namespace)).To(Succeed())
	fixture.cluster = managedMachinePoolCAPICluster(fixture.namespace.Name)
	g.Expect(testEnvironment.Create(testContext, fixture.cluster)).To(Succeed())
	storedCluster := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.cluster), storedCluster)).To(Succeed())
	storedCluster.Status.Initialization.InfrastructureProvisioned = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedCluster)).To(Succeed())
	fixture.managedCluster = managedMachinePoolCluster(fixture.cluster)
	g.Expect(testEnvironment.Create(testContext, fixture.managedCluster)).To(Succeed())
	storedManagedCluster := &infrastructurev1beta3.OCIManagedCluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.managedCluster), storedManagedCluster)).To(Succeed())
	storedManagedCluster.Status.Initialization.Provisioned = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedManagedCluster)).To(Succeed())
	fixture.controlPlane = managedMachinePoolControlPlane(fixture.cluster, fixture.version)
	g.Expect(testEnvironment.Create(testContext, fixture.controlPlane)).To(Succeed())
	storedControlPlane := &infrastructurev1beta3.OCIManagedControlPlane{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.controlPlane), storedControlPlane)).To(Succeed())
	storedControlPlane.Status.Initialization.ControlPlaneInitialized = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedControlPlane)).To(Succeed())
	fixture.machinePool = virtualCAPIMachinePool(fixture.cluster, fixture.version, replicas)
	if externallyManaged {
		fixture.machinePool.Annotations = map[string]string{clusterv1.ReplicasManagedByAnnotation: "integration-test"}
	}
	g.Expect(testEnvironment.Create(testContext, fixture.machinePool)).To(Succeed())
	fixture.virtualMachinePool = virtualInfrastructureMachinePool(fixture.machinePool, fixture.cluster.Name)
	g.Expect(testEnvironment.Create(testContext, fixture.virtualMachinePool)).To(Succeed())
	t.Cleanup(func() {
		machines := &infrav3exp.OCIMachinePoolMachineList{}
		if err := testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(fixture.namespace.Name)); err == nil {
			for i := range machines.Items {
				deleteAndIgnoreNotFound(t, &machines.Items[i])
			}
		}
		for _, object := range []client.Object{fixture.virtualMachinePool, fixture.machinePool, fixture.controlPlane, fixture.managedCluster, fixture.cluster, fixture.namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})
	return fixture
}
