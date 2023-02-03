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

package vcn

import (
	"context"

	"github.com/oracle/oci-go-sdk/v65/core"
)

type Client interface {
	//VCN
	ListVcns(ctx context.Context, request core.ListVcnsRequest) (response core.ListVcnsResponse, err error)
	GetVcn(ctx context.Context, request core.GetVcnRequest) (response core.GetVcnResponse, err error)
	CreateVcn(ctx context.Context, request core.CreateVcnRequest) (response core.CreateVcnResponse, err error)
	UpdateVcn(ctx context.Context, request core.UpdateVcnRequest) (response core.UpdateVcnResponse, err error)
	DeleteVcn(ctx context.Context, request core.DeleteVcnRequest) (response core.DeleteVcnResponse, err error)
	//Subnet
	GetSubnet(ctx context.Context, request core.GetSubnetRequest) (response core.GetSubnetResponse, err error)
	CreateSubnet(ctx context.Context, request core.CreateSubnetRequest) (response core.CreateSubnetResponse, err error)
	UpdateSubnet(ctx context.Context, request core.UpdateSubnetRequest) (response core.UpdateSubnetResponse, err error)
	ListSubnets(ctx context.Context, request core.ListSubnetsRequest) (response core.ListSubnetsResponse, err error)
	DeleteSubnet(ctx context.Context, request core.DeleteSubnetRequest) (response core.DeleteSubnetResponse, err error)
	//RouteTable
	ListRouteTables(ctx context.Context, request core.ListRouteTablesRequest) (response core.ListRouteTablesResponse, err error)
	DeleteRouteTable(ctx context.Context, request core.DeleteRouteTableRequest) (response core.DeleteRouteTableResponse, err error)
	GetRouteTable(ctx context.Context, request core.GetRouteTableRequest) (response core.GetRouteTableResponse, err error)
	CreateRouteTable(ctx context.Context, request core.CreateRouteTableRequest) (response core.CreateRouteTableResponse, err error)
	UpdateRouteTable(ctx context.Context, request core.UpdateRouteTableRequest) (response core.UpdateRouteTableResponse, err error)
	//SecurityList
	ListSecurityLists(ctx context.Context, request core.ListSecurityListsRequest) (response core.ListSecurityListsResponse, err error)
	DeleteSecurityList(ctx context.Context, request core.DeleteSecurityListRequest) (response core.DeleteSecurityListResponse, err error)
	GetSecurityList(ctx context.Context, request core.GetSecurityListRequest) (response core.GetSecurityListResponse, err error)
	CreateSecurityList(ctx context.Context, request core.CreateSecurityListRequest) (response core.CreateSecurityListResponse, err error)
	UpdateSecurityList(ctx context.Context, request core.UpdateSecurityListRequest) (response core.UpdateSecurityListResponse, err error)
	//InternetGateway
	ListInternetGateways(ctx context.Context, request core.ListInternetGatewaysRequest) (response core.ListInternetGatewaysResponse, err error)
	DeleteInternetGateway(ctx context.Context, request core.DeleteInternetGatewayRequest) (response core.DeleteInternetGatewayResponse, err error)
	GetInternetGateway(ctx context.Context, request core.GetInternetGatewayRequest) (response core.GetInternetGatewayResponse, err error)
	UpdateInternetGateway(ctx context.Context, request core.UpdateInternetGatewayRequest) (response core.UpdateInternetGatewayResponse, err error)
	CreateInternetGateway(ctx context.Context, request core.CreateInternetGatewayRequest) (response core.CreateInternetGatewayResponse, err error)
	//NatGateway
	ListNatGateways(ctx context.Context, request core.ListNatGatewaysRequest) (response core.ListNatGatewaysResponse, err error)
	DeleteNatGateway(ctx context.Context, request core.DeleteNatGatewayRequest) (response core.DeleteNatGatewayResponse, err error)
	GetNatGateway(ctx context.Context, request core.GetNatGatewayRequest) (response core.GetNatGatewayResponse, err error)
	UpdateNatGateway(ctx context.Context, request core.UpdateNatGatewayRequest) (response core.UpdateNatGatewayResponse, err error)
	CreateNatGateway(ctx context.Context, request core.CreateNatGatewayRequest) (response core.CreateNatGatewayResponse, err error)
	//ServiceGateway
	ListServiceGateways(ctx context.Context, request core.ListServiceGatewaysRequest) (response core.ListServiceGatewaysResponse, err error)
	DeleteServiceGateway(ctx context.Context, request core.DeleteServiceGatewayRequest) (response core.DeleteServiceGatewayResponse, err error)
	GetServiceGateway(ctx context.Context, request core.GetServiceGatewayRequest) (response core.GetServiceGatewayResponse, err error)
	UpdateServiceGateway(ctx context.Context, request core.UpdateServiceGatewayRequest) (response core.UpdateServiceGatewayResponse, err error)
	CreateServiceGateway(ctx context.Context, request core.CreateServiceGatewayRequest) (response core.CreateServiceGatewayResponse, err error)
	//Service
	ListServices(ctx context.Context, request core.ListServicesRequest) (response core.ListServicesResponse, err error)
	// Vnic
	GetVnic(ctx context.Context, request core.GetVnicRequest) (response core.GetVnicResponse, err error)
	UpdateVnic(ctx context.Context, request core.UpdateVnicRequest) (response core.UpdateVnicResponse, err error)
	// NSG
	GetNetworkSecurityGroup(ctx context.Context, request core.GetNetworkSecurityGroupRequest) (response core.GetNetworkSecurityGroupResponse, err error)
	ListNetworkSecurityGroups(ctx context.Context, request core.ListNetworkSecurityGroupsRequest) (response core.ListNetworkSecurityGroupsResponse, err error)
	CreateNetworkSecurityGroup(ctx context.Context, request core.CreateNetworkSecurityGroupRequest) (response core.CreateNetworkSecurityGroupResponse, err error)
	UpdateNetworkSecurityGroup(ctx context.Context, request core.UpdateNetworkSecurityGroupRequest) (response core.UpdateNetworkSecurityGroupResponse, err error)
	AddNetworkSecurityGroupSecurityRules(ctx context.Context, request core.AddNetworkSecurityGroupSecurityRulesRequest) (response core.AddNetworkSecurityGroupSecurityRulesResponse, err error)
	UpdateNetworkSecurityGroupSecurityRules(ctx context.Context, request core.UpdateNetworkSecurityGroupSecurityRulesRequest) (response core.UpdateNetworkSecurityGroupSecurityRulesResponse, err error)
	DeleteNetworkSecurityGroup(ctx context.Context, request core.DeleteNetworkSecurityGroupRequest) (response core.DeleteNetworkSecurityGroupResponse, err error)
	ListNetworkSecurityGroupSecurityRules(ctx context.Context, request core.ListNetworkSecurityGroupSecurityRulesRequest) (response core.ListNetworkSecurityGroupSecurityRulesResponse, err error)
	RemoveNetworkSecurityGroupSecurityRules(ctx context.Context, request core.RemoveNetworkSecurityGroupSecurityRulesRequest) (response core.RemoveNetworkSecurityGroupSecurityRulesResponse, err error)
	// Dynamic Routing Gateways (DRG)
	GetDrg(ctx context.Context, request core.GetDrgRequest) (response core.GetDrgResponse, err error)
	CreateDrg(ctx context.Context, request core.CreateDrgRequest) (response core.CreateDrgResponse, err error)
	UpdateDrg(ctx context.Context, request core.UpdateDrgRequest) (response core.UpdateDrgResponse, err error)
	DeleteDrg(ctx context.Context, request core.DeleteDrgRequest) (response core.DeleteDrgResponse, err error)
	ListDrgs(ctx context.Context, request core.ListDrgsRequest) (response core.ListDrgsResponse, err error)
	ListDrgAttachments(ctx context.Context, request core.ListDrgAttachmentsRequest) (response core.ListDrgAttachmentsResponse, err error)
	CreateDrgAttachment(ctx context.Context, request core.CreateDrgAttachmentRequest) (response core.CreateDrgAttachmentResponse, err error)
	GetDrgAttachment(ctx context.Context, request core.GetDrgAttachmentRequest) (response core.GetDrgAttachmentResponse, err error)
	UpdateDrgAttachment(ctx context.Context, request core.UpdateDrgAttachmentRequest) (response core.UpdateDrgAttachmentResponse, err error)
	DeleteDrgAttachment(ctx context.Context, request core.DeleteDrgAttachmentRequest) (response core.DeleteDrgAttachmentResponse, err error)
	GetRemotePeeringConnection(ctx context.Context, request core.GetRemotePeeringConnectionRequest) (response core.GetRemotePeeringConnectionResponse, err error)
	CreateRemotePeeringConnection(ctx context.Context, request core.CreateRemotePeeringConnectionRequest) (response core.CreateRemotePeeringConnectionResponse, err error)
	DeleteRemotePeeringConnection(ctx context.Context, request core.DeleteRemotePeeringConnectionRequest) (response core.DeleteRemotePeeringConnectionResponse, err error)
	UpdateRemotePeeringConnection(ctx context.Context, request core.UpdateRemotePeeringConnectionRequest) (response core.UpdateRemotePeeringConnectionResponse, err error)
	ListRemotePeeringConnections(ctx context.Context, request core.ListRemotePeeringConnectionsRequest) (response core.ListRemotePeeringConnectionsResponse, err error)
	ConnectRemotePeeringConnections(ctx context.Context, request core.ConnectRemotePeeringConnectionsRequest) (response core.ConnectRemotePeeringConnectionsResponse, err error)
}
