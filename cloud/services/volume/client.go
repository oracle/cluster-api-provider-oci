/*
 Copyright (c) 2025 Oracle and/or its affiliates.

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

package volume

import (
	"context"

	"github.com/oracle/oci-go-sdk/v65/core"
)

type BlockVolumeClient interface {
	CreateVolume(ctx context.Context, request core.CreateVolumeRequest) (response core.CreateVolumeResponse, err error)
	DeleteVolume(ctx context.Context, request core.DeleteVolumeRequest) (response core.DeleteVolumeResponse, err error)
	GetVolume(ctx context.Context, request core.GetVolumeRequest) (response core.GetVolumeResponse, err error)
	ListVolumes(ctx context.Context, request core.ListVolumesRequest) (response core.ListVolumesResponse, err error)
	// UpdateVolume(ctx context.Context, request core.UpdateVolumeRequest) (response core.UpdateVolumeResponse, err error)
}
