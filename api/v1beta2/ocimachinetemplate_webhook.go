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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type OCIMachineTemplateWebhook struct{}

var (
	_ webhook.CustomValidator = &OCIMachineTemplateWebhook{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ocimachinetemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocimachinetemplates,versions=v1beta2,name=validation.ocimachinetemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

func (m *OCIMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(OCIMachineTemplateWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		WithValidator(w).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*OCIMachineTemplateWebhook) ValidateCreate(_ context.Context, raw runtime.Object) (admission.Warnings, error) {
	m, ok := raw.(*OCIMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a OCIMachineTemplate but got a %T", raw))
	}

	clusterlogger.Info("validate create machinetemplate", "name", m.Name)

	var allErrs field.ErrorList

	allErrs = append(allErrs, m.validate()...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(m.GroupVersionKind().GroupKind(), m.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*OCIMachineTemplateWebhook) ValidateDelete(_ context.Context, raw runtime.Object) (admission.Warnings, error) {
	m, ok := raw.(*OCIMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a OCIMachineTemplate but got a %T", raw))
	}
	clusterlogger.Info("validate delete machinetemplate", "name", m.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*OCIMachineTemplateWebhook) ValidateUpdate(ctx context.Context, oldRaw runtime.Object, newRaw runtime.Object) (admission.Warnings, error) {
	m, ok := newRaw.(*OCIMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a OCIMachineTemplate but got a %T", newRaw))
	}
	clusterlogger.Info("validate update machinetemplate", "name", m.Name)

	var allErrs field.ErrorList

	allErrs = append(allErrs, m.validate()...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(m.GroupVersionKind().GroupKind(), m.Name, allErrs)
}

func (m *OCIMachineTemplate) validate() field.ErrorList {
	var allErrs field.ErrorList

	if len(m.Spec.Template.Spec.ImageId) > 0 && !ValidOcid(m.Spec.Template.Spec.ImageId) {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "imageId"),
				m.Spec.Template.Spec.ImageId,
				"field is invalid"),
		)
	}

	// simple validity test for compartment
	if len(m.Spec.Template.Spec.CompartmentId) > 0 && !ValidOcid(m.Spec.Template.Spec.CompartmentId) {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "compartmentId"),
				m.Spec.Template.Spec.CompartmentId,
				"field is invalid"),
		)
	}

	if !validShape(m.Spec.Template.Spec.Shape) {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "shape"),
				m.Spec.Template.Spec.Shape,
				fmt.Sprintf("shape is invalid - %s", m.Spec.Template.Spec.Shape)),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}

	return allErrs
}
