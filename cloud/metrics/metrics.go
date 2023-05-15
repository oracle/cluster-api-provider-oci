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

	"github.com/oracle/oci-go-sdk/v65/common"
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

type verb string

const (
	Get    string = "get"
	List   string = "list"
	Create string = "create"
	Update string = "update"
	Delete string = "delete"
)

func IncRequestCounter(err error, service string, verb string, response *http.Response) {
	statusCode := 999
	if err != nil {
		if serviceErr, ok := err.(common.ServiceError); ok {
			statusCode = serviceErr.GetHTTPStatusCode()
		}
	} else {
		statusCode = response.StatusCode
	}

	ociRequestCounter.With(prometheus.Labels{
		"service": service,
		"verb":    verb,
		"code":    strconv.Itoa(statusCode),
	}).Inc()
}

func init() {
	prometheus.MustRegister(ociRequestCounter)
}
