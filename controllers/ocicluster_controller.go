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
	"strings"

	"github.com/go-logr/logr"
	"github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	cloudutil "github.com/oracle/cluster-api-provider-oci/cloud/util"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// OCIClusterReconciler reconciles a OciCluster object
type OCIClusterReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	Region         string
	ClientProvider *scope.ClientProvider
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ociclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ociclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ociclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the machine closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *OCIClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)
	logger = logger.WithValues(scope.OCIClusterKind, req.NamespacedName)

	logger.Info("Inside cluster reconciler")

	// Fetch the OCICluster instance
	ociCluster := &infrastructurev1beta1.OCICluster{}
	err := r.Get(ctx, req.NamespacedName, ociCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, ociCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		r.Recorder.Eventf(ociCluster, corev1.EventTypeNormal, "OwnerRefNotSet", "Cluster Controller has not yet set OwnerRef")
		logger.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, ociCluster) {
		r.Recorder.Eventf(ociCluster, corev1.EventTypeNormal, "ClusterPaused", "Cluster is paused")
		logger.Info("OCICluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	var clusterScope scope.ClusterScopeClient
	var clients scope.OCIClients
	var clientProvider *scope.ClientProvider
	clusterAccessor := scope.OCISelfManagedCluster{
		OCICluster: ociCluster,
	}
	clientProvider, clusterRegion, clients, err := cloudutil.InitClientsAndRegion(ctx, r.Client, r.Region, clusterAccessor, r.ClientProvider)
	if err != nil {
		return ctrl.Result{}, err
	}
	helper, err := patch.NewHelper(ociCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	clusterScope, err = scope.NewClusterScope(scope.ClusterScopeParams{
		Client:             r.Client,
		Logger:             &logger,
		Cluster:            cluster,
		OCIClusterAccessor: clusterAccessor,
		ClientProvider:     clientProvider,
		VCNClient:          clients.VCNClient,
		LoadBalancerClient: clients.LoadBalancerClient,
		IdentityClient:     clients.IdentityClient,
		RegionIdentifier:   clusterRegion,
	})
	if err != nil {
		logger.Error(err, "Couldn't create cluster scope")
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function so we can persist any OCICluster changes.
	defer func() {
		logger.Info("Closing cluster scope")
		conditions.SetSummary(ociCluster)

		if err := helper.Patch(ctx, ociCluster); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !ociCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, logger, clusterScope, ociCluster)
	}

	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	} else {
		return r.reconcile(ctx, logger, clusterScope, ociCluster)
	}

}

func (r *OCIClusterReconciler) reconcileComponent(ctx context.Context, cluster *v1beta1.OCICluster,
	reconciler func(context.Context) error,
	componentName string, failReason string, readyEventtype string) error {

	err := reconciler(ctx)
	if err != nil {
		r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err,
			fmt.Sprintf("failed to reconcile %s", componentName)).Error())
		conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, failReason, clusterv1.ConditionSeverityError, "")
		return errors.Wrapf(err, "failed to reconcile %s for OCICluster %s/%s", componentName, cluster.Namespace,
			cluster.Name)
	}

	trimmedComponentName := strings.ReplaceAll(componentName, " ", "")
	r.Recorder.Eventf(cluster, corev1.EventTypeNormal, readyEventtype,
		fmt.Sprintf("%s is in ready state", trimmedComponentName))

	return nil
}

func (r *OCIClusterReconciler) reconcile(ctx context.Context, logger logr.Logger, clusterScope scope.ClusterScopeClient, cluster *infrastructurev1beta1.OCICluster) (ctrl.Result, error) {
	// If the OCICluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(cluster, infrastructurev1beta1.ClusterFinalizer)

	// This below if condition specifies if the network related infrastructure needs to be reconciled. Any new
	// network related reconcilication should happen in this if condition
	if !cluster.Spec.NetworkSpec.SkipNetworkManagement {
		err := clusterScope.SetRegionKey(ctx)
		if err != nil {
			logger.Error(err, "Couldn't get region code")
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileDRG, "DRG",
			infrastructurev1beta1.DrgReconciliationFailedReason, infrastructurev1beta1.DrgEventReady); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileVCN, "VCN",
			infrastructurev1beta1.VcnReconciliationFailedReason, infrastructurev1beta1.VcnEventReady); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileInternetGateway, "Internet Gateway",
			infrastructurev1beta1.InternetGatewayReconciliationFailedReason, infrastructurev1beta1.InternetGatewayEventReady); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileNatGateway, "NAT Gateway",
			infrastructurev1beta1.NatGatewayReconciliationFailedReason, infrastructurev1beta1.NatEventReady); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileServiceGateway, "Service Gateway",
			infrastructurev1beta1.ServiceGatewayReconciliationFailedReason, infrastructurev1beta1.ServiceGatewayEventReady); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileNSG, "Network Security Group",
			infrastructurev1beta1.NSGReconciliationFailedReason, infrastructurev1beta1.NetworkSecurityEventReady); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileRouteTable, "Route Table",
			infrastructurev1beta1.RouteTableReconciliationFailedReason, infrastructurev1beta1.RouteTableEventReady); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileSubnet, "Subnet",
			infrastructurev1beta1.SubnetReconciliationFailedReason, infrastructurev1beta1.SubnetEventReady); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileDRGVCNAttachment, "DRGVCNAttachment",
			infrastructurev1beta1.DRGVCNAttachmentReconciliationFailedReason, infrastructurev1beta1.DRGVCNAttachmentEventReady); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileDRGRPCAttachment, "DRGRPCAttachment",
			infrastructurev1beta1.DRGRPCAttachmentReconciliationFailedReason, infrastructurev1beta1.DRGRPCAttachmentEventReady); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		logger.Info("VCN Reconciliation is skipped")
	}

	if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileFailureDomains, "Failure Domain",
		infrastructurev1beta1.FailureDomainFailedReason, infrastructurev1beta1.FailureDomainEventReady); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileComponent(ctx, cluster, clusterScope.ReconcileApiServerLB, "Api Server Loadbalancer",
		infrastructurev1beta1.APIServerLoadBalancerFailedReason, infrastructurev1beta1.ApiServerLoadBalancerEventReady); err != nil {
		return ctrl.Result{}, err
	}

	conditions.MarkTrue(cluster, infrastructurev1beta1.ClusterReadyCondition)
	cluster.Status.Ready = true
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCIClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrastructurev1beta1.OCICluster{}).
		WithEventFilter(predicates.ResourceNotPaused(log)).              // don't queue reconcile if resource is paused
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(log)). //the externally managed cluster won't be reconciled
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.clusterToInfrastructureMapFunc(ctx, log)),
		predicates.ClusterUnpaused(log),
		predicates.ResourceNotPausedAndHasFilterLabel(log, ""),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// ClusterToInfrastructureMapFunc returns a handler.ToRequestsFunc that watches for
// Cluster events and returns reconciliation requests for an infrastructure provider object.
func (r *OCIClusterReconciler) clusterToInfrastructureMapFunc(ctx context.Context, log logr.Logger) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		c, ok := o.(*clusterv1.Cluster)
		if !ok {
			return nil
		}

		// Make sure the ref is set
		if c.Spec.InfrastructureRef == nil {
			log.V(4).Info("Cluster does not have an InfrastructureRef, skipping mapping.")
			return nil
		}

		if c.Spec.InfrastructureRef.GroupVersionKind().Kind != "OCICluster" {
			log.V(4).Info("Cluster has an InfrastructureRef for a different type, skipping mapping.")
			return nil
		}

		ociCluster := &infrastructurev1beta1.OCICluster{}
		key := types.NamespacedName{Namespace: c.Spec.InfrastructureRef.Namespace, Name: c.Spec.InfrastructureRef.Name}

		if err := r.Get(ctx, key, ociCluster); err != nil {
			log.V(4).Error(err, "Failed to get OCI cluster")
			return nil
		}

		if annotations.IsExternallyManaged(ociCluster) {
			log.V(4).Info("OCICluster is externally managed, skipping mapping.")
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

func (r *OCIClusterReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, clusterScope scope.ClusterScopeClient, cluster *infrastructurev1beta1.OCICluster) (ctrl.Result, error) {
	err := clusterScope.DeleteApiServerLB(ctx)
	if err != nil {
		r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete Api Server Loadbalancer").Error())
		conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.APIServerLoadBalancerFailedReason, clusterv1.ConditionSeverityError, "")
		return ctrl.Result{}, errors.Wrapf(err, "failed to delete apiserver LB for OCICluster %s/%s", cluster.Namespace, cluster.Name)
	}

	// This below if condition specifies if the network related infrastructure needs to be reconciled. Any new
	// network related reconcilication should happen in this if condition
	if !cluster.Spec.NetworkSpec.SkipNetworkManagement {
		err = clusterScope.DeleteDRGRPCAttachment(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete DRG RPC attachment").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.DRGRPCAttachmentReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete DRG RPC Attachment  for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteDRGVCNAttachment(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete DRG VCN attachment").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.DRGVCNAttachmentReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete DRG VCN Attachment  for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteNSGs(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete Network Security Group").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.NSGReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete Network Security Groups for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteSubnets(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete Subnet").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.SubnetReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete subnet for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteRouteTables(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete Route Table").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.RouteTableReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete RouteTables for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteSecurityLists(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete Security Lists").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.SecurityListReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete SecurityLists for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteServiceGateway(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete Service Gateway").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.ServiceGatewayReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete ServiceGateway for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteNatGateway(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete NAT Gateway").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.NatGatewayReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete NatGateway for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteInternetGateway(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete Internet Gateway").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.InternetGatewayReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete InternetGateway for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteVCN(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete VCN").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.VcnReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete VCN for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

		err = clusterScope.DeleteDRG(ctx)
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to delete DRG").Error())
			conditions.MarkFalse(cluster, infrastructurev1beta1.ClusterReadyCondition, infrastructurev1beta1.DrgReconciliationFailedReason, clusterv1.ConditionSeverityError, "")
			return ctrl.Result{}, errors.Wrapf(err, "failed to delete DRG for OCICluster %s/%s", cluster.Namespace, cluster.Name)
		}

	} else {
		logger.Info("VCN Reconciliation is skipped, none of the VCN related resources will be deleted")
	}
	controllerutil.RemoveFinalizer(cluster, v1beta1.ClusterFinalizer)

	return reconcile.Result{}, nil
}
