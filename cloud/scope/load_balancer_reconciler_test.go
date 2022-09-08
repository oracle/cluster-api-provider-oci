/*
Copyright 2022 The Kubernetes Authors.

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

package scope

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer/mock_nlb"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/networkloadbalancer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLBReconciliation(t *testing.T) {
	var (
		cs         *ClusterScope
		mockCtrl   *gomock.Controller
		nlbClient  *mock_nlb.MockNetworkLoadBalancerClient
		ociCluster infrastructurev1beta1.OCICluster
		tags       map[string]string
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociCluster = infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID:  "a",
				Name: "cluster",
			},
			Spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId:         "compartment-id",
				OCIResourceIdentifier: "resource_uid",
			},
		}
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			LoadBalancerClient: nlbClient,
			Cluster:            &clusterv1.Cluster{},
			OCICluster:         &ociCluster,
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
		testSpecificSetup   func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient)
	}{
		{
			name:          "nlb exists",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
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
			name:          "nlb does not have ip address",
			errorExpected: true,
			matchError:    errors.New("lb does not have valid ip addresses"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						},
					}, nil)
			},
		},
		{
			name:          "nlb does not have public ip address",
			errorExpected: true,
			matchError:    errors.New("lb does not have valid public ip address"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
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
			name:          "nlb lookup by display name",
			errorExpected: true,
			matchError:    errors.New("lb does not have valid public ip address"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{
						NetworkLoadBalancerCollection: networkloadbalancer.NetworkLoadBalancerCollection{
							Items: []networkloadbalancer.NetworkLoadBalancerSummary{
								{
									Id:           common.String("nlb-id"),
									FreeformTags: tags,
									DefinedTags:  make(map[string]map[string]interface{}),
									IsPrivate:    common.Bool(false),
									DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
									IpAddresses: []networkloadbalancer.IpAddress{
										{
											IpAddress: common.String("2.2.2.2"),
											IsPublic:  common.Bool(false),
										},
									},
								},
							},
						},
					}, nil)
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
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
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{}, nil)
			},
		},
		{
			name:          "more than one cp subnet",
			errorExpected: true,
			matchError:    errors.New("cannot have more than 1 control plane endpoint subnet"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.Vcn.Subnets = []*infrastructurev1beta1.Subnet{
					{
						Role: infrastructurev1beta1.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
					{
						Role: infrastructurev1beta1.ControlPlaneEndpointRole,
						ID:   common.String("s2"),
					},
				}
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{}, nil)
			},
		},
		{
			name:          "create load balancer",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.Vcn.Subnets = []*infrastructurev1beta1.Subnet{
					{
						Role: infrastructurev1beta1.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
				}
				definedTags, definedTagsInterface := getDefinedTags()
				clusterScope.OCICluster.Spec.DefinedTags = definedTags
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{}, nil)
				nlbClient.EXPECT().CreateNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.CreateNetworkLoadBalancerRequest{
					CreateNetworkLoadBalancerDetails: networkloadbalancer.CreateNetworkLoadBalancerDetails{
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetId:      common.String("s1"),
						IsPrivate:     common.Bool(false),
						Listeners: map[string]networkloadbalancer.ListenerDetails{
							APIServerLBListener: {
								Protocol:              networkloadbalancer.ListenerProtocolsTcp,
								Port:                  common.Int(6443),
								DefaultBackendSetName: common.String(APIServerLBBackendSetName),
								Name:                  common.String(APIServerLBListener),
							},
						},
						BackendSets: map[string]networkloadbalancer.BackendSetDetails{
							APIServerLBBackendSetName: networkloadbalancer.BackendSetDetails{
								Policy:           LoadBalancerPolicy,
								IsPreserveSource: common.Bool(false),
								HealthChecker: &networkloadbalancer.HealthChecker{
									Port:       common.Int(6443),
									Protocol:   networkloadbalancer.HealthCheckProtocolsHttps,
									UrlPath:    common.String("/healthz"),
									ReturnCode: common.Int(200),
								},
								Backends: []networkloadbalancer.Backend{},
							},
						},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-lb", string("resource_uid")),
				})).
					Return(networkloadbalancer.CreateNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id: common.String("nlb-id"),
						},
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					},
				}, nil)

				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
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
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.Vcn.Subnets = []*infrastructurev1beta1.Subnet{
					{
						Role: infrastructurev1beta1.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
				}
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{}, nil)
				nlbClient.EXPECT().CreateNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.CreateNetworkLoadBalancerRequest{
					CreateNetworkLoadBalancerDetails: networkloadbalancer.CreateNetworkLoadBalancerDetails{
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetId:      common.String("s1"),
						IsPrivate:     common.Bool(false),
						Listeners: map[string]networkloadbalancer.ListenerDetails{
							APIServerLBListener: {
								Protocol:              networkloadbalancer.ListenerProtocolsTcp,
								Port:                  common.Int(6443),
								DefaultBackendSetName: common.String(APIServerLBBackendSetName),
								Name:                  common.String(APIServerLBListener),
							},
						},
						BackendSets: map[string]networkloadbalancer.BackendSetDetails{
							APIServerLBBackendSetName: networkloadbalancer.BackendSetDetails{
								Policy:           LoadBalancerPolicy,
								IsPreserveSource: common.Bool(false),
								HealthChecker: &networkloadbalancer.HealthChecker{
									Port:       common.Int(6443),
									Protocol:   networkloadbalancer.HealthCheckProtocolsHttps,
									UrlPath:    common.String("/healthz"),
									ReturnCode: common.Int(200),
								},
								Backends: []networkloadbalancer.Backend{},
							},
						},
						FreeformTags: tags,
						DefinedTags:  map[string]map[string]interface{}{},
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-lb", string("resource_uid")),
				})).
					Return(networkloadbalancer.CreateNetworkLoadBalancerResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("WorkRequest opc-wr-id failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.Vcn.Subnets = []*infrastructurev1beta1.Subnet{
					{
						Role: infrastructurev1beta1.ControlPlaneEndpointRole,
						ID:   common.String("s1"),
					},
				}
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{}, nil)
				nlbClient.EXPECT().CreateNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.CreateNetworkLoadBalancerRequest{
					CreateNetworkLoadBalancerDetails: networkloadbalancer.CreateNetworkLoadBalancerDetails{
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetId:      common.String("s1"),
						IsPrivate:     common.Bool(false),
						Listeners: map[string]networkloadbalancer.ListenerDetails{
							APIServerLBListener: {
								Protocol:              networkloadbalancer.ListenerProtocolsTcp,
								Port:                  common.Int(6443),
								DefaultBackendSetName: common.String(APIServerLBBackendSetName),
								Name:                  common.String(APIServerLBListener),
							},
						},
						BackendSets: map[string]networkloadbalancer.BackendSetDetails{
							APIServerLBBackendSetName: networkloadbalancer.BackendSetDetails{
								Policy:           LoadBalancerPolicy,
								IsPreserveSource: common.Bool(false),
								HealthChecker: &networkloadbalancer.HealthChecker{
									Port:       common.Int(6443),
									Protocol:   networkloadbalancer.HealthCheckProtocolsHttps,
									UrlPath:    common.String("/healthz"),
									ReturnCode: common.Int(200),
								},
								Backends: []networkloadbalancer.Backend{},
							},
						},
						FreeformTags: tags,
						DefinedTags:  make(map[string]map[string]interface{}),
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-lb", string("resource_uid")),
				})).
					Return(networkloadbalancer.CreateNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id: common.String("nlb-id"),
						},
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusFailed,
					},
				}, nil)
			},
		},
		{
			name:                "nlb update failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("failed to reconcile the apiserver LB, failed to update lb"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
							IpAddresses: []networkloadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				nlbClient.EXPECT().UpdateNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.UpdateNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
					UpdateNetworkLoadBalancerDetails: networkloadbalancer.UpdateNetworkLoadBalancerDetails{
						DisplayName: common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
					},
				})).
					Return(networkloadbalancer.UpdateNetworkLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusFailed,
					},
				}, nil)
			},
		},
		{
			name:                "nlb update request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
							IpAddresses: []networkloadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				nlbClient.EXPECT().UpdateNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.UpdateNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
					UpdateNetworkLoadBalancerDetails: networkloadbalancer.UpdateNetworkLoadBalancerDetails{
						DisplayName: common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
					},
				})).
					Return(networkloadbalancer.UpdateNetworkLoadBalancerResponse{}, errors.New("request failed"))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, nlbClient)
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
		cs         *ClusterScope
		mockCtrl   *gomock.Controller
		nlbClient  *mock_nlb.MockNetworkLoadBalancerClient
		ociCluster infrastructurev1beta1.OCICluster
		tags       map[string]string
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociCluster = infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID:  "a",
				Name: "cluster",
			},
			Spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId:         "compartment-id",
				OCIResourceIdentifier: "resource_uid",
			},
		}
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			LoadBalancerClient: nlbClient,
			Cluster:            &clusterv1.Cluster{},
			OCICluster:         &ociCluster,
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
		testSpecificSetup   func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient)
	}{
		{
			name:          "nlb already deleted",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:          "list nlb by display name",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{
						NetworkLoadBalancerCollection: networkloadbalancer.NetworkLoadBalancerCollection{
							Items: []networkloadbalancer.NetworkLoadBalancerSummary{
								{
									Id:           common.String("nlb-id"),
									FreeformTags: tags,
								},
							},
						},
					}, nil)
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				nlbClient.EXPECT().DeleteNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.DeleteNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.DeleteNetworkLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					},
				}, nil)
			},
		},
		{
			name:          "nlb delete by id",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				nlbClient.EXPECT().DeleteNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.DeleteNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.DeleteNetworkLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					},
				}, nil)
			},
		},
		{
			name:                "nlb delete request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				nlbClient.EXPECT().DeleteNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.DeleteNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.DeleteNetworkLoadBalancerResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "nlb delete work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("work request to delete lb failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:           common.String("nlb-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
							IsPrivate:    common.Bool(false),
							DisplayName:  common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
						},
					}, nil)
				nlbClient.EXPECT().DeleteNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.DeleteNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.DeleteNetworkLoadBalancerResponse{
						OpcWorkRequestId: common.String("opc-wr-id"),
					}, nil)
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-wr-id"),
				})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusFailed,
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
			tc.testSpecificSetup(cs, nlbClient)
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
func getDefinedTags() (map[string]map[string]string, map[string]map[string]interface{}) {
	definedTags := map[string]map[string]string{
		"ns1": {
			"tag1": "foo",
			"tag2": "bar",
		},
		"ns2": {
			"tag1": "foo1",
			"tag2": "bar1",
		},
	}
	definedTagsInterface := make(map[string]map[string]interface{})
	for ns, mapNs := range definedTags {
		mapValues := make(map[string]interface{})
		for k, v := range mapNs {
			mapValues[k] = v
		}
		definedTagsInterface[ns] = mapValues
	}
	return definedTags, definedTagsInterface
}
