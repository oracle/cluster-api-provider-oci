package scope

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
)

func desiredAPIServerListenerPort(defaultPort int32, backendSet infrastructurev1beta2.NLBBackendSet) int32 {
	if backendSet.ListenerPort != nil {
		return *backendSet.ListenerPort
	}
	return defaultPort
}

func desiredAPIServerListenerName(index int, total int, backendSetName string) string {
	// Preserve legacy name for legacy/single-backend-set behavior.
	if total == 1 && backendSetName == APIServerLBBackendSetName {
		return APIServerLBListener
	}
	// Keep the first listener stable for the control plane endpoint.
	if index == 0 {
		return APIServerLBListener
	}
	return fmt.Sprintf("%s-%s", APIServerLBListener, stableShortHash(backendSetName))
}

func stableShortHash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}
