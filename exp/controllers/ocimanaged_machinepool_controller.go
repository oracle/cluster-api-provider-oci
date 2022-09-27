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
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	infrav1exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// OCIManagedMachinePoolReconciler reconciles a OCIManagedMachinePool object
type OCIManagedMachinePoolReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ClientProvider *scope.ClientProvider
	Region         string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedmachinepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedmachinepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedmachinepools/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the machinepool closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *OCIManagedMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)
	logger.Info("Got reconciliation event for managedmachine pool")

	// Fetch the OCIManagedMachinePool.
	ociManagedMachinePool := &infrav1exp.OCIManagedMachinePool{}
	err := r.Get(ctx, req.NamespacedName, ociManagedMachinePool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the CAPI MachinePool
	machinePool, err := getOwnerMachinePool(ctx, r.Client, ociManagedMachinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machinePool == nil {
		r.Recorder.Eventf(ociManagedMachinePool, corev1.EventTypeNormal, "OwnerRefNotSet", "Cluster Controller has not yet set OwnerRef")
		logger.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}
	logger = logger.WithValues("machinePool", machinePool.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, ociManagedMachinePool.ObjectMeta)
	if err != nil {
		r.Recorder.Eventf(ociManagedMachinePool, corev1.EventTypeWarning, "ClusterDoesNotExist", "MachinePool is missing cluster label or cluster does not exist")
		logger.Info("MachinePool is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}
	logger = logger.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, ociManagedMachinePool) {
		logger.Info("OCIMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	ociManagedCluster := &infrav1exp.OCIManagedCluster{}
	ociClusterName := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}

	if err := r.Client.Get(ctx, ociClusterName, ociManagedCluster); err != nil {
		logger.Info("Cluster is not available yet")
		r.Recorder.Eventf(ociManagedMachinePool, corev1.EventTypeWarning, "ClusterNotAvailable", "Cluster is not available yet")
		logger.V(2).Info("OCICluster is not available yet")
		return ctrl.Result{}, nil
	}

	regionOverride := r.Region
	if len(ociManagedCluster.Spec.Region) > 0 {
		regionOverride = ociManagedCluster.Spec.Region
	}
	if len(regionOverride) <= 0 {
		return ctrl.Result{}, errors.New("OCIMachinePoolReconciler RegionIdentifier can't be nil")
	}

	clients, err := r.ClientProvider.GetOrBuildClient(regionOverride)
	if err != nil {
		logger.Error(err, "Couldn't get the clients for region")
	}

	controlPlane := &infrav1exp.OCIManagedControlPlane{}
	controlPlaneRef := types.NamespacedName{
		Name:      cluster.Spec.ControlPlaneRef.Name,
		Namespace: cluster.Namespace,
	}

	if err := r.Get(ctx, controlPlaneRef, controlPlane); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get control plane ref")
	}

	// Create the machine pool scope
	machinePoolScope, err := scope.NewManagedMachinePoolScope(scope.ManagedMachinePoolScopeParams{
		Client:                  r.Client,
		ComputeManagementClient: clients.ComputeManagementClient,
		Logger:                  &logger,
		Cluster:                 cluster,
		OCIManagedCluster:       ociManagedCluster,
		MachinePool:             machinePool,
		OCIManagedMachinePool:   ociManagedMachinePool,
		ContainerEngineClient:   clients.ContainerEngineClient,
		OCIManagedControlPlane:  controlPlane,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		if err := machinePoolScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !ociManagedMachinePool.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machinePoolScope)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, logger, machinePoolScope)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCIManagedMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	logger := log.FromContext(ctx)
	gvk, err := apiutil.GVKForObject(new(infrav1exp.OCIManagedMachinePool), mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to find GVK for OCIManagedMachinePool")
	}
	managedControlPlaneToManagedMachinePoolMap := managedClusterToManagedMachinePoolMapFunc(r.Client, gvk, logger)
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1exp.OCIManagedMachinePool{}).
		Watches(
			&source.Kind{Type: &expclusterv1.MachinePool{}},
			handler.EnqueueRequestsFromMapFunc(machinePoolToInfrastructureMapFunc(infrastructurev1beta1.
				GroupVersion.WithKind(scope.OCIManagedMachinePoolKind), logger)),
		).
		Watches(
			&source.Kind{Type: &infrav1exp.OCIManagedCluster{}},
			handler.EnqueueRequestsFromMapFunc(managedControlPlaneToManagedMachinePoolMap),
		).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(ctx))).
		Complete(r)
}

func managedClusterToManagedMachinePoolMapFunc(c client.Client, gvk schema.GroupVersionKind, log logr.Logger) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		ctx := context.Background()
		ociCluster, ok := o.(*infrav1exp.OCIManagedCluster)
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
			ctx, &managedPoolForClusterList, client.InNamespace(cluster.Namespace), client.MatchingLabels{clusterv1.ClusterLabelName: cluster.Name},
		); err != nil {
			log.Error(err, "couldn't list pools for cluster")
			return nil
		}

		mapFunc := machinePoolToInfrastructureMapFunc(gvk, log)

		var results []ctrl.Request
		for i := range managedPoolForClusterList.Items {
			managedPool := mapFunc(&managedPoolForClusterList.Items[i])
			results = append(results, managedPool...)
		}

		return results
	}
}

func (r *OCIManagedMachinePoolReconciler) reconcileNormal(ctx context.Context, logger logr.Logger, machinePoolScope *scope.ManagedMachinePoolScope) (ctrl.Result, error) {
	machinePoolScope.Info("Handling reconcile OCIMachinePool")
	// If the OCIMachinePool doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machinePoolScope.OCIManagedMachinePool, infrav1exp.ManagedMachinePoolFinalizer)

	if !machinePoolScope.Cluster.Status.InfrastructureReady {
		logger.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	if !machinePoolScope.OCIManagedControlPlane.Status.Ready {
		logger.Info("Cluster control plane is not ready yet")
		return reconcile.Result{}, nil
	}

	// Find existing Node Pool
	nodePool, err := machinePoolScope.FindNodePool(ctx)
	if err != nil {
		conditions.MarkUnknown(machinePoolScope.OCIManagedMachinePool, infrav1exp.NodePoolReadyCondition, infrav1exp.NodePoolNotFoundReason, err.Error())
		return ctrl.Result{}, err
	}

	if nodePool == nil {
		if nodePool, err = machinePoolScope.CreateNodePool(ctx); err != nil {
			conditions.MarkFalse(machinePoolScope.OCIManagedMachinePool, infrav1exp.NodePoolReadyCondition, infrav1exp.NodePoolProvisionFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return ctrl.Result{}, err
		}
		// record the event only when node pool is created
		r.Recorder.Eventf(machinePoolScope.OCIManagedMachinePool, corev1.EventTypeNormal, "NodePool",
			"Created new Node Pool: %s", machinePoolScope.OCIManagedMachinePool.GetName())
	}

	machinePoolScope.Info("Node Pool found", "NodePoolId", *nodePool.Id)
	machinePoolScope.OCIManagedMachinePool.Spec.ProviderID = common.String(fmt.Sprintf("oci://%s", *nodePool.Id))
	machinePoolScope.OCIManagedMachinePool.Spec.ID = nodePool.Id

	failureMessages := make([]string, 0)
	for _, node := range nodePool.Nodes {
		if node.NodeError != nil {
			failureMessages = append(failureMessages, *node.NodeError.Message)
		}
	}
	if len(failureMessages) > 0 {
		machinePoolScope.OCIManagedMachinePool.Status.FailureMessages = failureMessages
	}

	switch nodePool.LifecycleState {
	case oke.NodePoolLifecycleStateCreating:
		machinePoolScope.Info("Node Pool is creating")
		conditions.MarkFalse(machinePoolScope.OCIManagedMachinePool, infrav1exp.NodePoolReadyCondition, infrav1exp.NodePoolNotReadyReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case oke.NodePoolLifecycleStateUpdating:
		machinePoolScope.Info("Node Pool is updating")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case oke.NodePoolLifecycleStateActive:
		machinePoolScope.Info("Node pool is active")
		instanceCount, err := machinePoolScope.SetListandSetMachinePoolInstances(ctx, nodePool)
		if err != nil {
			return reconcile.Result{}, err
		}
		machinePoolScope.SetReplicaCount(instanceCount)
		machinePoolScope.OCIManagedMachinePool.Status.Ready = true
		// record the event only when pool goes from not ready to ready state
		r.Recorder.Eventf(machinePoolScope.OCIManagedMachinePool, corev1.EventTypeNormal, "NodePoolReady",
			"Node pool is in ready state")
		conditions.MarkTrue(machinePoolScope.OCIManagedMachinePool, infrav1exp.NodePoolReadyCondition)
		isUpdated, err := machinePoolScope.UpdateNodePool(ctx, nodePool)
		if err != nil {
			return reconcile.Result{}, err
		}
		if isUpdated {
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}

		return reconcile.Result{}, nil
	default:
		err := errors.Errorf("Node Pool status %s is unexpected", nodePool.LifecycleState)
		machinePoolScope.OCIManagedMachinePool.Status.FailureMessages = append(machinePoolScope.OCIManagedMachinePool.Status.FailureMessages, err.Error())
		conditions.MarkFalse(machinePoolScope.OCIManagedMachinePool, infrav1exp.NodePoolReadyCondition, infrav1exp.NodePoolProvisionFailedReason, clusterv1.ConditionSeverityError, "")
		r.Recorder.Eventf(machinePoolScope.OCIManagedMachinePool, corev1.EventTypeWarning, "ReconcileError",
			"Node pool has invalid lifecycle state %s, lifecycle details is %s", nodePool.LifecycleState, nodePool.LifecycleDetails)
		return reconcile.Result{}, err
	}
}

func (r *OCIManagedMachinePoolReconciler) reconcileDelete(ctx context.Context, machinePoolScope *scope.ManagedMachinePoolScope) (_ ctrl.Result, reterr error) {
	machinePoolScope.Info("Handling deleted OCIMachinePool")
	machinePool := machinePoolScope.OCIManagedMachinePool
	// Find existing Node Pool
	nodePool, err := machinePoolScope.FindNodePool(ctx)
	if err != nil {
		if ociutil.IsNotFound(err) {
			controllerutil.RemoveFinalizer(machinePoolScope.OCIManagedMachinePool, infrav1exp.ManagedMachinePoolFinalizer)
			machinePoolScope.Info("Node pool not found, may have been deleted")
			conditions.MarkTrue(machinePoolScope.OCIManagedMachinePool, infrav1exp.NodePoolNotFoundReason)
			machinePoolScope.OCIManagedMachinePool.Status.Ready = false
			return reconcile.Result{}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	switch nodePool.LifecycleState {
	case oke.NodePoolLifecycleStateDeleting:
		// Node Pool is already deleting
		machinePool.Status.Ready = false
		conditions.MarkFalse(machinePool, infrav1exp.NodePoolReadyCondition, infrav1exp.NodePoolDeletionInProgress, clusterv1.ConditionSeverityWarning, "")
		r.Recorder.Eventf(machinePool, corev1.EventTypeWarning, "DeletionInProgress", "Node Pool deletion in progress")
		machinePoolScope.Info("Node Pool is deleting")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case oke.NodePoolLifecycleStateDeleted:
		controllerutil.RemoveFinalizer(machinePoolScope.OCIManagedMachinePool, infrav1exp.ManagedMachinePoolFinalizer)
		conditions.MarkFalse(machinePool, infrav1exp.NodePoolReadyCondition, infrav1exp.NodePoolDeletedReason, clusterv1.ConditionSeverityWarning, "")
		machinePoolScope.Info("Node Pool is already deleted")
		return reconcile.Result{}, nil
	default:
		err = machinePoolScope.DeleteNodePool(ctx, nodePool)
		if err != nil {
			machinePoolScope.Error(err, "Terminate node pool request failed")
			return ctrl.Result{}, err
		} else {
			machinePoolScope.OCIManagedMachinePool.Status.Ready = false
			conditions.MarkFalse(machinePoolScope.OCIManagedMachinePool, infrav1exp.NodePoolReadyCondition, infrav1exp.NodePoolDeletionInProgress, clusterv1.ConditionSeverityWarning, "")
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}
}