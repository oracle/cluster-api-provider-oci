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
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
	// GetNetworkSpec returns the NetworkSpec of the cluster.
	GetNetworkSpec() *infrastructurev1beta1.NetworkSpec
	// SetControlPlaneEndpoint sets the control plane endpoint of the cluster.
	SetControlPlaneEndpoint(endpoint clusterv1.APIEndpoint)
	// GetFailureDomains returns the failure domains of the cluster.
	GetFailureDomains() clusterv1.FailureDomains
	// SetFailureDomain sets the failure domain.
	SetFailureDomain(id string, spec clusterv1.FailureDomainSpec)
	// SetAvailabilityDomains sets the availability domain.
	SetAvailabilityDomains(ads map[string]infrastructurev1beta1.OCIAvailabilityDomain)
}
