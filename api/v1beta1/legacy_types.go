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

package v1beta1

import (
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// APIEndpoint represents a reachable Kubernetes API endpoint.
// Preserved locally for v1beta1 wire compatibility after removing Cluster API v1beta1 imports.
type APIEndpoint struct {
	// host is the hostname on which the API server is serving.
	// +optional
	// +kubebuilder:validation:MaxLength=512
	Host string `json:"host"`

	// port is the port on which the API server is serving.
	// +optional
	Port int32 `json:"port"`
}

// IsZero returns true if both host and port are zero values.
func (v APIEndpoint) IsZero() bool {
	return v.Host == "" && v.Port == 0
}

// IsValid returns true if both host and port are non-zero values.
func (v APIEndpoint) IsValid() bool {
	return v.Host != "" && v.Port != 0
}

// String returns a formatted version HOST:PORT of this APIEndpoint.
func (v APIEndpoint) String() string {
	return net.JoinHostPort(v.Host, fmt.Sprintf("%d", v.Port))
}

// FailureDomains is a map of FailureDomainSpec keyed by domain name.
// Preserved locally for v1beta1 wire compatibility after removing Cluster API v1beta1 imports.
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

// MachineAddressType describes a valid MachineAddress type.
type MachineAddressType string

const (
	// MachineHostName identifies a machine address which is a HostName.
	MachineHostName MachineAddressType = "Hostname"
	// MachineExternalIP identifies a machine address which is an external IP.
	MachineExternalIP MachineAddressType = "ExternalIP"
	// MachineInternalIP identifies a machine address which is an internal IP.
	MachineInternalIP MachineAddressType = "InternalIP"
	// MachineExternalDNS identifies a machine address which is an external DNS name.
	MachineExternalDNS MachineAddressType = "ExternalDNS"
	// MachineInternalDNS identifies a machine address which is an internal DNS name.
	MachineInternalDNS MachineAddressType = "InternalDNS"
)

// MachineAddress contains information for the node's address.
type MachineAddress struct {
	// type is the machine address type, one of Hostname, ExternalIP, InternalIP, ExternalDNS or InternalDNS.
	// +required
	Type MachineAddressType `json:"type"`

	// address is the machine address.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Address string `json:"address"`
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

// MachineStatusError defines error states for Machine objects.
type MachineStatusError string
