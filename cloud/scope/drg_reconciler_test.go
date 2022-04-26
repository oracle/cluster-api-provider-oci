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
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDRGReconciliation(t *testing.T) {
	var (
		cs         *ClusterScope
		mockCtrl   *gomock.Controller
		vcnClient  *mock_vcn.MockClient
		ociCluster infrastructurev1beta1.OCICluster
		tags       map[string]string
		vcnPeering infrastructurev1beta1.VCNPeering
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		vcnClient = mock_vcn.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociCluster = infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID:  "cluster_uid",
				Name: "cluster",
			},
			Spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId:         "compartment-id",
				OCIResourceIdentifier: "resource_uid",
			},
		}
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			VCNClient:  vcnClient,
			Cluster:    &clusterv1.Cluster{},
			OCICluster: &ociCluster,
			Client:     client,
		})
		tags = make(map[string]string)
		tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
		tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
		vcnPeering = infrastructurev1beta1.VCNPeering{}
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name                string
		errorExpected       bool
		objects             []client.Object
		expectedEvent       string
		eventNotExpected    string
		matchError          error
		errorSubStringMatch bool
		testSpecificSetup   func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient)
	}{
		{
			name:          "drg disabled",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
			},
		},
		{
			name:          "vcn peering, but drg disabled",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
			},
		},
		{
			name:          "drg is unmanaged",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = false
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
			},
		},
		{
			name:                "get drg call failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrg(gomock.Any(), gomock.Eq(core.GetDrgRequest{
					DrgId: common.String("drg-id"),
				})).
					Return(core.GetDrgResponse{}, errors.New("request failed"))
			},
		},
		{
			name:                "get drg call failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrg(gomock.Any(), gomock.Eq(core.GetDrgRequest{
					DrgId: common.String("drg-id"),
				})).
					Return(core.GetDrgResponse{}, errors.New("request failed"))
			},
		},
		{
			name:          "get drg call success",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrg(gomock.Any(), gomock.Eq(core.GetDrgRequest{
					DrgId: common.String("drg-id"),
				})).
					Return(core.GetDrgResponse{
						Drg: core.Drg{
							Id:           common.String("drg-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
			},
		},
		{
			name:          "drg update",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				existingTags := make(map[string]string)
				existingTags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
				existingTags[ociutil.ClusterResourceIdentifier] = "resource_uid"
				existingTags["test"] = "test"
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrg(gomock.Any(), gomock.Eq(core.GetDrgRequest{
					DrgId: common.String("drg-id"),
				})).
					Return(core.GetDrgResponse{
						Drg: core.Drg{
							Id:           common.String("drg-id"),
							FreeformTags: existingTags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
				vcnClient.EXPECT().UpdateDrg(gomock.Any(), gomock.Eq(core.UpdateDrgRequest{
					DrgId: common.String("drg-id"),
					UpdateDrgDetails: core.UpdateDrgDetails{
						FreeformTags: tags,
						DefinedTags:  make(map[string]map[string]interface{}),
					},
				})).
					Return(core.UpdateDrgResponse{}, nil)
			},
		},
		{
			name:          "drg create",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = true
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().ListDrgs(gomock.Any(), gomock.Eq(core.ListDrgsRequest{
					CompartmentId: common.String("compartment-id"),
				})).
					Return(core.ListDrgsResponse{}, nil)
				vcnClient.EXPECT().CreateDrg(gomock.Any(), gomock.Eq(core.CreateDrgRequest{
					CreateDrgDetails: core.CreateDrgDetails{
						CompartmentId: common.String("compartment-id"),
						FreeformTags:  tags,
						DefinedTags:   make(map[string]map[string]interface{}),
						DisplayName:   common.String("cluster"),
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-drg", "resource_uid"),
				})).
					Return(core.CreateDrgResponse{
						Drg: core.Drg{
							Id: common.String("drg-id"),
						},
					}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, vcnClient)
			err := cs.ReconcileDRG(context.Background())
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
				if tc.errorSubStringMatch {
					g.Expect(err.Error()).To(ContainSubstring(tc.matchError.Error()))
				} else {
					g.Expect(err.Error()).To(Equal(tc.matchError.Error()))
				}
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestDRGDeletion(t *testing.T) {
	var (
		cs         *ClusterScope
		mockCtrl   *gomock.Controller
		vcnClient  *mock_vcn.MockClient
		ociCluster infrastructurev1beta1.OCICluster
		tags       map[string]string
		vcnPeering infrastructurev1beta1.VCNPeering
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		vcnClient = mock_vcn.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociCluster = infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID:  "cluster_uid",
				Name: "cluster",
			},
			Spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId:         "compartment-id",
				OCIResourceIdentifier: "resource_uid",
			},
		}
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			VCNClient:  vcnClient,
			Cluster:    &clusterv1.Cluster{},
			OCICluster: &ociCluster,
			Client:     client,
		})
		tags = make(map[string]string)
		tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
		tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
		vcnPeering = infrastructurev1beta1.VCNPeering{}
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name                string
		errorExpected       bool
		objects             []client.Object
		expectedEvent       string
		eventNotExpected    string
		matchError          error
		errorSubStringMatch bool
		testSpecificSetup   func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient)
	}{
		{
			name:          "drg disabled",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
			},
		},
		{
			name:          "vcn peering, but drg disabled",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
			},
		},
		{
			name:          "drg is unmanaged",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = false
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
			},
		},
		{
			name:                "get drg call failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrg(gomock.Any(), gomock.Eq(core.GetDrgRequest{
					DrgId: common.String("drg-id"),
				})).
					Return(core.GetDrgResponse{}, errors.New("request failed"))
			},
		},
		{
			name:          "drg not found",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrg(gomock.Any(), gomock.Eq(core.GetDrgRequest{
					DrgId: common.String("drg-id"),
				})).
					Return(core.GetDrgResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:          "delete success",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta1.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				clusterScope.OCICluster.Spec.NetworkSpec.VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrg(gomock.Any(), gomock.Eq(core.GetDrgRequest{
					DrgId: common.String("drg-id"),
				})).
					Return(core.GetDrgResponse{
						Drg: core.Drg{
							Id:           common.String("drg-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
				vcnClient.EXPECT().DeleteDrg(gomock.Any(), gomock.Eq(core.DeleteDrgRequest{
					DrgId: common.String("drg-id"),
				})).
					Return(core.DeleteDrgResponse{}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, vcnClient)
			err := cs.DeleteDRG(context.Background())
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
				if tc.errorSubStringMatch {
					g.Expect(err.Error()).To(ContainSubstring(tc.matchError.Error()))
				} else {
					g.Expect(err.Error()).To(Equal(tc.matchError.Error()))
				}
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}
