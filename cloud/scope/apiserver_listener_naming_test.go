package scope

import (
	"testing"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
)

func TestDesiredAPIServerListenerName(t *testing.T) {
	t.Run("keeps legacy listener name for single legacy backend set", func(t *testing.T) {
		got := desiredAPIServerListenerName(0, 1, APIServerLBBackendSetName)
		if got != APIServerLBListener {
			t.Fatalf("expected legacy listener name %q, got %q", APIServerLBListener, got)
		}
	})

	t.Run("uses stable hashed names for secondary listeners", func(t *testing.T) {
		a := desiredAPIServerListenerName(1, 2, "rollout-set")
		b := desiredAPIServerListenerName(1, 2, "rollout-set")
		c := desiredAPIServerListenerName(1, 2, "another-set")
		if a != b {
			t.Fatalf("expected deterministic name for same backend set, got %q and %q", a, b)
		}
		if a == c {
			t.Fatalf("expected different names for different backend sets, got %q and %q", a, c)
		}
	})
}

func TestDesiredAPIServerListenerPort(t *testing.T) {
	defaultPort := int32(6443)
	if got := desiredAPIServerListenerPort(defaultPort, infrastructurev1beta2.NLBBackendSet{}); got != defaultPort {
		t.Fatalf("expected default port %d, got %d", defaultPort, got)
	}
	customPort := int32(9345)
	if got := desiredAPIServerListenerPort(defaultPort, infrastructurev1beta2.NLBBackendSet{ListenerPort: &customPort}); got != customPort {
		t.Fatalf("expected custom port %d, got %d", customPort, got)
	}
}
