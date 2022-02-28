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
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v53/common"
	"github.com/oracle/oci-go-sdk/v53/core"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
)

func TestClusterScope_DeleteSecurityLists(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags["CreatedBy"] = "OCIClusterAPIProvider"
	tags["ClusterUUID"] = "a"
	vcnClient.EXPECT().GetSecurityList(gomock.Any(), gomock.Eq(core.GetSecurityListRequest{
		SecurityListId: common.String("cp_endpoint_id"),
	})).
		Return(core.GetSecurityListResponse{
			SecurityList: core.SecurityList{
				Id:           common.String("cp_endpoint_id"),
				FreeformTags: tags,
			},
		}, nil)

	vcnClient.EXPECT().GetSecurityList(gomock.Any(), gomock.Eq(core.GetSecurityListRequest{
		SecurityListId: common.String("cp_mc_id"),
	})).
		Return(core.GetSecurityListResponse{
			SecurityList: core.SecurityList{
				Id:           common.String("cp_mc_id"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetSecurityList(gomock.Any(), gomock.Eq(core.GetSecurityListRequest{
		SecurityListId: common.String("cp_endpoint_id_error_delete"),
	})).
		Return(core.GetSecurityListResponse{
			SecurityList: core.SecurityList{
				Id:           common.String("cp_endpoint_id_error_delete"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetSecurityList(gomock.Any(), gomock.Eq(core.GetSecurityListRequest{
		SecurityListId: common.String("cp_endpoint_id_error")})).
		Return(core.GetSecurityListResponse{}, errors.New("some error in GetSecurityList"))

	vcnClient.EXPECT().GetSecurityList(gomock.Any(), gomock.Eq(core.GetSecurityListRequest{SecurityListId: common.String("ep_SecurityList_deleted")})).
		Return(core.GetSecurityListResponse{}, errors.New("not found"))
	vcnClient.EXPECT().GetSecurityList(gomock.Any(), gomock.Eq(core.GetSecurityListRequest{SecurityListId: common.String("mc_SecurityList_deleted")})).
		Return(core.GetSecurityListResponse{}, errors.New("not found"))
	vcnClient.EXPECT().DeleteSecurityList(gomock.Any(), gomock.Eq(core.DeleteSecurityListRequest{
		SecurityListId: common.String("cp_endpoint_id"),
	})).
		Return(core.DeleteSecurityListResponse{}, nil)
	vcnClient.EXPECT().DeleteSecurityList(gomock.Any(), gomock.Eq(core.DeleteSecurityListRequest{
		SecurityListId: common.String("cp_mc_id"),
	})).
		Return(core.DeleteSecurityListResponse{}, nil)
	vcnClient.EXPECT().DeleteSecurityList(gomock.Any(), gomock.Eq(core.DeleteSecurityListRequest{
		SecurityListId: common.String("cp_endpoint_id_error_delete"),
	})).
		Return(core.DeleteSecurityListResponse{}, errors.New("some error in SecurityList delete"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta1.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete SecurityList is successful",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								SecurityList: &infrastructurev1beta1.SecurityList{
									ID: common.String("cp_mc_id"),
								},
							},
							{
								SecurityList: &infrastructurev1beta1.SecurityList{
									ID: common.String("cp_endpoint_id"),
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "SecurityList already deleted",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								SecurityList: &infrastructurev1beta1.SecurityList{
									ID: common.String("ep_SecurityList_deleted"),
								},
							},
							{
								SecurityList: &infrastructurev1beta1.SecurityList{
									ID: common.String("mc_SecurityList_deleted"),
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete SecurityList error when calling get SecurityList",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								SecurityList: &infrastructurev1beta1.SecurityList{
									ID: common.String("cp_endpoint_id_error"),
								},
							},
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "some error in GetSecurityList",
		},
		{
			name: "delete security list error when calling delete security list",
			spec: infrastructurev1beta1.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								SecurityList: &infrastructurev1beta1.SecurityList{
									ID: common.String("cp_endpoint_id_error_delete"),
								},
							},
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to delete security list: some error in SecurityList delete",
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociCluster := infrastructurev1beta1.OCICluster{
				Spec: tt.spec,
				ObjectMeta: metav1.ObjectMeta{
					UID: "a",
				},
			}
			s := &ClusterScope{
				VCNClient:  vcnClient,
				OCICluster: &ociCluster,
				Logger:     &l,
			}
			err := s.DeleteSecurityLists(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteSecurityLists() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("DeleteSecurityLists() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}
}
