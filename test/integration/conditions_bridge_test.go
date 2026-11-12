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
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestConditionsBridgeAPI(t *testing.T) {
	g := NewWithT(t)
	apiClient, err := client.New(testEnvironment.Config, client.Options{Scheme: integrationScheme()})
	g.Expect(err).NotTo(HaveOccurred())
	crdClient, err := apiextensionsclient.NewForConfig(testEnvironment.Config)
	g.Expect(err).NotTo(HaveOccurred())

	kinds := []string{
		"OCICluster", "OCIManagedCluster", "OCIMachine", "OCIManagedControlPlane",
		"OCIMachinePool", "OCIManagedMachinePool", "OCIVirtualMachinePool", "OCIMachinePoolMachine",
	}
	for _, kind := range kinds {
		name := strings.ToLower(kind) + "s." + infrastructurev1beta2.GroupVersion.Group
		crd, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(testContext, name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		// Envtest wires the manager's conversion webhook for convertible types.
		// With v1beta2 storage, every v1beta1 request exercises that webhook.
		g.Expect(crd.Spec.Conversion).NotTo(BeNil())
		g.Expect(crd.Spec.Conversion.Strategy).To(Equal(apiextensionsv1.WebhookConverter))
		var storageVersions []string
		for _, version := range crd.Spec.Versions {
			if version.Storage {
				storageVersions = append(storageVersions, version.Name)
			}
		}
		g.Expect(storageVersions).To(Equal([]string{"v1beta2"}))
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "conditions-bridge-"}}
	g.Expect(apiClient.Create(testContext, namespace)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, namespace) })

	for i, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			g := NewWithT(t)
			object := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": infrastructurev1beta2.GroupVersion.Group + "/v1beta1",
				"kind":       kind,
				"metadata": map[string]interface{}{
					"name": fmt.Sprintf("bridge-%d", i), "namespace": namespace.Name,
					"annotations": map[string]interface{}{"cluster.x-k8s.io/paused": "true"},
				},
				"spec": map[string]interface{}{},
			}}
			if kind == "OCICluster" || kind == "OCIManagedCluster" {
				g.Expect(unstructured.SetNestedField(object.Object, "ocid1.compartment.oc1..conditions-bridge", "spec", "compartmentId")).To(Succeed())
			}
			if kind == "OCIManagedMachinePool" {
				g.Expect(unstructured.SetNestedField(object.Object, "v1.34.0", "spec", "version")).To(Succeed())
			}
			g.Expect(apiClient.Create(testContext, object)).To(Succeed())
			uid := object.GetUID()
			key := client.ObjectKeyFromObject(object)
			t.Cleanup(func() { deleteAndIgnoreNotFound(t, object) })

			g.Expect(apiClient.Get(testContext, key, object)).To(Succeed())

			legacy := []interface{}{map[string]interface{}{
				"type": "LegacyReady", "status": "False", "severity": "Warning",
				"lastTransitionTime": "2026-11-11T00:00:00Z", "reason": "LegacyReason", "message": "legacy diagnostics",
			}}
			modern := []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionTrue, ObservedGeneration: object.GetGeneration(),
				LastTransitionTime: metav1.NewTime(time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)), Reason: "Ready", Message: "infrastructure ready",
			}}
			data, err := json.Marshal(map[string]interface{}{
				"ready": true, "conditions": legacy, "v1beta2": map[string]interface{}{"conditions": modern},
			})
			g.Expect(err).NotTo(HaveOccurred())
			var status map[string]interface{}
			g.Expect(json.Unmarshal(data, &status)).To(Succeed())
			object.Object["status"] = status
			g.Expect(apiClient.Status().Update(testContext, object, client.FieldOwner("bridge-initial"))).To(Succeed())

			// Legacy metadata, spec and status writes must retain the modern condition list.
			object.SetLabels(map[string]string{"legacy-client-edit": "retained"})
			editTags := kind == "OCICluster" || kind == "OCIManagedCluster" || kind == "OCIMachine"
			if editTags {
				g.Expect(unstructured.SetNestedStringMap(object.Object, map[string]string{"bridge-edit": "retained"}, "spec", "freeformTags")).To(Succeed())
			}
			g.Expect(apiClient.Update(testContext, object)).To(Succeed())
			g.Expect(apiClient.Status().Patch(testContext, object,
				client.RawPatch(types.MergePatchType, []byte(`{"status":{"ready":false}}`)))).To(Succeed())

			// Separate SSA owners must merge conditions by type rather than replace the list.
			other := metav1.Condition{
				Type: "ProviderSpecificReady", Status: metav1.ConditionUnknown, ObservedGeneration: object.GetGeneration(),
				LastTransitionTime: modern[0].LastTransitionTime, Reason: "Observing", Message: "waiting for inventory",
			}
			data, err = json.Marshal(map[string]interface{}{
				"apiVersion": object.GetAPIVersion(), "kind": kind,
				"metadata": map[string]interface{}{"name": object.GetName(), "namespace": namespace.Name},
				"status":   map[string]interface{}{"v1beta2": map[string]interface{}{"conditions": []metav1.Condition{other}}},
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(apiClient.Status().Patch(testContext, object,
				client.RawPatch(types.ApplyPatchType, data), client.FieldOwner("bridge-other"))).To(Succeed())

			for _, version := range []string{"v1beta1", "v1beta2"} {
				stored := &unstructured.Unstructured{}
				stored.SetAPIVersion(infrastructurev1beta2.GroupVersion.Group + "/" + version)
				stored.SetKind(kind)
				g.Expect(apiClient.Get(testContext, key, stored)).To(Succeed())
				g.Expect(stored.GetUID()).To(Equal(uid))
				g.Expect(stored.GetLabels()).To(HaveKeyWithValue("legacy-client-edit", "retained"))
				if editTags {
					tags, _, err := unstructured.NestedStringMap(stored.Object, "spec", "freeformTags")
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(tags).To(HaveKeyWithValue("bridge-edit", "retained"))
				}
				ready, _, err := unstructured.NestedBool(stored.Object, "status", "ready")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeFalse())
				legacyConditions, _, err := unstructured.NestedSlice(stored.Object, "status", "conditions")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(legacyConditions).To(Equal(legacy))
				standardConditions, err := conditions.UnstructuredGetAll(stored)
				g.Expect(err).NotTo(HaveOccurred())
				for i := range standardConditions {
					standardConditions[i].LastTransitionTime = metav1.NewTime(standardConditions[i].LastTransitionTime.UTC())
				}
				g.Expect(standardConditions).To(ConsistOf(modern[0], other))
			}
		})
	}
}
