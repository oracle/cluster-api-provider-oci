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
	"testing"

	. "github.com/onsi/gomega"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

func TestComputeHash_NilInput(t *testing.T) {
	g := NewWithT(t)
	hash, err := ComputeHash(nil)
	g.Expect(err).To(BeNil())
	g.Expect(hash).ToNot(BeEmpty())
}

func TestComputeHash_BasicLaunchDetails(t *testing.T) {
	g := NewWithT(t)
	ld := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
	}

	hash, err := ComputeHash(ld)
	g.Expect(err).To(BeNil())
	g.Expect(hash).ToNot(BeEmpty())
	g.Expect(hash).To(HaveLen(64)) // SHA-256 hex is 64 characters
}

func TestComputeHash_ConsistentResults(t *testing.T) {
	g := NewWithT(t)
	ld := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	hash1, err := ComputeHash(ld)
	g.Expect(err).To(BeNil())

	hash2, err := ComputeHash(ld)
	g.Expect(err).To(BeNil())

	g.Expect(hash1).To(Equal(hash2))
}

func TestNormalizeLaunchDetails_NilInput(t *testing.T) {
	g := NewWithT(t)
	result := normalizeLaunchDetails(nil)
	g.Expect(result).To(BeNil())
}

func TestNormalizeLaunchDetails_StripsIgnoredFields(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		DisplayName:        common.String("test-instance"),
		Shape:              common.String("VM.Standard2.1"),
		FreeformTags:       map[string]string{"tag1": "value1"},
		DefinedTags:        map[string]map[string]interface{}{"namespace": {"key": "value"}},
		SecurityAttributes: map[string]map[string]interface{}{"security": {"attr": "val"}},
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			DisplayName:        common.String("vnic"),
			FreeformTags:       map[string]string{"vnic-tag": "vnic-value"},
			DefinedTags:        map[string]map[string]interface{}{"vnic-ns": {"vnic-key": "vnic-value"}},
			SecurityAttributes: map[string]map[string]interface{}{"vnic-sec": {"vnic-attr": "vnic-val"}},
			NsgIds:             []string{"nsg1", "nsg2"},
		},
	}

	normalized := normalizeLaunchDetails(original)

	// Fields that should be stripped
	g.Expect(normalized.DisplayName).To(BeNil())
	g.Expect(normalized.FreeformTags).To(BeNil())
	g.Expect(normalized.DefinedTags).To(BeNil())
	g.Expect(normalized.SecurityAttributes).To(BeNil())

	// Fields that should be preserved
	g.Expect(*normalized.Shape).To(Equal("VM.Standard2.1"))

	// VNIC details should be stripped of ignored fields but NSGs preserved and sorted
	g.Expect(normalized.CreateVnicDetails.DisplayName).To(BeNil())
	g.Expect(normalized.CreateVnicDetails.FreeformTags).To(BeNil())
	g.Expect(normalized.CreateVnicDetails.DefinedTags).To(BeNil())
	g.Expect(normalized.CreateVnicDetails.SecurityAttributes).To(BeNil())
	g.Expect(normalized.CreateVnicDetails.NsgIds).To(Equal([]string{"nsg1", "nsg2"}))
}

func TestNormalizeLaunchDetails_SortsNSGIds(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			NsgIds: []string{"nsg3", "nsg1", "nsg2"},
		},
	}

	normalized := normalizeLaunchDetails(original)

	expected := []string{"nsg1", "nsg2", "nsg3"}
	g.Expect(normalized.CreateVnicDetails.NsgIds).To(Equal(expected))
}

func TestNormalizeLaunchDetails_EmptyNSGIds(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			NsgIds: []string{},
		},
	}

	normalized := normalizeLaunchDetails(original)

	g.Expect(normalized.CreateVnicDetails.NsgIds).To(BeNil())
}

func TestNormalizeLaunchDetails_EmptyShapeConfig(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{},
	}

	normalized := normalizeLaunchDetails(original)

	g.Expect(normalized.ShapeConfig).To(BeNil())
}

func TestNormalizeLaunchDetails_NonEmptyShapeConfig(t *testing.T) {
	g := NewWithT(t)
	var ocpus float32 = 2.0
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
			Ocpus: &ocpus,
		},
	}

	normalized := normalizeLaunchDetails(original)

	g.Expect(normalized.ShapeConfig).ToNot(BeNil())
	g.Expect(*normalized.ShapeConfig.Ocpus).To(Equal(ocpus))
}

func TestNormalizeLaunchDetails_EmptyExtendedMetadata(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		ExtendedMetadata: map[string]interface{}{},
	}

	normalized := normalizeLaunchDetails(original)

	g.Expect(normalized.ExtendedMetadata).To(BeNil())
}

func TestNormalizeMetadata_NilInput(t *testing.T) {
	g := NewWithT(t)
	result := normalizeMetadata(nil)
	g.Expect(result).To(BeNil())
}

func TestNormalizeMetadata_ExcludesUserData(t *testing.T) {
	g := NewWithT(t)
	original := map[string]string{
		"user_data":           "sensitive-data",
		"ssh_authorized_keys": "ssh-key",
		"other_key":           "other_value",
	}

	normalized := normalizeMetadata(original)

	expected := map[string]string{
		"ssh_authorized_keys": "ssh-key",
		"other_key":           "other_value",
	}
	g.Expect(normalized).To(Equal(expected))
}

func TestNormalizeMetadata_AllUserData(t *testing.T) {
	g := NewWithT(t)
	original := map[string]string{
		"user_data": "sensitive-data",
	}

	normalized := normalizeMetadata(original)

	g.Expect(normalized).To(BeNil())
}

func TestNormalizeMetadata_NoUserData(t *testing.T) {
	g := NewWithT(t)
	original := map[string]string{
		"ssh_authorized_keys": "ssh-key",
		"other_key":           "other_value",
	}

	normalized := normalizeMetadata(original)

	g.Expect(normalized).To(Equal(original))
}

func TestHashChanged_SameHashes(t *testing.T) {
	g := NewWithT(t)
	g.Expect(hashChanged("hash123", "hash123")).To(BeFalse())
}

func TestHashChanged_DifferentHashes(t *testing.T) {
	g := NewWithT(t)
	g.Expect(hashChanged("hash123", "hash456")).To(BeTrue())
}

func TestLaunchDetailsEqual_NilInputs(t *testing.T) {
	g := NewWithT(t)
	g.Expect(launchDetailsEqual(nil, nil)).To(BeTrue())
}

func TestLaunchDetailsEqual_OneNil(t *testing.T) {
	g := NewWithT(t)
	ld := &core.InstanceConfigurationLaunchInstanceDetails{Shape: common.String("VM.Standard2.1")}
	g.Expect(launchDetailsEqual(ld, nil)).To(BeFalse())
	g.Expect(launchDetailsEqual(nil, ld)).To(BeFalse())
}

func TestLaunchDetailsEqual_EquivalentAfterNormalization(t *testing.T) {
	g := NewWithT(t)
	ld1 := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape:        common.String("VM.Standard2.1"),
		DisplayName:  common.String("instance1"),
		FreeformTags: map[string]string{"tag": "value"},
		Metadata:     map[string]string{"user_data": "data", "key": "value"},
	}

	ld2 := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape:        common.String("VM.Standard2.1"),
		DisplayName:  common.String("instance2"),                                   // different but ignored
		FreeformTags: map[string]string{"other": "tag"},                            // different but ignored
		Metadata:     map[string]string{"user_data": "other-data", "key": "value"}, // user_data ignored
	}

	g.Expect(launchDetailsEqual(ld1, ld2)).To(BeTrue())
}

func TestLaunchDetailsEqual_DifferentAfterNormalization(t *testing.T) {
	g := NewWithT(t)
	ld1 := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape:    common.String("VM.Standard2.1"),
		Metadata: map[string]string{"key": "value1"},
	}

	ld2 := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape:    common.String("VM.Standard2.1"),
		Metadata: map[string]string{"key": "value2"}, // different value
	}

	g.Expect(launchDetailsEqual(ld1, ld2)).To(BeFalse())
}

func TestComputeHash_DifferentInputsProduceDifferentHashes(t *testing.T) {
	g := NewWithT(t)
	ld1 := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
	}

	ld2 := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.2"),
	}

	hash1, err := ComputeHash(ld1)
	g.Expect(err).To(BeNil())

	hash2, err := ComputeHash(ld2)
	g.Expect(err).To(BeNil())

	g.Expect(hash1).ToNot(Equal(hash2))
}

func TestComputeHash_IgnoresDisplayName(t *testing.T) {
	g := NewWithT(t)
	baseLd := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
	}

	ld1 := *baseLd
	ld1.DisplayName = common.String("instance1")

	ld2 := *baseLd
	ld2.DisplayName = common.String("instance2")

	hash1, err := ComputeHash(&ld1)
	g.Expect(err).To(BeNil())

	hash2, err := ComputeHash(&ld2)
	g.Expect(err).To(BeNil())

	g.Expect(hash1).To(Equal(hash2))
}

func TestComputeHash_IgnoresFreeformTags(t *testing.T) {
	g := NewWithT(t)
	baseLd := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
	}

	ld1 := *baseLd
	ld1.FreeformTags = map[string]string{"tag1": "value1"}

	ld2 := *baseLd
	ld2.FreeformTags = map[string]string{"tag2": "value2"}

	hash1, err := ComputeHash(&ld1)
	g.Expect(err).To(BeNil())

	hash2, err := ComputeHash(&ld2)
	g.Expect(err).To(BeNil())

	g.Expect(hash1).To(Equal(hash2))
}

func TestComputeHash_IgnoresUserData(t *testing.T) {
	g := NewWithT(t)
	baseLd := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
	}

	ld1 := *baseLd
	ld1.Metadata = map[string]string{"user_data": "data1", "key": "value"}

	ld2 := *baseLd
	ld2.Metadata = map[string]string{"user_data": "data2", "key": "value"}

	hash1, err := ComputeHash(&ld1)
	g.Expect(err).To(BeNil())

	hash2, err := ComputeHash(&ld2)
	g.Expect(err).To(BeNil())

	g.Expect(hash1).To(Equal(hash2))
}

func TestComputeHash_SortsNSGIds(t *testing.T) {
	g := NewWithT(t)
	ld1 := &core.InstanceConfigurationLaunchInstanceDetails{
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			NsgIds: []string{"nsg2", "nsg1"},
		},
	}

	ld2 := &core.InstanceConfigurationLaunchInstanceDetails{
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			NsgIds: []string{"nsg1", "nsg2"},
		},
	}

	hash1, err := ComputeHash(ld1)
	g.Expect(err).To(BeNil())

	hash2, err := ComputeHash(ld2)
	g.Expect(err).To(BeNil())

	g.Expect(hash1).To(Equal(hash2))
}

func TestComputeHash_ComprehensiveTest(t *testing.T) {
	g := NewWithT(t)

	// Create a fully-populated InstanceConfigurationLaunchInstanceDetails
	var ocpus float32 = 2.0
	var memory float32 = 16.0
	var vcpus int = 4
	var nvmes int = 1

	ld := &core.InstanceConfigurationLaunchInstanceDetails{
		// Fields that should be EXCLUDED
		DisplayName:        common.String("test-instance"),
		FreeformTags:       map[string]string{"env": "test", "team": "platform"},
		DefinedTags:        map[string]map[string]interface{}{"oracle-tags": {"CreatedBy": "test"}},
		SecurityAttributes: map[string]map[string]interface{}{"security": {"level": "high"}},

		// Fields that should be included
		Shape:             common.String("VM.Standard.E4.Flex"),
		CompartmentId:     common.String("ocid1.compartment.oc1..test"),
		DedicatedVmHostId: common.String("ocid1.dedicatedvmhost.oc1..test"),

		// Metadata - user_data should be excluded
		Metadata: map[string]string{
			"user_data":           "sensitive-bootstrap-script",
			"ssh_authorized_keys": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
			"custom_key":          "custom_value",
		},

		// Shape config - should be included
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
			Ocpus:       &ocpus,
			MemoryInGBs: &memory,
			Vcpus:       &vcpus,
			Nvmes:       &nvmes,
		},

		// VNIC details - display name/tags/security should be EXCLUDED, NSGs sorted
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			DisplayName:            common.String("test-vnic"),
			FreeformTags:           map[string]string{"vnic-tag": "value"},
			DefinedTags:            map[string]map[string]interface{}{"vnic-ns": {"key": "val"}},
			SecurityAttributes:     map[string]map[string]interface{}{"vnic-sec": {"attr": "val"}},
			AssignPublicIp:         common.Bool(false),
			SkipSourceDestCheck:    common.Bool(false),
			AssignPrivateDnsRecord: common.Bool(true),
			HostnameLabel:          common.String("test-host"),
			NsgIds:                 []string{"ocid1.nsg.oc1..nsg3", "ocid1.nsg.oc1..nsg1", "ocid1.nsg.oc1..nsg2"},
		},

		// Platform config - should be included
		PlatformConfig: core.AmdVmPlatformConfig{
			IsSecureBootEnabled:            common.Bool(true),
			IsTrustedPlatformModuleEnabled: common.Bool(false),
			IsMeasuredBootEnabled:          common.Bool(true),
			IsMemoryEncryptionEnabled:      common.Bool(false),
		},

		// Agent config - should be included
		AgentConfig: &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{
			IsMonitoringDisabled:  common.Bool(false),
			IsManagementDisabled:  common.Bool(false),
			AreAllPluginsDisabled: common.Bool(false),
			PluginsConfig: []core.InstanceAgentPluginConfigDetails{
				{
					Name:         common.String("test-plugin"),
					DesiredState: "ENABLED",
				},
			},
		},

		// Launch options - should be included
		LaunchOptions: &core.InstanceConfigurationLaunchOptions{
			BootVolumeType:                  "ISCSI",
			Firmware:                        "UEFI_64",
			NetworkType:                     "VFIO",
			RemoteDataVolumeType:            "ISCSI",
			IsConsistentVolumeNamingEnabled: common.Bool(true),
		},

		// Instance options - should be included
		InstanceOptions: &core.InstanceConfigurationInstanceOptions{
			AreLegacyImdsEndpointsDisabled: common.Bool(true),
		},

		// Availability config - should be included
		AvailabilityConfig: &core.InstanceConfigurationAvailabilityConfig{
			RecoveryAction: "RESTORE_INSTANCE",
		},

		// Preemptible instance config - should be included
		PreemptibleInstanceConfig: &core.PreemptibleInstanceConfigDetails{
			PreemptionAction: core.TerminatePreemptionAction{
				PreserveBootVolume: common.Bool(false),
			},
		},

		// Source details - should be included
		SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{
			ImageId:             common.String("ocid1.image.oc1..test"),
			BootVolumeVpusPerGB: common.Int64(120),
			BootVolumeSizeInGBs: common.Int64(50),
		},

		// Extended metadata - should be included (non-empty)
		ExtendedMetadata: map[string]interface{}{
			"custom_metadata": map[string]interface{}{
				"nested": "value",
			},
		},
	}

	// Compute hash
	hash, err := ComputeHash(ld)
	g.Expect(err).To(BeNil())
	g.Expect(hash).ToNot(BeEmpty())
	g.Expect(hash).To(HaveLen(64))

	// Verify the normalized result contains only the expected fields
	normalized := normalizeLaunchDetails(ld)

	// Fields that should be nil
	g.Expect(normalized.DisplayName).To(BeNil())
	g.Expect(normalized.FreeformTags).To(BeNil())
	g.Expect(normalized.DefinedTags).To(BeNil())
	g.Expect(normalized.SecurityAttributes).To(BeNil())

	// Fields that should be preserved
	g.Expect(*normalized.Shape).To(Equal("VM.Standard.E4.Flex"))
	g.Expect(*normalized.CompartmentId).To(Equal("ocid1.compartment.oc1..test"))
	g.Expect(*normalized.DedicatedVmHostId).To(Equal("ocid1.dedicatedvmhost.oc1..test"))

	// Metadata should exclude user_data but keep other keys
	expectedMetadata := map[string]string{
		"ssh_authorized_keys": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
		"custom_key":          "custom_value",
	}
	g.Expect(normalized.Metadata).To(Equal(expectedMetadata))

	// Shape config should be preserved
	g.Expect(normalized.ShapeConfig).ToNot(BeNil())
	g.Expect(*normalized.ShapeConfig.Ocpus).To(Equal(ocpus))
	g.Expect(*normalized.ShapeConfig.MemoryInGBs).To(Equal(memory))
	g.Expect(*normalized.ShapeConfig.Vcpus).To(Equal(vcpus))
	g.Expect(*normalized.ShapeConfig.Nvmes).To(Equal(nvmes))

	// VNIC details should have some excluded fields but NSGs sorted
	g.Expect(normalized.CreateVnicDetails).ToNot(BeNil())
	g.Expect(normalized.CreateVnicDetails.DisplayName).To(BeNil())
	g.Expect(normalized.CreateVnicDetails.FreeformTags).To(BeNil())
	g.Expect(normalized.CreateVnicDetails.DefinedTags).To(BeNil())
	g.Expect(normalized.CreateVnicDetails.SecurityAttributes).To(BeNil())
	g.Expect(*normalized.CreateVnicDetails.AssignPublicIp).To(BeFalse())
	g.Expect(*normalized.CreateVnicDetails.SkipSourceDestCheck).To(BeFalse())
	g.Expect(*normalized.CreateVnicDetails.AssignPrivateDnsRecord).To(BeTrue())
	g.Expect(*normalized.CreateVnicDetails.HostnameLabel).To(Equal("test-host"))
	// NSGs should be sorted
	expectedNSGs := []string{"ocid1.nsg.oc1..nsg1", "ocid1.nsg.oc1..nsg2", "ocid1.nsg.oc1..nsg3"}
	g.Expect(normalized.CreateVnicDetails.NsgIds).To(Equal(expectedNSGs))

	// Other configs should be preserved
	g.Expect(normalized.PlatformConfig).ToNot(BeNil())
	g.Expect(normalized.AgentConfig).ToNot(BeNil())
	g.Expect(normalized.LaunchOptions).ToNot(BeNil())
	g.Expect(normalized.InstanceOptions).ToNot(BeNil())
	g.Expect(normalized.AvailabilityConfig).ToNot(BeNil())
	g.Expect(normalized.PreemptibleInstanceConfig).ToNot(BeNil())
	g.Expect(normalized.ExtendedMetadata).ToNot(BeNil())
	g.Expect(normalized.SourceDetails).ToNot(BeNil())
}
