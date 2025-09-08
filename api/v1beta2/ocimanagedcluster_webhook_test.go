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
	"strings"
	"testing"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/oracle/oci-go-sdk/v65/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOCIManagedCluster_ValidateCreate(t *testing.T) {
	goodSubnets := []*Subnet{
		&Subnet{
			Role: ControlPlaneEndpointRole,
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
			Role: ControlPlaneEndpointRole,
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
		name                  string
		c                     *OCIManagedCluster
		errorMgsShouldContain string
		expectErr             bool
	}{
		{
			name: "shouldn't allow spaces in region",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
					Region: "us city 1",
				},
			},
			errorMgsShouldContain: "region",
			expectErr:             true,
		},
		{
			name: "shouldn't allow bad CompartmentId",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
					CompartmentId: "badocid",
				},
			},
			errorMgsShouldContain: "compartmentId",
			expectErr:             true,
		},
		{
			name: "shouldn't allow blank CompartmentId",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{},
			},
			errorMgsShouldContain: "compartmentId",
			expectErr:             true,
		},
		{
			name: "shouldn't allow bad vcn cider",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
					CompartmentId: "ocid",
				},
			},
			errorMgsShouldContain: "ociResourceIdentifier",
			expectErr:             true,
		},
		{
			name: "shouldn't allow subnet cidr outside of vcn cidr",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			name: "shouldn't allow invalid role",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							CIDR: "10.0.0.0/16",
							Subnets: []*Subnet{
								&Subnet{
									Role: ControlPlaneRole,
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
			name: "should allow custom subnet role",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
					Region:                "",
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
			name: "should allow empty subnet name",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			errorMgsShouldContain: "invalid egressRules CIDR format",
			expectErr:             true,
		},
		{
			name: "shouldn't allow bad NSG ingress cidr",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			errorMgsShouldContain: "invalid ingressRule CIDR format",
			expectErr:             true,
		},
		{
			name: "shouldn't allow bad NSG role",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: ControlPlaneRole,
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
			name: "should allow custom NSG role",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: NetworkSpec{
						Vcn: VCN{
							NetworkSecurityGroup: NetworkSecurityGroup{
								List: []*NSG{{
									Role: Custom,
								}},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "should allow blank region",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-name",
				},
				Spec: OCIManagedClusterSpec{
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
			name: "shouldn't allow loadbalancer",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
					CompartmentId: "ocid",
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancer{
							Name: "test",
						},
					},
				},
			},
			errorMgsShouldContain: "loadbalancer",
			expectErr:             true,
		},
		{
			name: "should succeed",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-name",
				},
				Spec: OCIManagedClusterSpec{
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
		{
			name: "shouldn't allow bad cluster names",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: badClusterName,
				},
				Spec: OCIManagedClusterSpec{
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			if test.expectErr {
				_, err := (&OCIManagedClusterWebhook{}).ValidateCreate(context.Background(), test.c)
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				_, err := (&OCIManagedClusterWebhook{}).ValidateCreate(context.Background(), test.c)
				g.Expect(err).To(gomega.Succeed())
			}
		})
	}
}

func TestOCIManagedCluster_ValidateUpdate(t *testing.T) {
	goodSubnets := []*Subnet{
		&Subnet{
			Role: ControlPlaneEndpointRole,
			Name: "test-subnet",
			CIDR: "10.0.0.0/16",
		},
	}

	tests := []struct {
		name                  string
		c                     *OCIManagedCluster
		old                   *OCIManagedCluster
		errorMgsShouldContain string
		expectErr             bool
	}{
		{
			name: "shouldn't allow region change",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
					Region: "new-region",
				},
			},
			old: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
					Region: "old-region",
				},
			},
			errorMgsShouldContain: "region",
			expectErr:             true,
		},
		{
			name: "shouldn't allow compartmentId change",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
					CompartmentId: "ocid.old",
				},
			},
			old: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
					CompartmentId: "ocid.new",
				},
			},
			errorMgsShouldContain: "compartmentId",
			expectErr:             true,
		},
		{
			name: "shouldn't change OCIResourceIdentifier",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
					CompartmentId:         "ocid",
					Region:                "old-region",
					OCIResourceIdentifier: "uuid-1",
				},
			},
			old: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
					Region:                "old-region",
					OCIResourceIdentifier: "uuid-2",
				},
			},
			errorMgsShouldContain: "ociResourceIdentifier",
			expectErr:             true,
		},
		{
			name: "should succeed",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: OCIManagedClusterSpec{
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
			old: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: OCIManagedClusterSpec{
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
				_, err := (&OCIManagedClusterWebhook{}).ValidateUpdate(context.Background(), test.old, test.c)
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				_, err := (&OCIManagedClusterWebhook{}).ValidateUpdate(context.Background(), test.old, test.c)
				g.Expect(err).To(gomega.Succeed())
			}
		})
	}
}

func TestOCIManagedCluster_CreateDefault(t *testing.T) {

	tests := []struct {
		name   string
		c      *OCIManagedCluster
		expect func(g *gomega.WithT, c *OCIManagedCluster)
	}{
		{
			name: "should set default OCIResourceIdentifier",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
					CompartmentId: "badocid",
				},
			},
			expect: func(g *gomega.WithT, c *OCIManagedCluster) {
				g.Expect(c.Spec.OCIResourceIdentifier).To(Not(BeNil()))
			},
		},
		{
			name: "should set default subnets",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
					CompartmentId: "ocid",
				},
			},
			expect: func(g *gomega.WithT, c *OCIManagedCluster) {
				subnets := make([]*Subnet, 0)
				subnets = append(subnets, &Subnet{
					Role: ControlPlaneEndpointRole,
					Name: ControlPlaneEndpointDefaultName,
					CIDR: ControlPlaneEndpointSubnetDefaultCIDR,
					Type: Public,
				})

				subnets = append(subnets, &Subnet{
					Role: ServiceLoadBalancerRole,
					Name: ServiceLBDefaultName,
					CIDR: ServiceLoadBalancerDefaultCIDR,
					Type: Public,
				})
				subnets = append(subnets, &Subnet{
					Role: WorkerRole,
					Name: WorkerDefaultName,
					CIDR: WorkerSubnetDefaultCIDR,
					Type: Private,
				})
				subnets = append(subnets, &Subnet{
					Role: PodRole,
					Name: PodDefaultName,
					CIDR: PodDefaultCIDR,
					Type: Private,
				})
				g.Expect(c.Spec.NetworkSpec.Vcn.Subnets).To(Equal(subnets))
			},
		},
		{
			name: "should set default nsg",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
					CompartmentId: "ocid",
				},
			},
			expect: func(g *gomega.WithT, c *OCIManagedCluster) {
				nsgs := make([]*NSG, 4)
				nsgs[0] = &NSG{
					Role:         ControlPlaneEndpointRole,
					Name:         ControlPlaneEndpointDefaultName,
					IngressRules: c.GetControlPlaneEndpointDefaultIngressRules(),
					EgressRules:  c.GetControlPlaneEndpointDefaultEgressRules(),
				}
				nsgs[1] = &NSG{
					Role:         WorkerRole,
					Name:         WorkerDefaultName,
					IngressRules: c.GetWorkerDefaultIngressRules(),
					EgressRules:  c.GetWorkerDefaultEgressRules(),
				}
				nsgs[2] = &NSG{
					Role:         ServiceLoadBalancerRole,
					Name:         ServiceLBDefaultName,
					IngressRules: c.GetLBServiceDefaultIngressRules(),
					EgressRules:  c.GetLBServiceDefaultEgressRules(),
				}
				nsgs[3] = &NSG{
					Role:         PodRole,
					Name:         PodDefaultName,
					IngressRules: c.GetPodDefaultIngressRules(),
					EgressRules:  c.GetPodDefaultEgressRules(),
				}
				g.Expect(c.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List).To(Equal(nsgs))
			},
		},
		{
			name: "should set default nsg",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIManagedClusterSpec{
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
			expect: func(g *gomega.WithT, c *OCIManagedCluster) {
				g.Expect(len(c.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List)).To(Equal(0))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			(&OCIManagedClusterWebhook{}).Default(context.Background(), test.c)
			test.expect(g, test.c)
		})
	}
}
