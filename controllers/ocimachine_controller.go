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
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/workrequests"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
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

// OCIMachineReconciler reconciles a OciMachine object
type OCIMachineReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ClientProvider *scope.ClientProvider
	Region         string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the machine closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *OCIMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	fmt.Print("hello")
	logger := log.FromContext(ctx)
	logger.Info("Got reconciliation event for machine")

	ociMachine := &infrastructurev1beta2.OCIMachine{}
	err := r.Get(ctx, req.NamespacedName, ociMachine)
	fmt.Print(err)
	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Println("Result1 ", ctrl.Result{})
			return ctrl.Result{}, nil
		}
		fmt.Println("Result2 ", ctrl.Result{})
		return ctrl.Result{}, err
	}
	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, ociMachine.ObjectMeta)
	fmt.Print(machine)
	if err != nil {
		fmt.Println("Result3 ", ctrl.Result{})
		return ctrl.Result{}, err
	}
	if machine == nil {
		r.Recorder.Eventf(ociMachine, corev1.EventTypeNormal, "OwnerRefNotSet", "Cluster Controller has not yet set OwnerRef")
		logger.Info("Machine Controller has not yet set OwnerRef")
		fmt.Println("Result4 ", ctrl.Result{})
		return ctrl.Result{}, nil
	}
	logger = logger.WithValues("machine-name", ociMachine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, ociMachine.ObjectMeta)
	if err != nil {
		r.Recorder.Eventf(ociMachine, corev1.EventTypeWarning, "ClusterDoesNotExist", "Machine is missing cluster label or cluster does not exist")
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, ociMachine) {
		logger.Info("OCIMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	ociCluster := &infrastructurev1beta2.OCICluster{}
	ociClusterName := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	var clusterAccessor scope.OCIClusterAccessor
	if cluster.Spec.InfrastructureRef.Kind == "OCICluster" {
		if err := r.Client.Get(ctx, ociClusterName, ociCluster); err != nil {
			logger.Info("Cluster is not available yet")
			r.Recorder.Eventf(ociMachine, corev1.EventTypeWarning, "ClusterNotAvailable", "Cluster is not available yet")
			return ctrl.Result{}, nil
		}
		clusterAccessor = scope.OCISelfManagedCluster{
			OCICluster: ociCluster,
		}
	} else if cluster.Spec.InfrastructureRef.Kind == "OCIManagedCluster" {
		// check for oci managed cluster
		ociManagedCluster := &infrastructurev1beta2.OCIManagedCluster{}
		ociManagedClusterName := client.ObjectKey{
			Namespace: cluster.Namespace,
			Name:      cluster.Spec.InfrastructureRef.Name,
		}
		if err := r.Client.Get(ctx, ociManagedClusterName, ociManagedCluster); err != nil {

		}
		clusterAccessor = scope.OCIManagedCluster{
			OCIManagedCluster: ociManagedCluster,
		}
	} else {
		r.Recorder.Eventf(ociMachine, corev1.EventTypeWarning, "InfrastructureClusterTypeNotSupported", fmt.Sprintf("Infrastructure Cluster Type %s is not supported", cluster.Spec.InfrastructureRef.Kind))
		return ctrl.Result{}, errors.New(fmt.Sprintf("Infrastructure Cluster Type %s is not supported", cluster.Spec.InfrastructureRef.Kind))
	}

	_, _, clients, err := cloudutil.InitClientsAndRegion(ctx, r.Client, r.Region, clusterAccessor, r.ClientProvider)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Client:                    r.Client,
		ComputeClient:             clients.ComputeClient,
		Logger:                    &logger,
		Cluster:                   cluster,
		OCIClusterAccessor:        clusterAccessor,
		Machine:                   machine,
		OCIMachine:                ociMachine,
		VCNClient:                 clients.VCNClient,
		NetworkLoadBalancerClient: clients.NetworkLoadBalancerClient,
		LoadBalancerClient:        clients.LoadBalancerClient,
		WorkRequestsClient:        clients.WorkRequestsClient,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}
	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		if err := machineScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !ociMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, logger, machineScope)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCIMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	clusterToObjectFunc, err := util.ClusterToTypedObjectsMapper(r.Client, &infrastructurev1beta2.OCIMachineList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to create mapper for Cluster to OCIMachines")
	}
	err = ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrastructurev1beta2.OCIMachine{}).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(ctx))).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrastructurev1beta2.
				GroupVersion.WithKind(scope.OCIMachineKind))),
		).
		Watches(
			&infrastructurev1beta2.OCICluster{},
			handler.EnqueueRequestsFromMapFunc(r.OCIClusterToOCIMachines()),
		).
		Watches(
			&infrastructurev1beta2.OCIManagedCluster{},
			handler.EnqueueRequestsFromMapFunc(r.OCIManagedClusterToOCIMachines()),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc),
			builder.WithPredicates(
				predicates.ClusterUnpausedAndInfrastructureReady(ctrl.LoggerFrom(ctx)),
			),
		).
		// don't queue reconcile if resource is paused
		Complete(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}
	return nil
}

func (r *OCIMachineReconciler) OCIClusterToOCIMachines() handler.MapFunc {
	return func(ctx context.Context, o client.Object) []ctrl.Request {
		log := ctrl.LoggerFrom(ctx)
		result := []ctrl.Request{}

		c, ok := o.(*infrastructurev1beta2.OCICluster)
		if !ok {
			log.Error(errors.Errorf("expected a OCICluster but got a %T", o), "failed to get OCIMachine for OCICluster")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			return result
		case err != nil:
			log.Error(err, "failed to get owning cluster")
			return result
		}

		labels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
		machineList := &clusterv1.MachineList{}
		if err := r.List(ctx, machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
			log.Error(err, "failed to list Machines")
			return nil
		}
		for _, m := range machineList.Items {
			if m.Spec.InfrastructureRef.Name == "" {
				continue
			}
			name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
			result = append(result, ctrl.Request{NamespacedName: name})
		}

		return result
	}
}

func (r *OCIMachineReconciler) OCIManagedClusterToOCIMachines() handler.MapFunc {
	return func(ctx context.Context, o client.Object) []ctrl.Request {
		log := ctrl.LoggerFrom(ctx)
		result := []ctrl.Request{}

		c, ok := o.(*infrastructurev1beta2.OCIManagedCluster)
		if !ok {
			log.Error(errors.Errorf("expected a OCICluster but got a %T", o), "failed to get OCIMachine for OCICluster")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			return result
		case err != nil:
			log.Error(err, "failed to get owning cluster")
			return result
		}

		labels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
		machineList := &clusterv1.MachineList{}
		if err := r.List(ctx, machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
			log.Error(err, "failed to list Machines")
			return nil
		}
		for _, m := range machineList.Items {
			if m.Spec.InfrastructureRef.Name == "" {
				continue
			}
			name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
			result = append(result, ctrl.Request{NamespacedName: name})
		}

		return result
	}
}

func (r *OCIMachineReconciler) reconcileNormal(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope) (ctrl.Result, error) {
	controllerutil.AddFinalizer(machineScope.OCIMachine, infrastructurev1beta2.MachineFinalizer)
	machine := machineScope.OCIMachine
	infraMachine := machineScope.Machine

	annotations := infraMachine.GetAnnotations()
	deleteMachineOnTermination := false
	if annotations != nil {
		_, deleteMachineOnTermination = annotations[infrastructurev1beta2.DeleteMachineOnInstanceTermination]
	}
	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		r.Recorder.Event(machine, corev1.EventTypeNormal, infrastructurev1beta2.WaitingForBootstrapDataReason, "Bootstrap data secret reference is not yet available")
		conditions.MarkFalse(machine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		logger.Info("Bootstrap data secret reference is not yet available")
		return ctrl.Result{}, nil
	}

	instance, err := r.getOrCreate(ctx, machineScope)
	if err != nil {
		r.Recorder.Event(machine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "Failed to reconcile OCIMachine").Error())
		conditions.MarkFalse(machine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.InstanceProvisionFailedReason, clusterv1.ConditionSeverityError, "")
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile OCI Machine %s/%s", machineScope.OCIMachine.Namespace, machineScope.OCIMachine.Name)
	}

	machineScope.Info("OCI Compute Instance found", "InstanceID", *instance.Id)
	machine.Spec.InstanceId = instance.Id

	machine.Spec.ProviderID = common.String(machineScope.OCIClusterAccessor.GetProviderID(*instance.Id))

	// Proceed to reconcile the DOMachine state.
	switch instance.LifecycleState {
	case core.InstanceLifecycleStateProvisioning, core.InstanceLifecycleStateStarting:
		machineScope.Info("Instance is pending")
		conditions.MarkFalse(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.InstanceNotReadyReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	case core.InstanceLifecycleStateStopping, core.InstanceLifecycleStateStopped, core.InstanceLifecycleStateMoving:
		machineScope.SetNotReady()
		machineScope.Info(fmt.Sprintf("Instance is in %s state and not ready", instance.LifecycleState))
		conditions.MarkFalse(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.InstanceNotReadyReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{}, nil
	case core.InstanceLifecycleStateRunning:
		machineScope.Info("Instance is active")
		if machine.Status.Addresses == nil || len(machine.Status.Addresses) == 0 {
			machineScope.Info("IP address is not set on the instance, looking up the address")
			ipAddress, err := machineScope.GetInstanceIp(ctx)
			if err != nil {
				r.Recorder.Event(machine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to reconcile OCIMachine").Error())
				conditions.MarkFalse(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.InstanceIPAddressNotFound, clusterv1.ConditionSeverityError, "")
				return ctrl.Result{}, err
			}
			machine.Status.Addresses = []clusterv1.MachineAddress{
				{
					Address: *ipAddress,
					Type:    clusterv1.MachineInternalIP,
				},
			}
		}
		if machineScope.IsControlPlane() {
			err := machineScope.ReconcileCreateInstanceOnLB(ctx)
			if err != nil {
				r.Recorder.Event(machine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to reconcile OCIMachine").Error())
				conditions.MarkFalse(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.InstanceLBBackendAdditionFailedReason, clusterv1.ConditionSeverityError, "")
				return ctrl.Result{}, err
			}
			machineScope.Info("Instance is added to the control plane LB")
		}

		if len(machine.Spec.VnicAttachments) > 0 {
			err := machineScope.ReconcileVnicAttachments(ctx)
			if err != nil {
				r.Recorder.Event(machine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to reconcile OCIMachine").Error())
				conditions.MarkFalse(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition,
					infrastructurev1beta2.InstanceVnicAttachmentFailedReason, clusterv1.ConditionSeverityError, "")
				return ctrl.Result{}, err
			}
			machineScope.Info("Instance vnic attachment success")
			r.Recorder.Eventf(machineScope.OCIMachine, corev1.EventTypeNormal, infrastructurev1beta2.InstanceVnicAttachmentReady,
				"VNICs have been attached to instance.")
		}

		// record the event only when machine goes from not ready to ready state
		r.Recorder.Eventf(machine, corev1.EventTypeNormal, "InstanceReady",
			"Instance is in ready state")
		conditions.MarkTrue(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition)
		machineScope.SetReady()
		if deleteMachineOnTermination {
			// typically, if the VM is terminated, we should get machine events, so ideally, the 300 seconds
			// requeue time is not required, but in case, the event is missed, adding the requeue time
			return reconcile.Result{RequeueAfter: 300 * time.Second}, nil
		} else {
			return reconcile.Result{}, nil
		}
	case core.InstanceLifecycleStateTerminated:
		if deleteMachineOnTermination && infraMachine.DeletionTimestamp == nil {
			logger.Info("Deleting underlying machine as instance is terminated")
			if err := machineScope.Client.Delete(ctx, infraMachine); err != nil {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete machine %s/%s", infraMachine.Namespace, infraMachine.Name)
			}
		}
		fallthrough
	default:
		machineScope.SetNotReady()
		conditions.MarkFalse(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.InstanceProvisionFailedReason, clusterv1.ConditionSeverityError, "")
		machineScope.SetFailureReason(capierrors.CreateMachineError)
		machineScope.SetFailureMessage(errors.Errorf("Instance status %q is unexpected", instance.LifecycleState))
		wrequest := workrequests.ListWorkRequestsRequest{
			CompartmentId: instance.CompartmentId,
			ResourceId:    instance.Id,
		}
		wresp, err := machineScope.WorkRequestsClient.ListWorkRequests(ctx, wrequest)
		if err != nil {
			r.Recorder.Event(machine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "Failed to normal reconcile OCIMachine").Error())
			return reconcile.Result{}, errors.Wrapf(err, "failed to normal reconcile OCI Machine %s/%s", machineScope.OCIMachine.Namespace, machineScope.OCIMachine.Name)
		}
		for _, wrqst := range wresp.Items {
			if wrqst.Status == "FAILED" {
				logger.Info("Fetching work-request errors for the terminating/terminated instances")
				wreq := workrequests.ListWorkRequestErrorsRequest{
					WorkRequestId: wrqst.Id,
				}
				wr_errs, err := machineScope.WorkRequestsClient.ListWorkRequestErrors(ctx, wreq)
				if err != nil {
					r.Recorder.Event(machine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "Failed to normal reconcile OCIMachine").Error())
					return reconcile.Result{}, errors.Wrapf(err, "failed to normal reconcile OCI Machine %s/%s", machineScope.OCIMachine.Namespace, machineScope.OCIMachine.Name)
				}
				for _, wr_err := range wr_errs.Items {
					fmt.Printf("WorkRequestErrorMessage: %s\n", *wr_err.Message)
					r.Recorder.Eventf(machine, corev1.EventTypeWarning, "ReconcileError", *wr_err.Message)
				}
			}
		}
		r.Recorder.Eventf(machine, corev1.EventTypeWarning, "ReconcileError",
			"Instance  has invalid lifecycle state %s", instance.LifecycleState)
		return reconcile.Result{}, errors.New(fmt.Sprintf("instance  has invalid lifecycle state %s", instance.LifecycleState))
	}
}

func (r *OCIMachineReconciler) getOrCreate(ctx context.Context, scope *scope.MachineScope) (*core.Instance, error) {
	instance, err := scope.GetOrCreateMachine(ctx)
	return instance, err
}

func (r *OCIMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.MachineScope) (_ ctrl.Result, reterr error) {
	machineScope.Info("Handling deleted OCIMachine")

	instance, err := machineScope.GetMachine(ctx)
	if err != nil {
		if ociutil.IsNotFound(err) {
			err := r.deleteInstanceFromControlPlaneLB(ctx, machineScope)
			if err != nil {
				return reconcile.Result{}, err
			}
			conditions.MarkFalse(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.InstanceNotFoundReason, clusterv1.ConditionSeverityInfo, "")
			machineScope.Info("Instance is not found, may have been deleted")
			controllerutil.RemoveFinalizer(machineScope.OCIMachine, infrastructurev1beta2.MachineFinalizer)
			return reconcile.Result{}, nil
		} else {
			return reconcile.Result{}, err
		}
	}
	if instance == nil {
		machineScope.Info("Instance is not found, may have been deleted")
		controllerutil.RemoveFinalizer(machineScope.OCIMachine, infrastructurev1beta2.MachineFinalizer)
		return reconcile.Result{}, nil
	}

	machineScope.Info("OCI Compute Instance found", "InstanceID", *instance.Id)

	switch instance.LifecycleState {
	case core.InstanceLifecycleStateTerminating:
		machineScope.Info("Instance is terminating")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	case core.InstanceLifecycleStateTerminated:
		conditions.MarkFalse(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.InstanceTerminatedReason, clusterv1.ConditionSeverityInfo, "")
		controllerutil.RemoveFinalizer(machineScope.OCIMachine, infrastructurev1beta2.MachineFinalizer)
		machineScope.Info("Instance is deleted")
		r.Recorder.Eventf(machineScope.OCIMachine, corev1.EventTypeNormal,
			"InstanceTerminated", "Deleted the instance")
		return reconcile.Result{}, nil
	default:
		if !machineScope.IsResourceCreatedByClusterAPI(instance.FreeformTags) {
			return reconcile.Result{}, errors.New("instance is not created by current cluster")
		}
		err := r.deleteInstanceFromControlPlaneLB(ctx, machineScope)
		if err != nil {
			return reconcile.Result{}, err
		}
		if err := machineScope.DeleteMachine(ctx, instance); err != nil {
			machineScope.Error(err, "Error deleting Instance")
			return ctrl.Result{}, errors.Wrapf(err, "error deleting instance %s", machineScope.Name())
		}
		conditions.MarkFalse(machineScope.OCIMachine, infrastructurev1beta2.InstanceReadyCondition, infrastructurev1beta2.InstanceTerminatingReason, clusterv1.ConditionSeverityInfo, "")
		r.Recorder.Eventf(machineScope.OCIMachine, corev1.EventTypeNormal,
			"InstanceTerminating", "Terminating the instance")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}
}

func (r *OCIMachineReconciler) deleteInstanceFromControlPlaneLB(ctx context.Context, machineScope *scope.MachineScope) error {
	if machineScope.IsControlPlane() {
		err := machineScope.ReconcileDeleteInstanceOnLB(ctx)
		if err != nil {
			return err
		}
		machineScope.Info("Instance is removed from the control plane LB")
		r.Recorder.Eventf(machineScope.OCIMachine, corev1.EventTypeNormal, "OCIMachineRemovedFromLB",
			"Instance has been removed from the control plane LB")
	}
	return nil
}
