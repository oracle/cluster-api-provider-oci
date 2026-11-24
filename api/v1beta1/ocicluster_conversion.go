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
	"github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts the v1beta1 OCICluster receiver to a v1beta3 OCICluster.
func (src *OCICluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCICluster)

	if err := Convert_v1beta1_OCICluster_To_v1beta3_OCICluster(src, dst, nil); err != nil {
		return err
	}

	ad, err := Convertv1beta1AdMapTov1beta3AdMap(src.Status.AvailabilityDomains)
	if err != nil {
		return err
	}
	dst.Spec.AvailabilityDomains = ad

	// Manually restore data.
	restored := &v1beta3.OCICluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.Skip = restored.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.Skip
	dst.Spec.NetworkSpec.Vcn.NATGateway.Skip = restored.Spec.NetworkSpec.Vcn.NATGateway.Skip
	dst.Spec.NetworkSpec.Vcn.ServiceGateway.Skip = restored.Spec.NetworkSpec.Vcn.ServiceGateway.Skip
	dst.Spec.NetworkSpec.Vcn.InternetGateway.Skip = restored.Spec.NetworkSpec.Vcn.InternetGateway.Skip
	dst.Spec.NetworkSpec.Vcn.RouteTable.Skip = restored.Spec.NetworkSpec.Vcn.RouteTable.Skip
	dst.Spec.NetworkSpec.APIServerLB.LoadBalancerType = restored.Spec.NetworkSpec.APIServerLB.LoadBalancerType
	dst.Spec.ClientOverrides = restored.Spec.ClientOverrides
	if restored.Status.Initialization.Provisioned != nil && !*restored.Status.Initialization.Provisioned && !src.Status.Ready {
		dst.Status.Initialization.Provisioned = restored.Status.Initialization.Provisioned
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)

	return nil
}

func convertv1beta1NSGListTov1beta3NSGList(in []*NSG) ([]*v1beta3.NSG, error) {
	out := make([]*v1beta3.NSG, len(in))
	for i, nsg := range in {
		if nsg == nil {
			out[i] = nil
			continue
		}
		out[i] = &v1beta3.NSG{}
		err := Convert_v1beta1_NSG_To_v1beta3_NSG(nsg, out[i], nil)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func convertv1beta3NSGListTov1beta1NSGList(in []*v1beta3.NSG) ([]*NSG, error) {
	out := make([]*NSG, len(in))
	for i, nsg := range in {
		if nsg == nil {
			out[i] = nil
			continue
		}
		out[i] = &NSG{}
		err := Convert_v1beta3_NSG_To_v1beta1_NSG(nsg, out[i], nil)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func Convertv1beta1AdMapTov1beta3AdMap(in map[string]OCIAvailabilityDomain) (map[string]v1beta3.OCIAvailabilityDomain, error) {
	if in == nil {
		return nil, nil
	}
	out := make(map[string]v1beta3.OCIAvailabilityDomain, len(in))
	for k := range in {
		outV := &v1beta3.OCIAvailabilityDomain{}
		inDomain := in[k]
		err := Convert_v1beta1_OCIAvailabilityDomain_To_v1beta3_OCIAvailabilityDomain(&inDomain, outV, nil)
		if err != nil {
			return nil, err
		}
		out[k] = *outV
	}
	return out, nil
}

func Convertv1beta3AdMapTov1beta1AdMap(in map[string]v1beta3.OCIAvailabilityDomain) (map[string]OCIAvailabilityDomain, error) {
	if in == nil {
		return nil, nil
	}
	out := make(map[string]OCIAvailabilityDomain, len(in))
	for k := range in {
		outV := &OCIAvailabilityDomain{}
		inDomain := in[k]
		err := Convert_v1beta3_OCIAvailabilityDomain_To_v1beta1_OCIAvailabilityDomain(&inDomain, outV, nil)
		if err != nil {
			return nil, err
		}
		out[k] = *outV
	}
	return out, nil
}

// ConvertFrom converts receiver to a v1beta1 OCICluster from a v1beta3 OCICluster.
func (r *OCICluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCICluster)

	if err := Convert_v1beta3_OCICluster_To_v1beta1_OCICluster(src, r, nil); err != nil {
		return err
	}

	ad, err := Convertv1beta3AdMapTov1beta1AdMap(src.Spec.AvailabilityDomains)
	if err != nil {
		return err
	}
	r.Status.AvailabilityDomains = ad

	// Preserve Hub data on down-conversion.
	return utilconversion.MarshalDataUnsafeNoCopy(src, r)
}

// ConvertTo converts the v1beta1 OCIClusterList receiver to a v1beta3 OCIClusterList.
func (src *OCIClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIClusterList)
	return Convert_v1beta1_OCIClusterList_To_v1beta3_OCIClusterList(src, dst, nil)
}

// ConvertFrom converts the v1beta3 OCIClusterList receiver to a v1beta1 OCIClusterList.
func (r *OCIClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIClusterList)
	return Convert_v1beta3_OCIClusterList_To_v1beta1_OCIClusterList(src, r, nil)
}
