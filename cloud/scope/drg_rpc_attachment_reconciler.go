/*
Copyright 2022 The Kubernetes Authors.

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
	"context"
	"time"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	vcn "github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	PollInterval   = 5 * time.Second
	RequestTimeout = 2 * time.Minute
)

// ReconcileDRGRPCAttachment reconciles DRG RPC attachments
func (s *ClusterScope) ReconcileDRGRPCAttachment(ctx context.Context) error {
	if !s.isPeeringEnabled() {
		s.Logger.Info("VCN Peering is not enabled, ignoring reconciliation")
		return nil
	}

	if s.getDRG() == nil {
		return errors.New("DRG has not been specified")
	}
	if s.getDrgID() == nil {
		return errors.New("DRG ID has not been set")
	}

	for _, rpcSpec := range s.OCICluster.Spec.NetworkSpec.VCNPeering.RemotePeeringConnections {
		localRpc, err := s.lookupRPC(ctx, s.getDrgID(), rpcSpec.RPCConnectionId, s.VCNClient)
		if err != nil {
			return err
		}
		if localRpc != nil {
			rpcSpec.RPCConnectionId = localRpc.Id
			s.Logger.Info("Local RPC exists", "rpcId", localRpc.Id)
			if !s.IsTagsEqual(localRpc.FreeformTags, localRpc.DefinedTags) {
				_, err := s.updateDRGRpc(ctx, localRpc.Id, s.VCNClient)
				if err != nil {
					return err
				}
				s.Logger.Info("Local RPC has been updated", "rpcId", localRpc.Id)
			}
		} else {
			localRpc, err = s.createRPC(ctx, s.getDrgID(), s.OCICluster.Name, s.VCNClient)
			if err != nil {
				return err
			}
			s.Logger.Info("Local RPC has been created", "rpcId", localRpc.Id)
			rpcSpec.RPCConnectionId = localRpc.Id
		}
		err = s.waitForRPCToBeProvisioned(ctx, localRpc, s.VCNClient)
		if err != nil {
			return err
		}

		if rpcSpec.PeerDRGId == nil {
			return errors.New("peer DRG ID has not been specified")
		}
		if rpcSpec.ManagePeerRPC {
			clientProvider, err := s.ClientProvider.GetOrBuildClient(rpcSpec.PeerRegionName)
			if err != nil {
				return err
			}
			remoteRpc, err := s.lookupRPC(ctx, rpcSpec.PeerDRGId, rpcSpec.PeerRPCConnectionId, clientProvider.VCNClient)
			if err != nil {
				return err
			}
			if remoteRpc != nil {
				s.Logger.Info("Remote RPC exists", "rpcId", localRpc.Id)
				s.Logger.Info("Connection status of 2 peered RPCs", "status", localRpc.PeeringStatus)
				rpcSpec.PeerRPCConnectionId = remoteRpc.Id
				if !s.IsTagsEqual(remoteRpc.FreeformTags, remoteRpc.DefinedTags) {
					_, err := s.updateDRGRpc(ctx, remoteRpc.Id, clientProvider.VCNClient)
					if err != nil {
						return err
					}
					s.Logger.Info("Remote RPC has been updated", "rpcId", remoteRpc.Id)
				}
			} else {
				remoteRpc, err = s.createRPC(ctx, rpcSpec.PeerDRGId, s.OCICluster.Name, clientProvider.VCNClient)
				if err != nil {
					return err
				}
				s.Logger.Info("Remote RPC has been created", "rpcId", remoteRpc.Id)
				rpcSpec.PeerRPCConnectionId = remoteRpc.Id
			}
			err = s.waitForRPCToBeProvisioned(ctx, remoteRpc, clientProvider.VCNClient)
			if err != nil {
				return err
			}
		}

		if localRpc.PeeringStatus == core.RemotePeeringConnectionPeeringStatusNew {
			if rpcSpec.PeerRPCConnectionId == nil {
				return errors.New("peer RPC Connection ID is empty")
			}
			_, err := s.VCNClient.ConnectRemotePeeringConnections(ctx, core.ConnectRemotePeeringConnectionsRequest{
				RemotePeeringConnectionId: rpcSpec.RPCConnectionId,
				ConnectRemotePeeringConnectionsDetails: core.ConnectRemotePeeringConnectionsDetails{
					PeerId:         rpcSpec.PeerRPCConnectionId,
					PeerRegionName: common.String(rpcSpec.PeerRegionName),
				},
			})
			if err != nil {
				return err
			}
			s.Logger.Info("Connect request initiated for local and peer RPCs", "rpcId", rpcSpec.RPCConnectionId)
		}
		if localRpc.PeeringStatus != core.RemotePeeringConnectionPeeringStatusPeered {
			err := wait.PollWithContext(ctx, PollInterval, RequestTimeout, func(ctx context.Context) (done bool, err error) {
				rpc, err := s.getRPC(ctx, localRpc.Id, s.VCNClient)
				if err != nil {
					return true, err
				}
				s.Logger.Info("RPC peering status", "rpcId", rpc.Id, "peeringStatus", localRpc.PeeringStatus)
				switch rpc.PeeringStatus {
				case core.RemotePeeringConnectionPeeringStatusPeered:
					return true, nil
				case core.RemotePeeringConnectionPeeringStatusRevoked, core.RemotePeeringConnectionPeeringStatusInvalid,
					core.RemotePeeringConnectionPeeringStatusNew:
					return true, errors.Errorf("invalid peering status %s of RPC %s", rpc.PeeringStatus, *rpc.Id)
				case core.RemotePeeringConnectionPeeringStatusPending:
					return false, nil
				}
				return false, nil
			})
			if err != nil {
				return err
			}
		}
		s.Logger.Info("Connection has been established between the 2 peered RPCs")
	}
	return nil
}

func (s *ClusterScope) createRPC(ctx context.Context, drgId *string, displayName string, vcnClient vcn.Client) (*core.RemotePeeringConnection, error) {
	response, err := vcnClient.CreateRemotePeeringConnection(ctx, core.CreateRemotePeeringConnectionRequest{
		CreateRemotePeeringConnectionDetails: core.CreateRemotePeeringConnectionDetails{
			DisplayName:   common.String(displayName),
			DrgId:         drgId,
			FreeformTags:  s.GetFreeFormTags(),
			DefinedTags:   s.GetDefinedTags(),
			CompartmentId: common.String(s.GetCompartmentId()),
		},
	})
	if err != nil {
		return nil, err
	}

	return &response.RemotePeeringConnection, nil
}

func (s *ClusterScope) lookupRPC(ctx context.Context, drgId *string, rpcId *string, vcnClient vcn.Client) (*core.RemotePeeringConnection, error) {
	if rpcId != nil {
		attachment, err := s.getRPC(ctx, rpcId, vcnClient)
		if err != nil {
			return nil, err
		}
		if s.IsResourceCreatedByClusterAPI(attachment.FreeformTags) {
			return attachment, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	} else {
		var rpcs []core.RemotePeeringConnection
		for {
			var page *string
			response, err := vcnClient.ListRemotePeeringConnections(ctx, core.ListRemotePeeringConnectionsRequest{
				DrgId:         drgId,
				CompartmentId: common.String(s.GetCompartmentId()),
				Page:          page,
			})
			if err != nil {
				return nil, err
			}
			for _, rpc := range response.Items {
				if *rpc.DisplayName == s.OCICluster.Name {
					if s.IsResourceCreatedByClusterAPI(rpc.FreeformTags) {
						rpcs = append(rpcs, rpc)
					} else {
						return nil, errors.New("cluster api tags have been modified out of context")
					}
				}
			}
			if response.OpcNextPage == nil {
				break
			} else {
				page = response.OpcNextPage
			}
		}

		if len(rpcs) == 0 {
			return nil, nil
		} else if len(rpcs) > 1 {
			return nil, errors.New("found more than one attachment with same display name")
		} else {
			return &rpcs[0], nil
		}
	}
}

func (s *ClusterScope) updateDRGRpc(ctx context.Context, rpcId *string, vcnClient vcn.Client) (*core.RemotePeeringConnection, error) {
	response, err := vcnClient.UpdateRemotePeeringConnection(ctx, core.UpdateRemotePeeringConnectionRequest{
		RemotePeeringConnectionId: rpcId,
		UpdateRemotePeeringConnectionDetails: core.UpdateRemotePeeringConnectionDetails{
			FreeformTags: s.GetFreeFormTags(),
			DefinedTags:  s.GetDefinedTags(),
		},
	})
	if err != nil {
		return nil, err
	}
	return &response.RemotePeeringConnection, nil
}

func (s *ClusterScope) getRPC(ctx context.Context, rpcId *string, vcnClient vcn.Client) (*core.RemotePeeringConnection, error) {
	response, err := vcnClient.GetRemotePeeringConnection(ctx, core.GetRemotePeeringConnectionRequest{
		RemotePeeringConnectionId: rpcId,
	})
	if err != nil {
		return nil, err
	}
	return &response.RemotePeeringConnection, nil
}

func (s *ClusterScope) deleteRPC(ctx context.Context, rpcId *string, vcnClient vcn.Client) error {
	_, err := vcnClient.DeleteRemotePeeringConnection(ctx, core.DeleteRemotePeeringConnectionRequest{
		RemotePeeringConnectionId: rpcId,
	})
	if err != nil {
		return err
	}
	err = s.waitForRPCToBeDeleted(ctx, rpcId, vcnClient)
	if err != nil {
		return err
	}
	return nil
}

func (s *ClusterScope) waitForRPCToBeProvisioned(ctx context.Context, rpc *core.RemotePeeringConnection, vcnClient vcn.Client) error {
	switch rpc.LifecycleState {
	case core.RemotePeeringConnectionLifecycleStateAvailable:
		return nil
	case core.RemotePeeringConnectionLifecycleStateTerminating, core.RemotePeeringConnectionLifecycleStateTerminated:
		return errors.New("invalid RPC lifecycle state")
	}

	err := wait.PollWithContext(ctx, PollInterval, RequestTimeout, func(ctx context.Context) (done bool, err error) {
		rpc, err := s.getRPC(ctx, rpc.Id, vcnClient)
		if err != nil {
			return true, err
		}
		s.Logger.Info("RPC lifecycle state", "rpcId", rpc.Id, "state", rpc.LifecycleState)
		switch rpc.LifecycleState {
		case core.RemotePeeringConnectionLifecycleStateAvailable:
			return true, nil
		case core.RemotePeeringConnectionLifecycleStateTerminating, core.RemotePeeringConnectionLifecycleStateTerminated:
			return true, errors.New("invalid RPC lifecycle state")
		}
		return false, nil
	})
	return err
}

func (s *ClusterScope) waitForRPCToBeDeleted(ctx context.Context, rpcId *string, vcnClient vcn.Client) error {
	err := wait.PollWithContext(ctx, PollInterval, RequestTimeout, func(ctx context.Context) (done bool, err error) {
		rpc, err := s.getRPC(ctx, rpcId, vcnClient)
		if err != nil {
			if ociutil.IsNotFound(err) {
				return true, nil
			} else {
				return true, err
			}
		}
		if rpc == nil || rpc.LifecycleState == core.RemotePeeringConnectionLifecycleStateTerminated {
			return true, nil
		}
		return false, nil
	})
	return err
}

// DeleteDRGRPCAttachment deletes DRG RPC attachments
func (s *ClusterScope) DeleteDRGRPCAttachment(ctx context.Context) error {
	if !s.isPeeringEnabled() {
		s.Logger.Info("VCN Peering is not enabled, ignoring reconciliation")
		return nil
	}

	if s.getDRG() == nil {
		s.Logger.Info("DRG has not been specified")
		return nil
	}
	if s.getDrgID() == nil {
		s.Logger.Info("DRG ID has not been set")
		return nil
	}

	for _, rpcSpec := range s.OCICluster.Spec.NetworkSpec.VCNPeering.RemotePeeringConnections {
		localRpc, err := s.lookupRPC(ctx, s.getDrgID(), rpcSpec.RPCConnectionId, s.VCNClient)
		if err != nil && !ociutil.IsNotFound(err) {
			return err
		}
		if localRpc == nil {
			s.Logger.Info("Local RPC is already deleted")
			return nil
		} else {
			err := s.deleteRPC(ctx, localRpc.Id, s.VCNClient)
			if err != nil {
				return err
			}
			s.Logger.Info("Local RPC has been deleted", "rpcId", localRpc.Id)
		}
		if rpcSpec.ManagePeerRPC {
			clientProvider, err := s.ClientProvider.GetOrBuildClient(rpcSpec.PeerRegionName)
			if err != nil {
				return err
			}
			remoteRpc, err := s.lookupRPC(ctx, rpcSpec.PeerDRGId, rpcSpec.PeerRPCConnectionId, clientProvider.VCNClient)
			if err != nil && !ociutil.IsNotFound(err) {
				return err
			}
			if remoteRpc == nil {
				s.Logger.Info("Remote RPC is already deleted")
				return nil
			} else {
				err := s.deleteRPC(ctx, remoteRpc.Id, clientProvider.VCNClient)
				if err != nil {
					return err
				}
				s.Logger.Info("Remote RPC has been deleted", "rpcId", remoteRpc.Id)
			}
		}
	}
	return nil
}
