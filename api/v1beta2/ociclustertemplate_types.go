/*
Copyright (c) 2022, Oracle and/or its affiliates.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OCIClusterTemplateSpec defines the desired state of OCIClusterTemplate.
type OCIClusterTemplateSpec struct {
	Template OCIClusterTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ociclustertemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// OCIClusterTemplate is the Schema for the ociclustertemplates API.
type OCIClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OCIClusterTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// OCIClusterTemplateList contains a list of OCIClusterTemplate.
type OCIClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []OCIClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCIClusterTemplate{}, &OCIClusterTemplateList{})
}

// OCIClusterTemplateResource describes the data needed to create an OCICluster from a template.
type OCIClusterTemplateResource struct {
	Spec OCIClusterSpec `json:"spec"`
}
