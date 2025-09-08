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

package v1beta2

import (
	"context"
	"strings"
	"testing"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
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
					CniType: infrastructurev1beta2.VCNNativeCNI,
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
							CniType: infrastructurev1beta2.FlannelCNI,
						},
					},
				},
			},
			expect: func(g *gomega.WithT, c *OCIManagedMachinePool) {
				g.Expect(c.Spec.NodePoolNodeConfig.NodePoolPodNetworkOptionDetails).To(Equal(&NodePoolPodNetworkOptionDetails{
					CniType: infrastructurev1beta2.FlannelCNI,
				}))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			(&OCIManagedMachinePoolWebhook{}).Default(context.Background(), test.m)
			test.expect(g, test.m)
		})
	}
}

func TestOCIManagedMachinePool_ValidateCreate(t *testing.T) {
	validVersion := common.String("v1.25.1")
	inValidVersion := common.String("abcd")
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
				Spec: OCIManagedMachinePoolSpec{
					Version: validVersion,
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
				Spec: OCIManagedMachinePoolSpec{
					Version: validVersion,
				},
			},
			expectErr: false,
		},
		{
			name: "should not allow nil version",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
			},
			expectErr: true,
		},
		{
			name: "should not allow invalid version",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: inValidVersion,
				},
			},
			expectErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			if test.expectErr {
				_, err := (&OCIManagedMachinePoolWebhook{}).ValidateCreate(context.Background(), test.m)
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				_, err := (&OCIManagedMachinePoolWebhook{}).ValidateCreate(context.Background(), test.m)
				g.Expect(err).To(gomega.Succeed())
			}
		})
	}
}

func TestOCIManagedMachinePool_ValidateUpdate(t *testing.T) {
	validVersion := common.String("v1.25.1")
	oldVersion := common.String("v1.24.1")
	inValidVersion := common.String("abcd")
	tests := []struct {
		name                  string
		m                     *OCIManagedMachinePool
		old                   *OCIManagedMachinePool
		errorMgsShouldContain string
		expectErr             bool
	}{
		{
			name: "should allow valid version",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcde",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: validVersion,
				},
			},
			old: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: validVersion,
				},
			},
			expectErr: false,
		},
		{
			name: "should not allow nil version",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
			},
			old: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
			},
			expectErr: true,
		},
		{
			name: "should not allow invalid version",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: inValidVersion,
				},
			},
			old: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
			},
			expectErr: true,
		},
		{
			name: "should allow version update with different images",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: validVersion,
					NodeSourceViaImage: &NodeSourceViaImage{
						ImageId: common.String("new"),
					},
				},
			},
			old: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: oldVersion,
					NodeSourceViaImage: &NodeSourceViaImage{
						ImageId: common.String("old"),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "should allow version update with both nil",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: validVersion,
				},
			},
			old: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: oldVersion,
				},
			},
			expectErr: false,
		},
		{
			name: "should allow version update with same image",
			m: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: validVersion,
					NodeSourceViaImage: &NodeSourceViaImage{
						ImageId: common.String("old"),
					},
				},
			},
			old: &OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
				Spec: OCIManagedMachinePoolSpec{
					Version: oldVersion,
					NodeSourceViaImage: &NodeSourceViaImage{
						ImageId: common.String("old"),
					},
				},
			},
			expectErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			_, err := (&OCIManagedMachinePoolWebhook{}).ValidateUpdate(context.Background(), test.old, test.m)
			if test.expectErr {
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				g.Expect(err).To(gomega.Succeed())
			}
		})
	}
}
