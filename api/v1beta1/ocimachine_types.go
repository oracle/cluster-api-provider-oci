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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

const (
	// MachineFinalizer allows ReconcileAzureMachine to clean up Azure resources associated with AzureMachine before
	// removing it from the apiserver.
	MachineFinalizer = "ocimachine.infrastructure.cluster.x-k8s.io"
)

// OCIMachineSpec defines the desired state of OCIMachine
// Please read the API https://docs.oracle.com/en-us/iaas/api/#/en/iaas/20160918/Instance/LaunchInstance
// for more information about the parameters below
type OCIMachineSpec struct {
	// OCID of launched compute instance.
	// +optional
	InstanceId *string `json:"instanceId,omitempty"`

	// OCID of the image to be used to launch the instance.
	ImageId string `json:"imageId,omitempty"`

	// Compartment to launch the instance in.
	CompartmentId string `json:"compartmentId,omitempty"`

	// Shape of the instance.
	Shape string `json:"shape,omitempty"`

	// The shape configuration of rhe instance, applicable for flex instances.
	ShapeConfig ShapeConfig `json:"shapeConfig,omitempty"`

	// PrimaryNetworkInterface is required to specify subnet.
	NetworkDetails NetworkDetails `json:"networkDetails,omitempty"`

	// Provider ID of the instance, this will be set by Cluster API provider itself,
	// users should not set this parameter.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Is in transit encryption of volumes required.
	// +kubebuilder:default=true
	// +optional
	IsPvEncryptionInTransitEnabled bool `json:"isPvEncryptionInTransitEnabled,omitempty"`

	// The size of boot volume. Please see https://docs.oracle.com/en-us/iaas/Content/Block/Tasks/extendingbootpartition.htm
	// to extend the boot volume size.
	BootVolumeSizeInGBs string `json:"bootVolumeSizeInGBs,omitempty"`

	// Custom metadata key/value pairs that you provide, such as the SSH public key
	// required to connect to the instance.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Free-form tags for this resource.
	// +optional
	FreeformTags map[string]string `json:"freeformTags,omitempty"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	// +optional
	DefinedTags map[string]map[string]string `json:"definedTags,omitempty"`

	// The name of the subnet to use. The name here refers to the subnets
	// defined in the OCICluster Spec. Optional, only if multiple subnets of a type
	// is defined, else the first element is used.
	// +optional
	SubnetName string `json:"subnetName,omitempty"`

	// The name of NSG to use. The name here refers to the NSGs
	// defined in the OCICluster Spec. Optional, only if multiple NSGs of a type
	// is defined, else the first element is used.
	// +optional
	NSGName string `json:"nsgName,omitempty"`
}

// OCIMachineStatus defines the observed state of OCIMachine.
type OCIMachineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of machine

	// Flag set to true when machine is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Addresses contains the addresses of the associated OCI instance.
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// Error status on the machine.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// The error message corresponding to the error on the machine.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Launch instance work request ID.
	// +optional
	LaunchInstanceWorkRequestId string `json:"launchInstanceWorkRequestId,omitempty"`

	// Create Backend OPC work request ID for the machine backend.
	// +optional
	CreateBackendWorkRequestId string `json:"createBackendWorkRequestId,omitempty"`

	// Delete Backend OPC work request ID for the machine backend.
	// +optional
	DeleteBackendWorkRequestId string `json:"deleteBackendWorkRequestId,omitempty"`

	// Conditions defines current service state of the OCIMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OCIMachine is the Schema for the ocimachines API.
type OCIMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIMachineSpec   `json:"spec,omitempty"`
	Status OCIMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OCIMachineList contains a list of OCIMachine.
type OCIMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIMachine `json:"items"`
}

// GetConditions returns the list of conditions for an OCIMachine API object.
func (m *OCIMachine) GetConditions() clusterv1.Conditions {
	return m.Status.Conditions
}

// SetConditions will set the given conditions on an OCIMachine object.
func (m *OCIMachine) SetConditions(conditions clusterv1.Conditions) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&OCIMachine{}, &OCIMachineList{})
}
