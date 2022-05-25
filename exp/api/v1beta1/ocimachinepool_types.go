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
	"sigs.k8s.io/cluster-api/errors"
)

// +kubebuilder:object:generate=true
// +groupName=infrastructure.cluster.x-k8s.io

// Constants block.
const (
	// MachinePoolFinalizer is the finalizer for the machine pool.
	MachinePoolFinalizer = "ocimachinepool.infrastructure.cluster.x-k8s.io"
)

// OCIMachinePoolSpec defines the desired state of OCIMachinePool
type OCIMachinePoolSpec struct {
	// ProviderID is the OCID of the associated InstancePool in a provider format
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// OCID is the OCID of the associated InstancePool
	// +optional
	OCID string `json:"ocid,omitempty"`

	// OCID of the image to be used to launch the instance.
	ImageId string `json:"imageId,omitempty"`

	// Custom metadata key/value pairs that you provide, such as the SSH public key
	// required to connect to the instance.
	Metadata map[string]string `json:"metadata,omitempty"`

	// The shape configuration of the instance, applicable for flex instances.
	ShapeConfig ShapeConfig `json:"shapeConfig,omitempty"`

	// Whether the VNIC should be assigned a public IP address.
	// +kubebuilder:default=false
	// +optional
	VNICAssignPublicIp bool `json:"vnicAssignPublicIp,omitempty"`

	PlacementDetails []PlacementDetails `json:"placementDetails,omitempty"`

	InstanceConfiguration InstanceConfiguration `json:"instanceConfiguration,omitempty"`

	// ProviderIDList are the identification IDs of machine instances provided by the provider.
	// This field must match the provider IDs as seen on the node objects corresponding to a machine pool's machine instances.
	// +optional
	ProviderIDList []string `json:"providerIDList,omitempty"`
}

type InstanceConfiguration struct {
	InstanceConfigurationId string          `json:"instanceConfigurationId,omitempty"`
	InstanceDetails         InstanceDetails `json:"instanceDetails,omitempty"`
}

type PlacementDetails struct {
	// The availability domain to place instances.
	AvailabilityDomain int `mandatory:"true" json:"availabilityDomain"`
}

type InstanceDetails struct {
	Shape string `json:"shape,omitempty"`
}

// LaunchDetails Instance launch details for creating an instance from an instance configuration
// https://docs.oracle.com/en-us/iaas/api/#/en/iaas/20160918/datatypes/InstanceConfigurationLaunchInstanceDetails
type LaunchDetails struct {
	// Custom metadata key/value pairs that you provide, such as the SSH public key
	// required to connect to the instance.
	Metadata map[string]string `json:"metadata,omitempty"`

	Shape string `json:"shape,omitempty"`
}

// ShapeConfig defines the configuration options for the compute instance shape
// https://docs.oracle.com/en-us/iaas/api/#/en/iaas/20160918/datatypes/LaunchInstanceShapeConfigDetails
type ShapeConfig struct {
	// The total number of OCPUs available to the instance.
	Ocpus string `json:"ocpus,omitempty"`

	// The total amount of memory available to the instance, in gigabytes.
	MemoryInGBs string `json:"memoryInGBs,omitempty"`

	// The baseline OCPU utilization for a subcore burstable VM instance. Leave this attribute blank for a
	// non-burstable instance, or explicitly specify non-burstable with `BASELINE_1_1`.
	// The following values are supported:
	// - `BASELINE_1_8` - baseline usage is 1/8 of an OCPU.
	// - `BASELINE_1_2` - baseline usage is 1/2 of an OCPU.
	// - `BASELINE_1_1` - baseline usage is an entire OCPU. This represents a non-burstable instance.
	BaselineOcpuUtilization string `json:"baselineOcpuUtilization,omitempty"`
}

// OCIMachinePoolStatus defines the observed state of OCIMachinePool
type OCIMachinePoolStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Replicas is the most recently observed number of replicas
	// +optional
	Replicas int32 `json:"replicas"`

	// Conditions defines current service state of the OCIMachinePool.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	FailureMessage *string `json:"failureMessage,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type OCIMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIMachinePoolSpec   `json:"spec,omitempty"`
	Status OCIMachinePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OCIMachinePoolList contains a list of OCIMachinePool.
type OCIMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIMachinePool `json:"items"`
}

// GetConditions returns the list of conditions for an OCIMachine API object.
func (m *OCIMachinePool) GetConditions() clusterv1.Conditions {
	return m.Status.Conditions
}

// SetConditions will set the given conditions on an OCIMachine object.
func (m *OCIMachinePool) SetConditions(conditions clusterv1.Conditions) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&OCIMachinePool{}, &OCIMachinePoolList{})
}
