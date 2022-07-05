/*
Copyright 2022 The Kubernetes Authors.

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
	"context"
	"github.com/oracle/cluster-api-provider-oci/api/v1beta1"
)

type ClusterScopeClient interface {
	ReconcileVCN(ctx context.Context) error
	ReconcileInternetGateway(ctx context.Context) error
	ReconcileNatGateway(ctx context.Context) error
	ReconcileServiceGateway(ctx context.Context) error
	ReconcileNSG(ctx context.Context) error
	ReconcileRouteTable(ctx context.Context) error
	ReconcileSubnet(ctx context.Context) error
	ReconcileApiServerLB(ctx context.Context) error
	ReconcileFailureDomains(ctx context.Context) error
	ReconcileDRG(ctx context.Context) error
	DeleteDRG(ctx context.Context) error
	ReconcileDRGVCNAttachment(ctx context.Context) error
	ReconcileDRGRPCAttachment(ctx context.Context) error
	DeleteApiServerLB(ctx context.Context) error
	DeleteNSGs(ctx context.Context) error
	DeleteSubnets(ctx context.Context) error
	DeleteRouteTables(ctx context.Context) error
	DeleteSecurityLists(ctx context.Context) error
	DeleteServiceGateway(ctx context.Context) error
	DeleteNatGateway(ctx context.Context) error
	DeleteInternetGateway(ctx context.Context) error
	DeleteVCN(ctx context.Context) error
	DeleteDRGVCNAttachment(ctx context.Context) error
	DeleteDRGRPCAttachment(ctx context.Context) error
	Close(ctx context.Context) error
	GetOCICluster() *v1beta1.OCICluster
}
