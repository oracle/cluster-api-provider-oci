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

package compute

import (
	"context"
	"net/http"

	"github.com/oracle/cluster-api-provider-oci/cloud/metrics"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ociRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oci_requests_total",
			Help: "OCI API requests total.",
		},
		[]string{"resource", "code", "verb"},
	)
)

type ClientWrapper struct {
	client ComputeClient
}

func NewClientWrapper(computeClient ComputeClient) ClientWrapper {
	return ClientWrapper{client: computeClient}
}
func (wrapper ClientWrapper) LaunchInstance(ctx context.Context, request core.LaunchInstanceRequest) (response core.LaunchInstanceResponse, err error) {
	resp, err := wrapper.client.LaunchInstance(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, "compute", "launch", httpResponse)
	}()
	return resp, err
}
func (wrapper ClientWrapper) TerminateInstance(ctx context.Context, request core.TerminateInstanceRequest) (response core.TerminateInstanceResponse, err error) {
	resp, err := wrapper.client.TerminateInstance(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, "compute", "delete", httpResponse)
	}()
	return resp, err
}
func (wrapper ClientWrapper) GetInstance(ctx context.Context, request core.GetInstanceRequest) (response core.GetInstanceResponse, err error) {
	resp, err := wrapper.client.GetInstance(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, "compute", "get", httpResponse)
	}()
	return resp, err
}
func (wrapper ClientWrapper) ListInstances(ctx context.Context, request core.ListInstancesRequest) (response core.ListInstancesResponse, err error) {
	resp, err := wrapper.client.ListInstances(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, "compute", "list", httpResponse)
	}()
	return resp, err
}
func (wrapper ClientWrapper) AttachVnic(ctx context.Context, request core.AttachVnicRequest) (response core.AttachVnicResponse, err error) {
	resp, err := wrapper.client.AttachVnic(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, "compute", "attachvnic", httpResponse)
	}()
	return resp, err
}
func (wrapper ClientWrapper) ListVnicAttachments(ctx context.Context, request core.ListVnicAttachmentsRequest) (response core.ListVnicAttachmentsResponse, err error) {
	resp, err := wrapper.client.ListVnicAttachments(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, "compute", "listvnicattachments", httpResponse)
	}()
	return resp, err
}
