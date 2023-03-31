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
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts the v1beta1 OCIManagedCluster receiver to a v1beta2 OCIManagedCluster.
func (src *OCIManagedCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIManagedCluster)

	if err := Convert_v1beta1_OCIManagedCluster_To_v1beta2_OCIManagedCluster(src, dst, nil); err != nil {
		return err
	}

	ad, err := infrastructurev1beta1.Convertv1beta1AdMapTov1beta2AdMap(src.Status.AvailabilityDomains)
	if err != nil {
		return err
	}
	dst.Spec.AvailabilityDomains = ad

	// Manually restore data.
	restored := &v1beta2.OCIManagedCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.Skip = restored.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.Skip
	dst.Spec.NetworkSpec.Vcn.NATGateway.Skip = restored.Spec.NetworkSpec.Vcn.NATGateway.Skip
	dst.Spec.NetworkSpec.Vcn.ServiceGateway.Skip = restored.Spec.NetworkSpec.Vcn.ServiceGateway.Skip
	dst.Spec.NetworkSpec.Vcn.InternetGateway.Skip = restored.Spec.NetworkSpec.Vcn.InternetGateway.Skip
	dst.Spec.NetworkSpec.Vcn.RouteTable.Skip = restored.Spec.NetworkSpec.Vcn.RouteTable.Skip
	dst.Spec.NetworkSpec.APIServerLB.LoadBalancerType = restored.Spec.NetworkSpec.APIServerLB.LoadBalancerType
	return nil
}

// ConvertFrom converts receiver to a v1beta2 OCICluster.
func (r *OCIManagedCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIManagedCluster)

	if err := Convert_v1beta2_OCIManagedCluster_To_v1beta1_OCIManagedCluster(src, r, nil); err != nil {
		return err
	}

	ad, err := infrastructurev1beta1.Convertv1beta2AdMapTov1beta1AdMap(src.Spec.AvailabilityDomains)
	if err != nil {
		return err
	}
	r.Status.AvailabilityDomains = ad

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, r); err != nil {
		return err
	}

	return nil
}
