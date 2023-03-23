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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ControlPlaneFinalizer allows OCIManagedControlPlaneFinalizer to clean up OCI resources associated with control plane
	// of OCIManagedControlPlane
	ControlPlaneFinalizer = "ocimanagedcontrolplane.infrastructure.cluster.x-k8s.io"
)

// OCIManagedControlPlaneSpec defines the desired state of OCIManagedControlPlane.
// The properties are generated from https://docs.oracle.com/en-us/iaas/api/#/en/containerengine/20180222/datatypes/CreateClusterDetails
type OCIManagedControlPlaneSpec struct {
	// ID of the OKEcluster.
	// +optional
	ID *string `json:"id,omitempty"`

	// ClusterPodNetworkOptions defines the available CNIs and network options for existing and new node pools of the cluster
	// +optional
	ClusterPodNetworkOptions []ClusterPodNetworkOptions `json:"clusterPodNetworkOptions,omitempty"`

	// ImagePolicyConfig defines the properties that define a image verification policy.
	// +optional
	ImagePolicyConfig *ImagePolicyConfig `json:"imagePolicyConfig,omitempty"`

	// ClusterOptions defines Optional attributes for the cluster.
	// +optional
	ClusterOption ClusterOptions `json:"clusterOptions,omitempty"`

	// KmsKeyId defines the OCID of the KMS key to be used as the master encryption key for Kubernetes secret encryption. When used,
	// +optional
	KmsKeyId *string `json:"kmsKeyId,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// Version represents the version of the Kubernetes Cluster Control Plane.
	Version *string `json:"version,omitempty"`
}

// ClusterPodNetworkOptions defines the available CNIs and network options for existing and new node pools of the cluster
type ClusterPodNetworkOptions struct {

	// The CNI to be used are OCI_VCN_IP_NATIVE and FLANNEL_OVERLAY
	CniType CNIOptionEnum `json:"cniType,omitempty"`
}

// EndpointConfig defines the network configuration for access to the Cluster control plane.
type EndpointConfig struct {
	// Flag to enable public endpoint address for the OKE cluster.
	// If not set, will calculate this using endpoint subnet type.
	// +optional
	IsPublicIpEnabled bool `json:"isPublicIpEnabled,omitempty"`
}

// ImagePolicyConfig defines the properties that define a image verification policy.
type ImagePolicyConfig struct {

	// IsPolicyEnabled defines Whether the image verification policy is enabled.
	// +optional
	IsPolicyEnabled *bool `json:"isPolicyEnabled,omitempty"`

	// KeyDetails defines a list of KMS key details.
	// +optional
	KeyDetails []KeyDetails `json:"keyDetails,omitempty"`
}

// KeyDetails defines the properties that define the kms keys used by OKE for Image Signature verification.
type KeyDetails struct {

	// KmsKeyId defines the OCID of the KMS key that will be used to verify whether the images are signed by an approved source.
	// +optional
	KmsKeyId *string `json:"keyDetails,omitempty"`
}

// ClusterOptions defines Optional attributes for the cluster.
type ClusterOptions struct {

	// AddOnOptions defines the properties that define options for supported add-ons.
	// +optional
	AddOnOptions *AddOnOptions `json:"addOnOptions,omitempty"`

	// AdmissionControllerOptions defines the properties that define supported admission controllers.
	// +optional
	AdmissionControllerOptions *AdmissionControllerOptions `json:"admissionControllerOptions,omitempty"`
}

// AddOnOptions defines the properties that define options for supported add-ons.
type AddOnOptions struct {
	// IsKubernetesDashboardEnabled defines whether or not to enable the Kubernetes Dashboard add-on.
	// +optional
	IsKubernetesDashboardEnabled *bool `json:"isKubernetesDashboardEnabled,omitempty"`

	// IsKubernetesDashboardEnabled defines whether or not to enable the Tiller add-on.
	// +optional
	IsTillerEnabled *bool `json:"isTillerEnabled,omitempty"`
}

// AdmissionControllerOptions defines the properties that define supported admission controllers.
type AdmissionControllerOptions struct {

	// IsPodSecurityPolicyEnabled defines whether or not to enable the Pod Security Policy admission controller.
	// +optional
	IsPodSecurityPolicyEnabled *bool `json:"isPodSecurityPolicyEnabled,omitempty"`
}

// KubernetesNetworkConfig defines the properties that define the network configuration for Kubernetes.
type KubernetesNetworkConfig struct {

	// PodsCidr defines the CIDR block for Kubernetes pods. Optional, defaults to 10.244.0.0/16.
	// +optional
	PodsCidr string `json:"isPodSecurityPolicyEnabled,omitempty"`

	// PodsCidr defines the CIDR block for Kubernetes services. Optional, defaults to 10.96.0.0/16.
	// +optional
	ServicesCidr string `json:"servicesCidr,omitempty"`
}

// OCIManagedControlPlaneStatus defines the observed state of OCIManagedControlPlane
type OCIManagedControlPlaneStatus struct {
	// +optional
	Ready bool `json:"ready"`
	// NetworkSpec encapsulates all things related to OCI network.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// Version represents the current Kubernetes version for the control plane.
	// +optional
	Version *string `json:"version,omitempty"`

	// Initialized denotes whether or not the control plane has the
	// uploaded kubernetes config-map.
	// +optional
	Initialized bool `json:"initialized"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:storageversion

// OCIManagedControlPlane is the Schema for the ocimanagedcontrolplane API.
type OCIManagedControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIManagedControlPlaneSpec   `json:"spec,omitempty"`
	Status OCIManagedControlPlaneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +kubebuilder:storageversion

// OCIManagedControlPlaneList contains a list of OCIManagedControlPlane.
type OCIManagedControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIManagedControlPlane `json:"items"`
}

// GetConditions returns the list of conditions for an OCICluster API object.
func (c *OCIManagedControlPlane) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an OCICluster object.
func (c *OCIManagedControlPlane) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&OCIManagedControlPlane{}, &OCIManagedControlPlaneList{})
}
