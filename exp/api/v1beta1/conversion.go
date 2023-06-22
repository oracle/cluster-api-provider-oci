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
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"k8s.io/apimachinery/pkg/conversion"
)

// Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec converts v1beta1 NetworkSpec to v1beta2 NetworkSpec
func Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec(in *infrastructurev1beta1.NetworkSpec, out *infrastructurev1beta2.NetworkSpec, s conversion.Scope) error {
	return infrastructurev1beta1.Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec(in, out, s)
}

// Convert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec converts v1beta1 OCIManagedClusterSpec to v1beta2 OCIManagedClusterSpec
func Convert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(in *v1beta2.OCIManagedClusterSpec, out *OCIManagedClusterSpec, s conversion.Scope) error {
	return autoConvert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(in, out, s)
}

// Convert_v1beta2_NetworkSpec_To_v1beta1_NetworkSpec converts v1beta2 NetworkSpec to v1beta1 NetworkSpec
func Convert_v1beta2_NetworkSpec_To_v1beta1_NetworkSpec(in *infrastructurev1beta2.NetworkSpec, out *infrastructurev1beta1.NetworkSpec, s conversion.Scope) error {
	return infrastructurev1beta1.Convert_v1beta2_NetworkSpec_To_v1beta1_NetworkSpec(in, out, s)
}

// Convert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus converts v1beta1 OCIManagedClusterStatus to v1beta2 OCIManagedClusterStatus
func Convert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus(in *OCIManagedClusterStatus, out *v1beta2.OCIManagedClusterStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus(in, out, s)
}

// Convert_v1beta2_NetworkDetails_To_v1beta1_NetworkDetails converts v1beta2 NetworkDetails to v1beta1 NetworkDetails
func Convert_v1beta2_NetworkDetails_To_v1beta1_NetworkDetails(in *infrastructurev1beta2.NetworkDetails, out *infrastructurev1beta1.NetworkDetails, s conversion.Scope) error {
	return infrastructurev1beta1.Convert_v1beta2_NetworkDetails_To_v1beta1_NetworkDetails(in, out, s)
}

func Convert_v1beta2_OCIManagedControlPlaneSpec_To_v1beta1_OCIManagedControlPlaneSpec(in *v1beta2.OCIManagedControlPlaneSpec, out *OCIManagedControlPlaneSpec, s conversion.Scope) error {
	return autoConvert_v1beta2_OCIManagedControlPlaneSpec_To_v1beta1_OCIManagedControlPlaneSpec(in, out, s)
}

func Convert_v1beta2_OCIManagedMachinePoolSpec_To_v1beta1_OCIManagedMachinePoolSpec(in *v1beta2.OCIManagedMachinePoolSpec, out *OCIManagedMachinePoolSpec, s conversion.Scope) error {
	return autoConvert_v1beta2_OCIManagedMachinePoolSpec_To_v1beta1_OCIManagedMachinePoolSpec(in, out, s)
}
