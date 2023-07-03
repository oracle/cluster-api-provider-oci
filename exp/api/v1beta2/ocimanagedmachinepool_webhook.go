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
	"fmt"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var logger = ctrl.Log.WithName("ocimachinepool-resource")

var (
	_ webhook.Defaulter = &OCIManagedMachinePool{}
	_ webhook.Validator = &OCIManagedMachinePool{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ocimanagedmachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedmachinepools,versions=v1beta2,name=validation.ocimanagedmachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ocimanagedmachinepool,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedmachinepools,versions=v1beta2,name=default.ocimanagedmachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

func (m *OCIManagedMachinePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

func (m *OCIManagedMachinePool) Default() {
	if m.Spec.NodePoolNodeConfig == nil {
		m.Spec.NodePoolNodeConfig = &NodePoolNodeConfig{}
	}
	if m.Spec.NodePoolNodeConfig.NodePoolPodNetworkOptionDetails == nil {
		m.Spec.NodePoolNodeConfig.NodePoolPodNetworkOptionDetails = &NodePoolPodNetworkOptionDetails{
			CniType: infrastructurev1beta2.VCNNativeCNI,
			VcnIpNativePodNetworkOptions: VcnIpNativePodNetworkOptions{
				SubnetNames: []string{PodDefaultName},
				NSGNames:    []string{PodDefaultName},
			},
		}
	}
}

func (m *OCIManagedMachinePool) ValidateCreate() error {
	var allErrs field.ErrorList
	if len(m.Name) > 31 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("Name"), m.Name, "Name cannot be more than 31 characters"))
	}
	allErrs = m.validateVersion(allErrs)
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(m.GroupVersionKind().GroupKind(), m.Name, allErrs)
}

func (m *OCIManagedMachinePool) validateVersion(allErrs field.ErrorList) field.ErrorList {
	if m.Spec.Version == nil {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "version"), m.Spec.Version, "field is required"))
	}
	if m.Spec.Version != nil {
		if !version.KubeSemver.MatchString(*m.Spec.Version) {
			allErrs = append(
				allErrs,
				field.Invalid(field.NewPath("spec", "version"), m.Spec.Version, "must be a valid semantic version"))
		}
	}
	return allErrs
}

func (m *OCIManagedMachinePool) ValidateUpdate(old runtime.Object) error {
	var allErrs field.ErrorList
	oldManagedMachinePool, ok := old.(*OCIManagedMachinePool)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an OCIManagedMachinePool but got a %T", old))
	}

	allErrs = m.validateVersion(allErrs)
	if !reflect.DeepEqual(m.Spec.Version, oldManagedMachinePool.Spec.Version) {
		newImage := m.getImageId()
		oldImage := oldManagedMachinePool.getImageId()
		// if an image has been provided in updated machine pool and it matches old image id,
		// and if Kubernetes version has been updated, that means the image is not correct. If the version has
		// been changed, the image should have been updated by the user, or set as nil in which case
		// CAPOCI will lookup a correct image
		if newImage != nil && reflect.DeepEqual(newImage, oldImage) {
			allErrs = append(
				allErrs,
				field.Invalid(field.NewPath("spec", "nodeSourceViaImage", "imageId"),
					m.getImageId(), "image id has not been updated for the newer version, "+
						"either provide a newer image or set the field as nil"))

		}
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(m.GroupVersionKind().GroupKind(), m.Name, allErrs)
}

func (m *OCIManagedMachinePool) ValidateDelete() error {
	return nil
}

func (m *OCIManagedMachinePool) getImageId() *string {
	if m.Spec.NodeSourceViaImage != nil {
		return m.Spec.NodeSourceViaImage.ImageId
	}
	return nil
}
