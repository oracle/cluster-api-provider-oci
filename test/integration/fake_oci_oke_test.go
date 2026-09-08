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

	nextID          int
	clusters        map[string]oke.Cluster
	workRequestIDs  map[string]string
	createCount     int
	updateCount     int
	deleteCount     int
	kubeconfigCount int
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
	f.workRequestIDs = map[string]string{}
	f.createCount = 0
	f.updateCount = 0
	f.deleteCount = 0
	f.kubeconfigCount = 0
}

func (f *fakeContainerEngineClient) counts() (active, creates, updates, deletes, kubeconfigs int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.clusters), f.createCount, f.updateCount, f.deleteCount, f.kubeconfigCount
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
	f.workRequestIDs[workRequestID] = id
	return oke.CreateClusterResponse{
		OpcWorkRequestId: common.String(workRequestID),
		OpcRequestId:     common.String("request-create-" + id),
	}, nil
}

func (f *fakeContainerEngineClient) GetWorkRequest(_ context.Context, request oke.GetWorkRequestRequest) (oke.GetWorkRequestResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	clusterID, ok := f.workRequestIDs[stringValue(request.WorkRequestId)]
	if !ok {
		return oke.GetWorkRequestResponse{}, ociutil.ErrNotFound
	}
	return oke.GetWorkRequestResponse{
		WorkRequest: oke.WorkRequest{
			Id: request.WorkRequestId,
			Resources: []oke.WorkRequestResource{{
				EntityType: common.String("cluster"),
				Identifier: common.String(clusterID),
			}},
		},
		OpcRequestId: common.String("request-work-" + clusterID),
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
