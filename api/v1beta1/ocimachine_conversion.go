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

// ConvertTo converts the v1beta1 OCIMachine receiver to a v1beta2 OCIMachine.
func (src *OCIMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIMachine)
	if err := Convert_v1beta1_OCIMachine_To_v1beta2_OCIMachine(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &v1beta2.OCIMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	if dst.Spec.InstanceSourceViaImageDetails != nil && restored.Spec.InstanceSourceViaImageDetails != nil {
		dst.Spec.InstanceSourceViaImageDetails.ImageLookup = restored.Spec.InstanceSourceViaImageDetails.ImageLookup
	}
	return nil
}

// ConvertFrom converts the v1beta2 OCIMachine to a v1beta1 OCIMachine.
func (dst *OCIMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIMachine)
	if err := Convert_v1beta2_OCIMachine_To_v1beta1_OCIMachine(src, dst, nil); err != nil {
		return err
	}
	// Preserve Hub data on down-conversion except for metadata.
	return utilconversion.MarshalData(src, dst)
}

// ConvertTo converts the v1beta1 OCIMachineList receiver to a v1beta2 OCIMachineList.
func (src *OCIMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIMachineList)
	return Convert_v1beta1_OCIMachineList_To_v1beta2_OCIMachineList(src, dst, nil)
}

// ConvertFrom converts the v1beta2 OCIMachineList to a v1beta1 OCIMachineList.
func (dst *OCIMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIMachineList)

	return Convert_v1beta2_OCIMachineList_To_v1beta1_OCIMachineList(src, dst, nil)
}

// ConvertTo converts the v1beta1 OCIMachineTemplate receiver to a v1beta2 OCIMachineTemplate.
func (r *OCIMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIMachineTemplate)

	if err := Convert_v1beta1_OCIMachineTemplate_To_v1beta2_OCIMachineTemplate(r, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &v1beta2.OCIMachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(r, restored); err != nil || !ok {
		return err
	}

	if dst.Spec.Template.Spec.InstanceSourceViaImageDetails != nil && restored.Spec.Template.Spec.InstanceSourceViaImageDetails != nil {
		dst.Spec.Template.Spec.InstanceSourceViaImageDetails.ImageLookup = restored.Spec.Template.Spec.InstanceSourceViaImageDetails.ImageLookup
	}
	return nil
}

// ConvertFrom converts the v1beta2 OCIMachineTemplate receiver to a v1beta1 OCIMachineTemplate.
func (r *OCIMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIMachineTemplate)

	if err := Convert_v1beta2_OCIMachineTemplate_To_v1beta1_OCIMachineTemplate(src, r, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, r); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts the v1beta1 OCIMachineTemplateList receiver to a v1beta2 OCIMachineTemplateList.
func (src *OCIMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIMachineTemplateList)
	return Convert_v1beta1_OCIMachineTemplateList_To_v1beta2_OCIMachineTemplateList(src, dst, nil)
}

// ConvertFrom converts the v1beta2 OCIMachineTemplateList to a v1beta1 OCIMachineTemplateList.
func (dst *OCIMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIMachineTemplateList)

	return Convert_v1beta2_OCIMachineTemplateList_To_v1beta1_OCIMachineTemplateList(src, dst, nil)
}
