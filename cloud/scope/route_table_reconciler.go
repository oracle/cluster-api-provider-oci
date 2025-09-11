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
	"strings"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

func (s *ClusterScope) ReconcileRouteTable(ctx context.Context) error {
	if s.OCIClusterAccessor.GetNetworkSpec().Vcn.RouteTable.Skip {
		s.Logger.Info("Skipping Route table reconciliation as per spec")
		return nil
	}
	desiredRouteTables := s.GetDesiredRouteTables()
	for _, rt := range desiredRouteTables {
		routeTable, err := s.getRouteTable(ctx, rt)
		if err != nil {
			return err
		}
		if routeTable != nil {
			routeTableOCID := routeTable.Id
			s.setRTStatus(routeTableOCID, rt)
			s.Logger.Info("No Reconciliation Required for Route Table", "route-table", routeTableOCID)
			continue
		}

		s.Logger.Info("Creating the route table")
		rtId, err := s.CreateRouteTable(ctx, rt)
		if err != nil {
			return err
		}
		s.Logger.Info("Created the route table", "route-table", rtId)
		s.setRTStatus(rtId, rt)
	}
	return nil
}

func (s *ClusterScope) GetDesiredRouteTables() []string {
	var desiredRouteTables []string
	if s.IsAllSubnetsPrivate() {
		desiredRouteTables = []string{infrastructurev1beta2.Private}
	} else if s.IsAllSubnetsPublic() {
		desiredRouteTables = []string{infrastructurev1beta2.Public}
	} else {
		desiredRouteTables = []string{infrastructurev1beta2.Private, infrastructurev1beta2.Public}
	}
	return desiredRouteTables
}

// nolint:nilnil
func (s *ClusterScope) getRouteTable(ctx context.Context, routeTableType string) (*core.RouteTable, error) {
	routeTableId := s.OCIClusterAccessor.GetNetworkSpec().Vcn.RouteTable.PublicRouteTableId
	if routeTableType == infrastructurev1beta2.Private {
		routeTableId = s.OCIClusterAccessor.GetNetworkSpec().Vcn.RouteTable.PrivateRouteTableId
	}
	if routeTableId != nil {
		resp, err := s.VCNClient.GetRouteTable(ctx, core.GetRouteTableRequest{
			RtId: routeTableId,
		})
		if err != nil {
			return nil, err
		}
		rt := resp.RouteTable
		if s.IsResourceCreatedByClusterAPI(rt.FreeformTags) {
			return &rt, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}

	}
	vcnId := s.getVcnId()
	routeTableName := PublicRouteTableName
	if routeTableType == infrastructurev1beta2.Private {
		routeTableName = PrivateRouteTableName
	}

	rts, err := s.VCNClient.ListRouteTables(ctx, core.ListRouteTablesRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		VcnId:         vcnId,
		DisplayName:   common.String(routeTableName),
	})
	if err != nil {
		s.Logger.Error(err, "failed to list route tables")
		return nil, errors.Wrap(err, "failed to list route tables")
	}
	for _, rt := range rts.Items {
		if s.IsResourceCreatedByClusterAPI(rt.FreeformTags) {
			return &rt, nil
		}
	}

	return nil, nil
}

func (s *ClusterScope) CreateRouteTable(ctx context.Context, routeTableType string) (*string, error) {
	var routeRules []core.RouteRule
	var routeTableName string
	if routeTableType == infrastructurev1beta2.Private {
		routeRules = []core.RouteRule{
			{
				DestinationType: core.RouteRuleDestinationTypeCidrBlock,
				Destination:     common.String("0.0.0.0/0"),
				NetworkEntityId: s.OCIClusterAccessor.GetNetworkSpec().Vcn.NATGateway.Id,
				Description:     common.String("traffic to the internet"),
			},
			{
				DestinationType: core.RouteRuleDestinationTypeServiceCidrBlock,
				Destination:     common.String(fmt.Sprintf("all-%s-services-in-oracle-services-network", strings.ToLower(s.RegionKey))),
				NetworkEntityId: s.OCIClusterAccessor.GetNetworkSpec().Vcn.ServiceGateway.Id,
				Description:     common.String("traffic to OCI services"),
			},
		}
		vcnPeering := s.OCIClusterAccessor.GetNetworkSpec().VCNPeering
		if vcnPeering != nil {
			for _, routeRule := range vcnPeering.PeerRouteRules {
				routeRules = append(routeRules, core.RouteRule{
					DestinationType: core.RouteRuleDestinationTypeCidrBlock,
					Destination:     common.String(routeRule.VCNCIDRRange),
					NetworkEntityId: s.getDrgID(),
					Description:     common.String("traffic to peer DRG"),
				})
			}
		}

		routeTableName = PrivateRouteTableName
	} else {
		routeRules = []core.RouteRule{
			{
				DestinationType: core.RouteRuleDestinationTypeCidrBlock,
				Destination:     common.String("0.0.0.0/0"),
				NetworkEntityId: s.OCIClusterAccessor.GetNetworkSpec().Vcn.InternetGateway.Id,
				Description:     common.String("traffic to/from internet"),
			},
		}
		resp, err := s.VCNClient.GetVcn(ctx, core.GetVcnRequest{VcnId: s.getVcnId()})
		if err != nil {
			panic(err)
		}
		if resp.Vcn.Ipv6CidrBlocks != nil {
			routeRules = append(routeRules, core.RouteRule{
				DestinationType: core.RouteRuleDestinationTypeCidrBlock,
				Destination:     common.String("::/0"),
				NetworkEntityId: s.OCIClusterAccessor.GetNetworkSpec().Vcn.InternetGateway.Id,
				Description:     common.String("ipv6 traffic to/from internet"),
			})
		}
		routeTableName = PublicRouteTableName
	}
	vcnId := s.getVcnId()
	routeTableDetails := core.CreateRouteTableDetails{
		VcnId:         vcnId,
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(routeTableName),
		RouteRules:    routeRules,
		FreeformTags:  s.GetFreeFormTags(),
		DefinedTags:   s.GetDefinedTags(),
	}
	routeTableResponse, err := s.VCNClient.CreateRouteTable(ctx, core.CreateRouteTableRequest{
		CreateRouteTableDetails: routeTableDetails,
	})
	if err != nil {
		s.Logger.Error(err, "failed create route table")
		return nil, errors.Wrap(err, "failed create route table")
	}
	s.Logger.Info("successfully created the route table", "route-table", *routeTableResponse.Id)
	return routeTableResponse.Id, nil
}

func (s *ClusterScope) setRTStatus(id *string, routeTableType string) {
	if routeTableType == infrastructurev1beta2.Private {
		s.OCIClusterAccessor.GetNetworkSpec().Vcn.RouteTable.PrivateRouteTableId = id
		return
	}
	s.OCIClusterAccessor.GetNetworkSpec().Vcn.RouteTable.PublicRouteTableId = id
}

func (s *ClusterScope) DeleteRouteTables(ctx context.Context) error {
	if s.OCIClusterAccessor.GetNetworkSpec().Vcn.RouteTable.Skip {
		s.Logger.Info("Skipping Route table reconciliation as per spec")
		return nil
	}
	desiredRouteTables := s.GetDesiredRouteTables()
	for _, routeTable := range desiredRouteTables {
		rt, err := s.getRouteTable(ctx, routeTable)
		if err != nil && !ociutil.IsNotFound(err) {
			return err
		}
		if rt == nil {
			s.Logger.Info("Route Table is already deleted", "rt", routeTable)
			continue
		}
		_, err = s.VCNClient.DeleteRouteTable(ctx, core.DeleteRouteTableRequest{
			RtId: rt.Id,
		})
		if err != nil {
			s.Logger.Error(err, "failed to delete route table")
			return errors.Wrap(err, "failed to delete route table")
		}
		s.Logger.Info("successfully deleted route table", "route-table", *rt.Id)
	}
	return nil
}

func (s *ClusterScope) getRouteTableId(routeTableType string) *string {
	if routeTableType == infrastructurev1beta2.Private {
		return s.OCIClusterAccessor.GetNetworkSpec().Vcn.RouteTable.PrivateRouteTableId
	}
	return s.OCIClusterAccessor.GetNetworkSpec().Vcn.RouteTable.PublicRouteTableId
}
