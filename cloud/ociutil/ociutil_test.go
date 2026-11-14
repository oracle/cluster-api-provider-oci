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
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer/mock_nlb"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
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

func TestIsOutOfHostCapacity(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "service error matches internal code and message",
			err:      fakeServiceError{status: http.StatusInternalServerError, message: OutOfHostCapacityErr},
			expected: true,
		},
		{
			name:     "non matching service error status",
			err:      fakeServiceError{status: http.StatusBadRequest, message: OutOfHostCapacityErr},
			expected: false,
		},
		{
			name:     "non matching service error message",
			err:      fakeServiceError{status: http.StatusInternalServerError, message: "other"},
			expected: false,
		},
		{
			name:     "non matching message",
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
			actual := IsOutOfHostCapacity(tc.err)
			if actual != tc.expected {
				t.Fatalf("expected %t but got %t for test %s", tc.expected, actual, tc.name)
			}
		})
	}
}

type fakeServiceError struct {
	status  int
	message string
}

func (f fakeServiceError) Error() string {
	return f.message
}

func (f fakeServiceError) GetHTTPStatusCode() int {
	return f.status
}

func (f fakeServiceError) GetMessage() string {
	return f.message
}

func (f fakeServiceError) GetCode() string {
	return ""
}

func (f fakeServiceError) GetOpcRequestID() string {
	return ""
}

func TestAwaitNLBWorkRequestPreservesListWorkRequestErrorsError(t *testing.T) {
	nlbClient := mock_nlb.NewMockNetworkLoadBalancerClient(gomock.NewController(t))
	workRequestID := common.String("work-request-id")
	compartmentID := common.String("compartment-id")
	requestID := "test-opc-request-id"
	originalErr := errors.New("ListWorkRequestErrors failed, opc-request-id: " + requestID)

	nlbClient.EXPECT().GetWorkRequest(gomock.Any(), networkloadbalancer.GetWorkRequestRequest{
		WorkRequestId: workRequestID,
	}).Return(networkloadbalancer.GetWorkRequestResponse{
		WorkRequest: networkloadbalancer.WorkRequest{
			Status:        networkloadbalancer.OperationStatusFailed,
			CompartmentId: compartmentID,
		},
	}, nil)
	nlbClient.EXPECT().ListWorkRequestErrors(gomock.Any(), networkloadbalancer.ListWorkRequestErrorsRequest{
		WorkRequestId: workRequestID,
		CompartmentId: compartmentID,
	}).Return(networkloadbalancer.ListWorkRequestErrorsResponse{}, originalErr)

	result, err := AwaitNLBWorkRequest(context.Background(), nlbClient, workRequestID)
	if result != nil {
		t.Errorf("expected nil work request, got %v", result)
	}
	if !errors.Is(err, originalErr) {
		t.Errorf("expected original ListWorkRequestErrors error, got %v", err)
	}
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, expected := range []string{requestID, "Failed to fetch the work-request-errors using nlb client"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("expected error to contain %q, got %v", expected, err)
		}
	}
}
