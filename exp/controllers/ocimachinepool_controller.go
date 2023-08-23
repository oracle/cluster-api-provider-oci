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

package controllers

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"time"

	"github.com/go-logr/logr"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	cloudutil "github.com/oracle/cluster-api-provider-oci/cloud/util"
	expV1Beta1 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// OCIMachinePoolReconciler reconciles a OCIMachinePool object
type OCIMachinePoolReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ClientProvider *scope.ClientProvider
	Region         string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimachinepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimachinepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimachinepools/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the machinepool closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *OCIMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)
	logger.Info("Got reconciliation event for machine pool")

	// Fetch the OCIMachinePool.
	ociMachinePool := &infrav2exp.OCIMachinePool{}
	err := r.Get(ctx, req.NamespacedName, ociMachinePool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the CAPI MachinePool
	machinePool, err := getOwnerMachinePool(ctx, r.Client, ociMachinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machinePool == nil {
		r.Recorder.Eventf(ociMachinePool, corev1.EventTypeNormal, "OwnerRefNotSet", "Cluster Controller has not yet set OwnerRef")
		logger.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}
	logger = logger.WithValues("machinePool", machinePool.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, ociMachinePool.ObjectMeta)
	if err != nil {
		r.Recorder.Eventf(ociMachinePool, corev1.EventTypeWarning, "ClusterDoesNotExist", "MachinePool is missing cluster label or cluster does not exist")
		logger.Info("MachinePool is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}
	logger = logger.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, ociMachinePool) {
		logger.Info("OCIMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}
	var clusterAccessor scope.OCIClusterAccessor
	if cluster.Spec.InfrastructureRef.Kind == "OCICluster" {
		ociCluster := &infrastructurev1beta2.OCICluster{}
		ociClusterName := client.ObjectKey{
			Namespace: cluster.Namespace,
			Name:      cluster.Spec.InfrastructureRef.Name,
		}
		if err := r.Client.Get(ctx, ociClusterName, ociCluster); err != nil {
			logger.Info("Cluster is not available yet")
			r.Recorder.Eventf(ociMachinePool, corev1.EventTypeWarning, "ClusterNotAvailable", "Cluster is not available yet")
			logger.V(2).Info("OCICluster is not available yet")
			return ctrl.Result{}, nil
		}
		clusterAccessor = scope.OCISelfManagedCluster{
			OCICluster: ociCluster,
		}
	} else if cluster.Spec.InfrastructureRef.Kind == "OCIManagedCluster" {
		ociManagedCluster := &infrastructurev1beta2.OCIManagedCluster{}
		ociManagedClusterName := client.ObjectKey{
			Namespace: cluster.Namespace,
			Name:      cluster.Spec.InfrastructureRef.Name,
		}
		if err := r.Client.Get(ctx, ociManagedClusterName, ociManagedCluster); err != nil {
			logger.Info("Cluster is not available yet")
			r.Recorder.Eventf(ociMachinePool, corev1.EventTypeWarning, "ClusterNotAvailable", "Cluster is not available yet")
			logger.V(2).Info("OCIManagedCluster is not available yet")
			return ctrl.Result{}, nil
		}
		clusterAccessor = scope.OCIManagedCluster{
			OCIManagedCluster: ociManagedCluster,
		}
	} else {
		r.Recorder.Eventf(ociMachinePool, corev1.EventTypeWarning, "InfrastructureClusterTypeNotSupported", fmt.Sprintf("Infrastructure Cluster Type %s is not supported", cluster.Spec.InfrastructureRef.Kind))
		return ctrl.Result{}, errors.New(fmt.Sprintf("Infrastructure Cluster Type %s is not supported", cluster.Spec.InfrastructureRef.Kind))
	}

	_, _, clients, err := cloudutil.InitClientsAndRegion(ctx, r.Client, r.Region, clusterAccessor, r.ClientProvider)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the machine pool scope
	machinePoolScope, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Client:                  r.Client,
		ComputeManagementClient: clients.ComputeManagementClient,
		Logger:                  &logger,
		Cluster:                 cluster,
		OCIClusterAccessor:      clusterAccessor,
		MachinePool:             machinePool,
		OCIMachinePool:          ociMachinePool,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any OCIMachinePool changes.
	defer func() {
		if err := machinePoolScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !ociMachinePool.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machinePoolScope)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, logger, machinePoolScope)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCIMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	logger := log.FromContext(ctx)
	clusterToObjectFunc, err := util.ClusterToTypedObjectsMapper(r.Client, &expV1Beta1.OCIMachinePoolList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to create mapper for Cluster to OCIMachinePool")
	}
	gvk, err := apiutil.GVKForObject(new(infrav2exp.OCIMachinePoolList), mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to find GVK for OCIMachinePool")
	}
	managedClusterToMachinePoolMap := managedClusterToManagedMachinePoolMapFunc(r.Client, gvk, logger)

	err = ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav2exp.OCIMachinePool{}).
		Watches(
			&expclusterv1.MachinePool{},
			handler.EnqueueRequestsFromMapFunc(machinePoolToInfrastructureMapFunc(infrav2exp.
				GroupVersion.WithKind(scope.OCIMachinePoolKind), logger)),
		).
		Watches(
			&infrastructurev1beta2.OCIManagedCluster{},
			handler.EnqueueRequestsFromMapFunc(managedClusterToMachinePoolMap),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc),
			builder.WithPredicates(
				predicates.ClusterUnpausedAndInfrastructureReady(ctrl.LoggerFrom(ctx)),
			),
		).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(ctx))).
		Complete(r)

	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}
	return nil
}

func machinePoolToInfrastructureMapFunc(gvk schema.GroupVersionKind, logger logr.Logger) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		m, ok := o.(*expclusterv1.MachinePool)
		if !ok {
			panic(fmt.Sprintf("Expected a MachinePool but got a %T", o))
		}

		gk := gvk.GroupKind()
		// Return early if the GroupKind doesn't match what we expect
		infraGK := m.Spec.Template.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if gk != infraGK {
			logger.V(4).Info("gk does not match", "gk", gk, "infraGK", infraGK)
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      m.Spec.Template.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}

// getOwnerMachinePool returns the MachinePool object owning the current resource.
func getOwnerMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*expclusterv1.MachinePool, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "MachinePool" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if gv.Group == expclusterv1.GroupVersion.Group {
			return getMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// getMachinePoolByName finds and return a Machine object using the specified params.
func getMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*expclusterv1.MachinePool, error) {
	m := &expclusterv1.MachinePool{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (r *OCIMachinePoolReconciler) reconcileNormal(ctx context.Context, logger logr.Logger, machinePoolScope *scope.MachinePoolScope) (ctrl.Result, error) {
	machinePoolScope.Info("Handling reconcile OCIMachinePool")

	// If the OCIMachinePool is in an error state, return early.
	if machinePoolScope.HasFailed() {
		machinePoolScope.Info("Error state detected, skipping reconciliation")

		return ctrl.Result{}, nil
	}

	// If the OCIMachinePool doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machinePoolScope.OCIMachinePool, infrav2exp.MachinePoolFinalizer)
	// Register the finalizer immediately to avoid orphaning OCI resources on delete
	if err := machinePoolScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if !machinePoolScope.Cluster.Status.InfrastructureReady {
		logger.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machinePoolScope.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		r.Recorder.Event(machinePoolScope.OCIMachinePool, corev1.EventTypeNormal, infrastructurev1beta2.WaitingForBootstrapDataReason, "Bootstrap data secret reference is not yet available")
		conditions.MarkFalse(machinePoolScope.OCIMachinePool, infrav2exp.InstancePoolReadyCondition, infrastructurev1beta2.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		logger.Info("Bootstrap data secret reference is not yet available")
		return reconcile.Result{}, nil
	}

	// get or create the InstanceConfiguration
	// https://docs.oracle.com/en-us/iaas/api/#/en/iaas/20160918/InstanceConfiguration/
	if err := machinePoolScope.ReconcileInstanceConfiguration(ctx); err != nil {
		r.Recorder.Eventf(machinePoolScope.OCIMachinePool, corev1.EventTypeWarning, "FailedLaunchTemplateReconcile", "Failed to reconcile launch template: %v", err)
		return ctrl.Result{}, err
	}

	// set the LaunchTemplateReady condition
	conditions.MarkTrue(machinePoolScope.OCIMachinePool, infrav2exp.LaunchTemplateReadyCondition)

	// Find existing Instance Pool
	instancePool, err := machinePoolScope.FindInstancePool(ctx)
	if err != nil {
		r.Recorder.Event(machinePoolScope.OCIMachinePool, corev1.EventTypeWarning, "ReconcileError", err.Error())
		conditions.MarkUnknown(machinePoolScope.OCIMachinePool, infrav2exp.InstancePoolReadyCondition, infrav2exp.InstancePoolNotFoundReason, "")
		return ctrl.Result{}, err
	}

	if instancePool == nil {
		if _, err := machinePoolScope.CreateInstancePool(ctx); err != nil {
			r.Recorder.Event(machinePoolScope.OCIMachinePool, corev1.EventTypeWarning, "ReconcileError", err.Error())
			conditions.MarkFalse(machinePoolScope.OCIMachinePool, infrav2exp.InstancePoolReadyCondition, infrav2exp.InstancePoolProvisionFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(machinePoolScope.OCIMachinePool, corev1.EventTypeNormal, "InstancePoolCreated", "Created new Instance Pool: %s", machinePoolScope.OCIMachinePool.GetName())
		return ctrl.Result{}, nil
	}

	machinePoolScope.Info("OCI Compute Instance Pool found", "InstancePoolId", *instancePool.Id)
	machinePoolScope.OCIMachinePool.Spec.ProviderID = common.String(fmt.Sprintf("oci://%s", *instancePool.Id))
	machinePoolScope.OCIMachinePool.Spec.OCID = instancePool.Id

	switch instancePool.LifecycleState {
	case core.InstancePoolLifecycleStateProvisioning, core.InstancePoolLifecycleStateStarting:
		machinePoolScope.Info("Instance Pool is pending")
		conditions.MarkFalse(machinePoolScope.OCIMachinePool, infrav2exp.InstancePoolReadyCondition, infrav2exp.InstancePoolNotReadyReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	case core.InstancePoolLifecycleStateScaling:
		machinePoolScope.Info("Instance Pool is scaling")
		conditions.MarkFalse(machinePoolScope.OCIMachinePool, infrav2exp.InstancePoolReadyCondition, infrav2exp.InstancePoolNotReadyReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	case core.InstancePoolLifecycleStateRunning:
		machinePoolScope.Info("Instance pool is active")

		// record the event only when pool goes from not ready to ready state
		r.Recorder.Eventf(machinePoolScope.OCIMachinePool, corev1.EventTypeNormal, "InstancePoolReady",
			"Instance pool is in ready state")
		conditions.MarkTrue(machinePoolScope.OCIMachinePool, infrav2exp.InstancePoolReadyCondition)

		machines, err := machinePoolScope.SetListandSetMachinePoolInstances(ctx)
		if err != nil {
			return reconcile.Result{}, err
		}
		if err != nil {
			return reconcile.Result{}, err
		}
		providerIDList := make([]string, 0)
		for _, machine := range machines {
			if machine.Status.Ready {
				providerIDList = append(providerIDList, *machine.Spec.ProviderID)
			}
		}
		machinePoolScope.OCIMachinePool.Spec.ProviderIDList = providerIDList

		err = r.reconcileMachines(ctx, err, machinePoolScope, machines)
		if err != nil {
			return reconcile.Result{}, err
		}

		instancePool, err = machinePoolScope.UpdatePool(ctx, instancePool)
		if err != nil {
			r.Recorder.Eventf(machinePoolScope.OCIMachinePool, corev1.EventTypeWarning, "FailedUpdate", "Failed to update instance pool: %v", err)
			machinePoolScope.Error(err, "error updating OCIMachinePool")
			return ctrl.Result{}, err
		}
		err = machinePoolScope.CleanupInstanceConfiguration(ctx, instancePool)
		if err != nil {
			return ctrl.Result{}, err
		}
		machinePoolScope.SetReplicaCount(int32(len(providerIDList)))
		machinePoolScope.SetReady()
	default:
		conditions.MarkFalse(machinePoolScope.OCIMachinePool, infrav2exp.InstancePoolReadyCondition, infrav2exp.InstancePoolProvisionFailedReason, clusterv1.ConditionSeverityError, "")
		machinePoolScope.SetFailureReason(capierrors.CreateMachineError)
		machinePoolScope.SetFailureMessage(errors.Errorf("Instance Pool status %q is unexpected", instancePool.LifecycleState))
		r.Recorder.Eventf(machinePoolScope.OCIMachinePool, corev1.EventTypeWarning, "ReconcileError",
			"Instance pool has invalid lifecycle state %s", instancePool.LifecycleState)
		return reconcile.Result{}, errors.New(fmt.Sprintf("instance pool has invalid lifecycle state %s", instancePool.LifecycleState))
	}
	// we reconcile every 5 minutes in case any reconciliation have happened behind the scenes by Instancepool service on
	// the instance pool(removing unhealthy nodes etc) which has to be percolated to machinepool machines
	return reconcile.Result{RequeueAfter: 300 * time.Second}, nil
}

func (r *OCIMachinePoolReconciler) reconcileDelete(ctx context.Context, machinePoolScope *scope.MachinePoolScope) (_ ctrl.Result, reterr error) {
	machinePoolScope.Info("Handling deleted OCIMachinePool")

	// Find existing Instance Pool
	instancePool, err := machinePoolScope.FindInstancePool(ctx)
	if err != nil {
		if !ociutil.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	if instancePool == nil {
		machinePoolScope.OCIMachinePool.Status.Ready = false
		conditions.MarkFalse(machinePoolScope.OCIMachinePool, infrav2exp.InstancePoolReadyCondition, infrav2exp.InstancePoolNotFoundReason, clusterv1.ConditionSeverityWarning, "")
		machinePoolScope.Info("Instance Pool may already be deleted")
		r.Recorder.Eventf(machinePoolScope.OCIMachinePool, corev1.EventTypeNormal, infrav2exp.InstancePoolNotFoundReason, "Unable to find matching instance pool")
	} else {
		switch instancePool.LifecycleState {
		case core.InstancePoolLifecycleStateTerminating:
			machinePoolScope.Info("Instance Pool is already deleting", "displayName", instancePool.DisplayName, "id", instancePool.Id)
			// check back after 30 seconds
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		case core.InstancePoolLifecycleStateTerminated:
			// Instance Pool is already deleted
			machinePoolScope.OCIMachinePool.Status.Ready = false
			conditions.MarkFalse(machinePoolScope.OCIMachinePool, infrav2exp.InstancePoolReadyCondition, infrav2exp.InstancePoolDeletionInProgress, clusterv1.ConditionSeverityWarning, "")
			r.Recorder.Eventf(machinePoolScope.OCIMachinePool, corev1.EventTypeWarning, "DeletionInProgress", "Instance Pool deletion in progress: %s - %s", instancePool.DisplayName, instancePool.Id)
			machinePoolScope.Info("Instance Pool is already deleted", "displayName", instancePool.DisplayName, "id", instancePool.Id)
		default:
			if err := machinePoolScope.TerminateInstancePool(ctx, instancePool); err != nil {
				r.Recorder.Eventf(machinePoolScope.OCIMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete instance pool %q: %v", instancePool.Id, err)
				return ctrl.Result{}, errors.Wrap(err, "failed to delete instance pool")
			} else {
				// check back after 30 seconds
				return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}
		}
	}

	err = machinePoolScope.CleanupInstanceConfiguration(ctx, nil)
	if err != nil {
		return ctrl.Result{}, err
	}
	instanceConfiguration, err := machinePoolScope.GetInstanceConfiguration(ctx)
	if err != nil {
		if !ociutil.IsNotFound(err) {
			return reconcile.Result{}, err
		}
	}
	if instanceConfiguration != nil {
		instanceConfigurationId := instanceConfiguration.Id
		machinePoolScope.Info("deleting instance configuration", "id", *instanceConfigurationId)
		req := core.DeleteInstanceConfigurationRequest{InstanceConfigurationId: instanceConfigurationId}
		if _, err := machinePoolScope.ComputeManagementClient.DeleteInstanceConfiguration(ctx, req); err != nil {
			r.Recorder.Eventf(machinePoolScope.OCIMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete instance configuration %q: %v", instanceConfigurationId, err)
			return ctrl.Result{}, errors.Wrap(err, "failed to delete instance configuration")
		}
	}
	machinePoolScope.Info("successfully deleted instance pool and Launch Template")
	// remove finalizer
	controllerutil.RemoveFinalizer(machinePoolScope.OCIMachinePool, infrav2exp.MachinePoolFinalizer)
	return ctrl.Result{}, nil
}

func (r *OCIMachinePoolReconciler) reconcileMachines(ctx context.Context, err error, machinePoolScope *scope.MachinePoolScope, specInfraMachines []infrav2exp.OCIMachinePoolMachine) error {
	params := cloudutil.MachineParams{
		Cluster:              machinePoolScope.Cluster,
		MachinePool:          machinePoolScope.MachinePool,
		InfraMachinePoolName: machinePoolScope.OCIMachinePool.Name,
		Namespace:            machinePoolScope.OCIMachinePool.Namespace,
		SpecInfraMachines:    specInfraMachines,
		Client:               r.Client,
		Logger:               machinePoolScope.Logger,
		InfraMachinePoolKind: machinePoolScope.OCIMachinePool.Kind,
		InfraMachinePoolUID:  machinePoolScope.OCIMachinePool.UID,
	}
	err = cloudutil.CreateMachinePoolMachinesIfNotExists(ctx, params)
	if err != nil {
		r.Recorder.Event(machinePoolScope.OCIMachinePool, corev1.EventTypeWarning, "FailedToCreateNewMachines", err.Error())
		conditions.MarkFalse(machinePoolScope.OCIMachinePool, clusterv1.ReadyCondition, "FailedToCreateNewMachines", clusterv1.ConditionSeverityWarning, "")
		return errors.Wrap(err, "failed to create missing machines")
	}

	err = cloudutil.DeleteOrphanedMachinePoolMachines(ctx, params)
	if err != nil {
		conditions.MarkFalse(machinePoolScope.OCIMachinePool, clusterv1.ReadyCondition, "FailedToDeleteOrphanedMachines", clusterv1.ConditionSeverityWarning, "")
		return errors.Wrap(err, "failed to delete orphaned machines")
	}
	return nil
}
