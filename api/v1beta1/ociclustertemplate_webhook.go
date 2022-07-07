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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	_ webhook.Defaulter = &OCIClusterTemplate{}
	_ webhook.Validator = &OCIClusterTemplate{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-ociclustertemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ociclustertemplates,versions=v1beta1,name=validation.ociclustertemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-ociclustertemplate,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ociclustertemplates,versions=v1beta1,name=default.ociclustertemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

func (c *OCIClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

func (c *OCIClusterTemplate) Default() {
	clusterSpec := c.Spec.Template.Spec
	for _, subnet := range clusterSpec.NetworkSpec.Vcn.Subnets {
		if subnet.UID == "empty" {
			subnet.UID = string(uuid.NewUUID())
		}
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *OCIClusterTemplate) ValidateCreate() error {
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *OCIClusterTemplate) ValidateDelete() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *OCIClusterTemplate) ValidateUpdate(old runtime.Object) error {
	return nil
}
