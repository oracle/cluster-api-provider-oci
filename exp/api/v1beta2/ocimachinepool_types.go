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

package v1beta2

import (
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// +kubebuilder:object:generate=true
// +groupName=infrastructure.cluster.x-k8s.io

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
	OCID *string `json:"ocid,omitempty"`

	// PlacementDetails defines the placement details of the instance pool.
	PlacementDetails []PlacementDetails `json:"placementDetails,omitempty"`

	// InstanceConfiguration defines the configuration of the instance pool instances.
	InstanceConfiguration InstanceConfiguration `json:"instanceConfiguration,omitempty"`

	// InstanceDisplayNameFormatter defines a formatter for instance display names in the instance pool.
	// +optional
	InstanceDisplayNameFormatter *string `json:"instanceDisplayNameFormatter,omitempty"`

	// InstanceHostnameFormatter defines a formatter for instance hostnames in the instance pool.
	// +optional
	InstanceHostnameFormatter *string `json:"instanceHostnameFormatter,omitempty"`

	// ProviderIDList are the identification IDs of machine instances provided by the provider.
	// This field must match the provider IDs as seen on the node objects corresponding to a machine pool's machine instances.
	// +optional
	ProviderIDList []string `json:"providerIDList,omitempty"`
}

type InstanceConfiguration struct {
	InstanceConfigurationId *string `json:"instanceConfigurationId,omitempty"`
	Shape                   *string `json:"shape,omitempty"`
	// The shape configuration of the instance, applicable for flex instances.
	ShapeConfig *ShapeConfig `json:"shapeConfig,omitempty"`

	// Free-form tags for instances launched by this machine pool.
	// These tags override OCICluster freeform tags with the same key.
	// +optional
	FreeformTags map[string]string `json:"freeformTags,omitempty"`

	// Defined tags for instances launched by this machine pool. Each key is
	// predefined and scoped to a namespace.
	// These tags override OCICluster defined tags with the same namespace/key.
	// Example: `{"Operations": {"CostCenter": "42"}}`
	// +optional
	DefinedTags map[string]map[string]string `json:"definedTags,omitempty"`

	InstanceVnicConfiguration *infrastructurev1beta2.NetworkDetails `json:"instanceVnicConfiguration,omitempty"`

	// PlatformConfig defines the platform config parameters
	PlatformConfig *infrastructurev1beta2.PlatformConfig `json:"platformConfig,omitempty"`

	// AgentConfig defines the options for the Oracle Cloud Agent software running on the instance.
	AgentConfig *infrastructurev1beta2.LaunchInstanceAgentConfig `json:"agentConfig,omitempty"`

	// PreemptibleInstanceConfig Configuration options for preemptible instances.
	PreemptibleInstanceConfig *infrastructurev1beta2.PreemptibleInstanceConfig `json:"preemptibleInstanceConfig,omitempty"`

	// LaunchInstanceAvailabilityConfig defines the options for VM migration during infrastructure maintenance events and for defining
	// the availability of a VM instance after a maintenance event that impacts the underlying hardware.
	AvailabilityConfig *infrastructurev1beta2.LaunchInstanceAvailabilityConfig `json:"availabilityConfig,omitempty"`

	// DedicatedVmHostId defines the OCID of the dedicated VM host.
	DedicatedVmHostId *string `json:"dedicatedVmHostId,omitempty"`

	// LaunchOptions defines the options for tuning the compatibility and performance of VM shapes
	LaunchOptions *infrastructurev1beta2.LaunchOptions `json:"launchOptions,omitempty"`

	// InstanceOptions defines the instance options
	InstanceOptions *infrastructurev1beta2.InstanceOptions `json:"instanceOptions,omitempty"`

	// Is in transit encryption of volumes required.
	// +optional
	IsPvEncryptionInTransitEnabled *bool `json:"isPvEncryptionInTransitEnabled,omitempty"`

	// InstanceSourceViaImageConfig defines the options for booting up instances via images
	InstanceSourceViaImageDetails *InstanceSourceViaImageConfig `json:"instanceSourceViaImageConfig,omitempty"`

	// CapacityReservationId defines the OCID of the compute capacity reservation this instance is launched under.
	// You can opt out of all default reservations by specifying an empty string as input for this field.
	// For more information, see Capacity Reservations (https://docs.cloud.oracle.com/iaas/Content/Compute/Tasks/reserve-capacity.htm#default).
	CapacityReservationId *string `json:"capacityReservationId,omitempty"`

	// ClusterPlacementGroupId defines the OCID of the cluster placement group of the instance.
	// +optional
	ClusterPlacementGroupId *string `json:"clusterPlacementGroupId,omitempty"`

	// IpxeScript is the custom iPXE script that will run when the instance boots.
	// +optional
	IpxeScript *string `json:"ipxeScript,omitempty"`

	// LaunchMode specifies the configuration mode for launching virtual machine instances.
	// +kubebuilder:validation:Enum=NATIVE;EMULATED;PARAVIRTUALIZED;CUSTOM
	// +optional
	LaunchMode LaunchModeEnum `json:"launchMode,omitempty"`

	// LicensingConfigs defines licensing configurations associated with target launch values.
	// +kubebuilder:validation:MaxItems=1
	// +optional
	LicensingConfigs []LaunchInstanceLicensingConfig `json:"licensingConfigs,omitempty"`

	// PreferredMaintenanceAction defines the preferred maintenance action for an instance.
	// +kubebuilder:validation:Enum=LIVE_MIGRATE;REBOOT
	// +optional
	PreferredMaintenanceAction PreferredMaintenanceActionEnum `json:"preferredMaintenanceAction,omitempty"`

	// Custom metadata key/value pairs that you provide, such as the SSH public key
	// required to connect to the instance.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Additional metadata key/value pairs that provide non-string values.
	// Values support nested JSON objects and arrays and map to OCI InstanceConfigurationLaunchInstanceDetails.extendedMetadata.
	// +kubebuilder:pruning:PreserveUnknownFields
	ExtendedMetadata map[string]apiextensionsv1.JSON `json:"extendedMetadata,omitempty"`
}

type PlacementDetails struct {
	// The availability domain to place instances.
	AvailabilityDomain int `mandatory:"true" json:"availabilityDomain"`

	// FaultDomains defines the list of fault domains in which to place instances.
	// If omitted, all known fault domains for the availability domain are used.
	// +optional
	FaultDomains []string `json:"faultDomains,omitempty"`

	// PrimaryVnicSubnets defines primary VNIC subnet placement details.
	// When set, overrides the cluster-level default worker subnet for this availability domain.
	// +optional
	PrimaryVnicSubnets *InstancePoolPlacementPrimarySubnet `json:"primaryVnicSubnets,omitempty"`
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
	Ocpus *string `json:"ocpus,omitempty"`

	// The total number of VCPUs available to the instance.
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:MultipleOf=2
	Vcpus *int `json:"vcpus,omitempty"`

	// The total amount of memory available to the instance, in gigabytes.
	MemoryInGBs *string `json:"memoryInGBs,omitempty"`

	// The baseline OCPU utilization for a subcore burstable VM instance. Leave this attribute blank for a
	// non-burstable instance, or explicitly specify non-burstable with `BASELINE_1_1`.
	// The following values are supported:
	// - `BASELINE_1_8` - baseline usage is 1/8 of an OCPU.
	// - `BASELINE_1_2` - baseline usage is 1/2 of an OCPU.
	// - `BASELINE_1_1` - baseline usage is an entire OCPU. This represents a non-burstable instance.
	BaselineOcpuUtilization string `json:"baselineOcpuUtilization,omitempty"`

	// Nvmes defines the number of NVMe drives to be used for storage. A single drive has 6.8 TB available.
	Nvmes *int `json:"nvmes,omitempty"`
}

// InstancePoolPlacementPrimarySubnet defines primary VNIC subnet placement details.
type InstancePoolPlacementPrimarySubnet struct {
	// SubnetId defines the subnet OCID for the primary VNIC.
	// +optional
	SubnetId *string `json:"subnetId,omitempty"`

	// IsAssignIpv6Ip determines whether to assign an IPv6 address at instance and VNIC creation.
	// +optional
	IsAssignIpv6Ip *bool `json:"isAssignIpv6Ip,omitempty"`
}

// LaunchModeEnum defines the launch mode.
type LaunchModeEnum string

const (
	LaunchModeNative          LaunchModeEnum = "NATIVE"
	LaunchModeEmulated        LaunchModeEnum = "EMULATED"
	LaunchModeParavirtualized LaunchModeEnum = "PARAVIRTUALIZED"
	LaunchModeCustom          LaunchModeEnum = "CUSTOM"
)

// PreferredMaintenanceActionEnum defines the preferred maintenance action.
type PreferredMaintenanceActionEnum string

const (
	PreferredMaintenanceActionLiveMigrate PreferredMaintenanceActionEnum = "LIVE_MIGRATE"
	PreferredMaintenanceActionReboot      PreferredMaintenanceActionEnum = "REBOOT"
)

// LaunchInstanceLicensingConfig defines a launch licensing configuration.
type LaunchInstanceLicensingConfig struct {
	// Type identifies the licensing config type.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=WINDOWS
	Type LaunchInstanceLicensingConfigTypeEnum `json:"type,omitempty"`

	// LicenseType defines the OS license type.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=OCI_PROVIDED;BRING_YOUR_OWN_LICENSE
	LicenseType LaunchInstanceLicensingConfigLicenseTypeEnum `json:"licenseType,omitempty"`
}

// LaunchInstanceLicensingConfigTypeEnum defines the licensing config type.
type LaunchInstanceLicensingConfigTypeEnum string

const (
	LaunchInstanceLicensingConfigTypeWindows LaunchInstanceLicensingConfigTypeEnum = "WINDOWS"
)

// LaunchInstanceLicensingConfigLicenseTypeEnum defines the licensing type.
type LaunchInstanceLicensingConfigLicenseTypeEnum string

const (
	LaunchInstanceLicensingConfigLicenseTypeOCIProvided         LaunchInstanceLicensingConfigLicenseTypeEnum = "OCI_PROVIDED"
	LaunchInstanceLicensingConfigLicenseTypeBringYourOwnLicense LaunchInstanceLicensingConfigLicenseTypeEnum = "BRING_YOUR_OWN_LICENSE"
)

// InstanceSourceViaImageConfig The configuration options for booting up instances via images
type InstanceSourceViaImageConfig struct {
	// OCID of the image to be used to launch the instance.
	ImageId *string `json:"imageId,omitempty"`

	// KmsKeyId defines the OCID of the Key Management key to assign as the master encryption key for the boot volume.
	KmsKeyId *string `json:"kmsKeyId,omitempty"`

	// The size of boot volume. Please see https://docs.oracle.com/en-us/iaas/Content/Block/Tasks/extendingbootpartition.htm
	// to extend the boot volume size.
	BootVolumeSizeInGBs *int64 `json:"bootVolumeSizeInGBs,omitempty"`

	// BootVolumeVpusPerGB defines the number of volume performance units (VPUs) that will be applied to this volume per GB,
	// representing the Block Volume service's elastic performance options.
	// See Block Volume Performance Levels (https://docs.cloud.oracle.com/iaas/Content/Block/Concepts/blockvolumeperformance.htm#perf_levels) for more information.
	// Allowed values:
	//   * `10`: Represents Balanced option.
	//   * `20`: Represents Higher Performance option.
	//   * `30`-`120`: Represents the Ultra High Performance option.
	// For volumes with the auto-tuned performance feature enabled, this is set to the default (minimum) VPUs/GB.
	BootVolumeVpusPerGB *int64 `json:"bootVolumeVpusPerGB,omitempty"`
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
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	FailureReason *string `json:"failureReason,omitempty"`

	FailureMessage *string `json:"failureMessage,omitempty"`

	// InfrastructureMachineKind is the kind of the infrastructure resources behind MachinePool Machines.
	// +optional
	InfrastructureMachineKind string `json:"infrastructureMachineKind,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:storageversion

type OCIMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIMachinePoolSpec   `json:"spec,omitempty"`
	Status OCIMachinePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// OCIMachinePoolList contains a list of OCIMachinePool.
type OCIMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIMachinePool `json:"items"`
}

// GetConditions returns the list of conditions for an OCIMachine API object.
func (m *OCIMachinePool) GetConditions() clusterv1beta1.Conditions {
	return m.Status.Conditions
}

// SetConditions will set the given conditions on an OCIMachine object.
func (m *OCIMachinePool) SetConditions(conditions clusterv1beta1.Conditions) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&OCIMachinePool{}, &OCIMachinePoolList{})
}
