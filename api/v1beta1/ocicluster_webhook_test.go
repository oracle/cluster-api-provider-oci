/*
 *
 * Copyright (c) 2022, Oracle and/or its affiliates.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 *
 */

package v1beta1

import (
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOCICluster_ValidateCreate(t *testing.T) {

	tests := []struct {
		name      string
		c         *OCICluster
		expectErr bool
	}{
		{
			name: "shouldn't allow bad ImageId",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "badocid",
				},
			},
			expectErr: true,
		},
		{
			name: "should succeed",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid",
				},
			},
			expectErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			if test.expectErr {
				g.Expect(test.c.ValidateCreate()).NotTo(gomega.Succeed())
			} else {
				g.Expect(test.c.ValidateCreate()).To(gomega.Succeed())
			}
		})
	}
}

func TestOCICluster_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name      string
		c         *OCICluster
		old       *OCICluster
		expectErr bool
	}{
		{
			name: "shouldn't region change",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					Region: "new-region",
				},
			},
			old: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					Region: "old-region",
				},
			},
			expectErr: true,
		},
		{
			name: "should succeed",
			c: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					CompartmentId: "ocid",
					Region:        "old-region",
				},
			},
			old: &OCICluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: OCIClusterSpec{
					Region: "old-region",
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			if test.expectErr {
				g.Expect(test.c.ValidateUpdate(test.old)).NotTo(gomega.Succeed())
			} else {
				g.Expect(test.c.ValidateUpdate(test.old)).To(gomega.Succeed())
			}
		})
	}

}
