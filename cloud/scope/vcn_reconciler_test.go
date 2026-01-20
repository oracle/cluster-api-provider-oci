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
	"fmt"
	"reflect"
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

func TestClusterScope_CreateVCN(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	vcnClient.EXPECT().CreateVcn(gomock.Any(), Eq(func(request interface{}) error {
		return vcnMatcher(request, "normal", common.String("label"), []string{"test-cidr"})
	})).
		Return(core.CreateVcnResponse{
			Vcn: core.Vcn{
				Id: common.String("normal_id"),
			},
		}, nil)
	vcnClient.EXPECT().CreateVcn(gomock.Any(), Eq(func(request interface{}) error {
		return vcnMatcher(request, "normal", common.String("label"), []string{"test-cidr1", "test-cidr2"})
	})).
		Return(core.CreateVcnResponse{
			Vcn: core.Vcn{
				Id: common.String("normal_id"),
			},
		}, nil)
	vcnClient.EXPECT().CreateVcn(gomock.Any(), Eq(func(request interface{}) error {
		return vcnMatcher(request, "error", nil, []string{VcnDefaultCidr})
	})).
		Return(core.CreateVcnResponse{}, errors.New("some error"))

	tests := []struct {
		name    string
		spec    infrastructurev1beta2.OCIClusterSpec
		want    *string
		wantErr bool
	}{
		{
			name: "create vcn is successful",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Name:          "normal",
						DnsLabel:      common.String("label"),
						CIDR:          "test-cidr",
						IsIpv6Enabled: common.Bool(true),
					},
				},
			},
			want:    common.String("normal_id"),
			wantErr: false,
		},
		{
			name: "create vcn is successful, multiple cidrs",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Name:          "normal",
						DnsLabel:      common.String("label"),
						CIDRS:         []string{"test-cidr1", "test-cidr2"},
						IsIpv6Enabled: common.Bool(true),
					},
				},
			},
			want:    common.String("normal_id"),
			wantErr: false,
		},
		{
			name: "create vcn error",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Name: "error",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociClusterAccessor := OCISelfManagedCluster{
				&infrastructurev1beta2.OCICluster{
					Spec: tt.spec,
				},
			}
			tt.spec.OCIResourceIdentifier = "resource_uid"
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
			got, err := s.CreateVCN(context.Background(), tt.spec.NetworkSpec.Vcn)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateVCN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateVCN() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterScope_DeleteVCN(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	vcnClient.EXPECT().GetVcn(gomock.Any(), gomock.Eq(core.GetVcnRequest{
		VcnId: common.String("normal_id"),
	})).
		Return(core.GetVcnResponse{
			Vcn: core.Vcn{
				Id:           common.String("normal_id"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetVcn(gomock.Any(), gomock.Eq(core.GetVcnRequest{
		VcnId: common.String("error_delete_vcn"),
	})).
		Return(core.GetVcnResponse{
			Vcn: core.Vcn{
				Id:           common.String("error_delete_vcn"),
				FreeformTags: tags,
			},
		}, nil)
	vcnClient.EXPECT().GetVcn(gomock.Any(), gomock.Eq(core.GetVcnRequest{VcnId: common.String("error")})).
		Return(core.GetVcnResponse{}, errors.New("some error in GetVcn"))

	vcnClient.EXPECT().GetVcn(gomock.Any(), gomock.Eq(core.GetVcnRequest{VcnId: common.String("vcn_deleted")})).
		Return(core.GetVcnResponse{}, errors.New("not found"))
	vcnClient.EXPECT().DeleteVcn(gomock.Any(), gomock.Eq(core.DeleteVcnRequest{
		VcnId: common.String("normal_id"),
	})).
		Return(core.DeleteVcnResponse{}, nil)
	vcnClient.EXPECT().DeleteVcn(gomock.Any(), gomock.Eq(core.DeleteVcnRequest{
		VcnId: common.String("error_delete_vcn"),
	})).
		Return(core.DeleteVcnResponse{}, errors.New("some error in DeleteVcn"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "delete vcn is successful",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("normal_id"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "vcn already deleted",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn_deleted"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete vcn error when calling get vcn",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("error"),
					},
				},
			},
			wantErr:       true,
			expectedError: "some error in GetVcn",
		},
		{
			name: "delete vcn error when calling delete vcn",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("error_delete_vcn"),
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to delete vcn: some error in DeleteVcn",
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
			err := s.DeleteVCN(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteVCN() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("DeleteVCN() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}
}

func TestClusterScope_GetVCN(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	vcnClient.EXPECT().ListVcns(gomock.Any(), gomock.Eq(core.ListVcnsRequest{
		CompartmentId: common.String("bar"),
		DisplayName:   common.String("foo"),
	})).Return(
		core.ListVcnsResponse{
			Items: []core.Vcn{
				{
					FreeformTags: tags,
					Id:           common.String("vcn_id"),
				},
			}}, nil)
	vcnClient.EXPECT().ListVcns(gomock.Any(), gomock.Eq(core.ListVcnsRequest{
		CompartmentId: common.String("bar"),
		DisplayName:   common.String("not_found"),
	})).Return(
		core.ListVcnsResponse{
			Items: []core.Vcn{
				{
					Id: common.String("vcn_id"),
				},
			}}, nil)
	vcnClient.EXPECT().ListVcns(gomock.Any(), gomock.Eq(core.ListVcnsRequest{
		CompartmentId: common.String("bar"),
		DisplayName:   common.String("error"),
	})).Return(
		core.ListVcnsResponse{}, errors.New("some error"))
	vcnClient.EXPECT().GetVcn(gomock.Any(), gomock.Eq(core.GetVcnRequest{
		VcnId: common.String("not_managed"),
	})).
		Return(core.GetVcnResponse{
			Vcn: core.Vcn{
				Id: common.String("not_managed"),
			},
		}, nil)
	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		want          *core.Vcn
		expectedError string
		wantErr       bool
	}{
		{
			name: "vcn id not present in spec find by name successful",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "bar",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Name: "foo",
					},
				},
			},
			want: &core.Vcn{
				Id:           common.String("vcn_id"),
				FreeformTags: tags,
			},
			wantErr: false,
		},
		{
			name: "vcn id not present in spec find by name error",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "bar",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Name: "error",
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to list vcn by name: some error",
		},
		{
			name: "vcn id not present in spec not found by name",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "bar",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Name: "not_found",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "vcn id not present in spec but not managed by clusterapi",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("not_managed"),
					},
				},
			},
			wantErr:       true,
			expectedError: "cluster api tags have been modified out of context",
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
			got, err := s.GetVCN(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVCN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetVCN() got = %v, want %v", got, tt.want)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("GetVCN() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}
}

func TestClusterScope_GetVcnCidr(t *testing.T) {
	tests := []struct {
		name string
		spec infrastructurev1beta2.OCIClusterSpec
		want []string
	}{
		{
			name: "cidr not present",
			want: []string{VcnDefaultCidr},
		},
		{
			name: "cidr present",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						CIDR: "foo",
					},
				},
			},
			want: []string{"foo"},
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
				OCIClusterAccessor: ociClusterAccessor,
				Logger:             &l,
			}
			if got := s.GetVcnCidrs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetVcnCidr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterScope_GetVcnName(t *testing.T) {
	tests := []struct {
		name string
		spec infrastructurev1beta2.OCIClusterSpec
		want string
	}{
		{
			name: "name not present",
			want: "bar",
		},
		{
			name: "name present",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Name: "foo",
					},
				},
			},
			want: "foo",
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociClusterAccessor := OCISelfManagedCluster{
				&infrastructurev1beta2.OCICluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
						UID:  "cluster_uid",
					},
					Spec: tt.spec,
				},
			}
			ociClusterAccessor.OCICluster.Spec.OCIResourceIdentifier = "resource_uid"
			s := &ClusterScope{
				OCIClusterAccessor: ociClusterAccessor,
				Logger:             &l,
			}
			if got := s.GetVcnName(); got != tt.want {
				t.Errorf("GetVcnName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterScope_IsVcnEquals(t *testing.T) {
	tests := []struct {
		name    string
		spec    infrastructurev1beta2.OCIClusterSpec
		actual  *core.Vcn
		desired infrastructurev1beta2.VCN
		want    bool
	}{
		{
			name: "name different",
			actual: &core.Vcn{
				DisplayName: common.String("foo"),
			},
			desired: infrastructurev1beta2.VCN{
				Name: "bar",
			},
			want: false,
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
				OCIClusterAccessor: ociClusterAccessor,
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						UID: "resource_uid",
					},
				},
				Logger: &l,
			}
			if got := s.IsVcnEquals(tt.actual); got != tt.want {
				t.Errorf("IsVcnEquals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterScope_ReconcileVCN(t *testing.T) {
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

	vcnClient.EXPECT().GetVcn(gomock.Any(), gomock.Eq(core.GetVcnRequest{
		VcnId: common.String("normal_id"),
	})).
		Return(core.GetVcnResponse{
			Vcn: core.Vcn{
				Id:           common.String("normal_id"),
				FreeformTags: tags,
				DisplayName:  common.String("foo"),
				DefinedTags:  definedTagsInterface,
			},
		}, nil).AnyTimes()

	vcnClient.EXPECT().UpdateVcn(gomock.Any(), gomock.Eq(core.UpdateVcnRequest{
		VcnId: common.String("normal_id"),
		UpdateVcnDetails: core.UpdateVcnDetails{
			DisplayName: common.String("foo1"),
		},
	})).
		Return(core.UpdateVcnResponse{
			Vcn: core.Vcn{
				Id:           common.String("normal_id"),
				FreeformTags: tags,
				DisplayName:  common.String("foo1"),
			},
		}, nil)

	vcnClient.EXPECT().UpdateVcn(gomock.Any(), gomock.Eq(core.UpdateVcnRequest{
		VcnId: common.String("normal_id"),
		UpdateVcnDetails: core.UpdateVcnDetails{
			DisplayName: common.String("foo2"),
		},
	})).
		Return(core.UpdateVcnResponse{
			Vcn: core.Vcn{},
		}, errors.New("some error"))

	vcnClient.EXPECT().ListVcns(gomock.Any(), gomock.Eq(core.ListVcnsRequest{
		CompartmentId: common.String("bar"),
		DisplayName:   common.String("not_found"),
	})).Return(
		core.ListVcnsResponse{
			Items: []core.Vcn{
				{
					Id: common.String("vcn_id"),
				},
			}}, nil)

	vcnClient.EXPECT().CreateVcn(gomock.Any(), Eq(func(request interface{}) error {
		return vcnMatcher(request, "not_found", common.String("label"), []string{VcnDefaultCidr})
	})).
		Return(core.CreateVcnResponse{
			Vcn: core.Vcn{
				Id: common.String("not_found"),
			},
		}, nil)

	tests := []struct {
		name          string
		spec          infrastructurev1beta2.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "no reconciliation needed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				DefinedTags: definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID:   common.String("normal_id"),
						Name: "foo",
						CIDR: "bar",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "vcn update needed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID:   common.String("normal_id"),
						Name: "foo1",
						CIDR: "bar",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "vcn update needed but error out",
			spec: infrastructurev1beta2.OCIClusterSpec{
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID:   common.String("normal_id"),
						Name: "foo2",
						CIDR: "bar",
					},
				},
			},
			wantErr:       true,
			expectedError: "failed to reconcile the vcn, failed to update: some error",
		},
		{
			name: "vcn creation needed",
			spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId: "bar",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						Name:     "not_found",
						DnsLabel: common.String("label"),
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
			err := s.ReconcileVCN(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileVCN() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("ReconcileVCN() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func vcnMatcher(request interface{}, displayName string, dnsLabel *string, cidrs []string) error {
	r, ok := request.(core.CreateVcnRequest)
	if !ok {
		return errors.New("expecting CreateVcnRequest type")
	}
	if *r.CreateVcnDetails.DisplayName != displayName {
		return errors.New(fmt.Sprintf("expecting DisplayName as %s", displayName))
	}
	if !reflect.DeepEqual(r.CreateVcnDetails.DnsLabel, dnsLabel) {
		return errors.New(fmt.Sprintf("expecting DnsLabel as %v", dnsLabel))
	}
	if !reflect.DeepEqual(r.CreateVcnDetails.CidrBlocks, cidrs) {
		return errors.New(fmt.Sprintf("expecting cidrblocks as %v, actual %v", cidrs, r.CreateVcnDetails.CidrBlocks))
	}
	return nil
}
