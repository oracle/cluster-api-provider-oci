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
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDRGRPCAttachmentReconciliation(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		vcnClient          *mock_vcn.MockClient
		peerVcnClient      *mock_vcn.MockClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
		vcnPeering         infrastructurev1beta1.VCNPeering
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		vcnClient = mock_vcn.NewMockClient(mockCtrl)
		peerVcnClient = mock_vcn.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociClusterAccessor = OCISelfManagedCluster{
			&infrastructurev1beta1.OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					UID:  "cluster_uid",
					Name: "cluster",
				},
				Spec: infrastructurev1beta1.OCIClusterSpec{
					CompartmentId:         "compartment-id",
					OCIResourceIdentifier: "resource_uid",
				},
			},
		}

		mockClients := MockOCIClients{
			VCNClient: peerVcnClient,
		}
		mockProvider, err := MockNewClientProvider(mockClients)
		g.Expect(err).To(BeNil())
		ociClusterAccessor.OCICluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			VCNClient:          vcnClient,
			Cluster:            &clusterv1.Cluster{},
			OCIClusterAccessor: ociClusterAccessor,
			Client:             client,
			ClientProvider:     mockProvider,
		})
		tags = make(map[string]string)
		tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
		tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
		vcnPeering = infrastructurev1beta1.VCNPeering{}
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
		testSpecificSetup   func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient)
	}{
		{
			name:          "peering disabled",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
			},
		},
		{
			name:          "create local rpc",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:       false,
						PeerDRGId:           common.String("peer-drg-id"),
						PeerRPCConnectionId: common.String("peer-connection-id"),
						PeerRegionName:      "us-sanjose-1",
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)
				vcnClient.EXPECT().ConnectRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ConnectRemotePeeringConnectionsRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
					ConnectRemotePeeringConnectionsDetails: core.ConnectRemotePeeringConnectionsDetails{
						PeerId:         common.String("peer-connection-id"),
						PeerRegionName: common.String("us-sanjose-1"),
					},
				})).
					Return(core.ConnectRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:            common.String("local-connection-id"),
							PeeringStatus: core.RemotePeeringConnectionPeeringStatusPeered,
						},
					}, nil)
			},
		},
		{
			name:          "create remote rpc",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  true,
						PeerDRGId:      common.String("peer-drg-id"),
						PeerRegionName: MockTestRegion,
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)

				peerVcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("peer-drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				peerVcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("peer-drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("peer-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
						},
					}, nil)
				peerVcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("peer-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("peer-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)

				vcnClient.EXPECT().ConnectRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ConnectRemotePeeringConnectionsRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
					ConnectRemotePeeringConnectionsDetails: core.ConnectRemotePeeringConnectionsDetails{
						PeerId:         common.String("peer-connection-id"),
						PeerRegionName: common.String(MockTestRegion),
					},
				})).
					Return(core.ConnectRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:            common.String("local-connection-id"),
							PeeringStatus: core.RemotePeeringConnectionPeeringStatusPeered,
						},
					}, nil)
			},
		},
		{
			name:                "drg not provided",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("DRG has not been specified"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
			},
		},
		{
			name:                "drg id not provided",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("DRG ID has not been set"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
			},
		},
		{
			name:                "peer drg id not provided",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("peer DRG ID has not been specified"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  true,
						PeerRegionName: "us-sanjose-1",
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)
			},
		},
		{
			name:                "peer rpc id not provided",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("peer RPC Connection ID is empty"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  false,
						PeerDRGId:      common.String("peer-drg-id"),
						PeerRegionName: "us-sanjose-1",
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)
			},
		},
		{
			name:                "list rpc error",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  true,
						PeerDRGId:      common.String("peer-drg-id"),
						PeerRegionName: "us-sanjose-1",
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "create rpc error",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  true,
						PeerDRGId:      common.String("peer-drg-id"),
						PeerRegionName: "us-sanjose-1",
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, errors.New("request failed"))
			},
		},
		{
			name:                "get rpc error",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  true,
						PeerDRGId:      common.String("peer-drg-id"),
						PeerRegionName: "us-sanjose-1",
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, nil)

				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, errors.New("request failed"))
			},
		},
		{
			name:                "create remote rpc error",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  true,
						PeerDRGId:      common.String("peer-drg-id"),
						PeerRegionName: MockTestRegion,
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)

				peerVcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("peer-drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				peerVcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("peer-drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "create remote rpc fetch error",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  true,
						PeerDRGId:      common.String("peer-drg-id"),
						PeerRegionName: MockTestRegion,
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)

				peerVcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("peer-drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				peerVcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("peer-drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("peer-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
						},
					}, nil)
				peerVcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("peer-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "create connection error",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  true,
						PeerDRGId:      common.String("peer-drg-id"),
						PeerRegionName: MockTestRegion,
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)

				peerVcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("peer-drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				peerVcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("peer-drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("peer-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
						},
					}, nil)
				peerVcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("peer-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("peer-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)

				vcnClient.EXPECT().ConnectRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ConnectRemotePeeringConnectionsRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
					ConnectRemotePeeringConnectionsDetails: core.ConnectRemotePeeringConnectionsDetails{
						PeerId:         common.String("peer-connection-id"),
						PeerRegionName: common.String(MockTestRegion),
					},
				})).
					Return(core.ConnectRemotePeeringConnectionsResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "invalid peering status",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("invalid peering status INVALID of RPC local-connection-id"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:  true,
						PeerDRGId:      common.String("peer-drg-id"),
						PeerRegionName: MockTestRegion,
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
							PeeringStatus:  core.RemotePeeringConnectionPeeringStatusNew,
						},
					}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)

				peerVcnClient.EXPECT().ListRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ListRemotePeeringConnectionsRequest{
					DrgId:         common.String("peer-drg-id"),
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListRemotePeeringConnectionsResponse{}, nil)
				peerVcnClient.EXPECT().CreateRemotePeeringConnection(gomock.Any(), gomock.Eq(core.CreateRemotePeeringConnectionRequest{
					CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
						DrgId:         common.String("peer-drg-id"),
						CompartmentId: common.String("compartment-id"),
						DisplayName:   common.String("cluster"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("peer-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateProvisioning,
						},
					}, nil)
				peerVcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("peer-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("peer-connection-id"),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateAvailable,
						},
					}, nil)

				vcnClient.EXPECT().ConnectRemotePeeringConnections(gomock.Any(), gomock.Eq(core.ConnectRemotePeeringConnectionsRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
					ConnectRemotePeeringConnectionsDetails: core.ConnectRemotePeeringConnectionsDetails{
						PeerId:         common.String("peer-connection-id"),
						PeerRegionName: common.String(MockTestRegion),
					},
				})).
					Return(core.ConnectRemotePeeringConnectionsResponse{}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:            common.String("local-connection-id"),
							PeeringStatus: core.RemotePeeringConnectionPeeringStatusInvalid,
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
			tc.testSpecificSetup(cs, vcnClient)
			err := cs.ReconcileDRGRPCAttachment(context.Background())
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

func TestDRGRPCAttachmentDeletion(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		vcnClient          *mock_vcn.MockClient
		peerVcnClient      *mock_vcn.MockClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
		vcnPeering         infrastructurev1beta1.VCNPeering
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		vcnClient = mock_vcn.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociClusterAccessor = OCISelfManagedCluster{
			&infrastructurev1beta1.OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					UID:  "cluster_uid",
					Name: "cluster",
				},
				Spec: infrastructurev1beta1.OCIClusterSpec{
					CompartmentId:         "compartment-id",
					OCIResourceIdentifier: "resource_uid",
				},
			},
		}

		peerVcnClient = mock_vcn.NewMockClient(mockCtrl)
		mockClients := MockOCIClients{
			VCNClient: peerVcnClient,
		}
		mockProvider, err := MockNewClientProvider(mockClients)
		g.Expect(err).To(BeNil())

		ociClusterAccessor.OCICluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			VCNClient:          vcnClient,
			Cluster:            &clusterv1.Cluster{},
			OCIClusterAccessor: ociClusterAccessor,
			Client:             client,
			ClientProvider:     mockProvider,
		})
		tags = make(map[string]string)
		tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
		tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
		vcnPeering = infrastructurev1beta1.VCNPeering{}
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
		testSpecificSetup   func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient)
	}{
		{
			name:          "vcn peering disabled",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
			},
		},
		{
			name:          "delete local rpc",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:       false,
						PeerDRGId:           common.String("peer-drg-id"),
						PeerRPCConnectionId: common.String("peer-connection-id"),
						PeerRegionName:      "us-sanjose-1",
						RPCConnectionId:     common.String("local-connection-id"),
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:           common.String("local-connection-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
				vcnClient.EXPECT().DeleteRemotePeeringConnection(gomock.Any(), gomock.Eq(core.DeleteRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.DeleteRemotePeeringConnectionResponse{}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateTerminated,
						},
					}, nil)
			},
		},
		{
			name:          "delete remote rpc",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:       true,
						PeerDRGId:           common.String("peer-drg-id"),
						PeerRPCConnectionId: common.String("peer-connection-id"),
						PeerRegionName:      MockTestRegion,
						RPCConnectionId:     common.String("local-connection-id"),
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:           common.String("local-connection-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
				vcnClient.EXPECT().DeleteRemotePeeringConnection(gomock.Any(), gomock.Eq(core.DeleteRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.DeleteRemotePeeringConnectionResponse{}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateTerminated,
						},
					}, nil)

				peerVcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("peer-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:           common.String("peer-connection-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
				peerVcnClient.EXPECT().DeleteRemotePeeringConnection(gomock.Any(), gomock.Eq(core.DeleteRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("peer-connection-id"),
				})).
					Return(core.DeleteRemotePeeringConnectionResponse{}, nil)
				peerVcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("peer-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("peer-connection-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateTerminated,
						},
					}, nil)
			},
		},
		{
			name:                "delete local rpc failure",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:       true,
						PeerDRGId:           common.String("peer-drg-id"),
						PeerRPCConnectionId: common.String("peer-connection-id"),
						PeerRegionName:      "us-sanjose-1",
						RPCConnectionId:     common.String("local-connection-id"),
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:           common.String("local-connection-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
				vcnClient.EXPECT().DeleteRemotePeeringConnection(gomock.Any(), gomock.Eq(core.DeleteRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.DeleteRemotePeeringConnectionResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "delete peer rpc failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.RemotePeeringConnections = []infrastructurev1beta1.RemotePeeringConnection{
					{
						ManagePeerRPC:       true,
						PeerDRGId:           common.String("peer-drg-id"),
						PeerRPCConnectionId: common.String("peer-connection-id"),
						PeerRegionName:      MockTestRegion,
						RPCConnectionId:     common.String("local-connection-id"),
					},
				}
				ociClusterAccessor.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:           common.String("local-connection-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
				vcnClient.EXPECT().DeleteRemotePeeringConnection(gomock.Any(), gomock.Eq(core.DeleteRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.DeleteRemotePeeringConnectionResponse{}, nil)
				vcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("local-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:             common.String("local-connection-id"),
							FreeformTags:   tags,
							DefinedTags:    make(map[string]map[string]interface{}),
							LifecycleState: core.RemotePeeringConnectionLifecycleStateTerminated,
						},
					}, nil)

				peerVcnClient.EXPECT().GetRemotePeeringConnection(gomock.Any(), gomock.Eq(core.GetRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("peer-connection-id"),
				})).
					Return(core.GetRemotePeeringConnectionResponse{
						RemotePeeringConnection: core.RemotePeeringConnection{
							Id:           common.String("peer-connection-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
				peerVcnClient.EXPECT().DeleteRemotePeeringConnection(gomock.Any(), gomock.Eq(core.DeleteRemotePeeringConnectionRequest{
					RemotePeeringConnectionId: common.String("peer-connection-id"),
				})).
					Return(core.DeleteRemotePeeringConnectionResponse{}, errors.New("request failed"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, vcnClient)
			err := cs.DeleteDRGRPCAttachment(context.Background())
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
