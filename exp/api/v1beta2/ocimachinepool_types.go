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

	InstanceVnicConfiguration *MachinePoolNetworkDetails `json:"instanceVnicConfiguration,omitempty"`

	// PlatformConfig defines the platform config parameters
	PlatformConfig *PlatformConfig `json:"platformConfig,omitempty"`

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
	// Supported values are NATIVE, EMULATED, PARAVIRTUALIZED, CUSTOM, and ACCELERATEDPV.
	// +optional
	LaunchMode LaunchModeEnum `json:"launchMode,omitempty"`

	// LicensingConfigs defines licensing configurations associated with target launch values.
	// +optional
	LicensingConfigs []LaunchInstanceLicensingConfig `json:"licensingConfigs,omitempty"`

	// PreferredMaintenanceAction defines the preferred maintenance action for an instance.
	// +optional
	PreferredMaintenanceAction PreferredMaintenanceActionEnum `json:"preferredMaintenanceAction,omitempty"`

	// SecurityAttributes are OCI security attributes for the launched instance.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	SecurityAttributes map[string]map[string]apiextensionsv1.JSON `json:"securityAttributes,omitempty"`

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

	// PrimaryVnicSubnets defines primary VNIC subnet placement details.
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

// MachinePoolNetworkDetails defines the configuration options for the MachinePool primary VNIC.
type MachinePoolNetworkDetails struct {
	// SubnetId defines the ID of the subnet to use. This parameter takes priority over SubnetName.
	SubnetId *string `json:"subnetId,omitempty"`

	// AssignIPv6 determines whether to assign an IPv6 address to the instance.
	// +optional
	AssignIpv6Ip bool `json:"assignIpv6Ip,omitempty"`

	// Ipv6AddressIpv6SubnetCidrPairDetails defines IPv6 CIDR pair details for the primary VNIC.
	// +optional
	Ipv6AddressIpv6SubnetCidrPairDetails []InstanceConfigurationIpv6AddressIpv6SubnetCidrPairDetails `json:"ipv6AddressIpv6SubnetCidrPairDetails,omitempty"`

	// AssignPublicIp defines whether the instance should have a public IP address.
	AssignPublicIp bool `json:"assignPublicIp,omitempty"`

	// SubnetName defines the subnet name to use for the VNIC.
	SubnetName string `json:"subnetName,omitempty"`

	// SkipSourceDestCheck defines whether the source/destination check is disabled on the VNIC.
	SkipSourceDestCheck *bool `json:"skipSourceDestCheck,omitempty"`

	// NSGId defines the ID of the NSG to use. This parameter takes priority over NsgNames.
	// Deprecated, please use MachinePoolNetworkDetails.NSGIds.
	NSGId *string `json:"nsgId,omitempty"`

	// NSGIds defines the list of NSG IDs to use. This parameter takes priority over NsgNames.
	NSGIds []string `json:"nsgIds,omitempty"`

	// NsgNames defines a list of the nsg names of the network security groups (NSGs) to add the VNIC to.
	NsgNames []string `json:"nsgNames,omitempty"`

	// HostnameLabel defines the hostname for the VNIC's primary private IP. Used for DNS.
	HostnameLabel *string `json:"hostnameLabel,omitempty"`

	// DisplayName defines a user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	DisplayName *string `json:"displayName,omitempty"`

	// AssignPrivateDnsRecord defines whether the VNIC should be assigned a DNS record.
	AssignPrivateDnsRecord *bool `json:"assignPrivateDnsRecord,omitempty"`

	// SecurityAttributes are OCI security attributes for the primary VNIC.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	SecurityAttributes map[string]map[string]apiextensionsv1.JSON `json:"securityAttributes,omitempty"`
}

// InstanceConfigurationIpv6AddressIpv6SubnetCidrPairDetails defines an IPv6 address and subnet CIDR pair.
type InstanceConfigurationIpv6AddressIpv6SubnetCidrPairDetails struct {
	// Ipv6SubnetCidr is used to disambiguate which subnet prefix should be used to create an IPv6 allocation.
	// +optional
	Ipv6SubnetCidr *string `json:"ipv6SubnetCidr,omitempty"`

	// Ipv6Address is an available IPv6 address of the subnet from a valid IPv6 prefix.
	// +optional
	Ipv6Address *string `json:"ipv6Address,omitempty"`
}

// InstancePoolPlacementPrimarySubnet defines primary VNIC subnet placement details.
type InstancePoolPlacementPrimarySubnet struct {
	// SubnetId defines the subnet OCID for the primary VNIC.
	// +optional
	SubnetId *string `json:"subnetId,omitempty"`

	// IsAssignIpv6Ip determines whether to assign an IPv6 address at instance and VNIC creation.
	// +optional
	IsAssignIpv6Ip *bool `json:"isAssignIpv6Ip,omitempty"`

	// Ipv6AddressIpv6SubnetCidrPairDetails defines IPv6 CIDR pair details for placement.
	// +optional
	Ipv6AddressIpv6SubnetCidrPairDetails []InstancePoolPlacementIpv6AddressIpv6SubnetCidrDetails `json:"ipv6AddressIpv6SubnetCidrPairDetails,omitempty"`
}

// InstancePoolPlacementIpv6AddressIpv6SubnetCidrDetails defines an IPv6 address and subnet CIDR pair for pool placement.
type InstancePoolPlacementIpv6AddressIpv6SubnetCidrDetails struct {
	// Ipv6SubnetCidr is used to disambiguate which subnet prefix should be used to create an IPv6 allocation.
	// +optional
	Ipv6SubnetCidr *string `json:"ipv6SubnetCidr,omitempty"`
}

// PlatformConfigTypeEnum defines the type of platform configuration.
type PlatformConfigTypeEnum string

const (
	PlatformConfigTypeAmdRomeBmGpu   PlatformConfigTypeEnum = "AMD_ROME_BM_GPU"
	PlatformConfigTypeAmdRomeBm      PlatformConfigTypeEnum = "AMD_ROME_BM"
	PlatformConfigTypeIntelIcelakeBm PlatformConfigTypeEnum = "INTEL_ICELAKE_BM"
	PlatformConfigTypeAmdvm          PlatformConfigTypeEnum = "AMD_VM"
	PlatformConfigTypeIntelVm        PlatformConfigTypeEnum = "INTEL_VM"
	PlatformConfigTypeIntelSkylakeBm PlatformConfigTypeEnum = "INTEL_SKYLAKE_BM"
	PlatformConfigTypeAmdMilanBm     PlatformConfigTypeEnum = "AMD_MILAN_BM"
)

// PlatformConfig defines the platform config parameters.
type PlatformConfig struct {
	PlatformConfigType PlatformConfigTypeEnum `json:"platformConfigType,omitempty"`

	AmdMilanBmPlatformConfig AmdMilanBmPlatformConfig `json:"amdMilanBmPlatformConfig,omitempty"`

	AmdRomeBmPlatformConfig AmdRomeBmPlatformConfig `json:"amdRomeBmPlatformConfig,omitempty"`

	IntelSkylakeBmPlatformConfig IntelSkylakeBmPlatformConfig `json:"intelSkylakeBmPlatformConfig,omitempty"`

	IntelIcelakeBmPlatformConfig IntelIcelakeBmPlatformConfig `json:"intelIcelakeBmPlatformConfig,omitempty"`

	AmdRomeBmGpuPlatformConfig AmdRomeBmGpuPlatformConfig `json:"amdRomeBmGpuPlatformConfig,omitempty"`

	IntelVmPlatformConfig IntelVmPlatformConfig `json:"intelVmPlatformConfig,omitempty"`

	AmdVmPlatformConfig AmdVmPlatformConfig `json:"amdVmPlatformConfig,omitempty"`
}

// AmdMilanBmPlatformConfigNumaNodesPerSocketEnum defines AMD Milan NUMA node options.
type AmdMilanBmPlatformConfigNumaNodesPerSocketEnum string

const (
	AmdMilanBmPlatformConfigNumaNodesPerSocketNps0 AmdMilanBmPlatformConfigNumaNodesPerSocketEnum = "NPS0"
	AmdMilanBmPlatformConfigNumaNodesPerSocketNps1 AmdMilanBmPlatformConfigNumaNodesPerSocketEnum = "NPS1"
	AmdMilanBmPlatformConfigNumaNodesPerSocketNps2 AmdMilanBmPlatformConfigNumaNodesPerSocketEnum = "NPS2"
	AmdMilanBmPlatformConfigNumaNodesPerSocketNps4 AmdMilanBmPlatformConfigNumaNodesPerSocketEnum = "NPS4"
)

// AmdMilanBmPlatformConfig describes AMD Milan BM platform configuration.
type AmdMilanBmPlatformConfig struct {
	IsSecureBootEnabled                      *bool                                          `json:"isSecureBootEnabled,omitempty"`
	IsTrustedPlatformModuleEnabled           *bool                                          `json:"isTrustedPlatformModuleEnabled,omitempty"`
	IsMeasuredBootEnabled                    *bool                                          `json:"isMeasuredBootEnabled,omitempty"`
	IsMemoryEncryptionEnabled                *bool                                          `json:"isMemoryEncryptionEnabled,omitempty"`
	IsSymmetricMultiThreadingEnabled         *bool                                          `json:"isSymmetricMultiThreadingEnabled,omitempty"`
	IsAccessControlServiceEnabled            *bool                                          `json:"isAccessControlServiceEnabled,omitempty"`
	AreVirtualInstructionsEnabled            *bool                                          `json:"areVirtualInstructionsEnabled,omitempty"`
	IsInputOutputMemoryManagementUnitEnabled *bool                                          `json:"isInputOutputMemoryManagementUnitEnabled,omitempty"`
	PercentageOfCoresEnabled                 *int                                           `json:"percentageOfCoresEnabled,omitempty"`
	ConfigMap                                map[string]string                              `json:"configMap,omitempty"`
	NumaNodesPerSocket                       AmdMilanBmPlatformConfigNumaNodesPerSocketEnum `json:"numaNodesPerSocket,omitempty"`
}

// AmdRomeBmPlatformConfigNumaNodesPerSocketEnum defines AMD Rome NUMA node options.
type AmdRomeBmPlatformConfigNumaNodesPerSocketEnum string

const (
	AmdRomeBmPlatformConfigNumaNodesPerSocketNps0 AmdRomeBmPlatformConfigNumaNodesPerSocketEnum = "NPS0"
	AmdRomeBmPlatformConfigNumaNodesPerSocketNps1 AmdRomeBmPlatformConfigNumaNodesPerSocketEnum = "NPS1"
	AmdRomeBmPlatformConfigNumaNodesPerSocketNps2 AmdRomeBmPlatformConfigNumaNodesPerSocketEnum = "NPS2"
	AmdRomeBmPlatformConfigNumaNodesPerSocketNps4 AmdRomeBmPlatformConfigNumaNodesPerSocketEnum = "NPS4"
)

// AmdRomeBmPlatformConfig describes AMD Rome BM platform configuration.
type AmdRomeBmPlatformConfig struct {
	IsSecureBootEnabled                      *bool                                         `json:"isSecureBootEnabled,omitempty"`
	IsTrustedPlatformModuleEnabled           *bool                                         `json:"isTrustedPlatformModuleEnabled,omitempty"`
	IsMeasuredBootEnabled                    *bool                                         `json:"isMeasuredBootEnabled,omitempty"`
	IsMemoryEncryptionEnabled                *bool                                         `json:"isMemoryEncryptionEnabled,omitempty"`
	IsSymmetricMultiThreadingEnabled         *bool                                         `json:"isSymmetricMultiThreadingEnabled,omitempty"`
	IsAccessControlServiceEnabled            *bool                                         `json:"isAccessControlServiceEnabled,omitempty"`
	AreVirtualInstructionsEnabled            *bool                                         `json:"areVirtualInstructionsEnabled,omitempty"`
	IsInputOutputMemoryManagementUnitEnabled *bool                                         `json:"isInputOutputMemoryManagementUnitEnabled,omitempty"`
	PercentageOfCoresEnabled                 *int                                          `json:"percentageOfCoresEnabled,omitempty"`
	ConfigMap                                map[string]string                             `json:"configMap,omitempty"`
	NumaNodesPerSocket                       AmdRomeBmPlatformConfigNumaNodesPerSocketEnum `json:"numaNodesPerSocket,omitempty"`
}

// IntelSkylakeBmPlatformConfigNumaNodesPerSocketEnum defines Intel Skylake NUMA node options.
type IntelSkylakeBmPlatformConfigNumaNodesPerSocketEnum string

const (
	IntelSkylakeBmPlatformConfigNumaNodesPerSocketNps1 IntelSkylakeBmPlatformConfigNumaNodesPerSocketEnum = "NPS1"
	IntelSkylakeBmPlatformConfigNumaNodesPerSocketNps2 IntelSkylakeBmPlatformConfigNumaNodesPerSocketEnum = "NPS2"
)

// IntelSkylakeBmPlatformConfig describes Intel Skylake BM platform configuration.
type IntelSkylakeBmPlatformConfig struct {
	IsSecureBootEnabled                      *bool                                              `json:"isSecureBootEnabled,omitempty"`
	IsTrustedPlatformModuleEnabled           *bool                                              `json:"isTrustedPlatformModuleEnabled,omitempty"`
	IsMeasuredBootEnabled                    *bool                                              `json:"isMeasuredBootEnabled,omitempty"`
	IsMemoryEncryptionEnabled                *bool                                              `json:"isMemoryEncryptionEnabled,omitempty"`
	IsSymmetricMultiThreadingEnabled         *bool                                              `json:"isSymmetricMultiThreadingEnabled,omitempty"`
	IsInputOutputMemoryManagementUnitEnabled *bool                                              `json:"isInputOutputMemoryManagementUnitEnabled,omitempty"`
	PercentageOfCoresEnabled                 *int                                               `json:"percentageOfCoresEnabled,omitempty"`
	ConfigMap                                map[string]string                                  `json:"configMap,omitempty"`
	NumaNodesPerSocket                       IntelSkylakeBmPlatformConfigNumaNodesPerSocketEnum `json:"numaNodesPerSocket,omitempty"`
}

// AmdRomeBmGpuPlatformConfigNumaNodesPerSocketEnum defines AMD Rome GPU NUMA node options.
type AmdRomeBmGpuPlatformConfigNumaNodesPerSocketEnum string

const (
	AmdRomeBmGpuPlatformConfigNumaNodesPerSocketNps0 AmdRomeBmGpuPlatformConfigNumaNodesPerSocketEnum = "NPS0"
	AmdRomeBmGpuPlatformConfigNumaNodesPerSocketNps1 AmdRomeBmGpuPlatformConfigNumaNodesPerSocketEnum = "NPS1"
	AmdRomeBmGpuPlatformConfigNumaNodesPerSocketNps2 AmdRomeBmGpuPlatformConfigNumaNodesPerSocketEnum = "NPS2"
	AmdRomeBmGpuPlatformConfigNumaNodesPerSocketNps4 AmdRomeBmGpuPlatformConfigNumaNodesPerSocketEnum = "NPS4"
)

// AmdRomeBmGpuPlatformConfig describes AMD Rome BM GPU platform configuration.
type AmdRomeBmGpuPlatformConfig struct {
	IsSecureBootEnabled                      *bool                                            `json:"isSecureBootEnabled,omitempty"`
	IsTrustedPlatformModuleEnabled           *bool                                            `json:"isTrustedPlatformModuleEnabled,omitempty"`
	IsMeasuredBootEnabled                    *bool                                            `json:"isMeasuredBootEnabled,omitempty"`
	IsMemoryEncryptionEnabled                *bool                                            `json:"isMemoryEncryptionEnabled,omitempty"`
	IsSymmetricMultiThreadingEnabled         *bool                                            `json:"isSymmetricMultiThreadingEnabled,omitempty"`
	IsAccessControlServiceEnabled            *bool                                            `json:"isAccessControlServiceEnabled,omitempty"`
	AreVirtualInstructionsEnabled            *bool                                            `json:"areVirtualInstructionsEnabled,omitempty"`
	IsInputOutputMemoryManagementUnitEnabled *bool                                            `json:"isInputOutputMemoryManagementUnitEnabled,omitempty"`
	ConfigMap                                map[string]string                                `json:"configMap,omitempty"`
	NumaNodesPerSocket                       AmdRomeBmGpuPlatformConfigNumaNodesPerSocketEnum `json:"numaNodesPerSocket,omitempty"`
}

// IntelIcelakeBmPlatformConfigNumaNodesPerSocketEnum defines Intel Icelake NUMA node options.
type IntelIcelakeBmPlatformConfigNumaNodesPerSocketEnum string

const (
	IntelIcelakeBmPlatformConfigNumaNodesPerSocketNps1 IntelIcelakeBmPlatformConfigNumaNodesPerSocketEnum = "NPS1"
	IntelIcelakeBmPlatformConfigNumaNodesPerSocketNps2 IntelIcelakeBmPlatformConfigNumaNodesPerSocketEnum = "NPS2"
)

// IntelIcelakeBmPlatformConfig describes Intel Icelake BM platform configuration.
type IntelIcelakeBmPlatformConfig struct {
	IsSecureBootEnabled                      *bool                                              `json:"isSecureBootEnabled,omitempty"`
	IsTrustedPlatformModuleEnabled           *bool                                              `json:"isTrustedPlatformModuleEnabled,omitempty"`
	IsMeasuredBootEnabled                    *bool                                              `json:"isMeasuredBootEnabled,omitempty"`
	IsMemoryEncryptionEnabled                *bool                                              `json:"isMemoryEncryptionEnabled,omitempty"`
	IsSymmetricMultiThreadingEnabled         *bool                                              `json:"isSymmetricMultiThreadingEnabled,omitempty"`
	IsInputOutputMemoryManagementUnitEnabled *bool                                              `json:"isInputOutputMemoryManagementUnitEnabled,omitempty"`
	PercentageOfCoresEnabled                 *int                                               `json:"percentageOfCoresEnabled,omitempty"`
	ConfigMap                                map[string]string                                  `json:"configMap,omitempty"`
	NumaNodesPerSocket                       IntelIcelakeBmPlatformConfigNumaNodesPerSocketEnum `json:"numaNodesPerSocket,omitempty"`
}

// IntelVmPlatformConfig describes Intel VM platform configuration.
type IntelVmPlatformConfig struct {
	IsSecureBootEnabled              *bool `json:"isSecureBootEnabled,omitempty"`
	IsTrustedPlatformModuleEnabled   *bool `json:"isTrustedPlatformModuleEnabled,omitempty"`
	IsMeasuredBootEnabled            *bool `json:"isMeasuredBootEnabled,omitempty"`
	IsMemoryEncryptionEnabled        *bool `json:"isMemoryEncryptionEnabled,omitempty"`
	IsSymmetricMultiThreadingEnabled *bool `json:"isSymmetricMultiThreadingEnabled,omitempty"`
}

// AmdVmPlatformConfig describes AMD VM platform configuration.
type AmdVmPlatformConfig struct {
	IsSecureBootEnabled              *bool `json:"isSecureBootEnabled,omitempty"`
	IsTrustedPlatformModuleEnabled   *bool `json:"isTrustedPlatformModuleEnabled,omitempty"`
	IsMeasuredBootEnabled            *bool `json:"isMeasuredBootEnabled,omitempty"`
	IsMemoryEncryptionEnabled        *bool `json:"isMemoryEncryptionEnabled,omitempty"`
	IsSymmetricMultiThreadingEnabled *bool `json:"isSymmetricMultiThreadingEnabled,omitempty"`
}

// LaunchModeEnum defines the launch mode.
type LaunchModeEnum string

const (
	LaunchModeNative          LaunchModeEnum = "NATIVE"
	LaunchModeEmulated        LaunchModeEnum = "EMULATED"
	LaunchModeParavirtualized LaunchModeEnum = "PARAVIRTUALIZED"
	LaunchModeCustom          LaunchModeEnum = "CUSTOM"
	LaunchModeAcceleratedPV   LaunchModeEnum = "ACCELERATEDPV"
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
	Type LaunchInstanceLicensingConfigTypeEnum `json:"type,omitempty"`

	// LicenseType defines the OS license type.
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
