package scope

import (
	"testing"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestBuildDesiredNLBListenersAndBackendSets(t *testing.T) {
	scope := &ClusterScope{Cluster: &clusterv1.Cluster{}}
	lb := infrastructurev1beta2.LoadBalancer{
		NLBSpec: infrastructurev1beta2.NLBSpec{
			BackendSets: []infrastructurev1beta2.NLBBackendSet{
				{Name: "primary-set"},
				{
					Name: "rollout-set",
					BackendSetDetails: infrastructurev1beta2.BackendSetDetails{
						HealthChecker: infrastructurev1beta2.HealthChecker{
							UrlPath: common.String("/readyz"),
						},
					},
				},
			},
		},
	}

	listeners, backendSets := scope.buildDesiredNLBListenersAndBackendSets(lb)
	if len(backendSets) != 2 {
		t.Fatalf("expected two backend sets, got %#v", backendSets)
	}
	if listeners[APIServerLBListener].DefaultBackendSetName == nil || *listeners[APIServerLBListener].DefaultBackendSetName != "primary-set" {
		t.Fatalf("expected listener to reference first backend set, got %#v", listeners[APIServerLBListener])
	}
	if backendSets["rollout-set"].HealthChecker == nil || backendSets["rollout-set"].HealthChecker.UrlPath == nil || *backendSets["rollout-set"].HealthChecker.UrlPath != "/readyz" {
		t.Fatalf("expected rollout backend set health checker override, got %#v", backendSets["rollout-set"])
	}
}

func TestBuildDesiredLBListenersAndBackendSets(t *testing.T) {
	scope := &ClusterScope{Cluster: &clusterv1.Cluster{}}
	lb := infrastructurev1beta2.LoadBalancer{
		NLBSpec: infrastructurev1beta2.NLBSpec{
			BackendSets: []infrastructurev1beta2.NLBBackendSet{
				{Name: "primary-set"},
				{Name: "rollout-set"},
			},
		},
	}

	listeners, backendSets := scope.buildDesiredLBListenersAndBackendSets(lb)
	if len(backendSets) != 2 {
		t.Fatalf("expected two backend sets, got %#v", backendSets)
	}
	if listeners[APIServerLBListener].DefaultBackendSetName == nil || *listeners[APIServerLBListener].DefaultBackendSetName != "primary-set" {
		t.Fatalf("expected listener to reference first backend set, got %#v", listeners[APIServerLBListener])
	}
}
