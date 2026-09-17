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
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestOCIManagedControlPlaneRecoversFromCloudFailuresAndAsyncStates(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fakeOCI.oke.setFailure(createClusterOperation, errors.New("injected OKE cluster create failure"))
	fakeOCI.oke.setClusterCreateState(oke.ClusterLifecycleStateCreating)

	fixture := createManagedControlPlaneFixture(t, "oke-recovery", "oke-recovery-integration")
	t.Cleanup(func() {
		for _, operation := range []fakeOKEOperation{createClusterOperation, createKubeconfigOperation, updateClusterOperation, deleteClusterOperation} {
			fakeOCI.oke.clearFailure(operation)
		}
	})
	unpauseCluster(t, fixture.cluster)
	triggerObjectReconcile(t, fixture.controlPlane)

	waitForOKEAttempt(t, createClusterOperation, 0)
	controlPlaneKey := client.ObjectKeyFromObject(fixture.controlPlane)
	g.Eventually(func(g Gomega) {
		active, creates, _, _, _ := fakeOCI.oke.counts()
		g.Expect(active).To(BeZero())
		g.Expect(creates).To(BeZero())
		stored := &infrastructurev1beta2.OCIManagedControlPlane{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, controlPlaneKey, stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrastructurev1beta2.ControlPlaneFinalizer)).To(BeTrue())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForWarningEvent(t, fixture.namespace.Name, fixture.controlPlane.Name, "ReconcileError", "injected OKE cluster create failure")

	fakeOCI.oke.clearFailure(createClusterOperation)
	triggerObjectReconcile(t, fixture.controlPlane)
	var clusterID string
	g.Eventually(func(g Gomega) {
		active, creates, _, _, kubeconfigs := fakeOCI.oke.counts()
		g.Expect(active).To(Equal(1))
		g.Expect(creates).To(Equal(1))
		g.Expect(kubeconfigs).To(BeZero())
		stored := &infrastructurev1beta2.OCIManagedControlPlane{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, controlPlaneKey, stored)).To(Succeed())
		g.Expect(stored.Spec.ID).NotTo(BeNil())
		clusterID = *stored.Spec.ID
		g.Expect(stored.Status.Ready).To(BeFalse())
		condition := v1beta1conditions.Get(stored, infrastructurev1beta2.ControlPlaneReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(condition.Reason).To(Equal(infrastructurev1beta2.ControlPlaneNotReadyReason))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	fakeOCI.oke.setFailure(createKubeconfigOperation, errors.New("injected kubeconfig create failure"))
	fakeOCI.oke.setClusterState(clusterID, oke.ClusterLifecycleStateActive)
	triggerObjectReconcile(t, fixture.controlPlane)
	waitForOKEAttempt(t, createKubeconfigOperation, 0)
	g.Consistently(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKey{Name: fixture.cluster.Name + "-kubeconfig", Namespace: fixture.namespace.Name}, &corev1.Secret{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(300 * time.Millisecond).WithPolling(50 * time.Millisecond).Should(BeTrue())

	fakeOCI.oke.clearFailure(createKubeconfigOperation)
	triggerObjectReconcile(t, fixture.controlPlane)
	waitForManagedControlPlaneReady(t, fixture, clusterID, 1)

	updateAttempts := fakeOCI.oke.operationAttemptCount(updateClusterOperation)
	fakeOCI.oke.setFailure(updateClusterOperation, errors.New("injected OKE cluster update failure"))
	fakeOCI.oke.setClusterUpdateState(oke.ClusterLifecycleStateUpdating)
	updatedVersion := "v1.34.2"
	g.Eventually(func() error {
		stored := &infrastructurev1beta2.OCIManagedControlPlane{}
		if err := testEnvironment.GetAPIReader().Get(testContext, controlPlaneKey, stored); err != nil {
			return err
		}
		stored.Spec.Version = common.String(updatedVersion)
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForOKEAttempt(t, updateClusterOperation, updateAttempts)
	g.Eventually(func(g Gomega) {
		cluster, ok := fakeOCI.oke.cluster(clusterID)
		g.Expect(ok).To(BeTrue())
		g.Expect(cluster.KubernetesVersion).To(Equal(common.String(fixture.version)))
		_, creates, updates, _, _ := fakeOCI.oke.counts()
		g.Expect(creates).To(Equal(1))
		g.Expect(updates).To(BeZero())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	fakeOCI.oke.clearFailure(updateClusterOperation)
	triggerObjectReconcile(t, fixture.controlPlane)
	g.Eventually(func(g Gomega) {
		cluster, ok := fakeOCI.oke.cluster(clusterID)
		g.Expect(ok).To(BeTrue())
		g.Expect(cluster.KubernetesVersion).To(Equal(common.String(updatedVersion)))
		g.Expect(cluster.LifecycleState).To(Equal(oke.ClusterLifecycleStateUpdating))
		_, _, updates, _, _ := fakeOCI.oke.counts()
		g.Expect(updates).To(Equal(1))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	fakeOCI.oke.setClusterState(clusterID, oke.ClusterLifecycleStateActive)
	triggerObjectReconcile(t, fixture.controlPlane)
	waitForManagedControlPlaneVersion(t, fixture.controlPlane, updatedVersion)

	deleteAttempts := fakeOCI.oke.operationAttemptCount(deleteClusterOperation)
	fakeOCI.oke.setFailure(deleteClusterOperation, errors.New("injected OKE cluster delete failure"))
	g.Expect(testEnvironment.Delete(testContext, fixture.controlPlane)).To(Succeed())
	waitForOKEAttempt(t, deleteClusterOperation, deleteAttempts)
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCIManagedControlPlane{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, controlPlaneKey, stored)).To(Succeed())
		g.Expect(stored.DeletionTimestamp.IsZero()).To(BeFalse())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrastructurev1beta2.ControlPlaneFinalizer)).To(BeTrue())
		active, _, _, deletes, _ := fakeOCI.oke.counts()
		g.Expect(active).To(Equal(1))
		g.Expect(deletes).To(BeZero())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	fakeOCI.oke.clearFailure(deleteClusterOperation)
	triggerObjectReconcile(t, fixture.controlPlane)
	g.Eventually(func() int {
		_, _, _, deletes, _ := fakeOCI.oke.counts()
		return deletes
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(1))
	triggerObjectReconcile(t, fixture.controlPlane)
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, controlPlaneKey, &infrastructurev1beta2.OCIManagedControlPlane{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
}

func TestOCIManagedControlPlaneAdoptsOnlyOwnedOKECluster(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fixture := createManagedControlPlaneFixture(t, "oke-adoption", "oke-adoption-integration")

	ownedID := "ocid1.cluster.oc1..adopted-owned"
	foreignID := "ocid1.cluster.oc1..adopted-foreign"
	foreign := adoptedOKECluster(fixture, foreignID, map[string]string{"owner": "another-cluster"})
	fakeOCI.oke.seedCluster(foreign)
	owned := adoptedOKECluster(fixture, ownedID, ociutil.BuildClusterTags(fixture.managedCluster.Spec.OCIResourceIdentifier))
	fakeOCI.oke.seedCluster(owned)

	unpauseCluster(t, fixture.cluster)
	triggerObjectReconcile(t, fixture.controlPlane)
	waitForManagedControlPlaneReady(t, fixture, ownedID, 0)
	foreignAfter, ok := fakeOCI.oke.cluster(foreignID)
	g.Expect(ok).To(BeTrue())
	g.Expect(foreignAfter.LifecycleState).To(Equal(oke.ClusterLifecycleStateActive))

	g.Expect(testEnvironment.Delete(testContext, fixture.controlPlane)).To(Succeed())
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.controlPlane), &infrastructurev1beta2.OCIManagedControlPlane{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
	active, creates, _, deletes, _ := fakeOCI.oke.counts()
	g.Expect(active).To(Equal(1), "foreign OKE cluster must not be deleted")
	g.Expect(creates).To(BeZero())
	g.Expect(deletes).To(Equal(1))
}

type managedControlPlaneFixture struct {
	namespace      *corev1.Namespace
	cluster        *clusterv1.Cluster
	managedCluster *infrastructurev1beta2.OCIManagedCluster
	controlPlane   *infrastructurev1beta2.OCIManagedControlPlane
	version        string
}

func createManagedControlPlaneFixture(t *testing.T, name, resourceIdentifier string) *managedControlPlaneFixture {
	t.Helper()
	g := NewWithT(t)
	fixture := &managedControlPlaneFixture{version: "v1.34.1"}
	fixture.namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: name + "-"}}
	g.Expect(testEnvironment.Create(testContext, fixture.namespace)).To(Succeed())
	fixture.cluster = &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: fixture.namespace.Name},
		Spec: clusterv1.ClusterSpec{
			Paused:            common.Bool(true),
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{APIGroup: infrastructurev1beta2.GroupVersion.Group, Kind: scope.OCIManagedClusterKind, Name: name},
			ControlPlaneRef:   clusterv1.ContractVersionedObjectReference{APIGroup: infrastructurev1beta2.GroupVersion.Group, Kind: "OCIManagedControlPlane", Name: name},
		},
	}
	g.Expect(testEnvironment.Create(testContext, fixture.cluster)).To(Succeed())
	network := managedIntegrationNetworkSpec()
	network.Vcn.ID = common.String("ocid1.vcn.oc1.." + name)
	for i, subnet := range network.Vcn.Subnets {
		subnet.ID = common.String("ocid1.subnet.oc1.." + name + "-" + strconv.Itoa(i+1))
	}
	for i, nsg := range network.Vcn.NetworkSecurityGroup.List {
		nsg.ID = common.String("ocid1.networksecuritygroup.oc1.." + name + "-" + strconv.Itoa(i+1))
	}
	fixture.managedCluster = &infrastructurev1beta2.OCIManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: fixture.namespace.Name,
			Annotations:     map[string]string{clusterv1.ManagedByAnnotation: "integration-test"},
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(fixture.cluster, clusterv1.GroupVersion.WithKind("Cluster"))},
		},
		Spec: infrastructurev1beta2.OCIManagedClusterSpec{
			OCIResourceIdentifier: resourceIdentifier,
			CompartmentId:         "ocid1.compartment.oc1..integration",
			Region:                scope.MockTestRegion,
			NetworkSpec:           network,
		},
	}
	g.Expect(testEnvironment.Create(testContext, fixture.managedCluster)).To(Succeed())
	storedManagedCluster := &infrastructurev1beta2.OCIManagedCluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.managedCluster), storedManagedCluster)).To(Succeed())
	storedManagedCluster.Status.Ready = true
	g.Expect(testEnvironment.Status().Update(testContext, storedManagedCluster)).To(Succeed())
	fixture.controlPlane = &infrastructurev1beta2.OCIManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: fixture.namespace.Name,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(fixture.cluster, clusterv1.GroupVersion.WithKind("Cluster"))},
		},
		Spec: infrastructurev1beta2.OCIManagedControlPlaneSpec{Version: common.String(fixture.version)},
	}
	g.Expect(testEnvironment.Create(testContext, fixture.controlPlane)).To(Succeed())
	t.Cleanup(func() {
		for _, object := range []client.Object{
			fixture.controlPlane,
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-kubeconfig", Namespace: fixture.namespace.Name}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-self-managed", Namespace: fixture.namespace.Name}},
			fixture.managedCluster, fixture.cluster, fixture.namespace,
		} {
			deleteAndIgnoreNotFound(t, object)
		}
	})
	return fixture
}

func adoptedOKECluster(fixture *managedControlPlaneFixture, id string, tags map[string]string) oke.Cluster {
	return oke.Cluster{
		Id:                common.String(id),
		Name:              common.String(fixture.controlPlane.Name),
		CompartmentId:     common.String(fixture.managedCluster.Spec.CompartmentId),
		VcnId:             fixture.managedCluster.Spec.NetworkSpec.Vcn.ID,
		KubernetesVersion: common.String(fixture.version),
		FreeformTags:      cloneStringMap(tags),
		DefinedTags:       map[string]map[string]interface{}{},
		EndpointConfig: &oke.ClusterEndpointConfig{
			SubnetId: fixture.managedCluster.Spec.NetworkSpec.Vcn.Subnets[0].ID,
			NsgIds:   []string{stringValue(fixture.managedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List[0].ID)},
		},
		Options:           normalizedOKEOptions(nil),
		LifecycleState:    oke.ClusterLifecycleStateActive,
		Endpoints:         &oke.ClusterEndpoints{PublicEndpoint: common.String("198.51.100.10"), PrivateEndpoint: common.String("10.0.0.2:6443")},
		ImagePolicyConfig: &oke.ImagePolicyConfig{IsPolicyEnabled: common.Bool(false), KeyDetails: []oke.KeyDetails{}},
		Type:              oke.ClusterTypeBasicCluster,
	}
}

func waitForManagedControlPlaneReady(t *testing.T, fixture *managedControlPlaneFixture, clusterID string, creates int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCIManagedControlPlane{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.controlPlane), stored)).To(Succeed())
		g.Expect(stored.Spec.ID).To(Equal(common.String(clusterID)))
		g.Expect(stored.Status.Ready).To(BeTrue())
		g.Expect(stored.Status.Initialized).To(BeTrue())
		g.Expect(v1beta1conditions.IsTrue(stored, infrastructurev1beta2.ControlPlaneReadyCondition)).To(BeTrue())
		active, createCalls, _, _, kubeconfigs := fakeOCI.oke.counts()
		g.Expect(active).To(BeNumerically(">=", 1))
		g.Expect(createCalls).To(Equal(creates))
		g.Expect(kubeconfigs).To(BeNumerically(">=", 2))
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKey{Name: fixture.cluster.Name + "-kubeconfig", Namespace: fixture.namespace.Name}, &corev1.Secret{})).To(Succeed())
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func waitForManagedControlPlaneVersion(t *testing.T, controlPlane *infrastructurev1beta2.OCIManagedControlPlane, version string) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCIManagedControlPlane{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(controlPlane), stored)).To(Succeed())
		g.Expect(stored.Status.Version).To(Equal(common.String(version)))
		g.Expect(stored.Status.Ready).To(BeTrue())
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}
