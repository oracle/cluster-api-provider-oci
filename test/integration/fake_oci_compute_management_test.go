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
	"strconv"
	"sync"
	"time"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	computemanagementservice "github.com/oracle/cluster-api-provider-oci/cloud/services/computemanagement"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type fakeComputeManagementOperation string

const (
	createInstancePoolOperation    fakeComputeManagementOperation = "create instance pool"
	updateInstancePoolOperation    fakeComputeManagementOperation = "update instance pool"
	terminateInstancePoolOperation fakeComputeManagementOperation = "terminate instance pool"
)

type fakeComputeManagementClient struct {
	mu sync.Mutex

	instanceConfigurations       map[string]core.InstanceConfiguration
	instanceConfigurationOrder   []string
	instancePools                map[string]core.InstancePool
	instanceConfigurationCreates int
	instanceConfigurationDeletes int
	instancePoolCreates          int
	instancePoolUpdates          int
	instancePoolTerminates       int
	instancePoolInspections      int
	operationFailures            map[fakeComputeManagementOperation]error
	operationAttempts            map[fakeComputeManagementOperation]int
}

func newFakeComputeManagementClient() *fakeComputeManagementClient {
	f := &fakeComputeManagementClient{}
	f.reset()
	return f
}

func (f *fakeComputeManagementClient) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.instanceConfigurations = map[string]core.InstanceConfiguration{}
	f.instanceConfigurationOrder = nil
	f.instancePools = map[string]core.InstancePool{}
	f.instanceConfigurationCreates = 0
	f.instanceConfigurationDeletes = 0
	f.instancePoolCreates = 0
	f.instancePoolUpdates = 0
	f.instancePoolTerminates = 0
	f.instancePoolInspections = 0
	f.operationFailures = map[fakeComputeManagementOperation]error{}
	f.operationAttempts = map[fakeComputeManagementOperation]int{}
}

func (f *fakeComputeManagementClient) counts() (configurations, pools, configCreates, configDeletes, poolCreates, poolUpdates, poolTerminates, inspections int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.instanceConfigurations), len(f.instancePools), f.instanceConfigurationCreates, f.instanceConfigurationDeletes, f.instancePoolCreates, f.instancePoolUpdates, f.instancePoolTerminates, f.instancePoolInspections
}

func (f *fakeComputeManagementClient) instancePoolState() (size int, configurationID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, pool := range f.instancePools {
		if pool.Size != nil {
			size = *pool.Size
		}
		configurationID = stringValue(pool.InstanceConfigurationId)
	}
	return size, configurationID
}

func (f *fakeComputeManagementClient) seedInstancePool(pool core.InstancePool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.instancePools[stringValue(pool.Id)] = pool
}

func (f *fakeComputeManagementClient) setInstancePoolState(id string, state core.InstancePoolLifecycleStateEnum) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pool, ok := f.instancePools[id]
	if !ok {
		return
	}
	pool.LifecycleState = state
	f.instancePools[id] = pool
}

func (f *fakeComputeManagementClient) resizeInstancePool(id string, size int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pool, ok := f.instancePools[id]
	if !ok {
		return
	}
	pool.Size = common.Int(size)
	f.instancePools[id] = pool
}

func (f *fakeComputeManagementClient) instancePool(id string) (core.InstancePool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pool, ok := f.instancePools[id]
	return pool, ok
}

func (f *fakeComputeManagementClient) setFailure(operation fakeComputeManagementOperation, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.operationFailures[operation] = err
}

func (f *fakeComputeManagementClient) clearFailure(operation fakeComputeManagementOperation) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.operationFailures, operation)
}

func (f *fakeComputeManagementClient) operationAttemptCount(operation fakeComputeManagementOperation) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.operationAttempts[operation]
}

func (f *fakeComputeManagementClient) operationFailureLocked(operation fakeComputeManagementOperation) error {
	f.operationAttempts[operation]++
	return f.operationFailures[operation]
}

func (f *fakeComputeManagementClient) CreateInstanceConfiguration(_ context.Context, request core.CreateInstanceConfigurationRequest) (core.CreateInstanceConfigurationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.instanceConfigurationCreates++
	id := nextFakeOCID("instanceconfiguration")
	now := common.SDKTime{Time: time.Now().UTC()}
	details, ok := request.CreateInstanceConfiguration.(core.CreateInstanceConfigurationDetails)
	if !ok {
		return core.CreateInstanceConfigurationResponse{}, ociutil.ErrNotFound
	}
	configuration := core.InstanceConfiguration{
		Id:              common.String(id),
		CompartmentId:   details.CompartmentId,
		DisplayName:     details.DisplayName,
		FreeformTags:    cloneStringMap(details.FreeformTags),
		DefinedTags:     cloneDefinedTags(details.DefinedTags),
		InstanceDetails: details.InstanceDetails,
		TimeCreated:     &now,
	}
	f.instanceConfigurations[id] = configuration
	f.instanceConfigurationOrder = append([]string{id}, f.instanceConfigurationOrder...)
	return core.CreateInstanceConfigurationResponse{InstanceConfiguration: configuration}, nil
}

func (f *fakeComputeManagementClient) GetInstanceConfiguration(_ context.Context, request core.GetInstanceConfigurationRequest) (core.GetInstanceConfigurationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.instancePoolInspections++
	configuration, ok := f.instanceConfigurations[stringValue(request.InstanceConfigurationId)]
	if !ok {
		return core.GetInstanceConfigurationResponse{}, ociutil.ErrNotFound
	}
	return core.GetInstanceConfigurationResponse{InstanceConfiguration: configuration}, nil
}

func (f *fakeComputeManagementClient) ListInstanceConfigurations(context.Context, core.ListInstanceConfigurationsRequest) (core.ListInstanceConfigurationsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.instancePoolInspections++
	items := make([]core.InstanceConfigurationSummary, 0, len(f.instanceConfigurationOrder))
	for _, id := range f.instanceConfigurationOrder {
		configuration, ok := f.instanceConfigurations[id]
		if !ok {
			continue
		}
		items = append(items, core.InstanceConfigurationSummary{
			Id:            configuration.Id,
			CompartmentId: configuration.CompartmentId,
			DisplayName:   configuration.DisplayName,
			FreeformTags:  cloneStringMap(configuration.FreeformTags),
			DefinedTags:   cloneDefinedTags(configuration.DefinedTags),
			TimeCreated:   configuration.TimeCreated,
		})
	}
	return core.ListInstanceConfigurationsResponse{Items: items}, nil
}

func (f *fakeComputeManagementClient) DeleteInstanceConfiguration(_ context.Context, request core.DeleteInstanceConfigurationRequest) (core.DeleteInstanceConfigurationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.InstanceConfigurationId)
	for _, pool := range f.instancePools {
		if stringValue(pool.InstanceConfigurationId) == id {
			return core.DeleteInstanceConfigurationResponse{}, fmt.Errorf("instance configuration %q is still referenced by an instance pool", id)
		}
	}
	if _, ok := f.instanceConfigurations[id]; !ok {
		return core.DeleteInstanceConfigurationResponse{}, ociutil.ErrNotFound
	}
	delete(f.instanceConfigurations, id)
	f.instanceConfigurationDeletes++
	return core.DeleteInstanceConfigurationResponse{}, nil
}

func (f *fakeComputeManagementClient) CreateInstancePool(_ context.Context, request core.CreateInstancePoolRequest) (core.CreateInstancePoolResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.operationFailureLocked(createInstancePoolOperation); err != nil {
		return core.CreateInstancePoolResponse{}, err
	}
	f.instancePoolCreates++
	id := nextFakeOCID("instancepool")
	now := common.SDKTime{Time: time.Now().UTC()}
	details := request.CreateInstancePoolDetails
	placements := make([]core.InstancePoolPlacementConfiguration, 0, len(details.PlacementConfigurations))
	for _, placement := range details.PlacementConfigurations {
		placements = append(placements, core.InstancePoolPlacementConfiguration{
			AvailabilityDomain: placement.AvailabilityDomain,
			PrimarySubnetId:    placement.PrimarySubnetId,
			FaultDomains:       append([]string(nil), placement.FaultDomains...),
		})
	}
	pool := core.InstancePool{
		Id:                      common.String(id),
		CompartmentId:           details.CompartmentId,
		InstanceConfigurationId: details.InstanceConfigurationId,
		LifecycleState:          core.InstancePoolLifecycleStateRunning,
		PlacementConfigurations: placements,
		Size:                    details.Size,
		TimeCreated:             &now,
		DisplayName:             details.DisplayName,
		FreeformTags:            cloneStringMap(details.FreeformTags),
	}
	f.instancePools[id] = pool
	return core.CreateInstancePoolResponse{InstancePool: pool}, nil
}

func (f *fakeComputeManagementClient) GetInstancePool(_ context.Context, request core.GetInstancePoolRequest) (core.GetInstancePoolResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.instancePoolInspections++
	pool, ok := f.instancePools[stringValue(request.InstancePoolId)]
	if !ok {
		return core.GetInstancePoolResponse{}, ociutil.ErrNotFound
	}
	return core.GetInstancePoolResponse{InstancePool: pool}, nil
}

func (f *fakeComputeManagementClient) ListInstancePools(_ context.Context, request core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.instancePoolInspections++
	items := make([]core.InstancePoolSummary, 0, len(f.instancePools))
	for _, pool := range f.instancePools {
		if request.DisplayName != nil && stringValue(pool.DisplayName) != stringValue(request.DisplayName) {
			continue
		}
		items = append(items, core.InstancePoolSummary{
			Id:                      pool.Id,
			CompartmentId:           pool.CompartmentId,
			InstanceConfigurationId: pool.InstanceConfigurationId,
			LifecycleState:          core.InstancePoolSummaryLifecycleStateEnum(pool.LifecycleState),
			Size:                    pool.Size,
			TimeCreated:             pool.TimeCreated,
			DisplayName:             pool.DisplayName,
			FreeformTags:            cloneStringMap(pool.FreeformTags),
		})
	}
	return core.ListInstancePoolsResponse{Items: items}, nil
}

func (f *fakeComputeManagementClient) ListInstancePoolInstances(_ context.Context, request core.ListInstancePoolInstancesRequest) (core.ListInstancePoolInstancesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.instancePoolInspections++
	pool, ok := f.instancePools[stringValue(request.InstancePoolId)]
	if !ok {
		return core.ListInstancePoolInstancesResponse{}, ociutil.ErrNotFound
	}
	items := make([]core.InstanceSummary, 0, *pool.Size)
	for i := 0; i < *pool.Size; i++ {
		number := strconv.Itoa(i + 1)
		items = append(items, core.InstanceSummary{
			Id:          common.String("ocid1.instance.oc1..pool-node-" + number),
			DisplayName: common.String("instance-pool-node-" + number),
			State:       common.String("RUNNING"),
		})
	}
	return core.ListInstancePoolInstancesResponse{Items: items}, nil
}

func (f *fakeComputeManagementClient) UpdateInstancePool(_ context.Context, request core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.operationFailureLocked(updateInstancePoolOperation); err != nil {
		return core.UpdateInstancePoolResponse{}, err
	}
	id := stringValue(request.InstancePoolId)
	pool, ok := f.instancePools[id]
	if !ok {
		return core.UpdateInstancePoolResponse{}, ociutil.ErrNotFound
	}
	f.instancePoolUpdates++
	details := request.UpdateInstancePoolDetails
	if details.InstanceConfigurationId != nil {
		pool.InstanceConfigurationId = details.InstanceConfigurationId
	}
	if details.Size != nil {
		pool.Size = details.Size
	}
	if details.FreeformTags != nil {
		pool.FreeformTags = cloneStringMap(details.FreeformTags)
	}
	if len(details.PlacementConfigurations) > 0 {
		placements := make([]core.InstancePoolPlacementConfiguration, 0, len(details.PlacementConfigurations))
		for _, placement := range details.PlacementConfigurations {
			placements = append(placements, core.InstancePoolPlacementConfiguration{
				AvailabilityDomain: placement.AvailabilityDomain,
				PrimarySubnetId:    placement.PrimarySubnetId,
				FaultDomains:       append([]string(nil), placement.FaultDomains...),
			})
		}
		pool.PlacementConfigurations = placements
	}
	f.instancePools[id] = pool
	return core.UpdateInstancePoolResponse{InstancePool: pool}, nil
}

func (f *fakeComputeManagementClient) TerminateInstancePool(_ context.Context, request core.TerminateInstancePoolRequest) (core.TerminateInstancePoolResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.operationFailureLocked(terminateInstancePoolOperation); err != nil {
		return core.TerminateInstancePoolResponse{}, err
	}
	id := stringValue(request.InstancePoolId)
	if _, ok := f.instancePools[id]; !ok {
		return core.TerminateInstancePoolResponse{}, ociutil.ErrNotFound
	}
	delete(f.instancePools, id)
	f.instancePoolTerminates++
	return core.TerminateInstancePoolResponse{}, nil
}

var _ computemanagementservice.Client = &fakeComputeManagementClient{}
