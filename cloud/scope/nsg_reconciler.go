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
	"reflect"
	"strings"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil/ptr"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

func (s *ClusterScope) ReconcileNSG(ctx context.Context) error {
	if s.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.Skip {
		s.Logger.Info("Skipping Network Security Group reconciliation as per spec")
		return nil
	}
	desiredNSGs := s.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup
	for _, desiredNSG := range desiredNSGs.List {
		if desiredNSG == nil {
			s.Logger.Info("Skipping nil NSG pointer in spec")
			continue
		}
		desiredNSGPtr := *desiredNSG
		nsg, err := s.GetNSG(ctx, desiredNSGPtr)
		if err != nil {
			s.Logger.Error(err, "error to get nsg")
			return err
		}
		if nsg != nil {
			nsgOCID := nsg.Id
			desiredNSG.ID = nsgOCID
			if !s.IsNSGEqual(nsg, desiredNSGPtr) {
				err = s.UpdateNSG(ctx, desiredNSGPtr)
				if err != nil {
					return err
				}
				s.Logger.Info("Successfully updated network security list", "nsg", nsgOCID)
			}
			continue
		}
		s.Logger.Info("Creating the network security list")
		nsgID, err := s.CreateNSG(ctx, desiredNSGPtr)
		if err != nil {
			return err
		}
		s.Logger.Info("Created the nsg", "nsg", nsgID)
		desiredNSG.ID = nsgID

	}
	for _, desiredNSG := range desiredNSGs.List {
		if desiredNSG == nil {
			s.Logger.Info("Skipping nil NSG pointer in spec")
			continue
		}
		s.adjustNSGRulesSpec(desiredNSG, desiredNSGs.List)
		isNSGUpdated, err := s.UpdateNSGSecurityRulesIfNeeded(ctx, *desiredNSG, desiredNSG.ID)
		if err != nil {
			return err
		}
		if !isNSGUpdated {
			s.Logger.Info("No Reconciliation Required for Network Security Group", "nsg", *desiredNSG.ID)
		}
	}
	return nil
}

func (s *ClusterScope) adjustNSGRulesSpec(desiredNSG *infrastructurev1beta2.NSG, nsgList []*infrastructurev1beta2.NSG) {
	ingressRules := make([]infrastructurev1beta2.IngressSecurityRuleForNSG, 0)
	for _, ingressRule := range desiredNSG.IngressRules {
		if ingressRule.SourceType == infrastructurev1beta2.IngressSecurityRuleSourceTypeServiceCidrBlock {
			ingressRule.Source = common.String(fmt.Sprintf("all-%s-services-in-oracle-services-network", strings.ToLower(s.RegionKey)))
		}
		ingressRules = append(ingressRules, ingressRule)
	}
	desiredNSG.IngressRules = ingressRules
	egressRules := make([]infrastructurev1beta2.EgressSecurityRuleForNSG, 0)
	for _, egressRule := range desiredNSG.EgressRules {
		if egressRule.DestinationType == infrastructurev1beta2.EgressSecurityRuleDestinationTypeServiceCidrBlock {
			egressRule.Destination = common.String(fmt.Sprintf("all-%s-services-in-oracle-services-network", strings.ToLower(s.RegionKey)))
		}
		egressRules = append(egressRules, egressRule)
	}
	desiredNSG.EgressRules = egressRules
}

// nolint:nilnil
func (s *ClusterScope) GetNSG(ctx context.Context, spec infrastructurev1beta2.NSG) (*core.NetworkSecurityGroup, error) {
	nsgOCID := spec.ID
	if nsgOCID != nil {
		resp, err := s.VCNClient.GetNetworkSecurityGroup(ctx, core.GetNetworkSecurityGroupRequest{
			NetworkSecurityGroupId: nsgOCID,
		})
		if err != nil {
			return nil, err
		}
		nsg := resp.NetworkSecurityGroup
		if s.IsResourceCreatedByClusterAPI(nsg.FreeformTags) {
			return &nsg, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	nsgs, err := s.VCNClient.ListNetworkSecurityGroups(ctx, core.ListNetworkSecurityGroupsRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		VcnId:         s.getVcnId(),
		DisplayName:   common.String(spec.Name),
	})
	if err != nil {
		s.Logger.Error(err, "failed to list network security groups")
		return nil, errors.Wrap(err, "failed to list network security groups")
	}
	for _, nsg := range nsgs.Items {
		if s.IsResourceCreatedByClusterAPI(nsg.FreeformTags) {
			return &nsg, nil
		}
	}
	return nil, nil
}

func (s *ClusterScope) DeleteNSGs(ctx context.Context) error {
	if s.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.Skip {
		s.Logger.Info("Skipping Network Security Group reconciliation as per spec")
		return nil
	}
	desiredNSGs := s.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup
	for _, desiredNSG := range desiredNSGs.List {
		nsg, err := s.GetNSG(ctx, *desiredNSG)
		if err != nil && !ociutil.IsNotFound(err) {
			return err
		}
		if nsg == nil {
			s.Logger.Info("nsg is already deleted", "nsg", desiredNSG.Name)
			continue
		}
		_, err = s.VCNClient.DeleteNetworkSecurityGroup(ctx, core.DeleteNetworkSecurityGroupRequest{
			NetworkSecurityGroupId: nsg.Id,
		})
		if err != nil {
			s.Logger.Error(err, "failed to delete nsg")
			return errors.Wrap(err, "failed to delete nsg")
		}
		s.Logger.Info("Successfully deleted nsg", "subnet", desiredNSG.Name)
	}
	return nil
}

func (s *ClusterScope) GetNSGSpec() []*infrastructurev1beta2.NSG {
	return s.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List
}

func (s *ClusterScope) IsNSGExitsByRole(role infrastructurev1beta2.Role) bool {
	for _, nsg := range s.GetNSGSpec() {
		if role == nsg.Role {
			return true
		}
	}
	return false
}

// IsNSGEqual compares the actual and desired NSG using name.
func (s *ClusterScope) IsNSGEqual(actual *core.NetworkSecurityGroup, desired infrastructurev1beta2.NSG) bool {
	if *actual.DisplayName != desired.Name {
		return false
	}
	return true
}

// UpdateNSGSecurityRulesIfNeeded updates NSG rules if required by comparing actual and desired.
func (s *ClusterScope) UpdateNSGSecurityRulesIfNeeded(ctx context.Context, desired infrastructurev1beta2.NSG,
	nsgId *string) (bool, error) {
	var ingressRulesToAdd []infrastructurev1beta2.IngressSecurityRuleForNSG
	var egressRulesToAdd []infrastructurev1beta2.EgressSecurityRuleForNSG
	var securityRulesToRemove []string
	var isNSGUpdated bool
	listSecurityRulesResponse, err := s.VCNClient.ListNetworkSecurityGroupSecurityRules(ctx, core.ListNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: nsgId,
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the network security group, failed to list security rules")
		return isNSGUpdated, errors.Wrap(err, "failed to reconcile the network security group, failed to list security rules")
	}
	ingressRules, egressRules := s.generateSpecFromSecurityRules(listSecurityRulesResponse.Items, nsgId)

	for i, ingressRule := range desired.IngressRules {
		if ingressRule.IsStateless == nil {
			desired.IngressRules[i].IsStateless = common.Bool(false)
		}
	}
	for i, egressRule := range desired.EgressRules {
		if egressRule.IsStateless == nil {
			desired.EgressRules[i].IsStateless = common.Bool(false)
		}
	}

	for _, desiredRule := range desired.IngressRules {
		found := false
		for _, actualRule := range ingressRules {
			if reflect.DeepEqual(desiredRule, actualRule) {
				found = true
				break
			}
		}
		if !found {
			ingressRulesToAdd = append(ingressRulesToAdd, desiredRule)
		}
	}

	for id, actualRule := range ingressRules {
		found := false
		for _, desiredRule := range desired.IngressRules {
			if reflect.DeepEqual(desiredRule, actualRule) {
				found = true
				break
			}
		}
		if !found {
			securityRulesToRemove = append(securityRulesToRemove, id)
		}
	}

	for _, desiredRule := range desired.EgressRules {
		found := false
		for _, actualRule := range egressRules {
			if reflect.DeepEqual(desiredRule, actualRule) {
				found = true
				break
			}
		}
		if !found {
			egressRulesToAdd = append(egressRulesToAdd, desiredRule)
		}
	}

	for id, actualRule := range egressRules {
		found := false
		for _, desiredRule := range desired.EgressRules {
			if reflect.DeepEqual(desiredRule, actualRule) {
				found = true
				break
			}
		}
		if !found {
			securityRulesToRemove = append(securityRulesToRemove, id)
		}
	}

	if len(ingressRulesToAdd) > 0 || len(egressRulesToAdd) > 0 {
		isNSGUpdated = true
		err := s.AddNSGSecurityRules(ctx, desired.ID, ingressRulesToAdd, egressRulesToAdd)
		if err != nil {
			s.Logger.Error(err, "failed to reconcile the network security group, failed to add security rules")
			return isNSGUpdated, err
		}
		s.Logger.Info("Successfully added missing rules in NSG", "nsg", *nsgId)
	}
	if len(securityRulesToRemove) > 0 {
		isNSGUpdated = true
		_, err = s.VCNClient.RemoveNetworkSecurityGroupSecurityRules(ctx, core.RemoveNetworkSecurityGroupSecurityRulesRequest{
			NetworkSecurityGroupId: desired.ID,
			RemoveNetworkSecurityGroupSecurityRulesDetails: core.RemoveNetworkSecurityGroupSecurityRulesDetails{
				SecurityRuleIds: securityRulesToRemove,
			},
		})
		if err != nil {
			s.Logger.Error(err, "failed to reconcile the network security group, failed to remove security rules")
			return isNSGUpdated, err
		}
		s.Logger.Info("Successfully deleted rules in NSG", "nsg", *nsgId)
	}
	return isNSGUpdated, nil
}

func (s *ClusterScope) UpdateNSG(ctx context.Context, nsgSpec infrastructurev1beta2.NSG) error {
	updateNSGDetails := core.UpdateNetworkSecurityGroupDetails{
		DisplayName: common.String(nsgSpec.Name),
	}
	nsgResponse, err := s.VCNClient.UpdateNetworkSecurityGroup(ctx, core.UpdateNetworkSecurityGroupRequest{
		NetworkSecurityGroupId:            nsgSpec.ID,
		UpdateNetworkSecurityGroupDetails: updateNSGDetails,
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the network security group, failed to update")
		return errors.Wrap(err, "failed to reconcile the network security group, failed to update")
	}
	s.Logger.Info("successfully updated the network security group", "network security group", *nsgResponse.Id)
	return nil
}

func (s *ClusterScope) generateAddSecurityRuleFromSpec(ingressRules []infrastructurev1beta2.IngressSecurityRuleForNSG,
	egressRules []infrastructurev1beta2.EgressSecurityRuleForNSG) []core.AddSecurityRuleDetails {
	var securityRules []core.AddSecurityRuleDetails
	var icmpOptions *core.IcmpOptions
	var tcpOptions *core.TcpOptions
	var udpOptions *core.UdpOptions
	var stateless *bool
	for _, ingressRule := range ingressRules {
		icmpOptions, tcpOptions, udpOptions = getProtocolOptions(ingressRule.IcmpOptions, ingressRule.TcpOptions, ingressRule.UdpOptions)
		// while comparing values, the boolean value has to be always set
		stateless = ingressRule.IsStateless
		if stateless == nil {
			stateless = common.Bool(false)
		}
		secRule := core.AddSecurityRuleDetails{
			Direction:   core.AddSecurityRuleDetailsDirectionIngress,
			Protocol:    ingressRule.Protocol,
			Description: ingressRule.Description,
			IcmpOptions: icmpOptions,
			IsStateless: stateless,
			Source:      ingressRule.Source,
			SourceType:  core.AddSecurityRuleDetailsSourceTypeEnum(ingressRule.SourceType),
			TcpOptions:  tcpOptions,
			UdpOptions:  udpOptions,
		}
		if ingressRule.SourceType == infrastructurev1beta2.IngressSecurityRuleSourceTypeNSG {
			secRule.Source = getNsgIdFromName(ingressRule.Source, s.GetNSGSpec())
		}
		securityRules = append(securityRules, secRule)
	}
	for _, egressRule := range egressRules {
		icmpOptions, tcpOptions, udpOptions = getProtocolOptions(egressRule.IcmpOptions, egressRule.TcpOptions, egressRule.UdpOptions)
		// while comparing values, the boolean value has to be always set
		stateless = egressRule.IsStateless
		if stateless == nil {
			stateless = common.Bool(false)
		}
		secRule := core.AddSecurityRuleDetails{
			Direction:       core.AddSecurityRuleDetailsDirectionEgress,
			Protocol:        egressRule.Protocol,
			Description:     egressRule.Description,
			IcmpOptions:     icmpOptions,
			IsStateless:     stateless,
			Destination:     egressRule.Destination,
			DestinationType: core.AddSecurityRuleDetailsDestinationTypeEnum(egressRule.DestinationType),
			TcpOptions:      tcpOptions,
			UdpOptions:      udpOptions,
		}
		if egressRule.DestinationType == infrastructurev1beta2.EgressSecurityRuleDestinationTypeNSG {
			secRule.Destination = getNsgIdFromName(egressRule.Destination, s.GetNSGSpec())
		}
		securityRules = append(securityRules, secRule)
	}
	return securityRules
}

func (s *ClusterScope) generateSpecFromSecurityRules(rules []core.SecurityRule, nsgId *string) (map[string]infrastructurev1beta2.IngressSecurityRuleForNSG, map[string]infrastructurev1beta2.EgressSecurityRuleForNSG) {
	var ingressRules = make(map[string]infrastructurev1beta2.IngressSecurityRuleForNSG)
	var egressRules = make(map[string]infrastructurev1beta2.EgressSecurityRuleForNSG)
	var stateless *bool
	for _, rule := range rules {

		icmpOptions, tcpOptions, udpOptions := getProtocolOptionsForSpec(rule.IcmpOptions, rule.TcpOptions, rule.UdpOptions)
		stateless = rule.IsStateless
		if stateless == nil {
			stateless = common.Bool(false)
		}
		switch rule.Direction {
		case core.SecurityRuleDirectionIngress:
			ingressRule := infrastructurev1beta2.IngressSecurityRuleForNSG{
				IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
					Protocol:    rule.Protocol,
					Source:      rule.Source,
					IcmpOptions: icmpOptions,
					IsStateless: stateless,
					SourceType:  infrastructurev1beta2.IngressSecurityRuleSourceTypeEnum(rule.SourceType),
					TcpOptions:  tcpOptions,
					UdpOptions:  udpOptions,
					Description: rule.Description,
				},
			}
			if rule.SourceType == core.SecurityRuleSourceTypeNetworkSecurityGroup {
				ingressRule.IngressSecurityRule.Source = getNsgNameFromId(rule.Source, s.GetNSGSpec())
			}
			ingressRules[*rule.Id] = ingressRule
		case core.SecurityRuleDirectionEgress:
			egressRule := infrastructurev1beta2.EgressSecurityRuleForNSG{
				EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
					Destination:     rule.Destination,
					Protocol:        rule.Protocol,
					DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeEnum(rule.DestinationType),
					IcmpOptions:     icmpOptions,
					IsStateless:     stateless,
					TcpOptions:      tcpOptions,
					UdpOptions:      udpOptions,
					Description:     rule.Description,
				},
			}
			if rule.DestinationType == core.SecurityRuleDestinationTypeNetworkSecurityGroup {
				egressRule.EgressSecurityRule.Destination = getNsgNameFromId(rule.Destination, s.GetNSGSpec())
			}
			egressRules[*rule.Id] = egressRule
		}
	}
	return ingressRules, egressRules

}

func (s *ClusterScope) AddNSGSecurityRules(ctx context.Context, nsgId *string, ingress []infrastructurev1beta2.IngressSecurityRuleForNSG,
	egress []infrastructurev1beta2.EgressSecurityRuleForNSG) error {
	securityRules := s.generateAddSecurityRuleFromSpec(ingress, egress)

	_, err := s.VCNClient.AddNetworkSecurityGroupSecurityRules(ctx, core.AddNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: nsgId,
		AddNetworkSecurityGroupSecurityRulesDetails: core.AddNetworkSecurityGroupSecurityRulesDetails{
			SecurityRules: securityRules,
		},
	})
	if err != nil {
		s.Logger.Error(err, "failed add nsg security rules")
		return errors.Wrap(err, "failed add nsg security rules")
	}
	return nil
}

func (s *ClusterScope) CreateNSG(ctx context.Context, nsg infrastructurev1beta2.NSG) (*string, error) {
	createNetworkSecurityGroupDetails := core.CreateNetworkSecurityGroupDetails{
		CompartmentId: common.String(s.GetCompartmentId()),
		VcnId:         s.getVcnId(),
		DefinedTags:   s.GetDefinedTags(),
		DisplayName:   common.String(nsg.Name),
		FreeformTags:  s.GetFreeFormTags(),
	}
	nsgResponse, err := s.VCNClient.CreateNetworkSecurityGroup(ctx, core.CreateNetworkSecurityGroupRequest{
		CreateNetworkSecurityGroupDetails: createNetworkSecurityGroupDetails,
		RequestMetadata:                   common.RequestMetadata{},
	})
	if err != nil {
		s.Logger.Error(err, "failed create nsg")
		return nil, errors.Wrap(err, "failed create nsg")
	}
	s.Logger.Info("successfully created the nsg", "nsg", *nsgResponse.Id)
	return nsgResponse.Id, nil
}

func getProtocolOptionsForSpec(icmp *core.IcmpOptions, tcp *core.TcpOptions, udp *core.UdpOptions) (*infrastructurev1beta2.IcmpOptions, *infrastructurev1beta2.TcpOptions,
	*infrastructurev1beta2.UdpOptions) {
	var icmpOptions *infrastructurev1beta2.IcmpOptions
	var tcpOptions *infrastructurev1beta2.TcpOptions
	var udpOptions *infrastructurev1beta2.UdpOptions
	if icmp != nil {
		icmpOptions = &infrastructurev1beta2.IcmpOptions{
			Type: icmp.Type,
			Code: icmp.Code,
		}
	}
	if tcp != nil {
		tcpOptions = &infrastructurev1beta2.TcpOptions{}
		if tcp.DestinationPortRange != nil {
			tcpOptions.DestinationPortRange = &infrastructurev1beta2.PortRange{}
			tcpOptions.DestinationPortRange.Max = tcp.DestinationPortRange.Max
			tcpOptions.DestinationPortRange.Min = tcp.DestinationPortRange.Min
		}
		if tcp.SourcePortRange != nil {
			tcpOptions.SourcePortRange = &infrastructurev1beta2.PortRange{}
			tcpOptions.SourcePortRange.Max = tcp.SourcePortRange.Max
			tcpOptions.SourcePortRange.Min = tcp.SourcePortRange.Min
		}
	}
	if udp != nil {
		udpOptions = &infrastructurev1beta2.UdpOptions{}
		if udp.DestinationPortRange != nil {
			udpOptions.DestinationPortRange = &infrastructurev1beta2.PortRange{}
			udpOptions.DestinationPortRange.Max = udp.DestinationPortRange.Max
			udpOptions.DestinationPortRange.Min = udp.DestinationPortRange.Min
		}
		if udp.SourcePortRange != nil {
			udpOptions.SourcePortRange = &infrastructurev1beta2.PortRange{}
			udpOptions.SourcePortRange.Max = udp.SourcePortRange.Max
			udpOptions.SourcePortRange.Min = udp.SourcePortRange.Min
		}
	}
	return icmpOptions, tcpOptions, udpOptions
}

func getNsgIdFromName(nsgName *string, list []*infrastructurev1beta2.NSG) *string {
	nsgSlice := ptr.ToNSGSlice(list)
	for i := range nsgSlice {
		if nsgSlice[i].Name == *nsgName {
			return nsgSlice[i].ID
		}
	}
	return nil
}

func getNsgNameFromId(nsgId *string, list []*infrastructurev1beta2.NSG) *string {
	if nsgId == nil {
		return nil
	}

	nsgSlice := ptr.ToNSGSlice(list)
	for i := range nsgSlice {
		nsg := &nsgSlice[i]
		if nsg.ID != nil && reflect.DeepEqual(nsg.ID, nsgId) {
			return &nsg.Name
		}
	}
	return nil
}
