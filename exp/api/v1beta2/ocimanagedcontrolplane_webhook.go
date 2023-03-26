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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var managedcplanelogger = ctrl.Log.WithName("ocimanagedcontrolplane-resource")

var (
	_ webhook.Defaulter = &OCIManagedControlPlane{}
	_ webhook.Validator = &OCIManagedControlPlane{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ocimanagedcontrolplane,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedcontrolplanes,versions=v1beta2,name=validation.ocimanagedcontrolplane.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta2
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ocimanagedcontrolplane,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedcontrolplanes,versions=v1beta2,name=default.ocimanagedcontrolplane.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

func (c *OCIManagedControlPlane) Default() {
	if len(c.Spec.ClusterPodNetworkOptions) == 0 {
		c.Spec.ClusterPodNetworkOptions = []ClusterPodNetworkOptions{
			{
				CniType: VCNNativeCNI,
			},
		}
	}
}

func (c *OCIManagedControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

func (c *OCIManagedControlPlane) ValidateCreate() error {
	var allErrs field.ErrorList
	if len(c.Name) > 31 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("Name"), c.Name, "Name cannot be more than 31 characters"))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(c.GroupVersionKind().GroupKind(), c.Name, allErrs)
}

func (c *OCIManagedControlPlane) ValidateUpdate(old runtime.Object) error {
	return nil
}

func (c *OCIManagedControlPlane) ValidateDelete() error {
	return nil
}
