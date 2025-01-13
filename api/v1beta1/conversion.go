/*
 Copyright (c) 2023 Oracle and/or its affiliates.

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

package v1beta1

import (
	"github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"k8s.io/apimachinery/pkg/conversion"
)

// Convert_v1beta1_VCN_To_v1beta2_VCN converts v1beta1 VCN to v1beta2 VCN
func Convert_v1beta1_VCN_To_v1beta2_VCN(in *VCN, out *v1beta2.VCN, s conversion.Scope) error {
	err := autoConvert_v1beta1_VCN_To_v1beta2_VCN(in, out, s)
	if err != nil {
		return err
	}
	if in.InternetGatewayId != nil {
		out.InternetGateway.Id = in.InternetGatewayId
	}
	if in.NatGatewayId != nil {
		out.NATGateway.Id = in.NatGatewayId
	}
	if in.ServiceGatewayId != nil {
		out.ServiceGateway.Id = in.ServiceGatewayId
	}
	if in.PrivateRouteTableId != nil {
		out.RouteTable.PrivateRouteTableId = in.PrivateRouteTableId
	}
	if in.PublicRouteTableId != nil {
		out.RouteTable.PublicRouteTableId = in.PublicRouteTableId
	}
	if in.NetworkSecurityGroups != nil {
		nsgList, err := convertv1beta1NSGListTov1beta2NSGList(in.NetworkSecurityGroups)
		if err != nil {
			return err
		}
		out.NetworkSecurityGroup.List = nsgList
	}
	return nil
}

// Convert_v1beta2_VCN_To_v1beta1_VCN converts v1beta2 VCN to v1beta1 VCN
func Convert_v1beta2_VCN_To_v1beta1_VCN(in *v1beta2.VCN, out *VCN, s conversion.Scope) error {
	err := autoConvert_v1beta2_VCN_To_v1beta1_VCN(in, out, s)
	if err != nil {
		return err
	}
	if in.InternetGateway.Id != nil {
		out.InternetGatewayId = in.InternetGateway.Id
	}
	if in.NATGateway.Id != nil {
		out.NatGatewayId = in.NATGateway.Id
	}
	if in.ServiceGateway.Id != nil {
		out.ServiceGatewayId = in.ServiceGateway.Id
	}
	if in.RouteTable.PublicRouteTableId != nil {
		out.PublicRouteTableId = in.RouteTable.PublicRouteTableId
	}
	if in.RouteTable.PrivateRouteTableId != nil {
		out.PrivateRouteTableId = in.RouteTable.PrivateRouteTableId
	}
	if in.NetworkSecurityGroup.List != nil {
		nsgList, err := convertv1beta2NSGListTov1beta1NSGList(in.NetworkSecurityGroup.List)
		if err != nil {
			return err
		}
		out.NetworkSecurityGroups = nsgList
	}
	return nil
}

// Convert_v1beta1_OCIClusterStatus_To_v1beta2_OCIClusterStatus converts v1beta1 OCIClusterStatus to v1beta2 OCIClusterStatus
func Convert_v1beta1_OCIClusterStatus_To_v1beta2_OCIClusterStatus(in *OCIClusterStatus, out *v1beta2.OCIClusterStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_OCIClusterStatus_To_v1beta2_OCIClusterStatus(in, out, s)
}

// Convert_v1beta2_OCIClusterSpec_To_v1beta1_OCIClusterSpec converts v1beta2 OCIClusterStatus to v1beta1 OCIClusterStatus
func Convert_v1beta2_OCIClusterSpec_To_v1beta1_OCIClusterSpec(in *v1beta2.OCIClusterSpec, out *OCIClusterSpec, s conversion.Scope) error {
	return autoConvert_v1beta2_OCIClusterSpec_To_v1beta1_OCIClusterSpec(in, out, s)
}

// Convert_v1beta1_EgressSecurityRuleForNSG_To_v1beta2_EgressSecurityRuleForNSG converts v1beta1 EgressSecurityRuleForNSG to v1beta2 EgressSecurityRuleForNSG
func Convert_v1beta1_EgressSecurityRuleForNSG_To_v1beta2_EgressSecurityRuleForNSG(in *EgressSecurityRuleForNSG, out *v1beta2.EgressSecurityRuleForNSG, s conversion.Scope) error {
	return autoConvert_v1beta1_EgressSecurityRuleForNSG_To_v1beta2_EgressSecurityRuleForNSG(in, out, s)
}

// Convert_v1beta1_IngressSecurityRuleForNSG_To_v1beta2_IngressSecurityRuleForNSG converts v1beta1 IngressSecurityRuleForNSG to v1beta2 IngressSecurityRuleForNSG
func Convert_v1beta1_IngressSecurityRuleForNSG_To_v1beta2_IngressSecurityRuleForNSG(in *IngressSecurityRuleForNSG, out *v1beta2.IngressSecurityRuleForNSG, s conversion.Scope) error {
	return autoConvert_v1beta1_IngressSecurityRuleForNSG_To_v1beta2_IngressSecurityRuleForNSG(in, out, s)
}

// Convert_v1beta1_NetworkDetails_To_v1beta2_NetworkDetails converts v1beta1 NetworkDetails to v1beta2 NetworkDetails
func Convert_v1beta1_NetworkDetails_To_v1beta2_NetworkDetails(in *NetworkDetails, out *v1beta2.NetworkDetails, s conversion.Scope) error {
	return autoConvert_v1beta1_NetworkDetails_To_v1beta2_NetworkDetails(in, out, s)
}

// Convert_v1beta1_OCIMachineSpec_To_v1beta2_OCIMachineSpec converts v1beta1 OCIMachineSpec to v1beta2 OCIMachineSpec
func Convert_v1beta1_OCIMachineSpec_To_v1beta2_OCIMachineSpec(in *OCIMachineSpec, out *v1beta2.OCIMachineSpec, s conversion.Scope) error {
	err := autoConvert_v1beta1_OCIMachineSpec_To_v1beta2_OCIMachineSpec(in, out, s)
	if err != nil {
		return err
	}
	if in.NSGName != "" && len(in.NetworkDetails.NsgNames) == 0 {
		out.NetworkDetails.NsgNames = []string{in.NSGName}
	}
	return nil
}

// Convert_v1beta2_LoadBalancer_To_v1beta1_LoadBalancer converts v1beta2 LoadBalancer to v1beta1 LoadBalancer
func Convert_v1beta2_LoadBalancer_To_v1beta1_LoadBalancer(in *v1beta2.LoadBalancer, out *LoadBalancer, s conversion.Scope) error {
	return autoConvert_v1beta2_LoadBalancer_To_v1beta1_LoadBalancer(in, out, s)
}

func Convert_v1beta2_OCIManagedControlPlaneStatus_To_v1beta1_OCIManagedControlPlaneStatus(in *v1beta2.OCIManagedControlPlaneStatus, out *OCIManagedControlPlaneStatus, s conversion.Scope) error {
	return autoConvert_v1beta2_OCIManagedControlPlaneStatus_To_v1beta1_OCIManagedControlPlaneStatus(in, out, s)
}

func Convert_v1beta2_OCIManagedControlPlaneSpec_To_v1beta1_OCIManagedControlPlaneSpec(in *v1beta2.OCIManagedControlPlaneSpec, out *OCIManagedControlPlaneSpec, s conversion.Scope) error {
	return autoConvert_v1beta2_OCIManagedControlPlaneSpec_To_v1beta1_OCIManagedControlPlaneSpec(in, out, s)
}

// Convert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus converts v1beta1 OCIManagedClusterStatus to v1beta2 OCIManagedClusterStatus
func Convert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus(in *OCIManagedClusterStatus, out *v1beta2.OCIManagedClusterStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus(in, out, s)
}

// Convert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec converts v1beta1 OCIManagedClusterSpec to v1beta2 OCIManagedClusterSpec
func Convert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(in *v1beta2.OCIManagedClusterSpec, out *OCIManagedClusterSpec, s conversion.Scope) error {
	return autoConvert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(in, out, s)
}

// Convert_v1beta2_ClusterOptions_To_v1beta1_ClusterOptions converts v1beta2 ClusterOptions to v1beta1 ClusterOptions
func Convert_v1beta2_ClusterOptions_To_v1beta1_ClusterOptions(in *v1beta2.ClusterOptions, out *ClusterOptions, s conversion.Scope) error {
	return autoConvert_v1beta2_ClusterOptions_To_v1beta1_ClusterOptions(in, out, s)
}
