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
	"github.com/oracle/oci-go-sdk/v65/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestOCIClusterLifecycle(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "ocicluster-integration-"},
	}
	g.Expect(testEnvironment.Create(testContext, namespace)).To(Succeed())

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-lifecycle",
			Namespace: namespace.Name,
		},
		Spec: clusterv1.ClusterSpec{
			Paused: common.Bool(true),
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     scope.OCIClusterKind,
				Name:     "cluster-lifecycle",
			},
		},
	}
	g.Expect(testEnvironment.Create(testContext, cluster)).To(Succeed())

	ociCluster := &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: namespace.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			CompartmentId: "ocid1.compartment.oc1..integration",
			Region:        scope.MockTestRegion,
			NetworkSpec: infrastructurev1beta2.NetworkSpec{
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
			},
		},
	}
	g.Expect(testEnvironment.Create(testContext, ociCluster)).To(Succeed())
	t.Cleanup(func() {
		for _, object := range []client.Object{ociCluster, cluster, namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})

	g.Consistently(func() int {
		return fakeOCI.vcn.networkCounts().Creates["vcn"]
	}).WithTimeout(time.Second).WithPolling(100 * time.Millisecond).Should(Equal(0))

	storedCluster := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())

	ociClusterKey := client.ObjectKeyFromObject(ociCluster)
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCICluster{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, ociClusterKey, stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrastructurev1beta2.ClusterFinalizer)).To(BeTrue())
		g.Expect(stored.Status.Ready).To(BeTrue())
		g.Expect(v1beta1conditions.IsTrue(stored, infrastructurev1beta2.ClusterReadyCondition)).To(BeTrue())
		g.Expect(stored.Spec.ControlPlaneEndpoint.Host).To(Equal("203.0.113.10"))
		g.Expect(stored.Spec.ControlPlaneEndpoint.Port).To(Equal(int32(6443)))
		g.Expect(stored.Status.FailureDomains).To(HaveLen(3))
		g.Expect(stored.Spec.AvailabilityDomains).To(HaveLen(1))
		g.Expect(stored.Spec.NetworkSpec.Vcn.ID).NotTo(BeNil())
		g.Expect(stored.Spec.NetworkSpec.Vcn.InternetGateway.Id).NotTo(BeNil())
		g.Expect(stored.Spec.NetworkSpec.Vcn.NATGateway.Id).NotTo(BeNil())
		g.Expect(stored.Spec.NetworkSpec.Vcn.ServiceGateway.Id).NotTo(BeNil())
		g.Expect(stored.Spec.NetworkSpec.Vcn.RouteTable.PrivateRouteTableId).NotTo(BeNil())
		g.Expect(stored.Spec.NetworkSpec.Vcn.RouteTable.PublicRouteTableId).NotTo(BeNil())
		g.Expect(stored.Spec.NetworkSpec.APIServerLB.LoadBalancerId).NotTo(BeNil())
		for _, subnet := range stored.Spec.NetworkSpec.Vcn.Subnets {
			g.Expect(subnet.ID).NotTo(BeNil())
		}
		for _, nsg := range stored.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
			g.Expect(nsg.ID).NotTo(BeNil())
		}

		network := fakeOCI.vcn.networkCounts()
		g.Expect(network.VCNs).To(Equal(1))
		g.Expect(network.InternetGateways).To(Equal(1))
		g.Expect(network.NATGateways).To(Equal(1))
		g.Expect(network.ServiceGateways).To(Equal(1))
		g.Expect(network.NSGs).To(Equal(4))
		g.Expect(network.RouteTables).To(Equal(2))
		g.Expect(network.Subnets).To(Equal(4))
		activeNLBs, createdNLBs, _ := fakeOCI.nlb.counts()
		g.Expect(activeNLBs).To(Equal(1))
		g.Expect(createdNLBs).To(Equal(1))
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	createdResources := fakeOCI.vcn.networkCounts().Creates
	regionsBeforeUpdate, _, _ := fakeOCI.identity.counts()
	storedCluster = &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(true)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())
	g.Eventually(func(g Gomega) {
		pausedCluster := &clusterv1.Cluster{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), pausedCluster)).To(Succeed())
		g.Expect(pausedCluster.Spec.Paused).NotTo(BeNil())
		g.Expect(*pausedCluster.Spec.Paused).To(BeTrue())
	}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	storedCluster = &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())
	g.Eventually(func() int {
		regions, _, _ := fakeOCI.identity.counts()
		return regions
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).
		Should(BeNumerically(">", regionsBeforeUpdate))
	g.Expect(fakeOCI.vcn.networkCounts().Creates).To(Equal(createdResources))
	_, createdNLBs, _ := fakeOCI.nlb.counts()
	g.Expect(createdNLBs).To(Equal(1))

	g.Expect(testEnvironment.Delete(testContext, ociCluster)).To(Succeed())
	g.Eventually(func(g Gomega) {
		network := fakeOCI.vcn.networkCounts()
		g.Expect(network.VCNs).To(BeZero())
		g.Expect(network.InternetGateways).To(BeZero())
		g.Expect(network.NATGateways).To(BeZero())
		g.Expect(network.ServiceGateways).To(BeZero())
		g.Expect(network.NSGs).To(BeZero())
		g.Expect(network.RouteTables).To(BeZero())
		g.Expect(network.Subnets).To(BeZero())
		activeNLBs, _, deletedNLBs := fakeOCI.nlb.counts()
		g.Expect(activeNLBs).To(BeZero())
		g.Expect(deletedNLBs).To(Equal(1))
		err := testEnvironment.GetAPIReader().Get(testContext, ociClusterKey, &infrastructurev1beta2.OCICluster{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected OCICluster deletion, got %v", err)
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Expect(fakeOCI.vcn.networkCounts().DeleteOrder).To(Equal([]string{
		"networksecuritygroup", "networksecuritygroup", "networksecuritygroup", "networksecuritygroup",
		"subnet", "subnet", "subnet", "subnet",
		"routetable", "routetable",
		"servicegateway", "natgateway", "internetgateway", "vcn",
	}))
}
