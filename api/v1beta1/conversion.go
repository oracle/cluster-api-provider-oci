/*
 Copyright (c) 2023 Oracle and/or its affiliates.

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

package v1beta1

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
	for len(s) > 0 && !((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) {
		s = s[1:]
	}
	if len(s) > 1024 {
		s = s[:1024]
	}
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

// Convert_v1beta1_Condition_To_v1_Condition converts legacy Condition to metav1.Condition.
func Convert_v1beta1_Condition_To_v1_Condition(in *Condition, out *metav1.Condition, _ apiconversion.Scope) error {
	out.Type = string(in.Type)
	out.Status = metav1.ConditionStatus(in.Status)
	out.LastTransitionTime = in.LastTransitionTime
	out.Reason = sanitizeConditionReason(in.Reason, in.Type, in.Status)
	out.Message = in.Message
	return nil
}

// Convert_v1_Condition_To_v1beta1_Condition converts metav1.Condition to legacy Condition.
func Convert_v1_Condition_To_v1beta1_Condition(in *metav1.Condition, out *Condition, _ apiconversion.Scope) error {
	out.Type = ConditionType(in.Type)
	out.Status = corev1.ConditionStatus(in.Status)
	out.LastTransitionTime = in.LastTransitionTime
	out.Reason = in.Reason
	out.Message = in.Message
	return nil
}

// ConvertConditionsToMeta converts legacy v1beta1 Conditions to standard metav1.Condition slice.
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

// ConvertConditionsFromMeta converts standard metav1.Condition slice to legacy v1beta1 Conditions.
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

func convertMachineAddressesToV1Beta3(in []MachineAddress) []clusterv1.MachineAddress {
	if in == nil {
		return nil
	}
	out := make([]clusterv1.MachineAddress, 0, len(in))
	for _, a := range in {
		out = append(out, clusterv1.MachineAddress{
			Type:    clusterv1.MachineAddressType(a.Type),
			Address: a.Address,
		})
	}
	return out
}

func convertMachineAddressesFromV1Beta3(in []clusterv1.MachineAddress) []MachineAddress {
	if in == nil {
		return nil
	}
	out := make([]MachineAddress, 0, len(in))
	for _, a := range in {
		out = append(out, MachineAddress{
			Type:    MachineAddressType(a.Type),
			Address: a.Address,
		})
	}
	return out
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

// Convert_v1beta1_VCN_To_v1beta3_VCN converts v1beta1 VCN to v1beta3 VCN
func Convert_v1beta1_VCN_To_v1beta3_VCN(in *VCN, out *v1beta3.VCN, s apiconversion.Scope) error {
	err := autoConvert_v1beta1_VCN_To_v1beta3_VCN(in, out, s)
	if err != nil {
		return err
	}
	if in.InternetGatewayId != nil {
		out.InternetGateway.Id = in.InternetGatewayId
	}
	if in.NatGatewayId != nil {
		out.NATGateway.Id = in.NatGatewayId
	}
	if in.ServiceGatewayId != nil {
		out.ServiceGateway.Id = in.ServiceGatewayId
	}
	if in.PrivateRouteTableId != nil {
		out.RouteTable.PrivateRouteTableId = in.PrivateRouteTableId
	}
	if in.PublicRouteTableId != nil {
		out.RouteTable.PublicRouteTableId = in.PublicRouteTableId
	}
	if in.NetworkSecurityGroups != nil {
		nsgList, err := convertv1beta1NSGListTov1beta3NSGList(in.NetworkSecurityGroups)
		if err != nil {
			return err
		}
		out.NetworkSecurityGroup.List = nsgList
	}
	return nil
}

// Convert_v1beta3_VCN_To_v1beta1_VCN converts v1beta3 VCN to v1beta1 VCN
func Convert_v1beta3_VCN_To_v1beta1_VCN(in *v1beta3.VCN, out *VCN, s apiconversion.Scope) error {
	err := autoConvert_v1beta3_VCN_To_v1beta1_VCN(in, out, s)
	if err != nil {
		return err
	}
	if in.InternetGateway.Id != nil {
		out.InternetGatewayId = in.InternetGateway.Id
	}
	if in.NATGateway.Id != nil {
		out.NatGatewayId = in.NATGateway.Id
	}
	if in.ServiceGateway.Id != nil {
		out.ServiceGatewayId = in.ServiceGateway.Id
	}
	if in.RouteTable.PublicRouteTableId != nil {
		out.PublicRouteTableId = in.RouteTable.PublicRouteTableId
	}
	if in.RouteTable.PrivateRouteTableId != nil {
		out.PrivateRouteTableId = in.RouteTable.PrivateRouteTableId
	}
	if in.NetworkSecurityGroup.List != nil {
		nsgList, err := convertv1beta3NSGListTov1beta1NSGList(in.NetworkSecurityGroup.List)
		if err != nil {
			return err
		}
		out.NetworkSecurityGroups = nsgList
	}
	return nil
}

func Convert_v1beta1_OCIClusterSpec_To_v1beta3_OCIClusterSpec(in *OCIClusterSpec, out *v1beta3.OCIClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_OCIClusterSpec_To_v1beta3_OCIClusterSpec(in, out, s); err != nil {
		return err
	}
	out.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: in.ControlPlaneEndpoint.Host,
		Port: in.ControlPlaneEndpoint.Port,
	}
	return nil
}

func Convert_v1beta3_OCIClusterSpec_To_v1beta1_OCIClusterSpec(in *v1beta3.OCIClusterSpec, out *OCIClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIClusterSpec_To_v1beta1_OCIClusterSpec(in, out, s); err != nil {
		return err
	}
	out.ControlPlaneEndpoint = APIEndpoint{
		Host: in.ControlPlaneEndpoint.Host,
		Port: in.ControlPlaneEndpoint.Port,
	}
	return nil
}

func Convert_v1beta1_OCIClusterStatus_To_v1beta3_OCIClusterStatus(in *OCIClusterStatus, out *v1beta3.OCIClusterStatus, _ apiconversion.Scope) error {
	out.FailureDomains = convertFailureDomainsToV1Beta3(in.FailureDomains)
	out.Conditions = ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	return nil
}

func Convert_v1beta3_OCIClusterStatus_To_v1beta1_OCIClusterStatus(in *v1beta3.OCIClusterStatus, out *OCIClusterStatus, _ apiconversion.Scope) error {
	out.FailureDomains = convertFailureDomainsFromV1Beta3(in.FailureDomains)
	out.Conditions = ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	return nil
}

func Convert_v1beta1_EgressSecurityRuleForNSG_To_v1beta3_EgressSecurityRuleForNSG(in *EgressSecurityRuleForNSG, out *v1beta3.EgressSecurityRuleForNSG, s apiconversion.Scope) error {
	return autoConvert_v1beta1_EgressSecurityRuleForNSG_To_v1beta3_EgressSecurityRuleForNSG(in, out, s)
}

func Convert_v1beta1_IngressSecurityRuleForNSG_To_v1beta3_IngressSecurityRuleForNSG(in *IngressSecurityRuleForNSG, out *v1beta3.IngressSecurityRuleForNSG, s apiconversion.Scope) error {
	return autoConvert_v1beta1_IngressSecurityRuleForNSG_To_v1beta3_IngressSecurityRuleForNSG(in, out, s)
}

func Convert_v1beta1_NetworkDetails_To_v1beta3_NetworkDetails(in *NetworkDetails, out *v1beta3.NetworkDetails, s apiconversion.Scope) error {
	return autoConvert_v1beta1_NetworkDetails_To_v1beta3_NetworkDetails(in, out, s)
}

func Convert_v1beta3_NetworkDetails_To_v1beta1_NetworkDetails(in *v1beta3.NetworkDetails, out *NetworkDetails, s apiconversion.Scope) error {
	return autoConvert_v1beta3_NetworkDetails_To_v1beta1_NetworkDetails(in, out, s)
}

func Convert_v1beta1_OCIMachineSpec_To_v1beta3_OCIMachineSpec(in *OCIMachineSpec, out *v1beta3.OCIMachineSpec, s apiconversion.Scope) error {
	err := autoConvert_v1beta1_OCIMachineSpec_To_v1beta3_OCIMachineSpec(in, out, s)
	if err != nil {
		return err
	}
	if in.NSGName != "" && len(in.NetworkDetails.NsgNames) == 0 {
		out.NetworkDetails.NsgNames = []string{in.NSGName}
	}
	return nil
}

func Convert_v1beta1_OCIMachineStatus_To_v1beta3_OCIMachineStatus(in *OCIMachineStatus, out *v1beta3.OCIMachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_OCIMachineStatus_To_v1beta3_OCIMachineStatus(in, out, s); err != nil {
		return err
	}
	out.Addresses = convertMachineAddressesToV1Beta3(in.Addresses)
	out.Conditions = ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	return nil
}

func Convert_v1beta3_OCIMachineStatus_To_v1beta1_OCIMachineStatus(in *v1beta3.OCIMachineStatus, out *OCIMachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIMachineStatus_To_v1beta1_OCIMachineStatus(in, out, s); err != nil {
		return err
	}
	out.Addresses = convertMachineAddressesFromV1Beta3(in.Addresses)
	out.Conditions = ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	return nil
}

func Convert_v1beta3_LoadBalancer_To_v1beta1_LoadBalancer(in *v1beta3.LoadBalancer, out *LoadBalancer, s apiconversion.Scope) error {
	return autoConvert_v1beta3_LoadBalancer_To_v1beta1_LoadBalancer(in, out, s)
}

func Convert_v1beta1_OCIManagedControlPlaneSpec_To_v1beta3_OCIManagedControlPlaneSpec(in *OCIManagedControlPlaneSpec, out *v1beta3.OCIManagedControlPlaneSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_OCIManagedControlPlaneSpec_To_v1beta3_OCIManagedControlPlaneSpec(in, out, s); err != nil {
		return err
	}
	out.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: in.ControlPlaneEndpoint.Host,
		Port: in.ControlPlaneEndpoint.Port,
	}
	return nil
}

func Convert_v1beta3_OCIManagedControlPlaneSpec_To_v1beta1_OCIManagedControlPlaneSpec(in *v1beta3.OCIManagedControlPlaneSpec, out *OCIManagedControlPlaneSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIManagedControlPlaneSpec_To_v1beta1_OCIManagedControlPlaneSpec(in, out, s); err != nil {
		return err
	}
	out.ControlPlaneEndpoint = APIEndpoint{
		Host: in.ControlPlaneEndpoint.Host,
		Port: in.ControlPlaneEndpoint.Port,
	}
	return nil
}

func Convert_v1beta1_OCIManagedControlPlaneStatus_To_v1beta3_OCIManagedControlPlaneStatus(in *OCIManagedControlPlaneStatus, out *v1beta3.OCIManagedControlPlaneStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_OCIManagedControlPlaneStatus_To_v1beta3_OCIManagedControlPlaneStatus(in, out, s); err != nil {
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

func Convert_v1beta3_OCIManagedControlPlaneStatus_To_v1beta1_OCIManagedControlPlaneStatus(in *v1beta3.OCIManagedControlPlaneStatus, out *OCIManagedControlPlaneStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIManagedControlPlaneStatus_To_v1beta1_OCIManagedControlPlaneStatus(in, out, s); err != nil {
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

func Convert_v1beta1_OCIManagedClusterSpec_To_v1beta3_OCIManagedClusterSpec(in *OCIManagedClusterSpec, out *v1beta3.OCIManagedClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_OCIManagedClusterSpec_To_v1beta3_OCIManagedClusterSpec(in, out, s); err != nil {
		return err
	}
	out.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: in.ControlPlaneEndpoint.Host,
		Port: in.ControlPlaneEndpoint.Port,
	}
	return nil
}

func Convert_v1beta3_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(in *v1beta3.OCIManagedClusterSpec, out *OCIManagedClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta3_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(in, out, s); err != nil {
		return err
	}
	out.ControlPlaneEndpoint = APIEndpoint{
		Host: in.ControlPlaneEndpoint.Host,
		Port: in.ControlPlaneEndpoint.Port,
	}
	return nil
}

func Convert_v1beta1_OCIManagedClusterStatus_To_v1beta3_OCIManagedClusterStatus(in *OCIManagedClusterStatus, out *v1beta3.OCIManagedClusterStatus, _ apiconversion.Scope) error {
	out.FailureDomains = convertFailureDomainsToV1Beta3(in.FailureDomains)
	out.Conditions = ConvertConditionsToMeta(in.Conditions)
	if in.Ready {
		out.Initialization.Provisioned = ptr.To(true)
	}
	return nil
}

func Convert_v1beta3_OCIManagedClusterStatus_To_v1beta1_OCIManagedClusterStatus(in *v1beta3.OCIManagedClusterStatus, out *OCIManagedClusterStatus, _ apiconversion.Scope) error {
	out.FailureDomains = convertFailureDomainsFromV1Beta3(in.FailureDomains)
	out.Conditions = ConvertConditionsFromMeta(in.Conditions)
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	return nil
}

func Convert_v1beta3_ClusterOptions_To_v1beta1_ClusterOptions(in *v1beta3.ClusterOptions, out *ClusterOptions, s apiconversion.Scope) error {
	return autoConvert_v1beta3_ClusterOptions_To_v1beta1_ClusterOptions(in, out, s)
}

func Convert_v1beta1_OCIClusterIdentityStatus_To_v1beta3_OCIClusterIdentityStatus(in *OCIClusterIdentityStatus, out *v1beta3.OCIClusterIdentityStatus, _ apiconversion.Scope) error {
	out.Conditions = ConvertConditionsToMeta(in.Conditions)
	return nil
}

func Convert_v1beta3_OCIClusterIdentityStatus_To_v1beta1_OCIClusterIdentityStatus(in *v1beta3.OCIClusterIdentityStatus, out *OCIClusterIdentityStatus, _ apiconversion.Scope) error {
	out.Conditions = ConvertConditionsFromMeta(in.Conditions)
	return nil
}

func Convert_v1beta3_OCIClusterTemplateResource_To_v1beta1_OCIClusterTemplateResource(in *v1beta3.OCIClusterTemplateResource, out *OCIClusterTemplateResource, s apiconversion.Scope) error {
	return autoConvert_v1beta3_OCIClusterTemplateResource_To_v1beta1_OCIClusterTemplateResource(in, out, s)
}

func Convert_v1beta3_OCIMachineTemplateResource_To_v1beta1_OCIMachineTemplateResource(in *v1beta3.OCIMachineTemplateResource, out *OCIMachineTemplateResource, s apiconversion.Scope) error {
	return autoConvert_v1beta3_OCIMachineTemplateResource_To_v1beta1_OCIMachineTemplateResource(in, out, s)
}

func Convert_v1beta3_OCIManagedClusterTemplateResource_To_v1beta1_OCIManagedClusterTemplateResource(in *v1beta3.OCIManagedClusterTemplateResource, out *OCIManagedClusterTemplateResource, s apiconversion.Scope) error {
	return autoConvert_v1beta3_OCIManagedClusterTemplateResource_To_v1beta1_OCIManagedClusterTemplateResource(in, out, s)
}

func Convert_v1beta3_OCIManagedControlPlaneTemplateResource_To_v1beta1_OCIManagedControlPlaneTemplateResource(in *v1beta3.OCIManagedControlPlaneTemplateResource, out *OCIManagedControlPlaneTemplateResource, s apiconversion.Scope) error {
	return autoConvert_v1beta3_OCIManagedControlPlaneTemplateResource_To_v1beta1_OCIManagedControlPlaneTemplateResource(in, out, s)
}

// Template conversion methods for OCIManagedClusterTemplate and OCIManagedControlPlaneTemplate
func (src *OCIManagedClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedClusterTemplate)
	if err := Convert_v1beta1_OCIManagedClusterTemplate_To_v1beta3_OCIManagedClusterTemplate(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIManagedClusterTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta
	dst.Spec.Template.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.Skip
	dst.Spec.Template.Spec.NetworkSpec.Vcn.NATGateway.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.NATGateway.Skip
	dst.Spec.Template.Spec.NetworkSpec.Vcn.ServiceGateway.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.ServiceGateway.Skip
	dst.Spec.Template.Spec.NetworkSpec.Vcn.InternetGateway.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.InternetGateway.Skip
	dst.Spec.Template.Spec.NetworkSpec.Vcn.RouteTable.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.RouteTable.Skip
	dst.Spec.Template.Spec.AvailabilityDomains = restored.Spec.Template.Spec.AvailabilityDomains
	dst.Spec.Template.Spec.NetworkSpec.APIServerLB.LoadBalancerType = restored.Spec.Template.Spec.NetworkSpec.APIServerLB.LoadBalancerType
	dst.Spec.Template.Spec.ClientOverrides = restored.Spec.Template.Spec.ClientOverrides
	return nil
}

func (dst *OCIManagedClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedClusterTemplate)
	if err := Convert_v1beta3_OCIManagedClusterTemplate_To_v1beta1_OCIManagedClusterTemplate(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIManagedClusterTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedClusterTemplateList)
	return Convert_v1beta1_OCIManagedClusterTemplateList_To_v1beta3_OCIManagedClusterTemplateList(src, dst, nil)
}

func (dst *OCIManagedClusterTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedClusterTemplateList)
	return Convert_v1beta3_OCIManagedClusterTemplateList_To_v1beta1_OCIManagedClusterTemplateList(src, dst, nil)
}

func (src *OCIManagedControlPlaneTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedControlPlaneTemplate)
	if err := Convert_v1beta1_OCIManagedControlPlaneTemplate_To_v1beta3_OCIManagedControlPlaneTemplate(src, dst, nil); err != nil {
		return err
	}
	restored := &v1beta3.OCIManagedControlPlaneTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta
	dst.Spec.Template.Spec.ClusterType = restored.Spec.Template.Spec.ClusterType
	dst.Spec.Template.Spec.Addons = restored.Spec.Template.Spec.Addons
	dst.Spec.Template.Spec.ClusterOption.OpenIdConnectDiscovery = restored.Spec.Template.Spec.ClusterOption.OpenIdConnectDiscovery
	dst.Spec.Template.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig = restored.Spec.Template.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig
	return nil
}

func (dst *OCIManagedControlPlaneTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedControlPlaneTemplate)
	if err := Convert_v1beta3_OCIManagedControlPlaneTemplate_To_v1beta1_OCIManagedControlPlaneTemplate(src, dst, nil); err != nil {
		return err
	}
	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIManagedControlPlaneTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedControlPlaneTemplateList)
	return Convert_v1beta1_OCIManagedControlPlaneTemplateList_To_v1beta3_OCIManagedControlPlaneTemplateList(src, dst, nil)
}

func (dst *OCIManagedControlPlaneTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedControlPlaneTemplateList)
	return Convert_v1beta3_OCIManagedControlPlaneTemplateList_To_v1beta1_OCIManagedControlPlaneTemplateList(src, dst, nil)
}
