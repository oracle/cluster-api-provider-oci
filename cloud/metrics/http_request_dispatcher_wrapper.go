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
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// HttpRequestDispatcherWrapper is a wrapper around standard common.HTTPRequestDispatcher to handle
// metrics
type HttpRequestDispatcherWrapper struct {
	dispatcher common.HTTPRequestDispatcher
	region     string
}

// Do is wrapper implementation of common.HTTPRequestDispatcher Do method
func (wrapper HttpRequestDispatcherWrapper) Do(req *http.Request) (*http.Response, error) {
	t := time.Now()
	resp, err := wrapper.dispatcher.Do(req)
	defer func() {
		// taken from https://docs.oracle.com/en-us/iaas/Content/API/Concepts/usingapi.htm
		// a URL consists of a version string and then a resource
		urlSplit := strings.Split(req.URL.Path, "/")
		if len(urlSplit) < 2 {
			return
		}
		resource := urlSplit[2]
		IncRequestCounter(err, resource, req.Method, wrapper.region, resp)
		ObserverRequestDuration(resource, req.Method, wrapper.region, time.Since(t))
	}()
	return resp, err
}

// NewHttpRequestDispatcherWrapper creates a new instance of HttpRequestDispatcherWrapper
func NewHttpRequestDispatcherWrapper(dispatcher common.HTTPRequestDispatcher, region string) HttpRequestDispatcherWrapper {
	return HttpRequestDispatcherWrapper{
		dispatcher: dispatcher,
		region:     region,
	}
}
