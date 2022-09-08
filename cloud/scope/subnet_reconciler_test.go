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
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestClusterScope_SubnetSpec(t *testing.T) {
	customIngress := []infrastructurev1beta1.IngressSecurityRule{
		{
			Description: common.String("test-ingress"),
			Protocol:    common.String("8"),
			TcpOptions: &infrastructurev1beta1.TcpOptions{
				DestinationPortRange: &infrastructurev1beta1.PortRange{
					Max: common.Int(123),
					Min: common.Int(234),
				},
			},
			SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
			Source:     common.String("1.1.1.1/1"),
		},
	}
	customEgress := []infrastructurev1beta1.EgressSecurityRule{
		{
			Description: common.String("test-egress"),
			Protocol:    common.String("1"),
			TcpOptions: &infrastructurev1beta1.TcpOptions{
				DestinationPortRange: &infrastructurev1beta1.PortRange{
					Max: common.Int(345),
					Min: common.Int(567),
				},
			},
			DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
			Destination:     common.String("2.2.2.2/2"),
		},
	}
	tests := []struct {
		name          string
		spec          infrastructurev1beta1.OCIClusterSpec
		want          []*infrastructurev1beta1.Subnet
		wantErr       bool
		expectedError string
	}{
		{
			name:    "all default",
			wantErr: false,
			want: []*infrastructurev1beta1.Subnet{
				{
					Role: infrastructurev1beta1.ControlPlaneEndpointRole,
					Name: "control-plane-endpoint",
					CIDR: ControlPlaneEndpointSubnetDefaultCIDR,
					Type: infrastructurev1beta1.Public,
				},
				{
					Role:         infrastructurev1beta1.ControlPlaneRole,
					Name:         "control-plane",
					CIDR:         ControlPlaneMachineSubnetDefaultCIDR,
					Type:         infrastructurev1beta1.Private,
					SecurityList: nil,
				},
				{
					Role:         infrastructurev1beta1.WorkerRole,
					Name:         "worker",
					CIDR:         WorkerSubnetDefaultCIDR,
					Type:         infrastructurev1beta1.Private,
					SecurityList: nil,
				},
				{
					Role:         infrastructurev1beta1.ServiceLoadBalancerRole,
					Name:         "service-lb",
					CIDR:         ServiceLoadBalancerDefaultCIDR,
					Type:         infrastructurev1beta1.Public,
					SecurityList: nil,
				},
			},
		},
		{
			name:    "all user provided",
			wantErr: false,
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
								Name: "test-cp-endpoint",
								CIDR: "2.2.2.2/10",
								Type: infrastructurev1beta1.Private,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "test-cp-endpoint-seclist",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
							},
							{
								Role: infrastructurev1beta1.ControlPlaneRole,
								Name: "test-mc",
								CIDR: "1.1.1.1/1",
								Type: infrastructurev1beta1.Public,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "test-mc-seclist",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
							},
							{
								Role: infrastructurev1beta1.WorkerRole,
								Name: "node-test",
								CIDR: "2.2.2.2/1",
								Type: infrastructurev1beta1.Public,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "test-node-sec",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
							},
							{
								Role: infrastructurev1beta1.ServiceLoadBalancerRole,
								Name: "loadbalancer-test",
								CIDR: "5.5.5.5/5",
								Type: infrastructurev1beta1.Private,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "test-lb-sec",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
							},
							{
								Role: infrastructurev1beta1.WorkerRole,
								Name: "node-test-1",
								CIDR: "4.2.2.4/4",
								Type: infrastructurev1beta1.Public,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "test-node-sec-1",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
							},
						},
					},
				},
			},
			want: []*infrastructurev1beta1.Subnet{
				{
					Role: infrastructurev1beta1.ControlPlaneEndpointRole,
					Name: "test-cp-endpoint",
					CIDR: "2.2.2.2/10",
					Type: infrastructurev1beta1.Private,
					SecurityList: &infrastructurev1beta1.SecurityList{
						Name:         "test-cp-endpoint-seclist",
						EgressRules:  customEgress,
						IngressRules: customIngress,
					},
				},
				{
					Role: infrastructurev1beta1.ControlPlaneRole,
					Name: "test-mc",
					CIDR: "1.1.1.1/1",
					Type: infrastructurev1beta1.Public,
					SecurityList: &infrastructurev1beta1.SecurityList{
						Name:         "test-mc-seclist",
						EgressRules:  customEgress,
						IngressRules: customIngress,
					},
				},
				{
					Role: infrastructurev1beta1.WorkerRole,
					Name: "node-test",
					CIDR: "2.2.2.2/1",
					Type: infrastructurev1beta1.Public,
					SecurityList: &infrastructurev1beta1.SecurityList{
						Name:         "test-node-sec",
						EgressRules:  customEgress,
						IngressRules: customIngress,
					},
				},
				{
					Role: infrastructurev1beta1.WorkerRole,
					Name: "node-test-1",
					CIDR: "4.2.2.4/4",
					Type: infrastructurev1beta1.Public,
					SecurityList: &infrastructurev1beta1.SecurityList{
						Name:         "test-node-sec-1",
						EgressRules:  customEgress,
						IngressRules: customIngress,
					},
				},
				{
					Role: infrastructurev1beta1.ServiceLoadBalancerRole,
					Name: "loadbalancer-test",
					CIDR: "5.5.5.5/5",
					Type: infrastructurev1beta1.Private,
					SecurityList: &infrastructurev1beta1.SecurityList{
						Name:         "test-lb-sec",
						EgressRules:  customEgress,
						IngressRules: customIngress,
					},
				},
			},
		},
		{
			name:    "some user provided, some default",
			wantErr: false,
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Name: "cp-mc",
								Role: infrastructurev1beta1.ControlPlaneRole,
								CIDR: "1.1.1.1/1",
								Type: infrastructurev1beta1.Public,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "cp-mc-sec",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
							},
							{
								Role: infrastructurev1beta1.WorkerRole,
								Name: "node-test",
								Type: infrastructurev1beta1.Public,
							},
							{
								Role: infrastructurev1beta1.ServiceLoadBalancerRole,
								Name: "loadbalancer-test",
								CIDR: "5.5.5.5/5",
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "lb-seclist",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
							},
						},
					},
				},
			},
			want: []*infrastructurev1beta1.Subnet{
				{
					Role: infrastructurev1beta1.ControlPlaneEndpointRole,
					Name: "control-plane-endpoint",
					CIDR: ControlPlaneEndpointSubnetDefaultCIDR,
					Type: infrastructurev1beta1.Public,
				},
				{
					Role: infrastructurev1beta1.ControlPlaneRole,
					Name: "cp-mc",
					CIDR: "1.1.1.1/1",
					Type: infrastructurev1beta1.Public,
					SecurityList: &infrastructurev1beta1.SecurityList{
						Name:         "cp-mc-sec",
						EgressRules:  customEgress,
						IngressRules: customIngress,
					},
				},
				{
					Role: infrastructurev1beta1.WorkerRole,
					Name: "node-test",
					CIDR: WorkerSubnetDefaultCIDR,
					Type: infrastructurev1beta1.Public,
				},
				{
					Role: infrastructurev1beta1.ServiceLoadBalancerRole,
					Name: "loadbalancer-test",
					CIDR: "5.5.5.5/5",
					Type: infrastructurev1beta1.Public,
					SecurityList: &infrastructurev1beta1.SecurityList{
						Name:         "lb-seclist",
						EgressRules:  customEgress,
						IngressRules: customIngress,
					},
				},
			},
		},
		{
			name:    "default names",
			wantErr: false,
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneRole,
							},
							{
								Role: infrastructurev1beta1.WorkerRole,
							},
							{
								Role: infrastructurev1beta1.ServiceLoadBalancerRole,
							},
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
							},
						},
					},
				},
			},
			want: []*infrastructurev1beta1.Subnet{
				{
					Role: infrastructurev1beta1.ControlPlaneEndpointRole,
					Name: ControlPlaneEndpointDefaultName,
					CIDR: ControlPlaneEndpointSubnetDefaultCIDR,
					Type: infrastructurev1beta1.Public,
				},
				{
					Role: infrastructurev1beta1.ControlPlaneRole,
					Name: ControlPlaneDefaultName,
					CIDR: ControlPlaneMachineSubnetDefaultCIDR,
					Type: infrastructurev1beta1.Private,
				},
				{
					Role: infrastructurev1beta1.WorkerRole,
					Name: WorkerDefaultName,
					CIDR: WorkerSubnetDefaultCIDR,
					Type: infrastructurev1beta1.Private,
				},
				{
					Role: infrastructurev1beta1.ServiceLoadBalancerRole,
					Name: ServiceLBDefaultName,
					CIDR: ServiceLoadBalancerDefaultCIDR,
					Type: infrastructurev1beta1.Public,
				},
			},
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociCluster := infrastructurev1beta1.OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					UID: "cluster_uid",
				},
				Spec: tt.spec,
			}
			ociCluster.Spec.OCIResourceIdentifier = "resource_uid"
			s := &ClusterScope{
				OCICluster: &ociCluster,
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "resource_uid",
					},
				},
				Logger: &l,
			}
			subnets, err := s.SubnetSpec()
			if !isSubnetsEqual(tt.want, subnets) {
				t.Errorf("SubnetSpec() want = %v, got %v", tt.want, subnets)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("SubnetSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("SubnetSpec() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}
			}
		})
	}
}

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

	customIngress := []infrastructurev1beta1.IngressSecurityRule{
		{
			Description: common.String("test-ingress"),
			Protocol:    common.String("8"),
			TcpOptions: &infrastructurev1beta1.TcpOptions{
				DestinationPortRange: &infrastructurev1beta1.PortRange{
					Max: common.Int(123),
					Min: common.Int(234),
				},
			},
			SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
			Source:     common.String("1.1.1.1/1"),
		},
	}
	customEgress := []infrastructurev1beta1.EgressSecurityRule{
		{
			Description: common.String("test-egress"),
			Protocol:    common.String("1"),
			TcpOptions: &infrastructurev1beta1.TcpOptions{
				DestinationPortRange: &infrastructurev1beta1.PortRange{
					Max: common.Int(345),
					Min: common.Int(567),
				},
			},
			DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
			Destination:     common.String("2.2.2.2/2"),
		},
	}

	customIngress_updated := []infrastructurev1beta1.IngressSecurityRule{
		{
			Description: common.String("test-ingress-updated"),
			Protocol:    common.String("7"),
			TcpOptions: &infrastructurev1beta1.TcpOptions{
				DestinationPortRange: &infrastructurev1beta1.PortRange{
					Max: common.Int(789),
					Min: common.Int(343),
				},
			},
			SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
			Source:     common.String("1.1.1.2/1"),
		},
	}

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
		}, nil).Times(2)

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
		}, nil).Times(2)

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
	vcnClient.EXPECT().UpdateSubnet(gomock.Any(), gomock.Eq(core.UpdateSubnetRequest{
		SubnetId: common.String("update_subnet_error"),
		UpdateSubnetDetails: core.UpdateSubnetDetails{
			DisplayName: common.String("update"),
			CidrBlock:   common.String("2.2.2.2/1"),
		},
	})).
		Return(core.UpdateSubnetResponse{}, errors.New("some error"))

	vcnClient.EXPECT().ListSubnets(gomock.Any(), gomock.Eq(core.ListSubnetsRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("creation_needed"),
		VcnId:         common.String("vcn"),
	})).Return(
		core.ListSubnetsResponse{}, nil).Times(2)

	vcnClient.EXPECT().ListSubnets(gomock.Any(), gomock.Eq(core.ListSubnetsRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("control-plane"),
		VcnId:         common.String("vcn"),
	})).Return(
		core.ListSubnetsResponse{
			Items: []core.Subnet{
				{
					Id:           common.String("no_update_needed_id"),
					CidrBlock:    common.String(ControlPlaneMachineSubnetDefaultCIDR),
					DisplayName:  common.String("control-plane"),
					FreeformTags: tags,
					DefinedTags:  definedTagsInterface,
				},
			}}, nil)
	vcnClient.EXPECT().ListSecurityLists(gomock.Any(), gomock.Eq(core.ListSecurityListsRequest{
		CompartmentId: common.String("foo"),
		VcnId:         common.String("vcn"),
		DisplayName:   common.String("test-cp-endpoint-seclist"),
	})).Return(core.ListSecurityListsResponse{}, nil).Times(2)

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
		},
	})).Return(
		core.CreateSubnetResponse{
			Subnet: core.Subnet{
				Id: common.String("subnet"),
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
		}, nil).Times(2)

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

	vcnClient.EXPECT().GetSecurityList(gomock.Any(), gomock.Eq(core.GetSecurityListRequest{
		SecurityListId: common.String("seclist_id"),
	})).
		Return(core.GetSecurityListResponse{
			SecurityList: core.SecurityList{
				DisplayName:  common.String("bar"),
				Id:           common.String("seclist_id"),
				FreeformTags: tags,
			},
		}, nil).Times(2)

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

	tests := []struct {
		name          string
		spec          infrastructurev1beta1.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "subnet reconciliation successful - one creation - one update - one no update - one security list " +
				"creation - one security list update",
			spec: infrastructurev1beta1.OCIClusterSpec{
				DefinedTags:   definedTags,
				CompartmentId: "foo",
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID: common.String("vcn"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
								Name: "creation_needed",
								CIDR: "2.2.2.2/10",
								Type: infrastructurev1beta1.Private,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "test-cp-endpoint-seclist",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
							},
							{
								Role: infrastructurev1beta1.ServiceLoadBalancerRole,
								ID:   common.String("update_needed_id"),
								Name: "update_needed",
								Type: infrastructurev1beta1.Private,
							},
							{
								Role: infrastructurev1beta1.WorkerRole,
								ID:   common.String("sec_list_added_id"),
								Name: "sec_list_added",
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "test-cp-endpoint-seclist",
									IngressRules: customIngress,
									EgressRules:  customEgress,
								},
							},
							{
								Role: infrastructurev1beta1.WorkerRole,
								ID:   common.String("sec_list_updated_id"),
								Name: "sec_list_added",
								SecurityList: &infrastructurev1beta1.SecurityList{
									ID:           common.String("seclist_id"),
									Name:         "update_seclist",
									IngressRules: customIngress,
									EgressRules:  customEgress,
								},
							},
						},
						PrivateRouteTableId: common.String("private"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "update subnet error",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
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
		},
		{
			name: "create subnet error",
			spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId: "foo",
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID:                 common.String("vcn"),
						PublicRouteTableId: common.String("public"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
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
		},
		{
			name: "create security list error",
			spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId: "foo",
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID:                 common.String("vcn"),
						PublicRouteTableId: common.String("public"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
								ID:   common.String("sec_list_added_id"),
								Name: "sec_list_added",
								SecurityList: &infrastructurev1beta1.SecurityList{
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
		},
		{
			name: "update security list error",
			spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId: "foo",
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID:                 common.String("vcn"),
						PublicRouteTableId: common.String("public"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
								ID:   common.String("sec_list_updated_id"),
								Name: "sec_list_added",
								SecurityList: &infrastructurev1beta1.SecurityList{
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
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociCluster := infrastructurev1beta1.OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					UID: "cluster_uid",
				},
				Spec: tt.spec,
			}
			ociCluster.Spec.OCIResourceIdentifier = "resource_uid"
			s := &ClusterScope{
				VCNClient:  vcnClient,
				OCICluster: &ociCluster,
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "resource_uid",
					},
				},
				Logger: &l,
			}
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
		spec          infrastructurev1beta1.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete subnet is successful",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
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
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
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
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
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
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
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
			ociCluster := infrastructurev1beta1.OCICluster{
				Spec: tt.spec,
				ObjectMeta: metav1.ObjectMeta{
					UID: "cluster_uid",
				},
			}
			ociCluster.Spec.OCIResourceIdentifier = "resource_uid"
			s := &ClusterScope{
				VCNClient:  vcnClient,
				OCICluster: &ociCluster,
				Logger:     &l,
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

func isSubnetsEqual(desiredSubnets []*infrastructurev1beta1.Subnet, actualSubnets []*infrastructurev1beta1.Subnet) bool {
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
