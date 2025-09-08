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
	"net"
	"strings"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

func (s *ClusterScope) ReconcileSubnet(ctx context.Context) error {
	desiredSubnets := s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets
	for _, desiredSubnet := range desiredSubnets {
		if desiredSubnet.Skip {
			s.Logger.Info("Skipping Subnet reconciliation as per spec")
			continue
		}
		subnet, err := s.GetSubnet(ctx, *desiredSubnet)
		if err != nil {
			return err
		}
		if subnet != nil {
			subnetOCID := subnet.Id
			desiredSubnet.ID = subnetOCID
			if desiredSubnet.SecurityList != nil {
				securityList, err := s.GetSecurityList(ctx, *desiredSubnet.SecurityList)
				if err != nil {
					return err
				}
				if securityList == nil {
					seclistId, err := s.CreateSecurityList(ctx, *desiredSubnet.SecurityList)
					if err != nil {
						return err
					}
					s.Logger.Info("Created the security list", "ocid", seclistId)
					desiredSubnet.SecurityList.ID = seclistId
				} else {
					if s.IsSecurityListEqual(*securityList, *desiredSubnet.SecurityList) {
						s.Logger.Info("No Reconciliation Required for Security List", "securitylist", securityList.Id)
					} else {
						err = s.UpdateSecurityList(ctx, *desiredSubnet.SecurityList)
						if err != nil {
							return err
						}
						s.Logger.Info("Successfully updated security list", "securitylist", securityList)
					}
				}
			}
			if s.IsSubnetsEqual(subnet, *desiredSubnet) {
				s.Logger.Info("No Reconciliation Required for Subnet", "subnet", subnetOCID)
			} else {
				err = s.UpdateSubnet(ctx, *desiredSubnet)
				if err != nil {
					return err
				}
				s.Logger.Info("Successfully updated subnet", "subnet", subnetOCID)
			}
			continue
		}
		s.Logger.Info("Creating the subnet")
		if desiredSubnet.SecurityList != nil {
			s.Logger.Info("Creating the security List")
			seclistId, err := s.CreateSecurityList(ctx, *desiredSubnet.SecurityList)
			if err != nil {
				return err
			}
			s.Logger.Info("Created the security list", "ocid", seclistId)
			desiredSubnet.SecurityList.ID = seclistId
		}
		subnetId, err := s.CreateSubnet(ctx, *desiredSubnet)
		if err != nil {
			return err
		}
		s.Logger.Info("Created the subnet", "ocid", subnetId)
		desiredSubnet.ID = subnetId
	}
	return nil
}

func (s *ClusterScope) CreateSubnet(ctx context.Context, spec infrastructurev1beta2.Subnet) (*string, error) {
	var err error
	var routeTable *string
	var isPrivate bool
	if spec.Type == infrastructurev1beta2.Private {
		isPrivate = true
		routeTable = s.getRouteTableId(infrastructurev1beta2.Private)
	} else {
		routeTable = s.getRouteTableId(infrastructurev1beta2.Public)
	}

	resp, err := s.VCNClient.GetVcn(ctx, core.GetVcnRequest{VcnId: s.getVcnId()})

	var ipv6subnetCIDR_Ptr *string

	// Constructing IPv6 Subnet CIDR
	if resp.Vcn.Ipv6CidrBlocks != nil {

		// VCNs can have multiple IPv6 CIDR Blocks, and the CIDR block with IPv6 GUA Allocated by Oracle is the first (index 0) in the list
		vcnCIDR := resp.Vcn.Ipv6CidrBlocks[0]

		// Split CIDR block into hextets
		ip, _, err := net.ParseCIDR(vcnCIDR)
		if err != nil {
			panic(err)
		}
		hextets := strings.Split(ip.String(), ":")

		// Modify the 4th hextet (index 3) of vcn CIDR to reflect the subnet CIDR with Ipv6CidrBlockHextet value in it
		if len(hextets) >= 4 {
			originalHextet := hextets[3]
			if len(originalHextet) < 4 {
				originalHextet = fmt.Sprintf("%04s", originalHextet)
			}
			newHextet := originalHextet[:2] + *spec.Ipv6CidrBlockHextet
			hextets[3] = newHextet

			// Reconstruct the IPv6 address with a /64 CIDR for the subnet
			ipv6subnetCIDR := strings.Join(hextets, ":") + "/64"
			ipv6subnetCIDR_Ptr = &ipv6subnetCIDR
		}
	}

	createSubnetDetails := core.CreateSubnetDetails{
		CompartmentId:           common.String(s.GetCompartmentId()),
		CidrBlock:               common.String(spec.CIDR),
		VcnId:                   s.getVcnId(),
		DisplayName:             common.String(spec.Name),
		ProhibitInternetIngress: common.Bool(isPrivate),
		ProhibitPublicIpOnVnic:  common.Bool(isPrivate),
		RouteTableId:            routeTable,
		FreeformTags:            s.GetFreeFormTags(),
		DefinedTags:             s.GetDefinedTags(),
		DnsLabel:                spec.DnsLabel,
		Ipv6CidrBlock:           ipv6subnetCIDR_Ptr,
	}

	if spec.SecurityList != nil {
		createSubnetDetails.SecurityListIds = []string{*spec.SecurityList.ID}
	}
	subnetResponse, err := s.VCNClient.CreateSubnet(ctx, core.CreateSubnetRequest{
		CreateSubnetDetails: createSubnetDetails,
	})
	if err != nil {
		s.Logger.Error(err, "failed create subnet")
		return nil, errors.Wrap(err, "failed create subnet")
	}
	s.Logger.Info("Successfully created the subnet", "subnet", *subnetResponse.Subnet.Id)
	return subnetResponse.Subnet.Id, nil
}

func (s *ClusterScope) UpdateSubnet(ctx context.Context, spec infrastructurev1beta2.Subnet) error {
	updateSubnetDetails := core.UpdateSubnetDetails{
		DisplayName: common.String(spec.Name),
		CidrBlock:   common.String(spec.CIDR),
	}
	if spec.SecurityList != nil {
		updateSubnetDetails.SecurityListIds = []string{*spec.SecurityList.ID}
	}
	subnetResponse, err := s.VCNClient.UpdateSubnet(ctx, core.UpdateSubnetRequest{
		UpdateSubnetDetails: updateSubnetDetails,
		SubnetId:            spec.ID,
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the subnet, failed to update")
		return errors.Wrap(err, "failed to reconcile the subnet, failed to update")
	}
	s.Logger.Info("Successfully updated the subnet", "subnet", *subnetResponse.Id)
	return nil
}

func (s *ClusterScope) DeleteSubnets(ctx context.Context) error {
	desiredSubnets := s.GetSubnetsSpec()
	for _, desiredSubnet := range desiredSubnets {
		if desiredSubnet.Skip {
			s.Logger.Info("Skipping Subnet reconciliation as per spec")
			continue
		}
		subnet, err := s.GetSubnet(ctx, *desiredSubnet)
		if err != nil && !ociutil.IsNotFound(err) {
			return err
		}
		if subnet == nil {
			s.Logger.Info("subnet is already deleted", "subnet", desiredSubnet.Name)
			continue
		}
		_, err = s.VCNClient.DeleteSubnet(ctx, core.DeleteSubnetRequest{
			SubnetId: subnet.Id,
		})
		if err != nil {
			s.Logger.Error(err, "failed to delete subnet")
			return errors.Wrap(err, "failed to delete subnet")
		}
		s.Logger.Info("Successfully deleted subnet", "subnet", desiredSubnet.Name)
	}
	return nil
}

func (s *ClusterScope) GetSubnet(ctx context.Context, spec infrastructurev1beta2.Subnet) (*core.Subnet, error) {
	var err error
	subnetOcid := spec.ID
	if subnetOcid != nil {
		resp, err := s.VCNClient.GetSubnet(ctx, core.GetSubnetRequest{
			SubnetId: subnetOcid,
		})
		if err != nil {
			return nil, err
		}
		subnet := resp.Subnet
		if s.IsResourceCreatedByClusterAPI(subnet.FreeformTags) {
			return &subnet, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	subnets, err := s.VCNClient.ListSubnets(ctx, core.ListSubnetsRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		VcnId:         s.getVcnId(),
		DisplayName:   common.String(spec.Name),
	})
	if err != nil {
		s.Logger.Error(err, "failed to list subnets")
		return nil, errors.Wrap(err, "failed to list subnets")
	}
	for _, subnet := range subnets.Items {
		if s.IsResourceCreatedByClusterAPI(subnet.FreeformTags) {
			return &subnet, nil
		}
	}
	return nil, err
}

func (s *ClusterScope) GetControlPlaneEndpointSubnet() *infrastructurev1beta2.Subnet {
	for _, subnet := range s.GetSubnetsSpec() {
		if subnet != nil && subnet.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			return subnet
		}
	}
	return nil
}

func (s *ClusterScope) GetControlPlaneMachineSubnet() *infrastructurev1beta2.Subnet {
	for _, subnet := range s.GetSubnetsSpec() {
		if subnet != nil && subnet.Role == infrastructurev1beta2.ControlPlaneRole {
			return subnet
		}
	}
	return nil
}

func (s *ClusterScope) GetServiceLoadBalancerSubnet() *infrastructurev1beta2.Subnet {
	for _, subnet := range s.GetSubnetsSpec() {
		if subnet != nil && subnet.Role == infrastructurev1beta2.ServiceLoadBalancerRole {
			return subnet
		}
	}
	return nil
}

func (s *ClusterScope) GetNodeSubnet() []*infrastructurev1beta2.Subnet {
	var nodeSubnets []*infrastructurev1beta2.Subnet
	for _, subnet := range s.GetSubnetsSpec() {
		if subnet != nil && subnet.Role == infrastructurev1beta2.WorkerRole {
			nodeSubnets = append(nodeSubnets, subnet)
		}
	}
	return nodeSubnets
}

func (s *ClusterScope) GetSubnetsSpec() []*infrastructurev1beta2.Subnet {
	return s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets
}

func (s *ClusterScope) IsSubnetsEqual(actual *core.Subnet, desired infrastructurev1beta2.Subnet) bool {
	if desired.Name != *actual.DisplayName {
		return false
	}
	if desired.CIDR != *actual.CidrBlock {
		return false
	}
	if desired.SecurityList != nil {
		if *desired.SecurityList.ID != actual.SecurityListIds[0] {
			return false
		}
	}
	return true
}

func (s *ClusterScope) isControlPlaneEndpointSubnetPrivate() bool {
	for _, subnet := range s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets {
		if subnet != nil && subnet.Role == infrastructurev1beta2.ControlPlaneEndpointRole && subnet.Type == infrastructurev1beta2.Private {
			return true
		}
	}
	return false
}

func (s *ClusterScope) GetControlPlaneEndpointSubnetCidr() string {
	for _, subnet := range s.GetSubnetsSpec() {
		if subnet != nil && subnet.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			if subnet.CIDR != "" {
				return subnet.CIDR
			}
		}
	}
	return ControlPlaneEndpointSubnetDefaultCIDR
}

func (s *ClusterScope) GetServiceLoadBalancerSubnetCidr() string {
	for _, subnet := range s.GetSubnetsSpec() {
		if subnet != nil && subnet.Role == infrastructurev1beta2.ServiceLoadBalancerRole {
			if subnet.CIDR != "" {
				return subnet.CIDR
			}
		}
	}
	return ServiceLoadBalancerDefaultCIDR
}

func (s *ClusterScope) NodeSubnetCidr() []string {
	subnets := s.GetNodeSubnet()
	var nodeCIDR []string
	for _, subnet := range subnets {
		if subnet != nil && subnet.CIDR != "" {
			nodeCIDR = append(nodeCIDR, subnet.CIDR)
		}
		nodeCIDR = append(nodeCIDR, WorkerSubnetDefaultCIDR)
	}
	if len(nodeCIDR) == 0 {
		return []string{WorkerSubnetDefaultCIDR}
	}
	return nodeCIDR
}
func (s *ClusterScope) GetControlPlaneMachineSubnetCidr() string {
	subnet := s.GetControlPlaneMachineSubnet()
	if subnet != nil {
		if subnet.CIDR != "" {
			return subnet.CIDR
		}
	}
	return ControlPlaneMachineSubnetDefaultCIDR
}

// IsAllSubnetsPrivate determines if all the ClusterScope's subnets are private
func (s *ClusterScope) IsAllSubnetsPrivate() bool {
	for _, subnet := range s.GetSubnetsSpec() {
		if subnet.Type == infrastructurev1beta2.Public {
			return false
		}
	}
	if (s.GetControlPlaneEndpointSubnet() == nil) || (s.GetServiceLoadBalancerSubnet() == nil) {
		return false
	}
	return true
}

// IsAllSubnetsPublic determines if all the ClusterScope's subnets are public
func (s *ClusterScope) IsAllSubnetsPublic() bool {
	for _, subnet := range s.GetSubnetsSpec() {
		if subnet != nil && subnet.Type == infrastructurev1beta2.Private {
			return false
		}
	}
	if (s.GetControlPlaneMachineSubnet() == nil) || (len(s.GetNodeSubnet()) == 0) {
		return false
	}
	return true
}
