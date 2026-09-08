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

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	identityservice "github.com/oracle/cluster-api-provider-oci/cloud/services/identity"
	nlbservice "github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

type fakeIdentityClient struct {
	mu sync.Mutex

	regionListCount             int
	availabilityDomainListCount int
	faultDomainListCount        int
}

func (f *fakeIdentityClient) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.regionListCount = 0
	f.availabilityDomainListCount = 0
	f.faultDomainListCount = 0
}

func (f *fakeIdentityClient) counts() (regions, availabilityDomains, faultDomains int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.regionListCount, f.availabilityDomainListCount, f.faultDomainListCount
}

func (f *fakeIdentityClient) ListRegions(context.Context) (identity.ListRegionsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.regionListCount++
	return identity.ListRegionsResponse{Items: []identity.Region{{
		Key:  common.String("LEX"),
		Name: common.String(scope.MockTestRegion),
	}}}, nil
}

func (f *fakeIdentityClient) ListAvailabilityDomains(context.Context, identity.ListAvailabilityDomainsRequest) (identity.ListAvailabilityDomainsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.availabilityDomainListCount++
	return identity.ListAvailabilityDomainsResponse{Items: []identity.AvailabilityDomain{{
		Name: common.String("integration-ad-1"),
	}}}, nil
}

func (f *fakeIdentityClient) ListFaultDomains(context.Context, identity.ListFaultDomainsRequest) (identity.ListFaultDomainsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.faultDomainListCount++
	return identity.ListFaultDomainsResponse{Items: []identity.FaultDomain{
		{Name: common.String("FAULT-DOMAIN-1")},
		{Name: common.String("FAULT-DOMAIN-2")},
		{Name: common.String("FAULT-DOMAIN-3")},
	}}, nil
}

var _ identityservice.Client = &fakeIdentityClient{}

type fakeNetworkLoadBalancerClient struct {
	mu sync.Mutex

	nextID        int
	loadBalancers map[string]networkloadbalancer.NetworkLoadBalancer
	createCount   int
	deleteCount   int
}

func newFakeNetworkLoadBalancerClient() *fakeNetworkLoadBalancerClient {
	f := &fakeNetworkLoadBalancerClient{}
	f.reset()
	return f
}

func (f *fakeNetworkLoadBalancerClient) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID = 0
	f.loadBalancers = map[string]networkloadbalancer.NetworkLoadBalancer{}
	f.createCount = 0
	f.deleteCount = 0
}

func (f *fakeNetworkLoadBalancerClient) counts() (active, creates, deletes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.loadBalancers), f.createCount, f.deleteCount
}

func (f *fakeNetworkLoadBalancerClient) ListNetworkLoadBalancers(_ context.Context, request networkloadbalancer.ListNetworkLoadBalancersRequest) (networkloadbalancer.ListNetworkLoadBalancersResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]networkloadbalancer.NetworkLoadBalancerSummary, 0, len(f.loadBalancers))
	for _, loadBalancer := range f.loadBalancers {
		if request.DisplayName != nil && stringValue(loadBalancer.DisplayName) != stringValue(request.DisplayName) {
			continue
		}
		items = append(items, networkloadbalancer.NetworkLoadBalancerSummary{
			Id:           loadBalancer.Id,
			DisplayName:  loadBalancer.DisplayName,
			FreeformTags: cloneStringMap(loadBalancer.FreeformTags),
		})
	}
	return networkloadbalancer.ListNetworkLoadBalancersResponse{
		NetworkLoadBalancerCollection: networkloadbalancer.NetworkLoadBalancerCollection{Items: items},
	}, nil
}

func (f *fakeNetworkLoadBalancerClient) GetNetworkLoadBalancer(_ context.Context, request networkloadbalancer.GetNetworkLoadBalancerRequest) (networkloadbalancer.GetNetworkLoadBalancerResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	loadBalancer, ok := f.loadBalancers[stringValue(request.NetworkLoadBalancerId)]
	if !ok {
		return networkloadbalancer.GetNetworkLoadBalancerResponse{}, ociutil.ErrNotFound
	}
	return networkloadbalancer.GetNetworkLoadBalancerResponse{NetworkLoadBalancer: loadBalancer}, nil
}

func (f *fakeNetworkLoadBalancerClient) CreateNetworkLoadBalancer(_ context.Context, request networkloadbalancer.CreateNetworkLoadBalancerRequest) (networkloadbalancer.CreateNetworkLoadBalancerResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.createCount++
	id := fmt.Sprintf("ocid1.networkloadbalancer.oc1..integration-%d", f.nextID)
	details := request.CreateNetworkLoadBalancerDetails
	backendSets := make(map[string]networkloadbalancer.BackendSet, len(details.BackendSets))
	for name, backendSet := range details.BackendSets {
		backendSets[name] = networkloadbalancer.BackendSet{
			Name:                     common.String(name),
			HealthChecker:            backendSet.HealthChecker,
			Policy:                   backendSet.Policy,
			IsPreserveSource:         backendSet.IsPreserveSource,
			IsFailOpen:               backendSet.IsFailOpen,
			IsInstantFailoverEnabled: backendSet.IsInstantFailoverEnabled,
			Backends:                 backendSet.Backends,
		}
	}
	isPrivate := details.IsPrivate != nil && *details.IsPrivate
	loadBalancer := networkloadbalancer.NetworkLoadBalancer{
		Id:                      common.String(id),
		CompartmentId:           details.CompartmentId,
		DisplayName:             details.DisplayName,
		LifecycleState:          networkloadbalancer.LifecycleStateActive,
		IpAddresses:             []networkloadbalancer.IpAddress{{IpAddress: common.String("203.0.113.10"), IsPublic: common.Bool(!isPrivate)}},
		SubnetId:                details.SubnetId,
		IsPrivate:               details.IsPrivate,
		NetworkSecurityGroupIds: append([]string(nil), details.NetworkSecurityGroupIds...),
		BackendSets:             backendSets,
		FreeformTags:            cloneStringMap(details.FreeformTags),
	}
	f.loadBalancers[id] = loadBalancer
	return networkloadbalancer.CreateNetworkLoadBalancerResponse{
		NetworkLoadBalancer: loadBalancer,
		OpcWorkRequestId:    common.String("work-request-create-" + id),
	}, nil
}

func (f *fakeNetworkLoadBalancerClient) DeleteNetworkLoadBalancer(_ context.Context, request networkloadbalancer.DeleteNetworkLoadBalancerRequest) (networkloadbalancer.DeleteNetworkLoadBalancerResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.NetworkLoadBalancerId)
	if _, ok := f.loadBalancers[id]; !ok {
		return networkloadbalancer.DeleteNetworkLoadBalancerResponse{}, ociutil.ErrNotFound
	}
	delete(f.loadBalancers, id)
	f.deleteCount++
	return networkloadbalancer.DeleteNetworkLoadBalancerResponse{
		OpcWorkRequestId: common.String("work-request-delete-" + id),
	}, nil
}

func (*fakeNetworkLoadBalancerClient) GetWorkRequest(_ context.Context, request networkloadbalancer.GetWorkRequestRequest) (networkloadbalancer.GetWorkRequestResponse, error) {
	return networkloadbalancer.GetWorkRequestResponse{WorkRequest: networkloadbalancer.WorkRequest{
		Id:              request.WorkRequestId,
		CompartmentId:   common.String("ocid1.compartment.oc1..integration"),
		Status:          networkloadbalancer.OperationStatusSucceeded,
		PercentComplete: common.Float32(100),
	}}, nil
}

func (*fakeNetworkLoadBalancerClient) ListWorkRequestErrors(context.Context, networkloadbalancer.ListWorkRequestErrorsRequest) (networkloadbalancer.ListWorkRequestErrorsResponse, error) {
	return networkloadbalancer.ListWorkRequestErrorsResponse{}, nil
}

func (*fakeNetworkLoadBalancerClient) CreateBackend(context.Context, networkloadbalancer.CreateBackendRequest) (networkloadbalancer.CreateBackendResponse, error) {
	return networkloadbalancer.CreateBackendResponse{}, fmt.Errorf("unexpected CreateBackend call")
}

func (*fakeNetworkLoadBalancerClient) DeleteBackend(context.Context, networkloadbalancer.DeleteBackendRequest) (networkloadbalancer.DeleteBackendResponse, error) {
	return networkloadbalancer.DeleteBackendResponse{}, fmt.Errorf("unexpected DeleteBackend call")
}

func (*fakeNetworkLoadBalancerClient) UpdateNetworkLoadBalancer(context.Context, networkloadbalancer.UpdateNetworkLoadBalancerRequest) (networkloadbalancer.UpdateNetworkLoadBalancerResponse, error) {
	return networkloadbalancer.UpdateNetworkLoadBalancerResponse{}, fmt.Errorf("unexpected UpdateNetworkLoadBalancer call")
}

func (*fakeNetworkLoadBalancerClient) UpdateHealthChecker(context.Context, networkloadbalancer.UpdateHealthCheckerRequest) (networkloadbalancer.UpdateHealthCheckerResponse, error) {
	return networkloadbalancer.UpdateHealthCheckerResponse{}, fmt.Errorf("unexpected UpdateHealthChecker call")
}

var _ nlbservice.NetworkLoadBalancerClient = &fakeNetworkLoadBalancerClient{}
