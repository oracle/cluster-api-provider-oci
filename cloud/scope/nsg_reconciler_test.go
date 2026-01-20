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

func TestClusterScope_DeleteNSGs(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	vcnClient.EXPECT().GetNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.GetNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("nsg1"),
	})).
		Return(core.GetNetworkSecurityGroupResponse{
			NetworkSecurityGroup: core.NetworkSecurityGroup{
				Id:           common.String("nsg1"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.GetNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("nsg2"),
	})).
		Return(core.GetNetworkSecurityGroupResponse{
			NetworkSecurityGroup: core.NetworkSecurityGroup{
				Id:           common.String("nsg2"),
				FreeformTags: tags,
			},
		}, nil)

	vcnClient.EXPECT().GetNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.GetNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("nsg_error_delete"),
	})).
		Return(core.GetNetworkSecurityGroupResponse{
			NetworkSecurityGroup: core.NetworkSecurityGroup{
				Id:           common.String("nsg_error_delete"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.GetNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("nsg_get_error")})).
		Return(core.GetNetworkSecurityGroupResponse{}, errors.New("some error"))

	vcnClient.EXPECT().GetNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.GetNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("nsg_deleted")})).
		Return(core.GetNetworkSecurityGroupResponse{}, errors.New("not found"))
	vcnClient.EXPECT().DeleteNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.DeleteNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("nsg1"),
	})).
		Return(core.DeleteNetworkSecurityGroupResponse{}, nil)
	vcnClient.EXPECT().DeleteNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.DeleteNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("nsg2"),
	})).
		Return(core.DeleteNetworkSecurityGroupResponse{}, nil)
	vcnClient.EXPECT().DeleteNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.DeleteNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("nsg_error_delete"),
	})).
		Return(core.DeleteNetworkSecurityGroupResponse{}, errors.New("some error"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete nsg is successful",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									ID: common.String("nsg1"),
								},
								{
									ID: common.String("nsg2"),
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nsg already deleted",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									ID: common.String("nsg_deleted"),
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete nsg error when calling get route table",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									ID: common.String("nsg_get_error"),
								},
							},
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "some error",
		},
		{
			name: "delete nsg error when calling delete route table",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									ID: common.String("nsg_error_delete"),
								},
							},
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to delete nsg: some error",
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
			err := s.DeleteNSGs(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteNSGs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("DeleteNSGs() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}
}

func TestClusterScope_ReconcileNSG(t *testing.T) {
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

	updatedTags := make(map[string]string)
	for k, v := range tags {
		updatedTags[k] = v
	}
	updatedTags["foo"] = "bar"

	customNSGIngress := []infrastructurev1beta2.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("External access to Kubernetes API endpoint"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("External access to Kubernetes API endpoint - 1"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeNSG,
				Source:     common.String("no-update"),
			},
		},
	}
	customNSGEgress := []infrastructurev1beta2.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("All traffic to control plane nodes"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("All traffic to control plane nodes - 1"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeNSG,
				Destination:     common.String("update-rules"),
			},
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

	customNSGEgressWithId := make([]infrastructurev1beta2.EgressSecurityRuleForNSG, len(customNSGEgress))
	copy(customNSGEgressWithId, customNSGEgress)

	customNSGIngressWithId := make([]infrastructurev1beta2.IngressSecurityRuleForNSG, len(customNSGIngress))
	copy(customNSGIngressWithId, customNSGIngress)

	tests := []struct {
		name              string
		spec              infrastructurev1beta2.OCIClusterSpec
		wantErr           bool
		expectedError     string
		testSpecificSetup func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient)
	}{
		{
			name: "one creation - one update - one update security rule - one no update",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									ID:           common.String("no-update-id"),
									Name:         "no-update",
									Role:         "control-plane-endpoint",
									EgressRules:  customNSGEgressWithId,
									IngressRules: customNSGIngressWithId,
								},
								{
									ID:           common.String("update-rules-id"),
									Name:         "update-rules",
									Role:         "control-plane",
									EgressRules:  customNSGEgress,
									IngressRules: customNSGIngressWithId,
								},
								{
									ID:           common.String("update-id"),
									Name:         "update-nsg",
									Role:         "worker",
									EgressRules:  customNSGEgressWithId,
									IngressRules: customNSGIngressWithId,
								},
							},
						},
					},
				},
				DefinedTags:   definedTags,
				CompartmentId: "foo",
			},
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				vcnClient.EXPECT().GetNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.GetNetworkSecurityGroupRequest{
					NetworkSecurityGroupId: common.String("no-update-id"),
				})).
					Return(core.GetNetworkSecurityGroupResponse{
						NetworkSecurityGroup: core.NetworkSecurityGroup{
							Id:           common.String("no-update-id"),
							DisplayName:  common.String("no-update"),
							FreeformTags: tags,
							DefinedTags:  definedTagsInterface,
						},
					}, nil)

				vcnClient.EXPECT().GetNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.GetNetworkSecurityGroupRequest{
					NetworkSecurityGroupId: common.String("update-id"),
				})).
					Return(core.GetNetworkSecurityGroupResponse{
						NetworkSecurityGroup: core.NetworkSecurityGroup{
							Id:           common.String("update-id"),
							DisplayName:  common.String("change-me"),
							FreeformTags: tags,
							DefinedTags:  definedTagsInterface,
						},
					}, nil).Times(2)

				vcnClient.EXPECT().GetNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.GetNetworkSecurityGroupRequest{
					NetworkSecurityGroupId: common.String("update-rules-id"),
				})).
					Return(core.GetNetworkSecurityGroupResponse{
						NetworkSecurityGroup: core.NetworkSecurityGroup{
							Id:           common.String("update-rules-id"),
							DisplayName:  common.String("update-rules"),
							FreeformTags: tags,
							DefinedTags:  definedTagsInterface,
						},
					}, nil)

				vcnClient.EXPECT().ListNetworkSecurityGroupSecurityRules(gomock.Any(), gomock.Eq(core.ListNetworkSecurityGroupSecurityRulesRequest{
					NetworkSecurityGroupId: common.String("no-update-id"),
				})).Return(core.ListNetworkSecurityGroupSecurityRulesResponse{
					Items: []core.SecurityRule{
						{
							Direction:       core.SecurityRuleDirectionEgress,
							Protocol:        common.String("6"),
							Description:     common.String("All traffic to control plane nodes"),
							Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							DestinationType: core.SecurityRuleDestinationTypeCidrBlock,
							Id:              common.String("egress-id"),
							IsStateless:     common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:       core.SecurityRuleDirectionEgress,
							Protocol:        common.String("6"),
							Description:     common.String("All traffic to control plane nodes - 1"),
							Destination:     common.String("update-rules-id"),
							DestinationType: core.SecurityRuleDestinationTypeNetworkSecurityGroup,
							Id:              common.String("egress-id-1"),
							IsStateless:     common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:   core.SecurityRuleDirectionIngress,
							Protocol:    common.String("6"),
							Description: common.String("External access to Kubernetes API endpoint"),
							Id:          common.String("ingress-id"),
							Source:      common.String("0.0.0.0/0"),
							SourceType:  core.SecurityRuleSourceTypeCidrBlock,
							IsStateless: common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:   core.SecurityRuleDirectionIngress,
							Protocol:    common.String("6"),
							Description: common.String("External access to Kubernetes API endpoint - 1"),
							Id:          common.String("ingress-id-1"),
							Source:      common.String("no-update-id"),
							SourceType:  core.SecurityRuleSourceTypeNetworkSecurityGroup,
							IsStateless: common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
					},
				}, nil)

				vcnClient.EXPECT().ListNetworkSecurityGroupSecurityRules(gomock.Any(), gomock.Eq(core.ListNetworkSecurityGroupSecurityRulesRequest{
					NetworkSecurityGroupId: common.String("update-id"),
				})).Return(core.ListNetworkSecurityGroupSecurityRulesResponse{
					Items: []core.SecurityRule{
						{
							Direction:       core.SecurityRuleDirectionEgress,
							Protocol:        common.String("6"),
							Description:     common.String("All traffic to control plane nodes"),
							Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							DestinationType: core.SecurityRuleDestinationTypeCidrBlock,
							Id:              common.String("egress-id"),
							IsStateless:     common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:       core.SecurityRuleDirectionEgress,
							Protocol:        common.String("6"),
							Description:     common.String("All traffic to control plane nodes - 1"),
							Destination:     common.String("update-rules-id"),
							DestinationType: core.SecurityRuleDestinationTypeNetworkSecurityGroup,
							Id:              common.String("egress-id-1"),
							IsStateless:     common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:   core.SecurityRuleDirectionIngress,
							Protocol:    common.String("6"),
							Description: common.String("External access to Kubernetes API endpoint"),
							Id:          common.String("ingress-id"),
							Source:      common.String("0.0.0.0/0"),
							SourceType:  core.SecurityRuleSourceTypeCidrBlock,
							IsStateless: common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:   core.SecurityRuleDirectionIngress,
							Protocol:    common.String("6"),
							Description: common.String("External access to Kubernetes API endpoint - 1"),
							Id:          common.String("ingress-id-1"),
							Source:      common.String("no-update-id"),
							SourceType:  core.SecurityRuleSourceTypeNetworkSecurityGroup,
							IsStateless: common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
					},
				}, nil)

				vcnClient.EXPECT().ListNetworkSecurityGroupSecurityRules(gomock.Any(), gomock.Eq(core.ListNetworkSecurityGroupSecurityRulesRequest{
					NetworkSecurityGroupId: common.String("update-rules-id"),
				})).Return(core.ListNetworkSecurityGroupSecurityRulesResponse{
					Items: []core.SecurityRule{
						{
							Direction:       core.SecurityRuleDirectionEgress,
							Protocol:        common.String("10"),
							Description:     common.String("All traffic to control plane nodes"),
							Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							DestinationType: core.SecurityRuleDestinationTypeCidrBlock,
							Id:              common.String("egress-id-changed"),
							IsStateless:     common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:   core.SecurityRuleDirectionIngress,
							Protocol:    common.String("7"),
							Description: common.String("External access to Kubernetes API endpoint"),
							Id:          common.String("ingress-id"),
							Source:      common.String("0.0.0.0/1"),
							SourceType:  core.SecurityRuleSourceTypeCidrBlock,
							IsStateless: common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
					},
				}, nil)

				vcnClient.EXPECT().AddNetworkSecurityGroupSecurityRules(gomock.Any(), gomock.Eq(core.AddNetworkSecurityGroupSecurityRulesRequest{
					NetworkSecurityGroupId: common.String("update-rules-id"),
					AddNetworkSecurityGroupSecurityRulesDetails: core.AddNetworkSecurityGroupSecurityRulesDetails{SecurityRules: []core.AddSecurityRuleDetails{
						{
							Direction:   core.AddSecurityRuleDetailsDirectionIngress,
							Protocol:    common.String("6"),
							Description: common.String("External access to Kubernetes API endpoint"),
							Source:      common.String("0.0.0.0/0"),
							SourceType:  core.AddSecurityRuleDetailsSourceTypeCidrBlock,
							IsStateless: common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:   core.AddSecurityRuleDetailsDirectionIngress,
							Protocol:    common.String("6"),
							Description: common.String("External access to Kubernetes API endpoint - 1"),
							Source:      common.String("no-update-id"),
							SourceType:  core.AddSecurityRuleDetailsSourceTypeNetworkSecurityGroup,
							IsStateless: common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:       core.AddSecurityRuleDetailsDirectionEgress,
							Protocol:        common.String("6"),
							Description:     common.String("All traffic to control plane nodes"),
							Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							DestinationType: core.AddSecurityRuleDetailsDestinationTypeCidrBlock,
							IsStateless:     common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
						{
							Direction:       core.AddSecurityRuleDetailsDirectionEgress,
							Protocol:        common.String("6"),
							Description:     common.String("All traffic to control plane nodes - 1"),
							Destination:     common.String("update-rules-id"),
							DestinationType: core.AddSecurityRuleDetailsDestinationTypeNetworkSecurityGroup,
							IsStateless:     common.Bool(false),
							TcpOptions: &core.TcpOptions{
								DestinationPortRange: &core.PortRange{
									Max: common.Int(6443),
									Min: common.Int(6443),
								},
							},
						},
					},
					},
				})).Return(core.AddNetworkSecurityGroupSecurityRulesResponse{
					AddedNetworkSecurityGroupSecurityRules: core.AddedNetworkSecurityGroupSecurityRules{
						SecurityRules: []core.SecurityRule{
							{

								Id: common.String("foo"),
							},
						},
					},
				}, nil)

				vcnClient.EXPECT().RemoveNetworkSecurityGroupSecurityRules(gomock.Any(), gomock.Eq(core.RemoveNetworkSecurityGroupSecurityRulesRequest{
					NetworkSecurityGroupId: common.String("update-rules-id"),
					RemoveNetworkSecurityGroupSecurityRulesDetails: core.RemoveNetworkSecurityGroupSecurityRulesDetails{
						SecurityRuleIds: []string{"ingress-id", "egress-id-changed"},
					},
				})).Return(core.RemoveNetworkSecurityGroupSecurityRulesResponse{}, nil)

				vcnClient.EXPECT().UpdateNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.UpdateNetworkSecurityGroupRequest{
					NetworkSecurityGroupId: common.String("update-id"),
					UpdateNetworkSecurityGroupDetails: core.UpdateNetworkSecurityGroupDetails{
						DisplayName: common.String("update-nsg"),
					},
				})).
					Return(core.UpdateNetworkSecurityGroupResponse{
						NetworkSecurityGroup: core.NetworkSecurityGroup{
							Id:           common.String("update-id"),
							FreeformTags: tags,
						},
					}, nil)
			},
		},
		{
			name: "update failed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta2.ControlPlaneRole,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name: "foo",
								},
							},
						},
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									ID:           common.String("update-id"),
									Name:         "update-nsg-error",
									Role:         "worker",
									EgressRules:  customNSGEgressWithId,
									IngressRules: customNSGIngressWithId,
								},
							},
						},
					},
				},
			},
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				vcnClient.EXPECT().UpdateNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.UpdateNetworkSecurityGroupRequest{
					NetworkSecurityGroupId: common.String("update-id"),
					UpdateNetworkSecurityGroupDetails: core.UpdateNetworkSecurityGroupDetails{
						DisplayName: common.String("update-nsg-error"),
					},
				})).
					Return(core.UpdateNetworkSecurityGroupResponse{}, errors.New("some error"))
			},
			wantErr:       true,
			expectedError: "failed to reconcile the network security group, failed to update: some error",
		},
		{
			name: "creation failed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				FreeformTags: map[string]string{
					"foo": "bar",
				},
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta2.WorkerRole,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta2.ControlPlaneRole,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name: "foo",
								},
							},
						},
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									Name: "service-lb",
									Role: "service-lb",
								},
							},
						},
					},
				},
			},
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				vcnClient.EXPECT().ListNetworkSecurityGroups(gomock.Any(), gomock.Eq(core.ListNetworkSecurityGroupsRequest{
					CompartmentId: common.String("foo"),
					DisplayName:   common.String("service-lb"),
					VcnId:         common.String("vcn"),
				})).Return(
					core.ListNetworkSecurityGroupsResponse{
						Items: []core.NetworkSecurityGroup{}}, nil)
				vcnClient.EXPECT().CreateNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.CreateNetworkSecurityGroupRequest{
					CreateNetworkSecurityGroupDetails: core.CreateNetworkSecurityGroupDetails{
						CompartmentId: common.String("foo"),
						DisplayName:   common.String("service-lb"),
						VcnId:         common.String("vcn"),
						FreeformTags:  updatedTags,
						DefinedTags:   definedTagsInterface,
					},
				})).Return(
					core.CreateNetworkSecurityGroupResponse{}, errors.New("some error"))
			},
			wantErr:       true,
			expectedError: "failed create nsg: some error",
		},
		{
			name: "add rules failed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta2.WorkerRole,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta2.ControlPlaneRole,
								SecurityList: &infrastructurev1beta2.SecurityList{
									Name: "foo",
								},
							},
						},
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									ID:          common.String("loadbalancer-nsg-id"),
									Name:        "service-lb",
									Role:        "service-lb",
									EgressRules: customNSGEgress,
								},
							},
						},
					},
				},
			},
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				vcnClient.EXPECT().GetNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.GetNetworkSecurityGroupRequest{
					NetworkSecurityGroupId: common.String("loadbalancer-nsg-id"),
				})).
					Return(core.GetNetworkSecurityGroupResponse{
						NetworkSecurityGroup: core.NetworkSecurityGroup{
							Id:           common.String("loadbalancer-nsg-id"),
							DisplayName:  common.String("service-lb"),
							FreeformTags: tags,
							DefinedTags:  definedTagsInterface,
						},
					}, nil)
				vcnClient.EXPECT().ListNetworkSecurityGroupSecurityRules(gomock.Any(), gomock.Eq(core.ListNetworkSecurityGroupSecurityRulesRequest{
					NetworkSecurityGroupId: common.String("loadbalancer-nsg-id"),
				})).Return(core.ListNetworkSecurityGroupSecurityRulesResponse{
					Items: []core.SecurityRule{},
				}, nil)
				vcnClient.EXPECT().AddNetworkSecurityGroupSecurityRules(gomock.Any(), gomock.Any()).Return(core.AddNetworkSecurityGroupSecurityRulesResponse{}, errors.New("some error"))
			},
			wantErr:       true,
			expectedError: "failed add nsg security rules: some error",
		},
		{
			name: "nil NSG in list",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{nil},
						},
					},
				},
			},
			wantErr: false,
			testSpecificSetup: func(clusterScope *ClusterScope, nlbClient *mock_vcn.MockClient) {
				// nothing should be called
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
			err := s.ReconcileNSG(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileNSG() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("ReconcileNSG() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func isNSGEqual(desiredNSGs []*infrastructurev1beta2.NSG, actualNSGs []*infrastructurev1beta2.NSG) bool {
	var found bool
	if len(desiredNSGs) != len(actualNSGs) {
		return false
	}
	for _, desired := range desiredNSGs {
		found = false
		for _, actual := range actualNSGs {
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
