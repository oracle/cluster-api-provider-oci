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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OCIManagedMachinePoolTemplateSpec defines the desired state of OCIManagedMachinePoolTemplate.
type OCIManagedMachinePoolTemplateSpec struct {
	Template OCIManagedMachinePoolTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ocimanagedmachinepooltemplates,scope=Namespaced,categories=cluster-api

// OCIManagedMachinePoolTemplate is the Schema for the OCIManagedMachinePoolTemplates API.
type OCIManagedMachinePoolTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OCIManagedMachinePoolTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// OCIManagedMachinePoolTemplateList contains a list of OCIManagedMachinePoolTemplate.
type OCIManagedMachinePoolTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []OCIManagedMachinePoolTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCIManagedMachinePoolTemplate{}, &OCIManagedMachinePoolTemplateList{})
}

// OCIManagedMachinePoolSpec describes the data needed to create an OCIManagedMachinePool from a template.
type OCIManagedMachinePoolTemplateResource struct {
	Spec OCIManagedMachinePoolSpec `json:"spec"`
}
