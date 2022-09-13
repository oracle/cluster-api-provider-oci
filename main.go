/*
Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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

package main

import (
	"flag"
	"os"

	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/config"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/controllers"
	expV1Beta1 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	infrav1exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	expcontrollers "github.com/oracle/cluster-api-provider-oci/exp/controllers"
	"github.com/oracle/cluster-api-provider-oci/feature"
	"github.com/oracle/cluster-api-provider-oci/version"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme         = runtime.NewScheme()
	setupLog       = ctrl.Log.WithName("setup")
	logOptions     = logs.NewOptions()
	webhookPort    int
	webhookCertDir string
)

const (
	AuthConfigDirectory = "AUTH_CONFIG_DIR"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1beta1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(expV1Beta1.AddToScheme(scheme))
	utilruntime.Must(expclusterv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var webhookPort int

	fs := pflag.CommandLine
	logs.AddFlags(fs, logs.SkipLoggingConfigurationFlags())
	logOptions.AddFlags(fs)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&webhookPort,
		"webhook-port",
		9443,
		"Webhook Server port.",
	)
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)

	// Need to use pflags for kubernetes feature flags
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	feature.MutableGates.AddFlag(pflag.CommandLine)
	pflag.Parse()

	if err := logOptions.ValidateAndApply(nil); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// klog.Background will automatically use the right logger.
	ctrl.SetLogger(klog.Background())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   webhookPort,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "controller-leader-elect-capoci",
		CertDir:                webhookCertDir,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	authConfigDir := os.Getenv(AuthConfigDirectory)
	if authConfigDir == "" {
		setupLog.Error(err, "auth config directory environment variable is not set")
		os.Exit(1)
	}

	authConfig, err := config.FromDir(authConfigDir)
	if err != nil {
		setupLog.Error(err, "invalid auth config file")
		os.Exit(1)
	}

	setupLog.Info("CAPOCI Version", "version", version.GitVersion)
	ociAuthConfigProvider, err := config.NewConfigurationProvider(authConfig)
	if err != nil {
		setupLog.Error(err, "authentication provider could not be initialised")
		os.Exit(1)
	}

	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	region, err := ociAuthConfigProvider.Region()
	if err != nil {
		setupLog.Error(err, "unable to get OCI region from AuthConfigProvider")
		os.Exit(1)
	}

	clientProvider, err := scope.NewClientProvider(ociAuthConfigProvider)
	if err != nil {
		setupLog.Error(err, "unable to create OCI ClientProvider")
		os.Exit(1)
	}
	_, err = clientProvider.GetOrBuildClient(region)
	if err != nil {
		setupLog.Error(err, "authentication provider could not be initialised")
		os.Exit(1)
	}

	if err = (&controllers.OCIClusterReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Region:         region,
		ClientProvider: clientProvider,
		Recorder:       mgr.GetEventRecorderFor("ocicluster-controller"),
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", scope.OCIClusterKind)
		os.Exit(1)
	}

	if err = (&controllers.OCIMachineReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		ClientProvider: clientProvider,
		Region:         region,
		Recorder:       mgr.GetEventRecorderFor("ocimachine-controller"),
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", scope.OCIMachineKind)
		os.Exit(1)
	}

	if feature.Gates.Enabled(feature.MachinePool) {
		setupLog.Info("MACHINE POOL experimental feature enabled")
		setupLog.V(1).Info("enabling machine pool controller")
		if err := (&expcontrollers.OCIMachinePoolReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			ClientProvider: clientProvider,
			Recorder:       mgr.GetEventRecorderFor("ocimachinepool-controller"),
			Region:         region,
		}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", scope.OCIMachinePoolKind)
			os.Exit(1)
		}
	}
	if feature.Gates.Enabled(feature.OKE) {
		setupLog.Info("OKE experimental feature enabled")
		setupLog.V(1).Info("enabling managed machine pool controller")
		if err = (&expcontrollers.OCIManagedMachinePoolReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			Region:         region,
			ClientProvider: clientProvider,
			Recorder:       mgr.GetEventRecorderFor("ocimanagedmachinepool-controller"),
		}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", scope.OCIManagedMachinePoolKind)
			os.Exit(1)
		}

		if err = (&expcontrollers.OCIManagedClusterReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			Region:         region,
			ClientProvider: clientProvider,
			Recorder:       mgr.GetEventRecorderFor("ocimanagedcluster-controller"),
		}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", scope.OCIManagedClusterKind)
			os.Exit(1)
		}

		if err = (&expcontrollers.OCIManagedClusterControlPlaneReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			Region:         region,
			ClientProvider: clientProvider,
			Recorder:       mgr.GetEventRecorderFor("ocimanagedclustercontrolplane-controller"),
		}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", scope.OCIManagedClusterControlPlaneKind)
			os.Exit(1)
		}
	}

	if err = (&infrastructurev1beta1.OCICluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCICluster")
		os.Exit(1)
	}

	if err = (&infrastructurev1beta1.OCIMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCIMachineTemplate")
		os.Exit(1)
	}

	if err = (&infrav1exp.OCIManagedCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCIManagedCluster")
		os.Exit(1)
	}

	if err = (&infrav1exp.OCIManagedControlPlane{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCIManagedControlPlane")
		os.Exit(1)
	}

	if err = (&infrav1exp.OCIManagedMachinePool{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCIManagedMachinePool")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
