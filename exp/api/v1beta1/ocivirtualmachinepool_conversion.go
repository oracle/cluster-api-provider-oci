/*
Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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
	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *OCIVirtualMachinePool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIVirtualMachinePool)

	if err := Convert_v1beta1_OCIVirtualMachinePool_To_v1beta3_OCIVirtualMachinePool(src, dst, nil); err != nil {
		return err
	}

	restored := &v1beta3.OCIVirtualMachinePool{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	if restored.Status.Initialization.Provisioned != nil && !*restored.Status.Initialization.Provisioned && !src.Status.Ready {
		dst.Status.Initialization.Provisioned = restored.Status.Initialization.Provisioned
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)

	return nil
}

func (r *OCIVirtualMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIVirtualMachinePool)

	if err := Convert_v1beta3_OCIVirtualMachinePool_To_v1beta1_OCIVirtualMachinePool(src, r, nil); err != nil {
		return err
	}

	return utilconversion.MarshalDataUnsafeNoCopy(src, r)
}

func (src *OCIVirtualMachinePoolList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIVirtualMachinePoolList)
	return Convert_v1beta1_OCIVirtualMachinePoolList_To_v1beta3_OCIVirtualMachinePoolList(src, dst, nil)
}

func (r *OCIVirtualMachinePoolList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIVirtualMachinePoolList)
	return Convert_v1beta3_OCIVirtualMachinePoolList_To_v1beta1_OCIVirtualMachinePoolList(src, r, nil)
}
