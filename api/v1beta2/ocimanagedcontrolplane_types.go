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
	"github.com/oracle/oci-go-sdk/v65/containerengine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

const (
	// ControlPlaneFinalizer allows OCIManagedControlPlaneFinalizer to clean up OCI resources associated with control plane
	// of OCIManagedControlPlane
	ControlPlaneFinalizer = "ocimanagedcontrolplane.infrastructure.cluster.x-k8s.io"
)

const (
	BasicClusterType    ClusterTypeEnum = "BASIC_CLUSTER"
	EnhancedClusterType ClusterTypeEnum = "ENHANCED_CLUSTER"
)

type ClusterTypeEnum string

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

	// ClusterTypeEnum defines the type of cluster. Supported types are
	// * `BASIC_CLUSTER`
	// * `ENHANCED_CLUSTER`
	// +optional
	ClusterType ClusterTypeEnum `json:"clusterType,omitempty"`

	// KmsKeyId defines the OCID of the KMS key to be used as the master encryption key for Kubernetes secret encryption. When used,
	// +optional
	KmsKeyId *string `json:"kmsKeyId,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// The list of addons to be applied to the OKE cluster.
	// +optional
	// +listType=map
	// +listMapKey=name
	Addons []Addon `json:"addons,omitempty"`

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

	// OpenIDConnectDiscovery specifies OIDC discovery settings
	// +optional
	OpenIdConnectDiscovery *OpenIDConnectDiscovery `json:"openIdConnectDiscovery,omitempty"`

	//OpenIDConnectTokenAuthenticationConfig
	// +optional
	OpenIdConnectTokenAuthenticationConfig *OpenIDConnectTokenAuthenticationConfig `json:"openIdConnectTokenAuthenticationConfig,omitempty"`
}

type OpenIDConnectDiscovery struct {
	// IsOpenIDConnectDiscoveryEnabled defines whether or not to enable the OIDC discovery.
	// +optional
	IsOpenIdConnectDiscoveryEnabled *bool `json:"isOpenIdConnectDiscoveryEnabled,omitempty"`
}

type OpenIDConnectTokenAuthenticationConfig struct {
	// A Base64 encoded public RSA or ECDSA certificates used to sign your identity provider's web certificate.
	// +optional
	CaCertificate *string `json:"caCertificate,omitempty"`

	// A client id that all tokens must be issued for.
	// +optional
	ClientId *string `json:"clientId,omitempty"`

	// JWT claim to use as the user's group. If the claim is present it must be an array of strings.
	// +optional
	GroupsClaim *string `json:"groupsClaim,omitempty"`

	// Prefix prepended to group claims to prevent clashes with existing names (such as system:groups).
	// +optional
	GroupsPrefix *string `json:"groupsPrefix,omitempty"`

	// IsOpenIdConnectAuthEnabled defines whether or not to enable the OIDC authentication.
	IsOpenIdConnectAuthEnabled bool `json:"isOpenIdConnectAuthEnabled"`

	// URL of the provider that allows the API server to discover public signing keys. Only URLs that use the https:// scheme are accepted. This is typically the provider's discovery URL, changed to have an empty path.
	// +optional
	IssuerUrl *string `json:"issuerUrl,omitempty"`

	// A key=value pair that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value. Repeat this flag to specify multiple claims.
	// +optional
	RequiredClaims []KeyValue `json:"requiredClaims,omitempty"`

	// The signing algorithms accepted. Default is ["RS256"].
	// +optional
	SigningAlgorithms []string `json:"signingAlgorithms,omitempty"`

	// JWT claim to use as the user name. By default sub, which is expected to be a unique identifier of the end user. Admins can choose other claims, such as email or name, depending on their provider. However, claims other than email will be prefixed with the issuer URL to prevent naming clashes with other plugins.
	// +optional
	UsernameClaim *string `json:"usernameClaim,omitempty"`

	// Prefix prepended to username claims to prevent clashes with existing names (such as system:users). For example, the value oidc: will create usernames like oidc:jane.doe. If this flag isn't provided and --oidc-username-claim is a value other than email the prefix defaults to ( Issuer URL )# where ( Issuer URL ) is the value of --oidc-issuer-url. The value - can be used to disable all prefixing.
	// +optional
	UsernamePrefix *string `json:"usernamePrefix,omitempty"`
}

// KeyValue defines the properties that define a key value pair. This is alias to containerengine.KeyValue, to support the sdk type
type KeyValue containerengine.KeyValue

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

	// AddonStatus represents the status of the addon.
	// +optional
	AddonStatus map[string]AddonStatus `json:"addonStatus,omitempty"`

	// Initialized denotes whether or not the control plane has the
	// uploaded kubernetes config-map.
	// +optional
	Initialized bool `json:"initialized"`
}

// Addon defines the properties of an addon.
type Addon struct {
	// Name represents the name of the addon.
	Name *string `json:"name"`

	// Version represents the version of the addon.
	// +optional
	Version *string `json:"version,omitempty"`

	// Configurations defines a list of configurations of the addon.
	// +optional
	Configurations []AddonConfiguration `json:"configurations,omitempty"`
}

// AddonConfiguration defines a configuration of an addon.
type AddonConfiguration struct {
	// The key of the configuration.
	Key *string `json:"key,omitempty"`

	// The value of the configuration.
	Value *string `json:"value,omitempty"`
}

// AddonStatus defines the status of an Addon.
type AddonStatus struct {
	// Version represents the version of the addon.
	// +optional
	CurrentlyInstalledVersion *string `json:"currentlyInstalledVersion,omitempty"`

	// AddonError defines the error encountered by the Addon.
	// +optional
	AddonError *AddonError `json:"addonError,omitempty"`

	// LifecycleState defines the lifecycle state of the addon.
	// +optional
	LifecycleState *string `json:"lifecycleState,omitempty"`
}

type AddonError struct {
	// Code defines a  short error code that defines the upstream error, meant for programmatic parsing.
	// +optional
	Code *string `json:"code,omitempty"`

	// Message defines a human-readable error string of the upstream error.
	// +optional
	Message *string `json:"message,omitempty"`

	// Status defines the status of the HTTP response encountered in the upstream error.
	// +optional
	Status *string `json:"status,omitempty"`
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

// SetAddonStatus sets the addon status in the OCIManagedControlPlane
func (c *OCIManagedControlPlane) SetAddonStatus(name string, status AddonStatus) {
	if c.Status.AddonStatus == nil {
		c.Status.AddonStatus = make(map[string]AddonStatus)
	}
	c.Status.AddonStatus[name] = status
}

// RemoveAddonStatus removes the addon status from OCIManagedControlPlane
func (c *OCIManagedControlPlane) RemoveAddonStatus(name string) {
	if c.Status.AddonStatus == nil {
		c.Status.AddonStatus = make(map[string]AddonStatus)
	}
	delete(c.Status.AddonStatus, name)
}

func init() {
	SchemeBuilder.Register(&OCIManagedControlPlane{}, &OCIManagedControlPlaneList{})
}
