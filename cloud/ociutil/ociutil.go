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
	"fmt"
	"net/http"
	"time"

	lb "github.com/oracle/cluster-api-provider-oci/cloud/services/loadbalancer"
	nlb "github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	WorkRequestPollInterval   = 5 * time.Second
	WorkRequestTimeout        = 2 * time.Minute
	MaxOPCRetryTokenBytes     = 64
	CreatedBy                 = "CreatedBy"
	OCIClusterAPIProvider     = "OCIClusterAPIProvider"
	ClusterResourceIdentifier = "ClusterResourceIdentifier"
)

// ErrNotFound is for simulation during testing, OCI SDK does not have a way
// to create Service Errors
var ErrNotFound = errors.New("not found")

// IsNotFound returns true if the given error indicates that a resource could
// not be found.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if err.Error() == ErrNotFound.Error() {
		return true
	}
	err = errors.Cause(err)
	serviceErr, ok := common.IsServiceError(err)
	return ok && serviceErr.GetHTTPStatusCode() == http.StatusNotFound
}

// AwaitNLBWorkRequest waits for the LB work request to either succeed, fail. See k8s.io/apimachinery/pkg/util/wait
func AwaitNLBWorkRequest(ctx context.Context, networkLoadBalancerClient nlb.NetworkLoadBalancerClient, workRequestId *string) (*networkloadbalancer.WorkRequest, error) {
	var wr *networkloadbalancer.WorkRequest
	immediate := true
	err := wait.PollUntilContextTimeout(ctx, WorkRequestPollInterval, WorkRequestTimeout, immediate, func(ctx context.Context) (done bool, err error) {
		twr, err := networkLoadBalancerClient.GetWorkRequest(ctx, networkloadbalancer.GetWorkRequestRequest{
			WorkRequestId: workRequestId,
		})
		if err != nil {
			return true, errors.Wrap(err, "failed create poll nlb workrequest")
		}
		switch twr.Status {
		case networkloadbalancer.OperationStatusSucceeded:
			wr = &twr.WorkRequest
			return true, nil
		case networkloadbalancer.OperationStatusFailed:
			return false, errors.Errorf("WorkRequest %s failed", *workRequestId)
		}
		return false, nil
	})
	return wr, err
}

// AwaitLBWorkRequest waits for the LBaaS work request to either succeed, fail. See k8s.io/apimachinery/pkg/util/wait
func AwaitLBWorkRequest(ctx context.Context, loadBalancerClient lb.LoadBalancerClient, workRequestId *string) (*loadbalancer.WorkRequest, error) {
	var wr *loadbalancer.WorkRequest
	immediate := true
	err := wait.PollUntilContextTimeout(ctx, WorkRequestPollInterval, WorkRequestTimeout, immediate, func(ctx context.Context) (done bool, err error) {
		twr, err := loadBalancerClient.GetWorkRequest(ctx, loadbalancer.GetWorkRequestRequest{
			WorkRequestId: workRequestId,
		})
		if err != nil {
			return true, errors.Wrap(err, "failed create poll lb workrequest")
		}
		switch twr.WorkRequest.LifecycleState {
		case loadbalancer.WorkRequestLifecycleStateSucceeded:
			wr = &twr.WorkRequest
			return true, nil
		case loadbalancer.WorkRequestLifecycleStateFailed:
			return false, errors.Errorf("WorkRequest %s failed", *workRequestId)
		}
		return false, nil
	})
	return wr, err
}

func truncateOPCRetryToken(str string) string {
	b := []byte(str)
	if len(b) > MaxOPCRetryTokenBytes {
		return string(b[0:MaxOPCRetryTokenBytes])
	}
	return str
}

// GetOPCRetryToken truncates the values input and returns the OPC retry token
func GetOPCRetryToken(format string, values ...interface{}) *string {
	return common.String(truncateOPCRetryToken(fmt.Sprintf(format, values...)))
}

// GetBaseLineOcpuOptimizationEnum iterates over the valid baseline OCPUs to validate the passed in value
func GetBaseLineOcpuOptimizationEnum(baseLineOcpuOptmimizationString string) (core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum, error) {
	for _, e := range core.GetLaunchInstanceShapeConfigDetailsBaselineOcpuUtilizationEnumValues() {
		if string(e) == baseLineOcpuOptmimizationString {
			return e, nil
		}
	}
	return "", errors.New("invalid baseline cpu optimization parameter")
}

// GetInstanceConfigBaseLineOcpuOptimizationEnum iterates over the valid baseline OCPUs to validate the passed in value
func GetInstanceConfigBaseLineOcpuOptimizationEnum(baseLineOcpuOptmimizationString string) (core.InstanceConfigurationLaunchInstanceShapeConfigDetailsBaselineOcpuUtilizationEnum, error) {
	for _, e := range core.GetInstanceConfigurationLaunchInstanceShapeConfigDetailsBaselineOcpuUtilizationEnumValues() {
		if string(e) == baseLineOcpuOptmimizationString {
			return e, nil
		}
	}
	return "", errors.New("invalid baseline cpu optimization parameter")
}

// GetDefaultClusterTags creates and returns a map of the default tags for all clusters
func GetDefaultClusterTags() map[string]string {
	tags := make(map[string]string)
	tags[CreatedBy] = OCIClusterAPIProvider
	return tags
}

// BuildClusterTags uses the default tags and adds the ClusterResourceUID tag
func BuildClusterTags(ClusterResourceUID string) map[string]string {
	tags := GetDefaultClusterTags()
	tags[ClusterResourceIdentifier] = ClusterResourceUID
	return tags
}

// DerefString returns the string value if the pointer isn't nil, otherwise returns empty string
func DerefString(s *string) string {
	if s != nil {
		return *s
	}

	return ""
}
