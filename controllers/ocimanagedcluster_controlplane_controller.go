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
	"github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
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

// OCIManagedClusterControlPlaneReconciler reconciles a OCIManagedControlPlane object
type OCIManagedClusterControlPlaneReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	Region         string
	ClientProvider *scope.ClientProvider
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedcontrolplanes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the machine closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *OCIManagedClusterControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)
	logger = logger.WithValues(scope.OCIManagedClusterKind, req.NamespacedName)

	logger.Info("Inside managed control plane reconciler")

	// Fetch the OCI managed control plane
	controlPlane := &infrastructurev1beta2.OCIManagedControlPlane{}
	err := r.Get(ctx, req.NamespacedName, controlPlane)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, controlPlane.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		r.Recorder.Eventf(controlPlane, corev1.EventTypeNormal, "OwnerRefNotSet", "Cluster Controller has not yet set OwnerRef")
		logger.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, controlPlane) {
		r.Recorder.Eventf(controlPlane, corev1.EventTypeNormal, "ClusterPaused", "Cluster is paused")
		logger.Info("OCIManagedCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	ociManagedCluster := &infrastructurev1beta2.OCIManagedCluster{}
	ociClusterName := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	if err := r.Client.Get(ctx, ociClusterName, ociManagedCluster); err != nil {
		logger.Info("Cluster is not available yet")
		r.Recorder.Eventf(controlPlane, corev1.EventTypeWarning, "ClusterNotAvailable", "Cluster is not available yet")
		return ctrl.Result{}, nil
	}

	if !ociManagedCluster.Status.Ready {
		logger.Info("Cluster infrastructure is not ready")
		r.Recorder.Eventf(controlPlane, corev1.EventTypeWarning, "ClusterInfrastructureNotReady", "Cluster infrastructure is not ready")
		return ctrl.Result{}, nil
	}

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, ociManagedCluster) {
		r.Recorder.Eventf(controlPlane, corev1.EventTypeNormal, "ClusterPaused", "Cluster is paused")
		logger.Info("OCIManagedCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	clusterAccessor := scope.OCIManagedCluster{
		OCIManagedCluster: ociManagedCluster,
	}
	clientProvider, clusterRegion, clients, err := cloudutil.InitClientsAndRegion(ctx, r.Client, r.Region, clusterAccessor, r.ClientProvider)
	if err != nil {
		return ctrl.Result{}, err
	}

	helper, err := patch.NewHelper(controlPlane, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}

	// Always close the scope when exiting this function so we can persist any OCIManagedCluster changes.
	defer func() {
		logger.Info("Closing control plane scope")
		conditions.SetSummary(controlPlane)

		if err := helper.Patch(ctx, controlPlane); err != nil && reterr == nil {
			reterr = err
		}
	}()

	var controlPlaneScope *scope.ManagedControlPlaneScope

	controlPlaneScope, err = scope.NewManagedControlPlaneScope(scope.ManagedControlPlaneScopeParams{
		Client:                 r.Client,
		Logger:                 &logger,
		Cluster:                cluster,
		OCIClusterAccessor:     clusterAccessor,
		ClientProvider:         clientProvider,
		ContainerEngineClient:  clients.ContainerEngineClient,
		RegionIdentifier:       clusterRegion,
		OCIManagedControlPlane: controlPlane,
		BaseClient:             clients.BaseClient,
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	// Handle deleted clusters
	if !controlPlane.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, controlPlaneScope, controlPlane)
	}

	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	} else {
		return r.reconcile(ctx, controlPlaneScope, controlPlane)
	}

}

func (r *OCIManagedClusterControlPlaneReconciler) reconcile(ctx context.Context, controlPlaneScope *scope.ManagedControlPlaneScope, controlPlane *infrastructurev1beta2.OCIManagedControlPlane) (ctrl.Result, error) {
	// If the OCIManagedControlPlane doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(controlPlane, infrastructurev1beta2.ControlPlaneFinalizer)

	okeControlPlane, err := controlPlaneScope.GetOrCreateControlPlane(ctx)
	if err != nil {
		r.Recorder.Event(controlPlane, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "Failed to reconcile OCIManagedcontrolPlane").Error())
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile OCI Managed Control Plane %s/%s", controlPlane.Namespace, controlPlaneScope.GetClusterName())
	}

	// Proceed to reconcile the DOMachine state.
	switch okeControlPlane.LifecycleState {
	case containerengine.ClusterLifecycleStateCreating:
		controlPlaneScope.Info("Managed control plane is pending")
		conditions.MarkFalse(controlPlane, infrastructurev1beta2.ControlPlaneReadyCondition, infrastructurev1beta2.ControlPlaneNotReadyReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case containerengine.ClusterLifecycleStateUpdating:
		controlPlaneScope.Info("Managed control plane is updating")
		r.Recorder.Eventf(controlPlane, corev1.EventTypeNormal, "ControlPlaneUpdating",
			"Managed control plane is in updating state")
		conditions.MarkTrue(controlPlane, infrastructurev1beta2.ControlPlaneReadyCondition)
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case containerengine.ClusterLifecycleStateActive:
		controlPlaneScope.Info("Managed control plane is active", "endpoints", okeControlPlane.Endpoints)
		if controlPlaneScope.IsControlPlaneEndpointSubnetPublic() {
			controlPlane.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
				Host: *okeControlPlane.Endpoints.PublicEndpoint,
				Port: 6443,
			}
		} else {
			controlPlane.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
				Host: *okeControlPlane.Endpoints.PrivateEndpoint,
				Port: 6443,
			}
		}
		controlPlane.Status.Version = okeControlPlane.KubernetesVersion
		// record the event only when machine goes from not ready to ready state
		r.Recorder.Eventf(controlPlane, corev1.EventTypeNormal, "ControlPlaneReady",
			"Managed control plane is in ready state")
		conditions.MarkTrue(controlPlane, infrastructurev1beta2.ControlPlaneReadyCondition)

		controlPlaneScope.OCIManagedControlPlane.Status.Ready = true
		err := controlPlaneScope.ReconcileKubeconfig(ctx, okeControlPlane)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = controlPlaneScope.ReconcileBootstrapSecret(ctx, okeControlPlane)
		if err != nil {
			return ctrl.Result{}, err
		}
		isUpdated, err := controlPlaneScope.UpdateControlPlane(ctx, okeControlPlane)
		if err != nil {
			return ctrl.Result{}, err
		}
		if isUpdated {
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
		err = controlPlaneScope.ReconcileAddons(ctx, okeControlPlane)
		if err != nil {
			return ctrl.Result{}, err
		}
		return reconcile.Result{RequeueAfter: 180 * time.Second}, nil
	default:
		conditions.MarkFalse(controlPlane, infrastructurev1beta2.ControlPlaneReadyCondition, infrastructurev1beta2.ControlPlaneProvisionFailedReason, clusterv1.ConditionSeverityError, "")
		r.Recorder.Eventf(controlPlane, corev1.EventTypeWarning, "ReconcileError",
			"Managed control plane has invalid lifecycle state %s", okeControlPlane.LifecycleState)
		return reconcile.Result{}, errors.New(fmt.Sprintf("Control plane has invalid lifecycle state %s", okeControlPlane.LifecycleState))
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCIManagedClusterControlPlaneReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)
	ociManagedClusterMapper, err := OCIManagedClusterToOCIManagedControlPlaneMapper(r.Client, log)
	if err != nil {
		return err
	}
	err = ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrastructurev1beta2.OCIManagedControlPlane{}).
		Watches(
			&infrastructurev1beta2.OCIManagedCluster{},
			handler.EnqueueRequestsFromMapFunc(ociManagedClusterMapper),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(ClusterToOCIManagedControlPlaneMapper()),
			builder.WithPredicates(
				predicates.ClusterUnpaused(mgr.GetScheme(), log),
				predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), log, ""),
			),
		).
		Complete(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	return nil
}

// ClusterToInfrastructureMapFunc returns a handler.ToRequestsFunc that watches for
// Cluster events and returns reconciliation requests for an infrastructure provider object.
func (r *OCIManagedClusterControlPlaneReconciler) clusterToInfrastructureMapFunc(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		c, ok := o.(*clusterv1.Cluster)
		if !ok {
			return nil
		}

		// Make sure the ref is set
		if c.Spec.InfrastructureRef == nil {
			log.V(4).Info("Cluster does not have an InfrastructureRef, skipping mapping.")
			return nil
		}

		if c.Spec.InfrastructureRef.GroupVersionKind().Kind != "OCIManagedCluster" {
			log.V(4).Info("Cluster has an InfrastructureRef for a different type, skipping mapping.")
			return nil
		}

		ociCluster := &infrastructurev1beta2.OCIManagedCluster{}
		key := types.NamespacedName{Namespace: c.Spec.InfrastructureRef.Namespace, Name: c.Spec.InfrastructureRef.Name}

		if err := r.Get(ctx, key, ociCluster); err != nil {
			log.V(4).Error(err, "Failed to get OCI cluster")
			return nil
		}

		if annotations.IsExternallyManaged(ociCluster) {
			log.V(4).Info("OCIManagedCluster is externally managed, skipping mapping.")
			return nil
		}

		log.V(4).Info("Adding request.", "ociCluster", c.Spec.InfrastructureRef.Name)

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: c.Namespace,
					Name:      c.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}

func (r *OCIManagedClusterControlPlaneReconciler) reconcileDelete(ctx context.Context,
	controlPlaneScope *scope.ManagedControlPlaneScope, controlPlane *infrastructurev1beta2.OCIManagedControlPlane) (ctrl.Result, error) {
	controlPlaneScope.Info("Handling deleted OCiManagedControlPlane")

	cluster, err := controlPlaneScope.GetOKECluster(ctx)
	if err != nil {
		if ociutil.IsNotFound(err) {
			controllerutil.RemoveFinalizer(controlPlaneScope.OCIManagedControlPlane, infrastructurev1beta2.ControlPlaneFinalizer)
			controlPlaneScope.Info("Cluster is not found, may have been deleted")
			conditions.MarkTrue(controlPlaneScope.OCIManagedControlPlane, infrastructurev1beta2.ControlPlaneNotFoundReason)
			controlPlaneScope.OCIManagedControlPlane.Status.Ready = false
			return reconcile.Result{}, nil
		} else {
			return reconcile.Result{}, err
		}
	}
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		controlPlaneScope.Info("Cluster is not found, may have been deleted")
		controllerutil.RemoveFinalizer(controlPlane, infrastructurev1beta2.ControlPlaneFinalizer)
		return reconcile.Result{}, nil
	}

	switch cluster.LifecycleState {
	case containerengine.ClusterLifecycleStateDeleting:
		controlPlane.Status.Ready = false
		conditions.MarkFalse(controlPlane, infrastructurev1beta2.ControlPlaneReadyCondition, infrastructurev1beta2.ControlPlaneDeletionInProgress, clusterv1.ConditionSeverityWarning, "")
		r.Recorder.Eventf(controlPlane, corev1.EventTypeWarning, "DeletionInProgress", "Managed control plane deletion in progress")
		controlPlaneScope.Info("Managed control plane is deleting")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case containerengine.ClusterLifecycleStateDeleted:
		conditions.MarkFalse(controlPlane, infrastructurev1beta2.ControlPlaneReadyCondition, infrastructurev1beta2.ControlPlaneDeletedReason, clusterv1.ConditionSeverityWarning, "")
		controllerutil.RemoveFinalizer(controlPlane, infrastructurev1beta2.ControlPlaneFinalizer)
		controlPlaneScope.Info("Managed control plane is deleted")
		return reconcile.Result{}, nil
	default:
		if err := controlPlaneScope.DeleteOKECluster(ctx, cluster); err != nil {
			controlPlaneScope.Error(err, "Error deleting managed control plane")
			return ctrl.Result{}, errors.Wrapf(err, "error deleting cluster %s", controlPlaneScope.GetClusterName())
		}
		conditions.MarkFalse(controlPlane, infrastructurev1beta2.ControlPlaneReadyCondition, infrastructurev1beta2.ControlPlaneDeletionInProgress, clusterv1.ConditionSeverityWarning, "")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}
}

func OCIManagedClusterToOCIManagedControlPlaneMapper(c client.Client, log logr.Logger) (handler.MapFunc, error) {
	return func(ctx context.Context, o client.Object) []ctrl.Request {
		ociCluster, ok := o.(*infrastructurev1beta2.OCIManagedCluster)
		if !ok {
			log.Error(errors.Errorf("expected an OCIManagedCluster, got %T instead", o), "failed to map OCIManagedCluster")
			return nil
		}

		log = log.WithValues("OCIManagedCluster", ociCluster.Name, "Namespace", ociCluster.Namespace)

		// Don't handle deleted OCIManagedClusters
		if !ociCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			log.V(4).Info("OCIManagedCluster has a deletion timestamp, skipping mapping.")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, c, ociCluster.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to get the owning cluster")
			return nil
		}

		if cluster == nil {
			log.Error(err, "cluster has not set owner ref yet")
			return nil
		}

		ref := cluster.Spec.ControlPlaneRef
		if ref == nil || ref.Name == "" {
			return nil
		}

		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: ref.Namespace,
					Name:      ref.Name,
				},
			},
		}
	}, nil
}

func ClusterToOCIManagedControlPlaneMapper() handler.MapFunc {
	return func(ctx context.Context, o client.Object) []ctrl.Request {
		cluster, ok := o.(*clusterv1.Cluster)
		if !ok {
			return nil
		}

		ref := cluster.Spec.ControlPlaneRef
		if ref == nil || ref.Name == "" {
			return nil
		}

		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: ref.Namespace,
					Name:      ref.Name,
				},
			},
		}
	}
}
