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
	"time"

	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/config"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/controllers"
	expV1Beta1 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	expV1Beta2 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	expcontrollers "github.com/oracle/cluster-api-provider-oci/exp/controllers"
	"github.com/oracle/cluster-api-provider-oci/feature"
	"github.com/oracle/cluster-api-provider-oci/version"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/logs"
	logsV1 "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme     = runtime.NewScheme()
	setupLog   = ctrl.Log.WithName("setup")
	logOptions = logs.NewOptions()
)

const (
	AuthConfigDirectory = "AUTH_CONFIG_DIR"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1beta1.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1beta2.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(expV1Beta1.AddToScheme(scheme))
	utilruntime.Must(expV1Beta2.AddToScheme(scheme))
	utilruntime.Must(expclusterv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var leaderElectionNamespace string
	var leaderElectionLeaseDuration time.Duration
	var leaderElectionRenewDeadline time.Duration
	var leaderElectionRetryPeriod time.Duration
	var watchNamespace string
	var probeAddr string
	var webhookPort int
	var webhookCertDir string
	// Flags for reconciler concurrency
	var ociClusterConcurrency int
	var ociMachineConcurrency int
	var ociMachinePoolConcurrency int
	var initOciClientsOnStartup bool
	var enableInstanceMetadataServiceLookup bool

	fs := pflag.CommandLine
	logs.AddFlags(fs, logs.SkipLoggingConfigurationFlags())
	logsV1.AddFlags(logOptions, fs)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(
		&leaderElectionNamespace,
		"leader-election-namespace",
		"",
		"Namespace that the controller performs leader election in. If unspecified, the controller will discover which namespace it is running in.",
	)
	flag.DurationVar(
		&leaderElectionLeaseDuration,
		"leader-elect-lease-duration",
		15*time.Second,
		"Interval at which non-leader candidates will wait to force acquire leadership (duration string)",
	)
	flag.DurationVar(
		&leaderElectionRenewDeadline,
		"leader-elect-renew-deadline",
		10*time.Second,
		"Duration that the leading controller manager will retry refreshing leadership before giving up (duration string)",
	)
	flag.DurationVar(
		&leaderElectionRetryPeriod,
		"leader-elect-retry-period",
		2*time.Second,
		"Duration the LeaderElector clients should wait between tries of actions (duration string)",
	)
	flag.IntVar(&webhookPort,
		"webhook-port",
		9443,
		"Webhook Server port.",
	)
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.")
	flag.IntVar(
		&ociClusterConcurrency,
		"ocicluster-concurrency",
		5,
		"Number of OciClusters to process simultaneously",
	)
	flag.IntVar(
		&ociMachineConcurrency,
		"ocimachine-concurrency",
		10,
		"Number of OciMachines to process simultaneously",
	)
	flag.IntVar(
		&ociMachinePoolConcurrency,
		"ocimachinepool-concurrency",
		5,
		"Number of OciMachinePools to process simultaneously",
	)
	flag.BoolVar(
		&initOciClientsOnStartup,
		"init-oci-clients-on-startup",
		true,
		"Initialize OCI clients on startup",
	)
	flag.StringVar(
		&watchNamespace,
		"namespace",
		"",
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.",
	)
	flag.BoolVar(
		&enableInstanceMetadataServiceLookup,
		"enable-instance-metadata-service-lookup",
		false,
		"Initialize OCI clients on startup",
	)

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)

	// Need to use pflags for kubernetes feature flags
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	feature.MutableGates.AddFlag(pflag.CommandLine)
	pflag.Parse()

	if err := logsV1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// klog.Background will automatically use the right logger.
	ctrl.SetLogger(klog.Background())

	var watchNamespaces map[string]cache.Config
	if watchNamespace != "" {
		watchNamespaces = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: webhookCertDir,
		}),
		HealthProbeBindAddress:     probeAddr,
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "controller-leader-elect-capoci",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaderElectionNamespace:    leaderElectionNamespace,
		LeaseDuration:              &leaderElectionLeaseDuration,
		RenewDeadline:              &leaderElectionRenewDeadline,
		RetryPeriod:                &leaderElectionRetryPeriod,
		Cache: cache.Options{
			DefaultNamespaces: watchNamespaces,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	var clientProvider *scope.ClientProvider
	var region string
	if initOciClientsOnStartup {
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

		region, err = ociAuthConfigProvider.Region()
		if err != nil {
			setupLog.Error(err, "unable to get OCI region from AuthConfigProvider")
			os.Exit(1)
		}

		clientProvider, err = scope.NewClientProvider(scope.ClientProviderParams{
			OciAuthConfigProvider: ociAuthConfigProvider})
		if err != nil {
			setupLog.Error(err, "unable to create OCI ClientProvider")
			os.Exit(1)
		}
		_, err = clientProvider.GetOrBuildClient(region)
		if err != nil {
			setupLog.Error(err, "authentication provider could not be initialised")
			os.Exit(1)
		}
	}
	if enableInstanceMetadataServiceLookup {
		common.EnableInstanceMetadataServiceLookup()
	}
	if err = (&controllers.OCIClusterReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Region:         region,
		ClientProvider: clientProvider,
		Recorder:       mgr.GetEventRecorderFor("ocicluster-controller"),
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: ociClusterConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", scope.OCIClusterKind)
		os.Exit(1)
	}

	if err = (&controllers.OCIMachineReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		ClientProvider: clientProvider,
		Region:         region,
		Recorder:       mgr.GetEventRecorderFor("ocimachine-controller"),
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: ociMachineConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", scope.OCIMachineKind)
		os.Exit(1)
	}

	if err = (&controllers.OCIManagedClusterReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Region:         region,
		ClientProvider: clientProvider,
		Recorder:       mgr.GetEventRecorderFor("ocimanagedcluster-controller"),
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: ociClusterConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", scope.OCIManagedClusterKind)
		os.Exit(1)
	}

	if err = (&controllers.OCIManagedClusterControlPlaneReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Region:         region,
		ClientProvider: clientProvider,
		Recorder:       mgr.GetEventRecorderFor("ocimanagedclustercontrolplane-controller"),
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: ociClusterConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", scope.OCIManagedClusterControlPlaneKind)
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
		}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: ociMachinePoolConcurrency}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", scope.OCIMachinePoolKind)
			os.Exit(1)
		}

		setupLog.Info("OKE experimental feature enabled")
		setupLog.V(1).Info("enabling managed machine pool controller")
		if err = (&expcontrollers.OCIManagedMachinePoolReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			Region:         region,
			ClientProvider: clientProvider,
			Recorder:       mgr.GetEventRecorderFor("ocimanagedmachinepool-controller"),
		}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: ociMachinePoolConcurrency}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", scope.OCIManagedMachinePoolKind)
			os.Exit(1)
		}

		if err = (&expcontrollers.OCIVirtualMachinePoolReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			Region:         region,
			ClientProvider: clientProvider,
			Recorder:       mgr.GetEventRecorderFor("ocivirtualmachinepool-controller"),
		}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: ociClusterConcurrency}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", scope.OCIVirtualMachinePoolKind)
			os.Exit(1)
		}

		if err = (&expcontrollers.OCIMachinePoolMachineReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			Region:         region,
			ClientProvider: clientProvider,
			Recorder:       mgr.GetEventRecorderFor("ocimachinepoolmachine-controller"),
		}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: ociClusterConcurrency}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "OCIMachinePoolMachine")
			os.Exit(1)
		}
	}

	if err = (&infrastructurev1beta2.OCICluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCICluster")
		os.Exit(1)
	}

	if err = (&infrastructurev1beta2.OCIMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCIMachineTemplate")
		os.Exit(1)
	}

	if err = (&infrastructurev1beta2.OCIManagedCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCIManagedCluster")
		os.Exit(1)
	}

	if err = (&infrastructurev1beta2.OCIManagedControlPlane{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCIManagedControlPlane")
		os.Exit(1)
	}

	if err = (&expV1Beta2.OCIManagedMachinePool{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCIManagedMachinePool")
		os.Exit(1)
	}

	if err = (&expV1Beta2.OCIVirtualMachinePool{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "OCIVirtualMachinePool")
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
