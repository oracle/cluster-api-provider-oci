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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	helper, err := patch.NewHelper(ociMachinePoolMachine, r.Client)
	defer func() {
		if err := helper.Patch(ctx, ociMachinePoolMachine); err != nil && reterr == nil {
			reterr = err
		}
	}()
	if !ociMachinePoolMachine.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(ociMachinePoolMachine, infrav2exp.MachinePoolMachineFinalizer)
	}
	return ctrl.Result{}, nil
}

func (r *OCIMachinePoolMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav2exp.OCIMachinePoolMachine{}).
		WithEventFilter(predicates.ResourceNotPaused(mgr.GetScheme(), ctrl.LoggerFrom(ctx))).
		Complete(r)

	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}
	return nil
}
