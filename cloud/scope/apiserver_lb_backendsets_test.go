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
					Name:         "rollout-set",
					ListenerPort: int32Ptr(9345),
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
	if len(listeners) != 2 {
		t.Fatalf("expected two listeners, got %#v", listeners)
	}
	if listeners[APIServerLBListener].DefaultBackendSetName == nil || *listeners[APIServerLBListener].DefaultBackendSetName != "primary-set" {
		t.Fatalf("expected listener to reference first backend set, got %#v", listeners[APIServerLBListener])
	}
	secondaryListenerName := desiredAPIServerListenerName(1, 2, "rollout-set")
	if listeners[secondaryListenerName].Port == nil || *listeners[secondaryListenerName].Port != 9345 {
		t.Fatalf("expected secondary listener to use custom port, got %#v", listeners[secondaryListenerName])
	}
	if backendSets["rollout-set"].HealthChecker == nil || backendSets["rollout-set"].HealthChecker.UrlPath == nil || *backendSets["rollout-set"].HealthChecker.UrlPath != "/readyz" {
		t.Fatalf("expected rollout backend set health checker override, got %#v", backendSets["rollout-set"])
	}
}

func int32Ptr(v int32) *int32 {
	return &v
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
	if len(listeners) != 2 {
		t.Fatalf("expected two listeners, got %#v", listeners)
	}
	if listeners[APIServerLBListener].DefaultBackendSetName == nil || *listeners[APIServerLBListener].DefaultBackendSetName != "primary-set" {
		t.Fatalf("expected listener to reference first backend set, got %#v", listeners[APIServerLBListener])
	}
}

func TestLBSpecPreservesBackendSets(t *testing.T) {
	secondaryPort := int32(9345)
	scope := &ClusterScope{
		Cluster: &clusterv1.Cluster{},
		OCIClusterAccessor: OCISelfManagedCluster{
			OCICluster: &infrastructurev1beta2.OCICluster{
				Spec: infrastructurev1beta2.OCIClusterSpec{
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						APIServerLB: infrastructurev1beta2.LoadBalancer{
							NLBSpec: infrastructurev1beta2.NLBSpec{
								BackendSets: []infrastructurev1beta2.NLBBackendSet{
									{Name: APIServerLBBackendSetName},
									{Name: "apiserver-lb-backendset-2", ListenerPort: &secondaryPort},
								},
							},
						},
					},
				},
			},
		},
	}

	got := scope.LBSpec()
	if len(got.NLBSpec.BackendSets) != 2 {
		t.Fatalf("expected backend sets to be preserved, got %#v", got.NLBSpec.BackendSets)
	}
	if got.NLBSpec.BackendSets[1].Name != "apiserver-lb-backendset-2" {
		t.Fatalf("expected secondary backend set to be preserved, got %#v", got.NLBSpec.BackendSets)
	}
}
