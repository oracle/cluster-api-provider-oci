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

// ConvertTo converts the v1beta1 OCIClusterTemplate receiver to a v1beta2 OCIClusterTemplate.
func (src *OCIClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIClusterTemplate)
	if err := Convert_v1beta1_OCIClusterTemplate_To_v1beta2_OCIClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &v1beta2.OCIClusterTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.Template.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.Skip
	dst.Spec.Template.Spec.NetworkSpec.Vcn.NATGateway.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.NATGateway.Skip
	dst.Spec.Template.Spec.NetworkSpec.Vcn.ServiceGateway.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.ServiceGateway.Skip
	dst.Spec.Template.Spec.NetworkSpec.Vcn.InternetGateway.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.InternetGateway.Skip
	dst.Spec.Template.Spec.NetworkSpec.Vcn.RouteTable.Skip = restored.Spec.Template.Spec.NetworkSpec.Vcn.RouteTable.Skip
	dst.Spec.Template.Spec.AvailabilityDomains = restored.Spec.Template.Spec.AvailabilityDomains
	return nil
}

// ConvertFrom converts the v1beta2 OCIClusterTemplate to a v1beta1 OCIClusterTemplate.
func (dst *OCIClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIClusterTemplate)

	if err := Convert_v1beta2_OCIClusterTemplate_To_v1beta1_OCIClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion except for metadata.
	return utilconversion.MarshalData(src, dst)
}

// ConvertTo converts the v1beta1 OCIMachineTemplateList receiver to a v1beta2 OCIMachineTemplateList.
func (src *OCIClusterTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIClusterTemplateList)
	return Convert_v1beta1_OCIClusterTemplateList_To_v1beta2_OCIClusterTemplateList(src, dst, nil)
}

// ConvertFrom converts the v1beta2 OCIMachineTemplateList to a v1beta1 OCIMachineTemplateList.
func (dst *OCIClusterTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIClusterTemplateList)

	return Convert_v1beta2_OCIClusterTemplateList_To_v1beta1_OCIClusterTemplateList(src, dst, nil)
}
