package v1beta1

import (
	"github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"k8s.io/apimachinery/pkg/conversion"
)

func Convert_v1beta1_VCN_To_v1beta2_VCN(in *VCN, out *v1beta2.VCN, s conversion.Scope) error {
	autoConvert_v1beta1_VCN_To_v1beta2_VCN(in, out, s)
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
		out.NetworkSecurityGroups.NSGList = nsgList
	}
	return nil
}

func Convert_v1beta2_VCN_To_v1beta1_VCN(in *v1beta2.VCN, out *VCN, s conversion.Scope) error {
	return autoConvert_v1beta2_VCN_To_v1beta1_VCN(in, out, s)
}
