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
	"time"

	"github.com/go-logr/logr"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	cloudutil "github.com/oracle/cluster-api-provider-oci/cloud/util"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// OCIVirtualMachinePoolReconciler reconciles a OCIVirtualMachinePool object
type OCIVirtualMachinePoolReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ClientProvider *scope.ClientProvider
	Region         string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocivirtualmachinepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocivirtualmachinepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocivirtualmachinepools/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the machinepool closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile
func (r *OCIVirtualMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)
	logger.Info("Got reconciliation event for virtual machine pool")

	// Fetch the OCIVirtualMachinePool.
	ociVirtualMachinePool := &infrav2exp.OCIVirtualMachinePool{}
	err := r.Get(ctx, req.NamespacedName, ociVirtualMachinePool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the CAPI MachinePool
	machinePool, err := getOwnerMachinePool(ctx, r.Client, ociVirtualMachinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machinePool == nil {
		r.Recorder.Eventf(ociVirtualMachinePool, corev1.EventTypeNormal, "OwnerRefNotSet", "Cluster Controller has not yet set OwnerRef")
		logger.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}
	logger = logger.WithValues("machinePool", machinePool.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, ociVirtualMachinePool.ObjectMeta)
	if err != nil {
		r.Recorder.Eventf(ociVirtualMachinePool, corev1.EventTypeWarning, "ClusterDoesNotExist", "MachinePool is missing cluster label or cluster does not exist")
		logger.Info("MachinePool is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}
	logger = logger.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, ociVirtualMachinePool) {
		logger.Info("OCIMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	ociManagedCluster := &infrastructurev1beta2.OCIManagedCluster{}
	ociClusterName := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	if err := r.Client.Get(ctx, ociClusterName, ociManagedCluster); err != nil {
		logger.Info("Cluster is not available yet")
		r.Recorder.Eventf(ociVirtualMachinePool, corev1.EventTypeWarning, "ClusterNotAvailable", "Cluster is not available yet")
		logger.V(2).Info("OCICluster is not available yet")
		return ctrl.Result{}, nil
	}

	clusterAccessor := scope.OCIManagedCluster{
		OCIManagedCluster: ociManagedCluster,
	}
	_, _, clients, err := cloudutil.InitClientsAndRegion(ctx, r.Client, r.Region, clusterAccessor, r.ClientProvider)
	if err != nil {
		return ctrl.Result{}, err
	}

	controlPlane := &infrastructurev1beta2.OCIManagedControlPlane{}
	controlPlaneRef := types.NamespacedName{
		Name:      cluster.Spec.ControlPlaneRef.Name,
		Namespace: cluster.Namespace,
	}

	if err := r.Get(ctx, controlPlaneRef, controlPlane); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get control plane ref")
	}

	// Create the machine pool scope
	machinePoolScope, err := scope.NewVirtualMachinePoolScope(scope.VirtualMachinePoolScopeParams{
		Client:                  r.Client,
		ComputeManagementClient: clients.ComputeManagementClient,
		Logger:                  &logger,
		Cluster:                 cluster,
		OCIManagedCluster:       ociManagedCluster,
		MachinePool:             machinePool,
		OCIVirtualMachinePool:   ociVirtualMachinePool,
		ContainerEngineClient:   clients.ContainerEngineClient,
		OCIManagedControlPlane:  controlPlane,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any virtual machine pool changes.
	defer func() {
		if err := machinePoolScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !ociVirtualMachinePool.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machinePoolScope)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, logger, machinePoolScope)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCIVirtualMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	logger := log.FromContext(ctx)
	gvk, err := apiutil.GVKForObject(new(infrav2exp.OCIVirtualMachinePool), mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to find GVK for OCIVirtualMachinePool")
	}
	clusterToObjectFunc, err := util.ClusterToTypedObjectsMapper(r.Client, &infrav2exp.OCIVirtualMachinePoolList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to create mapper for Cluster to OCIVirtualMachinePool")
	}
	managedClusterToVirtualMachinePoolMapFunc := managedClusterToVirtualMachinePoolMapFunc(r.Client, gvk, logger)
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav2exp.OCIVirtualMachinePool{}).
		Watches(
			&expclusterv1.MachinePool{},
			handler.EnqueueRequestsFromMapFunc(machinePoolToInfrastructureMapFunc(infrastructurev1beta2.
				GroupVersion.WithKind(scope.OCIVirtualMachinePoolKind), logger)),
		).
		Watches(
			&infrastructurev1beta2.OCIManagedCluster{},
			handler.EnqueueRequestsFromMapFunc(managedClusterToVirtualMachinePoolMapFunc),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc),
			builder.WithPredicates(
				predicates.ClusterUnpausedAndInfrastructureReady(mgr.GetScheme(), ctrl.LoggerFrom(ctx)),
			),
		).
		WithEventFilter(predicates.ResourceNotPaused(mgr.GetScheme(), ctrl.LoggerFrom(ctx))).
		Complete(r)
}

func managedClusterToVirtualMachinePoolMapFunc(c client.Client, gvk schema.GroupVersionKind, log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		ociCluster, ok := o.(*infrastructurev1beta2.OCIManagedCluster)
		if !ok {
			panic(fmt.Sprintf("Expected a OCIManagedControlPlane but got a %T", o))
		}

		if !ociCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, c, ociCluster.ObjectMeta)
		if err != nil {
			log.Error(err, "couldn't get OCI control plane owner ObjectKey")
			return nil
		}
		if cluster == nil {
			return nil
		}

		managedPoolForClusterList := expclusterv1.MachinePoolList{}
		if err := c.List(
			ctx, &managedPoolForClusterList, client.InNamespace(cluster.Namespace), client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name},
		); err != nil {
			log.Error(err, "couldn't list pools for cluster")
			return nil
		}

		mapFunc := machinePoolToInfrastructureMapFunc(gvk, log)

		var results []ctrl.Request
		for i := range managedPoolForClusterList.Items {
			managedPool := mapFunc(ctx, &managedPoolForClusterList.Items[i])
			results = append(results, managedPool...)
		}

		return results
	}
}

func (r *OCIVirtualMachinePoolReconciler) reconcileNormal(ctx context.Context, logger logr.Logger, machinePoolScope *scope.VirtualMachinePoolScope) (ctrl.Result, error) {
	machinePoolScope.Info("Handling reconcile OCIVirtualMachinePool")
	// If the OCIMachinePool doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualMachinePoolFinalizer)

	if !machinePoolScope.Cluster.Status.InfrastructureReady {
		logger.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	if !machinePoolScope.OCIManagedControlPlane.Status.Ready {
		logger.Info("Cluster control plane is not ready yet")
		return reconcile.Result{}, nil
	}

	// Find existing Virtual Node Pool
	nodePool, err := machinePoolScope.FindVirtualNodePool(ctx)
	if err != nil {
		r.Recorder.Event(machinePoolScope.OCIVirtualMachinePool, corev1.EventTypeWarning, "ReconcileError", err.Error())
		conditions.MarkUnknown(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualNodePoolReadyCondition, infrav2exp.VirtualNodePoolNotFoundReason, "")
		return ctrl.Result{}, err
	}

	if nodePool == nil {
		if nodePool, err = machinePoolScope.CreateVirtualNodePool(ctx); err != nil {
			r.Recorder.Event(machinePoolScope.OCIVirtualMachinePool, corev1.EventTypeWarning, "ReconcileError", err.Error())
			conditions.MarkFalse(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualNodePoolReadyCondition, infrav2exp.VirtualNodePoolProvisionFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, err
		}
		// record the event only when node pool is created
		r.Recorder.Eventf(machinePoolScope.OCIVirtualMachinePool, corev1.EventTypeNormal, "VirtualNodePool",
			"Created new Virtual Node Pool: %s", machinePoolScope.OCIVirtualMachinePool.GetName())
	}

	machinePoolScope.Info("VirtualNode Pool found", "VirtualNodePoolId", *nodePool.Id)
	machinePoolScope.OCIVirtualMachinePool.Spec.ProviderID = common.String(fmt.Sprintf("oci://%s", *nodePool.Id))
	machinePoolScope.OCIVirtualMachinePool.Spec.ID = nodePool.Id

	machinePoolScope.OCIVirtualMachinePool.Status.NodepoolLifecycleState = fmt.Sprintf("%s", nodePool.LifecycleState)

	switch nodePool.LifecycleState {
	case oke.VirtualNodePoolLifecycleStateCreating:
		machinePoolScope.Info("Node Pool is creating")
		conditions.MarkFalse(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualNodePoolReadyCondition, infrav2exp.VirtualNodePoolNotReadyReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case oke.VirtualNodePoolLifecycleStateUpdating:
		machinePoolScope.Info("Virtual Node Pool is updating")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case oke.VirtualNodePoolLifecycleStateActive:
		machinePoolScope.Info("Virtual Node pool is active")
		machines, err := machinePoolScope.ListandSetMachinePoolInstances(ctx, nodePool)
		if err != nil {
			return reconcile.Result{}, err
		}
		providerIDList := make([]string, 0)
		for _, machine := range machines {
			if machine.Status.Ready {
				providerIDList = append(providerIDList, *machine.Spec.ProviderID)
			}
		}
		machinePoolScope.OCIVirtualMachinePool.Spec.ProviderIDList = providerIDList
		machinePoolScope.SetReplicaCount(int32(len(providerIDList)))

		err = r.reconcileVirtualMachines(ctx, err, machinePoolScope, machines)
		if err != nil {
			return reconcile.Result{}, err
		}
		machinePoolScope.OCIVirtualMachinePool.Status.Ready = true
		// record the event only when pool goes from not ready to ready state
		r.Recorder.Eventf(machinePoolScope.OCIVirtualMachinePool, corev1.EventTypeNormal, "VirtualNodePoolReady",
			"Node pool is in ready state")
		conditions.MarkTrue(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualNodePoolReadyCondition)
		isUpdated, err := machinePoolScope.UpdateVirtualNodePool(ctx, nodePool)
		if err != nil {
			return reconcile.Result{}, err
		}
		if isUpdated {
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}

		// we reconcile every 5 minutes in case any reconciliation have happened behind the scenes by OKE service on
		// the virtual node pool(removing unhealthy nodes etc) which has to be percolated to machinepool machines
		return reconcile.Result{RequeueAfter: 300 * time.Second}, nil
	default:
		err := errors.Errorf("Virtual Node Pool status %s is unexpected", nodePool.LifecycleState)
		machinePoolScope.OCIVirtualMachinePool.Status.FailureMessages = append(machinePoolScope.OCIVirtualMachinePool.Status.FailureMessages, err.Error())
		conditions.MarkFalse(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualNodePoolReadyCondition, infrav2exp.VirtualNodePoolProvisionFailedReason, clusterv1.ConditionSeverityError, "")
		r.Recorder.Eventf(machinePoolScope.OCIVirtualMachinePool, corev1.EventTypeWarning, "ReconcileError",
			"Virtual Node pool has invalid lifecycle state %s, lifecycle details is %s", nodePool.LifecycleState, nodePool.LifecycleDetails)
		return reconcile.Result{}, err
	}
}

func (r *OCIVirtualMachinePoolReconciler) reconcileDelete(ctx context.Context, machinePoolScope *scope.VirtualMachinePoolScope) (_ ctrl.Result, reterr error) {
	machinePoolScope.Info("Handling deleted OCIMachinePool")
	machinePool := machinePoolScope.OCIVirtualMachinePool
	// Find existing Virtual Node Pool
	nodePool, err := machinePoolScope.FindVirtualNodePool(ctx)
	if err != nil {
		if ociutil.IsNotFound(err) {
			controllerutil.RemoveFinalizer(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualMachinePoolFinalizer)
			machinePoolScope.Info("Node pool not found, may have been deleted")
			conditions.MarkTrue(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualNodePoolNotFoundReason)
			machinePoolScope.OCIVirtualMachinePool.Status.Ready = false
			return reconcile.Result{}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	if nodePool == nil {
		machinePoolScope.Info("Node Pool is not found, may have been deleted")
		controllerutil.RemoveFinalizer(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualMachinePoolFinalizer)
		conditions.MarkFalse(machinePool, infrav2exp.VirtualNodePoolReadyCondition, infrav2exp.VirtualNodePoolDeletedReason, clusterv1.ConditionSeverityWarning, "")
		return reconcile.Result{}, nil
	}

	machinePoolScope.OCIVirtualMachinePool.Status.NodepoolLifecycleState = fmt.Sprintf("%s", nodePool.LifecycleState)
	machinePoolScope.Info(fmt.Sprintf("Node Pool lifecycle state is %v", nodePool.LifecycleState))
	switch nodePool.LifecycleState {
	case oke.VirtualNodePoolLifecycleStateDeleting:
		// Node Pool is already deleting
		machinePool.Status.Ready = false
		conditions.MarkFalse(machinePool, infrav2exp.VirtualNodePoolReadyCondition, infrav2exp.VirtualNodePoolDeletionInProgress, clusterv1.ConditionSeverityWarning, "")
		r.Recorder.Eventf(machinePool, corev1.EventTypeWarning, "DeletionInProgress", "Virtual Node Pool deletion in progress")
		machinePoolScope.Info("Node Pool is deleting")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case oke.VirtualNodePoolLifecycleStateDeleted:
		controllerutil.RemoveFinalizer(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualMachinePoolFinalizer)
		conditions.MarkFalse(machinePool, infrav2exp.VirtualNodePoolReadyCondition, infrav2exp.VirtualNodePoolDeletedReason, clusterv1.ConditionSeverityWarning, "")
		machinePoolScope.Info("Virtual Node Pool is already deleted")
		return reconcile.Result{}, nil
	default:
		err = machinePoolScope.DeleteVirtualNodePool(ctx, nodePool)
		if err != nil {
			machinePoolScope.Error(err, "Terminate virtual node pool request failed")
			return ctrl.Result{}, err
		} else {
			machinePoolScope.OCIVirtualMachinePool.Status.Ready = false
			conditions.MarkFalse(machinePoolScope.OCIVirtualMachinePool, infrav2exp.VirtualNodePoolReadyCondition, infrav2exp.VirtualNodePoolDeletionInProgress, clusterv1.ConditionSeverityWarning, "")
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}
}

func (r *OCIVirtualMachinePoolReconciler) reconcileVirtualMachines(ctx context.Context, err error, machinePoolScope *scope.VirtualMachinePoolScope, specInfraMachines []infrav2exp.OCIMachinePoolMachine) error {
	params := cloudutil.MachineParams{
		Cluster:              machinePoolScope.Cluster,
		MachinePool:          machinePoolScope.MachinePool,
		InfraMachinePoolName: machinePoolScope.OCIVirtualMachinePool.Name,
		Namespace:            machinePoolScope.OCIVirtualMachinePool.Namespace,
		SpecInfraMachines:    specInfraMachines,
		Client:               r.Client,
		Logger:               machinePoolScope.Logger,
		InfraMachinePoolKind: machinePoolScope.OCIVirtualMachinePool.Kind,
		InfraMachinePoolUID:  machinePoolScope.OCIVirtualMachinePool.UID,
	}
	err = cloudutil.CreateMachinePoolMachinesIfNotExists(ctx, params)
	if err != nil {
		r.Recorder.Event(machinePoolScope.OCIVirtualMachinePool, corev1.EventTypeWarning, "FailedToCreateNewMachines", err.Error())
		conditions.MarkFalse(machinePoolScope.OCIVirtualMachinePool, clusterv1.ReadyCondition, "FailedToCreateNewMachines", clusterv1.ConditionSeverityWarning, "")
		return errors.Wrap(err, "failed to create missing machines")
	}

	err = cloudutil.DeleteOrphanedMachinePoolMachines(ctx, params)
	if err != nil {
		r.Recorder.Event(machinePoolScope.OCIVirtualMachinePool, corev1.EventTypeWarning, "FailedToDeleteOrphanedMachines", err.Error())
		conditions.MarkFalse(machinePoolScope.OCIVirtualMachinePool, clusterv1.ReadyCondition, "FailedToDeleteOrphanedMachines", clusterv1.ConditionSeverityWarning, "")
		return errors.Wrap(err, "failed to delete orphaned machines")
	}
	return nil
}
