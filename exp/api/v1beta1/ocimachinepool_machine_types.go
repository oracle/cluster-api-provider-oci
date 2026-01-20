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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// +kubebuilder:object:generate=true
// +groupName=infrastructure.cluster.x-k8s.io

const (
	Managed     MachineTypeEnum = "MANAGED_TYPE"
	Virtual     MachineTypeEnum = "VIRTUAL_TYPE"
	SelfManaged MachineTypeEnum = "SELF_MANAGED_TYPE"
)

type MachineTypeEnum string

// OCIMachinePoolMachineSpec defines the desired state of OCIMachinePoolMachine
type OCIMachinePoolMachineSpec struct {
	// ProviderID is the Oracle Cloud Identifier of the associated instance.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// OCID is the OCID of the associated instance.
	// +optional
	OCID *string `json:"ocid,omitempty"`

	// InstanceName is the name of the instance.
	// +optional
	InstanceName *string `json:"instanceName,omitempty"`

	// MachineType is the type of the machine.
	MachineType MachineTypeEnum `json:"machineType,omitempty"`
}

// OCIMachinePoolMachineStatus defines the observed state of OCIMachinePoolMachine
type OCIMachinePoolMachineStatus struct {
	// Flag set to true when machine is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Conditions defines current service state of the OCIMachinePool.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type OCIMachinePoolMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIMachinePoolMachineSpec   `json:"spec,omitempty"`
	Status OCIMachinePoolMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OCIMachinePoolMachineList contains a list of OCIMachinePoolMachine.
type OCIMachinePoolMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIMachinePoolMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCIMachinePoolMachine{}, &OCIMachinePoolMachineList{})
}
