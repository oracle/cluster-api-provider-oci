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
	"sync"

	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type fakeOCIBackend struct {
	compute  *fakeComputeClient
	vcn      *fakeVCNClient
	identity *fakeIdentityClient
	nlb      *fakeNetworkLoadBalancerClient
	oke      *fakeContainerEngineClient
	base     *fakeBaseClient
}

func newFakeOCIBackend() *fakeOCIBackend {
	return &fakeOCIBackend{
		compute:  newFakeComputeClient(),
		vcn:      newFakeVCNClient(),
		identity: &fakeIdentityClient{},
		nlb:      newFakeNetworkLoadBalancerClient(),
		oke:      newFakeContainerEngineClient(),
		base:     &fakeBaseClient{},
	}
}

func (f *fakeOCIBackend) reset() {
	f.compute.reset()
	f.vcn.reset()
	f.identity.reset()
	f.nlb.reset()
	f.oke.reset()
	f.base.reset()
}

func (f *fakeOCIBackend) clientProvider() (*scope.ClientProvider, error) {
	return scope.MockNewClientProvider(scope.MockOCIClients{
		ComputeClient:             f.compute,
		VCNClient:                 f.vcn,
		IdentityClient:            f.identity,
		NetworkLoadBalancerClient: f.nlb,
		ContainerEngineClient:     f.oke,
		BaseClient:                f.base,
	})
}

type fakeComputeClient struct {
	mu sync.Mutex

	instances       map[string]core.Instance
	launchCount     int
	terminateCount  int
	inspectionCount int
}

func newFakeComputeClient() *fakeComputeClient {
	f := &fakeComputeClient{}
	f.reset()
	return f
}

func (f *fakeComputeClient) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.instances = map[string]core.Instance{}
	f.launchCount = 0
	f.terminateCount = 0
	f.inspectionCount = 0
}

func (f *fakeComputeClient) counts() (launches, terminations, inspections int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.launchCount, f.terminateCount, f.inspectionCount
}

func (f *fakeComputeClient) LaunchInstance(_ context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.launchCount++
	instanceID := fmt.Sprintf("ocid1.instance.oc1..integration-%d", f.launchCount)
	instance := core.Instance{
		Id:                 common.String(instanceID),
		DisplayName:        request.LaunchInstanceDetails.DisplayName,
		CompartmentId:      request.LaunchInstanceDetails.CompartmentId,
		AvailabilityDomain: request.LaunchInstanceDetails.AvailabilityDomain,
		FaultDomain:        request.LaunchInstanceDetails.FaultDomain,
		FreeformTags:       cloneStringMap(request.LaunchInstanceDetails.FreeformTags),
		LifecycleState:     core.InstanceLifecycleStateRunning,
	}
	f.instances[instanceID] = instance
	return core.LaunchInstanceResponse{Instance: instance}, nil
}

func (f *fakeComputeClient) TerminateInstance(_ context.Context, request core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	instance, ok := f.instances[stringValue(request.InstanceId)]
	if !ok {
		return core.TerminateInstanceResponse{}, fmt.Errorf("terminate unknown instance %q", stringValue(request.InstanceId))
	}
	f.terminateCount++
	instance.LifecycleState = core.InstanceLifecycleStateTerminated
	f.instances[stringValue(request.InstanceId)] = instance
	return core.TerminateInstanceResponse{}, nil
}

func (f *fakeComputeClient) GetInstance(_ context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.inspectionCount++
	instance, ok := f.instances[stringValue(request.InstanceId)]
	if !ok {
		return core.GetInstanceResponse{}, fmt.Errorf("get unknown instance %q", stringValue(request.InstanceId))
	}
	return core.GetInstanceResponse{Instance: instance}, nil
}

func (f *fakeComputeClient) ListInstances(_ context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.inspectionCount++
	instances := make([]core.Instance, 0, len(f.instances))
	for _, instance := range f.instances {
		if request.DisplayName != nil && stringValue(instance.DisplayName) != stringValue(request.DisplayName) {
			continue
		}
		instances = append(instances, instance)
	}
	return core.ListInstancesResponse{Items: instances}, nil
}

func (*fakeComputeClient) AttachVolume(context.Context, core.AttachVolumeRequest) (core.AttachVolumeResponse, error) {
	return core.AttachVolumeResponse{}, fmt.Errorf("unexpected AttachVolume call")
}

func (*fakeComputeClient) DetachVolume(context.Context, core.DetachVolumeRequest) (core.DetachVolumeResponse, error) {
	return core.DetachVolumeResponse{}, fmt.Errorf("unexpected DetachVolume call")
}

func (*fakeComputeClient) ListVolumeAttachments(context.Context, core.ListVolumeAttachmentsRequest) (core.ListVolumeAttachmentsResponse, error) {
	return core.ListVolumeAttachmentsResponse{}, fmt.Errorf("unexpected ListVolumeAttachments call")
}

func (*fakeComputeClient) AttachVnic(context.Context, core.AttachVnicRequest) (core.AttachVnicResponse, error) {
	return core.AttachVnicResponse{}, fmt.Errorf("unexpected AttachVnic call")
}

func (*fakeComputeClient) ListVnicAttachments(_ context.Context, request core.ListVnicAttachmentsRequest) (core.ListVnicAttachmentsResponse, error) {
	instanceID := stringValue(request.InstanceId)
	attachmentID := "vnic-attachment-" + instanceID
	vnicID := "vnic-" + instanceID
	return core.ListVnicAttachmentsResponse{Items: []core.VnicAttachment{
		{
			Id:             common.String(attachmentID),
			VnicId:         common.String(vnicID),
			LifecycleState: core.VnicAttachmentLifecycleStateAttached,
		},
	}}, nil
}

type fakeVCNClient struct {
	vcn.Client

	mu sync.Mutex

	getVNICCount          int
	nextID                int
	vcns                  map[string]core.Vcn
	internetGateways      map[string]core.InternetGateway
	natGateways           map[string]core.NatGateway
	serviceGateways       map[string]core.ServiceGateway
	networkSecurityGroups map[string]core.NetworkSecurityGroup
	routeTables           map[string]core.RouteTable
	subnets               map[string]core.Subnet
	createCounts          map[string]int
	deleteOrder           []string
}

func newFakeVCNClient() *fakeVCNClient {
	f := &fakeVCNClient{}
	f.reset()
	return f
}

func (f *fakeVCNClient) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getVNICCount = 0
	f.nextID = 0
	f.vcns = map[string]core.Vcn{}
	f.internetGateways = map[string]core.InternetGateway{}
	f.natGateways = map[string]core.NatGateway{}
	f.serviceGateways = map[string]core.ServiceGateway{}
	f.networkSecurityGroups = map[string]core.NetworkSecurityGroup{}
	f.routeTables = map[string]core.RouteTable{}
	f.subnets = map[string]core.Subnet{}
	f.createCounts = map[string]int{}
	f.deleteOrder = nil
}

func (f *fakeVCNClient) getVNICCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getVNICCount
}

func (f *fakeVCNClient) GetVnic(_ context.Context, request core.GetVnicRequest) (core.GetVnicResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getVNICCount++
	return core.GetVnicResponse{Vnic: core.Vnic{
		Id:        request.VnicId,
		IsPrimary: common.Bool(true),
		PrivateIp: common.String("10.0.0.10"),
	}}, nil
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var (
	_ compute.ComputeClient = &fakeComputeClient{}
	_ vcn.Client            = &fakeVCNClient{}
)
