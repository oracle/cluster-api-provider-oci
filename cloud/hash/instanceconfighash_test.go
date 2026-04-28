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

func TestComputeHash_ApprovedParityFieldsAffectDesiredHashAndProjection(t *testing.T) {
	baseLaunchDetails := func() *core.InstanceConfigurationLaunchInstanceDetails {
		return &core.InstanceConfigurationLaunchInstanceDetails{
			Shape: common.String("VM.Standard.E4.Flex"),
		}
	}

	tests := []struct {
		name   string
		mutate func(*core.InstanceConfigurationLaunchInstanceDetails)
		assert func(g *WithT, projected *comparableLaunchDetails)
	}{
		{
			name: "cluster placement group id",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.ClusterPlacementGroupId = common.String("ocid1.clusterplacementgroup.oc1..test")
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(*projected.ClusterPlacementGroupID).To(Equal("ocid1.clusterplacementgroup.oc1..test"))
			},
		},
		{
			name: "ipxe script",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.IpxeScript = common.String("#!ipxe")
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(*projected.IpxeScript).To(Equal("#!ipxe"))
			},
		},
		{
			name: "launch mode",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.LaunchMode = core.InstanceConfigurationLaunchInstanceDetailsLaunchModeNative
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.LaunchMode).To(Equal(string(core.InstanceConfigurationLaunchInstanceDetailsLaunchModeNative)))
			},
		},
		{
			name: "licensing configs",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.LicensingConfigs = []core.LaunchInstanceLicensingConfig{
					core.LaunchInstanceWindowsLicensingConfig{
						LicenseType: core.LaunchInstanceLicensingConfigLicenseTypeBringYourOwnLicense,
					},
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.LicensingConfigs).To(Equal([]comparableLicensingConfig{{
					Type:        string(core.LaunchInstanceLicensingConfigTypeWindows),
					LicenseType: string(core.LaunchInstanceLicensingConfigLicenseTypeBringYourOwnLicense),
				}}))
			},
		},
		{
			name: "preferred maintenance action",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.PreferredMaintenanceAction = core.InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionReboot
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.PreferredMaintenanceAction).To(Equal(string(core.InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionReboot)))
			},
		},
		{
			name: "launch security attributes",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.SecurityAttributes = map[string]map[string]interface{}{
					"Oracle-DataSecurity-ZPR": {
						"MaxEgressCount": map[string]interface{}{"value": "42", "mode": "audit"},
					},
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.SecurityAttributes).To(Equal(map[string]map[string]interface{}{
					"Oracle-DataSecurity-ZPR": {
						"MaxEgressCount": map[string]interface{}{"value": "42", "mode": "audit"},
					},
				}))
			},
		},
		{
			name: "shape vcpus",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.ShapeConfig = &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
					Vcpus: common.Int(4),
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(*projected.ShapeConfig.VCPUs).To(Equal(4))
			},
		},
		{
			name: "primary VNIC IPv6 CIDR pairs",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.CreateVnicDetails = &core.InstanceConfigurationCreateVnicDetails{
					Ipv6AddressIpv6SubnetCidrPairDetails: []core.InstanceConfigurationIpv6AddressIpv6SubnetCidrPairDetails{{
						Ipv6SubnetCidr: common.String("2001:db8::/64"),
						Ipv6Address:    common.String("2001:db8::10"),
					}},
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.CreateVnicDetails.IPv6AddressCIDRPairs).To(Equal([]comparableIPv6AddressCIDRPair{{
					IPv6SubnetCIDR: common.String("2001:db8::/64"),
					IPv6Address:    common.String("2001:db8::10"),
				}}))
			},
		},
		{
			name: "primary VNIC security attributes",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.CreateVnicDetails = &core.InstanceConfigurationCreateVnicDetails{
					SecurityAttributes: map[string]map[string]interface{}{
						"Oracle-DataSecurity-ZPR": {
							"VnicEgress": "audit",
						},
					},
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.CreateVnicDetails.SecurityAttributes).To(Equal(map[string]map[string]interface{}{
					"Oracle-DataSecurity-ZPR": {
						"VnicEgress": "audit",
					},
				}))
			},
		},
		{
			name: "AMD Milan BM config map",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.PlatformConfig = core.AmdMilanBmPlatformConfig{
					ConfigMap: map[string]string{"hpc": "enabled"},
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.PlatformConfig.Type).To(Equal("AmdMilanBmPlatformConfig"))
				g.Expect(projected.PlatformConfig.ConfigMap).To(Equal(map[string]string{"hpc": "enabled"}))
			},
		},
		{
			name: "AMD Rome BM GPU config map",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.PlatformConfig = core.AmdRomeBmGpuPlatformConfig{
					ConfigMap: map[string]string{"gpu": "enabled"},
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.PlatformConfig.Type).To(Equal("AmdRomeBmGpuPlatformConfig"))
				g.Expect(projected.PlatformConfig.ConfigMap).To(Equal(map[string]string{"gpu": "enabled"}))
			},
		},
		{
			name: "AMD Rome BM config map",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.PlatformConfig = core.AmdRomeBmPlatformConfig{
					ConfigMap: map[string]string{"numa": "compact"},
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.PlatformConfig.Type).To(Equal("AmdRomeBmPlatformConfig"))
				g.Expect(projected.PlatformConfig.ConfigMap).To(Equal(map[string]string{"numa": "compact"}))
			},
		},
		{
			name: "AMD VM SMT",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.PlatformConfig = core.AmdVmPlatformConfig{
					IsSymmetricMultiThreadingEnabled: common.Bool(true),
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.PlatformConfig.Type).To(Equal("AmdVmPlatformConfig"))
				g.Expect(*projected.PlatformConfig.IsSymmetricMultiThreadingEnabled).To(BeTrue())
			},
		},
		{
			name: "Intel Icelake BM config map",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.PlatformConfig = core.IntelIcelakeBmPlatformConfig{
					ConfigMap: map[string]string{"ice": "lake"},
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.PlatformConfig.Type).To(Equal("IntelIcelakeBmPlatformConfig"))
				g.Expect(projected.PlatformConfig.ConfigMap).To(Equal(map[string]string{"ice": "lake"}))
			},
		},
		{
			name: "Intel Skylake BM approved knobs",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.PlatformConfig = core.IntelSkylakeBmPlatformConfig{
					IsSymmetricMultiThreadingEnabled:         common.Bool(true),
					IsInputOutputMemoryManagementUnitEnabled: common.Bool(true),
					PercentageOfCoresEnabled:                 common.Int(50),
					ConfigMap:                                map[string]string{"sky": "lake"},
					NumaNodesPerSocket:                       core.IntelSkylakeBmPlatformConfigNumaNodesPerSocketNps2,
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.PlatformConfig.Type).To(Equal("IntelSkylakeBmPlatformConfig"))
				g.Expect(*projected.PlatformConfig.IsSymmetricMultiThreadingEnabled).To(BeTrue())
				g.Expect(*projected.PlatformConfig.IsInputOutputMemoryManagementUnitEnabled).To(BeTrue())
				g.Expect(*projected.PlatformConfig.PercentageOfCoresEnabled).To(Equal(50))
				g.Expect(projected.PlatformConfig.ConfigMap).To(Equal(map[string]string{"sky": "lake"}))
				g.Expect(projected.PlatformConfig.NumaNodesPerSocket).To(Equal(string(core.IntelSkylakeBmPlatformConfigNumaNodesPerSocketNps2)))
			},
		},
		{
			name: "Intel VM SMT",
			mutate: func(ld *core.InstanceConfigurationLaunchInstanceDetails) {
				ld.PlatformConfig = core.IntelVmPlatformConfig{
					IsSymmetricMultiThreadingEnabled: common.Bool(true),
				}
			},
			assert: func(g *WithT, projected *comparableLaunchDetails) {
				g.Expect(projected.PlatformConfig.Type).To(Equal("IntelVmPlatformConfig"))
				g.Expect(*projected.PlatformConfig.IsSymmetricMultiThreadingEnabled).To(BeTrue())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			base := baseLaunchDetails()
			baseHash, err := ComputeHash(base)
			g.Expect(err).To(BeNil())

			desired := baseLaunchDetails()
			tt.mutate(desired)

			desiredHash, err := ComputeHash(desired)
			g.Expect(err).To(BeNil())
			g.Expect(desiredHash).ToNot(Equal(baseHash))

			projected := projectLaunchDetails(desired, desired)
			tt.assert(g, projected)

			comparableHash, err := ComputeComparableHash(desired, desired)
			g.Expect(err).To(BeNil())
			g.Expect(comparableHash).To(Equal(desiredHash))
		})
	}
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

func TestComputeComparableHash_IgnoresDefaultOnlyReadbackChurn(t *testing.T) {
	g := NewWithT(t)
	desired := &core.InstanceConfigurationLaunchInstanceDetails{
		CompartmentId: common.String("ocid1.compartment.oc1..test"),
		Shape:         common.String("VM.Standard.E4.Flex"),
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
			Ocpus: common.Float32(1),
		},
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			SubnetId: common.String("ocid1.subnet.oc1..test"),
		},
		Metadata: map[string]string{
			"user_data": "desired-bootstrap",
		},
	}
	actual := &core.InstanceConfigurationLaunchInstanceDetails{
		CompartmentId: common.String("ocid1.compartment.oc1..test"),
		DisplayName:   common.String("server-filled-name"),
		FreeformTags:  map[string]string{"oci-default": "ignored"},
		DefinedTags:   map[string]map[string]interface{}{"Oracle-Tags": {"CreatedBy": "oci"}},
		Shape:         common.String("VM.Standard.E4.Flex"),
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
			Ocpus:       common.Float32(1),
			MemoryInGBs: common.Float32(16),
		},
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			SubnetId:       common.String("ocid1.subnet.oc1..test"),
			DisplayName:    common.String("server-filled-vnic"),
			AssignPublicIp: common.Bool(false),
		},
		AgentConfig: &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{},
		Metadata: map[string]string{
			"user_data": "different-bootstrap-is-tracked-separately",
		},
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
		// Fields that should be excluded from the InstanceConfiguration identity.
		DisplayName:  common.String("test-instance"),
		FreeformTags: map[string]string{"env": "test", "team": "platform"},
		DefinedTags:  map[string]map[string]interface{}{"oracle-tags": {"CreatedBy": "test"}},

		// Fields that should be included
		Shape:              common.String("VM.Standard.E4.Flex"),
		CompartmentId:      common.String("ocid1.compartment.oc1..test"),
		DedicatedVmHostId:  common.String("ocid1.dedicatedvmhost.oc1..test"),
		SecurityAttributes: map[string]map[string]interface{}{"security": {"level": "high"}},

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

		// VNIC details - display name/tags should be excluded, security attributes included, NSGs sorted
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
	g.Expect(normalized.SecurityAttributes).To(Equal(map[string]map[string]interface{}{"security": {"level": "high"}}))

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
	g.Expect(normalized.CreateVnicDetails.SecurityAttributes).To(Equal(map[string]map[string]interface{}{"vnic-sec": {"attr": "val"}}))
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

func TestComputeComparableHash_ExtendedMetadataKeyRemovalDetected(t *testing.T) {
	g := NewWithT(t)

	// Actual instance config still has keys {a, b}
	actual := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
		ExtendedMetadata: map[string]interface{}{
			"cilium-primary-vnic": map[string]interface{}{
				"ip-count": float64(32),
			},
			"removed-key": "old-value",
		},
	}

	// Desired spec now only has {a} — user removed "removed-key"
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

	// Hashes must differ so the key removal triggers an instance config update
	g.Expect(hashActual).ToNot(Equal(hashDesired))
}

func TestComputeComparableHash_ExtendedMetadataTopLevelWorkloadKeyRemovalDetected(t *testing.T) {
	g := NewWithT(t)

	actual := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
		ExtendedMetadata: map[string]interface{}{
			"workload": map[string]interface{}{
				"profile": "standard",
				"features": map[string]interface{}{
					"hpc": true,
					"gpu": true,
				},
			},
			"network": map[string]interface{}{
				"cni": map[string]interface{}{
					"type": "cilium",
					"mode": "overlay",
				},
			},
		},
	}

	// Desired spec keeps network metadata but removes the top-level workload key.
	desired := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
		ExtendedMetadata: map[string]interface{}{
			"network": map[string]interface{}{
				"cni": map[string]interface{}{
					"type": "cilium",
					"mode": "overlay",
				},
			},
		},
	}

	hashActual, err := ComputeComparableHash(actual, desired)
	g.Expect(err).To(BeNil())

	hashDesired, err := ComputeHash(desired)
	g.Expect(err).To(BeNil())

	g.Expect(hashActual).ToNot(Equal(hashDesired))
}

func TestComputeComparableHash_ExtendedMetadataSameKeysMatch(t *testing.T) {
	g := NewWithT(t)

	actual := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
		ExtendedMetadata: map[string]interface{}{
			"cilium-primary-vnic": map[string]interface{}{
				"ip-count": float64(32),
			},
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

	g.Expect(hashActual).To(Equal(hashDesired))
}

func TestComputeComparableHash_ExtendedMetadataNilDesiredDetected(t *testing.T) {
	g := NewWithT(t)

	actual := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape: common.String("VM.Standard2.1"),
		ExtendedMetadata: map[string]interface{}{
			"stale-key": "old-value",
		},
	}

	desired := &core.InstanceConfigurationLaunchInstanceDetails{
		Shape:            common.String("VM.Standard2.1"),
		ExtendedMetadata: nil,
	}

	hashActual, err := ComputeComparableHash(actual, desired)
	g.Expect(err).To(BeNil())

	hashDesired, err := ComputeHash(desired)
	g.Expect(err).To(BeNil())

	g.Expect(hashActual).ToNot(Equal(hashDesired))
}

func TestComputeComparableHash_ExtendedMetadataClearDetected(t *testing.T) {
	tests := []struct {
		name                string
		desiredExtendedMeta map[string]interface{}
	}{
		{
			name:                "nil desired extended metadata",
			desiredExtendedMeta: nil,
		},
		{
			name:                "empty desired extended metadata",
			desiredExtendedMeta: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			actual := &core.InstanceConfigurationLaunchInstanceDetails{
				Shape: common.String("VM.Standard2.1"),
				ExtendedMetadata: map[string]interface{}{
					"stale-key": "old-value",
				},
			}

			desired := &core.InstanceConfigurationLaunchInstanceDetails{
				Shape:            common.String("VM.Standard2.1"),
				ExtendedMetadata: tt.desiredExtendedMeta,
			}

			hashActual, err := ComputeComparableHash(actual, desired)
			g.Expect(err).To(BeNil())

			hashDesired, err := ComputeHash(desired)
			g.Expect(err).To(BeNil())

			g.Expect(hashActual).ToNot(Equal(hashDesired))
		})
	}
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
