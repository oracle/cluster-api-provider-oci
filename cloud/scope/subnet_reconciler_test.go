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
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestClusterScope_ReconcileSubnet(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

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

	customIngress := []infrastructurev1beta2.IngressSecurityRule{
		{
			Description: common.String("test-ingress"),
			Protocol:    common.String("8"),
			TcpOptions: &infrastructurev1beta2.TcpOptions{
				DestinationPortRange: &infrastructurev1beta2.PortRange{
					Max: common.Int(123),
					Min: common.Int(234),
				},
			},
			SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
			Source:     common.String("1.1.1.1/1"),
		},
	}
	customEgress := []infrastructurev1beta2.EgressSecurityRule{
		{
			Description: common.String("test-egress"),
			Protocol:    common.String("1"),
			TcpOptions: &infrastructurev1beta2.TcpOptions{
				DestinationPortRange: &infrastructurev1beta2.PortRange{
					Max: common.Int(345),
					Min: common.Int(567),
				},
			},
			DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
			Destination:     common.String("2.2.2.2/2"),
		},
	}

	customIngress_updated := []infrastructurev1beta2.IngressSecurityRule{
		{
			Description: common.String("test-ingress-updated"),
			Protocol:    common.String("7"),
			TcpOptions: &infrastructurev1beta2.TcpOptions{
				DestinationPortRange: &infrastructurev1beta2.PortRange{
					Max: common.Int(789),
					Min: common.Int(343),
				},
			},
			SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
			Source:     common.String("1.1.1.2/1"),
		},
	}

	tests := []struct {
		name              string
		spec              infrastructurev1beta2.OCIClusterSpec
		wantErr           bool
		expectedError     string
		testSpecificSetup func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient)
	}{
		{
			name: "subnet reconciliation successful - one creation - one update - one no update - one security list " +
				"creation - one security list update",
			spec: infrastructurev1beta2.OCIClusterSpec{
				DefinedTags:   definedTags,
				CompartmentId: "foo",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								Name: "creation_needed",
								CIDR: "2.2.2.2/10",
								Type: infrastructurev1beta2.Private,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name:         "test-cp-endpoint-seclist",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
								DnsLabel: common.String("label"),
							},
							{
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
								ID:   common.String("update_needed_id"),
								Name: "update_needed",
								Type: infrastructurev1beta2.Private,
								CIDR: "10.0.0.32/27",
							},
							{
								Role: infrastructurev1beta2.WorkerRole,
								ID:   common.String("sec_list_added_id"),
								Name: "sec_list_added",
								CIDR: "10.0.64.0/20",
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name:         "test-cp-endpoint-seclist",
									IngressRules: customIngress,
									EgressRules:  customEgress,
								},
							},
							{
								Role: infrastructurev1beta2.WorkerRole,
								ID:   common.String("sec_list_updated_id"),
								Name: "sec_list_added",
								CIDR: "10.0.64.0/20",
								SecurityList: &infrastructurev1beta2.SecurityList{
									ID:           common.String("seclist_id"),
									Name:         "update_seclist",
									IngressRules: customIngress,
									EgressRules:  customEgress,
								},
							},
						},
						RouteTable: infrastructurev1beta2.RouteTable{
							PrivateRouteTableId: common.String("private"),
						},
					},
				},
			},
			wantErr: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				vcnClient.EXPECT().GetVcn(gomock.Any(), gomock.Eq(core.GetVcnRequest{
					VcnId: common.String("vcn"),
				})).
					Return(core.GetVcnResponse{
						Vcn: core.Vcn{
							Id: common.String("vcn"),
						},
					}, nil).Times(2)
				vcnClient.EXPECT().ListSubnets(gomock.Any(), gomock.Eq(core.ListSubnetsRequest{
					CompartmentId: common.String("foo"),
					DisplayName:   common.String("creation_needed"),
					VcnId:         common.String("vcn"),
				})).Return(
					core.ListSubnetsResponse{}, nil)
				vcnClient.EXPECT().CreateSecurityList(gomock.Any(), gomock.Eq(core.CreateSecurityListRequest{
					CreateSecurityListDetails: core.CreateSecurityListDetails{
						CompartmentId: common.String("foo"),
						EgressSecurityRules: []core.EgressSecurityRule{
							{
								Description: common.String("test-egress"),
								Protocol:    common.String("1"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(345),
										Min: common.Int(567),
									},
								},
								DestinationType: core.EgressSecurityRuleDestinationTypeCidrBlock,
								Destination:     common.String("2.2.2.2/2"),
								IsStateless:     common.Bool(false),
							},
						},
						IngressSecurityRules: []core.IngressSecurityRule{
							{
								Description: common.String("test-ingress"),
								Protocol:    common.String("8"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(123),
										Min: common.Int(234),
									},
								},
								SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String("1.1.1.1/1"),
								IsStateless: common.Bool(false),
							},
						},
						VcnId:        common.String("vcn"),
						DefinedTags:  definedTagsInterface,
						DisplayName:  common.String("test-cp-endpoint-seclist"),
						FreeformTags: tags,
					},
				})).Return(
					core.CreateSecurityListResponse{
						SecurityList: core.SecurityList{
							Id: common.String("sec_list_id"),
						},
					}, nil)
				vcnClient.EXPECT().CreateSubnet(gomock.Any(), gomock.Eq(core.CreateSubnetRequest{
					CreateSubnetDetails: core.CreateSubnetDetails{
						CidrBlock:               common.String("2.2.2.2/10"),
						CompartmentId:           common.String("foo"),
						VcnId:                   common.String("vcn"),
						DefinedTags:             definedTagsInterface,
						DisplayName:             common.String("creation_needed"),
						FreeformTags:            tags,
						ProhibitInternetIngress: common.Bool(true),
						ProhibitPublicIpOnVnic:  common.Bool(true),
						RouteTableId:            common.String("private"),
						SecurityListIds:         []string{"sec_list_id"},
						DnsLabel:                common.String("label"),
					},
				})).Return(
					core.CreateSubnetResponse{
						Subnet: core.Subnet{
							Id: common.String("subnet"),
						},
					}, nil)
				vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
					SubnetId: common.String("update_needed_id"),
				})).
					Return(core.GetSubnetResponse{
						Subnet: core.Subnet{
							Id:           common.String("update_needed_id"),
							FreeformTags: tags,
							DefinedTags:  definedTagsInterface,
							DisplayName:  common.String("bar"),
							CidrBlock:    common.String("1.2.3.4/5"),
						},
					}, nil)
				vcnClient.EXPECT().UpdateSubnet(gomock.Any(), gomock.Eq(core.UpdateSubnetRequest{
					SubnetId: common.String("update_needed_id"),
					UpdateSubnetDetails: core.UpdateSubnetDetails{
						DisplayName: common.String("update_needed"),
						CidrBlock:   common.String(ServiceLoadBalancerDefaultCIDR),
					},
				})).
					Return(core.UpdateSubnetResponse{
						Subnet: core.Subnet{
							Id: common.String("private"),
						},
					}, nil)

				vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
					SubnetId: common.String("sec_list_added_id"),
				})).
					Return(core.GetSubnetResponse{
						Subnet: core.Subnet{
							Id:              common.String("sec_list_added_id"),
							DisplayName:     common.String("sec_list_added"),
							FreeformTags:    tags,
							DefinedTags:     definedTagsInterface,
							CidrBlock:       common.String(WorkerSubnetDefaultCIDR),
							SecurityListIds: []string{"foo"},
						},
					}, nil)
				vcnClient.EXPECT().UpdateSubnet(gomock.Any(), gomock.Eq(core.UpdateSubnetRequest{
					SubnetId: common.String("sec_list_added_id"),
					UpdateSubnetDetails: core.UpdateSubnetDetails{
						DisplayName:     common.String("sec_list_added"),
						CidrBlock:       common.String(WorkerSubnetDefaultCIDR),
						SecurityListIds: []string{"sec_list_id"},
					},
				})).
					Return(core.UpdateSubnetResponse{
						Subnet: core.Subnet{
							Id: common.String("private"),
						},
					}, nil)

				vcnClient.EXPECT().ListSecurityLists(gomock.Any(), gomock.Eq(core.ListSecurityListsRequest{
					CompartmentId: common.String("foo"),
					VcnId:         common.String("vcn"),
					DisplayName:   common.String("test-cp-endpoint-seclist"),
				})).Return(core.ListSecurityListsResponse{}, nil)

				vcnClient.EXPECT().CreateSecurityList(gomock.Any(), gomock.Eq(core.CreateSecurityListRequest{
					CreateSecurityListDetails: core.CreateSecurityListDetails{
						CompartmentId: common.String("foo"),
						EgressSecurityRules: []core.EgressSecurityRule{
							{
								Description: common.String("test-egress"),
								Protocol:    common.String("1"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(345),
										Min: common.Int(567),
									},
								},
								DestinationType: core.EgressSecurityRuleDestinationTypeCidrBlock,
								Destination:     common.String("2.2.2.2/2"),
								IsStateless:     common.Bool(false),
							},
						},
						IngressSecurityRules: []core.IngressSecurityRule{
							{
								Description: common.String("test-ingress"),
								Protocol:    common.String("8"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(123),
										Min: common.Int(234),
									},
								},
								SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String("1.1.1.1/1"),
								IsStateless: common.Bool(false),
							},
						},
						VcnId:        common.String("vcn"),
						DefinedTags:  definedTagsInterface,
						DisplayName:  common.String("test-cp-endpoint-seclist"),
						FreeformTags: tags,
					},
				})).Return(
					core.CreateSecurityListResponse{
						SecurityList: core.SecurityList{
							Id: common.String("sec_list_id"),
						},
					}, nil)

				vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
					SubnetId: common.String("sec_list_updated_id"),
				})).
					Return(core.GetSubnetResponse{
						Subnet: core.Subnet{
							Id:              common.String("sec_list_added_id"),
							DisplayName:     common.String("sec_list_added"),
							FreeformTags:    tags,
							DefinedTags:     definedTagsInterface,
							CidrBlock:       common.String(WorkerSubnetDefaultCIDR),
							SecurityListIds: []string{"seclist_id"},
						},
					}, nil)

				vcnClient.EXPECT().GetSecurityList(gomock.Any(), gomock.Eq(core.GetSecurityListRequest{
					SecurityListId: common.String("seclist_id"),
				})).
					Return(core.GetSecurityListResponse{
						SecurityList: core.SecurityList{
							DisplayName:  common.String("bar"),
							Id:           common.String("seclist_id"),
							FreeformTags: tags,
						},
					}, nil)

				vcnClient.EXPECT().UpdateSecurityList(gomock.Any(), gomock.Eq(core.UpdateSecurityListRequest{
					SecurityListId: common.String("seclist_id"),
					UpdateSecurityListDetails: core.UpdateSecurityListDetails{
						DisplayName: common.String("update_seclist"),
						EgressSecurityRules: []core.EgressSecurityRule{
							{
								Description: common.String("test-egress"),
								Protocol:    common.String("1"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(345),
										Min: common.Int(567),
									},
								},
								DestinationType: core.EgressSecurityRuleDestinationTypeCidrBlock,
								Destination:     common.String("2.2.2.2/2"),
								IsStateless:     common.Bool(false),
							},
						},
						IngressSecurityRules: []core.IngressSecurityRule{
							{
								Description: common.String("test-ingress"),
								Protocol:    common.String("8"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(123),
										Min: common.Int(234),
									},
								},
								SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String("1.1.1.1/1"),
								IsStateless: common.Bool(false),
							},
						},
					},
				})).Return(core.UpdateSecurityListResponse{
					SecurityList: core.SecurityList{
						Id: common.String("bar"),
					},
				}, nil)
			},
		},
		{
			name: "update subnet error",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								ID:   common.String("update_subnet_error"),
								Name: "update",
								CIDR: "2.2.2.2/1",
							},
						},
					},
				},
				DefinedTags: definedTags,
			},
			wantErr:       true,
			expectedError: "failed to reconcile the subnet, failed to update: some error",
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
					SubnetId: common.String("update_subnet_error"),
				})).
					Return(core.GetSubnetResponse{
						Subnet: core.Subnet{
							CidrBlock:    common.String("1.1.1.1/1"),
							Id:           common.String("update_subnet_error"),
							DisplayName:  common.String("name"),
							FreeformTags: tags,
						},
					}, nil)

				vcnClient.EXPECT().UpdateSubnet(gomock.Any(), gomock.Eq(core.UpdateSubnetRequest{
					SubnetId: common.String("update_subnet_error"),
					UpdateSubnetDetails: core.UpdateSubnetDetails{
						DisplayName: common.String("update"),
						CidrBlock:   common.String("2.2.2.2/1"),
					},
				})).
					Return(core.UpdateSubnetResponse{}, errors.New("some error"))
			},
		},
		{
			name: "create subnet error",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						RouteTable: infrastructurev1beta2.RouteTable{
							PublicRouteTableId: common.String("public"),
						},
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								Name: "creation_needed",
								CIDR: "2.2.2.2/10",
							},
						},
					},
				},
				DefinedTags: definedTags,
			},
			wantErr:       true,
			expectedError: "failed create subnet: some error",
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				vcnClient.EXPECT().ListSubnets(gomock.Any(), gomock.Eq(core.ListSubnetsRequest{
					CompartmentId: common.String("foo"),
					DisplayName:   common.String("creation_needed"),
					VcnId:         common.String("vcn"),
				})).Return(
					core.ListSubnetsResponse{}, nil)

				vcnClient.EXPECT().CreateSubnet(gomock.Any(), gomock.Eq(core.CreateSubnetRequest{
					CreateSubnetDetails: core.CreateSubnetDetails{
						CidrBlock:               common.String("2.2.2.2/10"),
						CompartmentId:           common.String("foo"),
						VcnId:                   common.String("vcn"),
						DefinedTags:             definedTagsInterface,
						DisplayName:             common.String("creation_needed"),
						FreeformTags:            tags,
						ProhibitInternetIngress: common.Bool(false),
						ProhibitPublicIpOnVnic:  common.Bool(false),
						RouteTableId:            common.String("public"),
					},
				})).Return(
					core.CreateSubnetResponse{}, errors.New("some error"))
			},
		},
		{
			name: "create subnet error - nil subnets",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						RouteTable: infrastructurev1beta2.RouteTable{
							PublicRouteTableId: common.String("public"),
						},
						Subnets: []*infrastructurev1beta2.Subnet{
							nil,
						},
					},
				},
				DefinedTags: definedTags,
			},
			wantErr:       true,
			expectedError: "Skipping Subnet reconciliation: Subnet can't be nil",
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				// nothing should be called
			},
		},
		{
			name: "create security list error",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						RouteTable: infrastructurev1beta2.RouteTable{
							PublicRouteTableId: common.String("public"),
						},
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								ID:   common.String("sec_list_added_id"),
								Name: "sec_list_added",
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name:         "test-cp-endpoint-seclist",
									IngressRules: customIngress_updated,
									EgressRules:  customEgress,
								},
							},
						},
					},
				},
				DefinedTags: definedTags,
			},
			wantErr:       true,
			expectedError: "failed create security list: some error",
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
					SubnetId: common.String("sec_list_added_id"),
				})).
					Return(core.GetSubnetResponse{
						Subnet: core.Subnet{
							Id:           common.String("sec_list_added_id"),
							DisplayName:  common.String("sec_list_added"),
							FreeformTags: tags,
							DefinedTags:  definedTagsInterface,
							CidrBlock:    common.String(WorkerSubnetDefaultCIDR),
						},
					}, nil)

				vcnClient.EXPECT().ListSecurityLists(gomock.Any(), gomock.Eq(core.ListSecurityListsRequest{
					CompartmentId: common.String("foo"),
					VcnId:         common.String("vcn"),
					DisplayName:   common.String("test-cp-endpoint-seclist"),
				})).Return(core.ListSecurityListsResponse{}, nil)

				vcnClient.EXPECT().CreateSecurityList(gomock.Any(), gomock.Eq(core.CreateSecurityListRequest{
					CreateSecurityListDetails: core.CreateSecurityListDetails{
						CompartmentId: common.String("foo"),
						EgressSecurityRules: []core.EgressSecurityRule{
							{
								Description: common.String("test-egress"),
								Protocol:    common.String("1"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(345),
										Min: common.Int(567),
									},
								},
								DestinationType: core.EgressSecurityRuleDestinationTypeCidrBlock,
								Destination:     common.String("2.2.2.2/2"),
								IsStateless:     common.Bool(false),
							},
						},
						IngressSecurityRules: []core.IngressSecurityRule{
							{
								Description: common.String("test-ingress-updated"),
								Protocol:    common.String("7"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(789),
										Min: common.Int(343),
									},
								},
								SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String("1.1.1.2/1"),
								IsStateless: common.Bool(false),
							},
						},
						VcnId:        common.String("vcn"),
						DefinedTags:  definedTagsInterface,
						DisplayName:  common.String("test-cp-endpoint-seclist"),
						FreeformTags: tags,
					},
				})).Return(
					core.CreateSecurityListResponse{}, errors.New("some error"))
			},
		},
		{
			name: "update security list error",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						RouteTable: infrastructurev1beta2.RouteTable{
							PublicRouteTableId: common.String("public"),
						},
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								ID:   common.String("sec_list_updated_id"),
								Name: "sec_list_added",
								SecurityList: &infrastructurev1beta2.SecurityList{
									ID:           common.String("seclist_id"),
									Name:         "bar",
									IngressRules: customIngress_updated,
									EgressRules:  customEgress,
								},
							},
						},
					},
				},
				DefinedTags: definedTags,
			},
			wantErr:       true,
			expectedError: "failed to reconcile the security list, failed to update: some error",
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
					SubnetId: common.String("sec_list_updated_id"),
				})).
					Return(core.GetSubnetResponse{
						Subnet: core.Subnet{
							Id:              common.String("sec_list_updated_id"),
							DisplayName:     common.String("sec_list_added"),
							FreeformTags:    tags,
							DefinedTags:     definedTagsInterface,
							CidrBlock:       common.String(WorkerSubnetDefaultCIDR),
							SecurityListIds: []string{"seclist_id"},
						},
					}, nil)
				vcnClient.EXPECT().GetSecurityList(gomock.Any(), gomock.Eq(core.GetSecurityListRequest{
					SecurityListId: common.String("seclist_id"),
				})).
					Return(core.GetSecurityListResponse{
						SecurityList: core.SecurityList{
							DisplayName:  common.String("bar"),
							Id:           common.String("seclist_id"),
							FreeformTags: tags,
						},
					}, nil)
				vcnClient.EXPECT().UpdateSecurityList(gomock.Any(), gomock.Eq(core.UpdateSecurityListRequest{
					SecurityListId: common.String("seclist_id"),
					UpdateSecurityListDetails: core.UpdateSecurityListDetails{
						DisplayName: common.String("bar"),
						EgressSecurityRules: []core.EgressSecurityRule{
							{
								Description: common.String("test-egress"),
								Protocol:    common.String("1"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(345),
										Min: common.Int(567),
									},
								},
								DestinationType: core.EgressSecurityRuleDestinationTypeCidrBlock,
								Destination:     common.String("2.2.2.2/2"),
								IsStateless:     common.Bool(false),
							},
						},
						IngressSecurityRules: []core.IngressSecurityRule{
							{
								Description: common.String("test-ingress-updated"),
								Protocol:    common.String("7"),
								TcpOptions: &core.TcpOptions{
									DestinationPortRange: &core.PortRange{
										Max: common.Int(789),
										Min: common.Int(343),
									},
								},
								SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String("1.1.1.2/1"),
								IsStateless: common.Bool(false),
							},
						},
					},
				})).Return(core.UpdateSecurityListResponse{}, errors.New("some error"))
			},
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
				VCNClient:          vcnClient,
				OCIClusterAccessor: ociClusterAccessor,
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "resource_uid",
					},
				},
				Logger: &l,
			}
			tt.testSpecificSetup(s, vcnClient)
			err := s.ReconcileSubnet(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileSubnet() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("ReconcileSubnet() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestClusterScope_DeleteSubnets(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
		SubnetId: common.String("cp_endpoint_id"),
	})).
		Return(core.GetSubnetResponse{
			Subnet: core.Subnet{
				Id:           common.String("cp_endpoint_id"),
				FreeformTags: tags,
			},
		}, nil)

	vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
		SubnetId: common.String("cp_mc_id"),
	})).
		Return(core.GetSubnetResponse{
			Subnet: core.Subnet{
				Id:           common.String("cp_mc_id"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
		SubnetId: common.String("cp_endpoint_id_error_delete"),
	})).
		Return(core.GetSubnetResponse{
			Subnet: core.Subnet{
				Id:           common.String("cp_endpoint_id_error_delete"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{
		SubnetId: common.String("cp_endpoint_id_error")})).
		Return(core.GetSubnetResponse{}, errors.New("some error in GetSubnet"))

	vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{SubnetId: common.String("ep_subnet_deleted")})).
		Return(core.GetSubnetResponse{}, errors.New("not found"))
	vcnClient.EXPECT().GetSubnet(gomock.Any(), gomock.Eq(core.GetSubnetRequest{SubnetId: common.String("mc_subnet_deleted")})).
		Return(core.GetSubnetResponse{}, errors.New("not found"))
	vcnClient.EXPECT().DeleteSubnet(gomock.Any(), gomock.Eq(core.DeleteSubnetRequest{
		SubnetId: common.String("cp_endpoint_id"),
	})).
		Return(core.DeleteSubnetResponse{}, nil)
	vcnClient.EXPECT().DeleteSubnet(gomock.Any(), gomock.Eq(core.DeleteSubnetRequest{
		SubnetId: common.String("cp_mc_id"),
	})).
		Return(core.DeleteSubnetResponse{}, nil)
	vcnClient.EXPECT().DeleteSubnet(gomock.Any(), gomock.Eq(core.DeleteSubnetRequest{
		SubnetId: common.String("cp_endpoint_id_error_delete"),
	})).
		Return(core.DeleteSubnetResponse{}, errors.New("some error in subnet delete"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete subnet is successful",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								ID: common.String("cp_endpoint_id"),
							},
							{
								ID: common.String("cp_mc_id"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "subnet already deleted",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								ID: common.String("ep_subnet_deleted"),
							},
							{
								ID: common.String("mc_subnet_deleted"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete subnet error when calling get subnet",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								ID: common.String("cp_endpoint_id_error"),
							},
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "some error in GetSubnet",
		},
		{
			name: "delete subnet error when calling delete subnet",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								ID: common.String("cp_endpoint_id_error_delete"),
							},
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to delete subnet: some error in subnet delete",
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
			err := s.DeleteSubnets(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteSubnets() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("DeleteSubnets() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}
}

func isSubnetsEqual(desiredSubnets []*infrastructurev1beta2.Subnet, actualSubnets []*infrastructurev1beta2.Subnet) bool {
	var found bool
	if len(desiredSubnets) != len(actualSubnets) {
		return false
	}
	for _, desired := range desiredSubnets {
		found = false
		for _, actual := range actualSubnets {
			if reflect.DeepEqual(*actual, *desired) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestClusterScope_IsSubnetsEqual(t *testing.T) {
	s := &ClusterScope{}
	secID := common.String("secid")

	tests := []struct {
		name    string
		actual  core.Subnet
		desired infrastructurev1beta2.Subnet
		want    bool
	}{
		{
			name: "equal without security list",
			actual: core.Subnet{
				DisplayName: common.String("name"),
				CidrBlock:   common.String("10.0.0.0/24"),
			},
			desired: infrastructurev1beta2.Subnet{
				Name: "name",
				CIDR: "10.0.0.0/24",
			},
			want: true,
		},
		{
			name: "actual nil DisplayName",
			actual: core.Subnet{
				DisplayName: nil,
				CidrBlock:   common.String("10.0.0.0/24"),
			},
			desired: infrastructurev1beta2.Subnet{
				Name: "",
				CIDR: "10.0.0.0/24",
			},
			want: true,
		},
		{
			name: "actual nil CidrBlock",
			actual: core.Subnet{
				DisplayName: common.String("name"),
				CidrBlock:   nil,
			},
			desired: infrastructurev1beta2.Subnet{
				Name: "name",
				CIDR: "",
			},
			want: true,
		},
		{
			name: "name mismatch",
			actual: core.Subnet{
				DisplayName: common.String("other"),
				CidrBlock:   common.String("10.0.0.0/24"),
			},
			desired: infrastructurev1beta2.Subnet{
				Name: "name",
				CIDR: "10.0.0.0/24",
			},
			want: false,
		},
		{
			name: "cidr mismatch",
			actual: core.Subnet{
				DisplayName: common.String("name"),
				CidrBlock:   common.String("10.0.1.0/24"),
			},
			desired: infrastructurev1beta2.Subnet{
				Name: "name",
				CIDR: "10.0.0.0/24",
			},
			want: false,
		},
		{
			name: "equal with matching security list first id",
			actual: core.Subnet{
				DisplayName:     common.String("name"),
				CidrBlock:       common.String("10.0.0.0/24"),
				SecurityListIds: []string{"secid", "other"},
			},
			desired: infrastructurev1beta2.Subnet{
				Name: "name",
				CIDR: "10.0.0.0/24",
				SecurityList: &infrastructurev1beta2.SecurityList{
					ID: secID,
				},
			},
			want: true,
		},
		{
			name: "desired security list id nil",
			actual: core.Subnet{
				DisplayName:     common.String("name"),
				CidrBlock:       common.String("10.0.0.0/24"),
				SecurityListIds: []string{"secid"},
			},
			desired: infrastructurev1beta2.Subnet{
				Name:         "name",
				CIDR:         "10.0.0.0/24",
				SecurityList: &infrastructurev1beta2.SecurityList{
					// ID intentionally nil
				},
			},
			want: false,
		},
		{
			name: "desired security list present but actual has none",
			actual: core.Subnet{
				DisplayName: common.String("name"),
				CidrBlock:   common.String("10.0.0.0/24"),
			},
			desired: infrastructurev1beta2.Subnet{
				Name: "name",
				CIDR: "10.0.0.0/24",
				SecurityList: &infrastructurev1beta2.SecurityList{
					ID: secID,
				},
			},
			want: false,
		},
		{
			name: "security list id mismatch with first element",
			// we should find the second item in the list
			actual: core.Subnet{
				DisplayName:     common.String("name"),
				CidrBlock:       common.String("10.0.0.0/24"),
				SecurityListIds: []string{"different", "secid"},
			},
			desired: infrastructurev1beta2.Subnet{
				Name: "name",
				CIDR: "10.0.0.0/24",
				SecurityList: &infrastructurev1beta2.SecurityList{
					ID: secID,
				},
			},
			want: true,
		},
		{
			name: "desired security list nil but actual has ids",
			actual: core.Subnet{
				DisplayName:     common.String("name"),
				CidrBlock:       common.String("10.0.0.0/24"),
				SecurityListIds: []string{"any"},
			},
			desired: infrastructurev1beta2.Subnet{
				Name: "name",
				CIDR: "10.0.0.0/24",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.IsSubnetsEqual(&tt.actual, tt.desired)
			if got != tt.want {
				t.Errorf("IsSubnetsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
