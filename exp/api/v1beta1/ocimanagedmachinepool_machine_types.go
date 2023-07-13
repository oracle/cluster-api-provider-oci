/*
Copyright (c) 2022 Oracle and/or its affiliates.

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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// +kubebuilder:object:generate=true
// +groupName=infrastructure.cluster.x-k8s.io

// OCIManagedMachinePoolMachineSpec defines the desired state of OCIManagedMachinePoolMachine
type OCIManagedMachinePoolMachineSpec struct {
	// ProviderID is the OCID of the associated InstancePool in a provider format
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// OCID is the OCID of the associated InstancePool
	// +optional
	OCID *string `json:"ocid,omitempty"`
}

// OCIManagedMachinePoolMachineStatus defines the observed state of OCIManagedMachinePoolMachine
type OCIManagedMachinePoolMachineStatus struct {
	// Flag set to true when machine is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Conditions defines current service state of the OCIMachinePool.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type OCIManagedMachinePoolMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIManagedMachinePoolMachineSpec   `json:"spec,omitempty"`
	Status OCIManagedMachinePoolMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OCIManagedMachinePoolMachineList contains a list of OCIManagedMachinePoolMachine.
type OCIManagedMachinePoolMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIManagedMachinePoolMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCIManagedMachinePoolMachine{}, &OCIManagedMachinePoolMachineList{})
}
