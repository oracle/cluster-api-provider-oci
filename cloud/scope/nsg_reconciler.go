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

	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v53/common"
	"github.com/oracle/oci-go-sdk/v53/core"
	"github.com/pkg/errors"
)

func (s *ClusterScope) ReconcileNSG(ctx context.Context) error {
	desiredNSGs, err := s.NSGSpec()
	if err != nil {
		s.Logger.Error(err, "error to generate nsg spec")
		return errors.Wrap(err, "error to generate nsg spec")
	}
	for _, desiredNSG := range desiredNSGs {
		nsg, err := s.GetNSG(ctx, *desiredNSG)
		if err != nil {
			s.Logger.Error(err, "error to get nsg")
			return err
		}
		if nsg != nil {
			nsgOCID := nsg.Id
			desiredNSG.ID = nsgOCID
			if !s.IsNSGEqual(nsg, *desiredNSG) {
				err = s.UpdateNSG(ctx, *desiredNSG)
				if err != nil {
					return err
				}
				s.Logger.Info("Successfully updated network security list", "nsg", nsgOCID)
			}
			ingressRules, egressRules, isNSGUpdated, err := s.UpdateNSGSecurityRulesIfNeeded(ctx, *desiredNSG, nsg)
			if err != nil {
				return err
			}
			desiredNSG.IngressRules = ingressRules
			desiredNSG.EgressRules = egressRules
			if !isNSGUpdated {
				s.Logger.Info("No Reconciliation Required for Network Security Group", "nsg", *desiredNSG.ID)
			}
			continue
		}
		s.Logger.Info("Creating the network security list")
		nsgID, err := s.CreateNSG(ctx, *desiredNSG)
		if err != nil {
			return err
		}
		s.Logger.Info("Created the nsg", "nsg", nsgID)
		desiredNSG.ID = nsgID
		ingressRules, egressRules, err := s.AddNSGSecurityRules(ctx, desiredNSG.ID, desiredNSG.IngressRules, desiredNSG.EgressRules)
		if err != nil {
			return err
		}
		desiredNSG.IngressRules = ingressRules
		desiredNSG.EgressRules = egressRules
	}
	return nil
}

func (s *ClusterScope) GetNSG(ctx context.Context, spec infrastructurev1beta1.NSG) (*core.NetworkSecurityGroup, error) {
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
	desiredNSGs := s.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups
	for _, desiredNSG := range desiredNSGs {
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

func (s *ClusterScope) NSGSpec() ([]*infrastructurev1beta1.NSG, error) {
	nsgs := s.GetNSGSpec()
	if !s.IsNSGExitsByRole(infrastructurev1beta1.ControlPlaneEndpointRole) && !s.IsSecurityListExitsByRole(infrastructurev1beta1.ControlPlaneEndpointRole) {
		nsgs = append(nsgs, &infrastructurev1beta1.NSG{
			Role:         infrastructurev1beta1.ControlPlaneEndpointRole,
			Name:         ControlPlaneEndpointDefaultName,
			IngressRules: s.GetControlPlaneEndpointDefaultIngressRules(),
			EgressRules:  s.GetControlPlaneEndpointDefaultEgressRules(),
		})
	}
	if !s.IsNSGExitsByRole(infrastructurev1beta1.ControlPlaneRole) && !s.IsSecurityListExitsByRole(infrastructurev1beta1.ControlPlaneRole) {
		nsgs = append(nsgs, &infrastructurev1beta1.NSG{
			Role:         infrastructurev1beta1.ControlPlaneRole,
			Name:         ControlPlaneDefaultName,
			IngressRules: s.GetControlPlaneMachineDefaultIngressRules(),
			EgressRules:  s.GetControlPlaneMachineDefaultEgressRules(),
		})
	}
	if !s.IsNSGExitsByRole(infrastructurev1beta1.WorkerRole) && !s.IsSecurityListExitsByRole(infrastructurev1beta1.WorkerRole) {
		nsgs = append(nsgs, &infrastructurev1beta1.NSG{
			Role:         infrastructurev1beta1.WorkerRole,
			Name:         WorkerDefaultName,
			IngressRules: s.GetNodeDefaultIngressRules(),
			EgressRules:  s.GetNodeDefaultEgressRules(),
		})
	}
	if !s.IsNSGExitsByRole(infrastructurev1beta1.ServiceLoadBalancerRole) && !s.IsSecurityListExitsByRole(infrastructurev1beta1.ServiceLoadBalancerRole) {
		nsgs = append(nsgs, &infrastructurev1beta1.NSG{
			Role:         infrastructurev1beta1.ServiceLoadBalancerRole,
			Name:         ServiceLBDefaultName,
			IngressRules: s.GetServiceLoadBalancerDefaultIngressRules(),
			EgressRules:  s.GetServiceLoadBalancerDefaultEgressRules(),
		})
	}
	s.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups = nsgs
	return s.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups, nil
}

func (s *ClusterScope) GetNSGSpec() []*infrastructurev1beta1.NSG {
	return s.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups
}

func (s *ClusterScope) IsNSGExitsByRole(role infrastructurev1beta1.Role) bool {
	for _, nsg := range s.GetNSGSpec() {
		if role == nsg.Role {
			return true
		}
	}
	return false
}

func (s *ClusterScope) IsNSGEqual(actual *core.NetworkSecurityGroup, desired infrastructurev1beta1.NSG) bool {
	if *actual.DisplayName != desired.Name {
		return false
	}
	return s.IsTagsEqual(actual.FreeformTags, actual.DefinedTags)
}

func (s *ClusterScope) UpdateNSGSecurityRulesIfNeeded(ctx context.Context, desired infrastructurev1beta1.NSG,
	actual *core.NetworkSecurityGroup) ([]infrastructurev1beta1.IngressSecurityRuleForNSG, []infrastructurev1beta1.EgressSecurityRuleForNSG, bool, error) {
	var ingressRulesToAdd, ingressRulesToUpdate, finalIngressRules []infrastructurev1beta1.IngressSecurityRuleForNSG
	var egressRulesToAdd, egressRulesToUpdate, finalEgressRules []infrastructurev1beta1.EgressSecurityRuleForNSG
	var securityRulesToRemove []string
	var isNSGUpdated bool
	listSecurityRulesResponse, err := s.VCNClient.ListNetworkSecurityGroupSecurityRules(ctx, core.ListNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: actual.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the network security group, failed to list security rules")
		return nil, nil, isNSGUpdated, errors.Wrap(err, "failed to reconcile the network security group, failed to list security rules")
	}
	ingressRules, egressRules := generateSpecFromSecurityRules(listSecurityRulesResponse.Items)

	for i, ingressRule := range desired.IngressRules {
		if ingressRule.ID == nil {
			ingressRulesToAdd = append(ingressRulesToAdd, ingressRule)
		}
		if ingressRule.IsStateless == nil {
			desired.IngressRules[i].IsStateless = common.Bool(false)
		}
	}
	for i, egressRule := range desired.EgressRules {
		if egressRule.ID == nil {
			egressRulesToAdd = append(egressRulesToAdd, egressRule)
		}
		if egressRule.IsStateless == nil {
			desired.EgressRules[i].IsStateless = common.Bool(false)
		}
	}

	for _, actualRule := range ingressRules {
		found := false
		for _, desiredRule := range desired.IngressRules {
			if (desiredRule.ID != nil) && (*actualRule.ID == *desiredRule.ID) {
				found = true
				if !reflect.DeepEqual(desiredRule, actualRule) {
					ingressRulesToUpdate = append(ingressRulesToUpdate, desiredRule)
				}
				finalIngressRules = append(finalIngressRules, desiredRule)
				break
			}
		}
		if !found {
			securityRulesToRemove = append(securityRulesToRemove, *actualRule.ID)
		}
	}
	for _, actualRule := range egressRules {
		found := false
		for _, desiredRule := range desired.EgressRules {
			if (desiredRule.ID != nil) && (*actualRule.ID == *desiredRule.ID) {
				found = true
				if !reflect.DeepEqual(desiredRule, actualRule) {
					egressRulesToUpdate = append(egressRulesToUpdate, desiredRule)
				}
				finalEgressRules = append(finalEgressRules, desiredRule)
				break
			}
		}
		if !found {
			securityRulesToRemove = append(securityRulesToRemove, *actualRule.ID)
		}
	}
	if len(ingressRulesToAdd) > 0 || len(egressRulesToAdd) > 0 {
		isNSGUpdated = true
		ingress, egress, err := s.AddNSGSecurityRules(ctx, desired.ID, ingressRulesToAdd, egressRulesToAdd)
		if err != nil {
			s.Logger.Error(err, "failed to reconcile the network security group, failed to add security rules")
			return nil, nil, isNSGUpdated, err
		}
		s.Logger.Info("Successfully added missing rules in NSG", "nsg", *actual.Id)
		finalEgressRules = append(finalEgressRules, egress...)
		finalIngressRules = append(finalIngressRules, ingress...)
	}
	if len(ingressRulesToUpdate) > 0 || len(egressRulesToUpdate) > 0 {
		isNSGUpdated = true
		securityRules := generateUpdateSecurityRuleFromSpec(ingressRulesToUpdate, egressRulesToUpdate)
		_, err := s.VCNClient.UpdateNetworkSecurityGroupSecurityRules(ctx, core.UpdateNetworkSecurityGroupSecurityRulesRequest{
			NetworkSecurityGroupId: desired.ID,
			UpdateNetworkSecurityGroupSecurityRulesDetails: core.UpdateNetworkSecurityGroupSecurityRulesDetails{
				SecurityRules: securityRules,
			},
		})
		if err != nil {
			s.Logger.Error(err, "failed to reconcile the network security group, failed to update security rules")
			return nil, nil, isNSGUpdated, err
		}
		s.Logger.Info("Successfully updated rules in NSG", "nsg", *actual.Id)
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
			return nil, nil, isNSGUpdated, err
		}
		s.Logger.Info("Successfully deleted rules in NSG", "nsg", *actual.Id)
	}
	s.Logger.Info("No Reconciliation Required for Network Security List rules", "nsg", desired.ID)
	return finalIngressRules, finalEgressRules, isNSGUpdated, nil
}

func (s *ClusterScope) UpdateNSG(ctx context.Context, nsgSpec infrastructurev1beta1.NSG) error {
	updateNSGDetails := core.UpdateNetworkSecurityGroupDetails{
		DisplayName:  common.String(nsgSpec.Name),
		FreeformTags: s.GetFreeFormTags(),
		DefinedTags:  s.GetDefinedTags(),
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

func generateAddSecurityRuleFromSpec(ingressRules []infrastructurev1beta1.IngressSecurityRuleForNSG,
	egressRules []infrastructurev1beta1.EgressSecurityRuleForNSG) []core.AddSecurityRuleDetails {
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
		securityRules = append(securityRules, core.AddSecurityRuleDetails{
			Direction:   core.AddSecurityRuleDetailsDirectionIngress,
			Protocol:    ingressRule.Protocol,
			Description: ingressRule.Description,
			IcmpOptions: icmpOptions,
			IsStateless: stateless,
			Source:      ingressRule.Source,
			SourceType:  core.AddSecurityRuleDetailsSourceTypeEnum(ingressRule.SourceType),
			TcpOptions:  tcpOptions,
			UdpOptions:  udpOptions,
		})
	}
	for _, egressRule := range egressRules {
		icmpOptions, tcpOptions, udpOptions = getProtocolOptions(egressRule.IcmpOptions, egressRule.TcpOptions, egressRule.UdpOptions)
		// while comparing values, the boolean value has to be always set
		stateless = egressRule.IsStateless
		if stateless == nil {
			stateless = common.Bool(false)
		}
		securityRules = append(securityRules, core.AddSecurityRuleDetails{
			Direction:       core.AddSecurityRuleDetailsDirectionEgress,
			Protocol:        egressRule.Protocol,
			Description:     egressRule.Description,
			IcmpOptions:     icmpOptions,
			IsStateless:     stateless,
			Destination:     egressRule.Destination,
			DestinationType: core.AddSecurityRuleDetailsDestinationTypeEnum(egressRule.DestinationType),
			TcpOptions:      tcpOptions,
			UdpOptions:      udpOptions,
		})
	}
	return securityRules
}

func generateUpdateSecurityRuleFromSpec(ingressRules []infrastructurev1beta1.IngressSecurityRuleForNSG,
	egressRules []infrastructurev1beta1.EgressSecurityRuleForNSG) []core.UpdateSecurityRuleDetails {
	var securityRules []core.UpdateSecurityRuleDetails
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
		securityRules = append(securityRules, core.UpdateSecurityRuleDetails{
			Direction:   core.UpdateSecurityRuleDetailsDirectionIngress,
			Id:          ingressRule.ID,
			Protocol:    ingressRule.Protocol,
			Description: ingressRule.Description,
			IcmpOptions: icmpOptions,
			IsStateless: stateless,
			Source:      ingressRule.Source,
			SourceType:  core.UpdateSecurityRuleDetailsSourceTypeEnum(ingressRule.SourceType),
			TcpOptions:  tcpOptions,
			UdpOptions:  udpOptions,
		})
	}
	for _, egressRule := range egressRules {
		icmpOptions, tcpOptions, udpOptions = getProtocolOptions(egressRule.IcmpOptions, egressRule.TcpOptions, egressRule.UdpOptions)
		stateless = egressRule.IsStateless
		if stateless == nil {
			stateless = common.Bool(false)
		}
		securityRules = append(securityRules, core.UpdateSecurityRuleDetails{
			Direction:       core.UpdateSecurityRuleDetailsDirectionEgress,
			Protocol:        egressRule.Protocol,
			Description:     egressRule.Description,
			IcmpOptions:     icmpOptions,
			IsStateless:     stateless,
			Destination:     egressRule.Destination,
			DestinationType: core.UpdateSecurityRuleDetailsDestinationTypeEnum(egressRule.DestinationType),
			TcpOptions:      tcpOptions,
			UdpOptions:      udpOptions,
			Id:              egressRule.ID,
		})
	}
	return securityRules
}

func generateSpecFromSecurityRules(rules []core.SecurityRule) ([]infrastructurev1beta1.IngressSecurityRuleForNSG, []infrastructurev1beta1.EgressSecurityRuleForNSG) {
	var ingressRules []infrastructurev1beta1.IngressSecurityRuleForNSG
	var egressRules []infrastructurev1beta1.EgressSecurityRuleForNSG
	var stateless *bool
	for _, rule := range rules {

		icmpOptions, tcpOptions, udpOptions := getProtocolOptionsForSpec(rule.IcmpOptions, rule.TcpOptions, rule.UdpOptions)
		stateless = rule.IsStateless
		if stateless == nil {
			stateless = common.Bool(false)
		}
		switch rule.Direction {
		case core.SecurityRuleDirectionIngress:
			ingressRule := infrastructurev1beta1.IngressSecurityRuleForNSG{
				ID: rule.Id,
				IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
					Protocol:    rule.Protocol,
					Source:      rule.Source,
					IcmpOptions: icmpOptions,
					IsStateless: stateless,
					SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeEnum(rule.SourceType),
					TcpOptions:  tcpOptions,
					UdpOptions:  udpOptions,
					Description: rule.Description,
				},
			}
			ingressRules = append(ingressRules, ingressRule)
		case core.SecurityRuleDirectionEgress:
			egressRule := infrastructurev1beta1.EgressSecurityRuleForNSG{
				ID: rule.Id,
				EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
					Destination:     rule.Destination,
					Protocol:        rule.Protocol,
					DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeEnum(rule.DestinationType),
					IcmpOptions:     icmpOptions,
					IsStateless:     stateless,
					TcpOptions:      tcpOptions,
					UdpOptions:      udpOptions,
					Description:     rule.Description,
				},
			}
			egressRules = append(egressRules, egressRule)
		}
	}
	return ingressRules, egressRules

}

func (s *ClusterScope) AddNSGSecurityRules(ctx context.Context, nsgID *string, ingress []infrastructurev1beta1.IngressSecurityRuleForNSG,
	egress []infrastructurev1beta1.EgressSecurityRuleForNSG) ([]infrastructurev1beta1.IngressSecurityRuleForNSG, []infrastructurev1beta1.EgressSecurityRuleForNSG, error) {
	securityRules := generateAddSecurityRuleFromSpec(ingress, egress)

	addNetworkSecurityGroupSecurityRulesResponse, err := s.VCNClient.AddNetworkSecurityGroupSecurityRules(ctx, core.AddNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: nsgID,
		AddNetworkSecurityGroupSecurityRulesDetails: core.AddNetworkSecurityGroupSecurityRulesDetails{
			SecurityRules: securityRules,
		},
	})
	if err != nil {
		s.Logger.Error(err, "failed add nsg security rules")
		return nil, nil, errors.Wrap(err, "failed add nsg security rules")
	}
	ingressRules, egressRules := generateSpecFromSecurityRules(addNetworkSecurityGroupSecurityRulesResponse.SecurityRules)
	s.Logger.Info("successfully added nsg rules", "nsg", *nsgID)
	return ingressRules, egressRules, nil
}

func (s *ClusterScope) CreateNSG(ctx context.Context, nsg infrastructurev1beta1.NSG) (*string, error) {
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

func (s *ClusterScope) GetControlPlaneMachineDefaultIngressRules() []infrastructurev1beta1.IngressSecurityRuleForNSG {
	return []infrastructurev1beta1.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Kubernetes API endpoint to Control Plane(apiserver port) communication"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(int(s.APIServerPort())),
						Min: common.Int(int(s.APIServerPort())),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneEndpointSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Control plane node to Control Plane(apiserver port) communication"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(int(s.APIServerPort())),
						Min: common.Int(int(s.APIServerPort())),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Worker Node to Control Plane(apiserver port) communication"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(int(s.APIServerPort())),
						Min: common.Int(int(s.APIServerPort())),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("etcd client communication"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(2379),
						Min: common.Int(2379),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("etcd peer"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(2380),
						Min: common.Int(2380),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Calico networking (BGP)"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(179),
						Min: common.Int(179),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Calico networking (BGP)"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(179),
						Min: common.Int(179),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Calico networking with IP-in-IP enabled"),
				Protocol:    common.String("4"),
				SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Calico networking with IP-in-IP enabled"),
				Protocol:    common.String("4"),
				SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Path discovery"),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta1.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(VcnDefaultCidr),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Inbound SSH traffic to Control Plane"),
				Protocol:    common.String("6"),
				SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String("0.0.0.0/0"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(22),
						Min: common.Int(22),
					},
				},
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Control Plane to Control Plane Kubelet Communication"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(10250),
						Min: common.Int(10250),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
	}
}

func (s *ClusterScope) GetControlPlaneMachineDefaultEgressRules() []infrastructurev1beta1.EgressSecurityRuleForNSG {
	return []infrastructurev1beta1.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
				Description:     common.String("Control Plane access to Internet"),
				Protocol:        common.String("all"),
				DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String("0.0.0.0/0"),
			},
		},
	}
}

func (s *ClusterScope) GetNodeDefaultIngressRules() []infrastructurev1beta1.IngressSecurityRuleForNSG {
	return []infrastructurev1beta1.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Inbound SSH traffic to worker node"),
				Protocol:    common.String("6"),
				SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String("0.0.0.0/0"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(22),
						Min: common.Int(22),
					},
				},
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Path discovery"),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta1.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(VcnDefaultCidr),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Control Plane to worker node Kubelet Communication"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(10250),
						Min: common.Int(10250),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Worker node to worker node Kubelet Communication"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(10250),
						Min: common.Int(10250),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Calico networking (BGP)"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(179),
						Min: common.Int(179),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Calico networking (BGP)"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(179),
						Min: common.Int(179),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Calico networking with IP-in-IP enabled"),
				Protocol:    common.String("4"),
				SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Calico networking with IP-in-IP enabled"),
				Protocol:    common.String("4"),
				SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Worker node to default NodePort ingress communication"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(32767),
						Min: common.Int(30000),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
	}
}

func (s *ClusterScope) GetNodeDefaultEgressRules() []infrastructurev1beta1.EgressSecurityRuleForNSG {
	return []infrastructurev1beta1.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
				Description:     common.String("Worker node access to Internet"),
				Protocol:        common.String("all"),
				DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String("0.0.0.0/0"),
			},
		},
	}
}

func (s *ClusterScope) GetServiceLoadBalancerDefaultIngressRules() []infrastructurev1beta1.IngressSecurityRuleForNSG {
	return []infrastructurev1beta1.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Path discovery"),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta1.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(VcnDefaultCidr),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Accept http traffic on port 80"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(80),
						Min: common.Int(80),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Accept https traffic on port 443"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(443),
						Min: common.Int(443),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
	}
}

func (s *ClusterScope) GetServiceLoadBalancerDefaultEgressRules() []infrastructurev1beta1.EgressSecurityRuleForNSG {
	return []infrastructurev1beta1.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
				Destination:     common.String(WorkerSubnetDefaultCIDR),
				Protocol:        common.String("6"),
				DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(32767),
						Min: common.Int(30000),
					},
				},
				Description: common.String("Service LoadBalancer to default NodePort egress communication"),
			},
		},
	}
}

func (s *ClusterScope) GetControlPlaneEndpointDefaultIngressRules() []infrastructurev1beta1.IngressSecurityRuleForNSG {
	return []infrastructurev1beta1.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("External access to Kubernetes API endpoint"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(int(s.APIServerPort())),
						Min: common.Int(int(s.APIServerPort())),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("Path discovery"),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta1.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(VcnDefaultCidr),
			},
		},
	}
}

func (s *ClusterScope) GetControlPlaneEndpointDefaultEgressRules() []infrastructurev1beta1.EgressSecurityRuleForNSG {
	return []infrastructurev1beta1.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
				Description: common.String("Kubernetes API traffic to Control Plane"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(int(s.APIServerPort())),
						Min: common.Int(int(s.APIServerPort())),
					},
				},
				DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
	}
}

func getProtocolOptionsForSpec(icmp *core.IcmpOptions, tcp *core.TcpOptions, udp *core.UdpOptions) (*infrastructurev1beta1.IcmpOptions, *infrastructurev1beta1.TcpOptions,
	*infrastructurev1beta1.UdpOptions) {
	var icmpOptions *infrastructurev1beta1.IcmpOptions
	var tcpOptions *infrastructurev1beta1.TcpOptions
	var udpOptions *infrastructurev1beta1.UdpOptions
	if icmp != nil {
		icmpOptions = &infrastructurev1beta1.IcmpOptions{
			Type: icmp.Type,
			Code: icmp.Type,
		}
	}
	if tcp != nil {
		tcpOptions = &infrastructurev1beta1.TcpOptions{}
		if tcp.DestinationPortRange != nil {
			tcpOptions.DestinationPortRange = &infrastructurev1beta1.PortRange{}
			tcpOptions.DestinationPortRange.Max = tcp.DestinationPortRange.Max
			tcpOptions.DestinationPortRange.Min = tcp.DestinationPortRange.Min
		}
		if tcp.SourcePortRange != nil {
			tcpOptions.SourcePortRange = &infrastructurev1beta1.PortRange{}
			tcpOptions.SourcePortRange.Max = tcp.SourcePortRange.Max
			tcpOptions.SourcePortRange.Min = tcp.SourcePortRange.Min
		}
	}
	if udp != nil {
		udpOptions = &infrastructurev1beta1.UdpOptions{}
		if udp.DestinationPortRange != nil {
			udpOptions.DestinationPortRange = &infrastructurev1beta1.PortRange{}
			udpOptions.DestinationPortRange.Max = udp.DestinationPortRange.Max
			udpOptions.DestinationPortRange.Min = udp.DestinationPortRange.Min
		}
		if udp.SourcePortRange != nil {
			udpOptions.SourcePortRange = &infrastructurev1beta1.PortRange{}
			udpOptions.SourcePortRange.Max = udp.SourcePortRange.Max
			udpOptions.SourcePortRange.Min = udp.SourcePortRange.Min
		}
	}
	return icmpOptions, tcpOptions, udpOptions
}
