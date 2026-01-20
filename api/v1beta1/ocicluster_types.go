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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

const (
	// ClusterFinalizer allows OCIClusterReconciler to clean up OCI resources associated with OCICluster before
	// removing it from the apiserver.
	ClusterFinalizer = "ocicluster.infrastructure.cluster.x-k8s.io"
)

// OCIClusterSpec defines the desired state of OciCluster
type OCIClusterSpec struct {

	// The unique ID which will be used to tag all the resources created by this Cluster.
	// The tag will be used to identify resources belonging to this cluster.
	// this will be auto-generated and should not be set by the user.
	// +optional
	OCIResourceIdentifier string `json:"ociResourceIdentifier,omitempty"`

	// IdentityRef is a reference to an identity(principal) to be used when reconciling this cluster
	// +optional
	IdentityRef *corev1.ObjectReference `json:"identityRef,omitempty"`

	// NetworkSpec encapsulates all things related to OCI network.
	// +optional
	NetworkSpec NetworkSpec `json:"networkSpec,omitempty"`

	// Free-form tags for this resource.
	// +optional
	FreeformTags map[string]string `json:"freeformTags,omitempty"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	// +optional
	DefinedTags map[string]map[string]string `json:"definedTags,omitempty"`

	// Compartment to create the cluster network.
	// +optional
	CompartmentId string `json:"compartmentId"`

	// Region the cluster operates in. It must be one of available regions in Region Identifier format.
	// See https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm
	Region string `json:"region,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// OCIClusterStatus defines the observed state of OCICluster
type OCIClusterStatus struct {
	// +optional
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	// AvailabilityDomains encapsulates the clusters Availability Domain (AD) information in a map
	// where the map key is the AD name and the struct is details about the AD.
	// +optional
	AvailabilityDomains map[string]OCIAvailabilityDomain `json:"availabilityDomains,omitempty"`

	// +optional
	Ready bool `json:"ready"`
	// NetworkSpec encapsulates all things related to OCI network.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OCICluster is the Schema for the ociclusters API.
type OCICluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIClusterSpec   `json:"spec,omitempty"`
	Status OCIClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OCIClusterList contains a list of OCICluster.
type OCIClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCICluster `json:"items"`
}

// OCIAvailabilityDomain contains information about an Availability Domain (AD).
type OCIAvailabilityDomain struct {

	// Name is the AD's full name. Example: Uocm:PHX-AD-1
	Name string `json:"name,omitempty"`

	// FaultDomains a list of fault domain (FD) names. Example: ["FAULT-DOMAIN-1"]
	FaultDomains []string `json:"faultDomains,omitempty"`
}

// GetConditions returns the list of conditions for an OCICluster API object.
func (c *OCICluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an OCICluster object.
func (c *OCICluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// GetOCIResourceIdentifier will return the OCI resource identifier.
func (c *OCICluster) GetOCIResourceIdentifier() string {
	return c.Spec.OCIResourceIdentifier
}

func init() {
	SchemeBuilder.Register(&OCICluster{}, &OCIClusterList{})
}
