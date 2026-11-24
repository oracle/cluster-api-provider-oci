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
	"github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *OCIClusterIdentity) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterIdentity)
	if err := Convert_v1beta1_OCIClusterIdentity_To_v1beta3_OCIClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	restored := &v1beta3.OCIClusterIdentity{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)

	return nil
}

func (dst *OCIClusterIdentity) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIClusterIdentity)

	if err := Convert_v1beta3_OCIClusterIdentity_To_v1beta1_OCIClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIClusterIdentityList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterIdentityList)
	return Convert_v1beta1_OCIClusterIdentityList_To_v1beta3_OCIClusterIdentityList(src, dst, nil)
}

func (dst *OCIClusterIdentityList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIClusterIdentityList)
	return Convert_v1beta3_OCIClusterIdentityList_To_v1beta1_OCIClusterIdentityList(src, dst, nil)
}
