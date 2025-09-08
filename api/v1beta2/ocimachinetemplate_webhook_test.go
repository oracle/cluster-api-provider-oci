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
	"strings"
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
