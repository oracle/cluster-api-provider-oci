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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

const (
	// MachineFinalizer allows ReconcileMachine to clean up OCI resources associated with OCIMachine before
	// removing it from the apiserver.
	MachineFinalizer                   = "ocimachine.infrastructure.cluster.x-k8s.io"
	DeleteMachineOnInstanceTermination = "ociclusters.x-k8s.io/delete-machine-on-instance-termination"
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

	// ComputeClusterId refers to OCID of the compute cluster that the instance will be created in.
	// Please refer https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/compute-clusters.htm for more details
	ComputeClusterId *string `json:"computeClusterId,omitempty"`

	// IpxeScript is the  custom iPXE script that will run when the instance boots.
	IpxeScript *string `json:"ipxeScript,omitempty"`

	// CapacityReservationId defines the OCID of the compute capacity reservation this instance is launched under.
	// You can opt out of all default reservations by specifying an empty string as input for this field.
	// For more information, see Capacity Reservations (https://docs.cloud.oracle.com/iaas/Content/Compute/Tasks/reserve-capacity.htm#default).
	CapacityReservationId *string `json:"capacityReservationId,omitempty"`

	// The shape configuration of rhe instance, applicable for flex instances.
	ShapeConfig ShapeConfig `json:"shapeConfig,omitempty"`

	// NetworkDetails defines the configuration options for the network
	NetworkDetails NetworkDetails `json:"networkDetails,omitempty"`

	// VnicAttachments defines the configuration options for the vnic(s) attached to the machine
	// The network bandwidth and number of VNICs scale proportionately with the number of OCPUs.
	VnicAttachments []VnicAttachment `json:"vnicAttachments,omitempty"`

	// LaunchOptions defines the options for tuning the compatibility and performance of VM shapes
	LaunchOptions *LaunchOptions `json:"launchOptions,omitempty"`

	// InstanceOptions defines the instance options
	InstanceOptions *InstanceOptions `json:"instanceOptions,omitempty"`

	// LaunchInstanceAvailabilityConfig defines the options for VM migration during infrastructure maintenance events and for defining
	// the availability of a VM instance after a maintenance event that impacts the underlying hardware.
	AvailabilityConfig *LaunchInstanceAvailabilityConfig `json:"availabilityConfig,omitempty"`

	// PreemptibleInstanceConfig Configuration options for preemptible instances.
	PreemptibleInstanceConfig *PreemptibleInstanceConfig `json:"preemptibleInstanceConfig,omitempty"`

	// AgentConfig defines the options for the Oracle Cloud Agent software running on the instance.
	AgentConfig *LaunchInstanceAgentConfig `json:"agentConfig,omitempty"`

	// InstanceSourceViaImageConfig defines the options for booting up instances via images
	InstanceSourceViaImageDetails *InstanceSourceViaImageConfig `json:"instanceSourceViaImageConfig,omitempty"`

	// PlatformConfig defines the platform config parameters
	PlatformConfig *PlatformConfig `json:"platformConfig,omitempty"`

	// DedicatedVmHostId defines the OCID of the dedicated VM host.
	DedicatedVmHostId *string `json:"dedicatedVmHostId,omitempty"`

	// Provider ID of the instance, this will be set by Cluster API provider itself,
	// users should not set this parameter.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Is in transit encryption of volumes required.
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

	// Volume attachments to create as part of the launch instance operation.
	LaunchVolumeAttachment []LaunchVolumeAttachment `json:"launchVolumeAttachments,omitempty"`

	// The name of the subnet to use. The name here refers to the subnets
	// defined in the OCICluster Spec. Optional, only if multiple subnets of a type
	// is defined, else the first element is used.
	// +optional
	SubnetName string `json:"subnetName,omitempty"`

	// Specifies whether to delete or preserve the boot volume when terminating an instance.
	// When set to true, the boot volume is preserved. The default value is false.
	PreserveBootVolume bool `json:"preserveBootVolume,omitempty"`

	// Specifies whether to delete or preserve the data volumes created during launch when
	//terminating an instance. When set to true, the data volumes are preserved. The default value is true.
	PreserveDataVolumesCreatedAtLaunch bool `json:"preserveDataVolumesCreatedAtLaunch,omitempty"`
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
	FailureReason *string `json:"failureReason,omitempty"`

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
// +kubebuilder:storageversion

// OCIMachine is the Schema for the ocimachines API.
type OCIMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIMachineSpec   `json:"spec,omitempty"`
	Status OCIMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +kubebuilder:storageversion

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
