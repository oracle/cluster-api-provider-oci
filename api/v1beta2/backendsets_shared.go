package v1beta2

import (
	apiserverlb "github.com/oracle/cluster-api-provider-oci/api/internal/apiserverlb"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func canonicalAPIServerBackendSets(configured []NLBBackendSet, legacy BackendSetDetails) []NLBBackendSet {
	return apiserverlb.CanonicalBackendSets(configured, func() NLBBackendSet {
		return NLBBackendSet{
			Name:              APIServerLBBackendSetName,
			BackendSetDetails: legacy,
		}
	})
}

func validateSharedAPIServerLBBackendSets(spec NLBSpec, apiServerPort *int32, fldPath *field.Path) field.ErrorList {
	backendSets := make([]apiserverlb.BackendSet, 0, len(spec.BackendSets))
	for _, backendSet := range spec.BackendSets {
		backendSets = append(backendSets, apiserverlb.BackendSet{
			Name:         backendSet.Name,
			ListenerPort: backendSet.ListenerPort,
		})
	}
	return apiserverlb.ValidateBackendSets(backendSets, spec.BackendSetDetails != (BackendSetDetails{}), apiServerPort, supportedSecondaryListenerPort, fldPath)
}

func effectiveAPIServerListenerPort(listenerPort *int32, apiServerPort *int32) (int32, bool) {
	return apiserverlb.EffectiveAPIServerListenerPort(listenerPort, apiServerPort)
}

func isSupportedAPIServerListenerPort(port int32, apiServerPort *int32) bool {
	return apiserverlb.IsSupportedAPIServerListenerPort(port, apiServerPort, supportedSecondaryListenerPort)
}

func supportedAPIServerListenerPorts(apiServerPort *int32) []int32 {
	return apiserverlb.SupportedAPIServerListenerPorts(apiServerPort, supportedSecondaryListenerPort)
}

func supportedAPIServerListenerPortsString(apiServerPort *int32) string {
	return apiserverlb.SupportedAPIServerListenerPortsString(apiServerPort, supportedSecondaryListenerPort)
}
