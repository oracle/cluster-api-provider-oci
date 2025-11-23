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

package ociutil

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/core"
)

func TestGetCloudProviderConfig(t *testing.T) {
	testCases := []struct {
		name        string
		in          string
		expected    core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum
		expectedErr error
	}{
		{
			name:     "BASELINE_1_8",
			in:       "BASELINE_1_8",
			expected: core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization8,
		},
		{
			name:     "BASELINE_1_2",
			in:       "BASELINE_1_2",
			expected: core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization2,
		},
		{
			name:     "BASELINE_1_1",
			in:       "BASELINE_1_1",
			expected: core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization1,
		},
		{
			name:        "invalid",
			in:          "invalid",
			expectedErr: fmt.Errorf("invalid baseline cpu optimization parameter"),
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetBaseLineOcpuOptimizationEnum(tt.in)
			if tt.expectedErr != nil {
				if !reflect.DeepEqual(err.Error(), tt.expectedErr.Error()) {
					t.Errorf("Test (%s) \n Expected %q, \n Actual %q", tt.name, tt.expectedErr, err)
				}
			} else {
				if result != tt.expected {
					t.Errorf("Test (%s) \n Expected %q, \n Actual %q", tt.name, tt.expected, result)
				}
			}
		})
	}
}

func TestAddToDefaultClusterTags(t *testing.T) {
	testUUID := "UUIDTEST"
	tags := BuildClusterTags(testUUID)
	if tags[ClusterResourceIdentifier] != testUUID {
		t.Errorf("Tags don't match Expected: %s, Actual: %s", testUUID, tags[ClusterResourceIdentifier])
	}

	// should also contain default tags
	defaultTags := GetDefaultClusterTags()
	for key, _ := range defaultTags {
		if defaultTags[key] != tags[key] {
			t.Errorf("Default tags don't match Expected: %s, Actual: %s", defaultTags[key], tags[key])
		}
	}
}

func TestIsOutOfHostCapacityError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "matches by code",
			err:      fakeServiceError{code: "OutOfHostCapacity", message: "any"},
			expected: true,
		},
		{
			name:     "matches by message",
			err:      fakeServiceError{code: "Other", message: "Instance launch failed due to out of host capacity"},
			expected: true,
		},
		{
			name:     "non service error",
			err:      fmt.Errorf("boom"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := IsOutOfHostCapacityError(tc.err)
			if actual != tc.expected {
				t.Fatalf("expected %t but got %t for test %s", tc.expected, actual, tc.name)
			}
		})
	}
}

type fakeServiceError struct {
	code    string
	message string
}

func (f fakeServiceError) Error() string {
	return f.message
}

func (f fakeServiceError) GetHTTPStatusCode() int {
	return 400
}

func (f fakeServiceError) GetMessage() string {
	return f.message
}

func (f fakeServiceError) GetCode() string {
	return f.code
}

func (f fakeServiceError) GetOpcRequestID() string { return "" }
