//go:build integration

/*
Copyright (c) 2026 Oracle and/or its affiliates.

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

package integration_test

import (
	"context"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	baseservice "github.com/oracle/cluster-api-provider-oci/cloud/services/base"
	okeservice "github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const integrationKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: integration
  cluster:
    certificate-authority-data: Y2E=
    server: https://10.0.0.2:6443
contexts:
- name: integration
  context:
    cluster: integration
    user: integration
current-context: integration
users:
- name: integration
  user: {}
`

type fakeBaseClient struct {
	mu sync.Mutex

	tokenCount int
}

func (f *fakeBaseClient) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenCount = 0
}

func (f *fakeBaseClient) GenerateToken(context.Context, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenCount++
	return "integration-token", nil
}

func (f *fakeBaseClient) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenCount
}

var _ baseservice.BaseClient = &fakeBaseClient{}

type fakeContainerEngineClient struct {
	okeservice.Client

	mu sync.Mutex

	nextID               int
	clusters             map[string]oke.Cluster
	workRequests         map[string]fakeWorkRequestResource
	createCount          int
	updateCount          int
	deleteCount          int
	kubeconfigCount      int
	nodePools            map[string]oke.NodePool
	nodePoolCreateCount  int
	nodePoolUpdateCount  int
	nodePoolDeleteCount  int
	nodePoolInspectCount int
}

type fakeWorkRequestResource struct {
	entityType string
	id         string
}

func newFakeContainerEngineClient() *fakeContainerEngineClient {
	f := &fakeContainerEngineClient{}
	f.reset()
	return f
}

func (f *fakeContainerEngineClient) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID = 0
	f.clusters = map[string]oke.Cluster{}
	f.workRequests = map[string]fakeWorkRequestResource{}
	f.nodePools = map[string]oke.NodePool{}
	f.createCount = 0
	f.updateCount = 0
	f.deleteCount = 0
	f.kubeconfigCount = 0
	f.nodePoolCreateCount = 0
	f.nodePoolUpdateCount = 0
	f.nodePoolDeleteCount = 0
	f.nodePoolInspectCount = 0
}

func (f *fakeContainerEngineClient) counts() (active, creates, updates, deletes, kubeconfigs int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.clusters), f.createCount, f.updateCount, f.deleteCount, f.kubeconfigCount
}

func (f *fakeContainerEngineClient) nodePoolCounts() (active, creates, updates, deletes, inspections int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.nodePools), f.nodePoolCreateCount, f.nodePoolUpdateCount, f.nodePoolDeleteCount, f.nodePoolInspectCount
}

func (f *fakeContainerEngineClient) CreateCluster(_ context.Context, request oke.CreateClusterRequest) (oke.CreateClusterResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.createCount++
	id := "ocid1.cluster.oc1..integration-" + strconv.Itoa(f.nextID)
	workRequestID := "work-request-create-" + id
	details := request.CreateClusterDetails
	clusterType := details.Type
	if clusterType == "" {
		clusterType = oke.ClusterTypeBasicCluster
	}
	options := normalizedOKEOptions(details.Options)
	cluster := oke.Cluster{
		Id:                       common.String(id),
		Name:                     details.Name,
		CompartmentId:            details.CompartmentId,
		EndpointConfig:           &oke.ClusterEndpointConfig{SubnetId: details.EndpointConfig.SubnetId, NsgIds: append([]string(nil), details.EndpointConfig.NsgIds...)},
		VcnId:                    details.VcnId,
		KubernetesVersion:        details.KubernetesVersion,
		KmsKeyId:                 details.KmsKeyId,
		FreeformTags:             cloneStringMap(details.FreeformTags),
		DefinedTags:              cloneDefinedTags(details.DefinedTags),
		Options:                  options,
		LifecycleState:           oke.ClusterLifecycleStateActive,
		Endpoints:                &oke.ClusterEndpoints{PublicEndpoint: common.String("198.51.100.10"), PrivateEndpoint: common.String("10.0.0.2:6443")},
		ImagePolicyConfig:        &oke.ImagePolicyConfig{IsPolicyEnabled: common.Bool(false), KeyDetails: []oke.KeyDetails{}},
		ClusterPodNetworkOptions: append([]oke.ClusterPodNetworkOptionDetails(nil), details.ClusterPodNetworkOptions...),
		Type:                     clusterType,
	}
	f.clusters[id] = cluster
	f.workRequests[workRequestID] = fakeWorkRequestResource{entityType: "cluster", id: id}
	return oke.CreateClusterResponse{
		OpcWorkRequestId: common.String(workRequestID),
		OpcRequestId:     common.String("request-create-" + id),
	}, nil
}

func (f *fakeContainerEngineClient) GetWorkRequest(_ context.Context, request oke.GetWorkRequestRequest) (oke.GetWorkRequestResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	resource, ok := f.workRequests[stringValue(request.WorkRequestId)]
	if !ok {
		return oke.GetWorkRequestResponse{}, ociutil.ErrNotFound
	}
	return oke.GetWorkRequestResponse{
		WorkRequest: oke.WorkRequest{
			Id: request.WorkRequestId,
			Resources: []oke.WorkRequestResource{{
				EntityType: common.String(resource.entityType),
				Identifier: common.String(resource.id),
			}},
		},
		OpcRequestId: common.String("request-work-" + resource.id),
	}, nil
}

func (f *fakeContainerEngineClient) GetCluster(_ context.Context, request oke.GetClusterRequest) (oke.GetClusterResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cluster, ok := f.clusters[stringValue(request.ClusterId)]
	if !ok {
		return oke.GetClusterResponse{}, ociutil.ErrNotFound
	}
	return oke.GetClusterResponse{Cluster: cluster}, nil
}

func (f *fakeContainerEngineClient) ListClusters(_ context.Context, request oke.ListClustersRequest) (oke.ListClustersResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]oke.ClusterSummary, 0, len(f.clusters))
	for _, cluster := range f.clusters {
		if request.Name != nil && stringValue(cluster.Name) != stringValue(request.Name) {
			continue
		}
		items = append(items, oke.ClusterSummary{
			Id:                       cluster.Id,
			Name:                     cluster.Name,
			CompartmentId:            cluster.CompartmentId,
			EndpointConfig:           cluster.EndpointConfig,
			VcnId:                    cluster.VcnId,
			KubernetesVersion:        cluster.KubernetesVersion,
			FreeformTags:             cloneStringMap(cluster.FreeformTags),
			DefinedTags:              cloneDefinedTags(cluster.DefinedTags),
			Options:                  cluster.Options,
			LifecycleState:           cluster.LifecycleState,
			Endpoints:                cluster.Endpoints,
			ImagePolicyConfig:        cluster.ImagePolicyConfig,
			ClusterPodNetworkOptions: append([]oke.ClusterPodNetworkOptionDetails(nil), cluster.ClusterPodNetworkOptions...),
			Type:                     cluster.Type,
		})
	}
	return oke.ListClustersResponse{Items: items}, nil
}

func (f *fakeContainerEngineClient) UpdateCluster(_ context.Context, _ oke.UpdateClusterRequest) (oke.UpdateClusterResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCount++
	return oke.UpdateClusterResponse{}, nil
}

func (f *fakeContainerEngineClient) DeleteCluster(_ context.Context, request oke.DeleteClusterRequest) (oke.DeleteClusterResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.ClusterId)
	if _, ok := f.clusters[id]; !ok {
		return oke.DeleteClusterResponse{}, ociutil.ErrNotFound
	}
	delete(f.clusters, id)
	f.deleteCount++
	return oke.DeleteClusterResponse{OpcWorkRequestId: common.String("work-request-delete-" + id)}, nil
}

func (f *fakeContainerEngineClient) CreateKubeconfig(context.Context, oke.CreateKubeconfigRequest) (oke.CreateKubeconfigResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kubeconfigCount++
	return oke.CreateKubeconfigResponse{
		Content:      io.NopCloser(strings.NewReader(integrationKubeconfig)),
		OpcRequestId: common.String("request-kubeconfig"),
	}, nil
}

func (f *fakeContainerEngineClient) CreateNodePool(_ context.Context, request oke.CreateNodePoolRequest) (oke.CreateNodePoolResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.nodePoolCreateCount++
	id := "ocid1.nodepool.oc1..integration-" + strconv.Itoa(f.nextID)
	workRequestID := "work-request-create-" + id
	details := request.CreateNodePoolDetails
	nodeConfig := normalizeNodePoolConfig(details.NodeConfigDetails)
	nodePool := oke.NodePool{
		Id:                           common.String(id),
		LifecycleState:               oke.NodePoolLifecycleStateActive,
		CompartmentId:                details.CompartmentId,
		ClusterId:                    details.ClusterId,
		Name:                         details.Name,
		KubernetesVersion:            details.KubernetesVersion,
		NodeMetadata:                 cloneStringMap(details.NodeMetadata),
		NodeShapeConfig:              normalizeNodeShapeConfig(details.NodeShapeConfig),
		NodeSourceDetails:            normalizeNodeSource(details.NodeSourceDetails),
		NodeShape:                    details.NodeShape,
		InitialNodeLabels:            append([]oke.KeyValue(nil), details.InitialNodeLabels...),
		SshPublicKey:                 details.SshPublicKey,
		NodeConfigDetails:            nodeConfig,
		FreeformTags:                 cloneStringMap(details.FreeformTags),
		DefinedTags:                  cloneDefinedTags(details.DefinedTags),
		NodeEvictionNodePoolSettings: details.NodeEvictionNodePoolSettings,
		NodePoolCyclingDetails:       details.NodePoolCyclingDetails,
	}
	nodePool.Nodes = fakeNodesForPool(&nodePool)
	f.nodePools[id] = nodePool
	f.workRequests[workRequestID] = fakeWorkRequestResource{entityType: "nodepool", id: id}
	return oke.CreateNodePoolResponse{
		OpcWorkRequestId: common.String(workRequestID),
		OpcRequestId:     common.String("request-create-" + id),
	}, nil
}

func (f *fakeContainerEngineClient) GetNodePool(_ context.Context, request oke.GetNodePoolRequest) (oke.GetNodePoolResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nodePoolInspectCount++
	nodePool, ok := f.nodePools[stringValue(request.NodePoolId)]
	if !ok {
		return oke.GetNodePoolResponse{}, ociutil.ErrNotFound
	}
	return oke.GetNodePoolResponse{NodePool: nodePool}, nil
}

func (f *fakeContainerEngineClient) ListNodePools(_ context.Context, request oke.ListNodePoolsRequest) (oke.ListNodePoolsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nodePoolInspectCount++
	items := make([]oke.NodePoolSummary, 0, len(f.nodePools))
	for _, nodePool := range f.nodePools {
		if request.Name != nil && stringValue(nodePool.Name) != stringValue(request.Name) {
			continue
		}
		items = append(items, nodePoolSummary(nodePool))
	}
	return oke.ListNodePoolsResponse{Items: items}, nil
}

func (f *fakeContainerEngineClient) UpdateNodePool(_ context.Context, request oke.UpdateNodePoolRequest) (oke.UpdateNodePoolResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.NodePoolId)
	nodePool, ok := f.nodePools[id]
	if !ok {
		return oke.UpdateNodePoolResponse{}, ociutil.ErrNotFound
	}
	f.nodePoolUpdateCount++
	details := request.UpdateNodePoolDetails
	nodePool.Name = details.Name
	nodePool.KubernetesVersion = details.KubernetesVersion
	nodePool.NodeShape = details.NodeShape
	nodePool.NodeShapeConfig = normalizeUpdateNodeShapeConfig(details.NodeShapeConfig)
	nodePool.NodeSourceDetails = normalizeNodeSource(details.NodeSourceDetails)
	nodePool.SshPublicKey = details.SshPublicKey
	nodePool.NodeMetadata = cloneStringMap(details.NodeMetadata)
	nodePool.InitialNodeLabels = append([]oke.KeyValue(nil), details.InitialNodeLabels...)
	if details.NodeConfigDetails != nil {
		applyNodePoolConfigUpdate(nodePool.NodeConfigDetails, details.NodeConfigDetails)
	}
	if details.NodeEvictionNodePoolSettings != nil {
		nodePool.NodeEvictionNodePoolSettings = details.NodeEvictionNodePoolSettings
	}
	if details.NodePoolCyclingDetails != nil {
		nodePool.NodePoolCyclingDetails = details.NodePoolCyclingDetails
	}
	nodePool.Nodes = fakeNodesForPool(&nodePool)
	f.nodePools[id] = nodePool
	return oke.UpdateNodePoolResponse{OpcWorkRequestId: common.String("work-request-update-" + id)}, nil
}

func (f *fakeContainerEngineClient) DeleteNodePool(_ context.Context, request oke.DeleteNodePoolRequest) (oke.DeleteNodePoolResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := stringValue(request.NodePoolId)
	if _, ok := f.nodePools[id]; !ok {
		return oke.DeleteNodePoolResponse{}, ociutil.ErrNotFound
	}
	delete(f.nodePools, id)
	f.nodePoolDeleteCount++
	return oke.DeleteNodePoolResponse{OpcWorkRequestId: common.String("work-request-delete-" + id)}, nil
}

func normalizeNodePoolConfig(input *oke.CreateNodePoolNodeConfigDetails) *oke.NodePoolNodeConfigDetails {
	if input == nil {
		return &oke.NodePoolNodeConfigDetails{}
	}
	podNetwork := input.NodePoolPodNetworkOptionDetails
	if details, ok := podNetwork.(oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails); ok {
		if details.MaxPodsPerNode == nil {
			details.MaxPodsPerNode = common.Int(31)
		}
		podNetwork = details
	}
	return &oke.NodePoolNodeConfigDetails{
		Size:                            input.Size,
		NsgIds:                          append([]string(nil), input.NsgIds...),
		KmsKeyId:                        input.KmsKeyId,
		IsPvEncryptionInTransitEnabled:  input.IsPvEncryptionInTransitEnabled,
		FreeformTags:                    cloneStringMap(input.FreeformTags),
		DefinedTags:                     cloneDefinedTags(input.DefinedTags),
		PlacementConfigs:                append([]oke.NodePoolPlacementConfigDetails(nil), input.PlacementConfigs...),
		NodePoolPodNetworkOptionDetails: podNetwork,
	}
}

func applyNodePoolConfigUpdate(actual *oke.NodePoolNodeConfigDetails, update *oke.UpdateNodePoolNodeConfigDetails) {
	if actual == nil || update == nil {
		return
	}
	if update.Size != nil {
		actual.Size = update.Size
	}
	actual.NsgIds = append([]string(nil), update.NsgIds...)
	actual.KmsKeyId = update.KmsKeyId
	actual.IsPvEncryptionInTransitEnabled = update.IsPvEncryptionInTransitEnabled
	if len(update.PlacementConfigs) > 0 {
		actual.PlacementConfigs = append([]oke.NodePoolPlacementConfigDetails(nil), update.PlacementConfigs...)
	}
	if update.NodePoolPodNetworkOptionDetails != nil {
		podNetwork := update.NodePoolPodNetworkOptionDetails
		if details, ok := podNetwork.(oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails); ok {
			if details.MaxPodsPerNode == nil {
				details.MaxPodsPerNode = common.Int(31)
			}
			podNetwork = details
		}
		actual.NodePoolPodNetworkOptionDetails = podNetwork
	}
}

func normalizeNodeShapeConfig(input *oke.CreateNodeShapeConfigDetails) *oke.NodeShapeConfig {
	if input == nil {
		return nil
	}
	return &oke.NodeShapeConfig{Ocpus: input.Ocpus, MemoryInGBs: input.MemoryInGBs}
}

func normalizeUpdateNodeShapeConfig(input *oke.UpdateNodeShapeConfigDetails) *oke.NodeShapeConfig {
	if input == nil {
		return nil
	}
	return &oke.NodeShapeConfig{Ocpus: input.Ocpus, MemoryInGBs: input.MemoryInGBs}
}

func normalizeNodeSource(input oke.NodeSourceDetails) oke.NodeSourceDetails {
	switch source := input.(type) {
	case *oke.NodeSourceViaImageDetails:
		return *source
	case oke.NodeSourceViaImageDetails:
		return source
	default:
		return input
	}
}

func fakeNodesForPool(nodePool *oke.NodePool) []oke.Node {
	if nodePool == nil || nodePool.NodeConfigDetails == nil || nodePool.NodeConfigDetails.Size == nil {
		return nil
	}
	var availabilityDomain, subnetID *string
	if len(nodePool.NodeConfigDetails.PlacementConfigs) > 0 {
		availabilityDomain = nodePool.NodeConfigDetails.PlacementConfigs[0].AvailabilityDomain
		subnetID = nodePool.NodeConfigDetails.PlacementConfigs[0].SubnetId
	}
	nodes := make([]oke.Node, 0, *nodePool.NodeConfigDetails.Size)
	for i := 0; i < *nodePool.NodeConfigDetails.Size; i++ {
		number := strconv.Itoa(i + 1)
		nodes = append(nodes, oke.Node{
			Id:                 common.String("ocid1.instance.oc1..managed-node-" + number),
			Name:               common.String("managed-node-" + number),
			KubernetesVersion:  nodePool.KubernetesVersion,
			AvailabilityDomain: availabilityDomain,
			SubnetId:           subnetID,
			NodePoolId:         nodePool.Id,
			LifecycleState:     oke.NodeLifecycleStateActive,
		})
	}
	return nodes
}

func nodePoolSummary(nodePool oke.NodePool) oke.NodePoolSummary {
	return oke.NodePoolSummary{
		Id:                           nodePool.Id,
		LifecycleState:               nodePool.LifecycleState,
		CompartmentId:                nodePool.CompartmentId,
		ClusterId:                    nodePool.ClusterId,
		Name:                         nodePool.Name,
		KubernetesVersion:            nodePool.KubernetesVersion,
		NodeShapeConfig:              nodePool.NodeShapeConfig,
		NodeSourceDetails:            nodePool.NodeSourceDetails,
		NodeShape:                    nodePool.NodeShape,
		InitialNodeLabels:            append([]oke.KeyValue(nil), nodePool.InitialNodeLabels...),
		SshPublicKey:                 nodePool.SshPublicKey,
		NodeConfigDetails:            nodePool.NodeConfigDetails,
		FreeformTags:                 cloneStringMap(nodePool.FreeformTags),
		DefinedTags:                  cloneDefinedTags(nodePool.DefinedTags),
		NodeEvictionNodePoolSettings: nodePool.NodeEvictionNodePoolSettings,
		NodePoolCyclingDetails:       nodePool.NodePoolCyclingDetails,
	}
}

func normalizedOKEOptions(input *oke.ClusterCreateOptions) *oke.ClusterCreateOptions {
	options := &oke.ClusterCreateOptions{}
	if input != nil {
		*options = *input
	}
	if options.AdmissionControllerOptions == nil {
		options.AdmissionControllerOptions = &oke.AdmissionControllerOptions{
			IsPodSecurityPolicyEnabled: common.Bool(false),
		}
	}
	if options.AddOns == nil {
		options.AddOns = &oke.AddOnOptions{
			IsTillerEnabled:              common.Bool(false),
			IsKubernetesDashboardEnabled: common.Bool(false),
		}
	}
	return options
}

func cloneDefinedTags(input map[string]map[string]interface{}) map[string]map[string]interface{} {
	if input == nil {
		return nil
	}
	output := make(map[string]map[string]interface{}, len(input))
	for namespace, tags := range input {
		output[namespace] = make(map[string]interface{}, len(tags))
		for key, value := range tags {
			output[namespace][key] = value
		}
	}
	return output
}

var _ okeservice.Client = &fakeContainerEngineClient{}
