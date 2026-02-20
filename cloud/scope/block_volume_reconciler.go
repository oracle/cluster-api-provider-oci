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
	// ADD THIS DEBUG LOGGING
	m.Logger.Info("GetBlockVolume called",

		"spec", spec)

	if spec.AvailabilityDomain == nil {
		return nil, errors.New("BlockVolumeSpec is not set")
	}

	// search by DisplayName.
	name := m.GetBlockVolumeName()
	if name == "" {
		return nil, errors.New("BlockVolume DisplayName cannot be empty")
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
	return nil, nil
}

// GetBlockVolumeId retrieves the block volume ID by searching for a volume
// by DisplayName within the compartment and ensuring Cluster API ownership via tags.
// Returns the volume ID as a string, or empty string if not found.
func (m *MachineScope) GetBlockVolumeId(ctx context.Context) (string, error) {
	spec := m.OCIMachine.Spec.BlockVolumeSpec

	m.Logger.Info("GetBlockVolumeId called",
		"spec", spec)

	if spec.AvailabilityDomain == nil {
		return "", errors.New("BlockVolumeSpec is not set")
	}

	// Search by DisplayName
	name := m.GetBlockVolumeName()
	if name == "" {
		return "", errors.New("BlockVolume DisplayName cannot be empty")
	}

	var page *string
	for {
		resp, err := m.BlockVolumeClient.ListVolumes(ctx, core.ListVolumesRequest{
			CompartmentId: common.String(m.getCompartmentId()),
			DisplayName:   common.String(name),
			Page:          page,
		})
		if err != nil {
			return "", err
		}

		for _, v := range resp.Items {
			if m.IsResourceCreatedByClusterAPI(v.FreeformTags) {
				return *v.Id, nil
			}
		}

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	return "", nil
}

// GetBlockVolumeName returns the desired display name for the block volume. If
// the user hasn't provided one, it derives a sensible default from the cluster name.
func (m *MachineScope) GetBlockVolumeName() string {
	spec := m.OCIMachine.Spec.BlockVolumeSpec
	if spec.AvailabilityDomain == nil {
		return ""
	}
	if spec.DisplayName != nil && *spec.DisplayName != "" {
		return *spec.DisplayName
	}
	return ""
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
		return errors.New("BlockVolumeSpec is not set")
	}

	// Default compartment if not provided
	compartmentId := spec.CompartmentId
	if compartmentId == nil {
		compartmentId = common.String(m.getCompartmentId())
	}

	// Validate required fields
	if spec.AvailabilityDomain == nil || *spec.AvailabilityDomain == "" {
		return errors.New("AvailabilityDomain is required but not set")
	}

	if spec.SizeInGBs == nil {
		return errors.New("SizeInGBs is required but not set")
	}

	blockVolumeName := m.GetBlockVolumeName()
	if blockVolumeName == "" {
		return errors.New("BlockVolume DisplayName cannot be empty")
	}

	// Build CreateVolumeDetails from spec
	createVolumeDetails := core.CreateVolumeDetails{
		AvailabilityDomain: spec.AvailabilityDomain,
		CompartmentId:      compartmentId,
		DisplayName:        common.String(blockVolumeName),
		VpusPerGB:          spec.VpusPerGB,
		SizeInGBs:          spec.SizeInGBs,
		FreeformTags:       m.getFreeFormTags(),
		IsAutoTuneEnabled:  spec.IsAutoTuneEnabled,
		AutotunePolicies:   m.ToOCIAutotunePolicy(),
	}

	// Convert block volume replica details if provided.
	if len(spec.BlockVolumeReplicas) > 0 {
		replicas := make([]core.BlockVolumeReplicaDetails, 0, len(spec.BlockVolumeReplicas))
		for _, r := range spec.BlockVolumeReplicas {
			replicas = append(replicas, core.BlockVolumeReplicaDetails{
				AvailabilityDomain: r.AvailabilityDomain,
				DisplayName:        r.DisplayName,
				XrrKmsKeyId:        r.XrrKmsKeyId,
			})
		}
		createVolumeDetails.BlockVolumeReplicas = replicas
	}

	if m.BlockVolumeClient == nil {
		return errors.New("BlockVolumeClient is nil - was not initialized properly")
	}

	m.Logger.Info("BlockVolumeClient is NOT nil, proceeding with API call")

	resp, err := m.BlockVolumeClient.CreateVolume(ctx, core.CreateVolumeRequest{
		CreateVolumeDetails: createVolumeDetails,
		OpcRetryToken:       ociutil.GetOPCRetryToken("%s-%s", "create-bv", string(m.OCIMachine.UID)),
	})
	if err != nil {
		return err
	}

	vol := resp.Volume
	// Sanity: ensure tag ownership
	if !m.IsResourceCreatedByClusterAPI(vol.FreeformTags) {
		return errors.New("created block volume missing cluster api ownership tags")
	}
	return nil
}

// ReconcileBlockVolume ensures a block volume exists as described by the
// cluster's BlockVolumeSpec. If the volume already exists, it records the ID on
// the spec and returns without changes.
func (m *MachineScope) ReconcileBlockVolume(ctx context.Context) error {
	spec := m.OCIMachine.Spec.BlockVolumeSpec
	if spec.AvailabilityDomain == nil {
		m.Logger.Info("BlockVolumeSpec is not set, skipping reconciliation")
		return nil
	}

	// If neither an explicit ID nor enough inputs to create is provided, skip.
	if spec.SourceDetails == "" && spec.SizeInGBs == nil && (spec.DisplayName == nil || *spec.DisplayName == "") {
		m.Logger.Info("BlockVolumeSpec does not contain an ID, name or size; skipping reconciliation")
		return nil
	}

	vol, err := m.GetBlockVolume(ctx)
	m.Logger.Info("ReconcileBlockVolume - Debug GetBlockVolume", "GetBlockVolume result", vol)
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
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if vol == nil || vol.Id == nil {
		m.Logger.Info("Block Volume already deleted or not found")
		return nil
	}

	_, err = m.BlockVolumeClient.DeleteVolume(ctx, core.DeleteVolumeRequest{VolumeId: vol.Id})
	if err != nil {
		return err
	}
	m.Logger.Info("Successfully initiated Block Volume deletion", "volume", vol.Id)
	return nil
}
