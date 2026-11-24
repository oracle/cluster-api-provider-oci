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
	infrastructurev1beta3 "github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// OCIManagedCluster is the ClusterAccessor implementation for managed clusters(OKE)
type OCIManagedCluster struct {
	OCIManagedCluster *infrastructurev1beta3.OCIManagedCluster
}

func (c OCIManagedCluster) GetNameSpace() string {
	return c.OCIManagedCluster.Namespace
}

func (c OCIManagedCluster) GetRegion() string {
	return c.OCIManagedCluster.Spec.Region
}

func (c OCIManagedCluster) GetClientOverrides() *infrastructurev1beta3.ClientOverrides {
	return c.OCIManagedCluster.Spec.ClientOverrides
}

func (c OCIManagedCluster) MarkConditionFalse(t string, reason string, messageFormat string, messageArgs ...any) {
	ociutil.MarkConditionFalse(c.OCIManagedCluster, infrastructurev1beta3.ClusterReadyCondition, reason, messageFormat, messageArgs...)
}

func (c OCIManagedCluster) GetIdentityRef() *corev1.ObjectReference {
	return c.OCIManagedCluster.Spec.IdentityRef
}

func (c OCIManagedCluster) GetOCIResourceIdentifier() string {
	return c.OCIManagedCluster.Spec.OCIResourceIdentifier
}

func (c OCIManagedCluster) GetName() string {
	return c.OCIManagedCluster.Name
}

func (c OCIManagedCluster) GetDefinedTags() map[string]map[string]string {
	return c.OCIManagedCluster.Spec.DefinedTags
}

func (c OCIManagedCluster) GetCompartmentId() string {
	return c.OCIManagedCluster.Spec.CompartmentId
}

func (c OCIManagedCluster) GetFreeformTags() map[string]string {
	return c.OCIManagedCluster.Spec.FreeformTags
}

func (c OCIManagedCluster) GetDRG() *infrastructurev1beta3.DRG {
	return c.OCIManagedCluster.Spec.NetworkSpec.VCNPeering.DRG
}

func (c OCIManagedCluster) GetVCNPeering() *infrastructurev1beta3.VCNPeering {
	return c.OCIManagedCluster.Spec.NetworkSpec.VCNPeering
}

func (c OCIManagedCluster) GetNetworkSpec() *infrastructurev1beta3.NetworkSpec {
	return &c.OCIManagedCluster.Spec.NetworkSpec
}

func (c OCIManagedCluster) SetControlPlaneEndpoint(endpoint clusterv1.APIEndpoint) {
	c.OCIManagedCluster.Spec.ControlPlaneEndpoint = endpoint
}

func (c OCIManagedCluster) GetControlPlaneEndpoint() clusterv1.APIEndpoint {
	return c.OCIManagedCluster.Spec.ControlPlaneEndpoint
}

func (c OCIManagedCluster) GetFailureDomains() []clusterv1.FailureDomain {
	return c.OCIManagedCluster.Status.FailureDomains
}

func (c OCIManagedCluster) SetFailureDomain(id string, spec clusterv1.FailureDomain) {
	spec.Name = id
	c.OCIManagedCluster.Status.FailureDomains = upsertFailureDomain(c.OCIManagedCluster.Status.FailureDomains, spec)
}

func (c OCIManagedCluster) GetAvailabilityDomains() map[string]infrastructurev1beta3.OCIAvailabilityDomain {
	return c.OCIManagedCluster.Spec.AvailabilityDomains
}

func (c OCIManagedCluster) SetAvailabilityDomains(ads map[string]infrastructurev1beta3.OCIAvailabilityDomain) {
	c.OCIManagedCluster.Spec.AvailabilityDomains = ads
}

func (c OCIManagedCluster) GetProviderID(instanceId string) string {
	return instanceId
}
