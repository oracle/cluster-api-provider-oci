/*
Copyright (c) 2026 Oracle and/or its affiliates.

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

package v1beta2

import (
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func Convert_v1beta2_OCIMachinePoolStatus_To_v1beta3_OCIMachinePoolStatus(in *OCIMachinePoolStatus, out *v1beta3.OCIMachinePoolStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_OCIMachinePoolStatus_To_v1beta3_OCIMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = infrastructurev1beta2.ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	return nil
}

func Convert_v1beta3_OCIMachinePoolStatus_To_v1beta2_OCIMachinePoolStatus(in *v1beta3.OCIMachinePoolStatus, out *OCIMachinePoolStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIMachinePoolStatus_To_v1beta2_OCIMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = infrastructurev1beta2.ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	return nil
}

func Convert_v1beta2_OCIManagedMachinePoolStatus_To_v1beta3_OCIManagedMachinePoolStatus(in *OCIManagedMachinePoolStatus, out *v1beta3.OCIManagedMachinePoolStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_OCIManagedMachinePoolStatus_To_v1beta3_OCIManagedMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = infrastructurev1beta2.ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	return nil
}

func Convert_v1beta3_OCIManagedMachinePoolStatus_To_v1beta2_OCIManagedMachinePoolStatus(in *v1beta3.OCIManagedMachinePoolStatus, out *OCIManagedMachinePoolStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIManagedMachinePoolStatus_To_v1beta2_OCIManagedMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = infrastructurev1beta2.ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	return nil
}

func Convert_v1beta2_OCIVirtualMachinePoolStatus_To_v1beta3_OCIVirtualMachinePoolStatus(in *OCIVirtualMachinePoolStatus, out *v1beta3.OCIVirtualMachinePoolStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_OCIVirtualMachinePoolStatus_To_v1beta3_OCIVirtualMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = infrastructurev1beta2.ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	if in.FailureReason != nil {
		v := string(*in.FailureReason)
		out.FailureReason = &v
	}
	return nil
}

func Convert_v1beta3_OCIVirtualMachinePoolStatus_To_v1beta2_OCIVirtualMachinePoolStatus(in *v1beta3.OCIVirtualMachinePoolStatus, out *OCIVirtualMachinePoolStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIVirtualMachinePoolStatus_To_v1beta2_OCIVirtualMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = infrastructurev1beta2.ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	if in.FailureReason != nil {
		v := infrastructurev1beta2.MachinePoolStatusFailure(*in.FailureReason)
		out.FailureReason = &v
	}
	return nil
}

func Convert_v1beta2_OCIMachinePoolMachineStatus_To_v1beta3_OCIMachinePoolMachineStatus(in *OCIMachinePoolMachineStatus, out *v1beta3.OCIMachinePoolMachineStatus, _ apiconversion.Scope) error {
	out.Conditions = infrastructurev1beta2.ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	return nil
}

func Convert_v1beta3_OCIMachinePoolMachineStatus_To_v1beta2_OCIMachinePoolMachineStatus(in *v1beta3.OCIMachinePoolMachineStatus, out *OCIMachinePoolMachineStatus, _ apiconversion.Scope) error {
	out.Conditions = infrastructurev1beta2.ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	return nil
}

func Convert_v1beta3_OCIManagedMachinePoolTemplateResource_To_v1beta2_OCIManagedMachinePoolTemplateResource(in *v1beta3.OCIManagedMachinePoolTemplateResource, out *OCIManagedMachinePoolTemplateResource, s apiconversion.Scope) error {
	return autoConvert_v1beta3_OCIManagedMachinePoolTemplateResource_To_v1beta2_OCIManagedMachinePoolTemplateResource(in, out, s)
}

func (src *OCIMachinePool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachinePool)
	if err := Convert_v1beta2_OCIMachinePool_To_v1beta3_OCIMachinePool(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIMachinePool{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	if restored.Status.Initialization.Provisioned != nil && !*restored.Status.Initialization.Provisioned && !src.Status.Ready {
		dst.Status.Initialization.Provisioned = restored.Status.Initialization.Provisioned
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)
	return nil
}

func (dst *OCIMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachinePool)
	if err := Convert_v1beta3_OCIMachinePool_To_v1beta2_OCIMachinePool(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIMachinePoolList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachinePoolList)
	return Convert_v1beta2_OCIMachinePoolList_To_v1beta3_OCIMachinePoolList(src, dst, nil)
}

func (dst *OCIMachinePoolList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachinePoolList)
	return Convert_v1beta3_OCIMachinePoolList_To_v1beta2_OCIMachinePoolList(src, dst, nil)
}

func (src *OCIManagedMachinePool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedMachinePool)
	if err := Convert_v1beta2_OCIManagedMachinePool_To_v1beta3_OCIManagedMachinePool(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIManagedMachinePool{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	if restored.Status.Initialization.Provisioned != nil && !*restored.Status.Initialization.Provisioned && !src.Status.Ready {
		dst.Status.Initialization.Provisioned = restored.Status.Initialization.Provisioned
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)
	return nil
}

func (dst *OCIManagedMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedMachinePool)
	if err := Convert_v1beta3_OCIManagedMachinePool_To_v1beta2_OCIManagedMachinePool(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIManagedMachinePoolList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedMachinePoolList)
	return Convert_v1beta2_OCIManagedMachinePoolList_To_v1beta3_OCIManagedMachinePoolList(src, dst, nil)
}

func (dst *OCIManagedMachinePoolList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedMachinePoolList)
	return Convert_v1beta3_OCIManagedMachinePoolList_To_v1beta2_OCIManagedMachinePoolList(src, dst, nil)
}

func (src *OCIManagedMachinePoolTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedMachinePoolTemplate)
	if err := Convert_v1beta2_OCIManagedMachinePoolTemplate_To_v1beta3_OCIManagedMachinePoolTemplate(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIManagedMachinePoolTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta
	return nil
}

func (dst *OCIManagedMachinePoolTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedMachinePoolTemplate)
	if err := Convert_v1beta3_OCIManagedMachinePoolTemplate_To_v1beta2_OCIManagedMachinePoolTemplate(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIManagedMachinePoolTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedMachinePoolTemplateList)
	return Convert_v1beta2_OCIManagedMachinePoolTemplateList_To_v1beta3_OCIManagedMachinePoolTemplateList(src, dst, nil)
}

func (dst *OCIManagedMachinePoolTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedMachinePoolTemplateList)
	return Convert_v1beta3_OCIManagedMachinePoolTemplateList_To_v1beta2_OCIManagedMachinePoolTemplateList(src, dst, nil)
}

func (src *OCIVirtualMachinePool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIVirtualMachinePool)
	if err := Convert_v1beta2_OCIVirtualMachinePool_To_v1beta3_OCIVirtualMachinePool(src, dst, nil); err != nil {
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

func (dst *OCIVirtualMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIVirtualMachinePool)
	if err := Convert_v1beta3_OCIVirtualMachinePool_To_v1beta2_OCIVirtualMachinePool(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIVirtualMachinePoolList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIVirtualMachinePoolList)
	return Convert_v1beta2_OCIVirtualMachinePoolList_To_v1beta3_OCIVirtualMachinePoolList(src, dst, nil)
}

func (dst *OCIVirtualMachinePoolList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIVirtualMachinePoolList)
	return Convert_v1beta3_OCIVirtualMachinePoolList_To_v1beta2_OCIVirtualMachinePoolList(src, dst, nil)
}

func (src *OCIMachinePoolMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachinePoolMachine)
	if err := Convert_v1beta2_OCIMachinePoolMachine_To_v1beta3_OCIMachinePoolMachine(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIMachinePoolMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	if restored.Status.Initialization.Provisioned != nil && !*restored.Status.Initialization.Provisioned && !src.Status.Ready {
		dst.Status.Initialization.Provisioned = restored.Status.Initialization.Provisioned
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)
	return nil
}

func (dst *OCIMachinePoolMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachinePoolMachine)
	if err := Convert_v1beta3_OCIMachinePoolMachine_To_v1beta2_OCIMachinePoolMachine(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIMachinePoolMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachinePoolMachineList)
	return Convert_v1beta2_OCIMachinePoolMachineList_To_v1beta3_OCIMachinePoolMachineList(src, dst, nil)
}

func (dst *OCIMachinePoolMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachinePoolMachineList)
	return Convert_v1beta3_OCIMachinePoolMachineList_To_v1beta2_OCIMachinePoolMachineList(src, dst, nil)
}

func restoreMetaConditions(dst, restored []metav1.Condition) {
	if len(dst) == 0 || len(restored) == 0 {
		return
	}
	byType := make(map[string]metav1.Condition, len(restored))
	for _, c := range restored {
		byType[c.Type] = c
	}
	for i := range dst {
		if prev, ok := byType[dst[i].Type]; ok {
			dst[i].ObservedGeneration = prev.ObservedGeneration
		}
	}
}
