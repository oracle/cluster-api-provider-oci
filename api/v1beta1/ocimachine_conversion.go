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

func (src *OCIMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachine)
	if err := Convert_v1beta1_OCIMachine_To_v1beta3_OCIMachine(src, dst, nil); err != nil {
		return err
	}

	restored := &v1beta3.OCIMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	if restored.Status.Initialization.Provisioned != nil && !*restored.Status.Initialization.Provisioned && !src.Status.Ready {
		dst.Status.Initialization.Provisioned = restored.Status.Initialization.Provisioned
	}
	dst.Status.FailureDomain = restored.Status.FailureDomain
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)

	return nil
}

func (dst *OCIMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachine)
	if err := Convert_v1beta3_OCIMachine_To_v1beta1_OCIMachine(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachineList)
	return Convert_v1beta1_OCIMachineList_To_v1beta3_OCIMachineList(src, dst, nil)
}

func (dst *OCIMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachineList)
	return Convert_v1beta3_OCIMachineList_To_v1beta1_OCIMachineList(src, dst, nil)
}

func (r *OCIMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachineTemplate)

	if err := Convert_v1beta1_OCIMachineTemplate_To_v1beta3_OCIMachineTemplate(r, dst, nil); err != nil {
		return err
	}

	restored := &v1beta3.OCIMachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(r, restored); err != nil || !ok {
		return err
	}
	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta

	return nil
}

func (r *OCIMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachineTemplate)

	if err := Convert_v1beta3_OCIMachineTemplate_To_v1beta1_OCIMachineTemplate(src, r, nil); err != nil {
		return err
	}

	return utilconversion.MarshalDataUnsafeNoCopy(src, r)
}

func (src *OCIMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachineTemplateList)
	return Convert_v1beta1_OCIMachineTemplateList_To_v1beta3_OCIMachineTemplateList(src, dst, nil)
}

func (dst *OCIMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachineTemplateList)
	return Convert_v1beta3_OCIMachineTemplateList_To_v1beta1_OCIMachineTemplateList(src, dst, nil)
}
