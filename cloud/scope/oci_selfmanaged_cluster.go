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
	"fmt"
	"slices"

	infrastructurev1beta3 "github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// OCISelfManagedCluster is the ClusterAccessor implementation for self managed clusters
type OCISelfManagedCluster struct {
	OCICluster *infrastructurev1beta3.OCICluster
}

func (c OCISelfManagedCluster) GetNameSpace() string {
	return c.OCICluster.Namespace
}

func (c OCISelfManagedCluster) GetRegion() string {
	return c.OCICluster.Spec.Region
}

func (c OCISelfManagedCluster) GetClientOverrides() *infrastructurev1beta3.ClientOverrides {
	return c.OCICluster.Spec.ClientOverrides
}

func (c OCISelfManagedCluster) GetIdentityRef() *corev1.ObjectReference {
	return c.OCICluster.Spec.IdentityRef
}

func (c OCISelfManagedCluster) MarkConditionFalse(t string, reason string, messageFormat string, messageArgs ...any) {
	ociutil.MarkConditionFalse(c.OCICluster, infrastructurev1beta3.ClusterReadyCondition, reason, messageFormat, messageArgs...)
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

func (c OCISelfManagedCluster) GetDRG() *infrastructurev1beta3.DRG {
	return c.OCICluster.Spec.NetworkSpec.VCNPeering.DRG
}

func (c OCISelfManagedCluster) GetVCNPeering() *infrastructurev1beta3.VCNPeering {
	return c.OCICluster.Spec.NetworkSpec.VCNPeering
}

func (c OCISelfManagedCluster) GetNetworkSpec() *infrastructurev1beta3.NetworkSpec {
	return &c.OCICluster.Spec.NetworkSpec
}

func (c OCISelfManagedCluster) SetControlPlaneEndpoint(endpoint clusterv1.APIEndpoint) {
	c.OCICluster.Spec.ControlPlaneEndpoint = endpoint
}

func (c OCISelfManagedCluster) GetFailureDomains() []clusterv1.FailureDomain {
	return c.OCICluster.Status.FailureDomains
}

func (c OCISelfManagedCluster) SetFailureDomain(id string, spec clusterv1.FailureDomain) {
	spec.Name = id
	c.OCICluster.Status.FailureDomains = upsertFailureDomain(c.OCICluster.Status.FailureDomains, spec)
}

func (c OCISelfManagedCluster) GetAvailabilityDomains() map[string]infrastructurev1beta3.OCIAvailabilityDomain {
	return c.OCICluster.Spec.AvailabilityDomains
}

func (c OCISelfManagedCluster) SetAvailabilityDomains(ads map[string]infrastructurev1beta3.OCIAvailabilityDomain) {
	c.OCICluster.Spec.AvailabilityDomains = ads
}

func (c OCISelfManagedCluster) GetControlPlaneEndpoint() clusterv1.APIEndpoint {
	return c.OCICluster.Spec.ControlPlaneEndpoint
}

func (c OCISelfManagedCluster) GetProviderID(instanceId string) string {
	return fmt.Sprintf("oci://%s", instanceId)
}

func upsertFailureDomain(domains []clusterv1.FailureDomain, fd clusterv1.FailureDomain) []clusterv1.FailureDomain {
	for i := range domains {
		if domains[i].Name == fd.Name {
			domains[i] = fd
			return domains
		}
	}
	domains = append(domains, fd)
	slices.SortFunc(domains, func(a, b clusterv1.FailureDomain) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return domains
}

func findFailureDomain(domains []clusterv1.FailureDomain, name string) (clusterv1.FailureDomain, bool) {
	for _, fd := range domains {
		if fd.Name == name {
			return fd, true
		}
	}
	return clusterv1.FailureDomain{}, false
}
