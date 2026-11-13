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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOCIMachinePoolShapeConfigValidation(t *testing.T) {
	g := NewWithT(t)
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "pool-shape-validation-"}}
	g.Expect(testEnvironment.Create(testContext, namespace)).To(Succeed())
	t.Cleanup(func() { deleteAndIgnoreNotFound(t, namespace) })

	for _, version := range []string{"v1beta1", "v1beta2"} {
		t.Run(version, func(t *testing.T) {
			newPool := func() *unstructured.Unstructured {
				pool := &unstructured.Unstructured{Object: map[string]interface{}{
					"spec": map[string]interface{}{"instanceConfiguration": map[string]interface{}{}},
				}}
				pool.SetGroupVersionKind(schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: version, Kind: "OCIMachinePool"})
				pool.SetNamespace(namespace.Name)
				pool.SetGenerateName("shape-")
				return pool
			}
			setShape := func(pool *unstructured.Unstructured, shape map[string]interface{}) {
				if shape == nil {
					unstructured.RemoveNestedField(pool.Object, "spec", "instanceConfiguration", "shapeConfig")
					return
				}
				NewWithT(t).Expect(unstructured.SetNestedMap(pool.Object, shape, "spec", "instanceConfiguration", "shapeConfig")).To(Succeed())
			}
			assertInvalid := func(t *testing.T, err error) {
				t.Helper()
				g := NewWithT(t)
				g.Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an invalid shapeConfig, got %v", err)
				status := err.(apierrors.APIStatus).Status()
				g.Expect(status.Details.Causes).To(ContainElement(And(
					HaveField("Field", "spec.instanceConfiguration.shapeConfig"),
					HaveField("Message", ContainSubstring("ocpus and vcpus are mutually exclusive; specify at most one")),
				)))
			}
			assertStored := func(t *testing.T, pool *unstructured.Unstructured, shape map[string]interface{}) {
				t.Helper()
				g := NewWithT(t)
				stored := newPool()
				g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(pool), stored)).To(Succeed())
				for _, field := range []string{"ocpus", "vcpus"} {
					got, found, err := unstructured.NestedFieldNoCopy(stored.Object, "spec", "instanceConfiguration", "shapeConfig", field)
					g.Expect(err).NotTo(HaveOccurred())
					want := shape[field]
					g.Expect(found).To(Equal(want != nil))
					if want == nil {
						g.Expect(got).To(BeNil())
					} else {
						g.Expect(got).To(Equal(want))
					}
				}
			}

			for _, tc := range []struct {
				name    string
				shape   map[string]interface{}
				invalid bool
			}{
				{name: "absent"},
				{name: "empty", shape: map[string]interface{}{}},
				{name: "ocpus", shape: map[string]interface{}{"ocpus": "1"}},
				{name: "vcpus", shape: map[string]interface{}{"vcpus": int64(2)}},
				{name: "zero vcpus", shape: map[string]interface{}{"vcpus": int64(0)}},
				{name: "both", shape: map[string]interface{}{"ocpus": "1", "vcpus": int64(2)}, invalid: true},
				{name: "empty ocpus and vcpus", shape: map[string]interface{}{"ocpus": "", "vcpus": int64(2)}, invalid: true},
				{name: "ocpus and zero vcpus", shape: map[string]interface{}{"ocpus": "1", "vcpus": int64(0)}, invalid: true},
				{name: "null ocpus", shape: map[string]interface{}{"ocpus": nil, "vcpus": int64(2)}},
				{name: "null vcpus", shape: map[string]interface{}{"ocpus": "1", "vcpus": nil}},
				{name: "both null", shape: map[string]interface{}{"ocpus": nil, "vcpus": nil}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					for _, operation := range []string{"create", "update"} {
						t.Run(operation, func(t *testing.T) {
							g := NewWithT(t)
							pool := newPool()
							if operation == "update" {
								g.Expect(testEnvironment.Create(testContext, pool)).To(Succeed())
								t.Cleanup(func() { deleteAndIgnoreNotFound(t, pool) })
							}
							setShape(pool, tc.shape)
							var err error
							if operation == "create" {
								err = testEnvironment.Create(testContext, pool)
								if err == nil {
									t.Cleanup(func() { deleteAndIgnoreNotFound(t, pool) })
								}
							} else {
								err = testEnvironment.Update(testContext, pool)
							}
							if tc.invalid {
								assertInvalid(t, err)
							} else {
								g.Expect(err).NotTo(HaveOccurred())
								assertStored(t, pool, tc.shape)
							}
						})
					}
				})
			}

			t.Run("switch CPU fields", func(t *testing.T) {
				g := NewWithT(t)
				pool := newPool()
				setShape(pool, map[string]interface{}{"ocpus": "1"})
				g.Expect(testEnvironment.Create(testContext, pool)).To(Succeed())
				t.Cleanup(func() { deleteAndIgnoreNotFound(t, pool) })
				for _, target := range []map[string]interface{}{{"vcpus": int64(2)}, {"ocpus": "1"}} {
					setShape(pool, map[string]interface{}{"ocpus": "1", "vcpus": int64(2)})
					assertInvalid(t, testEnvironment.Update(testContext, pool))
					setShape(pool, target)
					g.Expect(testEnvironment.Update(testContext, pool)).To(Succeed())
					assertStored(t, pool, target)
				}
			})
		})
	}
}
