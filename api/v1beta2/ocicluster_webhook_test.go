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
		name                  string
		c                     *OCICluster
		errorMgsShouldContain string
		expectErr             bool
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
			errorMgsShouldContain: "invalid egressRules CIDR format",
			expectErr:             true,
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
			errorMgsShouldContain: "invalid ingressRule CIDR format",
			expectErr:             true,
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
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				_, err := (&OCIClusterWebhook{}).ValidateCreate(context.Background(), test.c)
				g.Expect(err).To(gomega.Succeed())
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
