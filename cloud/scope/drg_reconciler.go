/*
 Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/pkg/errors"
)

// ReconcileDRG tries to move the DRG to the desired OCICluster Spec
func (s *ClusterScope) ReconcileDRG(ctx context.Context) error {
	if !s.isPeeringEnabled() {
		s.Logger.Info("VCN Peering is not enabled, ignoring reconciliation")
		return nil
	}
	if s.getDRG() == nil {
		s.Logger.Info("DRG is not enabled, ignoring reconciliation")
		return nil
	}
	if !s.getDRG().Manage {
		s.Logger.Info("DRG Manage is specified as false, will skip reconciliation of the DRG")
		return nil
	}
	drg, err := s.GetDRG(ctx)
	if err != nil {
		return err
	}
	if drg != nil {
		s.getDRG().ID = drg.Id
		if !s.IsTagsEqual(drg.FreeformTags, drg.DefinedTags) {
			_, err := s.updateDRG(ctx)
			if err != nil {
				return err
			}
		}
		s.Logger.Info("No Reconciliation Required for DRG", "drg", drg.Id)
		return nil
	}

	drg, err = s.createDRG(ctx)
	if err != nil {
		return err
	}
	s.getDRG().ID = drg.Id
	s.Logger.Info("Successfully created DRG", "drg", *drg.Id)
	return nil
}

// GetDRG retrieves the Cluster's core.Drg using the one of the following methods
//
// 1. the OCICluster's spec Drg
//
// 2. Listing the Drgs for the Compartment (by ID) and filtering by tag
func (s *ClusterScope) GetDRG(ctx context.Context) (*core.Drg, error) {
	drgId := s.getDRG().ID
	if drgId != nil {
		response, err := s.VCNClient.GetDrg(ctx, core.GetDrgRequest{
			DrgId: drgId,
		})
		if err != nil {
			return nil, err
		}
		drg := response.Drg
		if s.IsResourceCreatedByClusterAPI(drg.FreeformTags) {
			return &drg, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	var page *string

	for {
		response, err := s.VCNClient.ListDrgs(ctx, core.ListDrgsRequest{
			CompartmentId: common.String(s.GetCompartmentId()),
			Page:          page,
		})
		if err != nil {
			return nil, err
		}
		for _, drg := range response.Items {
			if *drg.DisplayName == s.getDRG().Name {
				if s.IsResourceCreatedByClusterAPI(drg.FreeformTags) {
					return &drg, nil
				} else {
					return nil, errors.New("cluster api tags have been modified out of context")
				}
			}
		}
		if response.OpcNextPage == nil {
			break
		} else {
			page = response.OpcNextPage
		}
	}

	return nil, nil
}

func (s *ClusterScope) createDRG(ctx context.Context) (*core.Drg, error) {
	response, err := s.VCNClient.CreateDrg(ctx, core.CreateDrgRequest{
		CreateDrgDetails: core.CreateDrgDetails{
			CompartmentId: common.String(s.GetCompartmentId()),
			FreeformTags:  s.GetFreeFormTags(),
			DefinedTags:   s.GetDefinedTags(),
			DisplayName:   common.String(s.GetDRGName()),
		},
		OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-drg", string(s.OCICluster.GetOCIResourceIdentifier())),
	})
	if err != nil {
		return nil, err
	}
	return &response.Drg, nil
}

func (s *ClusterScope) updateDRG(ctx context.Context) (*core.Drg, error) {
	response, err := s.VCNClient.UpdateDrg(ctx, core.UpdateDrgRequest{
		DrgId: s.getDRG().ID,
		UpdateDrgDetails: core.UpdateDrgDetails{
			FreeformTags: s.GetFreeFormTags(),
			DefinedTags:  s.GetDefinedTags(),
		},
	})
	if err != nil {
		return nil, err
	}
	return &response.Drg, nil
}

func (s *ClusterScope) GetDRGName() string {
	if s.getDRG().Name != "" {
		return s.getDRG().Name
	}
	return fmt.Sprintf("%s", s.OCICluster.Name)
}

// DeleteDRG tries to delete the DRG
func (s *ClusterScope) DeleteDRG(ctx context.Context) error {
	if !s.isPeeringEnabled() {
		s.Logger.Info("VCN Peering is not enabled, ignoring reconciliation")
		return nil
	}
	if s.getDRG() == nil {
		s.Logger.Info("DRG is not enabled, ignoring reconciliation")
		return nil
	}
	if !s.getDRG().Manage {
		s.Logger.Info("DRG Manage is specified as false, will skip deletion of the DRG")
		return nil
	}

	drg, err := s.GetDRG(ctx)

	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if drg == nil {
		s.Logger.Info("DRG is already deleted")
		return nil
	}
	_, err = s.VCNClient.DeleteDrg(ctx, core.DeleteDrgRequest{
		DrgId: drg.Id,
	})
	if err != nil {
		return err
	}
	return nil
}
