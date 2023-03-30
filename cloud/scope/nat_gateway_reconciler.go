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

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

// ReconcileNatGateway tries to move the NAT Gateway to the desired OCICluster Spec
func (s *ClusterScope) ReconcileNatGateway(ctx context.Context) error {
	if s.OCIClusterAccessor.GetNetworkSpec().Vcn.NATGateway.Skip {
		s.Logger.Info("Skipping NAT Gateway reconciliation as per spec")
		return nil
	}
	if s.IsAllSubnetsPublic() {
		s.Logger.Info("All subnets are public, we don't need NAT gateway")
		return nil
	}
	var err error
	ngw, err := s.GetNatGateway(ctx)
	if err != nil {
		return err
	}
	if ngw != nil {
		s.OCIClusterAccessor.GetNetworkSpec().Vcn.NATGateway.Id = ngw.Id
		s.Logger.Info("No Reconciliation Required for Nat Gateway", "nat_gateway", ngw.Id)
		return nil
	}
	natGateway, err := s.CreateNatGateway(ctx)
	s.OCIClusterAccessor.GetNetworkSpec().Vcn.NATGateway.Id = natGateway
	return err
}

// GetNatGateway retrieves the Cluster's core.NatGateway using the one of the following methods
//
// 1. the OCICluster's spec NatGatewayId
//
// 2. Listing the NAT Gateways for the Compartment (by ID), VCN and DisplayName and filtering by tag
func (s *ClusterScope) GetNatGateway(ctx context.Context) (*core.NatGateway, error) {
	ngwId := s.OCIClusterAccessor.GetNetworkSpec().Vcn.NATGateway.Id
	if ngwId != nil {
		resp, err := s.VCNClient.GetNatGateway(ctx, core.GetNatGatewayRequest{
			NatGatewayId: ngwId,
		})
		if err != nil {
			return nil, err
		}
		ngw := resp.NatGateway
		if s.IsResourceCreatedByClusterAPI(ngw.FreeformTags) {
			return &ngw, err
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	ngws, err := s.VCNClient.ListNatGateways(ctx, core.ListNatGatewaysRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		VcnId:         s.getVcnId(),
		DisplayName:   common.String(NatGatewayName),
	})
	if err != nil {
		s.Logger.Error(err, "Failed to list NAT gateways")
		return nil, errors.Wrap(err, "failed to list NAT gateways")
	}
	for _, ngw := range ngws.Items {
		if s.IsResourceCreatedByClusterAPI(ngw.FreeformTags) {
			return &ngw, nil
		}
	}
	return nil, nil
}

// UpdateNatGateway updates the FreeFormTags and DefinedTags
func (s *ClusterScope) UpdateNatGateway(ctx context.Context) error {
	updateNGWDetails := core.UpdateNatGatewayDetails{
		FreeformTags: s.GetFreeFormTags(),
		DefinedTags:  s.GetDefinedTags(),
	}
	igwResponse, err := s.VCNClient.UpdateNatGateway(ctx, core.UpdateNatGatewayRequest{
		NatGatewayId:            s.OCIClusterAccessor.GetNetworkSpec().Vcn.NATGateway.Id,
		UpdateNatGatewayDetails: updateNGWDetails,
	})
	if err != nil {
		s.Logger.Error(err, "Failed to reconcile the nat gateway, failed to update")
		return errors.Wrap(err, "failed to reconcile the nat gateway, failed to update")
	}
	s.Logger.Info("Successfully updated the nat gateway", "nat_gateway", *igwResponse.Id)
	return nil
}

// CreateNatGateway creates the NAT Gateway for the cluster based on the ClusterScope
func (s *ClusterScope) CreateNatGateway(ctx context.Context) (*string, error) {
	ngwDetails := core.CreateNatGatewayDetails{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(NatGatewayName),
		VcnId:         s.getVcnId(),
		FreeformTags:  s.GetFreeFormTags(),
		DefinedTags:   s.GetDefinedTags(),
	}
	ngwResponse, err := s.VCNClient.CreateNatGateway(ctx, core.CreateNatGatewayRequest{
		CreateNatGatewayDetails: ngwDetails,
	})
	if err != nil {
		s.Logger.Error(err, "failed create nat gateway")
		return nil, errors.Wrap(err, "failed create nat gateway")
	}
	s.Logger.Info("Successfully created the nat gateway", "ngw", *ngwResponse.Id)
	return ngwResponse.Id, nil
}

// DeleteNatGateway retrieves and attempts to delete the NAT Gateway if found.
func (s *ClusterScope) DeleteNatGateway(ctx context.Context) error {
	if s.OCIClusterAccessor.GetNetworkSpec().Vcn.NATGateway.Skip {
		s.Logger.Info("Skipping NAT Gateway reconciliation as per spec")
		return nil
	}
	ngw, err := s.GetNatGateway(ctx)
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if ngw == nil {
		s.Logger.Info("NAT Gateway is already deleted")
		return nil
	}
	_, err = s.VCNClient.DeleteNatGateway(ctx, core.DeleteNatGatewayRequest{
		NatGatewayId: ngw.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to delete NatGateway")
		return errors.Wrap(err, "failed to delete NatGateway")
	}
	s.Logger.Info("Successfully deleted NatGateway", "ngw", ngw.Id)
	return nil
}
