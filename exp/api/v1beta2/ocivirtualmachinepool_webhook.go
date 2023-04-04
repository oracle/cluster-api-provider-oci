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

package v1beta2

import (
	"github.com/oracle/oci-go-sdk/v65/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var virtualMpLogger = ctrl.Log.WithName("ocivirtualmachinepool-resource")

var (
	_ webhook.Defaulter = &OCIVirtualMachinePool{}
	_ webhook.Validator = &OCIVirtualMachinePool{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ocivirtualmachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocivirtualmachinepools,versions=v1beta2,name=validation.ocivirtualmachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ocivirtualmachinepool,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocivirtualmachinepools,versions=v1beta2,name=default.ocivirtualmachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

func (m *OCIVirtualMachinePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

func (m *OCIVirtualMachinePool) Default() {
	if m.Spec.PodConfiguration.NsgNames == nil {
		m.Spec.PodConfiguration.NsgNames = []string{PodDefaultName}
	}
	if m.Spec.PodConfiguration.SubnetName == nil {
		m.Spec.PodConfiguration.SubnetName = common.String(PodDefaultName)
	}
	if m.Spec.PodConfiguration.Shape == nil {
		m.Spec.PodConfiguration.Shape = common.String(PodDefaultName)
	}
}

func (m *OCIVirtualMachinePool) ValidateCreate() error {
	var allErrs field.ErrorList
	return apierrors.NewInvalid(m.GroupVersionKind().GroupKind(), m.Name, allErrs)
}

func (m *OCIVirtualMachinePool) ValidateUpdate(old runtime.Object) error {
	var allErrs field.ErrorList
	return apierrors.NewInvalid(m.GroupVersionKind().GroupKind(), m.Name, allErrs)
}

func (m *OCIVirtualMachinePool) ValidateDelete() error {
	return nil
}
