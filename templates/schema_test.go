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
	"strings"
	"testing"

	infrastructurev1beta3 "github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	infrav3exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta3"
	corev1 "k8s.io/api/core/v1"
	addonsv1 "sigs.k8s.io/cluster-api/api/addons/v1beta2"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/yamlprocessor"
	utilyaml "sigs.k8s.io/cluster-api/util/yaml"
	"sigs.k8s.io/yaml"
)

// TestTemplateSchemas validates that all release templates and compiled E2E
// cluster templates strictly conform to Cluster API v1beta2 and CAPOCI v1beta3
// schemas with no deprecated v1beta1 API versions or removed fields.
func TestTemplateSchemas(t *testing.T) {
	releaseTemplates, err := filepath.Glob("*.yaml")
	if err != nil || len(releaseTemplates) == 0 {
		t.Fatalf("finding release templates: %v", err)
	}
	e2eV2Templates, err := filepath.Glob(filepath.Join("..", "test", "e2e", "data", "infrastructure-oci", "v1beta2", "cluster-template*.yaml"))
	if err != nil || len(e2eV2Templates) == 0 {
		t.Fatalf("finding e2e v1beta2 templates: %v", err)
	}
	e2eExtra := []string{
		filepath.Join("..", "test", "e2e", "data", "infrastructure-oci", "v1beta2", "cluster-template-cluster-class", "clusterclass-test-cluster-class.yaml"),
	}

	allPaths := append(releaseTemplates, e2eV2Templates...)
	allPaths = append(allPaths, e2eExtra...)

	for _, path := range allPaths {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			rendered, err := yamlprocessor.NewSimpleProcessor().Process(raw, func(name string) (string, error) {
				switch name {
				case "NAMESPACE":
					return "default", nil
				case "CLUSTER_NAME":
					return "test-cluster", nil
				case "CNI_RESOURCES", "ANTREA_RESOURCES":
					return "{}", nil
				case "OCI_CONTROL_PLANE_PV_TRANSIT_ENCRYPTION", "OCI_NODE_PV_TRANSIT_ENCRYPTION", "OCI_PV_TRANSIT_ENCRYPTION":
					return "true", nil
				case "CONTROL_PLANE_MACHINE_COUNT", "NODE_MACHINE_COUNT", "WORKER_MACHINE_COUNT", "NODE_DRAIN_TIMEOUT", "OCI_MANAGED_NODE_BOOT_VOLUME_SIZE":
					return "1", nil
				default:
					if strings.HasSuffix(name, "_B64") {
						return "dGVzdA==", nil
					}
					if strings.HasSuffix(name, "_COUNT") || strings.HasSuffix(name, "_TIMEOUT") || strings.HasSuffix(name, "_SIZE") ||
						strings.HasSuffix(name, "_IN_MILLIS") || strings.HasSuffix(name, "_PORT") || strings.HasSuffix(name, "_RETRIES") {
						return "1", nil
					}
					return "placeholder", nil
				}
			})
			if err != nil {
				t.Fatalf("processing template: %v", err)
			}
			objects, err := utilyaml.ToUnstructured(rendered)
			if err != nil {
				t.Fatalf("parsing YAML: %v", err)
			}
			if len(objects) == 0 {
				t.Fatal("template contains no resources")
			}

			for _, obj := range objects {
				gvk := obj.GroupVersionKind()
				objBytes, err := yaml.Marshal(obj.Object)
				if err != nil {
					t.Fatalf("marshaling %s/%s: %v", gvk, obj.GetName(), err)
				}

				var target any
				switch gvk.Group + "/" + gvk.Version + "/" + gvk.Kind {
				case "cluster.x-k8s.io/v1beta2/Cluster":
					target = &clusterv1.Cluster{}
				case "cluster.x-k8s.io/v1beta2/ClusterClass":
					target = &clusterv1.ClusterClass{}
				case "cluster.x-k8s.io/v1beta2/MachineDeployment":
					target = &clusterv1.MachineDeployment{}
				case "cluster.x-k8s.io/v1beta2/MachinePool":
					target = &clusterv1.MachinePool{}
				case "cluster.x-k8s.io/v1beta2/MachineHealthCheck":
					target = &clusterv1.MachineHealthCheck{}
				case "controlplane.cluster.x-k8s.io/v1beta2/KubeadmControlPlane":
					target = &controlplanev1.KubeadmControlPlane{}
				case "controlplane.cluster.x-k8s.io/v1beta2/KubeadmControlPlaneTemplate":
					target = &controlplanev1.KubeadmControlPlaneTemplate{}
				case "bootstrap.cluster.x-k8s.io/v1beta2/KubeadmConfig":
					target = &bootstrapv1.KubeadmConfig{}
				case "bootstrap.cluster.x-k8s.io/v1beta2/KubeadmConfigTemplate":
					target = &bootstrapv1.KubeadmConfigTemplate{}
				case "addons.cluster.x-k8s.io/v1beta2/ClusterResourceSet":
					target = &addonsv1.ClusterResourceSet{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCICluster":
					target = &infrastructurev1beta3.OCICluster{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIClusterTemplate":
					target = &infrastructurev1beta3.OCIClusterTemplate{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIMachine":
					target = &infrastructurev1beta3.OCIMachine{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIMachineTemplate":
					target = &infrastructurev1beta3.OCIMachineTemplate{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIClusterIdentity":
					target = &infrastructurev1beta3.OCIClusterIdentity{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIManagedCluster":
					target = &infrastructurev1beta3.OCIManagedCluster{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIManagedClusterTemplate":
					target = &infrastructurev1beta3.OCIManagedClusterTemplate{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIManagedControlPlane":
					target = &infrastructurev1beta3.OCIManagedControlPlane{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIManagedControlPlaneTemplate":
					target = &infrastructurev1beta3.OCIManagedControlPlaneTemplate{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIMachinePool":
					target = &infrav3exp.OCIMachinePool{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIManagedMachinePool":
					target = &infrav3exp.OCIManagedMachinePool{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIManagedMachinePoolTemplate":
					target = &infrav3exp.OCIManagedMachinePoolTemplate{}
				case "infrastructure.cluster.x-k8s.io/v1beta3/OCIVirtualMachinePool":
					target = &infrav3exp.OCIVirtualMachinePool{}
				case "/v1/ConfigMap":
					target = &corev1.ConfigMap{}
				case "/v1/Secret":
					target = &corev1.Secret{}
				default:
					t.Errorf("unexpected GVK %s on %s (expected CAPI v1beta2 or CAPOCI v1beta3)", gvk, obj.GetName())
					continue
				}

				if err := yaml.UnmarshalStrict(objBytes, target); err != nil {
					t.Errorf("strict unmarshal failed for %s %s: %v", gvk, obj.GetName(), err)
				}
			}
		})
	}
}
