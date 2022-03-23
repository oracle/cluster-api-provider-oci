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
	"reflect"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"

	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/pkg/errors"
)

func (s *ClusterScope) DeleteSecurityLists(ctx context.Context) error {
	desiredSubnets := s.GetSubnetsSpec()
	for _, desiredSubnet := range desiredSubnets {
		if desiredSubnet.SecurityList != nil {
			securityList, err := s.GetSecurityList(ctx, *desiredSubnet.SecurityList)
			if err != nil && !ociutil.IsNotFound(err) {
				return err
			}
			if securityList == nil {
				s.Logger.Info("security list is already deleted", "securityList", desiredSubnet.SecurityList.Name)
				continue
			}
			_, err = s.VCNClient.DeleteSecurityList(ctx, core.DeleteSecurityListRequest{
				SecurityListId: securityList.Id,
			})
			if err != nil {
				s.Logger.Error(err, "failed to delete security list")
				return errors.Wrap(err, "failed to delete security list")
			}
			s.Logger.Info("Successfully deleted security list", "subnet", desiredSubnet.SecurityList.Name)
		}
	}
	return nil
}

func (s *ClusterScope) CreateSecurityList(ctx context.Context, secList infrastructurev1beta1.SecurityList) (*string, error) {
	var ingressRules []core.IngressSecurityRule
	var egressRules []core.EgressSecurityRule
	for _, rule := range secList.EgressRules {
		egressRules = append(egressRules, convertSecurityListEgressRule(rule))
	}
	for _, rule := range secList.IngressRules {
		ingressRules = append(ingressRules, convertSecurityListIngressRule(rule))
	}
	if ingressRules == nil {
		ingressRules = []core.IngressSecurityRule{}
	}
	if egressRules == nil {
		egressRules = []core.EgressSecurityRule{}
	}
	securityListDetails := core.CreateSecurityListDetails{
		VcnId:                s.getVcnId(),
		CompartmentId:        common.String(s.GetCompartmentId()),
		DisplayName:          common.String(secList.Name),
		EgressSecurityRules:  egressRules,
		IngressSecurityRules: ingressRules,
		FreeformTags:         s.GetFreeFormTags(),
		DefinedTags:          s.GetDefinedTags(),
	}
	securityListResponse, err := s.VCNClient.CreateSecurityList(ctx, core.CreateSecurityListRequest{
		CreateSecurityListDetails: securityListDetails,
	})
	if err != nil {
		s.Logger.Error(err, "failed create security list")
		return nil, errors.Wrap(err, "failed create security list")
	}
	s.Logger.Info("successfully created the security list", "security-list", *securityListResponse.Id)
	return securityListResponse.Id, nil
}

func (s *ClusterScope) IsSecurityListEqual(actual core.SecurityList, desired infrastructurev1beta1.SecurityList) bool {
	if *actual.DisplayName != desired.Name {
		return false
	}
	if len(actual.IngressSecurityRules) != len(desired.IngressRules) {
		return false
	}
	if len(actual.EgressSecurityRules) != len(desired.EgressRules) {
		return false
	}
	for _, rule := range desired.IngressRules {
		found := false
		for _, existingRule := range actual.IngressSecurityRules {
			if reflect.DeepEqual(existingRule, convertSecurityListIngressRule(rule)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, rule := range desired.EgressRules {
		found := false
		for _, existingRule := range actual.EgressSecurityRules {
			if reflect.DeepEqual(existingRule, convertSecurityListEgressRule(rule)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return s.IsTagsEqual(actual.FreeformTags, actual.DefinedTags)
}

func (s *ClusterScope) UpdateSecurityList(ctx context.Context, securityListSpec infrastructurev1beta1.SecurityList) error {
	var ingressRules []core.IngressSecurityRule
	var egressRules []core.EgressSecurityRule
	for _, rule := range securityListSpec.EgressRules {
		egressRules = append(egressRules, convertSecurityListEgressRule(rule))
	}
	for _, rule := range securityListSpec.IngressRules {
		ingressRules = append(ingressRules, convertSecurityListIngressRule(rule))
	}
	if ingressRules == nil {
		ingressRules = []core.IngressSecurityRule{}
	}
	if egressRules == nil {
		egressRules = []core.EgressSecurityRule{}
	}
	updateSecurityListDetails := core.UpdateSecurityListDetails{
		DisplayName:          common.String(securityListSpec.Name),
		EgressSecurityRules:  egressRules,
		IngressSecurityRules: ingressRules,
		FreeformTags:         s.GetFreeFormTags(),
		DefinedTags:          s.GetDefinedTags(),
	}
	securityListResponse, err := s.VCNClient.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
		UpdateSecurityListDetails: updateSecurityListDetails,
		SecurityListId:            common.String(*securityListSpec.ID),
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the security list, failed to update")
		return errors.Wrap(err, "failed to reconcile the security list, failed to update")
	}
	s.Logger.Info("successfully updated the security list", "security list", *securityListResponse.Id)
	return nil
}

func convertSecurityListIngressRule(rule infrastructurev1beta1.IngressSecurityRule) core.IngressSecurityRule {
	var icmpOptions *core.IcmpOptions
	var tcpOptions *core.TcpOptions
	var udpOptions *core.UdpOptions
	icmpOptions, tcpOptions, udpOptions = getProtocolOptions(rule.IcmpOptions, rule.TcpOptions, rule.UdpOptions)

	// while comparing values, the boolean value has to be always set
	stateless := rule.IsStateless
	if stateless == nil {
		stateless = common.Bool(false)
	}
	return core.IngressSecurityRule{
		Protocol:    rule.Protocol,
		Source:      rule.Source,
		IcmpOptions: icmpOptions,
		IsStateless: stateless,
		SourceType:  core.IngressSecurityRuleSourceTypeEnum(rule.SourceType),
		TcpOptions:  tcpOptions,
		UdpOptions:  udpOptions,
		Description: rule.Description,
	}
}

func convertSecurityListEgressRule(rule infrastructurev1beta1.EgressSecurityRule) core.EgressSecurityRule {
	var icmpOptions *core.IcmpOptions
	var tcpOptions *core.TcpOptions
	var udpOptions *core.UdpOptions

	// while comparing values, the boolean value has to be always set
	stateless := rule.IsStateless
	if stateless == nil {
		stateless = common.Bool(false)
	}
	icmpOptions, tcpOptions, udpOptions = getProtocolOptions(rule.IcmpOptions, rule.TcpOptions, rule.UdpOptions)
	return core.EgressSecurityRule{
		Protocol:        rule.Protocol,
		Destination:     rule.Destination,
		IcmpOptions:     icmpOptions,
		IsStateless:     stateless,
		DestinationType: core.EgressSecurityRuleDestinationTypeEnum(rule.DestinationType),
		TcpOptions:      tcpOptions,
		UdpOptions:      udpOptions,
		Description:     rule.Description,
	}
}

func getProtocolOptions(icmp *infrastructurev1beta1.IcmpOptions, tcp *infrastructurev1beta1.TcpOptions,
	udp *infrastructurev1beta1.UdpOptions) (*core.IcmpOptions, *core.TcpOptions, *core.UdpOptions) {
	var icmpOptions *core.IcmpOptions
	var tcpOptions *core.TcpOptions
	var udpOptions *core.UdpOptions
	if icmp != nil {
		icmpOptions = &core.IcmpOptions{
			Type: icmp.Type,
			Code: icmp.Code,
		}
	}
	if tcp != nil {
		tcpOptions = &core.TcpOptions{}
		if tcp.DestinationPortRange != nil {
			tcpOptions.DestinationPortRange = &core.PortRange{}
			tcpOptions.DestinationPortRange.Max = tcp.DestinationPortRange.Max
			tcpOptions.DestinationPortRange.Min = tcp.DestinationPortRange.Min
		}
		if tcp.SourcePortRange != nil {
			tcpOptions.SourcePortRange = &core.PortRange{}
			tcpOptions.SourcePortRange.Max = tcp.SourcePortRange.Max
			tcpOptions.SourcePortRange.Min = tcp.SourcePortRange.Min
		}
	}
	if udp != nil {
		udpOptions = &core.UdpOptions{}
		if udp.DestinationPortRange != nil {
			udpOptions.DestinationPortRange = &core.PortRange{}
			udpOptions.DestinationPortRange.Max = udp.DestinationPortRange.Max
			udpOptions.DestinationPortRange.Min = udp.DestinationPortRange.Min
		}
		if udp.SourcePortRange != nil {
			udpOptions.SourcePortRange = &core.PortRange{}
			udpOptions.SourcePortRange.Max = udp.SourcePortRange.Max
			udpOptions.SourcePortRange.Min = udp.SourcePortRange.Min
		}
	}
	return icmpOptions, tcpOptions, udpOptions
}

func (s *ClusterScope) IsSecurityListExitsByRole(role infrastructurev1beta1.Role) bool {
	for _, subnet := range s.GetSubnetsSpec() {
		if role == subnet.Role {
			if subnet.SecurityList != nil {
				return true
			}
		}
	}
	return false
}

func (s *ClusterScope) GetSecurityList(ctx context.Context, spec infrastructurev1beta1.SecurityList) (*core.SecurityList, error) {
	securityListOcid := spec.ID
	if securityListOcid != nil {
		resp, err := s.VCNClient.GetSecurityList(ctx, core.GetSecurityListRequest{
			SecurityListId: securityListOcid,
		})
		if err != nil {
			return nil, err
		}
		securityList := resp.SecurityList
		if s.IsResourceCreatedByClusterAPI(securityList.FreeformTags) {
			return &securityList, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	securityLists, err := s.VCNClient.ListSecurityLists(ctx, core.ListSecurityListsRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		VcnId:         s.getVcnId(),
		DisplayName:   common.String(spec.Name),
	})
	if err != nil {
		s.Logger.Error(err, "failed to list security lists")
		return nil, errors.Wrap(err, "failed to list security lists")
	}
	for _, securityList := range securityLists.Items {
		if s.IsResourceCreatedByClusterAPI(securityList.FreeformTags) {
			return &securityList, nil
		}
	}
	return nil, nil
}
