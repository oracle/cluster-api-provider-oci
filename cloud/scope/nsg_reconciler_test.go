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

func TestClusterScope_NSGSpec(t *testing.T) {
	customNSGIngress := []infrastructurev1beta1.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("External access to Kubernetes API endpoint"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
	}
	customNSGEgress := []infrastructurev1beta1.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
				Description: common.String("All traffic to control plane nodes"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
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

	tests := []struct {
		name          string
		spec          infrastructurev1beta1.OCIClusterSpec
		want          []*infrastructurev1beta1.NSG
		wantErr       bool
		expectedError string
	}{
		{
			name:    "all default",
			wantErr: false,
			want: []*infrastructurev1beta1.NSG{
				{
					Name: "control-plane-endpoint",
					Role: "control-plane-endpoint",
					EgressRules: []infrastructurev1beta1.EgressSecurityRuleForNSG{
						{
							EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
								Description: common.String("Kubernetes API traffic to Control Plane"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(6443),
										Min: common.Int(6443),
									},
								},
								DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
								Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
					},
					IngressRules: []infrastructurev1beta1.IngressSecurityRuleForNSG{
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("External access to Kubernetes API endpoint"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(6443),
										Min: common.Int(6443),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String("0.0.0.0/0"),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Path discovery"),
								Protocol:    common.String("1"),
								IcmpOptions: &infrastructurev1beta1.IcmpOptions{
									Type: common.Int(3),
									Code: common.Int(4),
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(VcnDefaultCidr),
							},
						},
					},
				},
				{
					Name: "control-plane",
					Role: "control-plane",
					EgressRules: []infrastructurev1beta1.EgressSecurityRuleForNSG{
						{
							EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
								Description:     common.String("Control Plane access to Internet"),
								Protocol:        common.String("all"),
								DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
								Destination:     common.String("0.0.0.0/0"),
							},
						},
					},
					IngressRules: []infrastructurev1beta1.IngressSecurityRuleForNSG{
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Kubernetes API endpoint to Control Plane(apiserver port) communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(6443),
										Min: common.Int(6443),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneEndpointSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Control plane node to Control Plane(apiserver port) communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(6443),
										Min: common.Int(6443),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Worker Node to Control Plane(apiserver port) communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(6443),
										Min: common.Int(6443),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(WorkerSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("etcd client communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(2379),
										Min: common.Int(2379),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("etcd peer"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(2380),
										Min: common.Int(2380),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking (BGP)"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(179),
										Min: common.Int(179),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking (BGP)"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(179),
										Min: common.Int(179),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(WorkerSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking with IP-in-IP enabled"),
								Protocol:    common.String("4"),
								SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking with IP-in-IP enabled"),
								Protocol:    common.String("4"),
								SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String(WorkerSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Path discovery"),
								Protocol:    common.String("1"),
								IcmpOptions: &infrastructurev1beta1.IcmpOptions{
									Type: common.Int(3),
									Code: common.Int(4),
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(VcnDefaultCidr),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Inbound SSH traffic to Control Plane"),
								Protocol:    common.String("6"),
								SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String("0.0.0.0/0"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(22),
										Min: common.Int(22),
									},
								},
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Control Plane to Control Plane Kubelet Communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(10250),
										Min: common.Int(10250),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
					},
				},
				{
					Name: "service-lb",
					Role: "service-lb",
					EgressRules: []infrastructurev1beta1.EgressSecurityRuleForNSG{
						{
							EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
								Destination:     common.String(WorkerSubnetDefaultCIDR),
								Protocol:        common.String("6"),
								DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(32767),
										Min: common.Int(30000),
									},
								},
								Description: common.String("Service LoadBalancer to default NodePort egress communication"),
							},
						},
					},
					IngressRules: []infrastructurev1beta1.IngressSecurityRuleForNSG{
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Path discovery"),
								Protocol:    common.String("1"),
								IcmpOptions: &infrastructurev1beta1.IcmpOptions{
									Type: common.Int(3),
									Code: common.Int(4),
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(VcnDefaultCidr),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Accept http traffic on port 80"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(80),
										Min: common.Int(80),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String("0.0.0.0/0"),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Accept https traffic on port 443"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(443),
										Min: common.Int(443),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String("0.0.0.0/0"),
							},
						},
					},
				},
				{
					Name: "worker",
					Role: "worker",
					EgressRules: []infrastructurev1beta1.EgressSecurityRuleForNSG{
						{
							EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
								Description:     common.String("Worker node access to Internet"),
								Protocol:        common.String("all"),
								DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
								Destination:     common.String("0.0.0.0/0"),
							},
						},
					},
					IngressRules: []infrastructurev1beta1.IngressSecurityRuleForNSG{
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Inbound SSH traffic to worker node"),
								Protocol:    common.String("6"),
								SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String("0.0.0.0/0"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(22),
										Min: common.Int(22),
									},
								},
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Path discovery"),
								Protocol:    common.String("1"),
								IcmpOptions: &infrastructurev1beta1.IcmpOptions{
									Type: common.Int(3),
									Code: common.Int(4),
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(VcnDefaultCidr),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Control Plane to worker node Kubelet Communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(10250),
										Min: common.Int(10250),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Worker node to worker node Kubelet Communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(10250),
										Min: common.Int(10250),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(WorkerSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking (BGP)"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(179),
										Min: common.Int(179),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking (BGP)"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(179),
										Min: common.Int(179),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(WorkerSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking with IP-in-IP enabled"),
								Protocol:    common.String("4"),
								SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking with IP-in-IP enabled"),
								Protocol:    common.String("4"),
								SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String(WorkerSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Worker node to default NodePort ingress communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(32767),
										Min: common.Int(30000),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(WorkerSubnetDefaultCIDR),
							},
						},
					},
				},
			},
		},
		{
			name:    "some user provided nsg and some user provided security list",
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
							},
							{
								Role: infrastructurev1beta1.WorkerRole,
								Name: "worker-test",
								CIDR: "2.2.2.2/1",
								Type: infrastructurev1beta1.Public,
							},
							{
								Role: infrastructurev1beta1.ServiceLoadBalancerRole,
								Name: "loadbalancer-test",
								CIDR: "5.5.5.5/5",
								Type: infrastructurev1beta1.Private,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name:         "test-cp-endpoint-seclist",
									EgressRules:  customEgress,
									IngressRules: customIngress,
								},
							},
						},
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
							{
								Name:         "test-node",
								Role:         "worker",
								EgressRules:  customNSGEgress,
								IngressRules: customNSGIngress,
							},
							{
								Name:         "test-nsg",
								Role:         "service-lb",
								EgressRules:  customNSGEgress,
								IngressRules: customNSGIngress,
							},
						},
					},
				},
			},
			want: []*infrastructurev1beta1.NSG{
				{
					Name:         "test-node",
					Role:         "worker",
					EgressRules:  customNSGEgress,
					IngressRules: customNSGIngress,
				},
				{
					Name:         "test-nsg",
					Role:         "service-lb",
					EgressRules:  customNSGEgress,
					IngressRules: customNSGIngress,
				},
				{
					Name: "control-plane",
					Role: "control-plane",
					EgressRules: []infrastructurev1beta1.EgressSecurityRuleForNSG{
						{
							EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
								Description:     common.String("Control Plane access to Internet"),
								Protocol:        common.String("all"),
								DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
								Destination:     common.String("0.0.0.0/0"),
							},
						},
					},
					IngressRules: []infrastructurev1beta1.IngressSecurityRuleForNSG{
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Kubernetes API endpoint to Control Plane(apiserver port) communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(6443),
										Min: common.Int(6443),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneEndpointSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Control plane node to Control Plane(apiserver port) communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(6443),
										Min: common.Int(6443),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Worker Node to Control Plane(apiserver port) communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(6443),
										Min: common.Int(6443),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(WorkerSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("etcd client communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(2379),
										Min: common.Int(2379),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("etcd peer"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(2380),
										Min: common.Int(2380),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking (BGP)"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(179),
										Min: common.Int(179),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking (BGP)"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(179),
										Min: common.Int(179),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(WorkerSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking with IP-in-IP enabled"),
								Protocol:    common.String("4"),
								SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Calico networking with IP-in-IP enabled"),
								Protocol:    common.String("4"),
								SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String(WorkerSubnetDefaultCIDR),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Path discovery"),
								Protocol:    common.String("1"),
								IcmpOptions: &infrastructurev1beta1.IcmpOptions{
									Type: common.Int(3),
									Code: common.Int(4),
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(VcnDefaultCidr),
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Inbound SSH traffic to Control Plane"),
								Protocol:    common.String("6"),
								SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:      common.String("0.0.0.0/0"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(22),
										Min: common.Int(22),
									},
								},
							},
						},
						{
							IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
								Description: common.String("Control Plane to Control Plane Kubelet Communication"),
								Protocol:    common.String("6"),
								TcpOptions: &infrastructurev1beta1.TcpOptions{
									DestinationPortRange: &infrastructurev1beta1.PortRange{
										Max: common.Int(10250),
										Min: common.Int(10250),
									},
								},
								SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
								Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
							},
						},
					},
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
						UID: "cluster_uid",
					},
				},
				Logger: &l,
			}
			nsgs, err := s.NSGSpec()
			if !isNSGEqual(tt.want, nsgs) {
				t.Errorf("NSGSpec() want = %v, got %v", tt.want, nsgs)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("NSGSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("NSGSpec() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}
			}
		})
	}
}

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
		spec          infrastructurev1beta1.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete nsg is successful",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
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
			wantErr: false,
		},
		{
			name: "nsg already deleted",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
							{
								ID: common.String("nsg_deleted"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete nsg error when calling get route table",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
							{
								ID: common.String("nsg_get_error"),
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
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
							{
								ID: common.String("nsg_error_delete"),
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

	customNSGIngress := []infrastructurev1beta1.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
				Description: common.String("External access to Kubernetes API endpoint"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: infrastructurev1beta1.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
	}
	customNSGEgress := []infrastructurev1beta1.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
				Description: common.String("All traffic to control plane nodes"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta1.TcpOptions{
					DestinationPortRange: &infrastructurev1beta1.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
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

	vcnClient.EXPECT().UpdateNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.UpdateNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("update-id"),
		UpdateNetworkSecurityGroupDetails: core.UpdateNetworkSecurityGroupDetails{
			FreeformTags: tags,
			DefinedTags:  definedTagsInterface,
			DisplayName:  common.String("update-nsg"),
		},
	})).
		Return(core.UpdateNetworkSecurityGroupResponse{
			NetworkSecurityGroup: core.NetworkSecurityGroup{
				Id:           common.String("update-id"),
				FreeformTags: tags,
			},
		}, nil)

	vcnClient.EXPECT().UpdateNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.UpdateNetworkSecurityGroupRequest{
		NetworkSecurityGroupId: common.String("update-id"),
		UpdateNetworkSecurityGroupDetails: core.UpdateNetworkSecurityGroupDetails{
			FreeformTags: tags,
			DefinedTags:  definedTagsInterface,
			DisplayName:  common.String("update-nsg-error"),
		},
	})).
		Return(core.UpdateNetworkSecurityGroupResponse{}, errors.New("some error"))

	vcnClient.EXPECT().ListNetworkSecurityGroups(gomock.Any(), gomock.Eq(core.ListNetworkSecurityGroupsRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("service-lb"),
		VcnId:         common.String("vcn"),
	})).Return(
		core.ListNetworkSecurityGroupsResponse{
			Items: []core.NetworkSecurityGroup{}}, nil).Times(3)

	vcnClient.EXPECT().CreateNetworkSecurityGroup(gomock.Any(), gomock.Eq(core.CreateNetworkSecurityGroupRequest{
		CreateNetworkSecurityGroupDetails: core.CreateNetworkSecurityGroupDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("service-lb"),
			VcnId:         common.String("vcn"),
			FreeformTags:  tags,
			DefinedTags:   definedTagsInterface,
		},
	})).Return(
		core.CreateNetworkSecurityGroupResponse{
			NetworkSecurityGroup: core.NetworkSecurityGroup{
				Id: common.String("loadbalancer-nsg-id"),
			},
		}, nil).Times(2)

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

	vcnClient.EXPECT().AddNetworkSecurityGroupSecurityRules(gomock.Any(), gomock.Eq(core.AddNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: common.String("loadbalancer-nsg-id"),
		AddNetworkSecurityGroupSecurityRulesDetails: core.AddNetworkSecurityGroupSecurityRulesDetails{SecurityRules: []core.AddSecurityRuleDetails{
			{
				Direction:   core.AddSecurityRuleDetailsDirectionIngress,
				Protocol:    common.String("1"),
				Description: common.String("Path discovery"),
				IcmpOptions: &core.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				IsStateless: common.Bool(false),
				Source:      common.String(VcnDefaultCidr),
				SourceType:  core.AddSecurityRuleDetailsSourceTypeCidrBlock,
			},
			{
				Direction:   core.AddSecurityRuleDetailsDirectionIngress,
				Protocol:    common.String("6"),
				Description: common.String("Accept http traffic on port 80"),
				Source:      common.String("0.0.0.0/0"),
				SourceType:  core.AddSecurityRuleDetailsSourceTypeCidrBlock,
				IsStateless: common.Bool(false),
				TcpOptions: &core.TcpOptions{
					DestinationPortRange: &core.PortRange{
						Max: common.Int(80),
						Min: common.Int(80),
					},
				},
			},
			{
				Direction:   core.AddSecurityRuleDetailsDirectionIngress,
				Protocol:    common.String("6"),
				Description: common.String("Accept https traffic on port 443"),
				Source:      common.String("0.0.0.0/0"),
				SourceType:  core.AddSecurityRuleDetailsSourceTypeCidrBlock,
				IsStateless: common.Bool(false),
				TcpOptions: &core.TcpOptions{
					DestinationPortRange: &core.PortRange{
						Max: common.Int(443),
						Min: common.Int(443),
					},
				},
			},
			{
				Direction:       core.AddSecurityRuleDetailsDirectionEgress,
				Protocol:        common.String("6"),
				Description:     common.String("Service LoadBalancer to default NodePort egress communication"),
				Destination:     common.String(WorkerSubnetDefaultCIDR),
				DestinationType: core.AddSecurityRuleDetailsDestinationTypeCidrBlock,
				IsStateless:     common.Bool(false),
				TcpOptions: &core.TcpOptions{
					DestinationPortRange: &core.PortRange{
						Max: common.Int(32767),
						Min: common.Int(30000),
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
				{

					Id: common.String("bar"),
				},
			},
		},
	}, nil)

	vcnClient.EXPECT().AddNetworkSecurityGroupSecurityRules(gomock.Any(), gomock.Eq(core.AddNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: common.String("loadbalancer-nsg-id"),
		AddNetworkSecurityGroupSecurityRulesDetails: core.AddNetworkSecurityGroupSecurityRulesDetails{SecurityRules: []core.AddSecurityRuleDetails{
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
		},
		},
	})).Return(core.AddNetworkSecurityGroupSecurityRulesResponse{}, errors.New("some error"))

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

	customNSGEgressWithId := make([]infrastructurev1beta1.EgressSecurityRuleForNSG, len(customNSGEgress))
	copy(customNSGEgressWithId, customNSGEgress)

	customNSGIngressWithId := make([]infrastructurev1beta1.IngressSecurityRuleForNSG, len(customNSGIngress))
	copy(customNSGIngressWithId, customNSGIngress)

	tests := []struct {
		name          string
		spec          infrastructurev1beta1.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "one creation - one update - one update security rule - one no update",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID: common.String("vcn"),
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
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
				DefinedTags:   definedTags,
				CompartmentId: "foo",
			},
		},
		{
			name: "update failed",
			spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID: common.String("vcn"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta1.ServiceLoadBalancerRole,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta1.ControlPlaneRole,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name: "foo",
								},
							},
						},
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
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
			wantErr:       true,
			expectedError: "failed to reconcile the network security group, failed to update: some error",
		},
		{
			name: "creation failed",
			spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				FreeformTags: map[string]string{
					"foo": "bar",
				},
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID: common.String("vcn"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta1.WorkerRole,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta1.ControlPlaneRole,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name: "foo",
								},
							},
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed create nsg: some error",
		},
		{
			name: "add rules failed",
			spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID: common.String("vcn"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.ControlPlaneEndpointRole,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta1.WorkerRole,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name: "foo",
								},
							},
							{
								Role: infrastructurev1beta1.ControlPlaneRole,
								SecurityList: &infrastructurev1beta1.SecurityList{
									Name: "foo",
								},
							},
						},
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
							{
								Name:        "service-lb",
								Role:        "service-lb",
								EgressRules: customNSGEgress,
							},
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed add nsg security rules: some error",
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

func isNSGEqual(desiredNSGs []*infrastructurev1beta1.NSG, actualNSGs []*infrastructurev1beta1.NSG) bool {
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
