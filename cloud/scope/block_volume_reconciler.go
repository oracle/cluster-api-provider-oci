/*
 Copyright (c) 2025 Oracle and/or its affiliates.

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

package scope

import (
	"context"
	"fmt"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

func (s *ClusterScope) CreateBlockVolumes(ctx context.Context) ([]*core.Volume, error) {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	// ADD THIS DEBUG LOGGING
	s.Logger.Info("CreateBlockVolume called",
		"hasSpec", spec != nil,
		"spec", spec)

	if spec == nil {
		return nil, errors.New("BlockVolumeSpec is not set")
	}

	// Default compartment if not provided
	compartmentId := spec.CompartmentId
	if compartmentId == nil {
		compartmentId = common.String(s.GetCompartmentId())
	}

	// Store created volumes
	var createdVolumes []*core.Volume

	for i := 0; i < int(*spec.NumberOfBlockVolumes); i++ {
		volumeName := common.String(fmt.Sprintf("%s-%d", s.GetBlockVolumeName(), i))

		// Check if volume already exists
		existingVol, err := s.GetBlockVolume(ctx)
		if err != nil {
			return createdVolumes, fmt.Errorf("failed to check existing volume %s: %w", *volumeName, err)
		}

		if existingVol != nil {
			s.Logger.Info("Block Volume already exists, skipping creation",
				"volume", *existingVol.Id,
				"name", *volumeName)
			createdVolumes = append(createdVolumes, existingVol)
			continue
		}
		// Build CreateVolumeDetails from spec
		details := core.CreateVolumeDetails{
			AvailabilityDomain: spec.AvailabilityDomain,
			CompartmentId:      compartmentId,
			DisplayName:        volumeName,
			VpusPerGB:          spec.VpusPerGB,
			SizeInGBs:          spec.SizeInGBs,
			FreeformTags:       s.GetFreeFormTags(),
			AutotunePolicies:   s.ToOCIAutotunePolicy(),
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
			details.BlockVolumeReplicas = replicas
		}

		resp, err := s.VolumeClient.CreateVolume(ctx, core.CreateVolumeRequest{
			CreateVolumeDetails: details,
			OpcRetryToken:       ociutil.GetOPCRetryToken("%s-%s", "create-bv", common.String(fmt.Sprintf("%s-%d", s.OCIClusterAccessor.GetOCIResourceIdentifier(), i)))})
		if err != nil {
			return nil, err
		}
		vol := resp.Volume
		// Sanity: ensure tag ownership
		if !s.IsResourceCreatedByClusterAPI(vol.FreeformTags) {
			return nil, errors.New("created block volume missing cluster api ownership tags")
		}

		createdVolumes = append(createdVolumes, &vol)
		s.Logger.Info("Successfully created Block Volume", "volume", *vol.Id, "name", *volumeName)

	}
	return createdVolumes, nil
}

// ReconcileBlockVolume ensures block volumes exist as described by the
// cluster's BlockVolumeSpec. If volumes already exist, it records the IDs on
// the spec and returns without changes.
func (s *ClusterScope) ReconcileBlockVolume(ctx context.Context) error {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()

	s.Logger.Info("ReconcileBlockVolume called",
		"hasSpec", spec != nil,
		"spec", spec)

	if spec == nil {
		s.Logger.Info("BlockVolumeSpec is not set, skipping reconciliation")
		return nil
	}

	// Check if volumes already exist
	existingVols, err := s.GetBlockVolumes(ctx)
	if err != nil {
		return err
	}

	if len(existingVols) >= int(*spec.NumberOfBlockVolumes) {
		s.Logger.Info("Block volumes already exist", "count", len(existingVols))
		return nil
	}

	// Create volumes
	vols, err := s.CreateBlockVolumes(ctx)
	if err != nil {
		return err
	}

	s.Logger.Info("Successfully reconciled Block Volumes", "count", len(vols))
	return nil
}

// DeleteBlockVolumes attempts to delete all block volumes if they exist.
func (s *ClusterScope) DeleteBlockVolumes(ctx context.Context) error {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	if spec == nil {
		s.Logger.Info("BlockVolumeSpec is not set, skipping deletion")
		return nil
	}

	vols, err := s.GetBlockVolumes(ctx)
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}

	if len(vols) == 0 {
		s.Logger.Info("Block Volumes already deleted or not found")
		return nil
	}

	// Delete all volumes
	var deletionErrors []error
	for _, vol := range vols {
		s.Logger.Info("Deleting Block Volume", "volume", *vol.Id, "name", *vol.DisplayName)

		_, err = s.VolumeClient.DeleteVolume(ctx, core.DeleteVolumeRequest{
			VolumeId: vol.Id,
		})
		if err != nil {
			if ociutil.IsNotFound(err) {
				s.Logger.Info("Block Volume already deleted", "volume", *vol.Id)
				continue
			}
			deletionErrors = append(deletionErrors, fmt.Errorf("failed to delete volume %s: %w", *vol.Id, err))
			continue
		}

		s.Logger.Info("Successfully initiated Block Volume deletion", "volume", *vol.Id)
	}

	// Return combined errors if any occurred
	if len(deletionErrors) > 0 {
		return fmt.Errorf("errors deleting volumes: %v", deletionErrors)
	}

	return nil
}

// GetBlockVolumeName returns the desired display name for the block volume. If
// the user hasn't provided one, it derives a sensible default from the cluster name.
func (s *ClusterScope) GetBlockVolumeName() string {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	if spec == nil {
		return ""
	}
	if spec.DisplayName != nil && *spec.DisplayName != "" {
		return *spec.DisplayName
	}
	// Default name: <clusterName>-block-volume
	return fmt.Sprintf("%s", s.OCIClusterAccessor.GetName())
}

// GetBlockVolume retrieves a core.Volume using either the VolumeId recorded in the
// spec or by listing volumes by DisplayName within the compartment and ensuring
// Cluster API ownership via tags.
// nolint:nilnil
func (s *ClusterScope) GetBlockVolume(ctx context.Context) (*core.Volume, error) {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	if spec == nil {
		return nil, nil
	}

	// First try by explicit ID if provided
	if spec.SourceDetails != "" {
		resp, err := s.VolumeClient.GetVolume(ctx, core.GetVolumeRequest{VolumeId: common.String(spec.SourceDetails)})
		if err != nil {
			return nil, err
		}
		vol := resp.Volume
		if s.IsResourceCreatedByClusterAPI(vol.FreeformTags) {
			return &vol, nil
		}
		return nil, errors.New("cluster api tags have been modified out of context")
	}

	// Otherwise search by DisplayName.
	name := s.GetBlockVolumeName()
	if name == "" {
		return nil, nil
	}

	var page *string
	for {
		resp, err := s.VolumeClient.ListVolumes(ctx, core.ListVolumesRequest{
			CompartmentId: common.String(s.GetCompartmentId()),
			DisplayName:   common.String(name),
			Page:          page,
		})
		if err != nil {
			return nil, err
		}
		for _, v := range resp.Items {
			if s.IsResourceCreatedByClusterAPI(v.FreeformTags) {
				// fetch full details to have consistent return type behavior with GetVolume
				getResp, err := s.VolumeClient.GetVolume(ctx, core.GetVolumeRequest{VolumeId: v.Id})
				if err != nil {
					return nil, err
				}
				vol := getResp.Volume
				return &vol, nil
			}
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return nil, nil
}

// GetBlockVolumes retrieves all block volumes associated with this cluster.
func (s *ClusterScope) GetBlockVolumes(ctx context.Context) ([]*core.Volume, error) {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	if spec == nil {
		return nil, nil
	}

	// Default compartment if not provided
	compartmentId := spec.CompartmentId
	if compartmentId == nil {
		compartmentId = common.String(s.GetCompartmentId())
	}

	var volumes []*core.Volume

	// Iterate through expected volume names
	for i := 1; i <= int(*spec.NumberOfBlockVolumes); i++ {
		volumeName := fmt.Sprintf("%s-%d", s.GetBlockVolumeName(), i)

		// List volumes with filter
		req := core.ListVolumesRequest{
			CompartmentId:      compartmentId,
			DisplayName:        common.String(volumeName),
			AvailabilityDomain: spec.AvailabilityDomain,
		}

		resp, err := s.VolumeClient.ListVolumes(ctx, req)
		if err != nil {
			return volumes, fmt.Errorf("failed to list volumes for %s: %w", volumeName, err)
		}

		// Filter by cluster ownership tags
		for _, vol := range resp.Items {
			if s.IsResourceCreatedByClusterAPI(vol.FreeformTags) {
				// Check if volume is in a valid state
				if vol.LifecycleState != core.VolumeLifecycleStateTerminated &&
					vol.LifecycleState != core.VolumeLifecycleStateTerminating {
					volumes = append(volumes, &vol)
					s.Logger.Info("Found Block Volume",
						"volume", *vol.Id,
						"name", *vol.DisplayName,
						"state", vol.LifecycleState)
				}
			}
		}
	}

	return volumes, nil
}

// ToOCIAutotunePolicy converts AutotunePolicies to OCI SDK AutotunePolicy interfaces
func (s *ClusterScope) ToOCIAutotunePolicy() []core.AutotunePolicy {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	if spec == nil || len(spec.AutotunePolicies) == 0 {
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
