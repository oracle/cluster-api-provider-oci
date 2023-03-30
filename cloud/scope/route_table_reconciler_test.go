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
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"testing"

	"github.com/golang/mock/gomock"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/identity/mock_identity"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
