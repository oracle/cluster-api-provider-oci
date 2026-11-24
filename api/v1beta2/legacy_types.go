/*
Copyright (c) 2026 Oracle and/or its affiliates.

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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// APIEndpoint is aliased to Cluster API v1beta2 APIEndpoint.
// +kubebuilder:object:generate=false
type APIEndpoint = clusterv1.APIEndpoint

// MachineAddressType is aliased to Cluster API v1beta2 MachineAddressType.
// +kubebuilder:object:generate=false
type MachineAddressType = clusterv1.MachineAddressType

// MachineAddress is aliased to Cluster API v1beta2 MachineAddress.
// +kubebuilder:object:generate=false
type MachineAddress = clusterv1.MachineAddress

const (
	// MachineHostName identifies a machine address which is a HostName.
	MachineHostName = clusterv1.MachineHostName
	// MachineExternalIP identifies a machine address which is an external IP.
	MachineExternalIP = clusterv1.MachineExternalIP
	// MachineInternalIP identifies a machine address which is an internal IP.
	MachineInternalIP = clusterv1.MachineInternalIP
	// MachineExternalDNS identifies a machine address which is an external DNS name.
	MachineExternalDNS = clusterv1.MachineExternalDNS
	// MachineInternalDNS identifies a machine address which is an internal DNS name.
	MachineInternalDNS = clusterv1.MachineInternalDNS
)

// FailureDomains is a map of FailureDomainSpec keyed by domain name.
// Preserved locally for v1beta2 wire compatibility after removing Cluster API v1beta1 imports.
type FailureDomains map[string]FailureDomainSpec

// FailureDomainSpec is the Schema for Cluster API failure domains.
type FailureDomainSpec struct {
	// controlPlane determines if this failure domain is suitable for use by control plane machines.
	// +optional
	ControlPlane bool `json:"controlPlane,omitempty"`

	// attributes is a free form map of attributes an infrastructure provider might use or require.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`
}

// ConditionSeverity expresses the severity of a Condition Type failing.
// +kubebuilder:validation:MaxLength=32
type ConditionSeverity string

const (
	// ConditionSeverityError specifies that a condition with `Status=False` is an error.
	ConditionSeverityError ConditionSeverity = "Error"

	// ConditionSeverityWarning specifies that a condition with `Status=False` is a warning.
	ConditionSeverityWarning ConditionSeverity = "Warning"

	// ConditionSeverityInfo specifies that a condition with `Status=False` is informative.
	ConditionSeverityInfo ConditionSeverity = "Info"

	// ConditionSeverityNone should apply only to conditions with `Status=True`.
	ConditionSeverityNone ConditionSeverity = ""
)

// ConditionType is a valid value for Condition.Type.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=256
type ConditionType string

// ReadyCondition defines the Ready condition type that summarizes the operational state of a Cluster API object.
const ReadyCondition ConditionType = "Ready"

// Condition defines an observation of a Cluster API resource operational state.
type Condition struct {
	// type of condition in CamelCase or in foo.example.com/CamelCase.
	// +required
	Type ConditionType `json:"type"`

	// status of the condition, one of True, False, Unknown.
	// +required
	Status corev1.ConditionStatus `json:"status"`

	// severity provides an explicit classification of Reason code.
	// +optional
	Severity ConditionSeverity `json:"severity,omitempty"`

	// lastTransitionTime is the last time the condition transitioned from one status to another.
	// +required
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// reason is the reason for the condition's last transition in CamelCase.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Reason string `json:"reason,omitempty"`

	// message is a human readable message indicating details about the transition.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=10240
	Message string `json:"message,omitempty"`
}

// Conditions provide observations of the operational state of a Cluster API resource.
type Conditions []Condition

// MachinePoolStatusFailure defines error states for MachinePool objects.
type MachinePoolStatusFailure string
