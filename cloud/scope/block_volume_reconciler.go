// /*
//  Copyright (c) 2025 Oracle and/or its affiliates.

//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at

//       https://www.apache.org/licenses/LICENSE-2.0

//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
// */

package scope

import (
	"context"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil/ptr"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

// GetBlockVolume retrieves a core.Volume using either the VolumeId recorded in the
// spec or by listing volumes by DisplayName within the compartment and ensuring
// Cluster API ownership via tags.
// nolint:nilnil
func (m *MachineScope) GetBlockVolume(ctx context.Context) (*core.Volume, error) {
	spec := m.OCIMachine.Spec.BlockVolumeSpec

	if spec.AvailabilityDomain == nil {
		return nil, errors.New("BlockVolumeSpec availabilityDomain is not set, but required")
	}

	// search by DisplayName.
	name := m.GetBlockVolumeDesiredName()
	if name == "" {
		return nil, errors.New("BlockVolumeSpec displayName is not set, but required")
	}

	var page *string
	for {
		resp, err := m.BlockVolumeClient.ListVolumes(ctx, core.ListVolumesRequest{
			CompartmentId: common.String(m.getCompartmentId()),
			DisplayName:   common.String(name),
			Page:          page,
		})
		if err != nil {
			return nil, err
		}
		for _, v := range resp.Items {
			if m.IsResourceCreatedByClusterAPI(v.FreeformTags) {
				return &v, nil
			}
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	m.Logger.Info("There is not any Block Volume available at the moment with specified DisplayName")
	return nil, nil
}

// GetBlockVolumeId retrieves the block volume OCID by searching for a volume
// by DisplayName within the compartment and ensuring Cluster API ownership via tags.
// Returns the volume OCID as a string, or empty string if not found.
func (m *MachineScope) GetBlockVolumeId(ctx context.Context) (string, error) {
	volume, err := m.GetBlockVolume(ctx)
	if err != nil {
		return "", err
	}
	return ptr.ToString(volume.Id), nil
}

// GetBlockVolumeDesiredName returns the desired display name for the block volume. If
// the user hasn't provided one, it derives a sensible default from the cluster name.
func (m *MachineScope) GetBlockVolumeDesiredName() string {
	return ptr.ToString(m.OCIMachine.Spec.BlockVolumeSpec.DisplayName)
}

// ToOCIAutotunePolicy converts your AutotunePolicy to the OCI SDK AutotunePolicy interface
func (m *MachineScope) ToOCIAutotunePolicy() []core.AutotunePolicy {
	spec := m.OCIMachine.Spec.BlockVolumeSpec
	if spec.AvailabilityDomain == nil || len(spec.AutotunePolicies) == 0 {
		return nil
	}

	policies := make([]core.AutotunePolicy, 0, len(spec.AutotunePolicies))

	for _, a := range spec.AutotunePolicies {
		switch a.AutotuneType {
		case "DETACHED_VOLUME":
			policies = append(policies, core.DetachedVolumeAutotunePolicy{})

		case "PERFORMANCE_BASED":
			if a.MaxVPUsPerGB == nil {
				// skip invalid policy
				continue
			}
			policies = append(policies, core.PerformanceBasedAutotunePolicy{
				MaxVpusPerGB: a.MaxVPUsPerGB,
			})

		default:
			// unknown policy type, skip
			continue
		}
	}

	if len(policies) == 0 {
		return nil
	}

	return policies
}

func (m *MachineScope) CreateBlockVolume(ctx context.Context) error {
	spec := m.OCIMachine.Spec.BlockVolumeSpec
	if spec.AvailabilityDomain == nil {
		return errors.New("BlockVolumeSpec availabilityDomain is not set, but required")
	}

	blockVolumeName := m.GetBlockVolumeDesiredName()
	if blockVolumeName == "" {
		return errors.New("BlockVolumeSpec displayName is not set, but required")
	}

	// If compartmentId not specified in BlockVolumeSpec, take compartmentId from default compartment
	compartmentId := spec.CompartmentId
	if compartmentId == nil {
		compartmentId = common.String(m.getCompartmentId())
	}

	if m.BlockVolumeClient == nil {
		return errors.New("BlockVolumeClient is nil - was not initialized properly")
	}

	// Build CreateVolumeDetails from spec
	createVolumeDetails := core.CreateVolumeDetails{
		AvailabilityDomain: spec.AvailabilityDomain,
		CompartmentId:      compartmentId,
		DisplayName:        common.String(blockVolumeName),
		VpusPerGB:          spec.VpusPerGB,
		SizeInGBs:          spec.SizeInGBs,
		FreeformTags:       m.getFreeFormTags(),
		AutotunePolicies:   m.ToOCIAutotunePolicy(),
	}

	_, err := m.BlockVolumeClient.CreateVolume(ctx, core.CreateVolumeRequest{
		CreateVolumeDetails: createVolumeDetails,
		OpcRetryToken:       ociutil.GetOPCRetryToken("%s-%s", "create-bv", string(m.OCIMachine.UID)),
	})

	if err != nil {
		return err
	}

	return nil
}

// ReconcileBlockVolume ensures a block volume exists as described by the
// cluster's BlockVolumeSpec. If the volume already exists, it records the OCID on
// the spec and returns without changes.
func (m *MachineScope) ReconcileBlockVolume(ctx context.Context) error {
	vol, err := m.GetBlockVolume(ctx)

	if err != nil {
		return err
	}

	// If there was not found any BlockVolume already created with BlockVolumeSpec configuration, it will create it
	if vol == nil {
		m.Logger.Info("Creating Block Volume")
		err = m.CreateBlockVolume(ctx)
	}

	return err
}

// DeleteBlockVolume attempts to delete the block volume if it exists.
func (m *MachineScope) DeleteBlockVolume(ctx context.Context) error {
	if m.BlockVolumeClient == nil {
		m.Logger.Info("BlockVolumeClient is not set, skipping deletion")
		return nil
	}

	spec := m.OCIMachine.Spec.BlockVolumeSpec
	if spec.AvailabilityDomain == nil {
		m.Logger.Info("BlockVolumeSpec is not set, skipping deletion")
		return nil
	}

	vol, err := m.GetBlockVolume(ctx)
	if vol == nil {
		m.Logger.Info("Block Volume already deleted or not found")
		return nil
	}

	_, err = m.BlockVolumeClient.DeleteVolume(ctx, core.DeleteVolumeRequest{VolumeId: vol.Id})
	if err != nil {
		return err
	}

	return nil
}
