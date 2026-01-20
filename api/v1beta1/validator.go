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

package v1beta1

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
)

// invalidNameRegex is a broad regex used to validate allows names in OCI
var invalidNameRegex = regexp.MustCompile("\\s")

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
		allErrs = append(allErrs, validateVCNCIDR(networkSpec.Vcn.CIDR, fldPath.Child("cidr"))...)
	}

	if networkSpec.Vcn.Subnets != nil {
		allErrs = append(allErrs, validateSubnets(validRoles, networkSpec.Vcn.Subnets, networkSpec.Vcn, fldPath.Child("subnets"))...)
	}

	if networkSpec.Vcn.NetworkSecurityGroups != nil {
		allErrs = append(allErrs, validateNSGs(validRoles, networkSpec.Vcn.NetworkSecurityGroups, fldPath.Child("networkSecurityGroups"))...)
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
		allErrs = append(allErrs, validateEgressSecurityRuleForNSG(nsg.EgressRules, fldPath.Index(i).Child("egressRules"))...)
		allErrs = append(allErrs, validateIngressSecurityRuleForNSG(nsg.IngressRules, fldPath.Index(i).Child("ingressRules"))...)
	}

	return allErrs
}

// validateEgressSecurityRuleForNSG validates the Egress Security Rule of a Subnet.
func validateEgressSecurityRuleForNSG(egressRules []EgressSecurityRuleForNSG, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for _, r := range egressRules {
		rule := r.EgressSecurityRule

		if rule.DestinationType == EgressSecurityRuleDestinationTypeCidrBlock && rule.Destination != nil {
			if _, _, err := net.ParseCIDR(ociutil.DerefString(rule.Destination)); err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath, rule.Destination, "invalid egressRules CIDR format"))
			}
		}
	}

	return allErrs
}

// validateEgressSecurityRuleForNSG validates the Egress Security Rule of a Subnet.
func validateIngressSecurityRuleForNSG(egressRules []IngressSecurityRuleForNSG, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for _, r := range egressRules {
		rule := r.IngressSecurityRule

		if rule.SourceType == IngressSecurityRuleSourceTypeCidrBlock && rule.Source != nil {
			if _, _, err := net.ParseCIDR(ociutil.DerefString(rule.Source)); err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath, rule.Source, "invalid ingressRule CIDR format"))
			}
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

		allErrs = append(allErrs, validateSubnetCIDR(subnet.CIDR, vcn.CIDR, fldPath.Index(i).Child("cidr"))...)
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
