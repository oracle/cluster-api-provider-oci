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

// OCISelfManagedCluster is the ClusterAccessor implementation for self managed clusters
type OCISelfManagedCluster struct {
	OCICluster *infrastructurev1beta1.OCICluster
}

func (c OCISelfManagedCluster) GetOCIResourceIdentifier() string {
	return c.OCICluster.Spec.OCIResourceIdentifier
}

func (c OCISelfManagedCluster) GetName() string {
	return c.OCICluster.Name
}

func (c OCISelfManagedCluster) GetDefinedTags() map[string]map[string]string {
	return c.OCICluster.Spec.DefinedTags
}

func (c OCISelfManagedCluster) GetCompartmentId() string {
	return c.OCICluster.Spec.CompartmentId
}

func (c OCISelfManagedCluster) GetFreeformTags() map[string]string {
	return c.OCICluster.Spec.FreeformTags
}

func (c OCISelfManagedCluster) GetDRG() *infrastructurev1beta1.DRG {
	return c.OCICluster.Spec.NetworkSpec.VCNPeering.DRG
}

func (c OCISelfManagedCluster) GetVCNPeering() *infrastructurev1beta1.VCNPeering {
	return c.OCICluster.Spec.NetworkSpec.VCNPeering
}

func (c OCISelfManagedCluster) GetNetworkSpec() *infrastructurev1beta1.NetworkSpec {
	return &c.OCICluster.Spec.NetworkSpec
}

func (c OCISelfManagedCluster) SetControlPlaneEndpoint(endpoint clusterv1.APIEndpoint) {
	c.OCICluster.Spec.ControlPlaneEndpoint = endpoint
}

func (c OCISelfManagedCluster) GetFailureDomains() clusterv1.FailureDomains {
	return c.OCICluster.Status.FailureDomains
}

func (c OCISelfManagedCluster) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	if c.OCICluster.Status.FailureDomains == nil {
		c.OCICluster.Status.FailureDomains = make(clusterv1.FailureDomains)
	}
	c.OCICluster.Status.FailureDomains[id] = spec
}

func (c OCISelfManagedCluster) SetAvailabilityDomains(ads map[string]infrastructurev1beta1.OCIAvailabilityDomain) {
	c.OCICluster.Status.AvailabilityDomains = ads
}
