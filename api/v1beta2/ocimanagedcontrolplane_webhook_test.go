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

	"github.com/oracle/oci-go-sdk/v65/common"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOCIManagedControlPlane_CreateDefault(t *testing.T) {
	tests := []struct {
		name   string
		c      *OCIManagedControlPlane
		expect func(g *gomega.WithT, c *OCIManagedControlPlane)
	}{
		{
			name: "should set default cni type",
			c:    &OCIManagedControlPlane{},
			expect: func(g *gomega.WithT, c *OCIManagedControlPlane) {
				g.Expect(c.Spec.ClusterPodNetworkOptions).To(Equal([]ClusterPodNetworkOptions{
					{
						CniType: VCNNativeCNI,
					},
				}))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			(&OCIManagedControlPlaneWebhook{}).Default(context.Background(), test.c)
			test.expect(g, test.c)
		})
	}
}

func TestOCIManagedControlPlane_ValidateCreate(t *testing.T) {
	tests := []struct {
		name                  string
		c                     *OCIManagedControlPlane
		errorMgsShouldContain string
		expectErr             bool
	}{
		{
			name: "shouldn't allow more than 31 characters",
			c: &OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrst",
				},
			},
			errorMgsShouldContain: "Name cannot be more than 31 characters",
			expectErr:             true,
		},
		{
			name: "should allow less than 31 characters",
			c: &OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abcdefghijklmno",
				},
			},
			expectErr: false,
		},
		{
			name: "OpenIdConnectAuthEnabledWithValidConfig",
			c: &OCIManagedControlPlane{
				Spec: OCIManagedControlPlaneSpec{
					ClusterType: EnhancedClusterType,
					ClusterOption: ClusterOptions{
						OpenIdConnectTokenAuthenticationConfig: &OpenIDConnectTokenAuthenticationConfig{
							IsOpenIdConnectAuthEnabled: *common.Bool(true),
							ClientId:                   common.String("client-id"),
							IssuerUrl:                  common.String("issuer-url"),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "OpenIdConnectAuthEnabledWithInvalidClusterType",
			c: &OCIManagedControlPlane{
				Spec: OCIManagedControlPlaneSpec{
					ClusterType: BasicClusterType,
					ClusterOption: ClusterOptions{
						OpenIdConnectTokenAuthenticationConfig: &OpenIDConnectTokenAuthenticationConfig{
							IsOpenIdConnectAuthEnabled: *common.Bool(true),
							ClientId:                   common.String("client-id"),
							IssuerUrl:                  common.String("issuer-url"),
						},
					},
				},
			},
			errorMgsShouldContain: "ClusterType needs to be set to ENHANCED_CLUSTER for OpenIdConnectTokenAuthenticationConfig to be enabled.",
			expectErr:             true,
		},
		{
			name: "OpenIdConnectAuthEnabledWithMissingClientId",
			c: &OCIManagedControlPlane{
				Spec: OCIManagedControlPlaneSpec{
					ClusterType: EnhancedClusterType,
					ClusterOption: ClusterOptions{
						OpenIdConnectTokenAuthenticationConfig: &OpenIDConnectTokenAuthenticationConfig{
							IsOpenIdConnectAuthEnabled: *common.Bool(true),
							IssuerUrl:                  common.String("issuer-url"),
						},
					},
				},
			},
			errorMgsShouldContain: "ClientId cannot be empty when OpenIdConnectAuth is enabled.",
			expectErr:             true,
		},
		{
			name: "OpenIdConnectAuthEnabledWithMissingIssuerUrl",
			c: &OCIManagedControlPlane{
				Spec: OCIManagedControlPlaneSpec{
					ClusterType: EnhancedClusterType,
					ClusterOption: ClusterOptions{
						OpenIdConnectTokenAuthenticationConfig: &OpenIDConnectTokenAuthenticationConfig{
							IsOpenIdConnectAuthEnabled: *common.Bool(true),
							ClientId:                   common.String("client-id"),
						},
					},
				},
			},
			errorMgsShouldContain: "IssuerUrl cannot be empty when OpenIdConnectAuth is enabled.",
			expectErr:             true,
		},
		{
			name: "OpenIdConnectAuthDisabled",
			c: &OCIManagedControlPlane{
				Spec: OCIManagedControlPlaneSpec{
					ClusterType: BasicClusterType,
					ClusterOption: ClusterOptions{
						OpenIdConnectTokenAuthenticationConfig: &OpenIDConnectTokenAuthenticationConfig{
							IsOpenIdConnectAuthEnabled: *common.Bool(false),
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
				_, err := (&OCIManagedControlPlaneWebhook{}).ValidateCreate(context.Background(), test.c)
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorMgsShouldContain)).To(gomega.BeTrue())
			} else {
				_, err := (&OCIManagedControlPlaneWebhook{}).ValidateCreate(context.Background(), test.c)
				g.Expect(err).To(gomega.Succeed())
			}
		})
	}
}
