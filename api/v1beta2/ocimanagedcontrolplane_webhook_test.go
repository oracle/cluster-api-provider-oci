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
	"reflect"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
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

func TestOCIManagedControlPlaneWebhook_ValidateDelete(t *testing.T) {
	h := &OCIManagedControlPlaneWebhook{}

	tests := []struct {
		name    string
		obj     runtime.Object
		wantErr bool
	}{
		{
			name:    "nil object",
			obj:     nil,
			wantErr: false,
		},
		{
			name:    "some OCIManagedControlPlane object",
			obj:     &OCIManagedControlPlane{}, // assuming you have this type
			wantErr: false,
		},
		{
			name:    "other type",
			obj:     &OCICluster{}, // passing a different type should still return nil
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWarnings, err := h.ValidateDelete(context.TODO(), tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotWarnings != nil && len(gotWarnings) != 0 {
				t.Errorf("ValidateDelete() warnings = %v, want nil", gotWarnings)
			}
		})
	}
}

func TestOCIManagedControlPlane_GetConditions(t *testing.T) {
	want := clusterv1.Conditions{
		{Type: "Ready", Status: corev1.ConditionTrue},
	}
	c := &OCIManagedControlPlane{
		Status: OCIManagedControlPlaneStatus{
			Conditions: want,
		},
	}

	got := c.GetConditions()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetConditions() = %v, want %v", got, want)
	}
}

func TestOCIManagedControlPlane_SetConditions(t *testing.T) {
	cond := clusterv1.Conditions{
		{Type: "Updated", Status: corev1.ConditionFalse},
	}
	c := &OCIManagedControlPlane{}

	c.SetConditions(cond)
	if !reflect.DeepEqual(c.Status.Conditions, cond) {
		t.Errorf("SetConditions() failed, got %v, want %v", c.Status.Conditions, cond)
	}
}

func TestOCIManagedControlPlane_SetAddonStatus(t *testing.T) {
	version := "v1.0.0"
	state := "Installed"
	message := "Addon installed successfully"

	status := AddonStatus{
		CurrentlyInstalledVersion: &version,
		LifecycleState:            &state,
		AddonError: &AddonError{
			Message: &message,
		},
	}

	c := &OCIManagedControlPlane{}

	// Add first addon
	c.SetAddonStatus("metrics-server", status)
	got := c.Status.AddonStatus["metrics-server"]

	if got.CurrentlyInstalledVersion == nil || *got.CurrentlyInstalledVersion != version {
		t.Errorf("SetAddonStatus() CurrentlyInstalledVersion = %v, want %v", got.CurrentlyInstalledVersion, version)
	}
	if got.LifecycleState == nil || *got.LifecycleState != state {
		t.Errorf("SetAddonStatus() LifecycleState = %v, want %v", got.LifecycleState, state)
	}
	if got.AddonError == nil || got.AddonError.Message == nil || *got.AddonError.Message != message {
		t.Errorf("SetAddonStatus() AddonError.Message = %v, want %v", got.AddonError.Message, message)
	}

	// Add second addon to test map initialization and multiple entries
	secondVersion := "v2.0.0"
	secondStatus := AddonStatus{
		CurrentlyInstalledVersion: &secondVersion,
	}
	c.SetAddonStatus("logging", secondStatus)

	if len(c.Status.AddonStatus) != 2 {
		t.Errorf("Expected 2 addon statuses, got %d", len(c.Status.AddonStatus))
	}
	got2 := c.Status.AddonStatus["logging"]
	if got2.CurrentlyInstalledVersion == nil || *got2.CurrentlyInstalledVersion != secondVersion {
		t.Errorf("SetAddonStatus() second addon CurrentlyInstalledVersion = %v, want %v", got2.CurrentlyInstalledVersion, secondVersion)
	}
}
