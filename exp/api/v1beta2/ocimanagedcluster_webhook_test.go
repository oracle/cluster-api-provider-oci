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
	"strings"
	"testing"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOCIManagedCluster_ValidateCreate(t *testing.T) {
	goodSubnets := []*infrastructurev1beta2.Subnet{
		&infrastructurev1beta2.Subnet{
			Role: infrastructurev1beta2.ControlPlaneEndpointRole,
			Name: "test-subnet",
			CIDR: "10.0.0.0/16",
		},
	}
	badSubnetCidr := []*infrastructurev1beta2.Subnet{
		&infrastructurev1beta2.Subnet{
			Name: "test-subnet",
			CIDR: "10.1.0.0/16",
		},
	}
	emptySubnetCidr := []*infrastructurev1beta2.Subnet{
		&infrastructurev1beta2.Subnet{
			Role: infrastructurev1beta2.ControlPlaneEndpointRole,
			Name: "test-subnet",
		},
	}
	badSubnetCidrFormat := []*infrastructurev1beta2.Subnet{
		&infrastructurev1beta2.Subnet{
			Name: "test-subnet",
			CIDR: "no-a-cidr",
		},
	}
	dupSubnetNames := []*infrastructurev1beta2.Subnet{
		&infrastructurev1beta2.Subnet{
			Name: "dup-name",
			CIDR: "10.0.0.0/16",
		},
		&infrastructurev1beta2.Subnet{
			Name: "dup-name",
			CIDR: "10.0.0.0/16",
		},
	}
	emptySubnetName := []*infrastructurev1beta2.Subnet{
		&infrastructurev1beta2.Subnet{
			Name: "",
			CIDR: "10.0.0.0/16",
			Role: infrastructurev1beta2.ControlPlaneEndpointRole,
		},
	}
	badSubnetRole := []*infrastructurev1beta2.Subnet{
		&infrastructurev1beta2.Subnet{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							CIDR: "10.0.0.0/16",
							Subnets: []*infrastructurev1beta2.Subnet{
								&infrastructurev1beta2.Subnet{
									Role: infrastructurev1beta2.ControlPlaneRole,
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
			name: "should allow empty subnet name",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: goodClusterName,
				},
				Spec: OCIManagedClusterSpec{
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
								List: []*infrastructurev1beta2.NSG{{
									EgressRules: []infrastructurev1beta2.EgressSecurityRuleForNSG{{
										EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
											Destination:     common.String("bad/15"),
											DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
								List: []*infrastructurev1beta2.NSG{{
									IngressRules: []infrastructurev1beta2.IngressSecurityRuleForNSG{{
										IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
											Source:     common.String("bad/15"),
											SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
								List: []*infrastructurev1beta2.NSG{{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
								List: []*infrastructurev1beta2.NSG{{
									Role: infrastructurev1beta2.ControlPlaneRole,
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
			name: "should allow blank region",
			c: &OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-name",
				},
				Spec: OCIManagedClusterSpec{
					Region:                "",
					CompartmentId:         "ocid",
					OCIResourceIdentifier: "uuid",
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						APIServerLB: infrastructurev1beta2.LoadBalancer{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
				err := test.c.ValidateCreate()
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				g.Expect(test.c.ValidateCreate()).To(gomega.Succeed())
			}
		})
	}
}

func TestOCIManagedCluster_ValidateUpdate(t *testing.T) {
	goodSubnets := []*infrastructurev1beta2.Subnet{
		&infrastructurev1beta2.Subnet{
			Role: infrastructurev1beta2.ControlPlaneEndpointRole,
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
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
				err := test.c.ValidateUpdate(test.old)
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				g.Expect(test.c.ValidateUpdate(test.old)).To(gomega.Succeed())
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
				subnets := make([]*infrastructurev1beta2.Subnet, 0)
				subnets = append(subnets, &infrastructurev1beta2.Subnet{
					Role: infrastructurev1beta2.ControlPlaneEndpointRole,
					Name: infrastructurev1beta2.ControlPlaneEndpointDefaultName,
					CIDR: infrastructurev1beta2.ControlPlaneEndpointSubnetDefaultCIDR,
					Type: infrastructurev1beta2.Public,
				})

				subnets = append(subnets, &infrastructurev1beta2.Subnet{
					Role: infrastructurev1beta2.ServiceLoadBalancerRole,
					Name: infrastructurev1beta2.ServiceLBDefaultName,
					CIDR: infrastructurev1beta2.ServiceLoadBalancerDefaultCIDR,
					Type: infrastructurev1beta2.Public,
				})
				subnets = append(subnets, &infrastructurev1beta2.Subnet{
					Role: infrastructurev1beta2.WorkerRole,
					Name: infrastructurev1beta2.WorkerDefaultName,
					CIDR: infrastructurev1beta2.WorkerSubnetDefaultCIDR,
					Type: infrastructurev1beta2.Private,
				})
				subnets = append(subnets, &infrastructurev1beta2.Subnet{
					Role: infrastructurev1beta2.PodRole,
					Name: PodDefaultName,
					CIDR: PodDefaultCIDR,
					Type: infrastructurev1beta2.Private,
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
				nsgs := make([]*infrastructurev1beta2.NSG, 4)
				nsgs[0] = &infrastructurev1beta2.NSG{
					Role:         infrastructurev1beta2.ControlPlaneEndpointRole,
					Name:         infrastructurev1beta2.ControlPlaneEndpointDefaultName,
					IngressRules: c.GetControlPlaneEndpointDefaultIngressRules(),
					EgressRules:  c.GetControlPlaneEndpointDefaultEgressRules(),
				}
				nsgs[1] = &infrastructurev1beta2.NSG{
					Role:         infrastructurev1beta2.WorkerRole,
					Name:         infrastructurev1beta2.WorkerDefaultName,
					IngressRules: c.GetWorkerDefaultIngressRules(),
					EgressRules:  c.GetWorkerDefaultEgressRules(),
				}
				nsgs[2] = &infrastructurev1beta2.NSG{
					Role:         infrastructurev1beta2.ServiceLoadBalancerRole,
					Name:         infrastructurev1beta2.ServiceLBDefaultName,
					IngressRules: c.GetLBServiceDefaultIngressRules(),
					EgressRules:  c.GetLBServiceDefaultEgressRules(),
				}
				nsgs[3] = &infrastructurev1beta2.NSG{
					Role:         infrastructurev1beta2.PodRole,
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
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
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
			test.c.Default()
			test.expect(g, test.c)
		})
	}
}
