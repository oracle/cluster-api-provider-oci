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

package v1beta3

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty,omitzero"`

	// AvailabilityDomains encapsulates the clusters Availability Domain (AD) information in a map
	// where the map key is the AD name and the struct is details about the AD.
	// +optional
	AvailabilityDomains map[string]OCIAvailabilityDomain `json:"availabilityDomains,omitempty"`

	// ClientOverrides allows the default client SDK URLs to be changed.
	//
	// +optional
	// +nullable
	ClientOverrides *ClientOverrides `json:"clientOverrides,omitempty"`
}

// OCIClusterStatus defines the observed state of OCICluster
type OCIClusterStatus struct {
	// initialization provides observations of the OCICluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization OCIClusterInitializationStatus `json:"initialization,omitempty,omitzero"`

	// failureDomains is a list of failure domain objects synced from the infrastructure provider.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	FailureDomains []clusterv1.FailureDomain `json:"failureDomains,omitempty"`

	// conditions represents the observations of an OCICluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// OCIClusterInitializationStatus provides observations of the OCICluster initialization process.
// +kubebuilder:validation:MinProperties=1
type OCIClusterInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the Cluster's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Cluster provisioning.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="Cluster infrastructure is ready"
// +kubebuilder:printcolumn:name="Provisioned",type="boolean",JSONPath=".status.initialization.provisioned",description="Cluster infrastructure is provisioned"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint.host",description="API Endpoint"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of OCICluster"

// OCICluster is the Schema for the ociclusters API.
type OCICluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIClusterSpec   `json:"spec,omitempty"`
	Status OCIClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +kubebuilder:storageversion

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

// ClientOverrides contains information about client host url overrides.
type ClientOverrides struct {

	// CertOverride is a secret that contains information about a cert override used by all the OCI SDK clients.
	// The secret must contain data with a `cert`property.
	//
	// +optional
	// +nullable
	CertOverride *corev1.SecretReference `json:"certOverride,omitempty"`

	// ComputeClientUrl allows the default compute SDK client URL to be changed.
	//
	// +optional
	// +nullable
	ComputeClientUrl *string `json:"computeClientUrl,omitempty"`

	// ComputeManagementClientUrl allows the default compute management SDK client URL to be changed.
	//
	// +optional
	// +nullable
	ComputeManagementClientUrl *string `json:"computeManagementClientUrl,omitempty"`

	// VCNClientUrl allows the default vcn SDK client URL to be changed.
	//
	// +optional
	// +nullable
	VCNClientUrl *string `json:"vCNClientUrl,omitempty"`

	// LoadBalancerClientUrl allows the default load balancer SDK client URL to be changed.
	//
	// +optional
	// +nullable
	LoadBalancerClientUrl *string `json:"loadBalancerClientUrl,omitempty"`

	// NetworkLoadBalancerClientUrl allows the default NLB SDK client URL to be changed.
	//
	// +optional
	// +nullable
	NetworkLoadBalancerClientUrl *string `json:"networkLoadBalancerClientUrl,omitempty"`

	// IdentityClientUrl allows the default identity SDK client URL to be changed.
	//
	// +optional
	// +nullable
	IdentityClientUrl *string `json:"identityClientUrl,omitempty"`

	// ContainerEngineClientUrl allows the default container engine SDK client URL to be changed.
	//
	// +optional
	// +nullable
	ContainerEngineClientUrl *string `json:"containerEngineClientUrl,omitempty"`

	// WorkrequestClientUrl allows the default work request SDK client URL to be changed.
	//
	// +optional
	// +nullable
	WorkrequestClientUrl *string `json:"workrequestClientUrl,omitempty"`
}

// GetConditions returns the list of conditions for an OCICluster API object.
func (c *OCICluster) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an OCICluster object.
func (c *OCICluster) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// GetOCIResourceIdentifier will return the OCI resource identifier.
func (c *OCICluster) GetOCIResourceIdentifier() string {
	return c.Spec.OCIResourceIdentifier
}

func init() {
	SchemeBuilder.Register(&OCICluster{}, &OCIClusterList{})
}
