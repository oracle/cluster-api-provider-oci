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
	DeleteApiServerLB(ctx context.Context) error
	DeleteNSGs(ctx context.Context) error
	DeleteSubnets(ctx context.Context) error
	DeleteRouteTables(ctx context.Context) error
	DeleteSecurityLists(ctx context.Context) error
	DeleteServiceGateway(ctx context.Context) error
	DeleteNatGateway(ctx context.Context) error
	DeleteInternetGateway(ctx context.Context) error
	DeleteVCN(ctx context.Context) error
	Close(ctx context.Context) error
	GetOCICluster() *v1beta1.OCICluster
}
