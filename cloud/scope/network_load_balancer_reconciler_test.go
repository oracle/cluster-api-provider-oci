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
	"github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer/mock_nlb"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type extendedMockNLBClient struct {
	*mock_nlb.MockNetworkLoadBalancerClient
	createBackendSetFn func(ctx context.Context, request networkloadbalancer.CreateBackendSetRequest) (networkloadbalancer.CreateBackendSetResponse, error)
	updateBackendSetFn func(ctx context.Context, request networkloadbalancer.UpdateBackendSetRequest) (networkloadbalancer.UpdateBackendSetResponse, error)
	deleteBackendSetFn func(ctx context.Context, request networkloadbalancer.DeleteBackendSetRequest) (networkloadbalancer.DeleteBackendSetResponse, error)
	createListenerFn   func(ctx context.Context, request networkloadbalancer.CreateListenerRequest) (networkloadbalancer.CreateListenerResponse, error)
	updateListenerFn   func(ctx context.Context, request networkloadbalancer.UpdateListenerRequest) (networkloadbalancer.UpdateListenerResponse, error)
	deleteListenerFn   func(ctx context.Context, request networkloadbalancer.DeleteListenerRequest) (networkloadbalancer.DeleteListenerResponse, error)
}

func (c *extendedMockNLBClient) CreateBackendSet(ctx context.Context, request networkloadbalancer.CreateBackendSetRequest) (networkloadbalancer.CreateBackendSetResponse, error) {
	return c.createBackendSetFn(ctx, request)
}

func (c *extendedMockNLBClient) UpdateBackendSet(ctx context.Context, request networkloadbalancer.UpdateBackendSetRequest) (networkloadbalancer.UpdateBackendSetResponse, error) {
	return c.updateBackendSetFn(ctx, request)
}

func (c *extendedMockNLBClient) DeleteBackendSet(ctx context.Context, request networkloadbalancer.DeleteBackendSetRequest) (networkloadbalancer.DeleteBackendSetResponse, error) {
	return c.deleteBackendSetFn(ctx, request)
}

func (c *extendedMockNLBClient) CreateListener(ctx context.Context, request networkloadbalancer.CreateListenerRequest) (networkloadbalancer.CreateListenerResponse, error) {
	return c.createListenerFn(ctx, request)
}

func (c *extendedMockNLBClient) UpdateListener(ctx context.Context, request networkloadbalancer.UpdateListenerRequest) (networkloadbalancer.UpdateListenerResponse, error) {
	return c.updateListenerFn(ctx, request)
}

func (c *extendedMockNLBClient) DeleteListener(ctx context.Context, request networkloadbalancer.DeleteListenerRequest) (networkloadbalancer.DeleteListenerResponse, error) {
	return c.deleteListenerFn(ctx, request)
}

func TestNLBReconciliation(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		nlbClient          *mock_nlb.MockNetworkLoadBalancerClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
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
			NetworkLoadBalancerClient: nlbClient,
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
		testSpecificSetup   func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient)
	}{
		{
			name:          "nlb exists",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							LifecycleState: networkloadbalancer.LifecycleStateActive,
							Id:             common.String("nlb-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
							Listeners: map[string]networkloadbalancer.Listener{
								APIServerLBListener: {
									Name:                  common.String(APIServerLBListener),
									DefaultBackendSetName: common.String(APIServerLBBackendSetName),
									Port:                  common.Int(6443),
									Protocol:              networkloadbalancer.ListenerProtocolsTcp,
								},
							},
							BackendSets: map[string]networkloadbalancer.BackendSet{
								APIServerLBBackendSetName: {
									Name:             common.String(APIServerLBBackendSetName),
									Policy:           LoadBalancerPolicy,
									IsPreserveSource: common.Bool(false),
									HealthChecker: &networkloadbalancer.HealthChecker{
										Port:     common.Int(6443),
										Protocol: networkloadbalancer.HealthCheckProtocolsHttps,
										UrlPath:  common.String("/healthz"),
									},
								},
							},
						},
					}, nil)
			},
		},
		{
			name:          "nlb spec-only resource change triggers update",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				secondaryPort := int32(9345)
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("nlb-id")
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.NLBSpec.BackendSets = []infrastructurev1beta2.NLBBackendSet{
					{Name: APIServerLBBackendSetName},
					{Name: "rollout-set", ListenerPort: &secondaryPort},
				}
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							LifecycleState: networkloadbalancer.LifecycleStateActive,
							Id:             common.String("nlb-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
							IpAddresses: []networkloadbalancer.IpAddress{
								{
									IpAddress: common.String("2.2.2.2"),
									IsPublic:  common.Bool(true),
								},
							},
							Listeners: map[string]networkloadbalancer.Listener{
								APIServerLBListener: {
									Name:                  common.String(APIServerLBListener),
									DefaultBackendSetName: common.String(APIServerLBBackendSetName),
									Port:                  common.Int(6443),
									Protocol:              networkloadbalancer.ListenerProtocolsTcp,
								},
							},
							BackendSets: map[string]networkloadbalancer.BackendSet{
								APIServerLBBackendSetName: {
									Name:             common.String(APIServerLBBackendSetName),
									Policy:           LoadBalancerPolicy,
									IsPreserveSource: common.Bool(false),
									HealthChecker: &networkloadbalancer.HealthChecker{
										Port:     common.Int(6443),
										Protocol: networkloadbalancer.HealthCheckProtocolsHttps,
										UrlPath:  common.String("/healthz"),
									},
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
						OpcWorkRequestId: common.String("wr-nlb-update"),
					}, nil)
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
					WorkRequestId: common.String("wr-nlb-update"),
				})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					},
				}, nil)
			},
		},
		{
			name:          "nlb does not have ip address",
			errorExpected: true,
			matchError:    errors.New("nlb does not have valid ip addresses"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							LifecycleState: networkloadbalancer.LifecycleStateActive,
							Id:             common.String("nlb-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						},
					}, nil)
			},
		},
		{
			name:          "nlb does not have public ip address",
			errorExpected: true,
			matchError:    errors.New("nlb does not have valid public ip address"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							LifecycleState: networkloadbalancer.LifecycleStateActive,
							Id:             common.String("nlb-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
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
			matchError:    errors.New("nlb does not have valid public ip address"),
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
							LifecycleState: networkloadbalancer.LifecycleStateActive,
							Id:             common.String("nlb-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
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
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{}, nil)
			},
		},
		{
			name:          "create network load balancer, default values",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
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
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB = infrastructurev1beta2.LoadBalancer{}
				definedTags, definedTagsInterface := getDefinedTags()
				ociClusterAccessor.OCICluster.Spec.DefinedTags = definedTags
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{}, nil)
				nlbClient.EXPECT().CreateNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.CreateNetworkLoadBalancerRequest{
					CreateNetworkLoadBalancerDetails: networkloadbalancer.CreateNetworkLoadBalancerDetails{
						CompartmentId:           common.String("compartment-id"),
						DisplayName:             common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetId:                common.String("s1"),
						IsPrivate:               common.Bool(false),
						NetworkSecurityGroupIds: []string{"nsg1", "nsg2"},
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
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-nlb", string("resource_uid")),
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
			name:          "create network load balancer",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
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
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB = infrastructurev1beta2.LoadBalancer{
					NLBSpec: infrastructurev1beta2.NLBSpec{
						BackendSetDetails: infrastructurev1beta2.BackendSetDetails{
							IsInstantFailoverEnabled: common.Bool(true),
							IsFailOpen:               common.Bool(false),
							IsPreserveSource:         common.Bool(false),
							HealthChecker: infrastructurev1beta2.HealthChecker{
								UrlPath: common.String("readyz"),
							},
						},
					},
				}
				definedTags, definedTagsInterface := getDefinedTags()
				ociClusterAccessor.OCICluster.Spec.DefinedTags = definedTags
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{}, nil)
				nlbClient.EXPECT().CreateNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.CreateNetworkLoadBalancerRequest{
					CreateNetworkLoadBalancerDetails: networkloadbalancer.CreateNetworkLoadBalancerDetails{
						CompartmentId:           common.String("compartment-id"),
						DisplayName:             common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetId:                common.String("s1"),
						IsPrivate:               common.Bool(false),
						NetworkSecurityGroupIds: []string{"nsg1", "nsg2"},
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
								Policy:                   LoadBalancerPolicy,
								IsInstantFailoverEnabled: common.Bool(true),
								IsFailOpen:               common.Bool(false),
								IsPreserveSource:         common.Bool(false),
								HealthChecker: &networkloadbalancer.HealthChecker{
									Port:       common.Int(6443),
									Protocol:   networkloadbalancer.HealthCheckProtocolsHttps,
									UrlPath:    common.String("readyz"),
									ReturnCode: common.Int(200),
								},
								Backends: []networkloadbalancer.Backend{},
							},
						},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-nlb", string("resource_uid")),
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
			name:          "create network load balancer, reserved ip",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
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
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB = infrastructurev1beta2.LoadBalancer{
					NLBSpec: infrastructurev1beta2.NLBSpec{
						ReservedIpIds: []string{"ocid1.publicip.oc1.iad.testocid"},
						BackendSetDetails: infrastructurev1beta2.BackendSetDetails{
							IsInstantFailoverEnabled: common.Bool(true),
							IsFailOpen:               common.Bool(false),
							IsPreserveSource:         common.Bool(false),
							HealthChecker: infrastructurev1beta2.HealthChecker{
								UrlPath: common.String("readyz"),
							},
						},
					},
				}
				definedTags, definedTagsInterface := getDefinedTags()
				ociClusterAccessor.OCICluster.Spec.DefinedTags = definedTags
				nlbClient.EXPECT().ListNetworkLoadBalancers(gomock.Any(), gomock.Eq(networkloadbalancer.ListNetworkLoadBalancersRequest{
					CompartmentId: common.String("compartment-id"),
					DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
				})).
					Return(networkloadbalancer.ListNetworkLoadBalancersResponse{}, nil)
				nlbClient.EXPECT().CreateNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.CreateNetworkLoadBalancerRequest{
					CreateNetworkLoadBalancerDetails: networkloadbalancer.CreateNetworkLoadBalancerDetails{
						CompartmentId:           common.String("compartment-id"),
						DisplayName:             common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetId:                common.String("s1"),
						IsPrivate:               common.Bool(false),
						NetworkSecurityGroupIds: []string{"nsg1", "nsg2"},
						Listeners: map[string]networkloadbalancer.ListenerDetails{
							APIServerLBListener: {
								Protocol:              networkloadbalancer.ListenerProtocolsTcp,
								Port:                  common.Int(6443),
								DefaultBackendSetName: common.String(APIServerLBBackendSetName),
								Name:                  common.String(APIServerLBListener),
							},
						},
						ReservedIps: []networkloadbalancer.ReservedIp{networkloadbalancer.ReservedIp{Id: common.String("ocid1.publicip.oc1.iad.testocid")}},
						BackendSets: map[string]networkloadbalancer.BackendSetDetails{
							APIServerLBBackendSetName: networkloadbalancer.BackendSetDetails{
								Policy:                   LoadBalancerPolicy,
								IsInstantFailoverEnabled: common.Bool(true),
								IsFailOpen:               common.Bool(false),
								IsPreserveSource:         common.Bool(false),
								HealthChecker: &networkloadbalancer.HealthChecker{
									Port:       common.Int(6443),
									Protocol:   networkloadbalancer.HealthCheckProtocolsHttps,
									UrlPath:    common.String("readyz"),
									ReturnCode: common.Int(200),
								},
								Backends: []networkloadbalancer.Backend{},
							},
						},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-nlb", string("resource_uid")),
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
			name:                "create network load balancer request fails",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets = []*infrastructurev1beta2.Subnet{
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
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
						CompartmentId:           common.String("compartment-id"),
						DisplayName:             common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetId:                common.String("s1"),
						IsPrivate:               common.Bool(false),
						NetworkSecurityGroupIds: make([]string, 0),
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
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-nlb", string("resource_uid")),
				})).
					Return(networkloadbalancer.CreateNetworkLoadBalancerResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("No more Ip available in CIDR 1.1.1.1/1: WorkRequest opc-wr-id failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets = []*infrastructurev1beta2.Subnet{
					{
						Role: infrastructurev1beta2.ControlPlaneEndpointRole,
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
						CompartmentId:           common.String("compartment-id"),
						DisplayName:             common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
						SubnetId:                common.String("s1"),
						NetworkSecurityGroupIds: make([]string, 0),
						IsPrivate:               common.Bool(false),
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
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-nlb", string("resource_uid")),
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
						CompartmentId: common.String("compartment-id"),
						Status:        networkloadbalancer.OperationStatusFailed,
					},
				}, nil)
				nlbClient.EXPECT().ListWorkRequestErrors(gomock.Any(), gomock.Eq(networkloadbalancer.ListWorkRequestErrorsRequest{
					WorkRequestId: common.String("opc-wr-id"),
					CompartmentId: common.String("compartment-id"),
				})).Return(networkloadbalancer.ListWorkRequestErrorsResponse{
					WorkRequestErrorCollection: networkloadbalancer.WorkRequestErrorCollection{
						Items: []networkloadbalancer.WorkRequestError{
							{
								Code:    common.String("OKE-001"),
								Message: common.String("No more Ip available in CIDR 1.1.1.1/1"),
							},
						},
					},
				}, nil)
			},
		},
		{
			name:                "nlb update failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("failed to reconcile the apiserver NLB, failed to update nlb"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							LifecycleState: networkloadbalancer.LifecycleStateActive,
							Id:             common.String("nlb-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
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
						CompartmentId: common.String("compartment-id"),
						Status:        networkloadbalancer.OperationStatusFailed,
					},
				}, nil)
				nlbClient.EXPECT().ListWorkRequestErrors(gomock.Any(), gomock.Eq(networkloadbalancer.ListWorkRequestErrorsRequest{
					WorkRequestId: common.String("opc-wr-id"),
					CompartmentId: common.String("compartment-id"),
				})).Return(networkloadbalancer.ListWorkRequestErrorsResponse{
					WorkRequestErrorCollection: networkloadbalancer.WorkRequestErrorCollection{
						Items: []networkloadbalancer.WorkRequestError{
							{
								Code:    common.String("OKE-001"),
								Message: common.String("No more Ip available in CIDR 1.1.1.1/1"),
							},
						},
					},
				}, nil)
			},
		},
		{
			name:                "nlb not active",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New(fmt.Sprintf("network load balancer is in %s state. Waiting for ACTIVE state.", networkloadbalancer.LifecycleStateCreating)),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlbId")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbId"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							LifecycleState: networkloadbalancer.LifecycleStateCreating,
							Id:             common.String("nlbId"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
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
			name:                "nlb update request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							LifecycleState: networkloadbalancer.LifecycleStateActive,
							Id:             common.String("nlb-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							IsPrivate:      common.Bool(false),
							DisplayName:    common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver-test")),
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
			err := cs.ReconcileApiServerNLB(context.Background())
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

func TestReconcileNLBResources_IgnoresAlreadyExistsOnCreate(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	baseClient := mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
	nlbClient := &extendedMockNLBClient{
		MockNetworkLoadBalancerClient: baseClient,
		createBackendSetFn: func(_ context.Context, request networkloadbalancer.CreateBackendSetRequest) (networkloadbalancer.CreateBackendSetResponse, error) {
			if request.CreateBackendSetDetails.Name == nil || *request.CreateBackendSetDetails.Name != "rollout-set" {
				t.Fatalf("unexpected backend set create request: %#v", request)
			}
			return networkloadbalancer.CreateBackendSetResponse{}, mockServiceError{
				status:  409,
				code:    "Conflict",
				message: "backend set already exists",
			}
		},
		updateBackendSetFn: func(_ context.Context, _ networkloadbalancer.UpdateBackendSetRequest) (networkloadbalancer.UpdateBackendSetResponse, error) {
			t.Fatalf("unexpected backend set update")
			return networkloadbalancer.UpdateBackendSetResponse{}, nil
		},
		deleteBackendSetFn: func(_ context.Context, _ networkloadbalancer.DeleteBackendSetRequest) (networkloadbalancer.DeleteBackendSetResponse, error) {
			t.Fatalf("unexpected backend set delete")
			return networkloadbalancer.DeleteBackendSetResponse{}, nil
		},
		createListenerFn: func(_ context.Context, request networkloadbalancer.CreateListenerRequest) (networkloadbalancer.CreateListenerResponse, error) {
			if request.CreateListenerDetails.Name == nil || *request.CreateListenerDetails.Name != desiredAPIServerListenerName(1, 2, "rollout-set") {
				t.Fatalf("unexpected listener create request: %#v", request)
			}
			return networkloadbalancer.CreateListenerResponse{}, mockServiceError{
				status:  409,
				code:    "Conflict",
				message: "listener already exists",
			}
		},
		updateListenerFn: func(_ context.Context, _ networkloadbalancer.UpdateListenerRequest) (networkloadbalancer.UpdateListenerResponse, error) {
			t.Fatalf("unexpected listener update")
			return networkloadbalancer.UpdateListenerResponse{}, nil
		},
		deleteListenerFn: func(_ context.Context, _ networkloadbalancer.DeleteListenerRequest) (networkloadbalancer.DeleteListenerResponse, error) {
			t.Fatalf("unexpected listener delete")
			return networkloadbalancer.DeleteListenerResponse{}, nil
		},
	}

	client := fake.NewClientBuilder().Build()
	ociClusterAccessor := OCISelfManagedCluster{
		&infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID:  "a",
				Name: "cluster",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{},
		},
	}
	ociClusterAccessor.OCICluster.Spec.ControlPlaneEndpoint.Port = 6443
	ociClusterAccessor.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")

	cs, err := NewClusterScope(ClusterScopeParams{
		NetworkLoadBalancerClient: nlbClient,
		Cluster:                   &clusterv1.Cluster{},
		OCIClusterAccessor:        ociClusterAccessor,
		Client:                    client,
	})
	g.Expect(err).To(BeNil())

	secondaryPort := int32(9345)
	desiredNLB := infrastructurev1beta2.LoadBalancer{
		NLBSpec: infrastructurev1beta2.NLBSpec{
			BackendSets: []infrastructurev1beta2.NLBBackendSet{
				{Name: APIServerLBBackendSetName},
				{Name: "rollout-set", ListenerPort: &secondaryPort},
			},
		},
	}

	baseClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: common.String("nlb-id"),
	})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
		NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
			Listeners: map[string]networkloadbalancer.Listener{
				APIServerLBListener: {
					Name:                  common.String(APIServerLBListener),
					DefaultBackendSetName: common.String(APIServerLBBackendSetName),
					Port:                  common.Int(6443),
					Protocol:              networkloadbalancer.ListenerProtocolsTcp,
				},
			},
			BackendSets: map[string]networkloadbalancer.BackendSet{
				APIServerLBBackendSetName: {
					Name:             common.String(APIServerLBBackendSetName),
					Policy:           LoadBalancerPolicy,
					IsPreserveSource: common.Bool(false),
					HealthChecker: &networkloadbalancer.HealthChecker{
						Port:     common.Int(6443),
						Protocol: networkloadbalancer.HealthCheckProtocolsHttps,
						UrlPath:  common.String("/healthz"),
					},
				},
			},
		},
	}, nil)

	g.Expect(cs.reconcileNLBResources(context.Background(), desiredNLB)).To(Succeed())
}

func TestReconcileNLBResources_RefreshesBeforeDeletingStaleBackendSets(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	baseClient := mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
	nlbClient := &extendedMockNLBClient{
		MockNetworkLoadBalancerClient: baseClient,
		createBackendSetFn: func(_ context.Context, _ networkloadbalancer.CreateBackendSetRequest) (networkloadbalancer.CreateBackendSetResponse, error) {
			t.Fatalf("unexpected backend set create")
			return networkloadbalancer.CreateBackendSetResponse{}, nil
		},
		updateBackendSetFn: func(_ context.Context, _ networkloadbalancer.UpdateBackendSetRequest) (networkloadbalancer.UpdateBackendSetResponse, error) {
			t.Fatalf("unexpected backend set update")
			return networkloadbalancer.UpdateBackendSetResponse{}, nil
		},
		deleteBackendSetFn: func(_ context.Context, _ networkloadbalancer.DeleteBackendSetRequest) (networkloadbalancer.DeleteBackendSetResponse, error) {
			t.Fatalf("unexpected backend set delete")
			return networkloadbalancer.DeleteBackendSetResponse{}, nil
		},
		createListenerFn: func(_ context.Context, _ networkloadbalancer.CreateListenerRequest) (networkloadbalancer.CreateListenerResponse, error) {
			t.Fatalf("unexpected listener create")
			return networkloadbalancer.CreateListenerResponse{}, nil
		},
		updateListenerFn: func(_ context.Context, _ networkloadbalancer.UpdateListenerRequest) (networkloadbalancer.UpdateListenerResponse, error) {
			t.Fatalf("unexpected listener update")
			return networkloadbalancer.UpdateListenerResponse{}, nil
		},
		deleteListenerFn: func(_ context.Context, request networkloadbalancer.DeleteListenerRequest) (networkloadbalancer.DeleteListenerResponse, error) {
			if request.ListenerName == nil || *request.ListenerName != "stale-listener" {
				t.Fatalf("unexpected listener delete request: %#v", request)
			}
			return networkloadbalancer.DeleteListenerResponse{OpcWorkRequestId: common.String("wr-delete-listener")}, nil
		},
	}

	client := fake.NewClientBuilder().Build()
	ociClusterAccessor := OCISelfManagedCluster{
		&infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID:  "a",
				Name: "cluster",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{},
		},
	}
	ociClusterAccessor.OCICluster.Spec.ControlPlaneEndpoint.Port = 6443
	ociClusterAccessor.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlb-id")

	cs, err := NewClusterScope(ClusterScopeParams{
		NetworkLoadBalancerClient: nlbClient,
		Cluster:                   &clusterv1.Cluster{},
		OCIClusterAccessor:        ociClusterAccessor,
		Client:                    client,
	})
	g.Expect(err).To(BeNil())

	desiredNLB := infrastructurev1beta2.LoadBalancer{
		NLBSpec: infrastructurev1beta2.NLBSpec{
			BackendSets: []infrastructurev1beta2.NLBBackendSet{
				{Name: APIServerLBBackendSetName},
			},
		},
	}

	gomock.InOrder(
		baseClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: common.String("nlb-id"),
		})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
			NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
				Listeners: map[string]networkloadbalancer.Listener{
					APIServerLBListener: {
						Name:                  common.String(APIServerLBListener),
						DefaultBackendSetName: common.String(APIServerLBBackendSetName),
						Port:                  common.Int(6443),
						Protocol:              networkloadbalancer.ListenerProtocolsTcp,
					},
					"stale-listener": {Name: common.String("stale-listener")},
				},
				BackendSets: map[string]networkloadbalancer.BackendSet{
					APIServerLBBackendSetName: {
						Name:             common.String(APIServerLBBackendSetName),
						Policy:           LoadBalancerPolicy,
						IsPreserveSource: common.Bool(false),
						HealthChecker: &networkloadbalancer.HealthChecker{
							Port:     common.Int(6443),
							Protocol: networkloadbalancer.HealthCheckProtocolsHttps,
							UrlPath:  common.String("/healthz"),
						},
					},
					"stale-set": {Name: common.String("stale-set"), Backends: []networkloadbalancer.Backend{}},
				},
			},
		}, nil),
		baseClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
			WorkRequestId: common.String("wr-delete-listener"),
		})).Return(networkloadbalancer.GetWorkRequestResponse{
			WorkRequest: networkloadbalancer.WorkRequest{Status: networkloadbalancer.OperationStatusSucceeded},
		}, nil),
		baseClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: common.String("nlb-id"),
		})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
			NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
				Listeners: map[string]networkloadbalancer.Listener{
					APIServerLBListener: {
						Name:                  common.String(APIServerLBListener),
						DefaultBackendSetName: common.String(APIServerLBBackendSetName),
						Port:                  common.Int(6443),
						Protocol:              networkloadbalancer.ListenerProtocolsTcp,
					},
				},
				BackendSets: map[string]networkloadbalancer.BackendSet{
					APIServerLBBackendSetName: {
						Name:             common.String(APIServerLBBackendSetName),
						Policy:           LoadBalancerPolicy,
						IsPreserveSource: common.Bool(false),
						HealthChecker: &networkloadbalancer.HealthChecker{
							Port:     common.Int(6443),
							Protocol: networkloadbalancer.HealthCheckProtocolsHttps,
							UrlPath:  common.String("/healthz"),
						},
					},
					"stale-set": {
						Name: common.String("stale-set"),
						Backends: []networkloadbalancer.Backend{
							{Name: common.String("backend-1")},
						},
					},
				},
			},
		}, nil),
	)

	g.Expect(cs.reconcileNLBResources(context.Background(), desiredNLB)).To(Succeed())
}

func TestNLBDeletion(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		nlbClient          *mock_nlb.MockNetworkLoadBalancerClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
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
			NetworkLoadBalancerClient: nlbClient,
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
		testSpecificSetup   func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient)
	}{
		{
			name:          "nlb already deleted",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("nlb-id")
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
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("nlb-id")
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
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("nlb-id")
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
			matchError:          errors.New("work request to delete nlb failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				clusterScope.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId = common.String("nlb-id")
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlb-id"),
				})).
					Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
						NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
							Id:            common.String("nlb-id"),
							CompartmentId: common.String("test-compartment"),
							FreeformTags:  tags,
							DefinedTags:   make(map[string]map[string]interface{}),
							IsPrivate:     common.Bool(false),
							DisplayName:   common.String(fmt.Sprintf("%s-%s", "cluster", "apiserver")),
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
						CompartmentId: common.String("test-compartment"),
						Status:        networkloadbalancer.OperationStatusFailed,
					},
				}, nil)
				nlbClient.EXPECT().ListWorkRequestErrors(gomock.Any(), gomock.Eq(networkloadbalancer.ListWorkRequestErrorsRequest{
					WorkRequestId: common.String("opc-wr-id"),
					CompartmentId: common.String("test-compartment"),
				})).Return(networkloadbalancer.ListWorkRequestErrorsResponse{
					WorkRequestErrorCollection: networkloadbalancer.WorkRequestErrorCollection{
						Items: []networkloadbalancer.WorkRequestError{
							{
								Code:    common.String("OKE-001"),
								Message: common.String("Failed to delete NLB"),
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
			tc.testSpecificSetup(cs, nlbClient)
			err := cs.DeleteApiServerNLB(context.Background())
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
