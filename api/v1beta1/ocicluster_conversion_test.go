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

func TestOCICluster_BackendSetsRoundTrip(t *testing.T) {
	t.Run("v1beta1 legacy -> v1beta2 -> v1beta1 keeps legacy shape", func(t *testing.T) {
		src := &OCICluster{
			Spec: OCIClusterSpec{
				NetworkSpec: NetworkSpec{
					APIServerLB: LoadBalancer{
						NLBSpec: NLBSpec{
							BackendSetDetails: BackendSetDetails{
								IsFailOpen: boolPtr(true),
							},
						},
					},
				},
			},
		}

		hub := &v1beta2.OCICluster{}
		if err := src.ConvertTo(hub); err != nil {
			t.Fatalf("convert to hub failed: %v", err)
		}
		if len(hub.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets) != 0 {
			t.Fatalf("expected no canonical backendSets to be synthesized during conversion, got %#v", hub.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets)
		}
		if hub.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSetDetails.IsFailOpen == nil || !*hub.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSetDetails.IsFailOpen {
			t.Fatalf("expected legacy backendSetDetails to be preserved on hub")
		}

		restored := &OCICluster{}
		if err := restored.ConvertFrom(hub); err != nil {
			t.Fatalf("convert from hub failed: %v", err)
		}
		if restored.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSetDetails.IsFailOpen == nil || !*restored.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSetDetails.IsFailOpen {
			t.Fatalf("expected legacy backendSetDetails after roundtrip, got %#v", restored.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSetDetails)
		}
	})

	t.Run("v1beta2 canonical -> v1beta1 -> v1beta2 keeps canonical shape", func(t *testing.T) {
		srcHub := &v1beta2.OCICluster{
			Spec: v1beta2.OCIClusterSpec{
				NetworkSpec: v1beta2.NetworkSpec{
					APIServerLB: v1beta2.LoadBalancer{
						NLBSpec: v1beta2.NLBSpec{
							BackendSets: []v1beta2.NLBBackendSet{
								{
									Name: "new-set",
									BackendSetDetails: v1beta2.BackendSetDetails{
										IsFailOpen: boolPtr(true),
									},
								},
							},
						},
					},
				},
			},
		}

		spoke := &OCICluster{}
		if err := spoke.ConvertFrom(srcHub); err != nil {
			t.Fatalf("convert from hub failed: %v", err)
		}
		if len(spoke.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets) != 1 || spoke.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets[0].Name != "new-set" {
			t.Fatalf("expected canonical backendSets after down-conversion, got %#v", spoke.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets)
		}

		restoredHub := &v1beta2.OCICluster{}
		if err := spoke.ConvertTo(restoredHub); err != nil {
			t.Fatalf("convert back to hub failed: %v", err)
		}
		if len(restoredHub.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets) != 1 || restoredHub.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets[0].Name != "new-set" {
			t.Fatalf("expected canonical backendSets after hub roundtrip, got %#v", restoredHub.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets)
		}
		if restoredHub.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets[0].BackendSetDetails.IsFailOpen == nil || !*restoredHub.Spec.NetworkSpec.APIServerLB.NLBSpec.BackendSets[0].BackendSetDetails.IsFailOpen {
			t.Fatalf("expected backendSetDetails to remain intact after roundtrip")
		}
	})
}

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
