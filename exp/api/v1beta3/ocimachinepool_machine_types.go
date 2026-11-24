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

package v1beta3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=true
// +groupName=infrastructure.cluster.x-k8s.io

const (
	Managed     MachineTypeEnum = "MANAGED_TYPE"
	Virtual     MachineTypeEnum = "VIRTUAL_TYPE"
	SelfManaged MachineTypeEnum = "SELF_MANAGED_TYPE"
	// MachinePoolMachineFinalizer is the finalizer for managed machine pool.
	MachinePoolMachineFinalizer = "ocimachinepoolmachine.infrastructure.cluster.x-k8s.io"
)

type MachineTypeEnum string

// OCIMachinePoolMachineSpec defines the desired state of OCIMachinePoolMachine
type OCIMachinePoolMachineSpec struct {
	// ProviderID is Oracle Cloud Identifier of the associated instance.
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
	// initialization provides observations of the OCIMachinePoolMachine initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization OCIMachinePoolMachineInitializationStatus `json:"initialization,omitempty,omitzero"`

	// Conditions defines current service state of the OCIMachinePoolMachine.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// OCIMachinePoolMachineInitializationStatus provides observations of the OCIMachinePoolMachine initialization process.
// +kubebuilder:validation:MinProperties=1
type OCIMachinePoolMachineInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the Machine's infrastructure is fully provisioned.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="MachinePool machine is ready"
// +kubebuilder:printcolumn:name="Provisioned",type="boolean",JSONPath=".status.initialization.provisioned",description="MachinePool machine is provisioned"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="OCI instance provider ID"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of OCIMachinePoolMachine"

type OCIMachinePoolMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIMachinePoolMachineSpec   `json:"spec,omitempty"`
	Status OCIMachinePoolMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// OCIMachinePoolMachineList contains a list of OCIMachinePoolMachine.
type OCIMachinePoolMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIMachinePoolMachine `json:"items"`
}

// GetConditions returns the list of conditions for an OCIMachinePoolMachine API object.
func (m *OCIMachinePoolMachine) GetConditions() []metav1.Condition {
	return m.Status.Conditions
}

// SetConditions will set the given conditions on an OCIMachinePoolMachine object.
func (m *OCIMachinePoolMachine) SetConditions(conditions []metav1.Condition) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&OCIMachinePoolMachine{}, &OCIMachinePoolMachineList{})
}
