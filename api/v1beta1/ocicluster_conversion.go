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
	"github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts the v1beta1 AWSCluster receiver to a v1beta2 AWSCluster.
func (src *OCICluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCICluster)

	if err := Convert_v1beta1_OCICluster_To_v1beta2_OCICluster(src, dst, nil); err != nil {
		return err
	}
	return nil
}

func convertv1beta1NSGListTov1beta2NSGList(in []*NSG) ([]*v1beta2.NSG, error) {
	out := make([]*v1beta2.NSG, len(in))
	for i, nsg := range in {
		out[i] = &v1beta2.NSG{}
		err := Convert_v1beta1_NSG_To_v1beta2_NSG(nsg, out[i], nil)
		if err == nil {
			return nil, err
		}
	}
	return out, nil
}

// ConvertFrom converts receiver to a v1beta2 OCICluster.
func (r *OCICluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCICluster)

	if err := Convert_v1beta2_OCICluster_To_v1beta1_OCICluster(src, r, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, r); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts the v1beta1 OCIClusterList receiver to a v1beta2 OCIClusterList.
func (src *OCIClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIClusterList)

	return Convert_v1beta1_OCIClusterList_To_v1beta2_OCIClusterList(src, dst, nil)
}

// ConvertFrom converts the v1beta2 AWSClusterList receiver to a v1beta1 OCIClusterList.
func (r *OCIClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIClusterList)

	return Convert_v1beta2_OCIClusterList_To_v1beta1_OCIClusterList(src, r, nil)
}
