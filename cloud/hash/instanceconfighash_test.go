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
	"encoding/base64"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

func encodeUserData(t *testing.T, value string) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte(value))
}

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
	result := projectLaunchDetails(nil, nil)
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

	normalized := projectLaunchDetails(original, original)

	// Only supported comparison fields should remain.
	g.Expect(*normalized.Shape).To(Equal("VM.Standard2.1"))
	g.Expect(normalized.CreateVnicDetails.NSGIDs).To(Equal([]string{"nsg1", "nsg2"}))
}

func TestNormalizeLaunchDetails_SortsNSGIds(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			NsgIds: []string{"nsg3", "nsg1", "nsg2"},
		},
	}

	normalized := projectLaunchDetails(original, original)

	expected := []string{"nsg1", "nsg2", "nsg3"}
	g.Expect(normalized.CreateVnicDetails.NSGIDs).To(Equal(expected))
}

func TestNormalizeLaunchDetails_EmptyNSGIds(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			NsgIds: []string{},
		},
	}

	normalized := projectLaunchDetails(original, original)

	g.Expect(normalized.CreateVnicDetails).To(BeNil())
}

func TestNormalizeLaunchDetails_EmptyShapeConfig(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{},
	}

	normalized := projectLaunchDetails(original, original)

	g.Expect(normalized.ShapeConfig).To(BeNil())
}

func TestNormalizeLaunchDetails_SortsAgentPlugins(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		AgentConfig: &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{
			PluginsConfig: []core.InstanceAgentPluginConfigDetails{
				{
					Name:         common.String("plugin-c"),
					DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateDisabled,
				},
				{
					Name:         common.String("plugin-a"),
					DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
				},
				{
					Name:         common.String("plugin-b"),
					DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
				},
			},
		},
	}

	normalized := projectLaunchDetails(original, original)

	g.Expect(normalized.AgentConfig).ToNot(BeNil())
	g.Expect(normalized.AgentConfig.PluginsConfig).To(HaveLen(3))
	g.Expect(*normalized.AgentConfig.PluginsConfig[0].Name).To(Equal("plugin-a"))
	g.Expect(*normalized.AgentConfig.PluginsConfig[1].Name).To(Equal("plugin-b"))
	g.Expect(*normalized.AgentConfig.PluginsConfig[2].Name).To(Equal("plugin-c"))
}

func TestNormalizeLaunchDetails_NonEmptyShapeConfig(t *testing.T) {
	g := NewWithT(t)
	var ocpus float32 = 2.0
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
			Ocpus: &ocpus,
		},
	}

	normalized := projectLaunchDetails(original, original)

	g.Expect(normalized.ShapeConfig).ToNot(BeNil())
	g.Expect(*normalized.ShapeConfig.OCPUs).To(Equal(ocpus))
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

func TestComputeUserDataHash_Present(t *testing.T) {
	g := NewWithT(t)
	md := map[string]string{"user_data": "dGVzdA==", "other": "value"}
	h := ComputeUserDataHash(md)
	g.Expect(h).To(HaveLen(64))
}

func TestComputeUserDataHash_Missing(t *testing.T) {
	g := NewWithT(t)
	md := map[string]string{"other": "value"}
	h := ComputeUserDataHash(md)
	g.Expect(h).To(BeEmpty())
}

func TestComputeUserDataHash_NilMetadata(t *testing.T) {
	g := NewWithT(t)
	h := ComputeUserDataHash(nil)
	g.Expect(h).To(BeEmpty())
}

func TestComputeUserDataHash_DifferentData(t *testing.T) {
	g := NewWithT(t)
	h1 := ComputeUserDataHash(map[string]string{"user_data": "data1"})
	h2 := ComputeUserDataHash(map[string]string{"user_data": "data2"})
	g.Expect(h1).ToNot(Equal(h2))
}

func TestComputeUserDataHash_SameData(t *testing.T) {
	g := NewWithT(t)
	h1 := ComputeUserDataHash(map[string]string{"user_data": "data1"})
	h2 := ComputeUserDataHash(map[string]string{"user_data": "data1"})
	g.Expect(h1).To(Equal(h2))
}

func TestComputeUserDataHash_EmptyString(t *testing.T) {
	g := NewWithT(t)
	h := ComputeUserDataHash(map[string]string{"user_data": ""})
	// An empty user_data key IS present, so a hash should be returned
	// (the SHA-256 of the empty string).
	g.Expect(h).To(HaveLen(64))
	g.Expect(h).ToNot(Equal(ComputeUserDataHash(map[string]string{"user_data": "non-empty"})))
}

func TestComputeUserDataHash_DetectsKubeadmBootstrapTokenRotation(t *testing.T) {
	g := NewWithT(t)
	first := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: abcdef.0123456789abcdef
    kind: JoinConfiguration
`
	second := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: zyxwvu.fedcba9876543210
    kind: JoinConfiguration
`

	h1 := ComputeUserDataHash(map[string]string{"user_data": encodeUserData(t, first)})
	h2 := ComputeUserDataHash(map[string]string{"user_data": encodeUserData(t, second)})
	g.Expect(h1).ToNot(Equal(h2))
}

func TestComputeUserDataHashIgnoringKubeadmToken_IgnoresKubeadmBootstrapTokenRotation(t *testing.T) {
	g := NewWithT(t)
	first := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: abcdef.0123456789abcdef
    kind: JoinConfiguration
`
	second := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: zyxwvu.fedcba9876543210
    kind: JoinConfiguration
`

	h1 := ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": encodeUserData(t, first)})
	h2 := ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": encodeUserData(t, second)})
	g.Expect(h1).To(Equal(h2))
}

func TestComputeUserDataHashIgnoringKubeadmToken_DetectsNonTokenKubeadmBootstrapChanges(t *testing.T) {
	g := NewWithT(t)
	first := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: abcdef.0123456789abcdef
    kind: JoinConfiguration
`
	second := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.2:6443
        token: abcdef.0123456789abcdef
    kind: JoinConfiguration
`

	h1 := ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": encodeUserData(t, first)})
	h2 := ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": encodeUserData(t, second)})
	g.Expect(h1).ToNot(Equal(h2))
}

func TestComputeUserDataHashIgnoringKubeadmToken_DetectsEndpointChangeEvenWhenTokenAlsoChanges(t *testing.T) {
	g := NewWithT(t)
	first := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: abcdef.0123456789abcdef
    kind: JoinConfiguration
`
	second := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.2:6443
        token: zyxwvu.fedcba9876543210
    kind: JoinConfiguration
`

	h1 := ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": encodeUserData(t, first)})
	h2 := ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": encodeUserData(t, second)})
	g.Expect(h1).ToNot(Equal(h2))
}

func TestComputeUserDataHashIgnoringKubeadmToken_DetectsNonKubeadmTokenChanges(t *testing.T) {
	g := NewWithT(t)
	first := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: abcdef.0123456789abcdef
    kind: JoinConfiguration
- path: /etc/custom/service.yaml
  content: |
    token: service-token-a
`
	second := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: abcdef.0123456789abcdef
    kind: JoinConfiguration
- path: /etc/custom/service.yaml
  content: |
    token: service-token-b
`

	h1 := ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": encodeUserData(t, first)})
	h2 := ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": encodeUserData(t, second)})
	g.Expect(h1).ToNot(Equal(h2))
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

	// Config hash should be the same — user_data is tracked separately
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

func TestComputeHash_SortsAgentPlugins(t *testing.T) {
	g := NewWithT(t)
	ld1 := &core.InstanceConfigurationLaunchInstanceDetails{
		AgentConfig: &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{
			PluginsConfig: []core.InstanceAgentPluginConfigDetails{
				{
					Name:         common.String("plugin-b"),
					DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateDisabled,
				},
				{
					Name:         common.String("plugin-a"),
					DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
				},
			},
		},
	}
	ld2 := &core.InstanceConfigurationLaunchInstanceDetails{
		AgentConfig: &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{
			PluginsConfig: []core.InstanceAgentPluginConfigDetails{
				{
					Name:         common.String("plugin-a"),
					DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
				},
				{
					Name:         common.String("plugin-b"),
					DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateDisabled,
				},
			},
		},
	}

	hash1, err := ComputeHash(ld1)
	g.Expect(err).To(BeNil())

	hash2, err := ComputeHash(ld2)
	g.Expect(err).To(BeNil())

	g.Expect(hash1).To(Equal(hash2))
}

func TestComputeComparableHash_IgnoresShapeDefaultsNotPresentInDesired(t *testing.T) {
	g := NewWithT(t)
	desired := &core.InstanceConfigurationLaunchInstanceDetails{
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
			Ocpus: common.Float32(1),
		},
	}
	actual := &core.InstanceConfigurationLaunchInstanceDetails{
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
			Ocpus:       common.Float32(1),
			MemoryInGBs: common.Float32(16),
		},
	}

	desiredHash, err := ComputeHash(desired)
	g.Expect(err).To(BeNil())

	actualHash, err := ComputeComparableHash(actual, desired)
	g.Expect(err).To(BeNil())

	g.Expect(actualHash).To(Equal(desiredHash))
}

func TestComputeComparableHash_IgnoresFalseDefaultsForSupportedFields(t *testing.T) {
	g := NewWithT(t)
	desired := &core.InstanceConfigurationLaunchInstanceDetails{
		AgentConfig: &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{
			IsMonitoringDisabled: common.Bool(false),
		},
		LaunchOptions: &core.InstanceConfigurationLaunchOptions{
			IsConsistentVolumeNamingEnabled: common.Bool(false),
		},
		InstanceOptions: &core.InstanceConfigurationInstanceOptions{
			AreLegacyImdsEndpointsDisabled: common.Bool(false),
		},
		IsPvEncryptionInTransitEnabled: common.Bool(false),
	}
	actual := &core.InstanceConfigurationLaunchInstanceDetails{
		AgentConfig:     &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{},
		LaunchOptions:   &core.InstanceConfigurationLaunchOptions{},
		InstanceOptions: &core.InstanceConfigurationInstanceOptions{},
	}

	desiredHash, err := ComputeHash(desired)
	g.Expect(err).To(BeNil())

	actualHash, err := ComputeComparableHash(actual, desired)
	g.Expect(err).To(BeNil())

	g.Expect(actualHash).To(Equal(desiredHash))
}

func TestComputeComparableHash_DetectsTrueToFalseVNICUpdates(t *testing.T) {
	g := NewWithT(t)
	desired := &core.InstanceConfigurationLaunchInstanceDetails{
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			AssignPublicIp: common.Bool(false),
			AssignIpv6Ip:   common.Bool(false),
		},
	}
	actual := &core.InstanceConfigurationLaunchInstanceDetails{
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			AssignPublicIp: common.Bool(true),
			AssignIpv6Ip:   common.Bool(true),
		},
	}

	desiredHash, err := ComputeHash(desired)
	g.Expect(err).To(BeNil())

	actualHash, err := ComputeComparableHash(actual, desired)
	g.Expect(err).To(BeNil())

	g.Expect(actualHash).ToNot(Equal(desiredHash))
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
	}

	// Compute hash
	hash, err := ComputeHash(ld)
	g.Expect(err).To(BeNil())
	g.Expect(hash).ToNot(BeEmpty())
	g.Expect(hash).To(HaveLen(64))

	// Verify the normalized result contains the canonical comparison projection
	normalized := projectLaunchDetails(ld, ld)

	g.Expect(*normalized.Shape).To(Equal("VM.Standard.E4.Flex"))
	g.Expect(*normalized.CompartmentID).To(Equal("ocid1.compartment.oc1..test"))
	g.Expect(*normalized.DedicatedVMHostID).To(Equal("ocid1.dedicatedvmhost.oc1..test"))

	// Metadata should exclude user_data (tracked separately) but keep other keys
	expectedMetadata := map[string]string{
		"ssh_authorized_keys": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
		"custom_key":          "custom_value",
	}
	g.Expect(normalized.Metadata).To(Equal(expectedMetadata))

	g.Expect(normalized.ShapeConfig).ToNot(BeNil())
	g.Expect(*normalized.ShapeConfig.OCPUs).To(Equal(ocpus))
	g.Expect(*normalized.ShapeConfig.MemoryInGBs).To(Equal(memory))
	g.Expect(*normalized.ShapeConfig.VCPUs).To(Equal(vcpus))
	g.Expect(*normalized.ShapeConfig.NVMEs).To(Equal(nvmes))

	g.Expect(normalized.CreateVnicDetails).ToNot(BeNil())
	g.Expect(normalized.CreateVnicDetails.AssignPublicIP).To(BeNil())
	g.Expect(normalized.CreateVnicDetails.SkipSourceDestCheck).To(BeNil())
	g.Expect(*normalized.CreateVnicDetails.AssignPrivateDNSRecord).To(BeTrue())
	g.Expect(*normalized.CreateVnicDetails.HostnameLabel).To(Equal("test-host"))
	expectedNSGs := []string{"ocid1.nsg.oc1..nsg1", "ocid1.nsg.oc1..nsg2", "ocid1.nsg.oc1..nsg3"}
	g.Expect(normalized.CreateVnicDetails.NSGIDs).To(Equal(expectedNSGs))

	g.Expect(normalized.PlatformConfig).ToNot(BeNil())
	g.Expect(normalized.PlatformConfig.Type).To(Equal("AmdVmPlatformConfig"))
	g.Expect(normalized.AgentConfig).ToNot(BeNil())
	g.Expect(normalized.LaunchOptions).ToNot(BeNil())
	g.Expect(normalized.InstanceOptions).ToNot(BeNil())
	g.Expect(normalized.AvailabilityConfig).ToNot(BeNil())
	g.Expect(normalized.PreemptibleInstanceConfig).To(BeNil())
	g.Expect(normalized.SourceDetails).ToNot(BeNil())
}

func TestComputeHash_ExtendedMetadataChangeProducesDifferentHash(t *testing.T) {
	g := NewWithT(t)

	base := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
	}

	withExtMeta := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
		ExtendedMetadata: map[string]interface{}{
			"cilium-primary-vnic": map[string]interface{}{
				"ip-count": float64(32),
			},
		},
	}

	hashBase, err := ComputeHash(base)
	g.Expect(err).To(BeNil())

	hashWithMeta, err := ComputeHash(withExtMeta)
	g.Expect(err).To(BeNil())

	g.Expect(hashBase).ToNot(Equal(hashWithMeta))
}

func TestComputeComparableHash_ExtendedMetadataProjection(t *testing.T) {
	g := NewWithT(t)

	actual := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
		ExtendedMetadata: map[string]interface{}{
			"cilium-primary-vnic": map[string]interface{}{
				"ip-count": float64(32),
			},
			"oci-added-key": "some-default",
		},
	}

	desired := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
		ExtendedMetadata: map[string]interface{}{
			"cilium-primary-vnic": map[string]interface{}{
				"ip-count": float64(32),
			},
		},
	}

	hashActual, err := ComputeComparableHash(actual, desired)
	g.Expect(err).To(BeNil())

	hashDesired, err := ComputeHash(desired)
	g.Expect(err).To(BeNil())

	// The projected hash should match because oci-added-key is not in the desired mask
	g.Expect(hashActual).To(Equal(hashDesired))
}

func TestComputeHash_NilExtendedMetadataDoesNotAffectHash(t *testing.T) {
	g := NewWithT(t)

	withNil := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
	}

	withEmpty := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape:            common.String("VM.Standard2.1"),
		ExtendedMetadata: map[string]interface{}{},
	}

	hashNil, err := ComputeHash(withNil)
	g.Expect(err).To(BeNil())

	hashEmpty, err := ComputeHash(withEmpty)
	g.Expect(err).To(BeNil())

	g.Expect(hashNil).To(Equal(hashEmpty))
}
