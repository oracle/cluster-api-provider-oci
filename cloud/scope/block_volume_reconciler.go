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

func (s *ClusterScope) CreateBlockVolume(ctx context.Context) (*core.Volume, error) {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	if spec == nil {
		return nil, errors.New("BlockVolumeSpec is not set")
	}

	// Default compartment if not provided
	compartmentId := spec.CompartmentId
	if compartmentId == nil {
		compartmentId = common.String(s.GetCompartmentId())
	}

	// Build CreateVolumeDetails from spec
	details := core.CreateVolumeDetails{
		AvailabilityDomain: spec.AvailabilityDomain,
		CompartmentId:      compartmentId,
		DisplayName:        common.String(s.GetBlockVolumeName()),
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
		OpcRetryToken:       ociutil.GetOPCRetryToken("%s-%s", "create-bv", string(s.OCIClusterAccessor.GetOCIResourceIdentifier())),
	})
	if err != nil {
		return nil, err
	}
	vol := resp.Volume
	// Sanity: ensure tag ownership
	if !s.IsResourceCreatedByClusterAPI(vol.FreeformTags) {
		return nil, errors.New("created block volume missing cluster api ownership tags")
	}
	return &vol, nil
}

// ReconcileBlockVolume ensures a block volume exists as described by the
// cluster's BlockVolumeSpec. If the volume already exists, it records the ID on
// the spec and returns without changes.
func (s *ClusterScope) ReconcileBlockVolume(ctx context.Context) error {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	if spec == nil {
		s.Logger.Info("BlockVolumeSpec is not set, skipping reconciliation")
		return nil
	}

	// If neither an explicit ID nor enough inputs to create is provided, skip.
	if spec.SourceDetails == "" && spec.SizeInGBs == nil && (spec.DisplayName == nil || *spec.DisplayName == "") {
		s.Logger.Info("BlockVolumeSpec does not contain an ID, name or size; skipping reconciliation")
		return nil
	}

	vol, err := s.GetBlockVolume(ctx)
	if err != nil {
		return err
	}
	if vol != nil {
		// Record ID on the spec for downstream consumers.
		spec.SourceDetails = ociutil.DerefString(vol.Id)
		s.Logger.Info("No reconciliation required for Block Volume", "volume", vol.Id)
		return nil
	}

	vol, err = s.CreateBlockVolume(ctx)
	if err != nil {
		return err
	}
	spec.SourceDetails = ociutil.DerefString(vol.Id)
	s.Logger.Info("Successfully created Block Volume", "volume", *vol.Id)
	return nil
}

// DeleteBlockVolume attempts to delete the block volume if it exists.
func (s *ClusterScope) DeleteBlockVolume(ctx context.Context) error {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	if spec == nil {
		s.Logger.Info("BlockVolumeSpec is not set, skipping deletion")
		return nil
	}

	vol, err := s.GetBlockVolume(ctx)
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if vol == nil {
		s.Logger.Info("Block Volume already deleted or not found")
		return nil
	}

	_, err = s.VolumeClient.DeleteVolume(ctx, core.DeleteVolumeRequest{VolumeId: vol.Id})
	if err != nil {
		return err
	}
	s.Logger.Info("Successfully initiated Block Volume deletion", "volume", vol.Id)
	return nil
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

// ToOCIAutotunePolicy converts your AutotunePolicy to the OCI SDK AutotunePolicy interface
func (s *ClusterScope) ToOCIAutotunePolicy() []core.AutotunePolicy {
	spec := s.OCIClusterAccessor.GetBlockVolumeSpec()
	if spec == nil {
		return nil
	}

	a := spec.AutotunePolicies
	if a == nil {
		return nil
	}

	var policy core.AutotunePolicy
	switch a.AutotuneType {
	case "DETACHED_VOLUME":
		policy = core.DetachedVolumeAutotunePolicy{}

	case "PERFORMANCE_BASED":
		if a.MaxVPUsPerGB == nil {
			return nil
		}
		policy = core.PerformanceBasedAutotunePolicy{
			MaxVpusPerGB: a.MaxVPUsPerGB,
		}

	default:
		return nil
	}

	return []core.AutotunePolicy{policy}
}
