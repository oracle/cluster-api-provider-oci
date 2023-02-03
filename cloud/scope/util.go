/*
Copyright (c) 2021, 2022 Oracle and/or its affiliates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scope

import (
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
)

// GetNsgNamesFromId returns the names of the NSGs with the provided IDs
func GetNsgNamesFromId(ids []string, nsgs []*infrastructurev1beta1.NSG) []string {
	names := make([]string, 0)
	for _, id := range ids {
		for _, nsg := range nsgs {
			if id == *nsg.ID {
				names = append(names, nsg.Name)
			}
		}
	}
	return names
}

// GetSubnetNameFromId returns the name of the Subnet with the provided ID
func GetSubnetNameFromId(id *string, subnets []*infrastructurev1beta1.Subnet) string {
	for _, subnet := range subnets {
		if *id == *subnet.ID {
			return subnet.Name
		}
	}
	return ""
}

// GetSubnetNamesFromId returns the names of the Subnets with the provided IDs
func GetSubnetNamesFromId(ids []string, subnets []*infrastructurev1beta1.Subnet) []string {
	names := make([]string, 0)
	for _, id := range ids {
		for _, subnet := range subnets {
			if id == *subnet.ID {
				names = append(names, subnet.Name)
			}
		}
	}
	return names
}

// ConvertMachineDefinedTags passes in the OCIMachineSpec DefinedTags and returns a converted map of defined tags
// to be used when creating API requests.
func ConvertMachineDefinedTags(machineDefinedTags map[string]map[string]string) map[string]map[string]interface{} {
	definedTags := make(map[string]map[string]interface{})
	if machineDefinedTags != nil {
		for ns, mapNs := range machineDefinedTags {
			mapValues := make(map[string]interface{})
			for k, v := range mapNs {
				mapValues[k] = v
			}
			definedTags[ns] = mapValues
		}
	}

	return definedTags
}
