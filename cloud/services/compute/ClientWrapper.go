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
	"time"

	"github.com/oracle/cluster-api-provider-oci/cloud/metrics"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	Service             = "compute"
	Launch              = "launch"
	AttachVnic          = "attachvnic"
	ListVnicAttachments = "listvnicattachments"
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
	region string
}

func NewClientWrapper(computeClient ComputeClient, region string) ClientWrapper {
	return ClientWrapper{client: computeClient, region: region}
}
func (wrapper ClientWrapper) LaunchInstance(ctx context.Context, request core.LaunchInstanceRequest) (response core.LaunchInstanceResponse, err error) {
	t := time.Now()
	resp, err := wrapper.client.LaunchInstance(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, Service, Launch, wrapper.region, httpResponse)
	}()
	defer func() {
		metrics.ObserverRequestDuration(Service, Launch, wrapper.region, time.Since(t))
	}()
	return resp, err
}
func (wrapper ClientWrapper) TerminateInstance(ctx context.Context, request core.TerminateInstanceRequest) (response core.TerminateInstanceResponse, err error) {
	t := time.Now()
	resp, err := wrapper.client.TerminateInstance(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, Service, metrics.Delete, wrapper.region, httpResponse)
	}()
	defer func() {
		metrics.ObserverRequestDuration(Service, metrics.Delete, wrapper.region, time.Since(t))
	}()
	return resp, err
}
func (wrapper ClientWrapper) GetInstance(ctx context.Context, request core.GetInstanceRequest) (response core.GetInstanceResponse, err error) {
	t := time.Now()
	resp, err := wrapper.client.GetInstance(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, Service, metrics.Get, wrapper.region, httpResponse)
	}()
	defer func() {
		metrics.ObserverRequestDuration(Service, metrics.Get, wrapper.region, time.Since(t))
	}()
	return resp, err
}
func (wrapper ClientWrapper) ListInstances(ctx context.Context, request core.ListInstancesRequest) (response core.ListInstancesResponse, err error) {
	t := time.Now()
	resp, err := wrapper.client.ListInstances(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, Service, metrics.List, wrapper.region, httpResponse)
	}()
	defer func() {
		metrics.ObserverRequestDuration(Service, metrics.List, wrapper.region, time.Since(t))
	}()
	return resp, err
}
func (wrapper ClientWrapper) AttachVnic(ctx context.Context, request core.AttachVnicRequest) (response core.AttachVnicResponse, err error) {
	t := time.Now()
	resp, err := wrapper.client.AttachVnic(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, Service, AttachVnic, wrapper.region, httpResponse)
	}()
	defer func() {
		metrics.ObserverRequestDuration(Service, AttachVnic, wrapper.region, time.Since(t))
	}()
	return resp, err
}
func (wrapper ClientWrapper) ListVnicAttachments(ctx context.Context, request core.ListVnicAttachmentsRequest) (response core.ListVnicAttachmentsResponse, err error) {
	t := time.Now()
	resp, err := wrapper.client.ListVnicAttachments(ctx, request)
	defer func() {
		var httpResponse *http.Response
		if err == nil {
			httpResponse = response.RawResponse
		}
		metrics.IncRequestCounter(err, Service, ListVnicAttachments, wrapper.region, httpResponse)
	}()
	defer func() {
		metrics.ObserverRequestDuration(Service, ListVnicAttachments, wrapper.region, time.Since(t))
	}()
	return resp, err
}
