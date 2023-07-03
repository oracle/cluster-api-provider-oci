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

// OCIManagedClusterTemplateSpec defines the desired state of OCIManagedClusterTemplate.
type OCIManagedClusterTemplateSpec struct {
	Template OCIManagedClusterTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ocimanagedclustertemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// OCIManagedClusterTemplate is the Schema for the ocimanagedclustertemplates API.
type OCIManagedClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OCIManagedClusterTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// OCIManagedClusterTemplateList contains a list of OCIManagedClusterTemplate.
type OCIManagedClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []OCIManagedClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCIManagedClusterTemplate{}, &OCIManagedClusterTemplateList{})
}

// OCIManagedClusterSpec describes the data needed to create an OCIManagedCluster from a template.
type OCIManagedClusterTemplateResource struct {
	Spec OCIManagedClusterSpec `json:"spec"`
}
