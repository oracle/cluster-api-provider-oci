package v1beta2

import (
	"context"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	pkgerrors "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func lookupClusterAPIServerPort(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*int32, error) {
	if c == nil {
		return nil, nil
	}

	cluster, err := util.GetOwnerCluster(ctx, c, obj)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		cluster, err = util.GetClusterFromMetadata(ctx, c, obj)
		if err != nil {
			if pkgerrors.Cause(err) == util.ErrNoCluster {
				return nil, nil
			}
			return nil, err
		}
	}
	if cluster == nil {
		return nil, nil
	}

	port := cluster.Spec.ClusterNetwork.APIServerPort
	if port == 0 {
		port = ociutil.DefaultAPIServerPort
	}
	return &port, nil
}

func newClusterOwnerReference(cluster *clusterv1.Cluster) metav1.OwnerReference {
	controller := true
	blockOwnerDeletion := true
	return metav1.OwnerReference{
		APIVersion:         clusterv1.GroupVersion.String(),
		Kind:               "Cluster",
		Name:               cluster.Name,
		UID:                cluster.UID,
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}
