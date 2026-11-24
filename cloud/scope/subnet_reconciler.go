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
	"net/netip"
	"slices"
	"strconv"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil/ptr"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

func (s *ClusterScope) ReconcileSubnet(ctx context.Context) error {
	desiredSubnets := s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets
	for _, desiredSubnet := range desiredSubnets {
		if desiredSubnet == nil {
			return errors.New("Skipping Subnet reconciliation: Subnet can't be nil")
		}
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
	var routeTable *string
	var isPrivate bool
	if spec.Type == infrastructurev1beta2.Private {
		isPrivate = true
		routeTable = s.getRouteTableId(infrastructurev1beta2.Private)
	} else {
		routeTable = s.getRouteTableId(infrastructurev1beta2.Public)
	}

	var ipv6CIDR *string
	if spec.Ipv6CidrBlockHextet != nil {
		resp, err := s.VCNClient.GetVcn(ctx, core.GetVcnRequest{VcnId: s.getVcnId()})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get VCN for IPv6 subnet creation")
		}
		if len(resp.Vcn.Ipv6CidrBlocks) > 0 {
			// OCI lists the Oracle-assigned IPv6 prefix first.
			cidr, err := ipv6SubnetCIDR(resp.Vcn.Ipv6CidrBlocks[0], *spec.Ipv6CidrBlockHextet)
			if err != nil {
				return nil, err
			}
			ipv6CIDR = common.String(cidr)
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
		Ipv6CidrBlock:           ipv6CIDR,
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

func ipv6SubnetCIDR(vcnCIDR, hextet string) (string, error) {
	prefix, err := netip.ParsePrefix(vcnCIDR)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse IPv6 CIDR block")
	}
	if !prefix.Addr().Is6() || prefix.Bits() != 56 {
		return "", fmt.Errorf("expected an IPv6 /56 VCN prefix, got %q", vcnCIDR)
	}
	subnetID, err := strconv.ParseUint(hextet, 16, 8)
	if err != nil {
		return "", errors.Wrapf(err, "invalid IPv6 subnet hextet %q", hextet)
	}
	// A /64 subnet of a /56 VCN keeps the first seven bytes and sets the eighth.
	addr := prefix.Masked().Addr().As16()
	addr[7] = byte(subnetID)
	return netip.PrefixFrom(netip.AddrFrom16(addr), 64).String(), nil
}

func (s *ClusterScope) UpdateSubnet(ctx context.Context, spec infrastructurev1beta2.Subnet) error {
	updateSubnetDetails := core.UpdateSubnetDetails{
		DisplayName: common.String(spec.Name),
		CidrBlock:   common.String(spec.CIDR),
	}
	if spec.SecurityList != nil {
		if spec.SecurityList.ID == nil {
			err := errors.New("SecurityListID required")
			s.Logger.Error(err, "failed updating subnet")
			return err
		}
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

// nolint:nilnil
func (s *ClusterScope) GetSubnet(ctx context.Context, spec infrastructurev1beta2.Subnet) (*core.Subnet, error) {
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
	return nil, nil
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
	if desired.Name != ptr.ToString(actual.DisplayName) {
		return false
	}
	if desired.CIDR != ptr.ToString(actual.CidrBlock) {
		return false
	}
	if desired.SecurityList != nil {
		if desired.SecurityList.ID == nil {
			return false
		}

		return slices.Contains(actual.SecurityListIds, ptr.ToString(desired.SecurityList.ID))
	}
	return true
}

// isControlPlaneEndpointSubnetPrivate is the legacy fallback used by isAPIServerLBPrivate when
// networkVisibility is empty or Inherited.
func (s *ClusterScope) isControlPlaneEndpointSubnetPrivate() bool {
	for _, subnet := range ptr.ToSubnetSlice(s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets) {
		if subnet.Role == infrastructurev1beta2.ControlPlaneEndpointRole && subnet.Type == infrastructurev1beta2.Private {
			return true
		}
	}
	return false
}

func (s *ClusterScope) isAPIServerLBPrivate() bool {
	visibility := s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.NetworkVisibility
	if visibility == infrastructurev1beta2.LBNetworkVisibilityPrivate {
		return true
	}
	if visibility == infrastructurev1beta2.LBNetworkVisibilityPublic {
		return false
	}
	// Empty string or Inherited → fall back to subnet derivation
	return s.isControlPlaneEndpointSubnetPrivate()
}

func (s *ClusterScope) GetControlPlaneEndpointSubnetCidr() string {
	subnetSlice := ptr.ToSubnetSlice(s.GetSubnetsSpec())
	for i := range subnetSlice {
		subnet := &subnetSlice[i]
		if subnet.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			if subnet.CIDR != "" {
				return subnet.CIDR
			}
		}
	}
	return ControlPlaneEndpointSubnetDefaultCIDR
}

func (s *ClusterScope) GetServiceLoadBalancerSubnetCidr() string {
	subnetSlice := ptr.ToSubnetSlice(s.GetSubnetsSpec())
	for i := range subnetSlice {
		subnet := &subnetSlice[i]
		if subnet.Role == infrastructurev1beta2.ServiceLoadBalancerRole {
			if subnet.CIDR != "" {
				return subnet.CIDR
			}
		}
	}
	return ServiceLoadBalancerDefaultCIDR
}

func (s *ClusterScope) NodeSubnetCidr() []string {
	var nodeCIDR []string
	subnetSlice := ptr.ToSubnetSlice(s.GetNodeSubnet())
	for i := range subnetSlice {
		subnet := &subnetSlice[i]
		if subnet.CIDR != "" {
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
	subnetSlice := ptr.ToSubnetSlice(s.GetSubnetsSpec())
	for i := range subnetSlice {
		subnet := &subnetSlice[i]
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
	for _, subnet := range ptr.ToSubnetSlice(s.GetSubnetsSpec()) {
		if subnet.Type == infrastructurev1beta2.Private {
			return false
		}
	}
	if (s.GetControlPlaneMachineSubnet() == nil) || (len(s.GetNodeSubnet()) == 0) {
		return false
	}
	return true
}
