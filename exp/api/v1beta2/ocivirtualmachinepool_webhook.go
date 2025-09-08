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
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var virtualMpLogger = ctrl.Log.WithName("ocivirtualmachinepool-resource")

type OCIVirtualMachinePoolWebhook struct{}

var (
	_ webhook.CustomDefaulter = &OCIVirtualMachinePoolWebhook{}
	_ webhook.CustomValidator = &OCIVirtualMachinePoolWebhook{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ocivirtualmachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocivirtualmachinepools,versions=v1beta2,name=validation.ocivirtualmachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ocivirtualmachinepool,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocivirtualmachinepools,versions=v1beta2,name=default.ocivirtualmachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

func (m *OCIVirtualMachinePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(OCIVirtualMachinePoolWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

func (*OCIVirtualMachinePoolWebhook) Default(_ context.Context, obj runtime.Object) error {
	m, ok := obj.(*OCIVirtualMachinePool)
	if !ok {
		return fmt.Errorf("expected an OCIVirtualMachinePool object but got %T", m)
	}

	if m.Spec.PodConfiguration.NsgNames == nil {
		m.Spec.PodConfiguration.NsgNames = []string{PodDefaultName}
	}
	if m.Spec.PodConfiguration.SubnetName == nil {
		m.Spec.PodConfiguration.SubnetName = common.String(PodDefaultName)
	}
	if m.Spec.PodConfiguration.Shape == nil {
		m.Spec.PodConfiguration.Shape = common.String("Pod.Standard.E4.Flex")
	}

	return nil
}

func (*OCIVirtualMachinePoolWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (*OCIVirtualMachinePoolWebhook) ValidateUpdate(_ context.Context, oldRaw, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (*OCIVirtualMachinePoolWebhook) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
