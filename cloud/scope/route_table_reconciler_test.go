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
	"testing"

	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"

	"github.com/golang/mock/gomock"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/identity/mock_identity"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestClusterScope_ReconcileRouteTable(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)
	identityClient := mock_identity.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

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

	privateRouteRules := []core.RouteRule{
		{
			DestinationType: core.RouteRuleDestinationTypeCidrBlock,
			Destination:     common.String("0.0.0.0/0"),
			NetworkEntityId: common.String("ngw"),
			Description:     common.String("traffic to the internet"),
		},
		{
			DestinationType: core.RouteRuleDestinationTypeServiceCidrBlock,
			Destination:     common.String("all-iad-services-in-oracle-services-network"),
			NetworkEntityId: common.String("sgw"),
			Description:     common.String("traffic to OCI services"),
		},
	}

	publicRoutingRules := []core.RouteRule{
		{
			DestinationType: core.RouteRuleDestinationTypeCidrBlock,
			Destination:     common.String("0.0.0.0/0"),
			NetworkEntityId: common.String("igw"),
			Description:     common.String("traffic to/from internet"),
		},
	}

	vcnClient.EXPECT().GetVcn(gomock.Any(), gomock.Eq(core.GetVcnRequest{
		VcnId: common.String("vcn1"),
	})).
		Return(core.GetVcnResponse{
			Vcn: core.Vcn{
				Id: common.String("vcn1"),
			},
		}, nil).Times(2)

	vcnClient.EXPECT().GetRouteTable(gomock.Any(), gomock.Eq(core.GetRouteTableRequest{
		RtId: common.String("private"),
	})).
		Return(core.GetRouteTableResponse{
			RouteTable: core.RouteTable{
				Id:           common.String("private"),
				FreeformTags: tags,
				DefinedTags:  definedTagsInterface,
			},
		}, nil).Times(2)

	vcnClient.EXPECT().GetRouteTable(gomock.Any(), gomock.Eq(core.GetRouteTableRequest{
		RtId: common.String("public"),
	})).
		Return(core.GetRouteTableResponse{
			RouteTable: core.RouteTable{
				Id:           common.String("public"),
				FreeformTags: tags,
				DefinedTags:  definedTagsInterface,
			},
		}, nil)

	updatedTags := make(map[string]string)
	for k, v := range tags {
		updatedTags[k] = v
	}
	updatedTags["foo"] = "bar"
	vcnClient.EXPECT().ListRouteTables(gomock.Any(), gomock.Eq(core.ListRouteTablesRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("private-route-table"),
		VcnId:         common.String("vcn"),
	})).Return(
		core.ListRouteTablesResponse{
			Items: []core.RouteTable{
				{
					Id:           common.String("rt_id"),
					FreeformTags: tags,
				},
			}}, nil)

	vcnClient.EXPECT().ListRouteTables(gomock.Any(), gomock.Eq(core.ListRouteTablesRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("public-route-table"),
		VcnId:         common.String("vcn1"),
	})).Return(
		core.ListRouteTablesResponse{
			Items: []core.RouteTable{
				{
					Id: common.String("rt_id"),
				},
			}}, nil).Times(2)

	vcnClient.EXPECT().ListRouteTables(gomock.Any(), gomock.Eq(core.ListRouteTablesRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("private-route-table"),
		VcnId:         common.String("vcn1"),
	})).Return(
		core.ListRouteTablesResponse{
			Items: []core.RouteTable{
				{
					Id: common.String("rt_id"),
				},
			}}, nil).Times(3)

	vcnClient.EXPECT().CreateRouteTable(gomock.Any(), gomock.Eq(core.CreateRouteTableRequest{
		CreateRouteTableDetails: core.CreateRouteTableDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("private-route-table"),
			VcnId:         common.String("vcn1"),
			FreeformTags:  tags,
			DefinedTags:   definedTagsInterface,
			RouteRules:    privateRouteRules,
		},
	})).Return(
		core.CreateRouteTableResponse{
			RouteTable: core.RouteTable{
				Id: common.String("rt"),
			},
		}, nil).Times(2)

	vcnClient.EXPECT().CreateRouteTable(gomock.Any(), gomock.Eq(core.CreateRouteTableRequest{
		CreateRouteTableDetails: core.CreateRouteTableDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("public-route-table"),
			VcnId:         common.String("vcn1"),
			FreeformTags:  tags,
			DefinedTags:   definedTagsInterface,
			RouteRules:    publicRoutingRules,
		},
	})).Return(
		core.CreateRouteTableResponse{
			RouteTable: core.RouteTable{
				Id: common.String("rt"),
			},
		}, nil).Times(2)

	vcnClient.EXPECT().CreateRouteTable(gomock.Any(), gomock.Eq(core.CreateRouteTableRequest{
		CreateRouteTableDetails: core.CreateRouteTableDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("private-route-table"),
			VcnId:         common.String("vcn1"),
			FreeformTags:  tags,
			DefinedTags:   definedTagsInterface,
			RouteRules:    privateRouteRules,
		},
	})).Return(
		core.CreateRouteTableResponse{}, errors.New("some error"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "all subnets are private and route table doesn't exists",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							},
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
							},
						},
						ID: common.String("vcn1"),
						ServiceGateway: infrastructurev1beta2.ServiceGateway{
							Id: common.String("sgw"),
						},
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("ngw"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "all subnets are public and route table doesn't exists",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Public,
								Role: infrastructurev1beta2.ControlPlaneRole,
							},
							{
								Type: infrastructurev1beta2.Public,
								Role: infrastructurev1beta2.WorkerRole,
							},
						},
						ID: common.String("vcn1"),
						InternetGateway: infrastructurev1beta2.InternetGateway{
							Id: common.String("igw"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "subnets are public and private and route table doesn't exists",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn1"),
						InternetGateway: infrastructurev1beta2.InternetGateway{
							Id: common.String("igw"),
						},
						ServiceGateway: infrastructurev1beta2.ServiceGateway{
							Id: common.String("sgw"),
						},
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("ngw"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no update needed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				DefinedTags: definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						RouteTable: infrastructurev1beta2.RouteTable{
							PrivateRouteTableId: common.String("private"),
							PublicRouteTableId:  common.String("public"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "update needed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				FreeformTags: map[string]string{
					"foo": "bar",
				},
				DefinedTags: definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							},
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
							},
						},
						RouteTable: infrastructurev1beta2.RouteTable{
							PrivateRouteTableId: common.String("private"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "id not present in spec but found by name and no update",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				FreeformTags: map[string]string{
					"foo": "bar",
				},
				DefinedTags: definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							},
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "creation failed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn1"),
						ServiceGateway: infrastructurev1beta2.ServiceGateway{
							Id: common.String("sgw"),
						},
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("ngw"),
						},
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							},
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
							},
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed create route table: some error",
		},
		{
			name: "route table creation skip",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn1"),
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("ngw"),
						},
						ServiceGateway: infrastructurev1beta2.ServiceGateway{
							Id: common.String("sgw"),
						},
						RouteTable: infrastructurev1beta2.RouteTable{
							Skip: true,
						},
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta1.Private,
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
							},
							{
								Type: infrastructurev1beta1.Private,
								Role: infrastructurev1beta1.ServiceLoadBalancerRole,
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociClusterAccessor := OCISelfManagedCluster{
				&infrastructurev1beta2.OCICluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cluster_uid",
					},
					Spec: tt.spec,
				},
			}
			ociClusterAccessor.OCICluster.Spec.OCIResourceIdentifier = "resource_uid"
			s := &ClusterScope{
				IdentityClient:     identityClient,
				RegionIdentifier:   "ashburn",
				RegionKey:          "iad",
				VCNClient:          vcnClient,
				OCIClusterAccessor: ociClusterAccessor,
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cluster_uid",
					},
				},
				Logger: &l,
			}
			err := s.ReconcileRouteTable(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileRouteTable() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("ReconcileRouteTable() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestClusterScope_CreateRouteTable(t *testing.T) {
	type args struct {
		routeTableType string
		spec           infrastructurev1beta2.OCIClusterSpec
		setupMock      func(t *testing.T, vcn *mock_vcn.MockClient)
	}
	tests := []struct {
		name                 string
		args                 args
		wantErr              bool
		expectedErrorMessage string
	}{
		{
			name: "private without peering (NAT + Service routes only)",
			args: args{
				routeTableType: infrastructurev1beta2.Private,
				spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId: "comp",
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							ID: common.String("vcn1"),
							NATGateway: infrastructurev1beta2.NATGateway{
								Id: common.String("ngw"),
							},
							ServiceGateway: infrastructurev1beta2.ServiceGateway{
								Id: common.String("sgw"),
							},
						},
					},
				},
				setupMock: func(t *testing.T, vcn *mock_vcn.MockClient) {
					_ = core.CreateRouteTableRequest{
						CreateRouteTableDetails: core.CreateRouteTableDetails{
							VcnId:         common.String("vcn1"),
							CompartmentId: common.String("comp"),
							DisplayName:   common.String(PrivateRouteTableName),
							FreeformTags:  map[string]string{},
							DefinedTags:   map[string]map[string]interface{}{},
							RouteRules: []core.RouteRule{
								{
									DestinationType: core.RouteRuleDestinationTypeCidrBlock,
									Destination:     common.String("0.0.0.0/0"),
									NetworkEntityId: common.String("ngw"),
									Description:     common.String("traffic to the internet"),
								},
								{
									DestinationType: core.RouteRuleDestinationTypeServiceCidrBlock,
									Destination:     common.String("all-iad-services-in-oracle-services-network"),
									NetworkEntityId: common.String("sgw"),
									Description:     common.String("traffic to OCI services"),
								},
							},
						},
					}
					vcn.EXPECT().
						CreateRouteTable(gomock.Any(), gomock.Any()).
						DoAndReturn(func(_ context.Context, req core.CreateRouteTableRequest) (core.CreateRouteTableResponse, error) {
							// Validate core fields; ignore tags differences
							if req.CreateRouteTableDetails.VcnId == nil || *req.CreateRouteTableDetails.VcnId != "vcn1" {
								t.Errorf("expected VcnId=vcn1, got %v", req.CreateRouteTableDetails.VcnId)
							}
							if req.CreateRouteTableDetails.CompartmentId == nil || *req.CreateRouteTableDetails.CompartmentId != "comp" {
								t.Errorf("expected CompartmentId=comp, got %v", req.CreateRouteTableDetails.CompartmentId)
							}
							if req.CreateRouteTableDetails.DisplayName == nil || *req.CreateRouteTableDetails.DisplayName != PrivateRouteTableName {
								t.Errorf("expected DisplayName=%s, got %v", PrivateRouteTableName, req.CreateRouteTableDetails.DisplayName)
							}
							rr := req.CreateRouteTableDetails.RouteRules
							if len(rr) != 2 {
								t.Fatalf("expected 2 route rules, got %d", len(rr))
							}
							// NAT route
							if rr[0].Destination == nil || *rr[0].Destination != "0.0.0.0/0" {
								t.Errorf("expected rule[0] dest 0.0.0.0/0, got %v", rr[0].Destination)
							}
							if rr[0].NetworkEntityId == nil || *rr[0].NetworkEntityId != "ngw" {
								t.Errorf("expected rule[0] NEI=ngw, got %v", rr[0].NetworkEntityId)
							}
							// Service gateway route
							if rr[1].Destination == nil || *rr[1].Destination != "all-iad-services-in-oracle-services-network" {
								t.Errorf("expected rule[1] dest all-iad-services-in-oracle-services-network, got %v", rr[1].Destination)
							}
							if rr[1].NetworkEntityId == nil || *rr[1].NetworkEntityId != "sgw" {
								t.Errorf("expected rule[1] NEI=sgw, got %v", rr[1].NetworkEntityId)
							}
							return core.CreateRouteTableResponse{RouteTable: core.RouteTable{Id: common.String("rt")}}, nil
						})
				},
			},
			wantErr: false,
		},
		{
			name: "private with peering (adds DRG peer routes)",
			args: args{
				routeTableType: infrastructurev1beta2.Private,
				spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId: "comp",
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						VCNPeering: &infrastructurev1beta2.VCNPeering{
							DRG: &infrastructurev1beta2.DRG{ID: common.String("drg-id")},
							PeerRouteRules: []infrastructurev1beta2.PeerRouteRule{
								{VCNCIDRRange: "10.1.0.0/16"},
							},
						},
						Vcn: infrastructurev1beta2.VCN{
							ID: common.String("vcn1"),
							NATGateway: infrastructurev1beta2.NATGateway{
								Id: common.String("ngw"),
							},
							ServiceGateway: infrastructurev1beta2.ServiceGateway{
								Id: common.String("sgw"),
							},
						},
					},
				},
				setupMock: func(t *testing.T, vcn *mock_vcn.MockClient) {
					_ = core.CreateRouteTableRequest{
						CreateRouteTableDetails: core.CreateRouteTableDetails{
							VcnId:         common.String("vcn1"),
							CompartmentId: common.String("comp"),
							DisplayName:   common.String(PrivateRouteTableName),
							FreeformTags:  map[string]string{},
							DefinedTags:   map[string]map[string]interface{}{},
							RouteRules: []core.RouteRule{
								{
									DestinationType: core.RouteRuleDestinationTypeCidrBlock,
									Destination:     common.String("0.0.0.0/0"),
									NetworkEntityId: common.String("ngw"),
									Description:     common.String("traffic to the internet"),
								},
								{
									DestinationType: core.RouteRuleDestinationTypeServiceCidrBlock,
									Destination:     common.String("all-iad-services-in-oracle-services-network"),
									NetworkEntityId: common.String("sgw"),
									Description:     common.String("traffic to OCI services"),
								},
								{
									DestinationType: core.RouteRuleDestinationTypeCidrBlock,
									Destination:     common.String("10.1.0.0/16"),
									NetworkEntityId: common.String("drg-id"),
									Description:     common.String("traffic to peer DRG"),
								},
							},
						},
					}
					vcn.EXPECT().
						CreateRouteTable(gomock.Any(), gomock.Any()).
						DoAndReturn(func(_ context.Context, req core.CreateRouteTableRequest) (core.CreateRouteTableResponse, error) {
							// Validate core fields; ignore tags differences
							if req.CreateRouteTableDetails.VcnId == nil || *req.CreateRouteTableDetails.VcnId != "vcn1" {
								t.Errorf("expected VcnId=vcn1, got %v", req.CreateRouteTableDetails.VcnId)
							}
							if req.CreateRouteTableDetails.CompartmentId == nil || *req.CreateRouteTableDetails.CompartmentId != "comp" {
								t.Errorf("expected CompartmentId=comp, got %v", req.CreateRouteTableDetails.CompartmentId)
							}
							if req.CreateRouteTableDetails.DisplayName == nil || *req.CreateRouteTableDetails.DisplayName != PrivateRouteTableName {
								t.Errorf("expected DisplayName=%s, got %v", PrivateRouteTableName, req.CreateRouteTableDetails.DisplayName)
							}
							rr := req.CreateRouteTableDetails.RouteRules
							if len(rr) != 3 {
								t.Fatalf("expected 3 route rules, got %d", len(rr))
							}
							// NAT route
							if rr[0].Destination == nil || *rr[0].Destination != "0.0.0.0/0" {
								t.Errorf("expected rule[0] dest 0.0.0.0/0, got %v", rr[0].Destination)
							}
							if rr[0].NetworkEntityId == nil || *rr[0].NetworkEntityId != "ngw" {
								t.Errorf("expected rule[0] NEI=ngw, got %v", rr[0].NetworkEntityId)
							}
							// Service gateway route
							if rr[1].Destination == nil || *rr[1].Destination != "all-iad-services-in-oracle-services-network" {
								t.Errorf("expected rule[1] dest all-iad-services-in-oracle-services-network, got %v", rr[1].Destination)
							}
							if rr[1].NetworkEntityId == nil || *rr[1].NetworkEntityId != "sgw" {
								t.Errorf("expected rule[1] NEI=sgw, got %v", rr[1].NetworkEntityId)
							}
							// DRG peering route
							if rr[2].Destination == nil || *rr[2].Destination != "10.1.0.0/16" {
								t.Errorf("expected rule[2] dest 10.1.0.0/16, got %v", rr[2].Destination)
							}
							if rr[2].NetworkEntityId == nil || *rr[2].NetworkEntityId != "drg-id" {
								t.Errorf("expected rule[2] NEI=drg-id, got %v", rr[2].NetworkEntityId)
							}
							return core.CreateRouteTableResponse{RouteTable: core.RouteTable{Id: common.String("rt")}}, nil
						})
				},
			},
			wantErr: false,
		},
		{
			name: "private with peering but DRG is nil",
			args: args{
				routeTableType: infrastructurev1beta2.Private,
				spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId: "comp",
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						VCNPeering: &infrastructurev1beta2.VCNPeering{
							DRG:                      nil, // Intentionally set to nil
							PeerRouteRules:           []infrastructurev1beta2.PeerRouteRule{{VCNCIDRRange: "10.1.0.0/16"}},
							RemotePeeringConnections: nil,
						},
						Vcn: infrastructurev1beta2.VCN{
							ID: common.String("vcn1"),
							NATGateway: infrastructurev1beta2.NATGateway{
								Id: common.String("ngw"),
							},
							ServiceGateway: infrastructurev1beta2.ServiceGateway{
								Id: common.String("sgw"),
							},
						},
					},
				},
				setupMock: func(t *testing.T, vcn *mock_vcn.MockClient) {
					// No CreateRouteTable call expected because we error out before
					vcn.EXPECT().CreateRouteTable(gomock.Any(), gomock.Any()).Times(0)
				},
			},
			wantErr:              true,
			expectedErrorMessage: "Create Route Table: DRG has not been specified",
		},
		{
			name: "private with peering but DRG ID is nil",
			args: args{
				routeTableType: infrastructurev1beta2.Private,
				spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId: "comp",
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						VCNPeering: &infrastructurev1beta2.VCNPeering{
							DRG:            &infrastructurev1beta2.DRG{ID: nil},
							PeerRouteRules: []infrastructurev1beta2.PeerRouteRule{{VCNCIDRRange: "10.1.0.0/16"}},
						},
						Vcn: infrastructurev1beta2.VCN{
							ID: common.String("vcn1"),
							NATGateway: infrastructurev1beta2.NATGateway{
								Id: common.String("ngw"),
							},
							ServiceGateway: infrastructurev1beta2.ServiceGateway{
								Id: common.String("sgw"),
							},
						},
					},
				},
				setupMock: func(t *testing.T, vcn *mock_vcn.MockClient) {
					// No CreateRouteTable call expected because we error out before
					vcn.EXPECT().CreateRouteTable(gomock.Any(), gomock.Any()).Times(0)
				},
			},
			wantErr:              true,
			expectedErrorMessage: "Create Route Table: DRG ID has not been set",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			vcnClient := mock_vcn.NewMockClient(mockCtrl)
			if tt.args.setupMock != nil {
				tt.args.setupMock(t, vcnClient)
			}

			l := log.FromContext(context.Background())
			ociClusterAccessor := OCISelfManagedCluster{
				OCICluster: &infrastructurev1beta2.OCICluster{
					ObjectMeta: metav1.ObjectMeta{UID: "cluster_uid"},
					Spec:       tt.args.spec,
				},
			}
			s := &ClusterScope{
				VCNClient:          vcnClient,
				OCIClusterAccessor: ociClusterAccessor,
				Logger:             &l,
				RegionKey:          "iad",
			}

			_, err := s.CreateRouteTable(context.Background(), tt.args.routeTableType)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateRouteTable() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.expectedErrorMessage != "" && err.Error() != tt.expectedErrorMessage {
				t.Fatalf("CreateRouteTable() expected error = %q, got %q", tt.expectedErrorMessage, err.Error())
			}
		})
	}
}

func TestClusterScope_DeleteRouteTables(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	vcnClient.EXPECT().GetRouteTable(gomock.Any(), gomock.Eq(core.GetRouteTableRequest{
		RtId: common.String("private_id"),
	})).
		Return(core.GetRouteTableResponse{
			RouteTable: core.RouteTable{
				Id:           common.String("private_id"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetRouteTable(gomock.Any(), gomock.Eq(core.GetRouteTableRequest{
		RtId: common.String("public_id"),
	})).
		Return(core.GetRouteTableResponse{
			RouteTable: core.RouteTable{
				Id:           common.String("public_id"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetRouteTable(gomock.Any(), gomock.Eq(core.GetRouteTableRequest{
		RtId: common.String("private_id_error_delete"),
	})).
		Return(core.GetRouteTableResponse{
			RouteTable: core.RouteTable{
				Id:           common.String("private_id_error_delete"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetRouteTable(gomock.Any(), gomock.Eq(core.GetRouteTableRequest{
		RtId: common.String("private_id_error")})).
		Return(core.GetRouteTableResponse{}, errors.New("some error in GetRouteTable"))

	vcnClient.EXPECT().GetRouteTable(gomock.Any(), gomock.Eq(core.GetRouteTableRequest{RtId: common.String("rt_deleted")})).
		Return(core.GetRouteTableResponse{}, errors.New("not found"))
	vcnClient.EXPECT().DeleteRouteTable(gomock.Any(), gomock.Eq(core.DeleteRouteTableRequest{
		RtId: common.String("private_id"),
	})).
		Return(core.DeleteRouteTableResponse{}, nil)
	vcnClient.EXPECT().DeleteRouteTable(gomock.Any(), gomock.Eq(core.DeleteRouteTableRequest{
		RtId: common.String("public_id"),
	})).
		Return(core.DeleteRouteTableResponse{}, nil)
	vcnClient.EXPECT().DeleteRouteTable(gomock.Any(), gomock.Eq(core.DeleteRouteTableRequest{
		RtId: common.String("private_id_error_delete"),
	})).
		Return(core.DeleteRouteTableResponse{}, errors.New("some error in DeleteRouteTable"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete route table is successful",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						RouteTable: infrastructurev1beta2.RouteTable{
							PrivateRouteTableId: common.String("private_id"),
							PublicRouteTableId:  common.String("public_id"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "route table already deleted",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							},
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
							},
						},
						RouteTable: infrastructurev1beta2.RouteTable{
							PrivateRouteTableId: common.String("rt_deleted"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete route table error when calling get route table",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							},
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
							},
						},
						RouteTable: infrastructurev1beta2.RouteTable{
							PrivateRouteTableId: common.String("private_id_error"),
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "some error in GetRouteTable",
		},
		{
			name: "delete route table error when calling delete route table",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							},
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
							},
						},
						RouteTable: infrastructurev1beta2.RouteTable{
							PrivateRouteTableId: common.String("private_id_error_delete"),
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to delete route table: some error in DeleteRouteTable",
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociClusterAccessor := OCISelfManagedCluster{
				&infrastructurev1beta2.OCICluster{
					Spec: tt.spec,
					ObjectMeta: metav1.ObjectMeta{
						UID: "cluster_uid",
					},
				},
			}
			ociClusterAccessor.OCICluster.Spec.OCIResourceIdentifier = "resource_uid"
			s := &ClusterScope{
				VCNClient:          vcnClient,
				OCIClusterAccessor: ociClusterAccessor,
				Logger:             &l,
			}
			err := s.DeleteRouteTables(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteRouteTables() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("DeleteRouteTables() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}
}
