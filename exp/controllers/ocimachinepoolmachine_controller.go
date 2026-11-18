/*
Copyright (c) 2023 Oracle and/or its affiliates.

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

	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type OCIMachinePoolMachineReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ClientProvider *scope.ClientProvider
	Region         string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ocimachinepoolmachines,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the machinepoolmachines closer to the desired state.
func (r *OCIMachinePoolMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	// currently, we only remove the finalizer on delete
	// at a later point, when machine pool machine feature improves with integration with autoscaler and deployment
	// orchestration etc, we should improve the below logic to actually delete the machine pool machine from the underlying
	// infra object such as OKE nodepool or instance pool etc
	ociMachinePoolMachine := &infrav2exp.OCIMachinePoolMachine{}
	err := r.Get(ctx, req.NamespacedName, ociMachinePoolMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	isPaused := annotations.HasPaused(ociMachinePoolMachine)
	if name := ociMachinePoolMachine.Labels[clusterv1.ClusterNameLabel]; name != "" {
		cluster := &clusterv1.Cluster{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: ociMachinePoolMachine.Namespace, Name: name}, cluster); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		} else {
			isPaused = annotations.IsPaused(cluster, ociMachinePoolMachine)
		}
	}
	if isPaused {
		return ctrl.Result{}, nil
	}
	helper, err := v1beta1patch.NewHelper(ociMachinePoolMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if !ociMachinePoolMachine.GetDeletionTimestamp().IsZero() {
			v1beta2conditions.Set(ociMachinePoolMachine, metav1.Condition{
				Type:   clusterv1.DeletingCondition,
				Status: metav1.ConditionTrue,
				Reason: clusterv1.DeletingReason,
			})
			v1beta2conditions.Set(ociMachinePoolMachine, metav1.Condition{
				Type:    clusterv1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  clusterv1.DeletingReason,
				Message: "Infrastructure is being deleted",
			})
		} else {
			v1beta2conditions.Set(ociMachinePoolMachine, metav1.Condition{
				Type:   clusterv1.DeletingCondition,
				Status: metav1.ConditionFalse,
				Reason: clusterv1.NotDeletingReason,
			})
		}
		if err := helper.Patch(ctx, ociMachinePoolMachine, v1beta1patch.WithOwnedV1Beta2Conditions{
			Conditions: []string{clusterv1.ReadyCondition, clusterv1.DeletingCondition},
		}); err != nil && reterr == nil {
			reterr = err
		}
	}()
	if !ociMachinePoolMachine.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(ociMachinePoolMachine, infrav2exp.MachinePoolMachineFinalizer)
	}
	return ctrl.Result{}, nil
}

func (r *OCIMachinePoolMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	clusterToObjectFunc, err := util.ClusterToTypedObjectsMapper(r.Client, &infrav2exp.OCIMachinePoolMachineList{}, mgr.GetScheme())
	if err != nil {
		return err
	}
	err = ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav2exp.OCIMachinePoolMachine{}).
		WithEventFilter(predicates.ResourceNotPaused(mgr.GetScheme(), ctrl.LoggerFrom(ctx))).
		Watches(&clusterv1.Cluster{}, handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc)).
		Complete(r)

	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}
	return nil
}
