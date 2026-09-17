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

	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/oracle/oci-go-sdk/v65/core"
)

func unexpectedFakeCall(method string) error {
	return fmt.Errorf("unexpected %s call", method)
}

// unexpectedVCNClient implements VCN operations that are deliberately outside
// the current integration scenarios. Stateful methods on fakeVCNClient take
// precedence over these promoted fallbacks.
type unexpectedVCNClient struct{}

func (unexpectedVCNClient) ListSecurityLists(context.Context, core.ListSecurityListsRequest) (core.ListSecurityListsResponse, error) {
	return core.ListSecurityListsResponse{}, unexpectedFakeCall("ListSecurityLists")
}

func (unexpectedVCNClient) DeleteSecurityList(context.Context, core.DeleteSecurityListRequest) (core.DeleteSecurityListResponse, error) {
	return core.DeleteSecurityListResponse{}, unexpectedFakeCall("DeleteSecurityList")
}

func (unexpectedVCNClient) GetSecurityList(context.Context, core.GetSecurityListRequest) (core.GetSecurityListResponse, error) {
	return core.GetSecurityListResponse{}, unexpectedFakeCall("GetSecurityList")
}

func (unexpectedVCNClient) CreateSecurityList(context.Context, core.CreateSecurityListRequest) (core.CreateSecurityListResponse, error) {
	return core.CreateSecurityListResponse{}, unexpectedFakeCall("CreateSecurityList")
}

func (unexpectedVCNClient) UpdateSecurityList(context.Context, core.UpdateSecurityListRequest) (core.UpdateSecurityListResponse, error) {
	return core.UpdateSecurityListResponse{}, unexpectedFakeCall("UpdateSecurityList")
}

func (unexpectedVCNClient) UpdateVnic(context.Context, core.UpdateVnicRequest) (core.UpdateVnicResponse, error) {
	return core.UpdateVnicResponse{}, unexpectedFakeCall("UpdateVnic")
}

func (unexpectedVCNClient) GetDrg(context.Context, core.GetDrgRequest) (core.GetDrgResponse, error) {
	return core.GetDrgResponse{}, unexpectedFakeCall("GetDrg")
}

func (unexpectedVCNClient) CreateDrg(context.Context, core.CreateDrgRequest) (core.CreateDrgResponse, error) {
	return core.CreateDrgResponse{}, unexpectedFakeCall("CreateDrg")
}

func (unexpectedVCNClient) UpdateDrg(context.Context, core.UpdateDrgRequest) (core.UpdateDrgResponse, error) {
	return core.UpdateDrgResponse{}, unexpectedFakeCall("UpdateDrg")
}

func (unexpectedVCNClient) DeleteDrg(context.Context, core.DeleteDrgRequest) (core.DeleteDrgResponse, error) {
	return core.DeleteDrgResponse{}, unexpectedFakeCall("DeleteDrg")
}

func (unexpectedVCNClient) ListDrgs(context.Context, core.ListDrgsRequest) (core.ListDrgsResponse, error) {
	return core.ListDrgsResponse{}, unexpectedFakeCall("ListDrgs")
}

func (unexpectedVCNClient) ListDrgAttachments(context.Context, core.ListDrgAttachmentsRequest) (core.ListDrgAttachmentsResponse, error) {
	return core.ListDrgAttachmentsResponse{}, unexpectedFakeCall("ListDrgAttachments")
}

func (unexpectedVCNClient) CreateDrgAttachment(context.Context, core.CreateDrgAttachmentRequest) (core.CreateDrgAttachmentResponse, error) {
	return core.CreateDrgAttachmentResponse{}, unexpectedFakeCall("CreateDrgAttachment")
}

func (unexpectedVCNClient) GetDrgAttachment(context.Context, core.GetDrgAttachmentRequest) (core.GetDrgAttachmentResponse, error) {
	return core.GetDrgAttachmentResponse{}, unexpectedFakeCall("GetDrgAttachment")
}

func (unexpectedVCNClient) UpdateDrgAttachment(context.Context, core.UpdateDrgAttachmentRequest) (core.UpdateDrgAttachmentResponse, error) {
	return core.UpdateDrgAttachmentResponse{}, unexpectedFakeCall("UpdateDrgAttachment")
}

func (unexpectedVCNClient) DeleteDrgAttachment(context.Context, core.DeleteDrgAttachmentRequest) (core.DeleteDrgAttachmentResponse, error) {
	return core.DeleteDrgAttachmentResponse{}, unexpectedFakeCall("DeleteDrgAttachment")
}

func (unexpectedVCNClient) GetRemotePeeringConnection(context.Context, core.GetRemotePeeringConnectionRequest) (core.GetRemotePeeringConnectionResponse, error) {
	return core.GetRemotePeeringConnectionResponse{}, unexpectedFakeCall("GetRemotePeeringConnection")
}

func (unexpectedVCNClient) CreateRemotePeeringConnection(context.Context, core.CreateRemotePeeringConnectionRequest) (core.CreateRemotePeeringConnectionResponse, error) {
	return core.CreateRemotePeeringConnectionResponse{}, unexpectedFakeCall("CreateRemotePeeringConnection")
}

func (unexpectedVCNClient) DeleteRemotePeeringConnection(context.Context, core.DeleteRemotePeeringConnectionRequest) (core.DeleteRemotePeeringConnectionResponse, error) {
	return core.DeleteRemotePeeringConnectionResponse{}, unexpectedFakeCall("DeleteRemotePeeringConnection")
}

func (unexpectedVCNClient) UpdateRemotePeeringConnection(context.Context, core.UpdateRemotePeeringConnectionRequest) (core.UpdateRemotePeeringConnectionResponse, error) {
	return core.UpdateRemotePeeringConnectionResponse{}, unexpectedFakeCall("UpdateRemotePeeringConnection")
}

func (unexpectedVCNClient) ListRemotePeeringConnections(context.Context, core.ListRemotePeeringConnectionsRequest) (core.ListRemotePeeringConnectionsResponse, error) {
	return core.ListRemotePeeringConnectionsResponse{}, unexpectedFakeCall("ListRemotePeeringConnections")
}

func (unexpectedVCNClient) ConnectRemotePeeringConnections(context.Context, core.ConnectRemotePeeringConnectionsRequest) (core.ConnectRemotePeeringConnectionsResponse, error) {
	return core.ConnectRemotePeeringConnectionsResponse{}, unexpectedFakeCall("ConnectRemotePeeringConnections")
}

// unexpectedContainerEngineClient implements OKE operations that are outside
// the current integration scenarios. Stateful methods on
// fakeContainerEngineClient take precedence over these promoted fallbacks.
type unexpectedContainerEngineClient struct{}

func (unexpectedContainerEngineClient) GetNodePoolOptions(context.Context, oke.GetNodePoolOptionsRequest) (oke.GetNodePoolOptionsResponse, error) {
	return oke.GetNodePoolOptionsResponse{}, unexpectedFakeCall("GetNodePoolOptions")
}

func (unexpectedContainerEngineClient) ListAddons(context.Context, oke.ListAddonsRequest) (oke.ListAddonsResponse, error) {
	return oke.ListAddonsResponse{}, unexpectedFakeCall("ListAddons")
}

func (unexpectedContainerEngineClient) InstallAddon(context.Context, oke.InstallAddonRequest) (oke.InstallAddonResponse, error) {
	return oke.InstallAddonResponse{}, unexpectedFakeCall("InstallAddon")
}

func (unexpectedContainerEngineClient) UpdateAddon(context.Context, oke.UpdateAddonRequest) (oke.UpdateAddonResponse, error) {
	return oke.UpdateAddonResponse{}, unexpectedFakeCall("UpdateAddon")
}

func (unexpectedContainerEngineClient) DisableAddon(context.Context, oke.DisableAddonRequest) (oke.DisableAddonResponse, error) {
	return oke.DisableAddonResponse{}, unexpectedFakeCall("DisableAddon")
}

func (unexpectedContainerEngineClient) GetAddon(context.Context, oke.GetAddonRequest) (oke.GetAddonResponse, error) {
	return oke.GetAddonResponse{}, unexpectedFakeCall("GetAddon")
}
