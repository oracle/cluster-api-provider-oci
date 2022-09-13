package computemanagement

import (
	"context"

	"github.com/oracle/oci-go-sdk/v65/core"
)

type Client interface {
	// Instance Pool
	CreateInstancePool(ctx context.Context, request core.CreateInstancePoolRequest) (response core.CreateInstancePoolResponse, err error)
	GetInstancePool(ctx context.Context, request core.GetInstancePoolRequest) (response core.GetInstancePoolResponse, err error)
	TerminateInstancePool(ctx context.Context, request core.TerminateInstancePoolRequest) (response core.TerminateInstancePoolResponse, err error)
	UpdateInstancePool(ctx context.Context, request core.UpdateInstancePoolRequest) (response core.UpdateInstancePoolResponse, err error)
	ListInstancePools(ctx context.Context, request core.ListInstancePoolsRequest) (response core.ListInstancePoolsResponse, err error)
	ListInstancePoolInstances(ctx context.Context, request core.ListInstancePoolInstancesRequest) (response core.ListInstancePoolInstancesResponse, err error)

	// Instance Configuration
	CreateInstanceConfiguration(ctx context.Context, request core.CreateInstanceConfigurationRequest) (response core.CreateInstanceConfigurationResponse, err error)
	GetInstanceConfiguration(ctx context.Context, request core.GetInstanceConfigurationRequest) (response core.GetInstanceConfigurationResponse, err error)
	ListInstanceConfigurations(ctx context.Context, request core.ListInstanceConfigurationsRequest) (response core.ListInstanceConfigurationsResponse, err error)
	DeleteInstanceConfiguration(ctx context.Context, request core.DeleteInstanceConfigurationRequest) (response core.DeleteInstanceConfigurationResponse, err error)
}
