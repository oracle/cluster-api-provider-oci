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

func (src *OCIManagedMachinePool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedMachinePool)

	if err := Convert_v1beta1_OCIManagedMachinePool_To_v1beta3_OCIManagedMachinePool(src, dst, nil); err != nil {
		return err
	}

	restored := &v1beta3.OCIManagedMachinePool{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.NodePoolCyclingDetails = restored.Spec.NodePoolCyclingDetails
	if restored.Status.Initialization.Provisioned != nil && !*restored.Status.Initialization.Provisioned && !src.Status.Ready {
		dst.Status.Initialization.Provisioned = restored.Status.Initialization.Provisioned
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)

	return nil
}

func (r *OCIManagedMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedMachinePool)

	if err := Convert_v1beta3_OCIManagedMachinePool_To_v1beta1_OCIManagedMachinePool(src, r, nil); err != nil {
		return err
	}

	return utilconversion.MarshalDataUnsafeNoCopy(src, r)
}

func (src *OCIManagedMachinePoolList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedMachinePoolList)
	return Convert_v1beta1_OCIManagedMachinePoolList_To_v1beta3_OCIManagedMachinePoolList(src, dst, nil)
}

func (r *OCIManagedMachinePoolList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedMachinePoolList)
	return Convert_v1beta3_OCIManagedMachinePoolList_To_v1beta1_OCIManagedMachinePoolList(src, r, nil)
}
