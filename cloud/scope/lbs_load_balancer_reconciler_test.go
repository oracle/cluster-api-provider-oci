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
	"github.com/oracle/cluster-api-provider-oci/cloud/services/loadbalancerservice/mock_lbs"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLBsReconciliation(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		lbsClient          *mock_lbs.MockLoadBalancerServiceClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		lbsClient = mock_lbs.NewMockLoadBalancerServiceClient(mockCtrl)
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
			LoadBalancerServiceClient: lbsClient,
			Cluster:                   &clusterv1.Cluster{},
			OCIClusterAccessor:        ociClusterAccessor,
			Client:                    client,
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
		testSpecificSetup   func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient)
	}{
		{
			name:          "lbs exists",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lbs-id")
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
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
			},
		},
		{
			name:          "lbs does not have ip address",
			errorExpected: true,
			matchError:    errors.New("lb does not have valid ip addresses"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lbs-id")
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						},
					}, nil)
			},
		},
		{
			name:          "lbs does not have public ip address",
			errorExpected: true,
			matchError:    errors.New("lb does not have valid public ip address"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lbs-id")
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
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
					}, nil)
			},
		},
		{
			name:          "lbs lookup by display name",
			errorExpected: true,
			matchError:    errors.New("lb does not have valid public ip address"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				lbsClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(loadbalancer.ListLoadBalancersResponse{
						Items: []loadbalancer.LoadBalancer{
							{
								Id:           common.String("lbs-id"),
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
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
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
					}, nil)
			},
		},
		{
			name:          "no cp subnet",
			errorExpected: true,
			matchError:    errors.New("control plane endpoint subnet not provided"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				lbsClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(loadbalancer.ListLoadBalancersResponse{}, nil)
			},
		},
		{
			name:          "more than one cp subnet",
			errorExpected: true,
			matchError:    errors.New("cannot have more than 1 control plane endpoint subnet"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
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
				lbsClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(loadbalancer.ListLoadBalancersResponse{}, nil)
			},
		},
		{
			name:          "create load balancer",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets = []*infrastructurev1beta2.Subnet{
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
				}
				definedTags, definedTagsInterface := getDefinedTags()
				ociClusterAccessor.OCICluster.Spec.DefinedTags = definedTags
				lbsClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).Return(loadbalancer.ListLoadBalancersResponse{
					Items: []loadbalancer.LoadBalancer{},
				}, nil)
				lbsClient.EXPECT().CreateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.CreateLoadBalancerRequest{
					CreateLoadBalancerDetails: loadbalancer.CreateLoadBalancerDetails{
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetIds:     []string{"s1"},
						IsPrivate:     common.Bool(false),
						ShapeName:     common.String("flexible"),
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
						// LoadBalancer: loadbalancer.LoadBalancer{
						// 	Id: common.String("lbs-id"),
						// },
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbsClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LoadBalancerId: common.String("lbs-id"),
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					},
				}, nil)

				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
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
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets = []*infrastructurev1beta2.Subnet{
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
				}
				lbsClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).Return(loadbalancer.ListLoadBalancersResponse{
					Items: []loadbalancer.LoadBalancer{},
				}, nil)
				lbsClient.EXPECT().CreateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.CreateLoadBalancerRequest{
					CreateLoadBalancerDetails: loadbalancer.CreateLoadBalancerDetails{
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetIds:     []string{"s1"},
						ShapeName:     common.String("flexible"),
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
			matchError:          errors.New("WorkRequest opc-wr-id failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets = []*infrastructurev1beta2.Subnet{
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
				}
				lbsClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(loadbalancer.ListLoadBalancersResponse{}, nil)
				lbsClient.EXPECT().CreateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.CreateLoadBalancerRequest{
					CreateLoadBalancerDetails: loadbalancer.CreateLoadBalancerDetails{
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetIds:     []string{"s1"},
						ShapeName:     common.String("flexible"),
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
						// LoadBalancer: loadbalancer.LoadBalancerr{
						// 	Id: common.String("lbs-id"),
						// },
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbsClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateFailed,
					},
				}, nil)
			},
		},
		{
			name:                "lbs update failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("failed to reconcile the apiserver LB, failed to update lb"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("lbs-id")
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				lbsClient.EXPECT().UpdateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.UpdateLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
					UpdateLoadBalancerDetails: loadbalancer.UpdateLoadBalancerDetails{
						DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						FreeformTags: tags,
						DefinedTags:  make(map[string]map[string]interface{}),
					},
				})).
					Return(loadbalancer.UpdateLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbsClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateFailed,
					},
				}, nil)
			},
		},
		{
			name:                "lbs update request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lbs-id")
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
							IpAddresses: []loadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				lbsClient.EXPECT().UpdateLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.UpdateLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
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
			tc.testSpecificSetup(cs, lbsClient)
			err := cs.ReconcileApiServerLbsLB(context.Background())
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

func TestLBsDeletion(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		lbsClient          *mock_lbs.MockLoadBalancerServiceClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		lbsClient = mock_lbs.NewMockLoadBalancerServiceClient(mockCtrl)
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
			LoadBalancerServiceClient: lbsClient,
			Cluster:                   &clusterv1.Cluster{},
			OCIClusterAccessor:        ociClusterAccessor,
			Client:                    client,
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
		testSpecificSetup   func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient)
	}{
		{
			name:          "lbs already deleted",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lbs-id")
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:          "list lbs by display name",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				lbsClient.EXPECT().ListLoadBalancers(gomock.Any(), gomock.Eq(loadbalancer.ListLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(loadbalancer.ListLoadBalancersResponse{
						Items: []loadbalancer.LoadBalancer{
							{
								Id:           common.String("lbs-id"),
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
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
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
				lbsClient.EXPECT().DeleteLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.DeleteLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.DeleteLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbsClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					},
				}, nil)
			},
		},
		{
			name:          "lbs delete by id",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lbs-id")
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
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
				lbsClient.EXPECT().DeleteLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.DeleteLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.DeleteLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbsClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					},
				}, nil)
			},
		},
		{
			name:                "lbs delete request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lbs-id")
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
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
				lbsClient.EXPECT().DeleteLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.DeleteLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.DeleteLoadBalancerResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "lbs delete work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("work request to delete lb failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, lbsClient *mock_lbs.MockLoadBalancerServiceClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("lbs-id")
				lbsClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.GetLoadBalancerResponse{
						LoadBalancer: loadbalancer.LoadBalancer{
							Id:           common.String("lbs-id"),
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
				lbsClient.EXPECT().DeleteLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.DeleteLoadBalancerRequest{
					LoadBalancerId: common.String("lbs-id"),
				})).
					Return(loadbalancer.DeleteLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				lbsClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(loadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateFailed,
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
			tc.testSpecificSetup(cs, lbsClient)
			err := cs.DeleteApiServerLbsLB(context.Background())
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
