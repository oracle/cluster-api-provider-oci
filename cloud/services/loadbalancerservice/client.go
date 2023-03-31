/*
 Copyright (c) 2022 Oracle and/or its affiliates.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package lbs

import (
	"context"

	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

type LoadBalancerServiceClient interface {
	ListLoadBalancers(ctx context.Context, request loadbalancer.ListLoadBalancersRequest) (response loadbalancer.ListLoadBalancersResponse, err error)
	GetLoadBalancer(ctx context.Context, request loadbalancer.GetLoadBalancerRequest) (response loadbalancer.GetLoadBalancerResponse, err error)
	CreateBackend(ctx context.Context, request loadbalancer.CreateBackendRequest) (response loadbalancer.CreateBackendResponse, err error)
	CreateLoadBalancer(ctx context.Context, request loadbalancer.CreateLoadBalancerRequest) (response loadbalancer.CreateLoadBalancerResponse, err error)
	DeleteBackend(ctx context.Context, request loadbalancer.DeleteBackendRequest) (response loadbalancer.DeleteBackendResponse, err error)
	GetWorkRequest(ctx context.Context, request loadbalancer.GetWorkRequestRequest) (response loadbalancer.GetWorkRequestResponse, err error)
	UpdateLoadBalancer(ctx context.Context, request loadbalancer.UpdateLoadBalancerRequest) (response loadbalancer.UpdateLoadBalancerResponse, err error)
	DeleteLoadBalancer(ctx context.Context, request loadbalancer.DeleteLoadBalancerRequest) (response loadbalancer.DeleteLoadBalancerResponse, err error)
}
