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
	"testing"

	"github.com/golang/mock/gomock"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestClusterScope_ReconcileNatGateway(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "a"

	definedTags := map[string]map[string]string{
		"ns1": {
			"tag1": "foo",
			"tag2": "bar",
		},
		"ns2": {
			"tag1": "foo1",
			"tag2": "bar1",
		},
	}

	definedTagsInterface := make(map[string]map[string]interface{})
	for ns, mapNs := range definedTags {
		mapValues := make(map[string]interface{})
		for k, v := range mapNs {
			mapValues[k] = v
		}
		definedTagsInterface[ns] = mapValues
	}

	vcnClient.EXPECT().GetNatGateway(gomock.Any(), gomock.Eq(core.GetNatGatewayRequest{
		NatGatewayId: common.String("foo"),
	})).
		Return(core.GetNatGatewayResponse{
			NatGateway: core.NatGateway{
				Id:           common.String("foo"),
				FreeformTags: tags,
				DefinedTags:  definedTagsInterface,
			},
		}, nil).Times(2)

	updatedTags := make(map[string]string)
	for k, v := range tags {
		updatedTags[k] = v
	}
	updatedTags["foo"] = "bar"
	vcnClient.EXPECT().ListNatGateways(gomock.Any(), gomock.Eq(core.ListNatGatewaysRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("nat-gateway"),
		VcnId:         common.String("vcn"),
	})).Return(
		core.ListNatGatewaysResponse{
			Items: []core.NatGateway{
				{
					Id:           common.String("ngw_id"),
					FreeformTags: tags,
				},
			}}, nil)

	vcnClient.EXPECT().ListNatGateways(gomock.Any(), gomock.Eq(core.ListNatGatewaysRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("nat-gateway"),
		VcnId:         common.String("vcn1"),
	})).Return(
		core.ListNatGatewaysResponse{
			Items: []core.NatGateway{
				{
					Id: common.String("ngw_id"),
				},
			}}, nil).Times(2)

	vcnClient.EXPECT().CreateNatGateway(gomock.Any(), gomock.Eq(core.CreateNatGatewayRequest{
		CreateNatGatewayDetails: core.CreateNatGatewayDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("nat-gateway"),
			VcnId:         common.String("vcn1"),
			FreeformTags:  updatedTags,
			DefinedTags:   definedTagsInterface,
		},
	})).Return(
		core.CreateNatGatewayResponse{
			NatGateway: core.NatGateway{
				Id: common.String("ngw"),
			},
		}, nil)

	vcnClient.EXPECT().CreateNatGateway(gomock.Any(), gomock.Eq(core.CreateNatGatewayRequest{
		CreateNatGatewayDetails: core.CreateNatGatewayDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("nat-gateway"),
			VcnId:         common.String("vcn1"),
			FreeformTags:  tags,
			DefinedTags:   definedTagsInterface,
		},
	})).Return(
		core.CreateNatGatewayResponse{}, errors.New("some error"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "all subnets are public",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Public,
								Role: infrastructurev1beta2.ControlPlaneRole,
							},
							{
								Type: infrastructurev1beta2.Public,
								Role: infrastructurev1beta2.WorkerRole,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no update needed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				DefinedTags: definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("foo"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no update needed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				DefinedTags: definedTags,
				FreeformTags: map[string]string{
					"foo": "bar",
				},
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("foo"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "id not present in spec but found by name and no update needed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				FreeformTags: map[string]string{
					"foo": "bar",
				},
				DefinedTags: definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "creation needed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				FreeformTags: map[string]string{
					"foo": "bar",
				},
				DefinedTags: definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn1"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "creation failed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn1"),
					},
				},
			},
			wantErr:       true,
			expectedError: "failed create nat gateway: some error",
		},
		{
			name: "creation skip",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn1"),
						NATGateway: infrastructurev1beta2.NATGateway{
							Skip: true,
						},
					},
				},
			},
			wantErr: false,
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociClusterAcccessor := OCISelfManagedCluster{
				&infrastructurev1beta2.OCICluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cluster_uid",
					},
					Spec: tt.spec,
				},
			}
			ociClusterAcccessor.OCICluster.Spec.OCIResourceIdentifier = "a"
			s := &ClusterScope{
				VCNClient:          vcnClient,
				OCIClusterAccessor: ociClusterAcccessor,
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cluster_uid",
					},
				},
				Logger: &l,
			}
			err := s.ReconcileNatGateway(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileNatGateway() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("ReconcileNatGateway() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestClusterScope_DeleteNatGateway(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	vcnClient.EXPECT().GetNatGateway(gomock.Any(), gomock.Eq(core.GetNatGatewayRequest{
		NatGatewayId: common.String("normal_id"),
	})).
		Return(core.GetNatGatewayResponse{
			NatGateway: core.NatGateway{
				Id:           common.String("normal_id"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetNatGateway(gomock.Any(), gomock.Eq(core.GetNatGatewayRequest{
		NatGatewayId: common.String("error_delete_ngw"),
	})).
		Return(core.GetNatGatewayResponse{
			NatGateway: core.NatGateway{
				Id:           common.String("error_delete_ngw"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetNatGateway(gomock.Any(), gomock.Eq(core.GetNatGatewayRequest{
		NatGatewayId: common.String("error")})).
		Return(core.GetNatGatewayResponse{}, errors.New("some error in GetNatGateway"))

	vcnClient.EXPECT().GetNatGateway(gomock.Any(), gomock.Eq(core.GetNatGatewayRequest{NatGatewayId: common.String("ngw_deleted")})).
		Return(core.GetNatGatewayResponse{}, errors.New("not found"))
	vcnClient.EXPECT().DeleteNatGateway(gomock.Any(), gomock.Eq(core.DeleteNatGatewayRequest{
		NatGatewayId: common.String("normal_id"),
	})).
		Return(core.DeleteNatGatewayResponse{}, nil)
	vcnClient.EXPECT().DeleteNatGateway(gomock.Any(), gomock.Eq(core.DeleteNatGatewayRequest{
		NatGatewayId: common.String("error_delete_ngw"),
	})).
		Return(core.DeleteNatGatewayResponse{}, errors.New("some error in DeleteNatGateway"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete nat gateway is successful",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("normal_id"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nat gateway already deleted",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("ngw_deleted"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete nat gateway error when calling get nat gateway",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("error"),
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "some error in GetNatGateway",
		},
		{
			name: "delete nat gateway error when calling delete nat gateway",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						NATGateway: infrastructurev1beta2.NATGateway{
							Id: common.String("error_delete_ngw"),
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to delete NatGateway: some error in DeleteNatGateway",
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociClusterAccessor := OCISelfManagedCluster{
				&infrastructurev1beta2.OCICluster{
					Spec: tt.spec,
					ObjectMeta: metav1.ObjectMeta{
						UID: "cluster_uid",
					},
				},
			}
			ociClusterAccessor.OCICluster.Spec.OCIResourceIdentifier = "resource_uid"
			s := &ClusterScope{
				VCNClient:          vcnClient,
				OCIClusterAccessor: ociClusterAccessor,
				Logger:             &l,
			}
			err := s.DeleteNatGateway(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteNatGateway() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("DeleteNatGateway() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}
}
