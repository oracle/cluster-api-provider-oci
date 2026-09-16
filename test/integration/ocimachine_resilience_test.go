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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestOCIMachineRecoversFromCloudFailuresAndPendingState(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fakeOCI.compute.setFailure(launchInstanceOperation, errors.New("injected instance launch failure"))
	fakeOCI.compute.setLaunchLifecycleState(core.InstanceLifecycleStateProvisioning)

	fixture := createOCIMachineFixture(t, "machine-recovery", "machine-recovery-integration")
	t.Cleanup(func() {
		fakeOCI.compute.clearFailure(launchInstanceOperation)
		fakeOCI.compute.clearFailure(terminateInstanceOperation)
	})
	unpauseCluster(t, fixture.cluster)

	waitForComputeAttempt(t, launchInstanceOperation, 0)
	g.Eventually(func(g Gomega) {
		launches, _, _ := fakeOCI.compute.counts()
		g.Expect(launches).To(BeZero())
		stored := &infrastructurev1beta2.OCIMachine{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.ociMachine), stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrastructurev1beta2.MachineFinalizer)).To(BeTrue())
		condition := v1beta1conditions.Get(stored, infrastructurev1beta2.InstanceReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(condition.Reason).To(Equal(infrastructurev1beta2.InstanceProvisionFailedReason))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForWarningEvent(t, fixture.namespace.Name, fixture.ociMachine.Name, "ReconcileError", "injected instance launch failure")

	fakeOCI.compute.clearFailure(launchInstanceOperation)
	triggerObjectReconcile(t, fixture.ociMachine)
	var instanceID string
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCIMachine{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.ociMachine), stored)).To(Succeed())
		g.Expect(stored.Spec.InstanceId).NotTo(BeNil())
		instanceID = *stored.Spec.InstanceId
		g.Expect(stored.Status.Ready).To(BeFalse())
		condition := v1beta1conditions.Get(stored, infrastructurev1beta2.InstanceReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(condition.Reason).To(Equal(infrastructurev1beta2.InstanceNotReadyReason))
		launches, _, _ := fakeOCI.compute.counts()
		g.Expect(launches).To(Equal(1))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	fakeOCI.compute.setInstanceLifecycleState(instanceID, core.InstanceLifecycleStateRunning)
	triggerObjectReconcile(t, fixture.ociMachine)
	waitForOCIMachineReady(t, fixture.ociMachine, instanceID, 1)

	terminateAttempts := fakeOCI.compute.operationAttemptCount(terminateInstanceOperation)
	fakeOCI.compute.setFailure(terminateInstanceOperation, errors.New("injected instance terminate failure"))
	g.Expect(testEnvironment.Delete(testContext, fixture.ociMachine)).To(Succeed())
	waitForComputeAttempt(t, terminateInstanceOperation, terminateAttempts)
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCIMachine{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.ociMachine), stored)).To(Succeed())
		g.Expect(stored.DeletionTimestamp.IsZero()).To(BeFalse())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrastructurev1beta2.MachineFinalizer)).To(BeTrue())
		_, terminations, _ := fakeOCI.compute.counts()
		g.Expect(terminations).To(BeZero())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	fakeOCI.compute.clearFailure(terminateInstanceOperation)
	triggerObjectReconcile(t, fixture.ociMachine)
	g.Eventually(func() int {
		_, terminations, _ := fakeOCI.compute.counts()
		return terminations
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(1))
	triggerObjectReconcile(t, fixture.ociMachine)
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.ociMachine), &infrastructurev1beta2.OCIMachine{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
}

func TestOCIMachineAdoptsOnlyOwnedInstance(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()

	fixture := createOCIMachineFixture(t, "machine-adoption", "machine-adoption-integration")
	goodID := "ocid1.instance.oc1..adopted-owned"
	wrongID := "ocid1.instance.oc1..adopted-foreign"
	base := core.Instance{
		DisplayName:        common.String(fixture.ociMachine.Name),
		CompartmentId:      common.String(fixture.ociCluster.Spec.CompartmentId),
		AvailabilityDomain: common.String("integration-ad-1"),
		FaultDomain:        common.String("FAULT-DOMAIN-1"),
		LifecycleState:     core.InstanceLifecycleStateRunning,
	}
	foreign := base
	foreign.Id = common.String(wrongID)
	foreign.FreeformTags = map[string]string{"owner": "another-cluster"}
	fakeOCI.compute.seedInstance(foreign)
	owned := base
	owned.Id = common.String(goodID)
	owned.FreeformTags = ociutil.BuildClusterTags(fixture.ociCluster.Spec.OCIResourceIdentifier)
	fakeOCI.compute.seedInstance(owned)

	unpauseCluster(t, fixture.cluster)
	waitForOCIMachineReady(t, fixture.ociMachine, goodID, 0)
	foreignAfter, ok := fakeOCI.compute.instance(wrongID)
	g.Expect(ok).To(BeTrue())
	g.Expect(foreignAfter.LifecycleState).To(Equal(core.InstanceLifecycleStateRunning))

	g.Expect(testEnvironment.Delete(testContext, fixture.ociMachine)).To(Succeed())
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.ociMachine), &infrastructurev1beta2.OCIMachine{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
	foreignAfter, ok = fakeOCI.compute.instance(wrongID)
	g.Expect(ok).To(BeTrue())
	g.Expect(foreignAfter.LifecycleState).To(Equal(core.InstanceLifecycleStateRunning))
	ownedAfter, ok := fakeOCI.compute.instance(goodID)
	g.Expect(ok).To(BeTrue())
	g.Expect(ownedAfter.LifecycleState).To(Equal(core.InstanceLifecycleStateTerminated))
}

func TestOCIMachineDeletionSucceedsWhenInstanceIsAlreadyAbsent(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fixture := createOCIMachineFixture(t, "machine-externally-deleted", "machine-externally-deleted-integration")
	unpauseCluster(t, fixture.cluster)

	key := client.ObjectKeyFromObject(fixture.ociMachine)
	var instanceID string
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCIMachine{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, stored)).To(Succeed())
		g.Expect(stored.Spec.InstanceId).NotTo(BeNil())
		instanceID = *stored.Spec.InstanceId
		g.Expect(stored.Status.Ready).To(BeTrue())
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	fakeOCI.compute.removeInstance(instanceID)
	g.Expect(testEnvironment.Delete(testContext, fixture.ociMachine)).To(Succeed())
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, &infrastructurev1beta2.OCIMachine{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
	_, terminations, _ := fakeOCI.compute.counts()
	g.Expect(terminations).To(BeZero(), "an already absent instance must not be terminated again")
}

type ociMachineFixture struct {
	namespace       *corev1.Namespace
	cluster         *clusterv1.Cluster
	ociCluster      *infrastructurev1beta2.OCICluster
	bootstrapSecret *corev1.Secret
	machine         *clusterv1.Machine
	ociMachine      *infrastructurev1beta2.OCIMachine
}

func createOCIMachineFixture(t *testing.T, name, resourceIdentifier string) *ociMachineFixture {
	t.Helper()
	g := NewWithT(t)
	fixture := &ociMachineFixture{}
	fixture.namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: name + "-"}}
	g.Expect(testEnvironment.Create(testContext, fixture.namespace)).To(Succeed())
	fixture.cluster = &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: fixture.namespace.Name},
		Spec: clusterv1.ClusterSpec{
			Paused: common.Bool(true),
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     scope.OCIClusterKind,
				Name:     name,
			},
		},
	}
	g.Expect(testEnvironment.Create(testContext, fixture.cluster)).To(Succeed())
	fixture.ociCluster = &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   fixture.namespace.Name,
			Annotations: map[string]string{clusterv1.ManagedByAnnotation: "integration-test"},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(fixture.cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			OCIResourceIdentifier: resourceIdentifier,
			CompartmentId:         "ocid1.compartment.oc1..integration",
			Region:                scope.MockTestRegion,
		},
	}
	g.Expect(testEnvironment.Create(testContext, fixture.ociCluster)).To(Succeed())
	storedOCICluster := &infrastructurev1beta2.OCICluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(fixture.ociCluster), storedOCICluster)).To(Succeed())
	storedOCICluster.Status.FailureDomains = clusterv1beta1.FailureDomains{
		"1": {Attributes: map[string]string{scope.AvailabilityDomain: "integration-ad-1", scope.FaultDomain: "FAULT-DOMAIN-1"}},
	}
	g.Expect(testEnvironment.Status().Update(testContext, storedOCICluster)).To(Succeed())
	fixture.bootstrapSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-bootstrap", Namespace: fixture.namespace.Name},
		Data:       map[string][]byte{"value": []byte("#!/bin/sh\necho integration")},
	}
	g.Expect(testEnvironment.Create(testContext, fixture.bootstrapSecret)).To(Succeed())
	fixture.machine = &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: fixture.namespace.Name, Labels: map[string]string{clusterv1.ClusterNameLabel: name}},
		Spec: clusterv1.MachineSpec{
			ClusterName: name,
			Bootstrap:   clusterv1.Bootstrap{DataSecretName: common.String(fixture.bootstrapSecret.Name)},
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     scope.OCIMachineKind,
				Name:     name,
			},
			FailureDomain: "1",
		},
	}
	g.Expect(testEnvironment.Create(testContext, fixture.machine)).To(Succeed())
	fixture.ociMachine = &infrastructurev1beta2.OCIMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fixture.namespace.Name,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: name},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(fixture.machine, clusterv1.GroupVersion.WithKind("Machine")),
			},
		},
		Spec: infrastructurev1beta2.OCIMachineSpec{
			ImageId: "ocid1.image.oc1..integration",
			Shape:   "VM.Standard.E4.Flex",
			NetworkDetails: infrastructurev1beta2.NetworkDetails{
				SubnetId: common.String("ocid1.subnet.oc1..integration"),
			},
		},
	}
	g.Expect(testEnvironment.Create(testContext, fixture.ociMachine)).To(Succeed())
	t.Cleanup(func() {
		for _, object := range []client.Object{fixture.ociMachine, fixture.machine, fixture.bootstrapSecret, fixture.ociCluster, fixture.cluster, fixture.namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})
	return fixture
}

func unpauseCluster(t *testing.T, cluster *clusterv1.Cluster) {
	t.Helper()
	g := NewWithT(t)
	stored := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), stored)).To(Succeed())
	stored.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, stored)).To(Succeed())
}

func waitForOCIMachineReady(t *testing.T, machine *infrastructurev1beta2.OCIMachine, instanceID string, launches int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCIMachine{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(machine), stored)).To(Succeed())
		g.Expect(stored.Spec.InstanceId).To(Equal(common.String(instanceID)))
		g.Expect(stored.Status.Ready).To(BeTrue())
		g.Expect(v1beta1conditions.IsTrue(stored, infrastructurev1beta2.InstanceReadyCondition)).To(BeTrue())
		actualLaunches, _, _ := fakeOCI.compute.counts()
		g.Expect(actualLaunches).To(Equal(launches))
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}
