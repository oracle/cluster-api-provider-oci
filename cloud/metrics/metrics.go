/*
Copyright (c) 2023 Oracle and/or its affiliates.

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

package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type verb string

const (
	SubSystemOCI     = "oci"
	OCIRequestsTotal = "requests_total"
	Duration         = "request_duration"
	Resource         = "resource"
	StatusCode       = "status_code"
	Operation        = "operation"

	Region        = "region"
	Get    string = "get"
	List   string = "list"
	Create string = "create"
	Update string = "update"
	Delete string = "delete"
)

var (
	ociRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubSystemOCI,
			Name:      OCIRequestsTotal,
			Help:      "OCI API requests total.",
		},
		[]string{Resource, StatusCode, Operation, Region},
	)
	ociRequestDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: SubSystemOCI,
		Name:      Duration,
		Help:      "Duration/Latency of HTTP requests to OCI",
	}, []string{Resource, Operation, Region})
)

// IncRequestCounter increments the request count metric for the given resource.
// Unknown errors from request dispatcher will have response code of 999
func IncRequestCounter(err error, resource string, operation string, region string, response *http.Response) {
	statusCode := 999
	if err == nil {
		statusCode = response.StatusCode
	}
	ociRequestCounter.With(prometheus.Labels{
		Resource:   resource,
		Operation:  operation,
		StatusCode: strconv.Itoa(statusCode),
		Region:     region,
	}).Inc()
}

// ObserverRequestDuration observes the request duration for the partcular OCI request
func ObserverRequestDuration(resource string, operation string, region string, duration time.Duration) {
	ociRequestDurationSeconds.With(prometheus.Labels{
		Resource:  resource,
		Operation: operation,
		Region:    region,
	}).Observe(duration.Seconds())
}

func init() {
	metrics.Registry.MustRegister(ociRequestCounter)
	metrics.Registry.MustRegister(ociRequestDurationSeconds)
}
