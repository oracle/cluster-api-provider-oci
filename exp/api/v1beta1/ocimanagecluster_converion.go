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
	v1 "k8s.io/api/core/v1"
	api_conversion "k8s.io/apimachinery/pkg/conversion"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"unsafe"
)

// ConvertTo converts the v1beta1 OCIManagedCluster receiver to a v1beta2 OCIManagedCluster.
func (src *OCIManagedCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIManagedCluster)

	if err := Convert_v1beta1_OCIManagedCluster_To_v1beta2_OCIManagedCluster(src, dst, nil); err != nil {
		return err
	}
	return nil
}

// ConvertFrom converts receiver to a v1beta2 OCICluster.
func (r *OCIManagedCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIManagedCluster)

	if err := Convert_v1beta2_OCIManagedCluster_To_v1beta1_OCIManagedCluster(src, r, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, r); err != nil {
		return err
	}

	return nil
}

func Convert_v1beta1_OCIManagedClusterSpec_To_v1beta2_OCIManagedClusterSpec(in *OCIManagedClusterSpec, out *v1beta2.OCIManagedClusterSpec, s api_conversion.Scope) error {
	out.OCIResourceIdentifier = in.OCIResourceIdentifier
	out.IdentityRef = (*v1.ObjectReference)(unsafe.Pointer(in.IdentityRef))
	if err := infrastructurev1beta1.Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec(&in.NetworkSpec, &out.NetworkSpec, s); err != nil {
		return err
	}
	out.FreeformTags = *(*map[string]string)(unsafe.Pointer(&in.FreeformTags))
	out.DefinedTags = *(*map[string]map[string]string)(unsafe.Pointer(&in.DefinedTags))
	out.CompartmentId = in.CompartmentId
	out.Region = in.Region
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
	return nil
}

func Convert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(in *v1beta2.OCIManagedClusterSpec, out *OCIManagedClusterSpec, s api_conversion.Scope) error {
	out.OCIResourceIdentifier = in.OCIResourceIdentifier
	out.IdentityRef = (*v1.ObjectReference)(unsafe.Pointer(in.IdentityRef))
	if err := infrastructurev1beta1.Convert_v1beta2_NetworkSpec_To_v1beta1_NetworkSpec(&in.NetworkSpec, &out.NetworkSpec, s); err != nil {
		return err
	}
	out.FreeformTags = *(*map[string]string)(unsafe.Pointer(&in.FreeformTags))
	out.DefinedTags = *(*map[string]map[string]string)(unsafe.Pointer(&in.DefinedTags))
	out.CompartmentId = in.CompartmentId
	out.Region = in.Region
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
	return nil
}
