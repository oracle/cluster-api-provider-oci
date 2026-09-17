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
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
