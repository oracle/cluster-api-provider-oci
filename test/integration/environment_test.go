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
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestManagerAndAdmissionWebhooks(t *testing.T) {
	g := NewWithT(t)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "capoci-integration-"},
	}
	g.Expect(testEnvironment.Create(testContext, namespace)).To(Succeed())

	controlPlane := &infrastructurev1beta2.OCIManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-control-plane",
			Namespace: namespace.Name,
		},
	}
	g.Expect(testEnvironment.Create(testContext, controlPlane)).To(Succeed())
	t.Cleanup(func() {
		deleteAndIgnoreNotFound(t, controlPlane)
		deleteAndIgnoreNotFound(t, namespace)
	})

	key := client.ObjectKeyFromObject(controlPlane)
	storedControlPlane := &infrastructurev1beta2.OCIManagedControlPlane{}
	g.Eventually(func(g Gomega) {
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, storedControlPlane)).To(Succeed())
		g.Expect(storedControlPlane.Spec.ClusterPodNetworkOptions).To(HaveLen(1))
		g.Expect(storedControlPlane.Spec.ClusterPodNetworkOptions[0].CniType).
			To(Equal(infrastructurev1beta2.VCNNativeCNI))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Eventually(func() bool {
		events := &corev1.EventList{}
		if err := testEnvironment.GetAPIReader().List(
			testContext,
			events,
			client.InNamespace(namespace.Name),
		); err != nil {
			return false
		}
		for _, event := range events.Items {
			if event.InvolvedObject.Name == controlPlane.Name && event.Reason == "OwnerRefNotSet" {
				return true
			}
		}
		return false
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())

	storedControlPlane.Status.Ready = true
	g.Expect(testEnvironment.Status().Update(testContext, storedControlPlane)).To(Succeed())
	g.Eventually(func(g Gomega) {
		updated := &infrastructurev1beta2.OCIManagedControlPlane{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, key, updated)).To(Succeed())
		g.Expect(updated.Status.Ready).To(BeTrue())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	invalidControlPlane := &infrastructurev1beta2.OCIManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Repeat("a", 32),
			Namespace: namespace.Name,
		},
	}
	err := testEnvironment.Create(testContext, invalidControlPlane)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected admission webhook rejection, got %v", err)
}

func deleteAndIgnoreNotFound(t *testing.T, object client.Object) {
	t.Helper()
	if err := testEnvironment.Delete(testContext, object); err != nil && !apierrors.IsNotFound(err) {
		t.Errorf("delete %T %s: %v", object, client.ObjectKeyFromObject(object), err)
		return
	}

	key := client.ObjectKeyFromObject(object)
	if _, ok := object.(*corev1.Namespace); ok {
		g := NewWithT(t)
		g.Eventually(func() bool {
			stored := &corev1.Namespace{}
			err := testEnvironment.GetAPIReader().Get(testContext, key, stored)
			if apierrors.IsNotFound(err) {
				return true
			}
			if err != nil {
				return false
			}
			if !stored.DeletionTimestamp.IsZero() && len(stored.Spec.Finalizers) > 0 {
				stored.Spec.Finalizers = nil
				if err := testEnvironment.SubResource("finalize").Update(testContext, stored); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
					return false
				}
			}
			return false
		}).WithTimeout(15*time.Second).WithPolling(50*time.Millisecond).
			Should(BeTrue(), "timed out waiting for namespace %s deletion", key)
		return
	}

	probe := object.DeepCopyObject().(client.Object)
	g := NewWithT(t)
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, key, probe)
		return apierrors.IsNotFound(err)
	}).WithTimeout(15*time.Second).WithPolling(50*time.Millisecond).
		Should(BeTrue(), "timed out waiting for %T %s deletion", object, key)
}
