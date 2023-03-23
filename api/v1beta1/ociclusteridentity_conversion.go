/*
Copyright (c) 2022, Oracle and/or its affiliates.

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

package v1beta1

import (
	"github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts the v1beta1 OCIClusterIdentity receiver to a v1beta2 OCIClusterIdentity.
func (src *OCIClusterIdentity) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIClusterIdentity)
	if err := Convert_v1beta1_OCIClusterIdentity_To_v1beta2_OCIClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &v1beta2.OCIClusterIdentity{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	return nil
}

// ConvertFrom converts the v1beta2 OCIClusterIdentity to a v1beta1 OCIClusterIdentity.
func (dst *OCIClusterIdentity) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIClusterIdentity)

	if err := Convert_v1beta2_OCIClusterIdentity_To_v1beta1_OCIClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion except for metadata.
	return utilconversion.MarshalData(src, dst)
}
