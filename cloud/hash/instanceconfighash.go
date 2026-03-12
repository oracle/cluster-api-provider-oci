/*
 Copyright (c) 2022 Oracle and/or its affiliates.

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

package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

type comparableLaunchDetails struct {
	CapacityReservationID          *string                              `json:"capacityReservationId,omitempty"`
	CompartmentID                  *string                              `json:"compartmentId,omitempty"`
	CreateVnicDetails              *comparableCreateVnicDetails         `json:"createVnicDetails,omitempty"`
	Metadata                       map[string]string                    `json:"metadata,omitempty"`
	Shape                          *string                              `json:"shape,omitempty"`
	ShapeConfig                    *comparableShapeConfig               `json:"shapeConfig,omitempty"`
	PlatformConfig                 *comparablePlatformConfig            `json:"platformConfig,omitempty"`
	SourceDetails                  *comparableSourceDetails             `json:"sourceDetails,omitempty"`
	DedicatedVMHostID              *string                              `json:"dedicatedVmHostId,omitempty"`
	LaunchOptions                  *comparableLaunchOptions             `json:"launchOptions,omitempty"`
	AgentConfig                    *comparableAgentConfig               `json:"agentConfig,omitempty"`
	IsPVEncryptionInTransitEnabled *bool                                `json:"isPvEncryptionInTransitEnabled,omitempty"`
	InstanceOptions                *comparableInstanceOptions           `json:"instanceOptions,omitempty"`
	AvailabilityConfig             *comparableAvailabilityConfig        `json:"availabilityConfig,omitempty"`
	PreemptibleInstanceConfig      *comparablePreemptibleInstanceConfig `json:"preemptibleInstanceConfig,omitempty"`
}

type comparableCreateVnicDetails struct {
	AssignIPv6IP           *bool    `json:"assignIpv6Ip,omitempty"`
	AssignPublicIP         *bool    `json:"assignPublicIp,omitempty"`
	AssignPrivateDNSRecord *bool    `json:"assignPrivateDnsRecord,omitempty"`
	HostnameLabel          *string  `json:"hostnameLabel,omitempty"`
	NSGIDs                 []string `json:"nsgIds,omitempty"`
	PrivateIP              *string  `json:"privateIp,omitempty"`
	SkipSourceDestCheck    *bool    `json:"skipSourceDestCheck,omitempty"`
	SubnetID               *string  `json:"subnetId,omitempty"`
}

type comparableShapeConfig struct {
	OCPUs                   *float32 `json:"ocpus,omitempty"`
	MemoryInGBs             *float32 `json:"memoryInGBs,omitempty"`
	VCPUs                   *int     `json:"vcpus,omitempty"`
	NVMEs                   *int     `json:"nvmes,omitempty"`
	BaselineOCPUUtilization string   `json:"baselineOcpuUtilization,omitempty"`
}

type comparablePlatformConfig struct {
	Type                                     string `json:"type,omitempty"`
	IsSecureBootEnabled                      *bool  `json:"isSecureBootEnabled,omitempty"`
	IsTrustedPlatformModuleEnabled           *bool  `json:"isTrustedPlatformModuleEnabled,omitempty"`
	IsMeasuredBootEnabled                    *bool  `json:"isMeasuredBootEnabled,omitempty"`
	IsMemoryEncryptionEnabled                *bool  `json:"isMemoryEncryptionEnabled,omitempty"`
	IsSymmetricMultiThreadingEnabled         *bool  `json:"isSymmetricMultiThreadingEnabled,omitempty"`
	IsAccessControlServiceEnabled            *bool  `json:"isAccessControlServiceEnabled,omitempty"`
	AreVirtualInstructionsEnabled            *bool  `json:"areVirtualInstructionsEnabled,omitempty"`
	IsInputOutputMemoryManagementUnitEnabled *bool  `json:"isInputOutputMemoryManagementUnitEnabled,omitempty"`
	PercentageOfCoresEnabled                 *int   `json:"percentageOfCoresEnabled,omitempty"`
	NumaNodesPerSocket                       string `json:"numaNodesPerSocket,omitempty"`
}

type comparableSourceDetails struct {
	SourceType          string  `json:"sourceType,omitempty"`
	ImageID             *string `json:"imageId,omitempty"`
	KMSKeyID            *string `json:"kmsKeyId,omitempty"`
	BootVolumeSizeInGBs *int64  `json:"bootVolumeSizeInGBs,omitempty"`
	BootVolumeVPUsPerGB *int64  `json:"bootVolumeVpusPerGB,omitempty"`
}

type comparableLaunchOptions struct {
	BootVolumeType                  string `json:"bootVolumeType,omitempty"`
	Firmware                        string `json:"firmware,omitempty"`
	NetworkType                     string `json:"networkType,omitempty"`
	RemoteDataVolumeType            string `json:"remoteDataVolumeType,omitempty"`
	IsConsistentVolumeNamingEnabled *bool  `json:"isConsistentVolumeNamingEnabled,omitempty"`
}

type comparableAgentConfig struct {
	IsMonitoringDisabled  *bool                    `json:"isMonitoringDisabled,omitempty"`
	IsManagementDisabled  *bool                    `json:"isManagementDisabled,omitempty"`
	AreAllPluginsDisabled *bool                    `json:"areAllPluginsDisabled,omitempty"`
	PluginsConfig         []comparablePluginConfig `json:"pluginsConfig,omitempty"`
}

type comparablePluginConfig struct {
	Name         *string `json:"name,omitempty"`
	DesiredState string  `json:"desiredState,omitempty"`
}

type comparableInstanceOptions struct {
	AreLegacyIMDSEndpointsDisabled *bool `json:"areLegacyImdsEndpointsDisabled,omitempty"`
}

type comparableAvailabilityConfig struct {
	IsLiveMigrationPreferred *bool  `json:"isLiveMigrationPreferred,omitempty"`
	RecoveryAction           string `json:"recoveryAction,omitempty"`
}

type comparablePreemptibleInstanceConfig struct {
	PreserveBootVolume *bool `json:"preserveBootVolume,omitempty"`
}

// ComputeHash computes a SHA-256 hash of normalized launch details
func ComputeHash(ld *core.InstanceConfigurationLaunchInstanceDetails) (string, error) {
	return computeProjectedHash(projectLaunchDetails(ld, ld))
}

// ComputeComparableHash computes the hash of the actual launch details projected onto the desired shape.
func ComputeComparableHash(actual, desired *core.InstanceConfigurationLaunchInstanceDetails) (string, error) {
	return computeProjectedHash(projectLaunchDetails(actual, desired))
}

func computeProjectedHash(projected *comparableLaunchDetails) (string, error) {
	b, err := json.Marshal(projected)
	if err != nil {
		return "", errors.Wrap(err, "marshal normalized launch details")
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeLaunchDetails(in *core.InstanceConfigurationLaunchInstanceDetails) *comparableLaunchDetails {
	return projectLaunchDetails(in, in)
}

func projectLaunchDetails(in, mask *core.InstanceConfigurationLaunchInstanceDetails) *comparableLaunchDetails {
	if in == nil {
		return nil
	}
	if mask == nil {
		mask = in
	}

	return &comparableLaunchDetails{
		CapacityReservationID:          pickString(in.CapacityReservationId, mask.CapacityReservationId),
		CompartmentID:                  pickString(in.CompartmentId, mask.CompartmentId),
		CreateVnicDetails:              projectCreateVnicDetails(in.CreateVnicDetails, mask.CreateVnicDetails),
		Metadata:                       normalizeMetadata(pickMetadata(in.Metadata, mask.Metadata)),
		Shape:                          pickString(in.Shape, mask.Shape),
		ShapeConfig:                    projectShapeConfig(in.ShapeConfig, mask.ShapeConfig),
		PlatformConfig:                 projectPlatformConfig(in.PlatformConfig, mask.PlatformConfig),
		SourceDetails:                  projectSourceDetails(in.SourceDetails, mask.SourceDetails),
		DedicatedVMHostID:              pickString(in.DedicatedVmHostId, mask.DedicatedVmHostId),
		LaunchOptions:                  projectLaunchOptions(in.LaunchOptions, mask.LaunchOptions),
		AgentConfig:                    projectAgentConfig(in.AgentConfig, mask.AgentConfig),
		IsPVEncryptionInTransitEnabled: pickDefaultFalseBool(in.IsPvEncryptionInTransitEnabled, mask.IsPvEncryptionInTransitEnabled),
		InstanceOptions:                projectInstanceOptions(in.InstanceOptions, mask.InstanceOptions),
		AvailabilityConfig:             projectAvailabilityConfig(in.AvailabilityConfig, mask.AvailabilityConfig),
		PreemptibleInstanceConfig:      projectPreemptibleInstanceConfig(in.PreemptibleInstanceConfig, mask.PreemptibleInstanceConfig),
	}
}

// normalizeMetadata filters instance metadata to exclude fields like user_data
func normalizeMetadata(md map[string]string) map[string]string {
	if md == nil {
		return nil
	}
	output := make(map[string]string, len(md))
	for k, v := range md {
		// exclude user_data
		if k == "user_data" {
			continue
		}
		output[k] = v
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

// hashChanged returns true if the two hashes are different, indicating a configuration change
func hashChanged(hash1, hash2 string) bool {
	return hash1 != hash2
}

// launchDetailsEqual returns true if two launch details are equivalent after normalization
func launchDetailsEqual(ld1, ld2 *core.InstanceConfigurationLaunchInstanceDetails) bool {
	normalized1 := normalizeLaunchDetails(ld1)
	normalized2 := normalizeLaunchDetails(ld2)
	return reflect.DeepEqual(normalized1, normalized2)
}

func projectCreateVnicDetails(in, mask *core.InstanceConfigurationCreateVnicDetails) *comparableCreateVnicDetails {
	if in == nil || mask == nil {
		return nil
	}

	nsgIDs := pickStrings(in.NsgIds, mask.NsgIds)
	if len(nsgIDs) > 0 {
		sort.Strings(nsgIDs)
	}

	result := &comparableCreateVnicDetails{
		AssignIPv6IP:           pickDefaultFalseBool(in.AssignIpv6Ip, mask.AssignIpv6Ip),
		AssignPublicIP:         pickDefaultFalseBool(in.AssignPublicIp, mask.AssignPublicIp),
		AssignPrivateDNSRecord: pickBool(in.AssignPrivateDnsRecord, mask.AssignPrivateDnsRecord),
		HostnameLabel:          pickString(in.HostnameLabel, mask.HostnameLabel),
		NSGIDs:                 nsgIDs,
		PrivateIP:              pickString(in.PrivateIp, mask.PrivateIp),
		SkipSourceDestCheck:    pickDefaultFalseBool(in.SkipSourceDestCheck, mask.SkipSourceDestCheck),
		SubnetID:               pickString(in.SubnetId, mask.SubnetId),
	}
	if reflect.DeepEqual(result, &comparableCreateVnicDetails{}) {
		return nil
	}
	return result
}

func projectShapeConfig(in, mask *core.InstanceConfigurationLaunchInstanceShapeConfigDetails) *comparableShapeConfig {
	if in == nil || mask == nil {
		return nil
	}
	result := &comparableShapeConfig{
		OCPUs:                   pickFloat32(in.Ocpus, mask.Ocpus),
		MemoryInGBs:             pickFloat32(in.MemoryInGBs, mask.MemoryInGBs),
		VCPUs:                   pickInt(in.Vcpus, mask.Vcpus),
		NVMEs:                   pickInt(in.Nvmes, mask.Nvmes),
		BaselineOCPUUtilization: pickEnum(string(in.BaselineOcpuUtilization), string(mask.BaselineOcpuUtilization)),
	}
	if reflect.DeepEqual(result, &comparableShapeConfig{}) {
		return nil
	}
	return result
}

func projectPlatformConfig(in, mask core.InstanceConfigurationLaunchInstancePlatformConfig) *comparablePlatformConfig {
	if in == nil || mask == nil {
		return nil
	}

	switch desired := mask.(type) {
	case core.AmdRomeBmGpuPlatformConfig:
		actual, ok := in.(core.AmdRomeBmGpuPlatformConfig)
		if !ok {
			return &comparablePlatformConfig{Type: fmt.Sprintf("%T", in)}
		}
		return &comparablePlatformConfig{
			Type:                                     "AmdRomeBmGpuPlatformConfig",
			IsSecureBootEnabled:                      pickDefaultFalseBool(actual.IsSecureBootEnabled, desired.IsSecureBootEnabled),
			IsTrustedPlatformModuleEnabled:           pickDefaultFalseBool(actual.IsTrustedPlatformModuleEnabled, desired.IsTrustedPlatformModuleEnabled),
			IsMeasuredBootEnabled:                    pickDefaultFalseBool(actual.IsMeasuredBootEnabled, desired.IsMeasuredBootEnabled),
			IsMemoryEncryptionEnabled:                pickDefaultFalseBool(actual.IsMemoryEncryptionEnabled, desired.IsMemoryEncryptionEnabled),
			IsSymmetricMultiThreadingEnabled:         pickDefaultFalseBool(actual.IsSymmetricMultiThreadingEnabled, desired.IsSymmetricMultiThreadingEnabled),
			IsAccessControlServiceEnabled:            pickDefaultFalseBool(actual.IsAccessControlServiceEnabled, desired.IsAccessControlServiceEnabled),
			AreVirtualInstructionsEnabled:            pickDefaultFalseBool(actual.AreVirtualInstructionsEnabled, desired.AreVirtualInstructionsEnabled),
			IsInputOutputMemoryManagementUnitEnabled: pickDefaultFalseBool(actual.IsInputOutputMemoryManagementUnitEnabled, desired.IsInputOutputMemoryManagementUnitEnabled),
			NumaNodesPerSocket:                       pickEnum(string(actual.NumaNodesPerSocket), string(desired.NumaNodesPerSocket)),
		}
	case core.AmdRomeBmPlatformConfig:
		actual, ok := in.(core.AmdRomeBmPlatformConfig)
		if !ok {
			return &comparablePlatformConfig{Type: fmt.Sprintf("%T", in)}
		}
		return &comparablePlatformConfig{
			Type:                                     "AmdRomeBmPlatformConfig",
			IsSecureBootEnabled:                      pickDefaultFalseBool(actual.IsSecureBootEnabled, desired.IsSecureBootEnabled),
			IsTrustedPlatformModuleEnabled:           pickDefaultFalseBool(actual.IsTrustedPlatformModuleEnabled, desired.IsTrustedPlatformModuleEnabled),
			IsMeasuredBootEnabled:                    pickDefaultFalseBool(actual.IsMeasuredBootEnabled, desired.IsMeasuredBootEnabled),
			IsMemoryEncryptionEnabled:                pickDefaultFalseBool(actual.IsMemoryEncryptionEnabled, desired.IsMemoryEncryptionEnabled),
			IsSymmetricMultiThreadingEnabled:         pickDefaultFalseBool(actual.IsSymmetricMultiThreadingEnabled, desired.IsSymmetricMultiThreadingEnabled),
			IsAccessControlServiceEnabled:            pickDefaultFalseBool(actual.IsAccessControlServiceEnabled, desired.IsAccessControlServiceEnabled),
			AreVirtualInstructionsEnabled:            pickDefaultFalseBool(actual.AreVirtualInstructionsEnabled, desired.AreVirtualInstructionsEnabled),
			IsInputOutputMemoryManagementUnitEnabled: pickDefaultFalseBool(actual.IsInputOutputMemoryManagementUnitEnabled, desired.IsInputOutputMemoryManagementUnitEnabled),
			PercentageOfCoresEnabled:                 pickInt(actual.PercentageOfCoresEnabled, desired.PercentageOfCoresEnabled),
			NumaNodesPerSocket:                       pickEnum(string(actual.NumaNodesPerSocket), string(desired.NumaNodesPerSocket)),
		}
	case core.IntelIcelakeBmPlatformConfig:
		actual, ok := in.(core.IntelIcelakeBmPlatformConfig)
		if !ok {
			return &comparablePlatformConfig{Type: fmt.Sprintf("%T", in)}
		}
		return &comparablePlatformConfig{
			Type:                                     "IntelIcelakeBmPlatformConfig",
			IsSecureBootEnabled:                      pickDefaultFalseBool(actual.IsSecureBootEnabled, desired.IsSecureBootEnabled),
			IsTrustedPlatformModuleEnabled:           pickDefaultFalseBool(actual.IsTrustedPlatformModuleEnabled, desired.IsTrustedPlatformModuleEnabled),
			IsMeasuredBootEnabled:                    pickDefaultFalseBool(actual.IsMeasuredBootEnabled, desired.IsMeasuredBootEnabled),
			IsMemoryEncryptionEnabled:                pickDefaultFalseBool(actual.IsMemoryEncryptionEnabled, desired.IsMemoryEncryptionEnabled),
			IsSymmetricMultiThreadingEnabled:         pickDefaultFalseBool(actual.IsSymmetricMultiThreadingEnabled, desired.IsSymmetricMultiThreadingEnabled),
			IsInputOutputMemoryManagementUnitEnabled: pickDefaultFalseBool(actual.IsInputOutputMemoryManagementUnitEnabled, desired.IsInputOutputMemoryManagementUnitEnabled),
			PercentageOfCoresEnabled:                 pickInt(actual.PercentageOfCoresEnabled, desired.PercentageOfCoresEnabled),
			NumaNodesPerSocket:                       pickEnum(string(actual.NumaNodesPerSocket), string(desired.NumaNodesPerSocket)),
		}
	case core.AmdVmPlatformConfig:
		actual, ok := in.(core.AmdVmPlatformConfig)
		if !ok {
			return &comparablePlatformConfig{Type: fmt.Sprintf("%T", in)}
		}
		return &comparablePlatformConfig{
			Type:                           "AmdVmPlatformConfig",
			IsSecureBootEnabled:            pickDefaultFalseBool(actual.IsSecureBootEnabled, desired.IsSecureBootEnabled),
			IsTrustedPlatformModuleEnabled: pickDefaultFalseBool(actual.IsTrustedPlatformModuleEnabled, desired.IsTrustedPlatformModuleEnabled),
			IsMeasuredBootEnabled:          pickDefaultFalseBool(actual.IsMeasuredBootEnabled, desired.IsMeasuredBootEnabled),
			IsMemoryEncryptionEnabled:      pickDefaultFalseBool(actual.IsMemoryEncryptionEnabled, desired.IsMemoryEncryptionEnabled),
		}
	case core.IntelVmPlatformConfig:
		actual, ok := in.(core.IntelVmPlatformConfig)
		if !ok {
			return &comparablePlatformConfig{Type: fmt.Sprintf("%T", in)}
		}
		return &comparablePlatformConfig{
			Type:                           "IntelVmPlatformConfig",
			IsSecureBootEnabled:            pickDefaultFalseBool(actual.IsSecureBootEnabled, desired.IsSecureBootEnabled),
			IsTrustedPlatformModuleEnabled: pickDefaultFalseBool(actual.IsTrustedPlatformModuleEnabled, desired.IsTrustedPlatformModuleEnabled),
			IsMeasuredBootEnabled:          pickDefaultFalseBool(actual.IsMeasuredBootEnabled, desired.IsMeasuredBootEnabled),
			IsMemoryEncryptionEnabled:      pickDefaultFalseBool(actual.IsMemoryEncryptionEnabled, desired.IsMemoryEncryptionEnabled),
		}
	case core.IntelSkylakeBmPlatformConfig:
		actual, ok := in.(core.IntelSkylakeBmPlatformConfig)
		if !ok {
			return &comparablePlatformConfig{Type: fmt.Sprintf("%T", in)}
		}
		return &comparablePlatformConfig{
			Type:                           "IntelSkylakeBmPlatformConfig",
			IsSecureBootEnabled:            pickDefaultFalseBool(actual.IsSecureBootEnabled, desired.IsSecureBootEnabled),
			IsTrustedPlatformModuleEnabled: pickDefaultFalseBool(actual.IsTrustedPlatformModuleEnabled, desired.IsTrustedPlatformModuleEnabled),
			IsMeasuredBootEnabled:          pickDefaultFalseBool(actual.IsMeasuredBootEnabled, desired.IsMeasuredBootEnabled),
			IsMemoryEncryptionEnabled:      pickDefaultFalseBool(actual.IsMemoryEncryptionEnabled, desired.IsMemoryEncryptionEnabled),
		}
	case core.AmdMilanBmPlatformConfig:
		actual, ok := in.(core.AmdMilanBmPlatformConfig)
		if !ok {
			return &comparablePlatformConfig{Type: fmt.Sprintf("%T", in)}
		}
		return &comparablePlatformConfig{
			Type:                                     "AmdMilanBmPlatformConfig",
			IsSecureBootEnabled:                      pickDefaultFalseBool(actual.IsSecureBootEnabled, desired.IsSecureBootEnabled),
			IsTrustedPlatformModuleEnabled:           pickDefaultFalseBool(actual.IsTrustedPlatformModuleEnabled, desired.IsTrustedPlatformModuleEnabled),
			IsMeasuredBootEnabled:                    pickDefaultFalseBool(actual.IsMeasuredBootEnabled, desired.IsMeasuredBootEnabled),
			IsMemoryEncryptionEnabled:                pickDefaultFalseBool(actual.IsMemoryEncryptionEnabled, desired.IsMemoryEncryptionEnabled),
			IsSymmetricMultiThreadingEnabled:         pickDefaultFalseBool(actual.IsSymmetricMultiThreadingEnabled, desired.IsSymmetricMultiThreadingEnabled),
			IsAccessControlServiceEnabled:            pickDefaultFalseBool(actual.IsAccessControlServiceEnabled, desired.IsAccessControlServiceEnabled),
			AreVirtualInstructionsEnabled:            pickDefaultFalseBool(actual.AreVirtualInstructionsEnabled, desired.AreVirtualInstructionsEnabled),
			IsInputOutputMemoryManagementUnitEnabled: pickDefaultFalseBool(actual.IsInputOutputMemoryManagementUnitEnabled, desired.IsInputOutputMemoryManagementUnitEnabled),
			PercentageOfCoresEnabled:                 pickInt(actual.PercentageOfCoresEnabled, desired.PercentageOfCoresEnabled),
			NumaNodesPerSocket:                       pickEnum(string(actual.NumaNodesPerSocket), string(desired.NumaNodesPerSocket)),
		}
	default:
		return nil
	}
}

func projectSourceDetails(in, mask core.InstanceConfigurationInstanceSourceDetails) *comparableSourceDetails {
	if in == nil || mask == nil {
		return nil
	}
	desired, ok := mask.(core.InstanceConfigurationInstanceSourceViaImageDetails)
	if !ok {
		return nil
	}
	actual, ok := in.(core.InstanceConfigurationInstanceSourceViaImageDetails)
	if !ok {
		return &comparableSourceDetails{SourceType: fmt.Sprintf("%T", in)}
	}
	result := &comparableSourceDetails{
		SourceType:          "image",
		ImageID:             pickString(actual.ImageId, desired.ImageId),
		KMSKeyID:            pickString(actual.KmsKeyId, desired.KmsKeyId),
		BootVolumeSizeInGBs: pickInt64(actual.BootVolumeSizeInGBs, desired.BootVolumeSizeInGBs),
		BootVolumeVPUsPerGB: pickInt64(actual.BootVolumeVpusPerGB, desired.BootVolumeVpusPerGB),
	}
	if reflect.DeepEqual(result, &comparableSourceDetails{}) {
		return nil
	}
	return result
}

func projectLaunchOptions(in, mask *core.InstanceConfigurationLaunchOptions) *comparableLaunchOptions {
	if in == nil || mask == nil {
		return nil
	}
	result := &comparableLaunchOptions{
		BootVolumeType:                  pickEnum(string(in.BootVolumeType), string(mask.BootVolumeType)),
		Firmware:                        pickEnum(string(in.Firmware), string(mask.Firmware)),
		NetworkType:                     pickEnum(string(in.NetworkType), string(mask.NetworkType)),
		RemoteDataVolumeType:            pickEnum(string(in.RemoteDataVolumeType), string(mask.RemoteDataVolumeType)),
		IsConsistentVolumeNamingEnabled: pickDefaultFalseBool(in.IsConsistentVolumeNamingEnabled, mask.IsConsistentVolumeNamingEnabled),
	}
	if reflect.DeepEqual(result, &comparableLaunchOptions{}) {
		return nil
	}
	return result
}

func projectAgentConfig(in, mask *core.InstanceConfigurationLaunchInstanceAgentConfigDetails) *comparableAgentConfig {
	if in == nil || mask == nil {
		return nil
	}
	var plugins []comparablePluginConfig
	if len(mask.PluginsConfig) > 0 {
		plugins = make([]comparablePluginConfig, 0, len(in.PluginsConfig))
		for _, plugin := range in.PluginsConfig {
			plugins = append(plugins, comparablePluginConfig{
				Name:         plugin.Name,
				DesiredState: string(plugin.DesiredState),
			})
		}
		sort.Slice(plugins, func(i, j int) bool {
			leftName := ""
			rightName := ""
			if plugins[i].Name != nil {
				leftName = *plugins[i].Name
			}
			if plugins[j].Name != nil {
				rightName = *plugins[j].Name
			}
			if leftName != rightName {
				return leftName < rightName
			}
			return plugins[i].DesiredState < plugins[j].DesiredState
		})
		if len(plugins) == 0 {
			plugins = nil
		}
	}
	result := &comparableAgentConfig{
		IsMonitoringDisabled:  pickDefaultFalseBool(in.IsMonitoringDisabled, mask.IsMonitoringDisabled),
		IsManagementDisabled:  pickDefaultFalseBool(in.IsManagementDisabled, mask.IsManagementDisabled),
		AreAllPluginsDisabled: pickDefaultFalseBool(in.AreAllPluginsDisabled, mask.AreAllPluginsDisabled),
		PluginsConfig:         plugins,
	}
	if reflect.DeepEqual(result, &comparableAgentConfig{}) {
		return nil
	}
	return result
}

func projectInstanceOptions(in, mask *core.InstanceConfigurationInstanceOptions) *comparableInstanceOptions {
	if in == nil || mask == nil {
		return nil
	}
	result := &comparableInstanceOptions{
		AreLegacyIMDSEndpointsDisabled: pickDefaultFalseBool(in.AreLegacyImdsEndpointsDisabled, mask.AreLegacyImdsEndpointsDisabled),
	}
	if reflect.DeepEqual(result, &comparableInstanceOptions{}) {
		return nil
	}
	return result
}

func projectAvailabilityConfig(in, mask *core.InstanceConfigurationAvailabilityConfig) *comparableAvailabilityConfig {
	if in == nil || mask == nil {
		return nil
	}
	result := &comparableAvailabilityConfig{
		IsLiveMigrationPreferred: pickBool(in.IsLiveMigrationPreferred, mask.IsLiveMigrationPreferred),
		RecoveryAction:           pickEnum(string(in.RecoveryAction), string(mask.RecoveryAction)),
	}
	if reflect.DeepEqual(result, &comparableAvailabilityConfig{}) {
		return nil
	}
	return result
}

func projectPreemptibleInstanceConfig(in, mask *core.PreemptibleInstanceConfigDetails) *comparablePreemptibleInstanceConfig {
	if in == nil || mask == nil {
		return nil
	}
	desiredAction, desiredOK := mask.PreemptionAction.(core.TerminatePreemptionAction)
	actualAction, actualOK := in.PreemptionAction.(core.TerminatePreemptionAction)
	if !desiredOK || !actualOK {
		return nil
	}
	result := &comparablePreemptibleInstanceConfig{
		PreserveBootVolume: pickDefaultFalseBool(actualAction.PreserveBootVolume, desiredAction.PreserveBootVolume),
	}
	if reflect.DeepEqual(result, &comparablePreemptibleInstanceConfig{}) {
		return nil
	}
	return result
}

func pickMetadata(actual, mask map[string]string) map[string]string {
	if len(mask) == 0 {
		return nil
	}
	result := make(map[string]string, len(mask))
	for key := range mask {
		if value, ok := actual[key]; ok {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func pickStrings(actual, mask []string) []string {
	if len(mask) == 0 {
		return nil
	}
	if len(actual) == 0 {
		return nil
	}
	result := append([]string(nil), actual...)
	return result
}

func pickString(actual, mask *string) *string {
	if mask == nil {
		return nil
	}
	return actual
}

func pickBool(actual, mask *bool) *bool {
	if mask == nil {
		return nil
	}
	return actual
}

func pickDefaultFalseBool(actual, mask *bool) *bool {
	if mask == nil {
		return nil
	}
	if actual == nil || !*actual {
		return nil
	}
	return actual
}

func pickFloat32(actual, mask *float32) *float32 {
	if mask == nil {
		return nil
	}
	return actual
}

func pickInt(actual, mask *int) *int {
	if mask == nil {
		return nil
	}
	return actual
}

func pickInt64(actual, mask *int64) *int64 {
	if mask == nil {
		return nil
	}
	return actual
}

func pickEnum(actual, mask string) string {
	if mask == "" {
		return ""
	}
	return actual
}
