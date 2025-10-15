/*
 Copyright (c) 2023 Oracle and/or its affiliates.

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

package v1beta1

import (
	"reflect"
	"testing"

	"github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func TestOCICluster_ConvertTo(t *testing.T) {
	type fields struct {
		TypeMeta   metav1.TypeMeta
		ObjectMeta metav1.ObjectMeta
		Spec       OCIClusterSpec
		Status     OCIClusterStatus
	}
	type args struct {
		dstRaw conversion.Hub
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful conversion",
			fields: fields{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: OCIClusterSpec{
					Region: "us-phoenix-1",
				},
				Status: OCIClusterStatus{
					AvailabilityDomains: map[string]OCIAvailabilityDomain{
						"AD-1": {
							Name:         "Uocm:PHX-AD-1",
							FaultDomains: []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
						},
					},
				},
			},
			args: args{
				dstRaw: &v1beta2.OCICluster{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &OCICluster{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
				Status:     tt.fields.Status,
			}
			if err := src.ConvertTo(tt.args.dstRaw); (err != nil) != tt.wantErr {
				t.Errorf("OCICluster.ConvertTo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_convertv1beta1NSGListTov1beta2NSGList(t *testing.T) {
	type args struct {
		in []*NSG
	}
	tests := []struct {
		name    string
		args    args
		want    []*v1beta2.NSG
		wantErr bool
	}{
		{
			name: "normal conversion",
			args: args{
				in: []*NSG{
					{ID: func() *string { s := "nsg1"; return &s }(), Name: "NSG-1"},
					{ID: func() *string { s := "nsg2"; return &s }(), Name: "NSG-2"},
				},
			},
			want: []*v1beta2.NSG{
				{ID: func() *string { s := "nsg1"; return &s }(), Name: "NSG-1"},
				{ID: func() *string { s := "nsg2"; return &s }(), Name: "NSG-2"},
			},
			wantErr: false,
		},
		{
			name: "slice contains nil",
			args: args{
				in: []*NSG{
					{ID: func() *string { s := "nsg1"; return &s }(), Name: "NSG-1"},
					nil,
				},
			},
			want: []*v1beta2.NSG{
				{ID: func() *string { s := "nsg1"; return &s }(), Name: "NSG-1"},
				nil,
			},
			wantErr: false,
		},
		{
			name: "empty slice",
			args: args{
				in: []*NSG{},
			},
			want:    []*v1beta2.NSG{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertv1beta1NSGListTov1beta2NSGList(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertv1beta1NSGListTov1beta2NSGList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertv1beta1NSGListTov1beta2NSGList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertv1beta2NSGListTov1beta1NSGList(t *testing.T) {
	type args struct {
		in []*v1beta2.NSG
	}
	tests := []struct {
		name    string
		args    args
		want    []*NSG
		wantErr bool
	}{
		{
			name: "normal conversion",
			args: args{
				in: []*v1beta2.NSG{
					{ID: func() *string { s := "nsg1"; return &s }(), Name: "NSG-1"},
					{ID: func() *string { s := "nsg2"; return &s }(), Name: "NSG-2"},
				},
			},
			want: []*NSG{
				{ID: func() *string { s := "nsg1"; return &s }(), Name: "NSG-1"},
				{ID: func() *string { s := "nsg2"; return &s }(), Name: "NSG-2"},
			},
			wantErr: false,
		},
		{
			name: "slice contains nil",
			args: args{
				in: []*v1beta2.NSG{
					{ID: func() *string { s := "nsg1"; return &s }(), Name: "NSG-1"},
					nil,
				},
			},
			want: []*NSG{
				{ID: func() *string { s := "nsg1"; return &s }(), Name: "NSG-1"},
				nil,
			},
			wantErr: false,
		},
		{
			name: "empty slice",
			args: args{
				in: []*v1beta2.NSG{},
			},
			want:    []*NSG{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertv1beta2NSGListTov1beta1NSGList(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertv1beta2NSGListTov1beta1NSGList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertv1beta2NSGListTov1beta1NSGList() = %v, want %v", got, tt.want)
			}
		})
	}
}
