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

package scope

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/loadbalancer/mock_lb"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLBReconciliation(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		lbClient           *mock_lb.MockLoadBalancerClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		lbClient = mock_lb.NewMockLoadBalancerClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociClusterAccessor = OCISelfManagedCluster{
			&infrastructurev1beta2.OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					UID:  "a",
					Name: "cluster",
				},
				Spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId:         "compartment-id",
					OCIResourceIdentifier: "resource_uid",
				},
			},
		}
		ociClusterAccessor.OCICluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			LoadBalancerClient: lbClient,
			Cluster:            &clusterv1.Cluster{},
			OCIClusterAccessor: ociClusterAccessor,
			Client:             client,
		})
		tags = make(map[string]string)
		tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
		tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name                string
		errorExpected       bool
		objects             []client.Object
		expectedEvent       string
		eventNotExpected    string
		matchError          error
		errorSubStringMatch bool
		testSpecificSetup   func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient)
	}{
		{
			name:          "lb exists",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:             common.String("lb-id"),
							LifecycleState: loadbalancer.LoadBalancerLifecycleStateActive,
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
			},
		},
		{
			name:          "lb does not have ip address",
			errorExpected: true,
			matchError:    errors.New("lb does not have valid ip addresses"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:             common.String("lb-id"),
							LifecycleState: loadbalancer.LoadBalancerLifecycleStateActive,
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						},
					}, nil)
			},
		},
		{
			name:          "lb does not have public ip address",
			errorExpected: true,
			matchError:    errors.New("lb does not have valid public ip address"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:             common.String("lb-id"),
							LifecycleState: loadbalancer.LoadBalancerLifecycleStateActive,
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(false),
								},
							},
						},
					}, nil)
			},
		},
		{
			name:          "lb lookup by display name",
			errorExpected: true,
			matchError:    errors.New("lb does not have valid public ip address"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				lbClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(loadbalancer.ListLoadBalancersResponse{
						Items: []loadbalancer.LoadBalancer{
							{
								Id:           common.String("lb-id"),
								FreeformTags: tags,
								DefinedTags:  make(map[string]map[string]interface{}),
								IsPrivate:    common.Bool(false),
								DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
								IpAddresses: []loadbalancer.IpAddress{
									{
										IpAddress: common.String("2.2.2.2"),
										IsPublic:  common.Bool(false),
									},
								},
							},
						},
					}, nil)
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:             common.String("lb-id"),
							LifecycleState: loadbalancer.LoadBalancerLifecycleStateActive,
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(false),
								},
							},
						},
					}, nil)
			},
		},
		{
			name:          "no cp subnet",
			errorExpected: true,
			matchError:    errors.New("control plane endpoint subnet not provided"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				lbClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(loadbalancer.ListLoadBalancersResponse{}, nil)
			},
		},
		{
			name:          "create load balancer more than one subnet",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets = []*infrastructurev1beta2.Subnet{
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
						ID:   common.String("s2"),
					},
				}
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup = infrastructurev1beta2.NetworkSecurityGroup{
					List: []*infrastructurev1beta2.NSG{
						{
							Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							ID:   common.String("nsg1"),
						},
						{
							Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							ID:   common.String("nsg2"),
						},
					},
				}
				definedTags, definedTagsInterface := getDefinedTags()
				ociClusterAccessor.OCICluster.Spec.DefinedTags = definedTags
				lbClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).Return(loadbalancer.ListLoadBalancersResponse{
					Items: []loadbalancer.LoadBalancer{},
				}, nil)
				lbClient.EXPECT().CreateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.CreateLoadBalancerRequest{
					CreateLoadBalancerDetails: loadbalancer.CreateLoadBalancerDetails{
						CompartmentId:           common.String("compartment-id"),
						DisplayName:             common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetIds:               []string{"s1", "s2"},
						NetworkSecurityGroupIds: []string{"nsg1", "nsg2"},
						IsPrivate:               common.Bool(false),
						ShapeName:               common.String("flexible"),
						ShapeDetails: &loadbalancer.ShapeDetails{MaximumBandwidthInMbps: common.Int(100),
							MinimumBandwidthInMbps: common.Int(10)},
						Listeners: map[string]loadbalancer.ListenerDetails{
							APIServerLBListener: {
								Protocol:              common.String("TCP"),
								Port:                  common.Int(int(6443)),
								DefaultBackendSetName: common.String(APIServerLBBackendSetName),
							},
						},
						BackendSets: map[string]loadbalancer.BackendSetDetails{
							APIServerLBBackendSetName: loadbalancer.BackendSetDetails{
								Policy: common.String("ROUND_ROBIN"),

								HealthChecker: &loadbalancer.HealthCheckerDetails{
									Port:     common.Int(6443),
									Protocol: common.String("TCP"),
								},
								Backends: []loadbalancer.BackendDetails{},
							},
						},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-lb", string("resource_uid")),
				})).
					Return(loadbalancer.CreateLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LoadBalancerId: common.String("lb-id"),
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					},
				}, nil)

				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lb-id"),
							FreeformTags: tags,
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)

			},
		},
		{
			name:          "create load balancer",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets = []*infrastructurev1beta2.Subnet{
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
				}
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup = infrastructurev1beta2.NetworkSecurityGroup{
					List: []*infrastructurev1beta2.NSG{
						{
							Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							ID:   common.String("nsg1"),
						},
						{
							Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							ID:   common.String("nsg2"),
						},
					},
				}
				definedTags, definedTagsInterface := getDefinedTags()
				ociClusterAccessor.OCICluster.Spec.DefinedTags = definedTags
				lbClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).Return(loadbalancer.ListLoadBalancersResponse{
					Items: []loadbalancer.LoadBalancer{},
				}, nil)
				lbClient.EXPECT().CreateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.CreateLoadBalancerRequest{
					CreateLoadBalancerDetails: loadbalancer.CreateLoadBalancerDetails{
						CompartmentId:           common.String("compartment-id"),
						DisplayName:             common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetIds:               []string{"s1"},
						NetworkSecurityGroupIds: []string{"nsg1", "nsg2"},
						IsPrivate:               common.Bool(false),
						ShapeName:               common.String("flexible"),
						ShapeDetails: &loadbalancer.ShapeDetails{MaximumBandwidthInMbps: common.Int(100),
							MinimumBandwidthInMbps: common.Int(10)},
						Listeners: map[string]loadbalancer.ListenerDetails{
							APIServerLBListener: {
								Protocol:              common.String("TCP"),
								Port:                  common.Int(int(6443)),
								DefaultBackendSetName: common.String(APIServerLBBackendSetName),
							},
						},
						BackendSets: map[string]loadbalancer.BackendSetDetails{
							APIServerLBBackendSetName: loadbalancer.BackendSetDetails{
								Policy: common.String("ROUND_ROBIN"),

								HealthChecker: &loadbalancer.HealthCheckerDetails{
									Port:     common.Int(6443),
									Protocol: common.String("TCP"),
								},
								Backends: []loadbalancer.BackendDetails{},
							},
						},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-lb", string("resource_uid")),
				})).
					Return(loadbalancer.CreateLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LoadBalancerId: common.String("lb-id"),
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					},
				}, nil)

				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lb-id"),
							FreeformTags: tags,
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)

			},
		},
		{
			name:                "create load balancer request fails",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets = []*infrastructurev1beta2.Subnet{
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
				}
				lbClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).Return(loadbalancer.ListLoadBalancersResponse{
					Items: []loadbalancer.LoadBalancer{},
				}, nil)
				lbClient.EXPECT().CreateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.CreateLoadBalancerRequest{
					CreateLoadBalancerDetails: loadbalancer.CreateLoadBalancerDetails{
						CompartmentId:           common.String("compartment-id"),
						DisplayName:             common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetIds:               []string{"s1"},
						NetworkSecurityGroupIds: make([]string, 0),
						ShapeName:               common.String("flexible"),
						ShapeDetails: &loadbalancer.ShapeDetails{MaximumBandwidthInMbps: common.Int(100),
							MinimumBandwidthInMbps: common.Int(10)},
						IsPrivate: common.Bool(false),
						Listeners: map[string]loadbalancer.ListenerDetails{
							APIServerLBListener: {
								Protocol:              common.String("TCP"),
								Port:                  common.Int(6443),
								DefaultBackendSetName: common.String(APIServerLBBackendSetName),
							},
						},
						BackendSets: map[string]loadbalancer.BackendSetDetails{
							APIServerLBBackendSetName: loadbalancer.BackendSetDetails{
								Policy: common.String("ROUND_ROBIN"),
								HealthChecker: &loadbalancer.HealthCheckerDetails{
									Port:     common.Int(6443),
									Protocol: common.String("TCP"),
								},
								Backends: []loadbalancer.BackendDetails{},
							},
						},
						FreeformTags: tags,
						DefinedTags:  map[string]map[string]interface{}{},
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-lb", string("resource_uid")),
				})).
					Return(loadbalancer.CreateLoadBalancerResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("No more Ip available in CIDR 1.1.1.1/1: WorkRequest opc-wr-id failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets = []*infrastructurev1beta2.Subnet{
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
				}
				lbClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(loadbalancer.ListLoadBalancersResponse{}, nil)
				lbClient.EXPECT().CreateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.CreateLoadBalancerRequest{
					CreateLoadBalancerDetails: loadbalancer.CreateLoadBalancerDetails{
						CompartmentId:           common.String("compartment-id"),
						DisplayName:             common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetIds:               []string{"s1"},
						NetworkSecurityGroupIds: make([]string, 0),
						ShapeName:               common.String("flexible"),
						ShapeDetails: &loadbalancer.ShapeDetails{MaximumBandwidthInMbps: common.Int(100),
							MinimumBandwidthInMbps: common.Int(10)},
						IsPrivate: common.Bool(false),
						Listeners: map[string]loadbalancer.ListenerDetails{
							APIServerLBListener: {
								Protocol:              common.String("TCP"),
								Port:                  common.Int(6443),
								DefaultBackendSetName: common.String(APIServerLBBackendSetName),
							},
						},
						BackendSets: map[string]loadbalancer.BackendSetDetails{
							APIServerLBBackendSetName: loadbalancer.BackendSetDetails{
								Policy: common.String("ROUND_ROBIN"),

								HealthChecker: &loadbalancer.HealthCheckerDetails{
									Port:     common.Int(6443),
									Protocol: common.String("TCP"),
								},
								Backends: []loadbalancer.BackendDetails{},
							},
						},
						FreeformTags: tags,
						DefinedTags:  make(map[string]map[string]interface{}),
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-lb", string("resource_uid")),
				})).
					Return(loadbalancer.CreateLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateFailed,
						ErrorDetails: []loadbalancer.WorkRequestError{
							{
								ErrorCode: loadbalancer.WorkRequestErrorErrorCodeBadInput,
								Message:   common.String("No more Ip available in CIDR 1.1.1.1/1"),
							},
						},
					},
				}, nil)
			},
		},
		{
			name:                "lb update failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("failed to reconcile the apiserver LB, failed to update lb"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:             common.String("lb-id"),
							LifecycleState: loadbalancer.LoadBalancerLifecycleStateActive,
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				lbClient.EXPECT().UpdateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.UpdateLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
					UpdateLoadBalancerDetails: loadbalancer.UpdateLoadBalancerDetails{
						DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						FreeformTags: tags,
						DefinedTags:  make(map[string]map[string]interface{}),
					},
				})).
					Return(loadbalancer.UpdateLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateFailed,
						ErrorDetails: []loadbalancer.WorkRequestError{
							{
								ErrorCode: loadbalancer.WorkRequestErrorErrorCodeInternalError,
								Message:   common.String("Lb not in active state"),
							},
						},
					},
				}, nil)
			},
		},
		{
			name:                "lb not active",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New(fmt.Sprintf("load balancer is in %s state. Waiting for ACTIVE state.", loadbalancer.LoadBalancerLifecycleStateCreating)),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:             common.String("lb-id"),
							LifecycleState: loadbalancer.LoadBalancerLifecycleStateCreating,
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
			},
		},
		{
			name:                "lb update request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:             common.String("lb-id"),
							LifecycleState: loadbalancer.LoadBalancerLifecycleStateActive,
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				lbClient.EXPECT().UpdateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.UpdateLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
					UpdateLoadBalancerDetails: loadbalancer.UpdateLoadBalancerDetails{
						DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						FreeformTags: tags,
						DefinedTags:  make(map[string]map[string]interface{}),
					},
				})).
					Return(loadbalancer.UpdateLoadBalancerResponse{}, errors.New("request failed"))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, lbClient)
			err := cs.ReconcileApiServerLB(context.Background())
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
				if tc.errorSubStringMatch {
					g.Expect(err.Error()).To(ContainSubstring(tc.matchError.Error()))
				} else {
					g.Expect(err.Error()).To(Equal(tc.matchError.Error()))
				}
			} else {
				g.Expect(err).To(BeNil())
			}
		})

	}
}

func TestLBDeletion(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		lbClient           *mock_lb.MockLoadBalancerClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		lbClient = mock_lb.NewMockLoadBalancerClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociClusterAccessor = OCISelfManagedCluster{
			&infrastructurev1beta2.OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					UID:  "a",
					Name: "cluster",
				},
				Spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId:         "compartment-id",
					OCIResourceIdentifier: "resource_uid",
				},
			},
		}
		ociClusterAccessor.OCICluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			LoadBalancerClient: lbClient,
			Cluster:            &clusterv1.Cluster{},
			OCIClusterAccessor: ociClusterAccessor,
			Client:             client,
		})
		tags = make(map[string]string)
		tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
		tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name                string
		errorExpected       bool
		objects             []client.Object
		expectedEvent       string
		eventNotExpected    string
		matchError          error
		errorSubStringMatch bool
		testSpecificSetup   func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient)
	}{
		{
			name:          "lb already deleted",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:          "list lb by display name",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				lbClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(loadbalancer.ListLoadBalancersResponse{
						Items: []loadbalancer.LoadBalancer{
							{
								Id:           common.String("lb-id"),
								FreeformTags: tags,
								DefinedTags:  make(map[string]map[string]interface{}),
								IsPrivate:    common.Bool(false),
								DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
								IpAddresses: []loadbalancer.IpAddress{
									{
										IpAddress: common.String("2.2.2.2"),
										IsPublic:  common.Bool(false),
									},
								},
							},
						},
					}, nil)
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				lbClient.EXPECT().DeleteLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.DeleteLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.DeleteLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					},
				}, nil)
			},
		},
		{
			name:          "lb delete by id",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				lbClient.EXPECT().DeleteLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.DeleteLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.DeleteLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					},
				}, nil)
			},
		},
		{
			name:                "lb delete request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				lbClient.EXPECT().DeleteLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.DeleteLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.DeleteLoadBalancerResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "lb delete work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("work request to delete lb failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbClient *mock_lb.MockLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lb-id")
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				lbClient.EXPECT().DeleteLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.DeleteLoadBalancerRequest{
					LoadBalancerId: common.String("lb-id"),
				})).
					Return(loadbalancer.DeleteLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateFailed,
						ErrorDetails: []loadbalancer.WorkRequestError{
							{
								ErrorCode: loadbalancer.WorkRequestErrorErrorCodeBadInput,
								Message:   common.String("Internal server error to delete lb"),
							},
						},
					},
				}, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, lbClient)
			err := cs.DeleteApiServerLB(context.Background())
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
				if tc.errorSubStringMatch {
					g.Expect(err.Error()).To(ContainSubstring(tc.matchError.Error()))
				} else {
					g.Expect(err.Error()).To(Equal(tc.matchError.Error()))
				}
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}
