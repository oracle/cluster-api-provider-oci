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

func TestClusterScope_ReconcileServiceGateway(t *testing.T) {
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

	vcnClient.EXPECT().GetServiceGateway(gomock.Any(), gomock.Eq(core.GetServiceGatewayRequest{
		ServiceGatewayId: common.String("foo"),
	})).
		Return(core.GetServiceGatewayResponse{
			ServiceGateway: core.ServiceGateway{
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

	vcnClient.EXPECT().ListServiceGateways(gomock.Any(), gomock.Eq(core.ListServiceGatewaysRequest{
		CompartmentId: common.String("foo"),
		VcnId:         common.String("vcn"),
	})).Return(
		core.ListServiceGatewaysResponse{
			Items: []core.ServiceGateway{
				{
					Id:           common.String("sgw_id"),
					FreeformTags: tags,
					DisplayName:  common.String("service-gateway"),
				},
			}}, nil)

	vcnClient.EXPECT().ListServiceGateways(gomock.Any(), gomock.Eq(core.ListServiceGatewaysRequest{
		CompartmentId: common.String("foo"),
		VcnId:         common.String("vcn1"),
	})).Return(
		core.ListServiceGatewaysResponse{
			Items: []core.ServiceGateway{
				{
					Id:          common.String("sgw_id"),
					DisplayName: common.String("service-gateway"),
				},
			}}, nil).Times(2)

	vcnClient.EXPECT().CreateServiceGateway(gomock.Any(), gomock.Eq(core.CreateServiceGatewayRequest{
		CreateServiceGatewayDetails: core.CreateServiceGatewayDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("service-gateway"),
			VcnId:         common.String("vcn1"),
			Services: []core.ServiceIdRequestDetails{
				{
					ServiceId: common.String("svc"),
				},
			},
			FreeformTags: updatedTags,
			DefinedTags:  definedTagsInterface,
		},
	})).Return(
		core.CreateServiceGatewayResponse{
			ServiceGateway: core.ServiceGateway{
				Id: common.String("sgw"),
			},
		}, nil)

	vcnClient.EXPECT().CreateServiceGateway(gomock.Any(), gomock.Eq(core.CreateServiceGatewayRequest{
		CreateServiceGatewayDetails: core.CreateServiceGatewayDetails{
			CompartmentId: common.String("foo"),
			DisplayName:   common.String("service-gateway"),
			VcnId:         common.String("vcn1"),
			Services: []core.ServiceIdRequestDetails{
				{
					ServiceId: common.String("svc"),
				},
			},
			FreeformTags: tags,
			DefinedTags:  definedTagsInterface,
		},
	})).Return(
		core.CreateServiceGatewayResponse{}, errors.New("some error"))

	vcnClient.EXPECT().ListServices(gomock.Any(), gomock.Eq(core.ListServicesRequest{})).Return(
		core.ListServicesResponse{
			Items: []core.Service{
				{
					Id:        common.String("svc"),
					CidrBlock: common.String("foo-services-in-oracle-services-network"),
				},
			},
		}, nil).Times(2)

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
						ServiceGateway: infrastructurev1beta2.ServiceGateway{
							Id: common.String("foo"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "id not present in spec but found by name and no update",
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
			expectedError: "failed create service gateway: some error",
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
				Logger:             &l,
			}
			err := s.ReconcileServiceGateway(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileServiceGateway() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("ReconcileServiceGateway() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestClusterScope_DeleteServiceGateway(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	vcnClient.EXPECT().GetServiceGateway(gomock.Any(), gomock.Eq(core.GetServiceGatewayRequest{
		ServiceGatewayId: common.String("normal_id"),
	})).
		Return(core.GetServiceGatewayResponse{
			ServiceGateway: core.ServiceGateway{
				Id:           common.String("normal_id"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetServiceGateway(gomock.Any(), gomock.Eq(core.GetServiceGatewayRequest{
		ServiceGatewayId: common.String("error_delete_sgw"),
	})).
		Return(core.GetServiceGatewayResponse{
			ServiceGateway: core.ServiceGateway{
				Id:           common.String("error_delete_sgw"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetServiceGateway(gomock.Any(), gomock.Eq(core.GetServiceGatewayRequest{
		ServiceGatewayId: common.String("error")})).
		Return(core.GetServiceGatewayResponse{}, errors.New("some error in GetServiceGateway"))

	vcnClient.EXPECT().GetServiceGateway(gomock.Any(), gomock.Eq(core.GetServiceGatewayRequest{ServiceGatewayId: common.String("sgw_deleted")})).
		Return(core.GetServiceGatewayResponse{}, errors.New("not found"))
	vcnClient.EXPECT().DeleteServiceGateway(gomock.Any(), gomock.Eq(core.DeleteServiceGatewayRequest{
		ServiceGatewayId: common.String("normal_id"),
	})).
		Return(core.DeleteServiceGatewayResponse{}, nil)
	vcnClient.EXPECT().DeleteServiceGateway(gomock.Any(), gomock.Eq(core.DeleteServiceGatewayRequest{
		ServiceGatewayId: common.String("error_delete_sgw"),
	})).
		Return(core.DeleteServiceGatewayResponse{}, errors.New("some error in DeleteServiceGateway"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete Service gateway is successful",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ServiceGateway: infrastructurev1beta2.ServiceGateway{
							Id: common.String("normal_id"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Service gateway already deleted",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ServiceGateway: infrastructurev1beta2.ServiceGateway{
							Id: common.String("sgw_deleted"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete Service gateway error when calling get Service gateway",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ServiceGateway: infrastructurev1beta2.ServiceGateway{
							Id: common.String("error"),
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "some error in GetServiceGateway",
		},
		{
			name: "delete Service gateway error when calling delete Service gateway",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ServiceGateway: infrastructurev1beta2.ServiceGateway{
							Id: common.String("error_delete_sgw"),
						},
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to delete ServiceGateways: some error in DeleteServiceGateway",
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
						UID: "resource_uid",
					},
				},
				Logger: &l,
			}
			err := s.DeleteServiceGateway(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteServiceGateway() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("DeleteServiceGateway() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}
}
