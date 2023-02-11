/*
Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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

package v1beta1

import (
	"strings"
	"testing"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOCIManagedMachinePool_CreateDefault(t *testing.T) {
	tests := []struct {
		name   string
		m      *OCIManagedMachinePool
		expect func(g *gomega.WithT, c *OCIManagedMachinePool)
	}{
		{
			name: "should set default cni type",
			m:    &OCIManagedMachinePool{},
			expect: func(g *gomega.WithT, c *OCIManagedMachinePool) {
				g.Expect(c.Spec.NodePoolNodeConfig.NodePoolPodNetworkOptionDetails).To(Equal(&NodePoolPodNetworkOptionDetails{
					CniType: VCNNativeCNI,
					VcnIpNativePodNetworkOptions: VcnIpNativePodNetworkOptions{
						SubnetNames: []string{PodDefaultName},
						NSGNames:    []string{PodDefaultName},
					},
				}))
			},
		},
		{
			name: "should not override cni type",
			m: &OCIManagedMachinePool{
				Spec: OCIManagedMachinePoolSpec{
					NodePoolNodeConfig: &NodePoolNodeConfig{
						NodePoolPodNetworkOptionDetails: &NodePoolPodNetworkOptionDetails{
							CniType: FlannelCNI,
						},
					},
				},
			},
			expect: func(g *gomega.WithT, c *OCIManagedMachinePool) {
				g.Expect(c.Spec.NodePoolNodeConfig.NodePoolPodNetworkOptionDetails).To(Equal(&NodePoolPodNetworkOptionDetails{
					CniType: FlannelCNI,
				}))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			test.m.Default()
			test.expect(g, test.m)
		})
	}
}

func TestOCIManagedMachinePool_ValidateCreate(t *testing.T) {
	tests := []struct {
		name                  string
		m                     *OCIManagedMachinePool
		errorMgsShouldContain string
		expectErr             bool
	}{
		{
			name: "shouldn't allow more than 31 characters",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrst",
				},
			},
			errorMgsShouldContain: "Name cannot be more than 31 characters",
			expectErr:             true,
		},
		{
			name: "should allow less than 31 characters",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
			},
			expectErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			if test.expectErr {
				err := test.m.ValidateCreate()
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				g.Expect(test.m.ValidateCreate()).To(gomega.Succeed())
			}
		})
	}
}
