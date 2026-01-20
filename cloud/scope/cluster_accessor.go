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

package scope

import (
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// OCIClusterAccessor interface defines the methods needed to access or modify
// member variables in Cluster object. This interface is required
// because there are 2 different cluster objects in OCI provider, OCiCluster(which is self managed)
// and OCIManagedCluster(which is a managed cluster - OKE). But both these clusters needs to
// share the same code for Network reconciliation. Hence this wrapper interface,
type OCIClusterAccessor interface {
	// GetOCIResourceIdentifier returns the OCI resource identifier of the cluster.
	GetOCIResourceIdentifier() string
	// GetDefinedTags returns the defined tags of the cluster.
	GetDefinedTags() map[string]map[string]string
	// GetCompartmentId returns the compartment id of the cluster.
	GetCompartmentId() string
	// GetFreeformTags returns the free form tags of the cluster.
	GetFreeformTags() map[string]string
	// GetName returns the name of the cluster.
	GetName() string
	// GetNameSpace returns the namespace of the cluster.
	GetNameSpace() string
	// GetRegion returns the region of the cluster, if specified in the spec.
	GetRegion() string
	// GetClientOverrides returns the client host url overrides for the cluster
	GetClientOverrides() *infrastructurev1beta2.ClientOverrides
	// GetNetworkSpec returns the NetworkSpec of the cluster.
	GetNetworkSpec() *infrastructurev1beta2.NetworkSpec
	// GetControlPlaneEndpoint returns the control plane endpoint of the cluster.
	GetControlPlaneEndpoint() clusterv1.APIEndpoint
	// SetControlPlaneEndpoint sets the control plane endpoint of the cluster.
	SetControlPlaneEndpoint(endpoint clusterv1.APIEndpoint)
	// GetFailureDomains returns the failure domains of the cluster.
	GetFailureDomains() clusterv1.FailureDomains
	// SetFailureDomain sets the failure domain.
	SetFailureDomain(id string, spec clusterv1.FailureDomainSpec)
	// GetAvailabilityDomains get the availability domain.
	GetAvailabilityDomains() map[string]infrastructurev1beta2.OCIAvailabilityDomain
	// SetAvailabilityDomains sets the availability domain.
	SetAvailabilityDomains(ads map[string]infrastructurev1beta2.OCIAvailabilityDomain)
	// MarkConditionFalse marks the provided condition as false in the cluster object
	MarkConditionFalse(t clusterv1.ConditionType, reason string, severity clusterv1.ConditionSeverity, messageFormat string, messageArgs ...interface{})
	// GetIdentityRef returns the Identity reference of the cluster
	GetIdentityRef() *corev1.ObjectReference
	// GetProviderID returns the provider id for the instance
	GetProviderID(instanceId string) string
}
