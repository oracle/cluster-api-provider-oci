/*
 Copyright (c) 2021, 2022 Oracle and/or its affiliates.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

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

// OCIMachineTemplateSpec defines the desired state of OCIMachineTemplate.
type OCIMachineTemplateSpec struct {
	Template OCIMachineTemplateResource `json:"template"`
}

// OCIMachineTemplateResource describes the data needed to create an OCIMachine from a template.
type OCIMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec OCIMachineSpec `json:"spec"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ocimachinetemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// OCIMachineTemplate is the schema for the OCI compute instance machine template.
type OCIMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OCIMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// OCIMachineTemplateList contains a list of OCIMachineTemplate.
type OCIMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCIMachineTemplate{}, &OCIMachineTemplateList{})
}
