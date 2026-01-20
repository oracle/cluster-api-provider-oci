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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var tests = []struct {
	name          string
	inputTemplate *OCIMachineTemplate
	errorField    string
	expectErr     bool
}{
	{
		name: "shouldn't allow bad ImageId",
		inputTemplate: &OCIMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: OCIMachineTemplateSpec{
				Template: OCIMachineTemplateResource{
					Spec: OCIMachineSpec{
						ImageId: "badocid",
					},
				},
			},
		},
		errorField: "imageId",
		expectErr:  true,
	},
	{
		name: "shouldn't allow bad CompartmentId",
		inputTemplate: &OCIMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: OCIMachineTemplateSpec{
				Template: OCIMachineTemplateResource{
					Spec: OCIMachineSpec{
						CompartmentId: "badocid",
					},
				},
			},
		},
		errorField: "compartmentId",
		expectErr:  true,
	},
	{
		name: "shouldn't allow empty shape",
		inputTemplate: &OCIMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: OCIMachineTemplateSpec{
				Template: OCIMachineTemplateResource{
					Spec: OCIMachineSpec{
						Shape: "",
					},
				},
			},
		},
		errorField: "shape",
		expectErr:  true,
	},
	{
		name: "should succeed",
		inputTemplate: &OCIMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: OCIMachineTemplateSpec{
				Template: OCIMachineTemplateResource{
					Spec: OCIMachineSpec{
						ImageId:       "ocid",
						CompartmentId: "ocid",
						Shape:         "DVH.DenseIO2.52",
					},
				},
			},
		},
		expectErr: false,
	},
}

func TestOCIMachineTemplate_ValidateCreate(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			if test.expectErr {
				_, err := (&OCIMachineTemplateWebhook{}).ValidateCreate(context.Background(), test.inputTemplate)
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorField)).To(gomega.BeTrue())
			} else {
				_, err := (&OCIMachineTemplateWebhook{}).ValidateCreate(context.Background(), test.inputTemplate)
				g.Expect(err).To(gomega.Succeed())
			}
		})
	}
}

func TestOCIMachineTemplateWebhook_ValidateDelete(t *testing.T) {
	tests := []struct {
		name    string
		obj     runtime.Object
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid OCIMachineTemplate object",
			obj:     &OCIMachineTemplate{ObjectMeta: metav1.ObjectMeta{Name: "test-machine-template"}},
			wantErr: false,
		},
		{
			name:    "invalid object type (Pod)",
			obj:     &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "not-a-machine-template"}},
			wantErr: true,
			errMsg:  "expected a OCIMachineTemplate",
		},
		{
			name:    "nil object",
			obj:     nil,
			wantErr: true,
			errMsg:  "expected a OCIMachineTemplate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &OCIMachineTemplateWebhook{}
			warnings, err := w.ValidateDelete(context.Background(), tt.obj)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else {
					// check it's an API error and contains the expected message
					if !apierrors.IsBadRequest(err) {
						t.Errorf("expected BadRequest error but got %T", err)
					}
					if !strings.Contains(err.Error(), tt.errMsg) {
						t.Errorf("expected error containing %q but got %q", tt.errMsg, err.Error())
					}
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// warnings should always be nil
			if warnings != nil {
				t.Errorf("expected no warnings but got: %v", warnings)
			}
		})
	}
}

func TestOCIMachineTemplate_ValidateUpdate(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			if test.expectErr {
				_, err := (&OCIMachineTemplateWebhook{}).ValidateUpdate(context.Background(), nil, test.inputTemplate)
				g.Expect(err).NotTo(gomega.Succeed())
				g.Expect(strings.Contains(err.Error(), test.errorField)).To(gomega.BeTrue())
			} else {
				_, err := (&OCIMachineTemplateWebhook{}).ValidateUpdate(context.Background(), nil, test.inputTemplate)
				g.Expect(err).To(gomega.Succeed())
			}
		})
	}
}

func TestOCIMachine_GetConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions clusterv1.Conditions
	}{
		{
			name: "non-empty conditions",
			conditions: clusterv1.Conditions{
				{
					Type:   "Ready",
					Status: corev1.ConditionTrue,
				},
				{
					Type:   "Provisioned",
					Status: corev1.ConditionFalse,
				},
			},
		},
		{
			name:       "empty conditions",
			conditions: clusterv1.Conditions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &OCIMachine{
				Status: OCIMachineStatus{
					Conditions: tt.conditions,
				},
			}

			if got := m.GetConditions(); !reflect.DeepEqual(got, tt.conditions) {
				t.Errorf("GetConditions() = %v, want %v", tt.conditions, tt.conditions)
			}
		})
	}
}

func TestOCIMachine_SetConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions clusterv1.Conditions
	}{
		{
			name: "set non-empty conditions",
			conditions: clusterv1.Conditions{
				{
					Type:   "Ready",
					Status: corev1.ConditionTrue,
				},
				{
					Type:   "Provisioned",
					Status: corev1.ConditionFalse,
				},
			},
		},
		{
			name:       "set empty conditions",
			conditions: clusterv1.Conditions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &OCIMachine{}
			m.SetConditions(tt.conditions)

			if !reflect.DeepEqual(m.Status.Conditions, tt.conditions) {
				t.Errorf("SetConditions() = %v, want %v", m.Status.Conditions, tt.conditions)
			}
		})
	}
}
