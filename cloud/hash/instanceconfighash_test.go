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
	result := NormalizeLaunchDetails(nil)
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

	normalized := NormalizeLaunchDetails(original)

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

	normalized := NormalizeLaunchDetails(original)

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

	normalized := NormalizeLaunchDetails(original)

	g.Expect(normalized.CreateVnicDetails.NsgIds).To(BeNil())
}

func TestNormalizeLaunchDetails_EmptyShapeConfig(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{},
	}

	normalized := NormalizeLaunchDetails(original)

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

	normalized := NormalizeLaunchDetails(original)

	g.Expect(normalized.ShapeConfig).ToNot(BeNil())
	g.Expect(*normalized.ShapeConfig.Ocpus).To(Equal(ocpus))
}

func TestNormalizeLaunchDetails_EmptyExtendedMetadata(t *testing.T) {
	g := NewWithT(t)
	original := &core.InstanceConfigurationLaunchInstanceDetails{
		ExtendedMetadata: map[string]interface{}{},
	}

	normalized := NormalizeLaunchDetails(original)

	g.Expect(normalized.ExtendedMetadata).To(BeNil())
}

func TestNormalizeMetadata_NilInput(t *testing.T) {
	g := NewWithT(t)
	result := NormalizeMetadata(nil)
	g.Expect(result).To(BeNil())
}

func TestNormalizeMetadata_ExcludesUserData(t *testing.T) {
	g := NewWithT(t)
	original := map[string]string{
		"user_data":           "sensitive-data",
		"ssh_authorized_keys": "ssh-key",
		"other_key":           "other_value",
	}

	normalized := NormalizeMetadata(original)

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

	normalized := NormalizeMetadata(original)

	g.Expect(normalized).To(BeNil())
}

func TestNormalizeMetadata_NoUserData(t *testing.T) {
	g := NewWithT(t)
	original := map[string]string{
		"ssh_authorized_keys": "ssh-key",
		"other_key":           "other_value",
	}

	normalized := NormalizeMetadata(original)

	g.Expect(normalized).To(Equal(original))
}

func TestHashChanged_SameHashes(t *testing.T) {
	g := NewWithT(t)
	g.Expect(HashChanged("hash123", "hash123")).To(BeFalse())
}

func TestHashChanged_DifferentHashes(t *testing.T) {
	g := NewWithT(t)
	g.Expect(HashChanged("hash123", "hash456")).To(BeTrue())
}

func TestLaunchDetailsEqual_NilInputs(t *testing.T) {
	g := NewWithT(t)
	g.Expect(LaunchDetailsEqual(nil, nil)).To(BeTrue())
}

func TestLaunchDetailsEqual_OneNil(t *testing.T) {
	g := NewWithT(t)
	ld := &core.InstanceConfigurationLaunchInstanceDetails{Shape: common.String("VM.Standard2.1")}
	g.Expect(LaunchDetailsEqual(ld, nil)).To(BeFalse())
	g.Expect(LaunchDetailsEqual(nil, ld)).To(BeFalse())
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

	g.Expect(LaunchDetailsEqual(ld1, ld2)).To(BeTrue())
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

	g.Expect(LaunchDetailsEqual(ld1, ld2)).To(BeFalse())
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
