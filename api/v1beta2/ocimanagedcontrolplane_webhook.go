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

var managedcplanelogger = ctrl.Log.WithName("ocimanagedcontrolplane-resource")

type OCIManagedControlPlaneWebhook struct{}

var (
	_ webhook.CustomDefaulter = &OCIManagedControlPlaneWebhook{}
	_ webhook.CustomValidator = &OCIManagedControlPlaneWebhook{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ocimanagedcontrolplane,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedcontrolplanes,versions=v1beta2,name=validation.ocimanagedcontrolplane.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ocimanagedcontrolplane,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedcontrolplanes,versions=v1beta2,name=default.ocimanagedcontrolplane.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

func (*OCIManagedControlPlaneWebhook) Default(_ context.Context, obj runtime.Object) error {
	c, ok := obj.(*OCIManagedControlPlane)
	if !ok {
		return fmt.Errorf("expected an OCIManagedControlPlane object but got %T", c)
	}

	if len(c.Spec.ClusterPodNetworkOptions) == 0 {
		c.Spec.ClusterPodNetworkOptions = []ClusterPodNetworkOptions{
			{
				CniType: VCNNativeCNI,
			},
		}
	}

	return nil
}

func (c *OCIManagedControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(OCIManagedControlPlaneWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

func (*OCIManagedControlPlaneWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*OCIManagedControlPlane)
	if !ok {
		return nil, fmt.Errorf("expected an OCIManagedControlPlane object but got %T", c)
	}

	var allErrs field.ErrorList
	if len(c.Name) > 31 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("Name"), c.Name, "Name cannot be more than 31 characters"))
	}

	if c.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig != nil && c.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig.IsOpenIdConnectAuthEnabled {
		if c.Spec.ClusterType != EnhancedClusterType {
			allErrs = append(allErrs, field.Invalid(field.NewPath("ClusterType"), c.Spec.ClusterType, "ClusterType needs to be set to ENHANCED_CLUSTER for OpenIdConnectTokenAuthenticationConfig to be enabled."))
		}
		if c.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig.ClientId == nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("ClientId"), c.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig.ClientId, "ClientId cannot be empty when OpenIdConnectAuth is enabled."))
		}
		if c.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig.IssuerUrl == nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("IssuerUrl "), c.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig.IssuerUrl, "IssuerUrl cannot be empty when OpenIdConnectAuth is enabled."))
		}
	}
	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(c.GroupVersionKind().GroupKind(), c.Name, allErrs)
}

func (*OCIManagedControlPlaneWebhook) ValidateUpdate(_ context.Context, oldRaw, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (*OCIManagedControlPlaneWebhook) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
