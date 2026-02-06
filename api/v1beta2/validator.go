/*
 *
 * Copyright (c) 2022, Oracle and/or its affiliates.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 *
 */

package v1beta2

import (
	"fmt"
	"net"
	"regexp"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// can't use: \/"'[]:|<>+=;,.?*@&, Can't start with underscore. Can't end with period or hyphen.
	// not using . in the name to avoid issues when the name is part of DNS name.
	clusterNameRegex = `^[a-z0-9][a-z0-9-]{0,42}[a-z0-9]$`

	// Rule type constants for error messages
	egressRulesType  = "egressRules"
	ingressRulesType = "ingressRules"

	// Error message formats for NSG security rule validation
	udpDestinationPortRangeMaxRequiredFormat = "invalid %s: UdpOptions DestinationPortRange Max may not be empty"
	udpDestinationPortRangeMinRequiredFormat = "invalid %s: UdpOptions DestinationPortRange Min may not be empty"
	udpSourcePortRangeMaxRequiredFormat      = "invalid %s: UdpOptions SourcePortRange Max may not be empty"
	udpSourcePortRangeMinRequiredFormat      = "invalid %s: UdpOptions SourcePortRange Min may not be empty"
	tcpDestinationPortRangeMaxRequiredFormat = "invalid %s: TcpOptions DestinationPortRange Max may not be empty"
	tcpDestinationPortRangeMinRequiredFormat = "invalid %s: TcpOptions DestinationPortRange Min may not be empty"
	tcpSourcePortRangeMaxRequiredFormat      = "invalid %s: TcpOptions SourcePortRange Max may not be empty"
	tcpSourcePortRangeMinRequiredFormat      = "invalid %s: TcpOptions SourcePortRange Min may not be empty"
	icmpTypeRequiredFormat                   = "invalid %s: IcmpOptions Type may not be empty"
	destinationRequiredFormat                = "invalid %s: Destination may not be empty"
	sourceRequiredFormat                     = "invalid %s: Source may not be empty"
	protocolRequiredFormat                   = "invalid %s: Protocol may not be empty"
	invalidCIDRFormatFormat                  = "invalid %s: CIDR format"
)

// invalidNameRegex is a broad regex used to validate allows names in OCI
var invalidNameRegex = regexp.MustCompile("\\s")

// validatePortRange validates that both Min and Max are set for a port range
func validatePortRange(portRange *PortRange, fldPath *field.Path, maxMsg, minMsg string) field.ErrorList {
	var allErrs field.ErrorList
	if portRange.Max == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, portRange.Max, maxMsg))
	}
	if portRange.Min == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, portRange.Min, minMsg))
	}
	return allErrs
}

// validateCIDR validates a CIDR string
func validateCIDR(cidr *string, fldPath *field.Path, errorMsg string) field.ErrorList {
	var allErrs field.ErrorList
	if _, _, err := net.ParseCIDR(ociutil.DerefString(cidr)); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, cidr, errorMsg))
	}
	return allErrs
}

// ValidOcid is a simple pre-flight
// we will let the serverside handle the more complex and compete validation
func ValidOcid(ocid string) bool {
	if len(ocid) >= 4 && ocid[:4] == "ocid" {
		return true
	}

	return false
}

// validShape is a simple pre-flight
// we will let the serverside handle the more complex and compete validation.
func validShape(shape string) bool {
	return len(shape) > 0
}

// ValidRegion test if the string can be a region.
func ValidRegion(stringRegion string) bool {

	// region can be blank since the regional information
	// can be derived from other sources
	if stringRegion == "" {
		return true
	}

	if invalidNameRegex.MatchString(stringRegion) {
		return false
	}
	return true
}

// ValidateNetworkSpec validates the NetworkSpec
func ValidateNetworkSpec(validRoles []Role, networkSpec NetworkSpec, old NetworkSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(networkSpec.Vcn.CIDR) > 0 {
		vcnErrors := validateVCNCIDR(networkSpec.Vcn.CIDR, fldPath.Child("cidr"))
		allErrs = append(allErrs, vcnErrors...)
	}

	if networkSpec.Vcn.Subnets != nil {
		subnetErrors := validateSubnets(validRoles, networkSpec.Vcn.Subnets, networkSpec.Vcn, fldPath.Child("subnets"))
		allErrs = append(allErrs, subnetErrors...)
	}

	if networkSpec.Vcn.NetworkSecurityGroup.List != nil {
		nsgErrors := validateNSGs(validRoles, networkSpec.Vcn.NetworkSecurityGroup.List, fldPath.Child("networkSecurityGroups"))
		allErrs = append(allErrs, nsgErrors...)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

// validateVCNCIDR validates the CIDR of a VNC.
func validateVCNCIDR(vncCIDR string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if _, _, err := net.ParseCIDR(vncCIDR); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, vncCIDR, "invalid CIDR format"))
	}
	return allErrs
}

// ValidateClusterName validates the name of the cluster.
func ValidateClusterName(name string) field.ErrorList {
	var allErrs field.ErrorList

	if success, _ := regexp.MatchString(clusterNameRegex, name); !success {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("Name"), name,
			fmt.Sprintf("Cluster Name doesn't match regex %s, can contain only lowercase alphanumeric characters and '-', must start/end with an alphanumeric character",
				clusterNameRegex)))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

// validateSubnetCIDR validates the CIDR blocks of a Subnet.
func validateSubnetCIDR(subnetCidr string, vcnCidr string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(subnetCidr) > 0 {
		subnetCidrIP, _, err := net.ParseCIDR(subnetCidr)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, subnetCidr, "invalid CIDR format"))
		}

		// Check subnet is in vcnCidr if vcnCidr is set
		if len(vcnCidr) > 0 {
			var vcnNetwork *net.IPNet
			if _, parseNetwork, err := net.ParseCIDR(vcnCidr); err == nil {
				vcnNetwork = parseNetwork
			}

			var found bool
			if vcnNetwork != nil && vcnNetwork.Contains(subnetCidrIP) {
				found = true
			}

			if !found {
				allErrs = append(allErrs, field.Invalid(fldPath, subnetCidr, fmt.Sprintf("subnet CIDR not in VCN address space: %s", vcnCidr)))
			}
		}

	}
	return allErrs
}

// validateNSGs validates a list of Subnets.
func validateNSGs(validRoles []Role, networkSecurityGroups []*NSG, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i, nsg := range networkSecurityGroups {
		if err := validateRole(validRoles, nsg.Role, fldPath.Index(i).Child("role"), "networkSecurityGroup role invalid"); err != nil {
			allErrs = append(allErrs, err)
		}
		egressErrors := validateEgressSecurityRuleForNSG(nsg.EgressRules, fldPath.Index(i).Child("egressRules"))
		allErrs = append(allErrs, egressErrors...)
		ingressErrors := validateIngressSecurityRuleForNSG(nsg.IngressRules, fldPath.Index(i).Child("ingressRules"))
		allErrs = append(allErrs, ingressErrors...)
	}

	return allErrs
}

// validateEgressSecurityRuleForNSG validates the Egress Security Rules for NSG.
func validateEgressSecurityRuleForNSG(egressRules []EgressSecurityRuleForNSG, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i, r := range egressRules {
		rulePath := fldPath.Index(i)
		rule := r.EgressSecurityRule

		// Validate UDP options port ranges
		if udpOptions := rule.UdpOptions; udpOptions != nil {
			if udpOptions.DestinationPortRange != nil {
				portRangeErrors := validatePortRange(
					udpOptions.DestinationPortRange,
					rulePath.Child("udpOptions").Child("destinationPortRange"),
					fmt.Sprintf(udpDestinationPortRangeMaxRequiredFormat, egressRulesType),
					fmt.Sprintf(udpDestinationPortRangeMinRequiredFormat, egressRulesType),
				)
				allErrs = append(allErrs, portRangeErrors...)
			}

			if udpOptions.SourcePortRange != nil {
				portRangeErrors := validatePortRange(
					udpOptions.SourcePortRange,
					rulePath.Child("udpOptions").Child("sourcePortRange"),
					fmt.Sprintf(udpSourcePortRangeMaxRequiredFormat, egressRulesType),
					fmt.Sprintf(udpSourcePortRangeMinRequiredFormat, egressRulesType),
				)
				allErrs = append(allErrs, portRangeErrors...)
			}
		}

		// Validate TCP options port ranges
		if tcpOptions := rule.TcpOptions; tcpOptions != nil {
			if tcpOptions.DestinationPortRange != nil {
				portRangeErrors := validatePortRange(
					tcpOptions.DestinationPortRange,
					rulePath.Child("tcpOptions").Child("destinationPortRange"),
					fmt.Sprintf(tcpDestinationPortRangeMaxRequiredFormat, egressRulesType),
					fmt.Sprintf(tcpDestinationPortRangeMinRequiredFormat, egressRulesType),
				)
				allErrs = append(allErrs, portRangeErrors...)
			}

			if tcpOptions.SourcePortRange != nil {
				portRangeErrors := validatePortRange(
					tcpOptions.SourcePortRange,
					rulePath.Child("tcpOptions").Child("sourcePortRange"),
					fmt.Sprintf(tcpSourcePortRangeMaxRequiredFormat, egressRulesType),
					fmt.Sprintf(tcpSourcePortRangeMinRequiredFormat, egressRulesType),
				)
				allErrs = append(allErrs, portRangeErrors...)
			}
		}

		// Validate ICMP options
		if rule.IcmpOptions != nil && rule.IcmpOptions.Type == nil {
			allErrs = append(allErrs, field.Invalid(
				rulePath.Child("icmpOptions").Child("type"),
				rule.IcmpOptions.Type,
				fmt.Sprintf(icmpTypeRequiredFormat, egressRulesType),
			))
		}

		// Validate destination is required (except for SERVICE_CIDR_BLOCK type)
		if rule.DestinationType != EgressSecurityRuleDestinationTypeServiceCidrBlock && rule.Destination == nil {
			allErrs = append(allErrs, field.Invalid(
				rulePath.Child("destination"),
				rule.Destination,
				fmt.Sprintf(destinationRequiredFormat, egressRulesType),
			))
		}

		// Validate protocol is required
		if rule.Protocol == nil {
			allErrs = append(allErrs, field.Invalid(
				rulePath.Child("protocol"),
				rule.Protocol,
				fmt.Sprintf(protocolRequiredFormat, egressRulesType),
			))
		}

		// Validate CIDR format for CIDR_BLOCK destination type
		if rule.DestinationType == EgressSecurityRuleDestinationTypeCidrBlock && rule.Destination != nil {
			cidrErrors := validateCIDR(
				rule.Destination,
				rulePath.Child("destination"),
				fmt.Sprintf(invalidCIDRFormatFormat, egressRulesType),
			)
			allErrs = append(allErrs, cidrErrors...)
		}
	}

	return allErrs
}

// validateIngressSecurityRuleForNSG validates the Ingress Security Rules for NSG.
func validateIngressSecurityRuleForNSG(ingressRules []IngressSecurityRuleForNSG, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i, r := range ingressRules {
		rulePath := fldPath.Index(i)
		rule := r.IngressSecurityRule

		// Validate UDP options port ranges
		if udpOptions := rule.UdpOptions; udpOptions != nil {
			if udpOptions.DestinationPortRange != nil {
				portRangeErrors := validatePortRange(
					udpOptions.DestinationPortRange,
					rulePath.Child("udpOptions").Child("destinationPortRange"),
					fmt.Sprintf(udpDestinationPortRangeMaxRequiredFormat, ingressRulesType),
					fmt.Sprintf(udpDestinationPortRangeMinRequiredFormat, ingressRulesType),
				)
				allErrs = append(allErrs, portRangeErrors...)
			}

			if udpOptions.SourcePortRange != nil {
				portRangeErrors := validatePortRange(
					udpOptions.SourcePortRange,
					rulePath.Child("udpOptions").Child("sourcePortRange"),
					fmt.Sprintf(udpSourcePortRangeMaxRequiredFormat, ingressRulesType),
					fmt.Sprintf(udpSourcePortRangeMinRequiredFormat, ingressRulesType),
				)
				allErrs = append(allErrs, portRangeErrors...)
			}
		}

		// Validate TCP options port ranges
		if tcpOptions := rule.TcpOptions; tcpOptions != nil {
			if tcpOptions.DestinationPortRange != nil {
				portRangeErrors := validatePortRange(
					tcpOptions.DestinationPortRange,
					rulePath.Child("tcpOptions").Child("destinationPortRange"),
					fmt.Sprintf(tcpDestinationPortRangeMaxRequiredFormat, ingressRulesType),
					fmt.Sprintf(tcpDestinationPortRangeMinRequiredFormat, ingressRulesType),
				)
				allErrs = append(allErrs, portRangeErrors...)
			}

			if tcpOptions.SourcePortRange != nil {
				portRangeErrors := validatePortRange(
					tcpOptions.SourcePortRange,
					rulePath.Child("tcpOptions").Child("sourcePortRange"),
					fmt.Sprintf(tcpSourcePortRangeMaxRequiredFormat, ingressRulesType),
					fmt.Sprintf(tcpSourcePortRangeMinRequiredFormat, ingressRulesType),
				)
				allErrs = append(allErrs, portRangeErrors...)
			}
		}

		// Validate ICMP options
		if rule.IcmpOptions != nil && rule.IcmpOptions.Type == nil {
			allErrs = append(allErrs, field.Invalid(
				rulePath.Child("icmpOptions").Child("type"),
				rule.IcmpOptions.Type,
				fmt.Sprintf(icmpTypeRequiredFormat, ingressRulesType),
			))
		}

		// Validate source is required (except for SERVICE_CIDR_BLOCK type)
		if rule.SourceType != IngressSecurityRuleSourceTypeServiceCidrBlock && rule.Source == nil {
			allErrs = append(allErrs, field.Invalid(
				rulePath.Child("source"),
				rule.Source,
				fmt.Sprintf(sourceRequiredFormat, ingressRulesType),
			))
		}

		// Validate protocol is required
		if rule.Protocol == nil {
			allErrs = append(allErrs, field.Invalid(
				rulePath.Child("protocol"),
				rule.Protocol,
				fmt.Sprintf(protocolRequiredFormat, ingressRulesType),
			))
		}

		// Validate CIDR format for CIDR_BLOCK source type
		if rule.SourceType == IngressSecurityRuleSourceTypeCidrBlock && rule.Source != nil {
			cidrErrors := validateCIDR(
				rule.Source,
				rulePath.Child("source"),
				fmt.Sprintf(invalidCIDRFormatFormat, ingressRulesType),
			)
			allErrs = append(allErrs, cidrErrors...)
		}
	}

	return allErrs
}

// validateSubnets validates a list of Subnets.
func validateSubnets(validRoles []Role, subnets []*Subnet, vcn VCN, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	subnetNames := make(map[string]bool, len(subnets))

	for i, subnet := range subnets {
		if err := validateSubnetName(subnet.Name, fldPath.Index(i).Child("name")); err != nil {
			allErrs = append(allErrs, err)
		}
		if len(subnet.Name) > 0 {
			if _, ok := subnetNames[subnet.Name]; ok {
				allErrs = append(allErrs, field.Duplicate(fldPath, subnet.Name))
			}
			subnetNames[subnet.Name] = true
		}

		if err := validateRole(validRoles, subnet.Role, fldPath.Index(i).Child("role"), "subnet role invalid"); err != nil {
			allErrs = append(allErrs, err)
		}

		subnetCIDRErrors := validateSubnetCIDR(subnet.CIDR, vcn.CIDR, fldPath.Index(i).Child("cidr"))
		allErrs = append(allErrs, subnetCIDRErrors...)
	}

	return allErrs
}

// validateSubnetName validates the Name of a Subnet.
func validateSubnetName(name string, fldPath *field.Path) *field.Error {
	// subnet name can be empty
	if len(name) > 0 {
		if invalidNameRegex.Match([]byte(name)) || name == "" {
			return field.Invalid(fldPath, name,
				fmt.Sprintf("subnet name invalid"))
		}
	}
	return nil
}

// validateRole validates that the subnet role is one of the allowed types
func validateRole(validRoles []Role, subnetRole Role, fldPath *field.Path, errorMsg string) *field.Error {
	for _, role := range validRoles {
		if subnetRole == role {
			return nil
		}
	}
	return field.Invalid(fldPath, subnetRole, errorMsg)
}
