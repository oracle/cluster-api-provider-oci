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

package scope

import (
	"testing"

	"github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func Test_GetNsgNamesFromId(t *testing.T) {
	tests := []struct {
		name     string
		ids      []string
		nsgs     []*infrastructurev1beta2.NSG
		expected []string
	}{
		{
			name: "single",
			ids:  []string{"id-1"},
			nsgs: []*infrastructurev1beta2.NSG{
				{
					ID:   common.String("id-1"),
					Name: "test-1",
				},
				{
					ID:   common.String("id-2"),
					Name: "test-2",
				},
			},
			expected: []string{"test-1"},
		},
		{
			name: "multiple",
			ids:  []string{"id-1", "id-2"},
			nsgs: []*infrastructurev1beta2.NSG{
				{
					ID:   common.String("id-1"),
					Name: "test-1",
				},
				{
					ID:   common.String("id-2"),
					Name: "test-2",
				},
			},
			expected: []string{"test-1", "test-2"},
		},
		{
			name: "none",
			ids:  []string{"id-3"},
			nsgs: []*infrastructurev1beta2.NSG{
				{
					ID:   common.String("id-1"),
					Name: "test-1",
				},
				{
					ID:   common.String("id-2"),
					Name: "test-2",
				},
			},
			expected: make([]string, 0),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			actual := GetNsgNamesFromId(test.ids, test.nsgs)
			g.Expect(actual).To(gomega.Equal(test.expected))
		})
	}
}

func Test_GetSubnetNameFromId(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		nsgs     []*infrastructurev1beta2.Subnet
		expected string
	}{
		{
			name: "single",
			id:   "id-1",
			nsgs: []*infrastructurev1beta2.Subnet{
				{
					ID:   common.String("id-1"),
					Name: "test-1",
				},
				{
					ID:   common.String("id-2"),
					Name: "test-2",
				},
			},
			expected: "test-1",
		},
		{
			name: "none",
			id:   "id-3",
			nsgs: []*infrastructurev1beta2.Subnet{
				{
					ID:   common.String("id-1"),
					Name: "test-1",
				},
				{
					ID:   common.String("id-2"),
					Name: "test-2",
				},
			},
			expected: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			actual := GetSubnetNameFromId(&test.id, test.nsgs)
			g.Expect(actual).To(gomega.Equal(test.expected))
		})
	}
}

func Test_GetSubnetNamesFromId(t *testing.T) {
	tests := []struct {
		name     string
		ids      []string
		subnets  []*infrastructurev1beta2.Subnet
		expected []string
	}{
		{
			name: "single",
			ids:  []string{"id-1"},
			subnets: []*infrastructurev1beta2.Subnet{
				{
					ID:   common.String("id-1"),
					Name: "test-1",
				},
				{
					ID:   common.String("id-2"),
					Name: "test-2",
				},
			},
			expected: []string{"test-1"},
		},
		{
			name: "multiple",
			ids:  []string{"id-1", "id-2"},
			subnets: []*infrastructurev1beta2.Subnet{
				{
					ID:   common.String("id-1"),
					Name: "test-1",
				},
				{
					ID:   common.String("id-2"),
					Name: "test-2",
				},
			},
			expected: []string{"test-1", "test-2"},
		},
		{
			name: "none",
			ids:  []string{"id-3"},
			subnets: []*infrastructurev1beta2.Subnet{
				{
					ID:   common.String("id-1"),
					Name: "test-1",
				},
				{
					ID:   common.String("id-2"),
					Name: "test-2",
				},
			},
			expected: make([]string, 0),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			actual := GetSubnetNamesFromId(test.ids, test.subnets)
			g.Expect(actual).To(gomega.Equal(test.expected))
		})
	}
}

func TestConvertMachineExtendedMetadata(t *testing.T) {
	t.Run("converts nested json values", func(t *testing.T) {
		g := gomega.NewWithT(t)
		input := map[string]apiextensionsv1.JSON{
			"cilium-primary-vnic": {
				Raw: []byte(`{"ip-count":32,"cidr-blocks":["10.0.0.0/24"]}`),
			},
		}

		actual, err := ConvertMachineExtendedMetadata(input)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(actual).To(gomega.Equal(map[string]interface{}{
			"cilium-primary-vnic": map[string]interface{}{
				"ip-count":    float64(32),
				"cidr-blocks": []interface{}{"10.0.0.0/24"},
			},
		}))
	})

	t.Run("returns nil for empty input", func(t *testing.T) {
		g := gomega.NewWithT(t)
		actual, err := ConvertMachineExtendedMetadata(nil)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(actual).To(gomega.BeNil())
	})

	t.Run("returns error for invalid json", func(t *testing.T) {
		g := gomega.NewWithT(t)
		input := map[string]apiextensionsv1.JSON{
			"invalid": {
				Raw: []byte(`{"broken":`),
			},
		}

		actual, err := ConvertMachineExtendedMetadata(input)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(actual).To(gomega.BeNil())
	})
}
