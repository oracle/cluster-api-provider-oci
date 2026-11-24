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

func (src *OCIClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterTemplate)
	if err := Convert_v1beta1_OCIClusterTemplate_To_v1beta3_OCIClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	restored := &v1beta3.OCIClusterTemplate{}
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

func (dst *OCIClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIClusterTemplate)

	if err := Convert_v1beta3_OCIClusterTemplate_To_v1beta1_OCIClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalDataUnsafeNoCopy(src, dst)
}

func (src *OCIClusterTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterTemplateList)
	return Convert_v1beta1_OCIClusterTemplateList_To_v1beta3_OCIClusterTemplateList(src, dst, nil)
}

func (dst *OCIClusterTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIClusterTemplateList)
	return Convert_v1beta3_OCIClusterTemplateList_To_v1beta1_OCIClusterTemplateList(src, dst, nil)
}
