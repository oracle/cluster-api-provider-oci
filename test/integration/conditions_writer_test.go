//go:build integration

/*
Copyright (c) 2026 Oracle and/or its affiliates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration_test

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	infrav1 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	cloudutil "github.com/oracle/cluster-api-provider-oci/cloud/util"
	"github.com/oracle/cluster-api-provider-oci/controllers"
	expinfrav1 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	legacy "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	standard "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func expectModernHealth(g Gomega, object interface {
	client.Object
	standard.Setter
}, conditionType string, status metav1.ConditionStatus) {
	condition := standard.Get(object, conditionType)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(status), "condition: %#v", condition)
	g.Expect(condition.Reason).NotTo(BeEmpty())
	g.Expect(condition.ObservedGeneration).To(BeNumerically("<=", object.GetGeneration()))
}

func TestConditionPatchPreservesConcurrentState(t *testing.T) {
	g := NewWithT(t)
	c, err := client.New(testEnvironment.Config, client.Options{Scheme: integrationScheme()})
	g.Expect(err).NotTo(HaveOccurred())
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "conditions-ownership-"}}
	g.Expect(c.Create(testContext, namespace)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, namespace) })
	object := &infrav1.OCIMachine{ObjectMeta: metav1.ObjectMeta{
		Name: "machine", Namespace: namespace.Name,
	}}
	g.Expect(c.Create(testContext, object)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, object) })
	g.Expect(c.Get(testContext, client.ObjectKeyFromObject(object), object)).To(Succeed())
	helper, err := v1beta1patch.NewHelper(object, c)
	g.Expect(err).NotTo(HaveOccurred())
	concurrent := object.DeepCopy()
	beforeSpec := concurrent.DeepCopy()
	concurrent.Spec.ProviderID = ptr.To("oci://current-instance")
	g.Expect(c.Patch(testContext, concurrent, client.MergeFrom(beforeSpec))).To(Succeed())
	g.Expect(c.Get(testContext, client.ObjectKeyFromObject(object), concurrent)).To(Succeed())
	beforeStatus := concurrent.DeepCopy()
	concurrent.Status.Ready = true
	legacy.MarkFalse(concurrent, "OtherLegacyCondition", "External", clusterv1beta1.ConditionSeverityWarning, "other writer")
	standard.Set(concurrent, metav1.Condition{
		Type:   "OtherControllerReady",
		Status: metav1.ConditionFalse,
		Reason: "External",
	})
	g.Expect(c.Status().Patch(testContext, concurrent, client.MergeFrom(beforeStatus))).To(Succeed())
	legacy.MarkTrue(object, infrav1.InstanceReadyCondition)
	standard.Set(object, metav1.Condition{
		Type:   string(infrav1.InstanceReadyCondition),
		Status: metav1.ConditionTrue,
		Reason: "Ready",
	})
	standard.Set(object, metav1.Condition{
		Type:   "Deleting",
		Status: metav1.ConditionFalse,
		Reason: "NotDeleting",
	})
	componentConditions := []string{string(infrav1.InstanceReadyCondition)}
	g.Expect(standard.SetSummaryCondition(object, object, "Ready", standard.ForConditionTypes(componentConditions))).To(Succeed())
	g.Expect(helper.Patch(testContext, object, v1beta1patch.WithOwnedConditions{
		Conditions: []clusterv1beta1.ConditionType{clusterv1beta1.ReadyCondition, clusterv1beta1.ConditionType("InstanceReady")},
	}, v1beta1patch.WithOwnedV1Beta2Conditions{
		Conditions: []string{"Ready", "Deleting", "InstanceReady"},
	})).To(Succeed())
	g.Eventually(func(g Gomega) {
		stored := &infrav1.OCIMachine{}
		g.Expect(c.Get(testContext, client.ObjectKeyFromObject(object), stored)).To(Succeed())
		g.Expect(stored.Spec.ProviderID).To(Equal(ptr.To("oci://current-instance")))
		g.Expect(stored.Status.Ready).To(BeTrue())
		g.Expect(legacy.Get(stored, "OtherLegacyCondition")).NotTo(BeNil())
		g.Expect(legacy.IsTrue(stored, infrav1.InstanceReadyCondition)).To(BeTrue())
		g.Expect(standard.IsFalse(stored, "OtherControllerReady")).To(BeTrue())
		expectModernHealth(g, stored, "Ready", metav1.ConditionTrue)
		g.Expect(standard.Get(stored, "Ready").ObservedGeneration).To(Equal(object.Generation))
	}).WithTimeout(10 * time.Second).WithPolling(50 * time.Millisecond).Should(Succeed())
}

func TestPoolMemberProducerPersistsObservedReadiness(t *testing.T) {
	g := NewWithT(t)
	c, err := client.New(testEnvironment.Config, client.Options{Scheme: integrationScheme()})
	g.Expect(err).NotTo(HaveOccurred())
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "member-observations-"}}
	g.Expect(c.Create(testContext, namespace)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, namespace) })
	pool := &expinfrav1.OCIMachinePool{ObjectMeta: metav1.ObjectMeta{Name: "parent", Namespace: namespace.Name}}
	g.Expect(c.Create(testContext, pool)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, pool) })
	logger := klogr.New()
	params := cloudutil.MachineParams{
		Client: c, Cluster: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}},
		MachinePool:      &clusterv1.MachinePool{ObjectMeta: metav1.ObjectMeta{Name: "workers"}},
		InfraMachinePool: pool, InfraMachinePoolName: pool.Name, InfraMachinePoolKind: "OCIMachinePool",
		InfraMachinePoolUID: pool.UID, Namespace: namespace.Name, Logger: &logger,
		SpecInfraMachines: []expinfrav1.OCIMachinePoolMachine{{
			Spec: expinfrav1.OCIMachinePoolMachineSpec{OCID: ptr.To("ocid1.instance.oc1..observed"), ProviderID: ptr.To("oci://observed"), InstanceName: ptr.To("observed")},
		}},
	}
	g.Expect(cloudutil.CreateMachinePoolMachinesIfNotExists(testContext, params)).To(Succeed())
	list := &expinfrav1.OCIMachinePoolMachineList{}
	g.Expect(c.List(testContext, list, client.InNamespace(namespace.Name))).To(Succeed())
	g.Expect(list.Items).To(HaveLen(1))
	member := list.Items[0].DeepCopy()
	uid := member.UID
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, member) })
	g.Expect(member.Status.Ready).To(BeFalse())
	expectModernHealth(g, member, "Ready", metav1.ConditionFalse)
	for _, ready := range []bool{true, false} {
		params.SpecInfraMachines[0].Status.Ready = ready
		g.Expect(cloudutil.CreateMachinePoolMachinesIfNotExists(testContext, params)).To(Succeed())
		g.Expect(c.Get(testContext, client.ObjectKeyFromObject(member), member)).To(Succeed())
		// Legacy readiness records completed provisioning; the condition tracks current health.
		g.Expect(member.Status.Ready).To(BeTrue())
		g.Expect(member.UID).To(Equal(uid))
		g.Expect(member.Spec.ProviderID).To(Equal(ptr.To("oci://observed")))
		status := metav1.ConditionFalse
		if ready {
			status = metav1.ConditionTrue
		}
		expectModernHealth(g, member, "Ready", status)
	}

	// An individually paused member must retain its health even when the parent observes a change.
	readyBeforePause := standard.Get(member, "Ready").DeepCopy()
	beforePause := member.DeepCopy()
	member.Annotations = map[string]string{clusterv1.PausedAnnotation: ""}
	g.Expect(c.Patch(testContext, member, client.MergeFrom(beforePause))).To(Succeed())
	params.SpecInfraMachines[0].Status.Ready = true
	g.Expect(cloudutil.CreateMachinePoolMachinesIfNotExists(testContext, params)).To(Succeed())
	g.Expect(c.Get(testContext, client.ObjectKeyFromObject(member), member)).To(Succeed())
	g.Expect(member.Status.Ready).To(BeTrue())
	g.Expect(standard.Get(member, "Ready")).To(Equal(readyBeforePause))

	beforeResume := member.DeepCopy()
	delete(member.Annotations, clusterv1.PausedAnnotation)
	g.Expect(c.Patch(testContext, member, client.MergeFrom(beforeResume))).To(Succeed())
	g.Expect(cloudutil.CreateMachinePoolMachinesIfNotExists(testContext, params)).To(Succeed())
	g.Expect(c.Get(testContext, client.ObjectKeyFromObject(member), member)).To(Succeed())
	g.Expect(member.Status.Ready).To(BeTrue())
	expectModernHealth(g, member, "Ready", metav1.ConditionTrue)
	params.SpecInfraMachines[0].Status.Ready = false
	g.Expect(cloudutil.CreateMachinePoolMachinesIfNotExists(testContext, params)).To(Succeed())

	params.Cluster.Spec.Paused = ptr.To(true)
	params.SpecInfraMachines[0].Status.Ready = true
	g.Expect(cloudutil.CreateMachinePoolMachinesIfNotExists(testContext, params)).To(Succeed())
	g.Expect(c.Get(testContext, client.ObjectKeyFromObject(member), member)).To(Succeed())
	g.Expect(member.Status.Ready).To(BeTrue())
	expectModernHealth(g, member, "Ready", metav1.ConditionFalse)
	params.Cluster.Spec.Paused = ptr.To(false)
	pool.Annotations = map[string]string{"cluster.x-k8s.io/paused": "true"}
	g.Expect(cloudutil.CreateMachinePoolMachinesIfNotExists(testContext, params)).To(Succeed())
	g.Expect(c.Get(testContext, client.ObjectKeyFromObject(member), member)).To(Succeed())
	g.Expect(member.Status.Ready).To(BeTrue())
	expectModernHealth(g, member, "Ready", metav1.ConditionFalse)
	params.SpecInfraMachines = nil
	g.Expect(cloudutil.DeleteOrphanedMachinePoolMachines(testContext, params)).To(Succeed())
	g.Expect(c.Get(testContext, client.ObjectKeyFromObject(member), member)).To(Succeed())
}

func TestPoolMemberPauseResumeWatch(t *testing.T) {
	g := NewWithT(t)
	c, err := client.New(testEnvironment.Config, client.Options{Scheme: integrationScheme()})
	g.Expect(err).NotTo(HaveOccurred())
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "member-pause-"}}
	g.Expect(c.Create(testContext, namespace)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, namespace) })
	cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: namespace.Name}, Spec: clusterv1.ClusterSpec{Paused: ptr.To(true)}}
	g.Expect(c.Create(testContext, cluster)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, cluster) })
	member := &expinfrav1.OCIMachinePoolMachine{ObjectMeta: metav1.ObjectMeta{
		Name: "member", Namespace: namespace.Name,
		Labels:     map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
		Finalizers: []string{expinfrav1.MachinePoolMachineFinalizer},
	}}
	g.Expect(c.Create(testContext, member)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, member) })
	key := client.ObjectKeyFromObject(member)
	// Wait for the controller cache to observe the Cluster before deleting its paused member.
	g.Eventually(func() error {
		return testEnvironment.Get(testContext, client.ObjectKeyFromObject(cluster), &clusterv1.Cluster{})
	}).WithTimeout(10 * time.Second).WithPolling(50 * time.Millisecond).Should(Succeed())
	g.Expect(c.Delete(testContext, member)).To(Succeed())
	g.Consistently(func() bool {
		stored := &expinfrav1.OCIMachinePoolMachine{}
		return c.Get(testContext, key, stored) == nil && len(stored.Finalizers) == 1
	}).WithTimeout(time.Second).WithPolling(50 * time.Millisecond).Should(BeTrue())
	g.Expect(c.Get(testContext, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
	cluster.Spec.Paused = ptr.To(false)
	g.Expect(c.Update(testContext, cluster)).To(Succeed())
	g.Eventually(func() bool {
		return apierrors.IsNotFound(c.Get(testContext, key, &expinfrav1.OCIMachinePoolMachine{}))
	}).WithTimeout(10 * time.Second).WithPolling(50 * time.Millisecond).Should(BeTrue())
}

// setLegacyReady updates only the compatibility bit, preserving controller-owned conditions.
func setLegacyReady(t *testing.T, object client.Object, ready bool) {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{"status": map[string]interface{}{"ready": ready}})
	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(testEnvironment.Status().Patch(testContext, object, client.RawPatch(types.MergePatchType, data))).To(Succeed())
}

func TestExternallyManagedClustersDoNotPublishConditions(t *testing.T) {
	g := NewWithT(t)
	c, err := client.New(testEnvironment.Config, client.Options{Scheme: integrationScheme()})
	g.Expect(err).NotTo(HaveOccurred())
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "externally-managed-"}}
	g.Expect(c.Create(testContext, namespace)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, namespace) })
	object := &infrav1.OCICluster{ObjectMeta: metav1.ObjectMeta{GenerateName: "external-", Namespace: namespace.Name}, Spec: infrav1.OCIClusterSpec{CompartmentId: "ocid1.compartment.oc1..external"}}
	object.Annotations = map[string]string{"cluster.x-k8s.io/managed-by": "external"}
	g.Expect(c.Create(testContext, object)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, object) })
	reconciler := &controllers.OCIClusterReconciler{Client: c}
	_, err = reconciler.Reconcile(testContext, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(object)})
	g.Expect(err).NotTo(HaveOccurred())
	g.Consistently(func(g Gomega) {
		stored := &infrav1.OCICluster{}
		g.Expect(c.Get(testContext, client.ObjectKeyFromObject(object), stored)).To(Succeed())
		g.Expect(stored.Status.V1Beta2).To(BeNil())
		g.Expect(stored.Finalizers).To(BeEmpty())
	}).WithTimeout(300 * time.Millisecond).WithPolling(50 * time.Millisecond).Should(Succeed())
}

func TestMissingMachinePrerequisitesDoNotPublishConditions(t *testing.T) {
	g := NewWithT(t)
	c, err := client.New(testEnvironment.Config, client.Options{Scheme: integrationScheme()})
	g.Expect(err).NotTo(HaveOccurred())
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "conditions-prerequisites-"}}
	g.Expect(c.Create(testContext, namespace)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, namespace) })

	owner := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: namespace.Name},
		Spec: clusterv1.MachineSpec{
			ClusterName:       "missing-cluster",
			Bootstrap:         clusterv1.Bootstrap{DataSecretName: ptr.To("unused-bootstrap")},
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{APIGroup: infrav1.GroupVersion.Group, Kind: "OCIMachine", Name: "missing-cluster"},
		},
	}
	g.Expect(c.Create(testContext, owner)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, owner) })
	objects := []*infrav1.OCIMachine{
		{ObjectMeta: metav1.ObjectMeta{Name: "missing-owner", Namespace: namespace.Name}},
		{ObjectMeta: metav1.ObjectMeta{
			Name: "missing-cluster", Namespace: namespace.Name,
			Labels:          map[string]string{clusterv1.ClusterNameLabel: "missing-cluster"},
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(owner, clusterv1.GroupVersion.WithKind("Machine"))},
		}},
	}
	reconciler := &controllers.OCIMachineReconciler{Client: c, Recorder: record.NewFakeRecorder(10)}
	for _, object := range objects {
		g.Expect(c.Create(testContext, object)).To(Succeed())
		t.Cleanup(func() { deleteAndIgnoreNotFound(t, object) })
		_, err := reconciler.Reconcile(testContext, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(object)})
		g.Expect(err).NotTo(HaveOccurred())
	}
	g.Consistently(func(g Gomega) {
		for _, object := range objects {
			stored := &infrav1.OCIMachine{}
			g.Expect(c.Get(testContext, client.ObjectKeyFromObject(object), stored)).To(Succeed())
			g.Expect(stored.Status.V1Beta2).To(BeNil())
			g.Expect(stored.Status.Conditions).To(BeEmpty())
			g.Expect(stored.Finalizers).To(BeEmpty())
		}
	}).WithTimeout(time.Second).WithPolling(50 * time.Millisecond).Should(Succeed())
}

func TestPausedMachineEventsAreFilteredBeforeOwnerLookup(t *testing.T) {
	g := NewWithT(t)
	c, err := client.New(testEnvironment.Config, client.Options{Scheme: integrationScheme()})
	g.Expect(err).NotTo(HaveOccurred())
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "paused-machine-events-"}}
	g.Expect(c.Create(testContext, namespace)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, namespace) })
	machine := &infrav1.OCIMachine{ObjectMeta: metav1.ObjectMeta{
		Name: "paused", Namespace: namespace.Name,
		Annotations: map[string]string{clusterv1.PausedAnnotation: ""},
	}}
	g.Expect(c.Create(testContext, machine)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, machine) })

	hasOwnerLookupEvent := func() (bool, error) {
		events := &corev1.EventList{}
		if err := c.List(testContext, events, client.InNamespace(namespace.Name)); err != nil {
			return false, err
		}
		for _, event := range events.Items {
			if event.InvolvedObject.UID == machine.UID && event.Reason == "OwnerRefNotSet" {
				return true, nil
			}
		}
		return false, nil
	}
	g.Consistently(hasOwnerLookupEvent).WithTimeout(time.Second).WithPolling(50 * time.Millisecond).Should(BeFalse())

	before := machine.DeepCopy()
	delete(machine.Annotations, clusterv1.PausedAnnotation)
	g.Expect(c.Patch(testContext, machine, client.MergeFrom(before))).To(Succeed())
	g.Eventually(hasOwnerLookupEvent).WithTimeout(10 * time.Second).WithPolling(50 * time.Millisecond).Should(BeTrue())
}
