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
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
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
			actual := GetSubnetNameFromId(&test.id, test.nsgs, nil)
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

func Test_GetSubnetIdFromName(t *testing.T) {
	tests := []struct {
		name         string
		compartmentId string
		subnetName   string
		subnets      []core.Subnet
		expected     string
		expectError  bool
	}{
		{
			name:         "subnet found",
			compartmentId: "test-compartment",
			subnetName:   "test-subnet",
			subnets: []core.Subnet{
				{
					Id:          common.String("ocid1.subnet.oc1.test.1"),
					DisplayName: common.String("test-subnet"),
				},
				{
					Id:          common.String("ocid1.subnet.oc1.test.2"),
					DisplayName: common.String("other-subnet"),
				},
			},
			expected:    "ocid1.subnet.oc1.test.1",
			expectError: false,
		},
		{
			name:         "subnet not found",
			compartmentId: "test-compartment",
			subnetName:   "nonexistent-subnet",
			subnets: []core.Subnet{
				{
					Id:          common.String("ocid1.subnet.oc1.test.1"),
					DisplayName: common.String("test-subnet"),
				},
			},
			expected:    "",
			expectError: true,
		},
		{
			name:         "empty subnets",
			compartmentId: "test-compartment",
			subnetName:   "test-subnet",
			subnets:      []core.Subnet{},
			expected:     "",
			expectError:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			vcnClient := mock_vcn.NewMockClient(ctrl)
			vcnClient.EXPECT().ListSubnets(gomock.Any(), gomock.Eq(core.ListSubnetsRequest{
				CompartmentId: common.String(test.compartmentId),
			})).Return(
				core.ListSubnetsResponse{
					Items: test.subnets,
				}, nil)

			actual, err := GetSubnetIdFromName(context.Background(), vcnClient, test.compartmentId, test.subnetName)
			if test.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(actual).To(gomega.Equal(test.expected))
			}
		})
	}
}
