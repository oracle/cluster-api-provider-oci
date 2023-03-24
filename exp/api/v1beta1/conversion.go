package v1beta1

import (
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"k8s.io/apimachinery/pkg/conversion"
)

// Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec is a conversion function.
func Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec(in *infrastructurev1beta1.NetworkSpec, out *infrastructurev1beta2.NetworkSpec, s conversion.Scope) error {
	return infrastructurev1beta1.Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec(in, out, s)
}

func Convert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(in *v1beta2.OCIManagedClusterSpec, out *OCIManagedClusterSpec, s conversion.Scope) error {
	return autoConvert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(in, out, s)
}

// Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec is a conversion function.
func Convert_v1beta2_NetworkSpec_To_v1beta1_NetworkSpec(in *infrastructurev1beta2.NetworkSpec, out *infrastructurev1beta1.NetworkSpec, s conversion.Scope) error {
	return infrastructurev1beta1.Convert_v1beta2_NetworkSpec_To_v1beta1_NetworkSpec(in, out, s)
}

func Convert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus(in *OCIManagedClusterStatus, out *v1beta2.OCIManagedClusterStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus(in, out, s)
}

func Convert_v1beta2_NetworkDetails_To_v1beta1_NetworkDetails(in *infrastructurev1beta2.NetworkDetails, out *infrastructurev1beta1.NetworkDetails, s conversion.Scope) error {
	return infrastructurev1beta1.Convert_v1beta2_NetworkDetails_To_v1beta1_NetworkDetails(in, out, s)
}
