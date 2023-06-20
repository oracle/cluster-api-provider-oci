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

package containerengine

import (
	"context"

	"github.com/oracle/oci-go-sdk/v65/containerengine"
)

type Client interface {
	//Cluster
	CreateCluster(ctx context.Context, request containerengine.CreateClusterRequest) (response containerengine.CreateClusterResponse, err error)
	GetCluster(ctx context.Context, request containerengine.GetClusterRequest) (response containerengine.GetClusterResponse, err error)
	UpdateCluster(ctx context.Context, request containerengine.UpdateClusterRequest) (response containerengine.UpdateClusterResponse, err error)
	ListClusters(ctx context.Context, request containerengine.ListClustersRequest) (response containerengine.ListClustersResponse, err error)
	DeleteCluster(ctx context.Context, request containerengine.DeleteClusterRequest) (response containerengine.DeleteClusterResponse, err error)
	CreateKubeconfig(ctx context.Context, request containerengine.CreateKubeconfigRequest) (response containerengine.CreateKubeconfigResponse, err error)
	//NodePool
	DeleteNodePool(ctx context.Context, request containerengine.DeleteNodePoolRequest) (response containerengine.DeleteNodePoolResponse, err error)
	CreateNodePool(ctx context.Context, request containerengine.CreateNodePoolRequest) (response containerengine.CreateNodePoolResponse, err error)
	UpdateNodePool(ctx context.Context, request containerengine.UpdateNodePoolRequest) (response containerengine.UpdateNodePoolResponse, err error)
	GetNodePool(ctx context.Context, request containerengine.GetNodePoolRequest) (response containerengine.GetNodePoolResponse, err error)
	ListNodePools(ctx context.Context, request containerengine.ListNodePoolsRequest) (response containerengine.ListNodePoolsResponse, err error)
	//NodePool Options
	GetNodePoolOptions(ctx context.Context, request containerengine.GetNodePoolOptionsRequest) (response containerengine.GetNodePoolOptionsResponse, err error)
	//VirtualNodePool
	DeleteVirtualNodePool(ctx context.Context, request containerengine.DeleteVirtualNodePoolRequest) (response containerengine.DeleteVirtualNodePoolResponse, err error)
	CreateVirtualNodePool(ctx context.Context, request containerengine.CreateVirtualNodePoolRequest) (response containerengine.CreateVirtualNodePoolResponse, err error)
	UpdateVirtualNodePool(ctx context.Context, request containerengine.UpdateVirtualNodePoolRequest) (response containerengine.UpdateVirtualNodePoolResponse, err error)
	GetVirtualNodePool(ctx context.Context, request containerengine.GetVirtualNodePoolRequest) (response containerengine.GetVirtualNodePoolResponse, err error)
	ListVirtualNodePools(ctx context.Context, request containerengine.ListVirtualNodePoolsRequest) (response containerengine.ListVirtualNodePoolsResponse, err error)
	ListVirtualNodes(ctx context.Context, request containerengine.ListVirtualNodesRequest) (response containerengine.ListVirtualNodesResponse, err error)

	//Work Request
	GetWorkRequest(ctx context.Context, request containerengine.GetWorkRequestRequest) (response containerengine.GetWorkRequestResponse, err error)

	// Addons
	ListAddons(ctx context.Context, request containerengine.ListAddonsRequest) (response containerengine.ListAddonsResponse, err error)
	InstallAddon(ctx context.Context, request containerengine.InstallAddonRequest) (response containerengine.InstallAddonResponse, err error)
	UpdateAddon(ctx context.Context, request containerengine.UpdateAddonRequest) (response containerengine.UpdateAddonResponse, err error)
	DisableAddon(ctx context.Context, request containerengine.DisableAddonRequest) (response containerengine.DisableAddonResponse, err error)
	GetAddon(ctx context.Context, request containerengine.GetAddonRequest) (response containerengine.GetAddonResponse, err error)
}
