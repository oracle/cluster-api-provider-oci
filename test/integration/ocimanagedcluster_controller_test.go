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

func TestOCIManagedClusterControlPlaneLifecycle(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "oke-integration-"}}
	g.Expect(testEnvironment.Create(testContext, namespace)).To(Succeed())

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "managed-lifecycle", Namespace: namespace.Name},
		Spec: clusterv1.ClusterSpec{
			Paused: common.Bool(true),
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     scope.OCIManagedClusterKind,
				Name:     "managed-lifecycle",
			},
			ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     "OCIManagedControlPlane",
				Name:     "managed-lifecycle",
			},
		},
	}
	g.Expect(testEnvironment.Create(testContext, cluster)).To(Succeed())

	version := "v1.34.1"
	controlPlane := &infrastructurev1beta2.OCIManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: namespace.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: infrastructurev1beta2.OCIManagedControlPlaneSpec{Version: common.String(version)},
	}
	g.Expect(testEnvironment.Create(testContext, controlPlane)).To(Succeed())

	managedCluster := &infrastructurev1beta2.OCIManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: namespace.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: infrastructurev1beta2.OCIManagedClusterSpec{
			CompartmentId: "ocid1.compartment.oc1..integration",
			Region:        scope.MockTestRegion,
			NetworkSpec:   managedIntegrationNetworkSpec(),
		},
	}
	g.Expect(testEnvironment.Create(testContext, managedCluster)).To(Succeed())

	kubeconfigSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cluster.Name + "-kubeconfig", Namespace: namespace.Name}}
	bootstrapSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cluster.Name + "-self-managed", Namespace: namespace.Name}}
	t.Cleanup(func() {
		for _, object := range []client.Object{
			controlPlane,
			managedCluster,
			kubeconfigSecret,
			bootstrapSecret,
			cluster,
			namespace,
		} {
			deleteAndIgnoreNotFound(t, object)
		}
	})

	g.Consistently(func() int {
		_, creates, _, _, _ := fakeOCI.oke.counts()
		return creates
	}).WithTimeout(time.Second).WithPolling(100 * time.Millisecond).Should(BeZero())

	storedCluster := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())

	managedClusterKey := client.ObjectKeyFromObject(managedCluster)
	controlPlaneKey := client.ObjectKeyFromObject(controlPlane)
	g.Eventually(func(g Gomega) {
		storedManagedCluster := &infrastructurev1beta2.OCIManagedCluster{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, managedClusterKey, storedManagedCluster)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(storedManagedCluster, infrastructurev1beta2.ManagedClusterFinalizer)).To(BeTrue())
		g.Expect(storedManagedCluster.Status.Ready).To(BeTrue())
		g.Expect(v1beta1conditions.IsTrue(storedManagedCluster, infrastructurev1beta2.ClusterReadyCondition)).To(BeTrue())
		g.Expect(storedManagedCluster.Spec.ControlPlaneEndpoint.Host).To(Equal("198.51.100.10"))
		g.Expect(storedManagedCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(int32(6443)))

		storedControlPlane := &infrastructurev1beta2.OCIManagedControlPlane{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, controlPlaneKey, storedControlPlane)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(storedControlPlane, infrastructurev1beta2.ControlPlaneFinalizer)).To(BeTrue())
		g.Expect(storedControlPlane.Spec.ID).NotTo(BeNil())
		g.Expect(storedControlPlane.Spec.ControlPlaneEndpoint.Host).To(Equal("198.51.100.10"))
		g.Expect(storedControlPlane.Status.Ready).To(BeTrue())
		g.Expect(storedControlPlane.Status.Initialized).To(BeTrue())
		g.Expect(storedControlPlane.Status.Version).To(Equal(common.String(version)))
		g.Expect(v1beta1conditions.IsTrue(storedControlPlane, infrastructurev1beta2.ControlPlaneReadyCondition)).To(BeTrue())

		active, creates, updates, deletes, kubeconfigs := fakeOCI.oke.counts()
		g.Expect(active).To(Equal(1))
		g.Expect(creates).To(Equal(1))
		g.Expect(updates).To(BeZero())
		g.Expect(deletes).To(BeZero())
		g.Expect(kubeconfigs).To(Equal(2))
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(kubeconfigSecret), kubeconfigSecret)).To(Succeed())
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(bootstrapSecret), bootstrapSecret)).To(Succeed())
	}).WithTimeout(25 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	_, createsBefore, _, _, _ := fakeOCI.oke.counts()
	tokensBefore := fakeOCI.base.count()
	g.Consistently(fakeOCI.base.count).WithTimeout(300 * time.Millisecond).WithPolling(50 * time.Millisecond).
		Should(Equal(tokensBefore))
	storedCluster = &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(true)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())
	storedCluster = &clusterv1.Cluster{}
	g.Eventually(func(g Gomega) {
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
		g.Expect(storedCluster.Spec.Paused).NotTo(BeNil())
		g.Expect(*storedCluster.Spec.Paused).To(BeTrue())
	}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())
	g.Eventually(fakeOCI.base.count).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).
		Should(BeNumerically(">", tokensBefore))
	g.Consistently(func() int {
		_, creates, _, _, _ := fakeOCI.oke.counts()
		return creates
	}).WithTimeout(time.Second).WithPolling(100 * time.Millisecond).Should(Equal(createsBefore))
	_, _, updatesAfterReconcile, _, _ := fakeOCI.oke.counts()
	g.Expect(updatesAfterReconcile).To(BeZero())

	g.Expect(testEnvironment.Delete(testContext, controlPlane)).To(Succeed())
	g.Eventually(func(g Gomega) {
		active, _, _, deletes, _ := fakeOCI.oke.counts()
		g.Expect(active).To(BeZero())
		g.Expect(deletes).To(Equal(1))
		err := testEnvironment.GetAPIReader().Get(testContext, controlPlaneKey, &infrastructurev1beta2.OCIManagedControlPlane{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected managed control-plane deletion, got %v", err)
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Expect(testEnvironment.Delete(testContext, managedCluster)).To(Succeed())
	g.Eventually(func(g Gomega) {
		network := fakeOCI.vcn.networkCounts()
		g.Expect(network.VCNs).To(BeZero())
		g.Expect(network.InternetGateways).To(BeZero())
		g.Expect(network.NATGateways).To(BeZero())
		g.Expect(network.ServiceGateways).To(BeZero())
		g.Expect(network.NSGs).To(BeZero())
		g.Expect(network.RouteTables).To(BeZero())
		g.Expect(network.Subnets).To(BeZero())
		err := testEnvironment.GetAPIReader().Get(testContext, managedClusterKey, &infrastructurev1beta2.OCIManagedCluster{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected managed-cluster deletion, got %v", err)
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func managedIntegrationNetworkSpec() infrastructurev1beta2.NetworkSpec {
	return infrastructurev1beta2.NetworkSpec{
		Vcn: infrastructurev1beta2.VCN{
			CIDR: "10.0.0.0/16",
			Subnets: []*infrastructurev1beta2.Subnet{
				{Role: infrastructurev1beta2.ControlPlaneEndpointRole, Name: "control-plane-endpoint", CIDR: "10.0.0.0/28", Type: infrastructurev1beta2.Public},
				{Role: infrastructurev1beta2.ServiceLoadBalancerRole, Name: "service-lb", CIDR: "10.0.1.0/24", Type: infrastructurev1beta2.Public},
				{Role: infrastructurev1beta2.WorkerRole, Name: "worker", CIDR: "10.0.2.0/24", Type: infrastructurev1beta2.Private},
				{Role: infrastructurev1beta2.PodRole, Name: "pod", CIDR: "10.0.3.0/24", Type: infrastructurev1beta2.Private},
			},
			NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
				List: []*infrastructurev1beta2.NSG{
					{Role: infrastructurev1beta2.ControlPlaneEndpointRole, Name: "control-plane-endpoint"},
					{Role: infrastructurev1beta2.ServiceLoadBalancerRole, Name: "service-lb"},
					{Role: infrastructurev1beta2.WorkerRole, Name: "worker"},
					{Role: infrastructurev1beta2.PodRole, Name: "pod"},
				},
			},
		},
	}
}
