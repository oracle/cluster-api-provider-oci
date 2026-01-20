/*
 *
 * Copyright (c) 2022, Oracle and/or its affiliates.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 *
 */

package v1beta2

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/oracle/oci-go-sdk/v65/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

func TestOCICluster_ValidateCreate(t *testing.T) {
	goodSubnets := []*Subnet{
		&Subnet{
			Role: ControlPlaneRole,
			Name: "test-subnet",
			CIDR: "10.0.0.0/16",
		},
	}
	badSubnetCidr := []*Subnet{
		&Subnet{
			Name: "test-subnet",
			CIDR: "10.1.0.0/16",
		},
	}
	emptySubnetCidr := []*Subnet{
		&Subnet{
			Role: ControlPlaneRole,
			Name: "test-subnet",
		},
	}
	badSubnetCidrFormat := []*Subnet{
		&Subnet{
			Name: "test-subnet",
			CIDR: "no-a-cidr",
		},
	}
	dupSubnetNames := []*Subnet{
		&Subnet{
			Name: "dup-name",
			CIDR: "10.0.0.0/16",
		},
		&Subnet{
			Name: "dup-name",
			CIDR: "10.0.0.0/16",
		},
	}
	emptySubnetName := []*Subnet{
		&Subnet{
			Name: "",
			CIDR: "10.0.0.0/16",
			Role: ControlPlaneEndpointRole,
		},
	}
	badSubnetRole := []*Subnet{
		&Subnet{
			Role: "not-control-plane",
		},
	}
	goodClusterName := "test-cluster"
	badClusterName := "bad.cluster"

	tests := []struct {
		name                          string
		c                             *OCICluster
		errorMgsShouldContain         string
		multipleErrorMgsShouldContain []string
		expectErr                     bool
	}{
		{
			name: "shouldn't allow spaces in region",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					Region: "us city 1",
				},
			},
			errorMgsShouldContain: "region",
			expectErr:             true,
		},
		{
			name: "shouldn't allow bad CompartmentId",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId: "badocid",
				},
			},
			errorMgsShouldContain: "compartmentId",
			expectErr:             true,
		},
		{
			name: "shouldn't allow blank CompartmentId",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{},
			},
			errorMgsShouldContain: "compartmentId",
			expectErr:             true,
		},
		{
			name: "shouldn't allow bad vcn cider",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR: "not-a-cidr",
						},
					},
				},
			},
			errorMgsShouldContain: "invalid CIDR format",
			expectErr:             true,
		},
		{
			name: "shouldn't allow blank OCIResourceIdentifier",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid",
				},
			},
			errorMgsShouldContain: "ociResourceIdentifier",
			expectErr:             true,
		},
		{
			name: "shouldn't allow subnet cidr outside of vcn cidr",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: badSubnetCidr,
						},
					},
				},
			},
			errorMgsShouldContain: "cidr",
			expectErr:             true,
		},
		{
			name: "should allow empty subnet cidr",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: emptySubnetCidr,
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "shouldn't allow subnet bad cidr format",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: badSubnetCidrFormat,
						},
					},
				},
			},
			errorMgsShouldContain: "invalid CIDR format",
			expectErr:             true,
		},
		{
			name: "shouldn't allow subnet cidr outside of vcn cidr",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: dupSubnetNames,
						},
					},
				},
			},
			errorMgsShouldContain: "networkSpec.subnets: Duplicate value",
			expectErr:             true,
		},
		{
			name: "shouldn't allow subnet role outside of pre-defined roles",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: badSubnetRole,
						},
					},
				},
			},
			errorMgsShouldContain: "subnet role invalid",
			expectErr:             true,
		},
		{
			name: "allow subnet custom role",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR: "10.0.0.0/16",
							Subnets: []*Subnet{
								&Subnet{
									Role: Custom,
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "shouldn't allow invalid role",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR: "10.0.0.0/16",
							Subnets: []*Subnet{
								&Subnet{
									Role: PodRole,
								},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "subnet role invalid",
			expectErr:             true,
		},
		{
			name: "shouldn't allow bad cluster names",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: badClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: emptySubnetName,
						},
					},
				},
			},
			errorMgsShouldContain: "Cluster Name doesn't match regex",
			expectErr:             true,
		},
		{
			name: "should allow empty subnet name",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: emptySubnetName,
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "shouldn't allow bad NSG egress cidr",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									EgressRules: []EgressSecurityRuleForNSG{{
										EgressSecurityRule: EgressSecurityRule{
											Destination:     common.String("bad/15"),
											DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "invalid egressRules: CIDR format",
			expectErr:             true,
		},
		{
			name: "shouldn't allow empty NSG egress destination",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: Custom,
									EgressRules: []EgressSecurityRuleForNSG{{
										EgressSecurityRule: EgressSecurityRule{
											Destination:     nil,
											DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
											Protocol:        common.String("all"),
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "invalid egressRules: Destination may not be empty",
			expectErr:             true,
		},
		{
			name: "shouldn't allow empty NSG egress icmpOptions nil",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: Custom,
									EgressRules: []EgressSecurityRuleForNSG{{
										EgressSecurityRule: EgressSecurityRule{
											Destination:     common.String("0.0.0.0/0"),
											DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
											Protocol:        common.String("all"),
											IcmpOptions: &IcmpOptions{
												Type: nil,
											},
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "invalid egressRules: IcmpOptions Type may not be empty",
			expectErr:             true,
		},
		{
			name: "shouldn't allow empty NSG egress protocol",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: Custom,
									EgressRules: []EgressSecurityRuleForNSG{{
										EgressSecurityRule: EgressSecurityRule{
											Destination:     common.String("10.0.0.0/15"),
											DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
											Protocol:        nil,
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "invalid egressRules: Protocol may not be empty",
			expectErr:             true,
		},
		{
			name: "shouldn't allow empty NSG egress udpSource PortRange Max",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: Custom,
									EgressRules: []EgressSecurityRuleForNSG{{
										EgressSecurityRule: EgressSecurityRule{
											Destination:     common.String("10.0.0.0/15"),
											DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
											Protocol:        common.String("all"),
											UdpOptions: &UdpOptions{
												DestinationPortRange: &PortRange{
													Max: nil,
													Min: common.Int(80),
												},
											},
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "invalid egressRules: UdpOptions DestinationPortRange Max may not be empty",
			expectErr:             true,
		},
		{
			name: "shouldn't allow empty NSG egress rule required values for port ranges",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: Custom,
									EgressRules: []EgressSecurityRuleForNSG{{
										EgressSecurityRule: EgressSecurityRule{
											Destination:     common.String("10.0.0.0/15"),
											DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
											Protocol:        common.String("all"),
											UdpOptions: &UdpOptions{
												DestinationPortRange: &PortRange{
													Max: nil, // required
													Min: nil, // required
												},
												SourcePortRange: &PortRange{
													Max: nil, // required
													Min: nil, // required
												},
											},
											TcpOptions: &TcpOptions{
												DestinationPortRange: &PortRange{
													Max: nil, // required
													Min: nil, // required
												},
												SourcePortRange: &PortRange{
													Max: nil, // required
													Min: nil, // required
												},
											},
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "",
			multipleErrorMgsShouldContain: []string{
				"invalid egressRules: UdpOptions DestinationPortRange Max may not be empty",
				"invalid egressRules: UdpOptions DestinationPortRange Min may not be empty",
				"invalid egressRules: UdpOptions SourcePortRange Max may not be empty",
				"invalid egressRules: UdpOptions SourcePortRange Min may not be empty",
				"invalid egressRules: TcpOptions DestinationPortRange Max may not be empty",
				"invalid egressRules: TcpOptions DestinationPortRange Min may not be empty",
				"invalid egressRules: TcpOptions SourcePortRange Max may not be empty",
				"invalid egressRules: TcpOptions SourcePortRange Min may not be empty",
			},
			expectErr: true,
		},
		{
			name: "shouldn't allow bad NSG ingress cidr",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									IngressRules: []IngressSecurityRuleForNSG{{
										IngressSecurityRule: IngressSecurityRule{
											Source:     common.String("bad/15"),
											SourceType: IngressSecurityRuleSourceTypeCidrBlock,
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "invalid ingressRules: CIDR format",
			expectErr:             true,
		},
		{
			name: "shouldn't allow empty NSG ingress protocol",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: Custom,
									IngressRules: []IngressSecurityRuleForNSG{{
										IngressSecurityRule: IngressSecurityRule{
											Source:     common.String("10.0.0.0/15"),
											SourceType: IngressSecurityRuleSourceTypeCidrBlock,
											Protocol:   nil,
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "invalid ingressRules: Protocol may not be empty",
			expectErr:             true,
		},
		{
			name: "shouldn't allow empty NSG ingress source",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: Custom,
									IngressRules: []IngressSecurityRuleForNSG{{
										IngressSecurityRule: IngressSecurityRule{
											Source:     nil,
											SourceType: IngressSecurityRuleSourceTypeCidrBlock,
											Protocol:   common.String("all"),
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "invalid ingressRules: Source may not be empty",
			expectErr:             true,
		},
		{
			name: "shouldn't allow empty NSG ingress rule required values for port ranges",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: Custom,
									IngressRules: []IngressSecurityRuleForNSG{{
										IngressSecurityRule: IngressSecurityRule{
											Source:     common.String("10.0.0.0/15"),
											SourceType: IngressSecurityRuleSourceTypeCidrBlock,
											Protocol:   common.String("all"),
											UdpOptions: &UdpOptions{
												DestinationPortRange: &PortRange{
													Max: nil, // required
													Min: nil, // required
												},
												SourcePortRange: &PortRange{
													Max: nil, // required
													Min: nil, // required
												},
											},
											TcpOptions: &TcpOptions{
												DestinationPortRange: &PortRange{
													Max: nil, // required
													Min: nil, // required
												},
												SourcePortRange: &PortRange{
													Max: nil, // required
													Min: nil, // required
												},
											},
										},
									}},
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "",
			multipleErrorMgsShouldContain: []string{
				"invalid ingressRules: UdpOptions DestinationPortRange Max may not be empty",
				"invalid ingressRules: UdpOptions DestinationPortRange Min may not be empty",
				"invalid ingressRules: UdpOptions SourcePortRange Max may not be empty",
				"invalid ingressRules: UdpOptions SourcePortRange Min may not be empty",
				"invalid ingressRules: TcpOptions DestinationPortRange Max may not be empty",
				"invalid ingressRules: TcpOptions DestinationPortRange Min may not be empty",
				"invalid ingressRules: TcpOptions SourcePortRange Max may not be empty",
				"invalid ingressRules: TcpOptions SourcePortRange Min may not be empty",
			},
			expectErr: true,
		},
		{
			name: "shouldn't allow bad NSG role",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: "bad-role",
								}},
							},
						},
					},
				},
			},
			errorMgsShouldContain: "networkSecurityGroup role invalid",
			expectErr:             true,
		},
		{
			name: "shouldn't allow invalid NSG role",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{List: []*NSG{{
								Role: PodRole,
							}}},
						},
					},
				},
			},
			errorMgsShouldContain: "networkSecurityGroup role invalid",
			expectErr:             true,
		},
		{
			name: "allow nsg custom role",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{List: []*NSG{{
								Role: Custom,
							}}},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "should allow blank region",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					Region:                "",
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: goodSubnets,
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "should succeed",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIClusterSpec{
					Region:                "us-lexington-1",
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: goodSubnets,
						},
					},
				},
			},
			expectErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			if test.expectErr {
				_, err := (&OCIClusterWebhook{}).ValidateCreate(context.Background(), test.c)
				g.Expect(err).NotTo(gomega.Succeed())

				// handle the tests that produce multiple error messages
				// to help keep the number of tests down
				if len(test.multipleErrorMgsShouldContain) > 0 {
					for _, errMgs := range test.multipleErrorMgsShouldContain {
						g.Expect(strings.Contains(err.Error(), errMgs)).To(gomega.BeTrue())
					}
				} else {
					g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
				}
			} else {
				_, err := (&OCIClusterWebhook{}).ValidateCreate(context.Background(), test.c)
				g.Expect(err).To(gomega.Succeed())
			}
		})
	}
}

func TestOCIClusterWebhook_ValidateDelete(t *testing.T) {
	tests := []struct {
		name    string
		obj     runtime.Object
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid OCICluster object",
			obj:     &OCICluster{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"}},
			wantErr: false,
		},
		{
			name:    "invalid object type",
			obj:     &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "not-a-cluster"}},
			wantErr: true,
			errMsg:  "expected an OCICluster object",
		},
		{
			name:    "nil object",
			obj:     nil,
			wantErr: true,
			errMsg:  "expected an OCICluster object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &OCIClusterWebhook{}
			warnings, err := w.ValidateDelete(context.Background(), tt.obj)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q but got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// In both cases warnings should be nil
			if warnings != nil {
				t.Errorf("expected no warnings but got: %v", warnings)
			}
		})
	}
}

func TestOCICluster_ValidateUpdate(t *testing.T) {
	goodSubnets := []*Subnet{
		&Subnet{
			Role: ControlPlaneRole,
			Name: "test-subnet",
			CIDR: "10.0.0.0/16",
		},
	}

	tests := []struct {
		name                  string
		c                     *OCICluster
		old                   *OCICluster
		errorMgsShouldContain string
		expectErr             bool
	}{
		{
			name: "shouldn't allow region change",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					Region: "new-region",
				},
			},
			old: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					Region: "old-region",
				},
			},
			errorMgsShouldContain: "region",
			expectErr:             true,
		},
		{
			name: "shouldn't allow compartmentId change",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid.old",
				},
			},
			old: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid.new",
				},
			},
			errorMgsShouldContain: "compartmentId",
			expectErr:             true,
		},
		{
			name: "shouldn't change OCIResourceIdentifier",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					Region:                "old-region",
					OCIResourceIdentifier: "uuid-1",
				},
			},
			old: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					Region:                "old-region",
					OCIResourceIdentifier: "uuid-2",
				},
			},
			errorMgsShouldContain: "ociResourceIdentifier",
			expectErr:             true,
		},
		{
			name: "should succeed",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: OCIClusterSpec{
					CompartmentId:         "ocid",
					Region:                "old-region",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: goodSubnets,
						},
					},
				},
			},
			old: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: OCIClusterSpec{
					Region:                "old-region",
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR:    "10.0.0.0/16",
							Subnets: goodSubnets,
						},
					},
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			if test.expectErr {
				_, err := (&OCIClusterWebhook{}).ValidateUpdate(context.Background(), test.old, test.c)
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				_, err := (&OCIClusterWebhook{}).ValidateUpdate(context.Background(), test.old, test.c)
				g.Expect(err).To(gomega.Succeed())
			}
		})
	}
}

func TestOCICluster_CreateDefault(t *testing.T) {

	tests := []struct {
		name   string
		c      *OCICluster
		expect func(g *gomega.WithT, c *OCICluster)
	}{
		{
			name: "should set default OCIResourceIdentifier",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "badocid",
				},
			},
			expect: func(g *gomega.WithT, c *OCICluster) {
				g.Expect(c.Spec.OCIResourceIdentifier).To(Not(BeNil()))
			},
		},
		{
			name: "should set default subnets",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid",
				},
			},
			expect: func(g *gomega.WithT, c *OCICluster) {
				g.Expect(c.Spec.NetworkSpec.Vcn.Subnets).To(Equal(c.SubnetSpec()))
			},
		},
		{
			name: "should set default nsg",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid",
				},
			},
			expect: func(g *gomega.WithT, c *OCICluster) {
				g.Expect(c.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List).To(Equal(c.NSGSpec()))
			},
		},
		{
			name: "should set default nsg",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								Skip: true,
							},
						},
					},
				},
			},
			expect: func(g *gomega.WithT, c *OCICluster) {
				g.Expect(len(c.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List)).To(Equal(0))
			},
		},
		{
			name: "should add missing subnets",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							Subnets: []*Subnet{
								{
									Role: WorkerRole,
									Name: WorkerDefaultName,
								},
								{
									Role: ControlPlaneEndpointRole,
									Name: ControlPlaneEndpointDefaultName,
								},
							},
						},
					},
				},
			},
			expect: func(g *gomega.WithT, c *OCICluster) {
				subnets := []*Subnet{
					{
						Role: WorkerRole,
						Name: WorkerDefaultName,
						CIDR: "10.0.64.0/20",
					},
					{
						Role: ControlPlaneEndpointRole,
						Name: ControlPlaneEndpointDefaultName,
						CIDR: "10.0.0.8/29",
					},
					{
						Role: ControlPlaneRole,
						Name: ControlPlaneDefaultName,
						CIDR: ControlPlaneMachineSubnetDefaultCIDR,
						Type: Private,
					},
					{
						Role: ServiceLoadBalancerRole,
						Name: ServiceLBDefaultName,
						CIDR: ServiceLoadBalancerDefaultCIDR,
						Type: Public,
					},
				}
				g.Expect(c.Spec.NetworkSpec.Vcn.Subnets).To(Equal(subnets))
			},
		},
		{
			name: "should add missing nsg",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{
									{
										Role: ServiceLoadBalancerRole,
										Name: ServiceLBDefaultName,
									},
									{
										Role: ControlPlaneEndpointRole,
										Name: ControlPlaneEndpointDefaultName,
									},
								},
							},
						},
					},
				},
			},
			expect: func(g *gomega.WithT, c *OCICluster) {
				nsgs := []*NSG{
					{
						Role: ServiceLoadBalancerRole,
						Name: ServiceLBDefaultName,
					},
					{
						Role: ControlPlaneEndpointRole,
						Name: ControlPlaneEndpointDefaultName,
					},
					{
						Role:         ControlPlaneRole,
						Name:         ControlPlaneDefaultName,
						IngressRules: c.GetControlPlaneMachineDefaultIngressRules(),
						EgressRules:  c.GetControlPlaneMachineDefaultEgressRules(),
					},
					{
						Role:         WorkerRole,
						Name:         WorkerDefaultName,
						IngressRules: c.GetNodeDefaultIngressRules(),
						EgressRules:  c.GetNodeDefaultEgressRules(),
					},
				}
				g.Expect(c.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List).To(Equal(nsgs))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			(&OCIClusterWebhook{}).Default(context.Background(), test.c)
			test.expect(g, test.c)
		})
	}
}

func TestOCICluster_GetConditions(t *testing.T) {
	tests := []struct {
		name    string
		cluster *OCICluster
		want    clusterv1.Conditions
	}{
		{
			name: "no conditions set, returns empty",
			cluster: &OCICluster{
				Status: OCIClusterStatus{
					Conditions: nil,
				},
			},
			want: nil,
		},
		{
			name: "returns existing conditions",
			cluster: &OCICluster{
				Status: OCIClusterStatus{
					Conditions: clusterv1.Conditions{
						{
							Type:   clusterv1.ReadyCondition,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: clusterv1.Conditions{
				{
					Type:   clusterv1.ReadyCondition,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cluster.GetConditions(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetConditions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOCICluster_SetConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions clusterv1.Conditions
	}{
		{
			name: "set single condition",
			conditions: clusterv1.Conditions{
				{
					Type:   "Ready",
					Status: corev1.ConditionTrue,
				},
			},
		},
		{
			name:       "set empty conditions",
			conditions: clusterv1.Conditions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &OCICluster{}
			c.SetConditions(tt.conditions)

			if !reflect.DeepEqual(c.Status.Conditions, tt.conditions) {
				t.Errorf("SetConditions() got = %v, want %v", c.Status.Conditions, tt.conditions)
			}
		})
	}
}

func TestOCICluster_GetOCIResourceIdentifier(t *testing.T) {
	tests := []struct {
		name string
		spec OCIClusterSpec
		want string
	}{
		{
			name: "with resource identifier",
			spec: OCIClusterSpec{
				OCIResourceIdentifier: "resource-123",
			},
			want: "resource-123",
		},
		{
			name: "empty resource identifier",
			spec: OCIClusterSpec{
				OCIResourceIdentifier: "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &OCICluster{
				Spec: tt.spec,
			}
			if got := c.GetOCIResourceIdentifier(); got != tt.want {
				t.Errorf("GetOCIResourceIdentifier() = %v, want %v", got, tt.want)
			}
		})
	}
}
