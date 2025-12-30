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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

const (
	// ManagedClusterFinalizer allows OCIManagedClusterReconciler to clean up OCI resources associated with OCIManagedCluster.
	ManagedClusterFinalizer = "ocimanagedcluster.infrastructure.cluster.x-k8s.io"
)

// OCIManagedClusterSpec defines the desired state of OCI OKE Cluster
type OCIManagedClusterSpec struct {

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

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane. This will not be set by the user, this will be updated by the Cluster Reconciler after OKe cluster has been created and the cluster has an endpoint address
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// AvailabilityDomains encapsulates the clusters Availability Domain (AD) information in a map
	// where the map key is the AD name and the struct is details about the AD.
	// +optional
	AvailabilityDomains map[string]OCIAvailabilityDomain `json:"availabilityDomains,omitempty"`

	// ClientOverrides allows the default client SDK URLs to be changed.
	//
	// +optional
	// +nullable
	ClientOverrides *ClientOverrides `json:"hostUrl,omitempty"`
}

// OCIManagedClusterStatus defines the observed state of OCICluster
type OCIManagedClusterStatus struct {
	// +optional
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	// +optional
	Ready bool `json:"ready"`
	// NetworkSpec encapsulates all things related to OCI network.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:storageversion

// OCIManagedCluster is the Schema for the ocimanagedclusters API.
type OCIManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIManagedClusterSpec   `json:"spec,omitempty"`
	Status OCIManagedClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +kubebuilder:storageversion

// OCIManagedClusterList contains a list of OCIManagedCluster.
type OCIManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIManagedCluster `json:"items"`
}

// GetConditions returns the list of conditions for an OCICluster API object.
func (c *OCIManagedCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an OCICluster object.
func (c *OCIManagedCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&OCIManagedCluster{}, &OCIManagedClusterList{})
}
