//go:build integration

/*
Copyright (c) 2026 Oracle and/or its affiliates.

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

package integration_test

import (
	"context"
	"fmt"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type fakeVCNOperation string

const (
	createSubnetOperation fakeVCNOperation = "create subnet"
	deleteVCNOperation    fakeVCNOperation = "delete VCN"
)

type fakeNetworkCounts struct {
	VCNs             int
	InternetGateways int
	NATGateways      int
	ServiceGateways  int
	NSGs             int
	RouteTables      int
	Subnets          int
	Creates          map[string]int
	DeleteOrder      []string
}

func (f *fakeVCNClient) networkCounts() fakeNetworkCounts {
	f.mu.Lock()
	defer f.mu.Unlock()
	creates := make(map[string]int, len(f.createCounts))
	for resource, count := range f.createCounts {
		creates[resource] = count
	}
	return fakeNetworkCounts{
		VCNs:             len(f.vcns),
		InternetGateways: len(f.internetGateways),
		NATGateways:      len(f.natGateways),
		ServiceGateways:  len(f.serviceGateways),
		NSGs:             len(f.networkSecurityGroups),
		RouteTables:      len(f.routeTables),
		Subnets:          len(f.subnets),
		Creates:          creates,
		DeleteOrder:      append([]string(nil), f.deleteOrder...),
	}
}

func (f *fakeVCNClient) nextResourceIDLocked(resource string) *string {
	f.nextID++
	f.createCounts[resource]++
	return common.String(fmt.Sprintf("ocid1.%s.oc1..integration-%d", resource, f.nextID))
}

func (f *fakeVCNClient) setFailure(operation fakeVCNOperation, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.operationFailures[operation] = err
}

func (f *fakeVCNClient) clearFailure(operation fakeVCNOperation) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.operationFailures, operation)
}

func (f *fakeVCNClient) operationAttemptCount(operation fakeVCNOperation) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.operationAttempts[operation]
}

func (f *fakeVCNClient) operationFailureLocked(operation fakeVCNOperation) error {
	f.operationAttempts[operation]++
	return f.operationFailures[operation]
}

func (f *fakeVCNClient) seedVCN(vcn core.Vcn) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vcns[stringValue(vcn.Id)] = vcn
}

func (f *fakeVCNClient) ListVcns(_ context.Context, request core.ListVcnsRequest) (core.ListVcnsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]core.Vcn, 0, len(f.vcns))
	for _, vcn := range f.vcns {
		if request.DisplayName != nil && stringValue(vcn.DisplayName) != stringValue(request.DisplayName) {
			continue
		}
		items = append(items, vcn)
	}
	return core.ListVcnsResponse{Items: items}, nil
}

func (f *fakeVCNClient) GetVcn(_ context.Context, request core.GetVcnRequest) (core.GetVcnResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	vcn, ok := f.vcns[stringValue(request.VcnId)]
	if !ok {
		return core.GetVcnResponse{}, ociutil.ErrNotFound
	}
	return core.GetVcnResponse{Vcn: vcn}, nil
}

func (f *fakeVCNClient) CreateVcn(_ context.Context, request core.CreateVcnRequest) (core.CreateVcnResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextResourceIDLocked("vcn")
	details := request.CreateVcnDetails
	vcn := core.Vcn{
		Id:             id,
		CompartmentId:  details.CompartmentId,
		DisplayName:    details.DisplayName,
		CidrBlocks:     append([]string(nil), details.CidrBlocks...),
		Ipv6CidrBlocks: nil,
		FreeformTags:   cloneStringMap(details.FreeformTags),
		LifecycleState: core.VcnLifecycleStateAvailable,
	}
	f.vcns[*id] = vcn
	return core.CreateVcnResponse{Vcn: vcn}, nil
}

func (f *fakeVCNClient) UpdateVcn(_ context.Context, request core.UpdateVcnRequest) (core.UpdateVcnResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.VcnId)
	vcn, ok := f.vcns[id]
	if !ok {
		return core.UpdateVcnResponse{}, ociutil.ErrNotFound
	}
	vcn.DisplayName = request.UpdateVcnDetails.DisplayName
	f.vcns[id] = vcn
	return core.UpdateVcnResponse{Vcn: vcn}, nil
}

func (f *fakeVCNClient) DeleteVcn(_ context.Context, request core.DeleteVcnRequest) (core.DeleteVcnResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.operationFailureLocked(deleteVCNOperation); err != nil {
		return core.DeleteVcnResponse{}, err
	}
	if len(f.internetGateways)+len(f.natGateways)+len(f.serviceGateways)+len(f.networkSecurityGroups)+len(f.routeTables)+len(f.subnets) != 0 {
		return core.DeleteVcnResponse{}, fmt.Errorf("cannot delete VCN before dependent resources")
	}
	id := stringValue(request.VcnId)
	if _, ok := f.vcns[id]; !ok {
		return core.DeleteVcnResponse{}, ociutil.ErrNotFound
	}
	delete(f.vcns, id)
	f.deleteOrder = append(f.deleteOrder, "vcn")
	return core.DeleteVcnResponse{}, nil
}

func (f *fakeVCNClient) ListInternetGateways(_ context.Context, request core.ListInternetGatewaysRequest) (core.ListInternetGatewaysResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]core.InternetGateway, 0, len(f.internetGateways))
	for _, gateway := range f.internetGateways {
		if request.DisplayName != nil && stringValue(gateway.DisplayName) != stringValue(request.DisplayName) {
			continue
		}
		items = append(items, gateway)
	}
	return core.ListInternetGatewaysResponse{Items: items}, nil
}

func (f *fakeVCNClient) GetInternetGateway(_ context.Context, request core.GetInternetGatewayRequest) (core.GetInternetGatewayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	gateway, ok := f.internetGateways[stringValue(request.IgId)]
	if !ok {
		return core.GetInternetGatewayResponse{}, ociutil.ErrNotFound
	}
	return core.GetInternetGatewayResponse{InternetGateway: gateway}, nil
}

func (f *fakeVCNClient) CreateInternetGateway(_ context.Context, request core.CreateInternetGatewayRequest) (core.CreateInternetGatewayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextResourceIDLocked("internetgateway")
	details := request.CreateInternetGatewayDetails
	gateway := core.InternetGateway{
		Id:             id,
		CompartmentId:  details.CompartmentId,
		DisplayName:    details.DisplayName,
		IsEnabled:      details.IsEnabled,
		VcnId:          details.VcnId,
		FreeformTags:   cloneStringMap(details.FreeformTags),
		LifecycleState: core.InternetGatewayLifecycleStateAvailable,
	}
	f.internetGateways[*id] = gateway
	return core.CreateInternetGatewayResponse{InternetGateway: gateway}, nil
}

func (f *fakeVCNClient) UpdateInternetGateway(_ context.Context, request core.UpdateInternetGatewayRequest) (core.UpdateInternetGatewayResponse, error) {
	return core.UpdateInternetGatewayResponse{}, fmt.Errorf("unexpected UpdateInternetGateway call for %q", stringValue(request.IgId))
}

func (f *fakeVCNClient) DeleteInternetGateway(_ context.Context, request core.DeleteInternetGatewayRequest) (core.DeleteInternetGatewayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.routeTables) != 0 {
		return core.DeleteInternetGatewayResponse{}, fmt.Errorf("cannot delete internet gateway before route tables")
	}
	id := stringValue(request.IgId)
	if _, ok := f.internetGateways[id]; !ok {
		return core.DeleteInternetGatewayResponse{}, ociutil.ErrNotFound
	}
	delete(f.internetGateways, id)
	f.deleteOrder = append(f.deleteOrder, "internetgateway")
	return core.DeleteInternetGatewayResponse{}, nil
}

func (f *fakeVCNClient) ListNatGateways(_ context.Context, request core.ListNatGatewaysRequest) (core.ListNatGatewaysResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]core.NatGateway, 0, len(f.natGateways))
	for _, gateway := range f.natGateways {
		if request.DisplayName != nil && stringValue(gateway.DisplayName) != stringValue(request.DisplayName) {
			continue
		}
		items = append(items, gateway)
	}
	return core.ListNatGatewaysResponse{Items: items}, nil
}

func (f *fakeVCNClient) GetNatGateway(_ context.Context, request core.GetNatGatewayRequest) (core.GetNatGatewayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	gateway, ok := f.natGateways[stringValue(request.NatGatewayId)]
	if !ok {
		return core.GetNatGatewayResponse{}, ociutil.ErrNotFound
	}
	return core.GetNatGatewayResponse{NatGateway: gateway}, nil
}

func (f *fakeVCNClient) CreateNatGateway(_ context.Context, request core.CreateNatGatewayRequest) (core.CreateNatGatewayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextResourceIDLocked("natgateway")
	details := request.CreateNatGatewayDetails
	gateway := core.NatGateway{
		Id:             id,
		CompartmentId:  details.CompartmentId,
		DisplayName:    details.DisplayName,
		VcnId:          details.VcnId,
		FreeformTags:   cloneStringMap(details.FreeformTags),
		LifecycleState: core.NatGatewayLifecycleStateAvailable,
	}
	f.natGateways[*id] = gateway
	return core.CreateNatGatewayResponse{NatGateway: gateway}, nil
}

func (f *fakeVCNClient) UpdateNatGateway(_ context.Context, request core.UpdateNatGatewayRequest) (core.UpdateNatGatewayResponse, error) {
	return core.UpdateNatGatewayResponse{}, fmt.Errorf("unexpected UpdateNatGateway call for %q", stringValue(request.NatGatewayId))
}

func (f *fakeVCNClient) DeleteNatGateway(_ context.Context, request core.DeleteNatGatewayRequest) (core.DeleteNatGatewayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.routeTables) != 0 {
		return core.DeleteNatGatewayResponse{}, fmt.Errorf("cannot delete NAT gateway before route tables")
	}
	id := stringValue(request.NatGatewayId)
	if _, ok := f.natGateways[id]; !ok {
		return core.DeleteNatGatewayResponse{}, ociutil.ErrNotFound
	}
	delete(f.natGateways, id)
	f.deleteOrder = append(f.deleteOrder, "natgateway")
	return core.DeleteNatGatewayResponse{}, nil
}

func (*fakeVCNClient) ListServices(context.Context, core.ListServicesRequest) (core.ListServicesResponse, error) {
	return core.ListServicesResponse{Items: []core.Service{{
		Id:        common.String("ocid1.service.oc1..integration"),
		CidrBlock: common.String("all-lex-services-in-oracle-services-network"),
	}}}, nil
}

func (f *fakeVCNClient) ListServiceGateways(_ context.Context, _ core.ListServiceGatewaysRequest) (core.ListServiceGatewaysResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]core.ServiceGateway, 0, len(f.serviceGateways))
	for _, gateway := range f.serviceGateways {
		items = append(items, gateway)
	}
	return core.ListServiceGatewaysResponse{Items: items}, nil
}

func (f *fakeVCNClient) GetServiceGateway(_ context.Context, request core.GetServiceGatewayRequest) (core.GetServiceGatewayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	gateway, ok := f.serviceGateways[stringValue(request.ServiceGatewayId)]
	if !ok {
		return core.GetServiceGatewayResponse{}, ociutil.ErrNotFound
	}
	return core.GetServiceGatewayResponse{ServiceGateway: gateway}, nil
}

func (f *fakeVCNClient) CreateServiceGateway(_ context.Context, request core.CreateServiceGatewayRequest) (core.CreateServiceGatewayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextResourceIDLocked("servicegateway")
	details := request.CreateServiceGatewayDetails
	services := make([]core.ServiceIdResponseDetails, 0, len(details.Services))
	for _, service := range details.Services {
		services = append(services, core.ServiceIdResponseDetails{ServiceId: service.ServiceId})
	}
	gateway := core.ServiceGateway{
		Id:             id,
		CompartmentId:  details.CompartmentId,
		DisplayName:    details.DisplayName,
		VcnId:          details.VcnId,
		Services:       services,
		FreeformTags:   cloneStringMap(details.FreeformTags),
		LifecycleState: core.ServiceGatewayLifecycleStateAvailable,
	}
	f.serviceGateways[*id] = gateway
	return core.CreateServiceGatewayResponse{ServiceGateway: gateway}, nil
}

func (f *fakeVCNClient) UpdateServiceGateway(_ context.Context, request core.UpdateServiceGatewayRequest) (core.UpdateServiceGatewayResponse, error) {
	return core.UpdateServiceGatewayResponse{}, fmt.Errorf("unexpected UpdateServiceGateway call for %q", stringValue(request.ServiceGatewayId))
}

func (f *fakeVCNClient) DeleteServiceGateway(_ context.Context, request core.DeleteServiceGatewayRequest) (core.DeleteServiceGatewayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.routeTables) != 0 {
		return core.DeleteServiceGatewayResponse{}, fmt.Errorf("cannot delete service gateway before route tables")
	}
	id := stringValue(request.ServiceGatewayId)
	if _, ok := f.serviceGateways[id]; !ok {
		return core.DeleteServiceGatewayResponse{}, ociutil.ErrNotFound
	}
	delete(f.serviceGateways, id)
	f.deleteOrder = append(f.deleteOrder, "servicegateway")
	return core.DeleteServiceGatewayResponse{}, nil
}

func (f *fakeVCNClient) ListNetworkSecurityGroups(_ context.Context, request core.ListNetworkSecurityGroupsRequest) (core.ListNetworkSecurityGroupsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]core.NetworkSecurityGroup, 0, len(f.networkSecurityGroups))
	for _, group := range f.networkSecurityGroups {
		if request.DisplayName != nil && stringValue(group.DisplayName) != stringValue(request.DisplayName) {
			continue
		}
		items = append(items, group)
	}
	return core.ListNetworkSecurityGroupsResponse{Items: items}, nil
}

func (f *fakeVCNClient) GetNetworkSecurityGroup(_ context.Context, request core.GetNetworkSecurityGroupRequest) (core.GetNetworkSecurityGroupResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	group, ok := f.networkSecurityGroups[stringValue(request.NetworkSecurityGroupId)]
	if !ok {
		return core.GetNetworkSecurityGroupResponse{}, ociutil.ErrNotFound
	}
	return core.GetNetworkSecurityGroupResponse{NetworkSecurityGroup: group}, nil
}

func (f *fakeVCNClient) CreateNetworkSecurityGroup(_ context.Context, request core.CreateNetworkSecurityGroupRequest) (core.CreateNetworkSecurityGroupResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextResourceIDLocked("networksecuritygroup")
	details := request.CreateNetworkSecurityGroupDetails
	group := core.NetworkSecurityGroup{
		Id:             id,
		CompartmentId:  details.CompartmentId,
		DisplayName:    details.DisplayName,
		VcnId:          details.VcnId,
		FreeformTags:   cloneStringMap(details.FreeformTags),
		LifecycleState: core.NetworkSecurityGroupLifecycleStateAvailable,
	}
	f.networkSecurityGroups[*id] = group
	return core.CreateNetworkSecurityGroupResponse{NetworkSecurityGroup: group}, nil
}

func (f *fakeVCNClient) UpdateNetworkSecurityGroup(_ context.Context, request core.UpdateNetworkSecurityGroupRequest) (core.UpdateNetworkSecurityGroupResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.NetworkSecurityGroupId)
	group, ok := f.networkSecurityGroups[id]
	if !ok {
		return core.UpdateNetworkSecurityGroupResponse{}, ociutil.ErrNotFound
	}
	group.DisplayName = request.UpdateNetworkSecurityGroupDetails.DisplayName
	f.networkSecurityGroups[id] = group
	return core.UpdateNetworkSecurityGroupResponse{NetworkSecurityGroup: group}, nil
}

func (f *fakeVCNClient) DeleteNetworkSecurityGroup(_ context.Context, request core.DeleteNetworkSecurityGroupRequest) (core.DeleteNetworkSecurityGroupResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.NetworkSecurityGroupId)
	if _, ok := f.networkSecurityGroups[id]; !ok {
		return core.DeleteNetworkSecurityGroupResponse{}, ociutil.ErrNotFound
	}
	delete(f.networkSecurityGroups, id)
	f.deleteOrder = append(f.deleteOrder, "networksecuritygroup")
	return core.DeleteNetworkSecurityGroupResponse{}, nil
}

func (*fakeVCNClient) ListNetworkSecurityGroupSecurityRules(context.Context, core.ListNetworkSecurityGroupSecurityRulesRequest) (core.ListNetworkSecurityGroupSecurityRulesResponse, error) {
	return core.ListNetworkSecurityGroupSecurityRulesResponse{Items: []core.SecurityRule{}}, nil
}

func (*fakeVCNClient) AddNetworkSecurityGroupSecurityRules(context.Context, core.AddNetworkSecurityGroupSecurityRulesRequest) (core.AddNetworkSecurityGroupSecurityRulesResponse, error) {
	return core.AddNetworkSecurityGroupSecurityRulesResponse{}, fmt.Errorf("unexpected AddNetworkSecurityGroupSecurityRules call")
}

func (*fakeVCNClient) UpdateNetworkSecurityGroupSecurityRules(context.Context, core.UpdateNetworkSecurityGroupSecurityRulesRequest) (core.UpdateNetworkSecurityGroupSecurityRulesResponse, error) {
	return core.UpdateNetworkSecurityGroupSecurityRulesResponse{}, fmt.Errorf("unexpected UpdateNetworkSecurityGroupSecurityRules call")
}

func (*fakeVCNClient) RemoveNetworkSecurityGroupSecurityRules(context.Context, core.RemoveNetworkSecurityGroupSecurityRulesRequest) (core.RemoveNetworkSecurityGroupSecurityRulesResponse, error) {
	return core.RemoveNetworkSecurityGroupSecurityRulesResponse{}, fmt.Errorf("unexpected RemoveNetworkSecurityGroupSecurityRules call")
}

func (f *fakeVCNClient) ListRouteTables(_ context.Context, request core.ListRouteTablesRequest) (core.ListRouteTablesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]core.RouteTable, 0, len(f.routeTables))
	for _, routeTable := range f.routeTables {
		if request.DisplayName != nil && stringValue(routeTable.DisplayName) != stringValue(request.DisplayName) {
			continue
		}
		items = append(items, routeTable)
	}
	return core.ListRouteTablesResponse{Items: items}, nil
}

func (f *fakeVCNClient) GetRouteTable(_ context.Context, request core.GetRouteTableRequest) (core.GetRouteTableResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	routeTable, ok := f.routeTables[stringValue(request.RtId)]
	if !ok {
		return core.GetRouteTableResponse{}, ociutil.ErrNotFound
	}
	return core.GetRouteTableResponse{RouteTable: routeTable}, nil
}

func (f *fakeVCNClient) CreateRouteTable(_ context.Context, request core.CreateRouteTableRequest) (core.CreateRouteTableResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextResourceIDLocked("routetable")
	details := request.CreateRouteTableDetails
	routeTable := core.RouteTable{
		Id:            id,
		CompartmentId: details.CompartmentId,
		DisplayName:   details.DisplayName,
		VcnId:         details.VcnId,
		RouteRules:    append([]core.RouteRule(nil), details.RouteRules...),
		FreeformTags:  cloneStringMap(details.FreeformTags),
	}
	f.routeTables[*id] = routeTable
	return core.CreateRouteTableResponse{RouteTable: routeTable}, nil
}

func (f *fakeVCNClient) UpdateRouteTable(_ context.Context, request core.UpdateRouteTableRequest) (core.UpdateRouteTableResponse, error) {
	return core.UpdateRouteTableResponse{}, fmt.Errorf("unexpected UpdateRouteTable call for %q", stringValue(request.RtId))
}

func (f *fakeVCNClient) DeleteRouteTable(_ context.Context, request core.DeleteRouteTableRequest) (core.DeleteRouteTableResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.subnets) != 0 {
		return core.DeleteRouteTableResponse{}, fmt.Errorf("cannot delete route table before subnets")
	}
	id := stringValue(request.RtId)
	if _, ok := f.routeTables[id]; !ok {
		return core.DeleteRouteTableResponse{}, ociutil.ErrNotFound
	}
	delete(f.routeTables, id)
	f.deleteOrder = append(f.deleteOrder, "routetable")
	return core.DeleteRouteTableResponse{}, nil
}

func (f *fakeVCNClient) ListSubnets(_ context.Context, request core.ListSubnetsRequest) (core.ListSubnetsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]core.Subnet, 0, len(f.subnets))
	for _, subnet := range f.subnets {
		if request.DisplayName != nil && stringValue(subnet.DisplayName) != stringValue(request.DisplayName) {
			continue
		}
		items = append(items, subnet)
	}
	return core.ListSubnetsResponse{Items: items}, nil
}

func (f *fakeVCNClient) GetSubnet(_ context.Context, request core.GetSubnetRequest) (core.GetSubnetResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	subnet, ok := f.subnets[stringValue(request.SubnetId)]
	if !ok {
		return core.GetSubnetResponse{}, ociutil.ErrNotFound
	}
	return core.GetSubnetResponse{Subnet: subnet}, nil
}

func (f *fakeVCNClient) CreateSubnet(_ context.Context, request core.CreateSubnetRequest) (core.CreateSubnetResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.operationFailureLocked(createSubnetOperation); err != nil {
		return core.CreateSubnetResponse{}, err
	}
	id := f.nextResourceIDLocked("subnet")
	details := request.CreateSubnetDetails
	subnet := core.Subnet{
		Id:                      id,
		CompartmentId:           details.CompartmentId,
		DisplayName:             details.DisplayName,
		VcnId:                   details.VcnId,
		CidrBlock:               details.CidrBlock,
		RouteTableId:            details.RouteTableId,
		SecurityListIds:         append([]string(nil), details.SecurityListIds...),
		ProhibitInternetIngress: details.ProhibitInternetIngress,
		ProhibitPublicIpOnVnic:  details.ProhibitPublicIpOnVnic,
		FreeformTags:            cloneStringMap(details.FreeformTags),
		LifecycleState:          core.SubnetLifecycleStateAvailable,
	}
	f.subnets[*id] = subnet
	return core.CreateSubnetResponse{Subnet: subnet}, nil
}

func (f *fakeVCNClient) UpdateSubnet(_ context.Context, request core.UpdateSubnetRequest) (core.UpdateSubnetResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.SubnetId)
	subnet, ok := f.subnets[id]
	if !ok {
		return core.UpdateSubnetResponse{}, ociutil.ErrNotFound
	}
	subnet.DisplayName = request.UpdateSubnetDetails.DisplayName
	subnet.CidrBlock = request.UpdateSubnetDetails.CidrBlock
	subnet.SecurityListIds = append([]string(nil), request.UpdateSubnetDetails.SecurityListIds...)
	f.subnets[id] = subnet
	return core.UpdateSubnetResponse{Subnet: subnet}, nil
}

func (f *fakeVCNClient) DeleteSubnet(_ context.Context, request core.DeleteSubnetRequest) (core.DeleteSubnetResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.SubnetId)
	if _, ok := f.subnets[id]; !ok {
		return core.DeleteSubnetResponse{}, ociutil.ErrNotFound
	}
	delete(f.subnets, id)
	f.deleteOrder = append(f.deleteOrder, "subnet")
	return core.DeleteSubnetResponse{}, nil
}
