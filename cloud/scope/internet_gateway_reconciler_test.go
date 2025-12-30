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

func TestClusterScope_ReconcileInternetGateway(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

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

	vcnClient.EXPECT().GetInternetGateway(gomock.Any(), gomock.Eq(core.GetInternetGatewayRequest{
		IgId: common.String("foo"),
	})).
		Return(core.GetInternetGatewayResponse{
			InternetGateway: core.InternetGateway{
				Id:           common.String("foo"),
				FreeformTags: tags,
				DefinedTags:  definedTagsInterface,
			},
		}, nil)

	updatedTags := make(map[string]string)
	for k, v := range tags {
		updatedTags[k] = v
	}
	updatedTags["foo"] = "bar"
	vcnClient.EXPECT().ListInternetGateways(gomock.Any(), gomock.Eq(core.ListInternetGatewaysRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("internet-gateway"),
		VcnId:         common.String("vcn"),
	})).Return(
		core.ListInternetGatewaysResponse{
			Items: []core.InternetGateway{
				{
					Id:           common.String("igw_id"),
					FreeformTags: tags,
				},
			}}, nil)

	vcnClient.EXPECT().ListInternetGateways(gomock.Any(), gomock.Eq(core.ListInternetGatewaysRequest{
		CompartmentId: common.String("foo"),
		DisplayName:   common.String("internet-gateway"),
		VcnId:         common.String("vcn1"),
	})).Return(
		core.ListInternetGatewaysResponse{
			Items: []core.InternetGateway{
				{
					Id: common.String("igw_id"),
				},
			}}, nil).Times(2)

	vcnClient.EXPECT().CreateInternetGateway(gomock.Any(), gomock.Eq(core.CreateInternetGatewayRequest{
		CreateInternetGatewayDetails: core.CreateInternetGatewayDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("internet-gateway"),
			IsEnabled:     common.Bool(true),
			VcnId:         common.String("vcn1"),
			FreeformTags:  updatedTags,
			DefinedTags:   definedTagsInterface,
		},
	})).Return(
		core.CreateInternetGatewayResponse{
			InternetGateway: core.InternetGateway{
				Id: common.String("igw"),
			},
		}, nil)

	vcnClient.EXPECT().CreateInternetGateway(gomock.Any(), gomock.Eq(core.CreateInternetGatewayRequest{
		CreateInternetGatewayDetails: core.CreateInternetGatewayDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("internet-gateway"),
			IsEnabled:     common.Bool(true),
			VcnId:         common.String("vcn1"),
			FreeformTags:  tags,
			DefinedTags:   definedTagsInterface,
		},
	})).Return(
		core.CreateInternetGatewayResponse{}, errors.New("some error"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "all subnets are private",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							},
							{
								Type: infrastructurev1beta2.Private,
								Role: infrastructurev1beta2.ServiceLoadBalancerRole,
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
						InternetGateway: infrastructurev1beta2.InternetGateway{
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
			expectedError: "failed create internet gateway: some error",
		},
		{
			name: "creation skip",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "foo",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn1"),
						InternetGateway: infrastructurev1beta2.InternetGateway{
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
			ociClusterAccessor := OCISelfManagedCluster{
				&infrastructurev1beta2.OCICluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cluster_uid",
					},
					Spec: tt.spec,
				},
			}
			ociClusterAccessor.OCICluster.Spec.OCIResourceIdentifier = "resource_uid"
			s := &ClusterScope{
				VCNClient:          vcnClient,
				OCIClusterAccessor: ociClusterAccessor,
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cluster_uid",
					},
				},
				Logger: &l,
			}
			err := s.ReconcileInternetGateway(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileInternetGateway() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("ReconcileInternetGateway() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestClusterScope_DeleteInternetGateway(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	vcnClient.EXPECT().GetInternetGateway(gomock.Any(), gomock.Eq(core.GetInternetGatewayRequest{
		IgId: common.String("normal_id"),
	})).
		Return(core.GetInternetGatewayResponse{
			InternetGateway: core.InternetGateway{
				Id:           common.String("normal_id"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetInternetGateway(gomock.Any(), gomock.Eq(core.GetInternetGatewayRequest{
		IgId: common.String("error_delete_igw"),
	})).
		Return(core.GetInternetGatewayResponse{
			InternetGateway: core.InternetGateway{
				Id:           common.String("error_delete_igw"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetInternetGateway(gomock.Any(), gomock.Eq(core.GetInternetGatewayRequest{
		IgId: common.String("error")})).
		Return(core.GetInternetGatewayResponse{}, errors.New("some error in GetInternetGateway"))

	vcnClient.EXPECT().GetInternetGateway(gomock.Any(), gomock.Eq(core.GetInternetGatewayRequest{IgId: common.String("igw_deleted")})).
		Return(core.GetInternetGatewayResponse{}, errors.New("not found"))
	vcnClient.EXPECT().DeleteInternetGateway(gomock.Any(), gomock.Eq(core.DeleteInternetGatewayRequest{
		IgId: common.String("normal_id"),
	})).
		Return(core.DeleteInternetGatewayResponse{}, nil)
	vcnClient.EXPECT().DeleteInternetGateway(gomock.Any(), gomock.Eq(core.DeleteInternetGatewayRequest{
		IgId: common.String("error_delete_igw"),
	})).
		Return(core.DeleteInternetGatewayResponse{}, errors.New("some error in DeleteInternetGateway"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete internet gateway is successful",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						InternetGateway: infrastructurev1beta2.InternetGateway{
							Id: common.String("normal_id"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "internet gateway already deleted",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						InternetGateway: infrastructurev1beta2.InternetGateway{
							Id: common.String("igw_deleted"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete internet gateway error when calling get internet gateway",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						InternetGateway: infrastructurev1beta2.InternetGateway{
							Id: common.String("error"),
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "some error in GetInternetGateway",
		},
		{
			name: "delete internet gateway error when calling delete internet gateway",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						InternetGateway: infrastructurev1beta2.InternetGateway{
							Id: common.String("error_delete_igw"),
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to delete InternetGateway: some error in DeleteInternetGateway",
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
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cluster_uid",
					},
				},
				Logger: &l,
			}
			err := s.DeleteInternetGateway(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteInternetGateway() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("DeleteInternetGateway() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}
}
