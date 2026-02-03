package scope

import (
	"context"
)

type ClusterScopeClient interface {
	ReconcileBlockVolume(ctx context.Context) error
	ReconcileVCN(ctx context.Context) error
	ReconcileInternetGateway(ctx context.Context) error
	ReconcileNatGateway(ctx context.Context) error
	ReconcileServiceGateway(ctx context.Context) error
	ReconcileNSG(ctx context.Context) error
	ReconcileRouteTable(ctx context.Context) error
	ReconcileSubnet(ctx context.Context) error
	ReconcileApiServerNLB(ctx context.Context) error
	ReconcileApiServerLB(ctx context.Context) error
	ReconcileFailureDomains(ctx context.Context) error
	ReconcileDRG(ctx context.Context) error
	DeleteDRG(ctx context.Context) error
	ReconcileDRGVCNAttachment(ctx context.Context) error
	ReconcileDRGRPCAttachment(ctx context.Context) error
	DeleteApiServerNLB(ctx context.Context) error
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
	GetOCIClusterAccessor() OCIClusterAccessor
	SetRegionKey(ctx context.Context) error
}
