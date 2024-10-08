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

package workrequests

import (
	"context"

	"github.com/oracle/oci-go-sdk/v65/workrequests"
)

type Client interface {
	//WorkRequest
	ListWorkRequests(ctx context.Context, request workrequests.ListWorkRequestsRequest) (response workrequests.ListWorkRequestsResponse, err error)
	ListWorkRequestErrors(ctx context.Context, request workrequests.ListWorkRequestErrorsRequest) (response workrequests.ListWorkRequestErrorsResponse, err error)
}
