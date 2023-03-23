package v1beta1

import (
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"k8s.io/apimachinery/pkg/conversion"
)

// Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec is a conversion function.
func Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec(in *infrastructurev1beta1.NetworkSpec, out *infrastructurev1beta2.NetworkSpec, s conversion.Scope) error {
	return infrastructurev1beta1.Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec(in, out, s)
}

// Convert_v1beta1_NetworkSpec_To_v1beta2_NetworkSpec is a conversion function.
func Convert_v1beta2_NetworkSpec_To_v1beta1_NetworkSpec(in *infrastructurev1beta2.NetworkSpec, out *infrastructurev1beta1.NetworkSpec, s conversion.Scope) error {
	return infrastructurev1beta1.Convert_v1beta2_NetworkSpec_To_v1beta1_NetworkSpec(in, out, s)
}
