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
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDRGVCNAttachmentReconciliation(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		vcnClient          *mock_vcn.MockClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
		vcnPeering         infrastructurev1beta2.VCNPeering
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		vcnClient = mock_vcn.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociClusterAccessor = OCISelfManagedCluster{
			&infrastructurev1beta2.OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					UID:  "cluster_uid",
					Name: "cluster",
				},
				Spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId:         "compartment-id",
					OCIResourceIdentifier: "resource_uid",
				},
			},
		}
		ociClusterAccessor.OCICluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			VCNClient:          vcnClient,
			Cluster:            &clusterv1.Cluster{},
			OCIClusterAccessor: ociClusterAccessor,
			Client:             client,
		})
		tags = make(map[string]string)
		tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
		tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
		vcnPeering = infrastructurev1beta2.VCNPeering{}
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
			name:          "peering disabled",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
			},
		},
		{
			name:                "get drg vcn attachment call failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta2.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.DRG.VcnAttachmentId = common.String("attachment-id")
				clusterScope.OCIClusterAccessor.GetNetworkSpec().VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrgAttachment(gomock.Any(), gomock.Eq(core.GetDrgAttachmentRequest{
					DrgAttachmentId: common.String("attachment-id"),
				})).
					Return(core.GetDrgAttachmentResponse{}, errors.New("request failed"))
			},
		},
		{
			name:          "get drg attachment call success",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta2.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.DRG.VcnAttachmentId = common.String("attachment-id")
				clusterScope.OCIClusterAccessor.GetNetworkSpec().VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrgAttachment(gomock.Any(), gomock.Eq(core.GetDrgAttachmentRequest{
					DrgAttachmentId: common.String("attachment-id"),
				})).
					Return(core.GetDrgAttachmentResponse{
						DrgAttachment: core.DrgAttachment{
							Id:           common.String("attachment-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
			},
		},
		{
			name:          "drg attachment create",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta2.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				clusterScope.OCIClusterAccessor.GetNetworkSpec().VCNPeering = &vcnPeering
				clusterScope.OCIClusterAccessor.GetNetworkSpec().Vcn.ID = common.String("vcn-id")
				vcnClient.EXPECT().ListDrgAttachments(gomock.Any(), gomock.Eq(core.ListDrgAttachmentsRequest{
					AttachmentType: core.ListDrgAttachmentsAttachmentTypeVcn,
					DrgId:          common.String("drg-id"),
					NetworkId:      common.String("vcn-id"),
					CompartmentId:  common.String("compartment-id"),
					DisplayName:    common.String("cluster"),
				})).
					Return(core.ListDrgAttachmentsResponse{}, nil)
				vcnClient.EXPECT().CreateDrgAttachment(gomock.Any(), gomock.Eq(core.CreateDrgAttachmentRequest{
					CreateDrgAttachmentDetails: core.CreateDrgAttachmentDetails{
						DisplayName:  common.String("cluster"),
						DrgId:        common.String("drg-id"),
						VcnId:        common.String("vcn-id"),
						FreeformTags: tags,
						DefinedTags:  make(map[string]map[string]interface{}),
					},
				})).
					Return(core.CreateDrgAttachmentResponse{
						DrgAttachment: core.DrgAttachment{
							Id: common.String("attachment-id"),
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
			err := cs.ReconcileDRGVCNAttachment(context.Background())
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

func TestDRGVcnAttachmentDeletion(t *testing.T) {
	var (
		cs                 *ClusterScope
		mockCtrl           *gomock.Controller
		vcnClient          *mock_vcn.MockClient
		ociClusterAccessor OCISelfManagedCluster
		tags               map[string]string
		vcnPeering         infrastructurev1beta2.VCNPeering
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		vcnClient = mock_vcn.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociClusterAccessor = OCISelfManagedCluster{
			&infrastructurev1beta2.OCICluster{
				ObjectMeta: metav1.ObjectMeta{
					UID:  "cluster_uid",
					Name: "cluster",
				},
				Spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId:         "compartment-id",
					OCIResourceIdentifier: "resource_uid",
				},
			},
		}
		ociClusterAccessor.OCICluster.Spec.ControlPlaneEndpoint.Port = 6443
		cs, err = NewClusterScope(ClusterScopeParams{
			VCNClient:          vcnClient,
			Cluster:            &clusterv1.Cluster{},
			OCIClusterAccessor: ociClusterAccessor,
			Client:             client,
		})
		tags = make(map[string]string)
		tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
		tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
		vcnPeering = infrastructurev1beta2.VCNPeering{}
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
			name:          "vcn peering, but drg disabled",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
			},
		},
		{
			name:                "get drg attachment call failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("request failed"),
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta2.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.DRG.VcnAttachmentId = common.String("attachment-id")
				clusterScope.OCIClusterAccessor.GetNetworkSpec().VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrgAttachment(gomock.Any(), gomock.Eq(core.GetDrgAttachmentRequest{
					DrgAttachmentId: common.String("attachment-id"),
				})).
					Return(core.GetDrgAttachmentResponse{}, errors.New("request failed"))
			},
		},
		{
			name:          "drg attachment not found",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta2.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.DRG.VcnAttachmentId = common.String("attachment-id")
				clusterScope.OCIClusterAccessor.GetNetworkSpec().VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrgAttachment(gomock.Any(), gomock.Eq(core.GetDrgAttachmentRequest{
					DrgAttachmentId: common.String("attachment-id"),
				})).
					Return(core.GetDrgAttachmentResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:          "delete success",
			errorExpected: false,
			testSpecificSetup: func(clusterScope *ClusterScope, vcnClient *mock_vcn.MockClient) {
				vcnPeering.DRG = &infrastructurev1beta2.DRG{}
				vcnPeering.DRG.Manage = true
				vcnPeering.DRG.ID = common.String("drg-id")
				vcnPeering.DRG.VcnAttachmentId = common.String("attachment-id")
				clusterScope.OCIClusterAccessor.GetNetworkSpec().VCNPeering = &vcnPeering
				vcnClient.EXPECT().GetDrgAttachment(gomock.Any(), gomock.Eq(core.GetDrgAttachmentRequest{
					DrgAttachmentId: common.String("attachment-id"),
				})).
					Return(core.GetDrgAttachmentResponse{
						DrgAttachment: core.DrgAttachment{
							Id:           common.String("attachment-id"),
							FreeformTags: tags,
							DefinedTags:  make(map[string]map[string]interface{}),
						},
					}, nil)
				vcnClient.EXPECT().DeleteDrgAttachment(gomock.Any(), gomock.Eq(core.DeleteDrgAttachmentRequest{
					DrgAttachmentId: common.String("attachment-id"),
				})).
					Return(core.DeleteDrgAttachmentResponse{}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, vcnClient)
			err := cs.DeleteDRGVCNAttachment(context.Background())
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
