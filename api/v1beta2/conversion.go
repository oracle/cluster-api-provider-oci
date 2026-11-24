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
	"maps"
	"regexp"
	"sort"
	"strings"

	"github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func convertFailureDomainsToV1Beta3(in FailureDomains) []clusterv1.FailureDomain {
	if len(in) == 0 {
		return nil
	}
	names := make([]string, 0, len(in))
	for name := range in {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]clusterv1.FailureDomain, 0, len(names))
	for _, name := range names {
		spec := in[name]
		var attrs map[string]string
		if spec.Attributes != nil {
			attrs = maps.Clone(spec.Attributes)
		}
		out = append(out, clusterv1.FailureDomain{
			Name:         name,
			ControlPlane: ptr.To(spec.ControlPlane),
			Attributes:   attrs,
		})
	}
	return out
}

func convertFailureDomainsFromV1Beta3(in []clusterv1.FailureDomain) FailureDomains {
	if len(in) == 0 {
		return nil
	}
	out := make(FailureDomains, len(in))
	for _, fd := range in {
		var attrs map[string]string
		if fd.Attributes != nil {
			attrs = maps.Clone(fd.Attributes)
		}
		out[fd.Name] = FailureDomainSpec{
			ControlPlane: ptr.Deref(fd.ControlPlane, false),
			Attributes:   attrs,
		}
	}
	return out
}

var validConditionReasonRegex = regexp.MustCompile(`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`)

func sanitizeReasonToken(raw string) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == ',' || ch == ':' {
			b.WriteByte(ch)
		}
	}
	s := b.String()
	// Trim leading non-letters.
	for len(s) > 0 && !((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) {
		s = s[1:]
	}
	if len(s) > 1024 {
		s = s[:1024]
	}
	// Trim trailing commas or colons.
	for len(s) > 0 && (s[len(s)-1] == ',' || s[len(s)-1] == ':') {
		s = s[:len(s)-1]
	}
	if len(s) > 0 && validConditionReasonRegex.MatchString(s) {
		return s
	}
	return ""
}

// sanitizeConditionReason ensures converted legacy condition reasons satisfy metav1.Condition OpenAPI validation.
func sanitizeConditionReason(reason string, condType ConditionType, status corev1.ConditionStatus) string {
	if len(reason) > 0 && len(reason) <= 1024 && validConditionReasonRegex.MatchString(reason) {
		return reason
	}
	if candidate := sanitizeReasonToken(reason); candidate != "" {
		return candidate
	}
	if candidate := sanitizeReasonToken(string(condType)); candidate != "" {
		return candidate
	}
	switch status {
	case corev1.ConditionTrue:
		return "Succeeded"
	case corev1.ConditionFalse:
		return "Failed"
	default:
		return "Unknown"
	}
}

// Convert_v1beta2_Condition_To_v1_Condition converts legacy Condition to metav1.Condition.
func Convert_v1beta2_Condition_To_v1_Condition(in *Condition, out *metav1.Condition, _ apiconversion.Scope) error {
	out.Type = string(in.Type)
	out.Status = metav1.ConditionStatus(in.Status)
	out.LastTransitionTime = in.LastTransitionTime
	out.Reason = sanitizeConditionReason(in.Reason, in.Type, in.Status)
	out.Message = in.Message
	return nil
}

// Convert_v1_Condition_To_v1beta2_Condition converts metav1.Condition to legacy Condition.
func Convert_v1_Condition_To_v1beta2_Condition(in *metav1.Condition, out *Condition, _ apiconversion.Scope) error {
	out.Type = ConditionType(in.Type)
	out.Status = corev1.ConditionStatus(in.Status)
	out.LastTransitionTime = in.LastTransitionTime
	out.Reason = in.Reason
	out.Message = in.Message
	return nil
}

// ConvertConditionsToMeta converts legacy v1beta2 Conditions to standard metav1.Condition slice.
func ConvertConditionsToMeta(in Conditions) []metav1.Condition {
	if in == nil {
		return nil
	}
	out := make([]metav1.Condition, 0, len(in))
	for _, c := range in {
		out = append(out, metav1.Condition{
			Type:               string(c.Type),
			Status:             metav1.ConditionStatus(c.Status),
			LastTransitionTime: c.LastTransitionTime,
			Reason:             sanitizeConditionReason(c.Reason, c.Type, c.Status),
			Message:            c.Message,
		})
	}
	return out
}

// ConvertConditionsFromMeta converts standard metav1.Condition slice to legacy v1beta2 Conditions.
func ConvertConditionsFromMeta(in []metav1.Condition) Conditions {
	if in == nil {
		return nil
	}
	out := make(Conditions, 0, len(in))
	for _, c := range in {
		out = append(out, Condition{
			Type:               ConditionType(c.Type),
			Status:             corev1.ConditionStatus(c.Status),
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}
	return out
}

func Convert_v1beta2_OCIClusterStatus_To_v1beta3_OCIClusterStatus(in *OCIClusterStatus, out *v1beta3.OCIClusterStatus, s apiconversion.Scope) error {
	out.FailureDomains = convertFailureDomainsToV1Beta3(in.FailureDomains)
	out.Conditions = ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	return nil
}

func Convert_v1beta3_OCIClusterStatus_To_v1beta2_OCIClusterStatus(in *v1beta3.OCIClusterStatus, out *OCIClusterStatus, s apiconversion.Scope) error {
	out.FailureDomains = convertFailureDomainsFromV1Beta3(in.FailureDomains)
	out.Conditions = ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	return nil
}

func Convert_v1beta2_OCIManagedClusterStatus_To_v1beta3_OCIManagedClusterStatus(in *OCIManagedClusterStatus, out *v1beta3.OCIManagedClusterStatus, s apiconversion.Scope) error {
	out.FailureDomains = convertFailureDomainsToV1Beta3(in.FailureDomains)
	out.Conditions = ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	return nil
}

func Convert_v1beta3_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus(in *v1beta3.OCIManagedClusterStatus, out *OCIManagedClusterStatus, s apiconversion.Scope) error {
	out.FailureDomains = convertFailureDomainsFromV1Beta3(in.FailureDomains)
	out.Conditions = ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	return nil
}

func Convert_v1beta2_OCIManagedControlPlaneStatus_To_v1beta3_OCIManagedControlPlaneStatus(in *OCIManagedControlPlaneStatus, out *v1beta3.OCIManagedControlPlaneStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_OCIManagedControlPlaneStatus_To_v1beta3_OCIManagedControlPlaneStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = ConvertConditionsToMeta(in.Conditions)
	if in.Initialized || in.Ready {
		out.Initialization.ControlPlaneInitialized = ptr.To(true)
	}
	if in.Version != nil && *in.Version != "" {
		out.Versions = []clusterv1.StatusVersion{{Version: *in.Version}}
	}
	return nil
}

func Convert_v1beta3_OCIManagedControlPlaneStatus_To_v1beta2_OCIManagedControlPlaneStatus(in *v1beta3.OCIManagedControlPlaneStatus, out *OCIManagedControlPlaneStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIManagedControlPlaneStatus_To_v1beta2_OCIManagedControlPlaneStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = ConvertConditionsFromMeta(in.Conditions)
	initialized := ptr.Deref(in.Initialization.ControlPlaneInitialized, false)
	out.Initialized = initialized
	out.Ready = initialized
	if len(in.Versions) > 0 {
		v := in.Versions[0].Version
		out.Version = &v
	}
	return nil
}

func Convert_v1beta2_OCIMachineStatus_To_v1beta3_OCIMachineStatus(in *OCIMachineStatus, out *v1beta3.OCIMachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_OCIMachineStatus_To_v1beta3_OCIMachineStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	return nil
}

func Convert_v1beta3_OCIMachineStatus_To_v1beta2_OCIMachineStatus(in *v1beta3.OCIMachineStatus, out *OCIMachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIMachineStatus_To_v1beta2_OCIMachineStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	return nil
}

func Convert_v1beta2_OCIClusterIdentityStatus_To_v1beta3_OCIClusterIdentityStatus(in *OCIClusterIdentityStatus, out *v1beta3.OCIClusterIdentityStatus, _ apiconversion.Scope) error {
	out.Conditions = ConvertConditionsToMeta(in.Conditions)
	return nil
}

func Convert_v1beta3_OCIClusterIdentityStatus_To_v1beta2_OCIClusterIdentityStatus(in *v1beta3.OCIClusterIdentityStatus, out *OCIClusterIdentityStatus, _ apiconversion.Scope) error {
	out.Conditions = ConvertConditionsFromMeta(in.Conditions)
	return nil
}

func Convert_v1beta3_OCIClusterTemplateResource_To_v1beta2_OCIClusterTemplateResource(in *v1beta3.OCIClusterTemplateResource, out *OCIClusterTemplateResource, s apiconversion.Scope) error {
	return autoConvert_v1beta3_OCIClusterTemplateResource_To_v1beta2_OCIClusterTemplateResource(in, out, s)
}

func Convert_v1beta3_OCIMachineTemplateResource_To_v1beta2_OCIMachineTemplateResource(in *v1beta3.OCIMachineTemplateResource, out *OCIMachineTemplateResource, s apiconversion.Scope) error {
	return autoConvert_v1beta3_OCIMachineTemplateResource_To_v1beta2_OCIMachineTemplateResource(in, out, s)
}

func Convert_v1beta3_OCIManagedClusterTemplateResource_To_v1beta2_OCIManagedClusterTemplateResource(in *v1beta3.OCIManagedClusterTemplateResource, out *OCIManagedClusterTemplateResource, s apiconversion.Scope) error {
	return autoConvert_v1beta3_OCIManagedClusterTemplateResource_To_v1beta2_OCIManagedClusterTemplateResource(in, out, s)
}

func Convert_v1beta3_OCIManagedControlPlaneTemplateResource_To_v1beta2_OCIManagedControlPlaneTemplateResource(in *v1beta3.OCIManagedControlPlaneTemplateResource, out *OCIManagedControlPlaneTemplateResource, s apiconversion.Scope) error {
	return autoConvert_v1beta3_OCIManagedControlPlaneTemplateResource_To_v1beta2_OCIManagedControlPlaneTemplateResource(in, out, s)
}

// ConvertTo converts the v1beta2 OCICluster receiver to a v1beta3 OCICluster.
func (src *OCICluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCICluster)
	if err := Convert_v1beta2_OCICluster_To_v1beta3_OCICluster(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCICluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	if restored.Status.Initialization.Provisioned != nil && !*restored.Status.Initialization.Provisioned && !src.Status.Ready {
		dst.Status.Initialization.Provisioned = restored.Status.Initialization.Provisioned
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)
	return nil
}

// ConvertFrom converts the v1beta3 OCICluster to a v1beta2 OCICluster.
func (dst *OCICluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCICluster)
	if err := Convert_v1beta3_OCICluster_To_v1beta2_OCICluster(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterList)
	return Convert_v1beta2_OCIClusterList_To_v1beta3_OCIClusterList(src, dst, nil)
}

func (dst *OCIClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIClusterList)
	return Convert_v1beta3_OCIClusterList_To_v1beta2_OCIClusterList(src, dst, nil)
}

func (src *OCIClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterTemplate)
	if err := Convert_v1beta2_OCIClusterTemplate_To_v1beta3_OCIClusterTemplate(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIClusterTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta
	return nil
}

func (dst *OCIClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIClusterTemplate)
	if err := Convert_v1beta3_OCIClusterTemplate_To_v1beta2_OCIClusterTemplate(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIClusterTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterTemplateList)
	return Convert_v1beta2_OCIClusterTemplateList_To_v1beta3_OCIClusterTemplateList(src, dst, nil)
}

func (dst *OCIClusterTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIClusterTemplateList)
	return Convert_v1beta3_OCIClusterTemplateList_To_v1beta2_OCIClusterTemplateList(src, dst, nil)
}

func (src *OCIMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachine)
	if err := Convert_v1beta2_OCIMachine_To_v1beta3_OCIMachine(src, dst, nil); err != nil {
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
	if err := Convert_v1beta3_OCIMachine_To_v1beta2_OCIMachine(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachineList)
	return Convert_v1beta2_OCIMachineList_To_v1beta3_OCIMachineList(src, dst, nil)
}

func (dst *OCIMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachineList)
	return Convert_v1beta3_OCIMachineList_To_v1beta2_OCIMachineList(src, dst, nil)
}

func (src *OCIMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachineTemplate)
	if err := Convert_v1beta2_OCIMachineTemplate_To_v1beta3_OCIMachineTemplate(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIMachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta
	return nil
}

func (dst *OCIMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachineTemplate)
	if err := Convert_v1beta3_OCIMachineTemplate_To_v1beta2_OCIMachineTemplate(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIMachineTemplateList)
	return Convert_v1beta2_OCIMachineTemplateList_To_v1beta3_OCIMachineTemplateList(src, dst, nil)
}

func (dst *OCIMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIMachineTemplateList)
	return Convert_v1beta3_OCIMachineTemplateList_To_v1beta2_OCIMachineTemplateList(src, dst, nil)
}

func (src *OCIClusterIdentity) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterIdentity)
	if err := Convert_v1beta2_OCIClusterIdentity_To_v1beta3_OCIClusterIdentity(src, dst, nil); err != nil {
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
	if err := Convert_v1beta3_OCIClusterIdentity_To_v1beta2_OCIClusterIdentity(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIClusterIdentityList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterIdentityList)
	return Convert_v1beta2_OCIClusterIdentityList_To_v1beta3_OCIClusterIdentityList(src, dst, nil)
}

func (dst *OCIClusterIdentityList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIClusterIdentityList)
	return Convert_v1beta3_OCIClusterIdentityList_To_v1beta2_OCIClusterIdentityList(src, dst, nil)
}

func (src *OCIManagedCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedCluster)
	if err := Convert_v1beta2_OCIManagedCluster_To_v1beta3_OCIManagedCluster(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIManagedCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	if restored.Status.Initialization.Provisioned != nil && !*restored.Status.Initialization.Provisioned && !src.Status.Ready {
		dst.Status.Initialization.Provisioned = restored.Status.Initialization.Provisioned
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)
	return nil
}

func (dst *OCIManagedCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedCluster)
	if err := Convert_v1beta3_OCIManagedCluster_To_v1beta2_OCIManagedCluster(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIManagedClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedClusterList)
	return Convert_v1beta2_OCIManagedClusterList_To_v1beta3_OCIManagedClusterList(src, dst, nil)
}

func (dst *OCIManagedClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedClusterList)
	return Convert_v1beta3_OCIManagedClusterList_To_v1beta2_OCIManagedClusterList(src, dst, nil)
}

func (src *OCIManagedClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedClusterTemplate)
	if err := Convert_v1beta2_OCIManagedClusterTemplate_To_v1beta3_OCIManagedClusterTemplate(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIManagedClusterTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta
	return nil
}

func (dst *OCIManagedClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedClusterTemplate)
	if err := Convert_v1beta3_OCIManagedClusterTemplate_To_v1beta2_OCIManagedClusterTemplate(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIManagedClusterTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedClusterTemplateList)
	return Convert_v1beta2_OCIManagedClusterTemplateList_To_v1beta3_OCIManagedClusterTemplateList(src, dst, nil)
}

func (dst *OCIManagedClusterTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedClusterTemplateList)
	return Convert_v1beta3_OCIManagedClusterTemplateList_To_v1beta2_OCIManagedClusterTemplateList(src, dst, nil)
}

func (src *OCIManagedControlPlane) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedControlPlane)
	if err := Convert_v1beta2_OCIManagedControlPlane_To_v1beta3_OCIManagedControlPlane(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIManagedControlPlane{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	if restored.Status.Initialization.ControlPlaneInitialized != nil && !*restored.Status.Initialization.ControlPlaneInitialized && !src.Status.Initialized && !src.Status.Ready {
		dst.Status.Initialization.ControlPlaneInitialized = restored.Status.Initialization.ControlPlaneInitialized
	}
	if len(restored.Status.Versions) > 0 && src.Status.Version != nil && *src.Status.Version != "" && restored.Status.Versions[0].Version == *src.Status.Version {
		dst.Status.Versions = restored.Status.Versions
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)
	return nil
}

func (dst *OCIManagedControlPlane) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedControlPlane)
	if err := Convert_v1beta3_OCIManagedControlPlane_To_v1beta2_OCIManagedControlPlane(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIManagedControlPlaneList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedControlPlaneList)
	return Convert_v1beta2_OCIManagedControlPlaneList_To_v1beta3_OCIManagedControlPlaneList(src, dst, nil)
}

func (dst *OCIManagedControlPlaneList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedControlPlaneList)
	return Convert_v1beta3_OCIManagedControlPlaneList_To_v1beta2_OCIManagedControlPlaneList(src, dst, nil)
}

func (src *OCIManagedControlPlaneTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedControlPlaneTemplate)
	if err := Convert_v1beta2_OCIManagedControlPlaneTemplate_To_v1beta3_OCIManagedControlPlaneTemplate(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIManagedControlPlaneTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta
	return nil
}

func (dst *OCIManagedControlPlaneTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedControlPlaneTemplate)
	if err := Convert_v1beta3_OCIManagedControlPlaneTemplate_To_v1beta2_OCIManagedControlPlaneTemplate(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIManagedControlPlaneTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedControlPlaneTemplateList)
	return Convert_v1beta2_OCIManagedControlPlaneTemplateList_To_v1beta3_OCIManagedControlPlaneTemplateList(src, dst, nil)
}

func (dst *OCIManagedControlPlaneTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedControlPlaneTemplateList)
	return Convert_v1beta3_OCIManagedControlPlaneTemplateList_To_v1beta2_OCIManagedControlPlaneTemplateList(src, dst, nil)
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
