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

// ReconcileInternetGateway tries to move the Internet Gateway to the desired OCICluster Spec
func (s *ClusterScope) ReconcileInternetGateway(ctx context.Context) error {
	if s.OCIClusterAccessor.GetNetworkSpec().Vcn.InternetGateway.Skip {
		s.Logger.Info("Skipping Internet Gateway reconciliation as per spec")
		return nil
	}
	if s.IsAllSubnetsPrivate() {
		s.Logger.Info("All subnets are private, we don't need internet gateway")
		return nil
	}
	var err error
	igw, err := s.GetInternetGateway(ctx)
	if err != nil {
		return err
	}
	if igw != nil {
		s.OCIClusterAccessor.GetNetworkSpec().Vcn.InternetGateway.Id = igw.Id
		s.Logger.Info("No Reconciliation Required for Internet Gateway", "internet_gateway", igw.Id)
		return nil
	}
	internetGateway, err := s.CreateInternetGateway(ctx)
	if err != nil {
		return err
	}
	s.OCIClusterAccessor.GetNetworkSpec().Vcn.InternetGateway.Id = internetGateway
	return err
}

// GetInternetGateway retrieves the Cluster's core.InternetGateway using the one of the following methods
//
// 1. the OCICluster's spec InternetGatewayId
//
// 2. Listing the Internet Gateways for the Compartment (by ID) and filtering by tag
// nolint:nilnil
func (s *ClusterScope) GetInternetGateway(ctx context.Context) (*core.InternetGateway, error) {
	gwId := s.OCIClusterAccessor.GetNetworkSpec().Vcn.InternetGateway.Id
	if gwId != nil {
		resp, err := s.VCNClient.GetInternetGateway(ctx, core.GetInternetGatewayRequest{
			IgId: gwId,
		})
		if err != nil {
			return nil, err
		}
		igw := resp.InternetGateway
		if s.IsResourceCreatedByClusterAPI(igw.FreeformTags) {
			return &igw, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	igws, err := s.VCNClient.ListInternetGateways(ctx, core.ListInternetGatewaysRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		VcnId:         s.getVcnId(),
		DisplayName:   common.String(InternetGatewayName),
	})
	if err != nil {
		s.Logger.Error(err, "failed to list internet gateways")
		return nil, errors.Wrap(err, "failed to list internet gateways")
	}
	for _, igw := range igws.Items {
		if s.IsResourceCreatedByClusterAPI(igw.FreeformTags) {
			return &igw, nil
		}
	}
	return nil, nil
}

// CreateInternetGateway creates the Internet Gateway for the cluster based on the ClusterScope
func (s *ClusterScope) CreateInternetGateway(ctx context.Context) (*string, error) {
	igwDetails := core.CreateInternetGatewayDetails{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(InternetGatewayName),
		IsEnabled:     common.Bool(true),
		VcnId:         s.getVcnId(),
		FreeformTags:  s.GetFreeFormTags(),
		DefinedTags:   s.GetDefinedTags(),
	}
	igwResponse, err := s.VCNClient.CreateInternetGateway(ctx, core.CreateInternetGatewayRequest{
		CreateInternetGatewayDetails: igwDetails,
	})
	if err != nil {
		s.Logger.Error(err, "failed create internet gateway")
		return nil, errors.Wrap(err, "failed create internet gateway")
	}
	s.Logger.Info("Successfully created the internet gateway", "igw", *igwResponse.Id)
	return igwResponse.Id, nil
}

// DeleteInternetGateway retrieves and attempts to delete the Internet Gateway if found.
func (s *ClusterScope) DeleteInternetGateway(ctx context.Context) error {
	if s.OCIClusterAccessor.GetNetworkSpec().Vcn.RouteTable.Skip {
		s.Logger.Info("Skipping Internet Gateway reconciliation as per spec")
		return nil
	}
	igw, err := s.GetInternetGateway(ctx)
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if igw == nil {
		s.Logger.Info("Internet Gateway is already deleted")
		return nil
	}
	_, err = s.VCNClient.DeleteInternetGateway(ctx, core.DeleteInternetGatewayRequest{
		IgId: igw.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to delete InternetGateway")
		return errors.Wrap(err, "failed to delete InternetGateway")
	}
	s.Logger.Info("Successfully deleted InternetGateway", "igw", igw.Id)
	return nil
}
