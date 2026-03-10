package v1beta2

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLookupClusterAPIServerPort(t *testing.T) {
	t.Run("uses owner cluster api server port when set", func(t *testing.T) {
		scheme := runtime.NewScheme()
		if err := clusterv1.AddToScheme(scheme); err != nil {
			t.Fatalf("failed to add cluster api scheme: %v", err)
		}

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
				UID:       "cluster-uid",
			},
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: clusterv1.ClusterNetwork{APIServerPort: 7443},
			},
		}

		client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
		objMeta := metav1.ObjectMeta{
			Name:            "test-oci-cluster",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{newClusterOwnerReference(cluster)},
		}

		port, err := lookupClusterAPIServerPort(context.Background(), client, objMeta)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if port == nil || *port != 7443 {
			t.Fatalf("expected api server port 7443, got %v", port)
		}
	})

	t.Run("falls back to cluster-name label and default port", func(t *testing.T) {
		scheme := runtime.NewScheme()
		if err := clusterv1.AddToScheme(scheme); err != nil {
			t.Fatalf("failed to add cluster api scheme: %v", err)
		}

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
		}

		client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
		objMeta := metav1.ObjectMeta{
			Name:      "test-oci-cluster",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
		}

		port, err := lookupClusterAPIServerPort(context.Background(), client, objMeta)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if port == nil || *port != 6443 {
			t.Fatalf("expected api server port 6443, got %v", port)
		}
	})
}
