/*
 *
 * Copyright (c) 2022, Oracle and/or its affiliates.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 *
 */

package v1beta1

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var clusterlogger = ctrl.Log.WithName("ocicluster-resource")

var (
	_ webhook.Defaulter = &OCICluster{}
	_ webhook.Validator = &OCICluster{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-ocicluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ociclusters,versions=v1beta1,name=validation.ocicluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-ocicluster,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ociclusters,versions=v1beta1,name=default.ocicluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

func (c *OCICluster) Default() {
	if c.Spec.OCIResourceIdentifier == "" {
		c.Spec.OCIResourceIdentifier = string(uuid.NewUUID())
	}
}

func (c *OCICluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *OCICluster) ValidateCreate() error {
	clusterlogger.Info("validate update cluster", "name", c.Name)

	var allErrs field.ErrorList

	allErrs = append(allErrs, c.validate()...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(c.GroupVersionKind().GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *OCICluster) ValidateDelete() error {
	clusterlogger.Info("validate delete cluster", "name", c.Name)

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *OCICluster) ValidateUpdate(old runtime.Object) error {
	clusterlogger.Info("validate update cluster", "name", c.Name)

	var allErrs field.ErrorList

	oldCluster, ok := old.(*OCICluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an OCICluster but got a %T", old))
	}

	if c.Spec.Region != oldCluster.Spec.Region {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "region"), c.Spec.Region, "field is immutable"))
	}

	if c.Spec.OCIResourceIdentifier != oldCluster.Spec.OCIResourceIdentifier {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "ociResourceIdentifier"), c.Spec.OCIResourceIdentifier, "field is immutable"))
	}

	allErrs = append(allErrs, c.validate()...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(c.GroupVersionKind().GroupKind(), c.Name, allErrs)
}

func (c *OCICluster) validate() field.ErrorList {
	var allErrs field.ErrorList

	if len(c.Spec.CompartmentId) <= 0 {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "compartmentId"), c.Spec.CompartmentId, "field is required"))
	}

	// Handle case where CompartmentId exists, but isn't valid
	// the separate "blank" check above is a more clear error for the user
	if len(c.Spec.CompartmentId) > 0 && !validOcid(c.Spec.CompartmentId) {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "compartmentId"), c.Spec.CompartmentId, "field is invalid"))
	}

	if len(c.Spec.OCIResourceIdentifier) <= 0 {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "ociResourceIdentifier"), c.Spec.OCIResourceIdentifier, "field is required"))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return allErrs
}
