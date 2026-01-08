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
	"reflect"
	"sort"

	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

// ComputeHash computes a SHA-256 hash of normalized launch details
func ComputeHash(ld *core.InstanceConfigurationLaunchInstanceDetails) (string, error) {
	normalized := NormalizeLaunchDetails(ld)

	// sort map keys for consistent hashing
	b, err := json.Marshal(normalized)
	if err != nil {
		return "", errors.Wrap(err, "marshal normalized launch details")
	}

	// compute hash
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// NormalizeLaunchDetails strips fields that should NOT trigger a new InstanceConfiguration
// we could decide which fields to ignore based on what OCI allows to be updated in an InstanceConfiguration
func NormalizeLaunchDetails(in *core.InstanceConfigurationLaunchInstanceDetails) *core.InstanceConfigurationLaunchInstanceDetails {
	if in == nil {
		return nil
	}

	output := *in
	output.DisplayName = nil
	output.DefinedTags = nil
	output.FreeformTags = nil
	output.SecurityAttributes = nil

	// Normalize Metadata: drop user_data
	output.Metadata = NormalizeMetadata(output.Metadata)

	// Normalize CreateVnicDetails
	if output.CreateVnicDetails != nil {
		v := *output.CreateVnicDetails
		v.DisplayName = nil
		v.DefinedTags = nil
		v.FreeformTags = nil
		v.SecurityAttributes = nil

		// Sort NSG IDs to avoid recreates due to ordering differences
		if len(v.NsgIds) == 0 {
			v.NsgIds = nil
		} else {
			sort.Strings(v.NsgIds)
		}

		output.CreateVnicDetails = &v
	}

	// Normalize ShapeConfig
	if output.ShapeConfig != nil {
		sc := *output.ShapeConfig
		// If all fields are empty, treat as nil
		if sc.Ocpus == nil && sc.MemoryInGBs == nil && sc.Vcpus == nil && sc.Nvmes == nil && sc.BaselineOcpuUtilization == "" {
			output.ShapeConfig = nil
		} else {
			output.ShapeConfig = &sc
		}
	}

	// Normalize LicensingConfigs
	if len(output.LicensingConfigs) == 0 {
		output.LicensingConfigs = nil
	}

	// Normalize ExtendedMetadata
	if output.ExtendedMetadata != nil && len(output.ExtendedMetadata) == 0 {
		output.ExtendedMetadata = nil
	}

	return &output
}

// NormalizeMetadata filters instance metadata to exclude fields like user_data
func NormalizeMetadata(md map[string]string) map[string]string {
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

// HashChanged returns true if the two hashes are different, indicating a configuration change
func HashChanged(hash1, hash2 string) bool {
	return hash1 != hash2
}

// LaunchDetailsEqual returns true if two launch details are equivalent after normalization
func LaunchDetailsEqual(ld1, ld2 *core.InstanceConfigurationLaunchInstanceDetails) bool {
	normalized1 := NormalizeLaunchDetails(ld1)
	normalized2 := NormalizeLaunchDetails(ld2)
	return reflect.DeepEqual(normalized1, normalized2)
}
