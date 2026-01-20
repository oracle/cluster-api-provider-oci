/*
 Copyright (c) 2022, 2023 Oracle and/or its affiliates.

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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

type PrincipalType string

const (
	// UserPrincipal represents a user principal.
	UserPrincipal PrincipalType = "UserPrincipal"
	// InstancePrincipal represents a instance principal.
	InstancePrincipal PrincipalType = "InstancePrincipal"
	// WorkloadPrincipal represents a workload principal.
	WorkloadPrincipal PrincipalType = "Workload"
)

// OCIClusterIdentitySpec defines the parameters that are used to create an OCIClusterIdentity.
type OCIClusterIdentitySpec struct {
	// Type is the type of OCI Principal used.
	// UserPrincipal is the only supported value
	Type PrincipalType `json:"type"`

	// PrincipalSecret is a secret reference which contains the authentication credentials for the principal.
	// +optional
	PrincipalSecret corev1.SecretReference `json:"principalSecret,omitempty"`

	// AllowedNamespaces is used to identify the namespaces the clusters are allowed to use the identity from.
	// Namespaces can be selected either using an array of namespaces or with label selector.
	// An empty allowedNamespaces object indicates that OCIClusters can use this identity from any namespace.
	// If this object is nil, no namespaces will be allowed (default behaviour, if this field is not provided)
	// A namespace should be either in the NamespaceList or match with Selector to use the identity.
	//
	// +optional
	// +nullable
	AllowedNamespaces *AllowedNamespaces `json:"allowedNamespaces"`
}

// AllowedNamespaces defines the namespaces the clusters are allowed to use the identity from
type AllowedNamespaces struct {
	// A nil or empty list indicates that OCICluster cannot use the identity from any namespace.
	// NamespaceList takes precedence over the Selector.
	// +optional
	// +nullable
	NamespaceList []string `json:"list"`

	// Selector is a selector of namespaces that OCICluster can
	// use this Identity from. This is a standard Kubernetes LabelSelector,
	// a label query over a set of resources. The result of matchLabels and
	// matchExpressions are ANDed.
	//
	// A nil or empty selector indicates that OCICluster cannot use this
	// OCIClusterIdentity from any namespace.
	// +optional
	Selector *metav1.LabelSelector `json:"selector"`
}

// OCIClusterIdentityStatus defines the observed state of OCIClusterIdentity.
type OCIClusterIdentityStatus struct {
	// Conditions defines current service state of the OCIClusterIdentity.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:storageversion

// OCIClusterIdentity is the Schema for the OCI Cluster Identity API
type OCIClusterIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OCIClusterIdentitySpec   `json:"spec,omitempty"`
	Status            OCIClusterIdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OCIClusterIdentityList contains a list of OCIClusterIdentity.
type OCIClusterIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIClusterIdentity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCIClusterIdentity{}, &OCIClusterIdentityList{})
}
