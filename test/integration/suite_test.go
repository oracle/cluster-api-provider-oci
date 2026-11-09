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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/controllers"
	expv1beta1 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	expv1beta2 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	expcontrollers "github.com/oracle/cluster-api-provider-oci/exp/controllers"
	testenv "github.com/oracle/cluster-api-provider-oci/internal/test/envtest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sversion "k8s.io/apimachinery/pkg/version"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	contractutil "sigs.k8s.io/cluster-api/util/contract"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	testContext               = context.Background()
	testEnvironment           *testenv.Environment
	fakeOCI                   *fakeOCIBackend
	integrationClientProvider *scope.ClientProvider
)

func TestMain(m *testing.M) {
	ctrl.SetLogger(klog.Background())

	fakeOCI = newFakeOCIBackend()
	var err error
	integrationClientProvider, err = fakeOCI.clientProvider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create fake OCI client provider: %v\n", err)
		os.Exit(1)
	}

	repositoryRoot, err := findRepositoryRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repository root: %v\n", err)
		os.Exit(1)
	}

	capiModuleDirectory, err := findModuleDirectory(testContext, repositoryRoot, "sigs.k8s.io/cluster-api")
	if err != nil {
		fmt.Fprintf(os.Stderr, "find Cluster API module directory: %v\n", err)
		os.Exit(1)
	}

	testEnvironment, err = testenv.Start(testContext, testenv.Options{
		Scheme: integrationScheme(),
		CRDDirectoryPaths: []string{
			filepath.Join(repositoryRoot, "config", "crd", "bases"),
			filepath.Join(capiModuleDirectory, "config", "crd", "bases"),
		},
		WebhookManifestPaths: []string{
			filepath.Join(repositoryRoot, "config", "webhook", "manifests.yaml"),
		},
		SetupManager: setupManager,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "start integration test environment: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	if err := testEnvironment.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "stop integration test environment: %v\n", err)
		code = 1
	}
	os.Exit(code)
}

func integrationScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1beta1.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1beta2.AddToScheme(scheme))
	utilruntime.Must(clusterv1beta1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(expv1beta1.AddToScheme(scheme))
	utilruntime.Must(expv1beta2.AddToScheme(scheme))
	return scheme
}

func setupManager(ctx context.Context, mgr manager.Manager) error {
	clusterv1beta1.SetAPIVersionGetter(func(gk schema.GroupKind) (string, error) {
		return apiVersionForContract(ctx, mgr.GetAPIReader(), gk)
	})

	controllerOptions := controller.Options{MaxConcurrentReconciles: 1}
	setups := []struct {
		name  string
		setup func() error
	}{
		{
			name: scope.OCIClusterKind,
			setup: func() error {
				return (&controllers.OCIClusterReconciler{
					Client:         mgr.GetClient(),
					Scheme:         mgr.GetScheme(),
					ClientProvider: integrationClientProvider,
					Region:         scope.MockTestRegion,
					Recorder:       mgr.GetEventRecorderFor("integration-ocicluster-controller"),
				}).SetupWithManager(ctx, mgr, controllerOptions)
			},
		},
		{
			name: scope.OCIMachineKind,
			setup: func() error {
				return (&controllers.OCIMachineReconciler{
					Client:         mgr.GetClient(),
					Scheme:         mgr.GetScheme(),
					ClientProvider: integrationClientProvider,
					Region:         scope.MockTestRegion,
					Recorder:       mgr.GetEventRecorderFor("integration-ocimachine-controller"),
				}).SetupWithManager(ctx, mgr, controllerOptions)
			},
		},
		{
			name: scope.OCIManagedClusterKind,
			setup: func() error {
				return (&controllers.OCIManagedClusterReconciler{
					Client:         mgr.GetClient(),
					Scheme:         mgr.GetScheme(),
					ClientProvider: integrationClientProvider,
					Region:         scope.MockTestRegion,
					Recorder:       mgr.GetEventRecorderFor("integration-ocimanagedcluster-controller"),
				}).SetupWithManager(ctx, mgr, controllerOptions)
			},
		},
		{
			name: scope.OCIManagedClusterControlPlaneKind,
			setup: func() error {
				return (&controllers.OCIManagedClusterControlPlaneReconciler{
					Client:         mgr.GetClient(),
					Scheme:         mgr.GetScheme(),
					ClientProvider: integrationClientProvider,
					Region:         scope.MockTestRegion,
					Recorder:       mgr.GetEventRecorderFor("integration-ocimanagedcontrolplane-controller"),
				}).SetupWithManager(ctx, mgr, controllerOptions)
			},
		},
		{
			name: scope.OCIMachinePoolKind,
			setup: func() error {
				return (&expcontrollers.OCIMachinePoolReconciler{
					Client:         mgr.GetClient(),
					Scheme:         mgr.GetScheme(),
					ClientProvider: integrationClientProvider,
					Region:         scope.MockTestRegion,
					Recorder:       mgr.GetEventRecorderFor("integration-ocimachinepool-controller"),
				}).SetupWithManager(ctx, mgr, controllerOptions)
			},
		},
		{
			name: scope.OCIManagedMachinePoolKind,
			setup: func() error {
				return (&expcontrollers.OCIManagedMachinePoolReconciler{
					Client:         mgr.GetClient(),
					Scheme:         mgr.GetScheme(),
					ClientProvider: integrationClientProvider,
					Region:         scope.MockTestRegion,
					Recorder:       mgr.GetEventRecorderFor("integration-ocimanagedmachinepool-controller"),
				}).SetupWithManager(ctx, mgr, controllerOptions)
			},
		},
		{
			name: scope.OCIVirtualMachinePoolKind,
			setup: func() error {
				return (&expcontrollers.OCIVirtualMachinePoolReconciler{
					Client:         mgr.GetClient(),
					Scheme:         mgr.GetScheme(),
					ClientProvider: integrationClientProvider,
					Region:         scope.MockTestRegion,
					Recorder:       mgr.GetEventRecorderFor("integration-ocivirtualmachinepool-controller"),
				}).SetupWithManager(ctx, mgr, controllerOptions)
			},
		},
		{
			name: "OCIMachinePoolMachine",
			setup: func() error {
				return (&expcontrollers.OCIMachinePoolMachineReconciler{
					Client:         mgr.GetClient(),
					Scheme:         mgr.GetScheme(),
					ClientProvider: integrationClientProvider,
					Region:         scope.MockTestRegion,
					Recorder:       mgr.GetEventRecorderFor("integration-ocimachinepoolmachine-controller"),
				}).SetupWithManager(ctx, mgr, controllerOptions)
			},
		},
		{
			name: "OCICluster webhook",
			setup: func() error {
				return (&infrastructurev1beta2.OCICluster{}).SetupWebhookWithManager(mgr)
			},
		},
		{
			name: "OCIMachineTemplate webhook",
			setup: func() error {
				return (&infrastructurev1beta2.OCIMachineTemplate{}).SetupWebhookWithManager(mgr)
			},
		},
		{
			name: "OCIManagedCluster webhook",
			setup: func() error {
				return (&infrastructurev1beta2.OCIManagedCluster{}).SetupWebhookWithManager(mgr)
			},
		},
		{
			name: "OCIManagedControlPlane webhook",
			setup: func() error {
				return (&infrastructurev1beta2.OCIManagedControlPlane{}).SetupWebhookWithManager(mgr)
			},
		},
		{
			name: "OCIManagedMachinePool webhook",
			setup: func() error {
				return (&expv1beta2.OCIManagedMachinePool{}).SetupWebhookWithManager(mgr)
			},
		},
		{
			name: "OCIVirtualMachinePool webhook",
			setup: func() error {
				return (&expv1beta2.OCIVirtualMachinePool{}).SetupWebhookWithManager(mgr)
			},
		},
	}

	for _, item := range setups {
		if err := item.setup(); err != nil {
			return fmt.Errorf("set up %s: %w", item.name, err)
		}
	}
	return nil
}

func apiVersionForContract(ctx context.Context, reader client.Reader, gk schema.GroupKind) (string, error) {
	crd := &unstructured.Unstructured{}
	crd.SetName(contractutil.CalculateCRDName(gk.Group, gk.Kind))
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})
	if err := reader.Get(ctx, client.ObjectKeyFromObject(crd), crd); err != nil {
		return "", fmt.Errorf("get CustomResourceDefinition for %s: %w", gk, err)
	}

	var versions []string
	for _, contractVersion := range []string{"v1beta2", "v1beta1"} {
		value := crd.GetLabels()[clusterv1.GroupVersion.Group+"/"+contractVersion]
		if value != "" {
			versions = strings.Split(value, "_")
			break
		}
	}

	// The integration environment installs generated CRD bases directly, before
	// Kustomize adds CAPOCI's effective contract labels. Fall back to the served
	// versions in those bases so CAPI reference conversion can run in envtest.
	if len(versions) == 0 {
		crdVersions, found, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
		if err != nil {
			return "", fmt.Errorf("read versions from %s: %w", crd.GetName(), err)
		}
		if !found {
			return "", fmt.Errorf("no versions found on %s", crd.GetName())
		}
		for _, item := range crdVersions {
			version, ok := item.(map[string]interface{})
			if !ok || version["served"] != true {
				continue
			}
			name, ok := version["name"].(string)
			if ok && name != "" {
				versions = append(versions, name)
			}
		}
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no compatible API versions found on %s", crd.GetName())
	}

	sort.Slice(versions, func(i, j int) bool {
		return k8sversion.CompareKubeAwareVersionStrings(versions[i], versions[j]) < 0
	})
	return schema.GroupVersion{Group: gk.Group, Version: versions[len(versions)-1]}.String(), nil
}

func findRepositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		goModPath := filepath.Join(directory, "go.mod")
		info, err := os.Stat(goModPath)
		if err == nil {
			if info.IsDir() {
				return "", fmt.Errorf("repository marker %s is a directory", goModPath)
			}
			return directory, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect repository marker %s: %w", goModPath, err)
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("could not find repository go.mod from working directory")
		}
		directory = parent
	}
}

func findModuleDirectory(ctx context.Context, workingDirectory, modulePath string) (string, error) {
	command := exec.CommandContext(ctx, "go", "list", "-m", "-f={{.Dir}}", modulePath)
	command.Dir = workingDirectory
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go list %s: %w: %s", modulePath, err, strings.TrimSpace(string(output)))
	}

	directory := strings.TrimSpace(string(output))
	if directory == "" {
		return "", fmt.Errorf("go list %s returned an empty directory", modulePath)
	}
	return directory, nil
}
