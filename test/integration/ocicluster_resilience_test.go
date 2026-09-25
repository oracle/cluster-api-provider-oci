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
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestOCIClusterRecoversPartialNetworkAndAdoptsOnlyOwnedVCN(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fakeOCI.vcn.setFailure(createSubnetOperation, errors.New("injected subnet create failure"))

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "cluster-resilience-"}}
	g.Expect(testEnvironment.Create(testContext, namespace)).To(Succeed())
	name := "cluster-resilience"
	resourceIdentifier := "cluster-resilience-integration"
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace.Name},
		Spec: clusterv1.ClusterSpec{
			Paused: common.Bool(true),
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     scope.OCIClusterKind,
				Name:     name,
			},
		},
	}
	g.Expect(testEnvironment.Create(testContext, cluster)).To(Succeed())
	ociCluster := &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			OCIResourceIdentifier: resourceIdentifier,
			CompartmentId:         "ocid1.compartment.oc1..integration",
			Region:                scope.MockTestRegion,
			NetworkSpec:           selfManagedIntegrationNetworkSpec(),
		},
	}
	g.Expect(testEnvironment.Create(testContext, ociCluster)).To(Succeed())
	t.Cleanup(func() {
		fakeOCI.vcn.clearFailure(createSubnetOperation)
		fakeOCI.vcn.clearFailure(deleteVCNOperation)
		for _, object := range []client.Object{ociCluster, cluster, namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})

	ownedVCNID := "ocid1.vcn.oc1..adopted-owned"
	foreignVCNID := "ocid1.vcn.oc1..adopted-foreign"
	baseVCN := core.Vcn{
		CompartmentId:  common.String(ociCluster.Spec.CompartmentId),
		DisplayName:    common.String(name),
		CidrBlocks:     []string{"10.0.0.0/16"},
		LifecycleState: core.VcnLifecycleStateAvailable,
	}
	foreignVCN := baseVCN
	foreignVCN.Id = common.String(foreignVCNID)
	foreignVCN.FreeformTags = map[string]string{"owner": "another-cluster"}
	fakeOCI.vcn.seedVCN(foreignVCN)
	ownedVCN := baseVCN
	ownedVCN.Id = common.String(ownedVCNID)
	ownedVCN.FreeformTags = ociutil.BuildClusterTags(resourceIdentifier)
	fakeOCI.vcn.seedVCN(ownedVCN)

	unpauseCluster(t, cluster)
	triggerObjectReconcile(t, ociCluster)
	waitForVCNAttempt(t, createSubnetOperation, 0)
	ociClusterKey := client.ObjectKeyFromObject(ociCluster)
	g.Eventually(func(g Gomega) {
		network := fakeOCI.vcn.networkCounts()
		g.Expect(network.VCNs).To(Equal(2))
		g.Expect(network.InternetGateways).To(Equal(1))
		g.Expect(network.NATGateways).To(Equal(1))
		g.Expect(network.ServiceGateways).To(Equal(1))
		g.Expect(network.NSGs).To(Equal(4))
		g.Expect(network.RouteTables).To(Equal(2))
		g.Expect(network.Subnets).To(BeZero())
		g.Expect(network.Creates["vcn"]).To(BeZero())

		stored := &infrastructurev1beta2.OCICluster{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, ociClusterKey, stored)).To(Succeed())
		g.Expect(stored.Spec.NetworkSpec.Vcn.ID).To(Equal(common.String(ownedVCNID)))
		g.Expect(controllerutil.ContainsFinalizer(stored, infrastructurev1beta2.ClusterFinalizer)).To(BeTrue())
		condition := v1beta1conditions.Get(stored, infrastructurev1beta2.ClusterReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(condition.Reason).To(Equal(infrastructurev1beta2.SubnetReconciliationFailedReason))
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForWarningEvent(t, namespace.Name, ociCluster.Name, "ReconcileError", "injected subnet create failure")

	fakeOCI.vcn.clearFailure(createSubnetOperation)
	triggerObjectReconcile(t, ociCluster)
	g.Eventually(func(g Gomega) {
		network := fakeOCI.vcn.networkCounts()
		g.Expect(network.VCNs).To(Equal(2))
		g.Expect(network.InternetGateways).To(Equal(1))
		g.Expect(network.NATGateways).To(Equal(1))
		g.Expect(network.ServiceGateways).To(Equal(1))
		g.Expect(network.NSGs).To(Equal(4))
		g.Expect(network.RouteTables).To(Equal(2))
		g.Expect(network.Subnets).To(Equal(4))
		g.Expect(network.Creates["vcn"]).To(BeZero())
		g.Expect(network.Creates["internetgateway"]).To(Equal(1))
		g.Expect(network.Creates["natgateway"]).To(Equal(1))
		g.Expect(network.Creates["servicegateway"]).To(Equal(1))
		g.Expect(network.Creates["networksecuritygroup"]).To(Equal(4))
		g.Expect(network.Creates["routetable"]).To(Equal(2))
		g.Expect(network.Creates["subnet"]).To(Equal(4))

		stored := &infrastructurev1beta2.OCICluster{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, ociClusterKey, stored)).To(Succeed())
		g.Expect(stored.Status.Ready).To(BeTrue())
		g.Expect(v1beta1conditions.IsTrue(stored, infrastructurev1beta2.ClusterReadyCondition)).To(BeTrue())
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	deleteAttempts := fakeOCI.vcn.operationAttemptCount(deleteVCNOperation)
	fakeOCI.vcn.setFailure(deleteVCNOperation, errors.New("injected VCN delete failure"))
	g.Expect(testEnvironment.Delete(testContext, ociCluster)).To(Succeed())
	waitForVCNAttempt(t, deleteVCNOperation, deleteAttempts)
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCICluster{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, ociClusterKey, stored)).To(Succeed())
		g.Expect(stored.DeletionTimestamp.IsZero()).To(BeFalse())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrastructurev1beta2.ClusterFinalizer)).To(BeTrue())
		network := fakeOCI.vcn.networkCounts()
		g.Expect(network.VCNs).To(Equal(2))
		g.Expect(network.InternetGateways + network.NATGateways + network.ServiceGateways + network.NSGs + network.RouteTables + network.Subnets).To(BeZero())
		condition := v1beta1conditions.Get(stored, infrastructurev1beta2.ClusterReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(infrastructurev1beta2.VcnReconciliationFailedReason))
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForWarningEvent(t, namespace.Name, ociCluster.Name, "ReconcileError", "injected VCN delete failure")

	fakeOCI.vcn.clearFailure(deleteVCNOperation)
	triggerObjectReconcile(t, ociCluster)
	g.Eventually(func(g Gomega) {
		err := testEnvironment.GetAPIReader().Get(testContext, ociClusterKey, &infrastructurev1beta2.OCICluster{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected OCICluster deletion, got %v", err)
		g.Expect(fakeOCI.vcn.networkCounts().VCNs).To(Equal(1), "foreign VCN must not be deleted")
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func selfManagedIntegrationNetworkSpec() infrastructurev1beta2.NetworkSpec {
	return infrastructurev1beta2.NetworkSpec{
		Vcn: infrastructurev1beta2.VCN{
			CIDR: "10.0.0.0/16",
			Subnets: []*infrastructurev1beta2.Subnet{
				{Role: infrastructurev1beta2.ControlPlaneEndpointRole, Name: "control-plane-endpoint", CIDR: "10.0.0.0/28", Type: infrastructurev1beta2.Public},
				{Role: infrastructurev1beta2.ControlPlaneRole, Name: "control-plane", CIDR: "10.0.1.0/24", Type: infrastructurev1beta2.Private},
				{Role: infrastructurev1beta2.ServiceLoadBalancerRole, Name: "service-lb", CIDR: "10.0.2.0/24", Type: infrastructurev1beta2.Public},
				{Role: infrastructurev1beta2.WorkerRole, Name: "worker", CIDR: "10.0.3.0/24", Type: infrastructurev1beta2.Private},
			},
			NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
				List: []*infrastructurev1beta2.NSG{
					{Role: infrastructurev1beta2.ControlPlaneEndpointRole, Name: "control-plane-endpoint"},
					{Role: infrastructurev1beta2.ControlPlaneRole, Name: "control-plane"},
					{Role: infrastructurev1beta2.ServiceLoadBalancerRole, Name: "service-lb"},
					{Role: infrastructurev1beta2.WorkerRole, Name: "worker"},
				},
			},
		},
	}
}
