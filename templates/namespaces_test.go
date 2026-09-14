/*
Copyright (c) 2026, Oracle and/or its affiliates.

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

package templates_test

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/yamlprocessor"
	utilyaml "sigs.k8s.io/cluster-api/util/yaml"
)

// TestTemplateNamespaces guards against hard-coded or omitted namespaces in the
// example templates. When applied after variable substitution, related resources
// must stay in the selected namespace, including when it is not "default".
// Clusterctl's later namespace rewrite can mask these mistakes, so this test
// checks substitution alone for both default and custom namespaces. It runs
// locally without a cluster or E2E setup; embedded workload addon YAML retains
// its own namespaces, such as kube-system.
func TestTemplateNamespaces(t *testing.T) {
	paths, err := filepath.Glob("*.yaml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("finding release templates: %v", err)
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, namespace := range []string{"default", "custom-namespace"} {
				t.Run(namespace, func(t *testing.T) {
					// Use clusterctl's substitution without its subsequent namespace rewrite,
					// which would hide hard-coded namespaces from this regression check.
					rendered, err := yamlprocessor.NewSimpleProcessor().Process(raw, func(name string) (string, error) {
						if name == "NAMESPACE" {
							return namespace, nil
						}
						return "1", nil
					})
					if err != nil {
						t.Fatal(err)
					}
					objects, err := utilyaml.ToUnstructured(rendered)
					if err != nil {
						t.Fatal(err)
					}
					if len(objects) == 0 {
						t.Fatal("template contains no resources")
					}
					for _, object := range objects {
						if object.GetNamespace() != namespace {
							t.Errorf("%s/%s namespace = %q, want %q", object.GetKind(), object.GetName(), object.GetNamespace(), namespace)
						}
						checkNamespaces(t, object.Object, namespace)
					}
				})
			}
		})
	}
}

func checkNamespaces(t *testing.T, value interface{}, namespace string) {
	t.Helper()
	switch value := value.(type) {
	case map[string]interface{}:
		for key, child := range value {
			if key == "namespace" && child != namespace {
				t.Errorf("namespace = %v, want %s", child, namespace)
			}
			// Embedded addon YAML is a string, so workload namespaces are not traversed.
			checkNamespaces(t, child, namespace)
		}
	case []interface{}:
		for _, child := range value {
			checkNamespaces(t, child, namespace)
		}
	}
}
