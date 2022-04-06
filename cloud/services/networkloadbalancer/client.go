/*
 Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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

package nlb

import (
	"context"
	"github.com/oracle/oci-go-sdk/v63/networkloadbalancer"
)

type Client interface {
	ListNetworkLoadBalancers(ctx context.Context, request networkloadbalancer.ListNetworkLoadBalancersRequest) (response networkloadbalancer.ListNetworkLoadBalancersResponse, err error)
	GetNetworkLoadBalancer(ctx context.Context, request networkloadbalancer.GetNetworkLoadBalancerRequest) (response networkloadbalancer.GetNetworkLoadBalancerResponse, err error)
	CreateBackend(ctx context.Context, request networkloadbalancer.CreateBackendRequest) (response networkloadbalancer.CreateBackendResponse, err error)
	CreateNetworkLoadBalancer(ctx context.Context, request networkloadbalancer.CreateNetworkLoadBalancerRequest) (response networkloadbalancer.CreateNetworkLoadBalancerResponse, err error)
	DeleteBackend(ctx context.Context, request networkloadbalancer.DeleteBackendRequest) (response networkloadbalancer.DeleteBackendResponse, err error)
	GetWorkRequest(ctx context.Context, request networkloadbalancer.GetWorkRequestRequest) (response networkloadbalancer.GetWorkRequestResponse, err error)
	UpdateNetworkLoadBalancer(ctx context.Context, request networkloadbalancer.UpdateNetworkLoadBalancerRequest) (response networkloadbalancer.UpdateNetworkLoadBalancerResponse, err error)
	DeleteNetworkLoadBalancer(ctx context.Context, request networkloadbalancer.DeleteNetworkLoadBalancerRequest) (response networkloadbalancer.DeleteNetworkLoadBalancerResponse, err error)
}
