/*
 Copyright (c) 2022 Oracle and/or its affiliates.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package scope

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/computemanagement"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine"
	expinfra1 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/pkg/errors"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OCIManagedMachinePoolKind = "OCIManagedMachinePool"
)

// ManagedMachinePoolScopeParams defines the params need to create a new ManagedMachinePoolScope
type ManagedMachinePoolScopeParams struct {
	Logger                  *logr.Logger
	Cluster                 *clusterv1.Cluster
	MachinePool             *expclusterv1.MachinePool
	Client                  client.Client
	ComputeManagementClient computemanagement.Client
	OCIManagedCluster       *infrastructurev1beta2.OCIManagedCluster
	OCIManagedControlPlane  *infrastructurev1beta2.OCIManagedControlPlane
	OCIManagedMachinePool   *expinfra1.OCIManagedMachinePool
	ContainerEngineClient   containerengine.Client
}

type ManagedMachinePoolScope struct {
	*logr.Logger
	Client                  client.Client
	patchHelper             *patch.Helper
	Cluster                 *clusterv1.Cluster
	MachinePool             *expclusterv1.MachinePool
	ComputeManagementClient computemanagement.Client
	OCIManagedCluster       *infrastructurev1beta2.OCIManagedCluster
	OCIManagedMachinePool   *expinfra1.OCIManagedMachinePool
	ContainerEngineClient   containerengine.Client
	OCIManagedControlPlane  *infrastructurev1beta2.OCIManagedControlPlane
}

// NewManagedMachinePoolScope creates a ManagedMachinePoolScope given the ManagedMachinePoolScopeParams
func NewManagedMachinePoolScope(params ManagedMachinePoolScopeParams) (*ManagedMachinePoolScope, error) {
	if params.MachinePool == nil {
		return nil, errors.New("failed to generate new scope from nil MachinePool")
	}
	if params.OCIManagedCluster == nil {
		return nil, errors.New("failed to generate new scope from nil OCIManagedCluster")
	}

	if params.Logger == nil {
		log := klogr.New()
		params.Logger = &log
	}
	helper, err := patch.NewHelper(params.OCIManagedMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	params.OCIManagedMachinePool.Status.InfrastructureMachineKind = "OCIMachinePoolMachine"

	return &ManagedMachinePoolScope{
		Logger:                  params.Logger,
		Client:                  params.Client,
		ComputeManagementClient: params.ComputeManagementClient,
		Cluster:                 params.Cluster,
		OCIManagedCluster:       params.OCIManagedCluster,
		patchHelper:             helper,
		MachinePool:             params.MachinePool,
		OCIManagedMachinePool:   params.OCIManagedMachinePool,
		ContainerEngineClient:   params.ContainerEngineClient,
		OCIManagedControlPlane:  params.OCIManagedControlPlane,
	}, nil
}

// PatchObject persists the cluster configuration and status.
func (m *ManagedMachinePoolScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.OCIManagedMachinePool)
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *ManagedMachinePoolScope) Close(ctx context.Context) error {
	return m.PatchObject(ctx)
}

// SetFailureReason sets the OCIMachine status error reason.
func (m *ManagedMachinePoolScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.OCIManagedMachinePool.Status.FailureReason = &v
}

func (m *ManagedMachinePoolScope) SetReplicaCount(count int32) {
	m.OCIManagedMachinePool.Status.Replicas = count
}

// GetWorkerMachineSubnet returns the WorkerRole core.Subnet id for the cluster
func (m *ManagedMachinePoolScope) GetWorkerMachineSubnet() *string {
	for _, subnet := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet != nil && subnet.Role == infrastructurev1beta2.WorkerRole {
			return subnet.ID
		}
	}
	return nil
}

// SetListandSetMachinePoolInstances retrieves a machine pools instances and sets them in the ProviderIDList
func (m *ManagedMachinePoolScope) SetListandSetMachinePoolInstances(ctx context.Context, nodePool *oke.NodePool) (int32, error) {
	providerIDList := make([]string, 0)
	for _, instance := range nodePool.Nodes {
		if instance.Id != nil && instance.LifecycleState == oke.NodeLifecycleStateActive {
			providerIDList = append(providerIDList, *instance.Id)
		}
	}
	m.OCIManagedMachinePool.Spec.ProviderIDList = providerIDList
	return int32(len(providerIDList)), nil
}

// IsResourceCreatedByClusterAPI determines if the instance was created by the cluster using the
// tags created at instance launch.
func (m *ManagedMachinePoolScope) IsResourceCreatedByClusterAPI(resourceFreeFormTags map[string]string) bool {
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(m.OCIManagedCluster.Spec.OCIResourceIdentifier)
	for k, v := range tagsAddedByClusterAPI {
		if resourceFreeFormTags[k] != v {
			return false
		}
	}
	return true
}

// FindNodePool attempts to find the node pool by id if the id exists or by name. It checks to make sure
// the node pool was created by the cluster before returning the correct pool.
// nolint:nilnil
func (m *ManagedMachinePoolScope) FindNodePool(ctx context.Context) (*oke.NodePool, error) {
	if m.OCIManagedMachinePool.Spec.ID != nil {
		response, err := m.ContainerEngineClient.GetNodePool(ctx, oke.GetNodePoolRequest{
			NodePoolId: m.OCIManagedMachinePool.Spec.ID,
		})
		if err != nil {
			return nil, err
		}
		if m.IsResourceCreatedByClusterAPI(response.FreeformTags) {
			return &response.NodePool, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}

	var page *string
	for {
		reqList := oke.ListNodePoolsRequest{
			CompartmentId: common.String(m.OCIManagedCluster.Spec.CompartmentId),
			ClusterId:     m.OCIManagedControlPlane.Spec.ID,
			Name:          common.String(m.getNodePoolName()),
			Page:          page,
		}

		response, err := m.ContainerEngineClient.ListNodePools(ctx, reqList)
		if err != nil {
			return nil, err
		}
		for _, i := range response.Items {
			if m.IsResourceCreatedByClusterAPI(i.FreeformTags) {
				return m.getOKENodePoolFromOCID(ctx, i.Id)
			}
		}
		if response.OpcNextPage == nil {
			break
		} else {
			page = response.OpcNextPage
		}
	}
	return nil, nil
}

func (m *ManagedMachinePoolScope) getNodePoolName() string {
	return m.OCIManagedMachinePool.GetName()
}

// CreateNodePool attempts to create a node pool
func (m *ManagedMachinePoolScope) CreateNodePool(ctx context.Context) (*oke.NodePool, error) {
	m.Info("Creating Node Pool")

	machinePool := m.OCIManagedMachinePool
	if machinePool.Spec.NodePoolNodeConfig == nil {
		m.OCIManagedMachinePool.Spec.NodePoolNodeConfig = &expinfra1.NodePoolNodeConfig{}
	}
	placementConfigs := m.OCIManagedMachinePool.Spec.NodePoolNodeConfig.PlacementConfigs
	if len(placementConfigs) == 0 {
		placementConfigs = make([]expinfra1.PlacementConfig, 0)
		workerSubnets := m.getWorkerMachineSubnets()
		if len(workerSubnets) == 0 {
			return nil, errors.New("worker subnets are not specified")
		}
		adMap := m.OCIManagedCluster.Spec.AvailabilityDomains
		for k, v := range adMap {
			placementConfigs = append(placementConfigs, expinfra1.PlacementConfig{
				AvailabilityDomain: common.String(k),
				FaultDomains:       v.FaultDomains,
				SubnetName:         common.String(workerSubnets[0]),
			})
		}
		m.OCIManagedMachinePool.Spec.NodePoolNodeConfig.PlacementConfigs = placementConfigs
	}
	if len(m.OCIManagedMachinePool.Spec.NodePoolNodeConfig.NsgNames) == 0 {
		m.OCIManagedMachinePool.Spec.NodePoolNodeConfig.NsgNames = m.getWorkerMachineNSGList()
	}
	placementConfig, err := m.buildPlacementConfig(placementConfigs)
	if err != nil {
		return nil, err
	}
	nodeConfigDetails := oke.CreateNodePoolNodeConfigDetails{
		Size:                           common.Int(int(*m.MachinePool.Spec.Replicas)),
		NsgIds:                         m.getWorkerMachineNSGs(),
		PlacementConfigs:               placementConfig,
		IsPvEncryptionInTransitEnabled: m.OCIManagedMachinePool.Spec.NodePoolNodeConfig.IsPvEncryptionInTransitEnabled,
		FreeformTags:                   m.getFreeFormTags(),
		DefinedTags:                    m.getDefinedTags(),
		KmsKeyId:                       m.OCIManagedMachinePool.Spec.NodePoolNodeConfig.KmsKeyId,
	}
	nodeShapeConfig := oke.CreateNodeShapeConfigDetails{}
	if machinePool.Spec.NodeShapeConfig != nil {
		ocpuString := m.OCIManagedMachinePool.Spec.NodeShapeConfig.Ocpus
		if ocpuString != nil {
			ocpus, err := strconv.ParseFloat(*ocpuString, 32)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("ocpus provided %s is not a valid floating point",
					*ocpuString))
			}
			nodeShapeConfig.Ocpus = common.Float32(float32(ocpus))
		}

		memoryInGBsString := m.OCIManagedMachinePool.Spec.NodeShapeConfig.MemoryInGBs
		if memoryInGBsString != nil {
			memoryInGBs, err := strconv.ParseFloat(*memoryInGBsString, 32)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("memoryInGBs provided %s is not a valid floating point",
					*memoryInGBsString))
			}
			nodeShapeConfig.MemoryInGBs = common.Float32(float32(memoryInGBs))
		}
	}
	err = m.setNodepoolImageId(ctx)
	if err != nil {
		return nil, err
	}
	sourceDetails := oke.NodeSourceViaImageDetails{
		ImageId:             machinePool.Spec.NodeSourceViaImage.ImageId,
		BootVolumeSizeInGBs: machinePool.Spec.NodeSourceViaImage.BootVolumeSizeInGBs,
	}
	podNetworkOptions := machinePool.Spec.NodePoolNodeConfig.NodePoolPodNetworkOptionDetails
	if podNetworkOptions != nil {
		if podNetworkOptions.CniType == infrastructurev1beta2.VCNNativeCNI {
			npnDetails := oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
				PodSubnetIds: m.getPodSubnets(podNetworkOptions.VcnIpNativePodNetworkOptions.SubnetNames),
				PodNsgIds:    m.getPodNSGs(podNetworkOptions.VcnIpNativePodNetworkOptions.NSGNames),
			}
			if podNetworkOptions.VcnIpNativePodNetworkOptions.MaxPodsPerNode != nil {
				npnDetails.MaxPodsPerNode = podNetworkOptions.VcnIpNativePodNetworkOptions.MaxPodsPerNode
			}
			nodeConfigDetails.NodePoolPodNetworkOptionDetails = npnDetails
		} else if podNetworkOptions.CniType == infrastructurev1beta2.FlannelCNI {
			nodeConfigDetails.NodePoolPodNetworkOptionDetails = oke.FlannelOverlayNodePoolPodNetworkOptionDetails{}
		}
	}
	nodePoolDetails := oke.CreateNodePoolDetails{
		CompartmentId:     common.String(m.OCIManagedCluster.Spec.CompartmentId),
		ClusterId:         m.OCIManagedControlPlane.Spec.ID,
		Name:              common.String(m.getNodePoolName()),
		KubernetesVersion: m.OCIManagedMachinePool.Spec.Version,
		NodeShape:         common.String(m.OCIManagedMachinePool.Spec.NodeShape),
		NodeShapeConfig:   &nodeShapeConfig,
		NodeSourceDetails: &sourceDetails,
		FreeformTags:      m.getFreeFormTags(),
		DefinedTags:       m.getDefinedTags(),
		SshPublicKey:      common.String(m.OCIManagedMachinePool.Spec.SshPublicKey),
		NodeConfigDetails: &nodeConfigDetails,
		NodeMetadata:      m.OCIManagedMachinePool.Spec.NodeMetadata,
	}
	if m.OCIManagedMachinePool.Spec.NodeEvictionNodePoolSettings != nil {
		nodePoolDetails.NodeEvictionNodePoolSettings = &oke.NodeEvictionNodePoolSettings{
			EvictionGraceDuration:           m.OCIManagedMachinePool.Spec.NodeEvictionNodePoolSettings.EvictionGraceDuration,
			IsForceDeleteAfterGraceDuration: m.OCIManagedMachinePool.Spec.NodeEvictionNodePoolSettings.IsForceDeleteAfterGraceDuration,
		}
	}
	recycleConfig := m.OCIManagedMachinePool.Spec.NodePoolCyclingDetails
	if recycleConfig != nil {
		nodePoolDetails.NodePoolCyclingDetails = &oke.NodePoolCyclingDetails{
			IsNodeCyclingEnabled: recycleConfig.IsNodeCyclingEnabled,
			MaximumSurge:         recycleConfig.MaximumSurge,
			MaximumUnavailable:   recycleConfig.MaximumUnavailable,
		}
	}
	nodePoolDetails.InitialNodeLabels = m.getInitialNodeKeyValuePairs()

	req := oke.CreateNodePoolRequest{
		CreateNodePoolDetails: nodePoolDetails,
	}
	response, err := m.ContainerEngineClient.CreateNodePool(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create OCIManagedMachinePool")
	}

	if err != nil {
		return nil, err
	}
	wrResponse, err := m.ContainerEngineClient.GetWorkRequest(ctx, oke.GetWorkRequestRequest{
		WorkRequestId: response.OpcWorkRequestId,
	})
	if err != nil {
		return nil, err
	}
	resources := wrResponse.Resources
	var nodePoolId *string
	for _, resource := range resources {
		if *resource.EntityType == "nodepool" {
			nodePoolId = resource.Identifier
		}
	}
	m.Logger.Info("Work request affected resources", "resources", resources)
	if nodePoolId == nil {
		return nil, errors.New(fmt.Sprintf("node pool ws not created with the request, please create a "+
			"support ticket with opc-request-id %s", *wrResponse.OpcRequestId))
	}
	m.OCIManagedMachinePool.Spec.ID = nodePoolId
	m.Info("Created Node Pool", "id", nodePoolId)
	return m.getOKENodePoolFromOCID(ctx, nodePoolId)
}

func (m *ManagedMachinePoolScope) setNodepoolImageId(ctx context.Context) error {
	imageId := m.OCIManagedMachinePool.Spec.NodeSourceViaImage.ImageId
	if imageId != nil && *imageId != "" {
		return nil
	}
	response, err := m.ContainerEngineClient.GetNodePoolOptions(ctx, oke.GetNodePoolOptionsRequest{
		NodePoolOptionId: common.String("all"),
		CompartmentId:    common.String(m.OCIManagedCluster.Spec.CompartmentId),
	})
	if err != nil {
		m.Logger.Error(err, "Could not lookup node pool options")
		return err
	}
	// version in the capoci spec starts with v, wheres as in the node pool options API in OKE
	// the image name does not contain . For example
	//       {
	//        "image-id": "ocid1.image.oc1.iad.aaaaaaaafp7ysdbfl2bg67s4jzbpewxo3k772baixa3vwzecwogl474qecza",
	//        "source-name": "Oracle-Linux-8.6-aarch64-2022.12.15-0-OKE-1.25.4-543",
	//        "source-type": "IMAGE"
	//      },
	//      {
	//        "image-id": "ocid1.image.oc1.iad.aaaaaaaauwwokidwf5nfi34ucbvgnjsni3klnfmu6pz73ctdohb2byiw6ztq",
	//        "source-name": "Oracle-Linux-8.6-2022.12.15-0-OKE-1.23.4-543",
	//        "source-type": "IMAGE"
	//      },
	// we will only default to non gpu image

	// proper validation exists in webhook, this is to be fail-safe
	specVersion := m.OCIManagedMachinePool.Spec.Version
	if specVersion == nil || len(*specVersion) < 1 {
		return errors.New(fmt.Sprintf("invalid/nil kubernetes version is set in OCIManagedMachinePool Spec"))
	}
	k8sVersion := (*specVersion)[1:]
	shape := m.OCIManagedMachinePool.Spec.NodeShape
	isArmShape := strings.Contains(shape, "A1")
	for _, source := range response.Sources {
		image, ok := source.(oke.NodeSourceViaImageOption)
		if ok {
			sourceName := *image.SourceName
			if isValidImage(sourceName) {
				// if source is an arm source and expectation is not an arm image, ignore
				if strings.Contains(sourceName, "aarch64") && !isArmShape {
					continue
				}
				if strings.Contains(sourceName, k8sVersion) {
					m.Info("Image being used", "Name", sourceName, "OCID", *image.ImageId)
					m.OCIManagedMachinePool.Spec.NodeSourceViaImage.ImageId = image.ImageId
					return nil
				}
			}
		}
	}
	sourceJson, err := json.Marshal(response.Sources)
	if err != nil {
		return err
	}
	err = errors.New(fmt.Sprintf("could not lookup nodepool image id from nodepool options"))
	m.Logger.Error(err, "Could not lookup an image corresponding to the kubernetes version from OKE nodepool options",
		"oke-node-pool-image-sources", sourceJson)
	return err
}

func isValidImage(sourceName string) bool {
	// invalidImageSources is the array of names of source images that should be ignored to be considered as defaults
	// for node pool images
	invalidImageSources := []string{"GPU"}
	for _, invalidImageSource := range invalidImageSources {
		if strings.Contains(sourceName, invalidImageSource) {
			return false
		}
	}
	return true
}

func (m *ManagedMachinePoolScope) getOKENodePoolFromOCID(ctx context.Context, nodePoolId *string) (*oke.NodePool, error) {
	req := oke.GetNodePoolRequest{NodePoolId: nodePoolId}

	// Send the request using the service client
	resp, err := m.ContainerEngineClient.GetNodePool(ctx, req)
	if err != nil {
		return nil, err
	}
	return &resp.NodePool, nil
}

// DeleteNodePool terminates a nodepool
func (m *ManagedMachinePoolScope) DeleteNodePool(ctx context.Context, nodePool *oke.NodePool) error {
	m.Info("Terminating node pool", "id", nodePool.Id)
	req := oke.DeleteNodePoolRequest{NodePoolId: nodePool.Id}
	if _, err := m.ContainerEngineClient.DeleteNodePool(ctx, req); err != nil {
		return err
	}

	return nil
}

func (m *ManagedMachinePoolScope) getDefinedTags() map[string]map[string]interface{} {
	tags := m.OCIManagedCluster.Spec.DefinedTags
	if tags == nil {
		return make(map[string]map[string]interface{})
	}
	definedTags := make(map[string]map[string]interface{})
	for ns, mapNs := range tags {
		mapValues := make(map[string]interface{})
		for k, v := range mapNs {
			mapValues[k] = v
		}
		definedTags[ns] = mapValues
	}
	return definedTags
}

func (m *ManagedMachinePoolScope) getFreeFormTags() map[string]string {
	tags := ociutil.BuildClusterTags(m.OCIManagedCluster.Spec.OCIResourceIdentifier)
	if m.OCIManagedCluster.Spec.FreeformTags != nil {
		for k, v := range m.OCIManagedCluster.Spec.FreeformTags {
			tags[k] = v
		}
	}

	return tags
}

func (m *ManagedMachinePoolScope) getWorkerMachineSubnets() []string {
	subnetList := make([]string, 0)
	for _, subnet := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet != nil && subnet.Role == infrastructurev1beta2.WorkerRole {
			subnetList = append(subnetList, subnet.Name)
		}
	}
	return subnetList
}

func (m *ManagedMachinePoolScope) getWorkerMachineNSGs() []string {
	nsgList := make([]string, 0)
	specNsgNames := m.OCIManagedMachinePool.Spec.NodePoolNodeConfig.NsgNames
	if len(specNsgNames) > 0 {
		for _, nsgName := range specNsgNames {
			for _, nsg := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
				if nsg != nil && nsg.ID != nil && nsg.Name == nsgName {
					nsgList = append(nsgList, *nsg.ID)
				}
			}
		}
	} else {
		for _, nsg := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
			if nsg != nil && nsg.ID != nil && nsg.Role == infrastructurev1beta2.WorkerRole {
				nsgList = append(nsgList, *nsg.ID)
			}
		}
	}
	return nsgList
}

func (m *ManagedMachinePoolScope) getWorkerMachineNSGList() []string {
	nsgList := make([]string, 0)
	for _, nsg := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
		if nsg != nil && nsg.Role == infrastructurev1beta2.WorkerRole {
			nsgList = append(nsgList, nsg.Name)
		}
	}
	return nsgList
}

func (m *ManagedMachinePoolScope) getPodSubnets(subnets []string) []string {
	subnetList := make([]string, 0)
	if len(subnets) > 0 {
		for _, subnetName := range subnets {
			for _, subnet := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets {
				if subnet != nil && subnet.ID != nil && subnet.Name == subnetName {
					subnetList = append(subnetList, *subnet.ID)
				}
			}
		}
	}
	return subnetList
}

func (m *ManagedMachinePoolScope) getPodNSGs(nsgs []string) []string {
	nsgList := make([]string, 0)
	if len(nsgs) > 0 {
		for _, nsgName := range nsgs {
			for _, nsg := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
				if nsg != nil && nsg.ID != nil && nsg.Name == nsgName {
					nsgList = append(nsgList, *nsg.ID)
				}
			}
		}
	}
	return nsgList
}

func (m *ManagedMachinePoolScope) buildPlacementConfig(configs []expinfra1.PlacementConfig) ([]oke.NodePoolPlacementConfigDetails, error) {
	placementConfigs := make([]oke.NodePoolPlacementConfigDetails, 0)
	for _, config := range configs {
		subnetId := m.getWorkerMachineSubnet(config.SubnetName)
		if subnetId == nil {
			return nil, errors.New(fmt.Sprintf("worker subnet with name %s is not present in spec",
				*config.SubnetName))
		}
		placementConfigs = append(placementConfigs, oke.NodePoolPlacementConfigDetails{
			AvailabilityDomain:    config.AvailabilityDomain,
			SubnetId:              subnetId,
			FaultDomains:          config.FaultDomains,
			CapacityReservationId: config.CapacityReservationId,
		})
	}
	return placementConfigs, nil
}

func (m *ManagedMachinePoolScope) getInitialNodeKeyValuePairs() []oke.KeyValue {
	keyValues := make([]oke.KeyValue, 0)
	for _, kv := range m.OCIManagedMachinePool.Spec.InitialNodeLabels {
		keyValues = append(keyValues, oke.KeyValue{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}
	return keyValues
}

func (m *ManagedMachinePoolScope) getWorkerMachineSubnet(name *string) *string {
	for _, subnet := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet != nil && subnet.ID != nil && subnet.Name == *name {
			return subnet.ID
		}
	}
	return nil
}

// UpdateNodePool updates a node pool, if needed, based on updated spec
func (m *ManagedMachinePoolScope) UpdateNodePool(ctx context.Context, pool *oke.NodePool) (bool, error) {
	spec := m.OCIManagedMachinePool.Spec.DeepCopy()
	setMachinePoolSpecDefaults(spec)
	nodePoolSizeUpdateRequired := false
	// if replicas is not managed by cluster autoscaler and if the number of nodes in the spec is not equal to number set in the node pool
	// update the node pool
	if !annotations.ReplicasManagedByExternalAutoscaler(m.MachinePool) && (*m.MachinePool.Spec.Replicas != int32(*pool.NodeConfigDetails.Size)) {
		nodePoolSizeUpdateRequired = true
	}
	actual := m.getSpecFromAPIObject(pool)
	if !reflect.DeepEqual(spec, actual) ||
		m.getNodePoolName() != *pool.Name || nodePoolSizeUpdateRequired {
		m.Logger.Info("Updating node pool")
		// printing json specs will help debug problems when there are spurious/unwanted updates
		jsonSpec, err := json.Marshal(*spec)
		if err != nil {
			return false, err
		}
		jsonActual, err := json.Marshal(*actual)
		if err != nil {
			return false, err
		}
		m.Logger.Info("Node pool", "spec", jsonSpec, "actual", jsonActual)

		nodeConfigDetails := oke.UpdateNodePoolNodeConfigDetails{
			NsgIds:                         m.getWorkerMachineNSGs(),
			IsPvEncryptionInTransitEnabled: spec.NodePoolNodeConfig.IsPvEncryptionInTransitEnabled,
			KmsKeyId:                       spec.NodePoolNodeConfig.KmsKeyId,
		}
		// send placement config only if there is an actual change in placement
		// placement config and recycle config cannot be sent at the same time, and most use cases will
		// be to update kubernetes version in which case, placement config is not required to be sent
		if !reflect.DeepEqual(spec.NodePoolNodeConfig.PlacementConfigs, actual.NodePoolNodeConfig.PlacementConfigs) {
			placementConfig, err := m.buildPlacementConfig(spec.NodePoolNodeConfig.PlacementConfigs)
			if err != nil {
				return false, err
			}
			nodeConfigDetails.PlacementConfigs = placementConfig
		}
		if nodePoolSizeUpdateRequired {
			nodeConfigDetails.Size = common.Int(int(*m.MachinePool.Spec.Replicas))
		}
		nodeShapeConfig := oke.UpdateNodeShapeConfigDetails{}
		if spec.NodeShapeConfig != nil {
			ocpuString := spec.NodeShapeConfig.Ocpus
			if ocpuString != nil {
				ocpus, err := strconv.ParseFloat(*ocpuString, 32)
				if err != nil {
					return false, errors.New(fmt.Sprintf("ocpus provided %s is not a valid floating point",
						*ocpuString))
				}
				nodeShapeConfig.Ocpus = common.Float32(float32(ocpus))
			}

			memoryInGBsString := spec.NodeShapeConfig.MemoryInGBs
			if memoryInGBsString != nil {
				memoryInGBs, err := strconv.ParseFloat(*memoryInGBsString, 32)
				if err != nil {
					return false, errors.New(fmt.Sprintf("memoryInGBs provided %s is not a valid floating point",
						*memoryInGBsString))
				}
				nodeShapeConfig.MemoryInGBs = common.Float32(float32(memoryInGBs))
			}
		}
		err = m.setNodepoolImageId(ctx)
		if err != nil {
			return false, err
		}
		sourceDetails := oke.NodeSourceViaImageDetails{
			// use image id from machinepool spec itself as the copy will not have the image set in the
			// setNodepoolImageId method above
			ImageId:             m.OCIManagedMachinePool.Spec.NodeSourceViaImage.ImageId,
			BootVolumeSizeInGBs: spec.NodeSourceViaImage.BootVolumeSizeInGBs,
		}

		podNetworkOptions := spec.NodePoolNodeConfig.NodePoolPodNetworkOptionDetails
		if podNetworkOptions != nil {
			if podNetworkOptions.CniType == infrastructurev1beta2.VCNNativeCNI {
				npnDetails := oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
					PodSubnetIds: m.getPodSubnets(podNetworkOptions.VcnIpNativePodNetworkOptions.SubnetNames),
					PodNsgIds:    m.getPodNSGs(podNetworkOptions.VcnIpNativePodNetworkOptions.NSGNames),
				}
				if podNetworkOptions.VcnIpNativePodNetworkOptions.MaxPodsPerNode != nil {
					npnDetails.MaxPodsPerNode = podNetworkOptions.VcnIpNativePodNetworkOptions.MaxPodsPerNode
				}
				nodeConfigDetails.NodePoolPodNetworkOptionDetails = npnDetails
			} else if podNetworkOptions.CniType == infrastructurev1beta2.FlannelCNI {
				nodeConfigDetails.NodePoolPodNetworkOptionDetails = oke.FlannelOverlayNodePoolPodNetworkOptionDetails{}
			}
		}
		nodePoolDetails := oke.UpdateNodePoolDetails{
			Name:              common.String(m.getNodePoolName()),
			KubernetesVersion: m.OCIManagedMachinePool.Spec.Version,
			NodeShape:         common.String(m.OCIManagedMachinePool.Spec.NodeShape),
			NodeShapeConfig:   &nodeShapeConfig,
			NodeSourceDetails: &sourceDetails,
			SshPublicKey:      common.String(m.OCIManagedMachinePool.Spec.SshPublicKey),
			NodeConfigDetails: &nodeConfigDetails,
			NodeMetadata:      spec.NodeMetadata,
		}
		recycleConfig := spec.NodePoolCyclingDetails
		// cannot send recycle config and placement config together
		if recycleConfig != nil && len(nodeConfigDetails.PlacementConfigs) == 0 {
			nodePoolDetails.NodePoolCyclingDetails = &oke.NodePoolCyclingDetails{
				IsNodeCyclingEnabled: recycleConfig.IsNodeCyclingEnabled,
				MaximumSurge:         recycleConfig.MaximumSurge,
				MaximumUnavailable:   recycleConfig.MaximumUnavailable,
			}
		}
		if recycleConfig != nil && len(nodeConfigDetails.PlacementConfigs) != 0 {
			m.Logger.V(LogLevelWarn).Info("Placement configuration has been changed in the update, " +
				"hence node pool recycling configuration will not be sent with the update request")
		}
		if spec.NodeEvictionNodePoolSettings != nil {
			nodePoolDetails.NodeEvictionNodePoolSettings = &oke.NodeEvictionNodePoolSettings{
				EvictionGraceDuration:           spec.NodeEvictionNodePoolSettings.EvictionGraceDuration,
				IsForceDeleteAfterGraceDuration: spec.NodeEvictionNodePoolSettings.IsForceDeleteAfterGraceDuration,
			}
		}
		nodePoolDetails.InitialNodeLabels = m.getInitialNodeKeyValuePairs()
		req := oke.UpdateNodePoolRequest{
			NodePoolId:            pool.Id,
			UpdateNodePoolDetails: nodePoolDetails,
		}
		_, err = m.ContainerEngineClient.UpdateNodePool(ctx, req)
		if err != nil {
			return false, errors.Wrapf(err, "failed to update Node Pool")
		}

		m.Info("Updated node pool")
		return true, nil
	} else {
		m.Info("No reconciliation needed for node pool")
	}
	return false, nil
}

// setMachinePoolSpecDefaults sets the defaults in the spec as returned by OKE API. We need to set defaults here rather than webhook as
// there is a chance user will edit the cluster
func setMachinePoolSpecDefaults(spec *infrav2exp.OCIManagedMachinePoolSpec) {
	spec.ProviderIDList = nil
	spec.ProviderID = nil

	if spec.NodePoolNodeConfig != nil {
		if spec.NodePoolNodeConfig.PlacementConfigs != nil {
			configs := spec.NodePoolNodeConfig.PlacementConfigs
			sort.Slice(configs, func(i, j int) bool {
				return *configs[i].AvailabilityDomain < *configs[j].AvailabilityDomain
			})
		}
	}
	podNetworkOptions := spec.NodePoolNodeConfig.NodePoolPodNetworkOptionDetails
	if podNetworkOptions != nil {
		if podNetworkOptions.CniType == infrastructurev1beta2.VCNNativeCNI {
			// 31 is the default max pods per node returned by OKE API
			spec.NodePoolNodeConfig.NodePoolPodNetworkOptionDetails.VcnIpNativePodNetworkOptions.MaxPodsPerNode = common.Int(31)
		}
	}
}

func (m *ManagedMachinePoolScope) getSpecFromAPIObject(pool *oke.NodePool) *expinfra1.OCIManagedMachinePoolSpec {
	nodePoolNodeConfig := expinfra1.NodePoolNodeConfig{}
	actualNodeConfigDetails := pool.NodeConfigDetails
	if actualNodeConfigDetails != nil {
		nodePoolNodeConfig.IsPvEncryptionInTransitEnabled = actualNodeConfigDetails.IsPvEncryptionInTransitEnabled
		nodePoolNodeConfig.KmsKeyId = actualNodeConfigDetails.KmsKeyId
		nodePoolNodeConfig.NsgNames = GetNsgNamesFromId(actualNodeConfigDetails.NsgIds, m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List)
		configs := m.buildPlacementConfigFromActual(actualNodeConfigDetails.PlacementConfigs)
		sort.Slice(configs, func(i, j int) bool {
			return *configs[i].AvailabilityDomain < *configs[j].AvailabilityDomain
		})
		nodePoolNodeConfig.PlacementConfigs = configs
		podDetails, ok := actualNodeConfigDetails.NodePoolPodNetworkOptionDetails.(oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails)
		if ok {
			nodePoolNodeConfig.NodePoolPodNetworkOptionDetails = &expinfra1.NodePoolPodNetworkOptionDetails{
				CniType: infrastructurev1beta2.VCNNativeCNI,
				VcnIpNativePodNetworkOptions: expinfra1.VcnIpNativePodNetworkOptions{
					MaxPodsPerNode: podDetails.MaxPodsPerNode,
					NSGNames:       GetNsgNamesFromId(podDetails.PodNsgIds, m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List),
					SubnetNames:    GetSubnetNamesFromId(podDetails.PodSubnetIds, m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets),
				},
			}
		} else {
			nodePoolNodeConfig.NodePoolPodNetworkOptionDetails = &expinfra1.NodePoolPodNetworkOptionDetails{
				CniType: infrastructurev1beta2.FlannelCNI,
			}
		}
	}
	spec := expinfra1.OCIManagedMachinePoolSpec{
		ID:                 pool.Id,
		NodePoolNodeConfig: &nodePoolNodeConfig,
		InitialNodeLabels:  getInitialNodeLabels(pool.InitialNodeLabels),
		NodeShape:          *pool.NodeShape,
		NodeMetadata:       pool.NodeMetadata,
		Version:            pool.KubernetesVersion,
	}
	if pool.NodeEvictionNodePoolSettings != nil {
		spec.NodeEvictionNodePoolSettings = &expinfra1.NodeEvictionNodePoolSettings{
			EvictionGraceDuration:           pool.NodeEvictionNodePoolSettings.EvictionGraceDuration,
			IsForceDeleteAfterGraceDuration: pool.NodeEvictionNodePoolSettings.IsForceDeleteAfterGraceDuration,
		}
	}
	sourceDetails, ok := pool.NodeSourceDetails.(oke.NodeSourceViaImageDetails)
	if ok {
		spec.NodeSourceViaImage = &expinfra1.NodeSourceViaImage{
			ImageId:             sourceDetails.ImageId,
			BootVolumeSizeInGBs: sourceDetails.BootVolumeSizeInGBs,
		}
	}
	if pool.SshPublicKey != nil {
		spec.SshPublicKey = *pool.SshPublicKey
	}
	if pool.NodeShapeConfig != nil {
		nodeShapeConfig := expinfra1.NodeShapeConfig{}
		if pool.NodeShapeConfig.MemoryInGBs != nil {
			mem := strconv.FormatFloat(float64(*pool.NodeShapeConfig.MemoryInGBs), 'f', -1, 32)
			nodeShapeConfig.MemoryInGBs = &mem
		}
		if pool.NodeShapeConfig.Ocpus != nil {
			ocpu := strconv.FormatFloat(float64(*pool.NodeShapeConfig.Ocpus), 'f', -1, 32)
			nodeShapeConfig.Ocpus = &ocpu
		}
		spec.NodeShapeConfig = &nodeShapeConfig
	}
	if pool.NodePoolCyclingDetails != nil {
		cyclingDetails := pool.NodePoolCyclingDetails
		spec.NodePoolCyclingDetails = &expinfra1.NodePoolCyclingDetails{
			IsNodeCyclingEnabled: cyclingDetails.IsNodeCyclingEnabled,
			MaximumSurge:         cyclingDetails.MaximumSurge,
			MaximumUnavailable:   cyclingDetails.MaximumUnavailable,
		}
	}
	return &spec
}

func getInitialNodeLabels(labels []oke.KeyValue) []expinfra1.KeyValue {
	if len(labels) == 0 {
		return nil
	}
	kv := make([]expinfra1.KeyValue, 0)
	for _, l := range labels {
		kv = append(kv, expinfra1.KeyValue{
			Key:   l.Key,
			Value: l.Value,
		})
	}
	return kv
}

func (m *ManagedMachinePoolScope) buildPlacementConfigFromActual(actualConfigs []oke.NodePoolPlacementConfigDetails) []expinfra1.PlacementConfig {
	configs := make([]expinfra1.PlacementConfig, 0)
	for _, config := range actualConfigs {
		configs = append(configs, expinfra1.PlacementConfig{
			AvailabilityDomain:    config.AvailabilityDomain,
			FaultDomains:          config.FaultDomains,
			CapacityReservationId: config.CapacityReservationId,
			SubnetName:            common.String(GetSubnetNameFromId(config.SubnetId, m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets)),
		})
	}
	return configs
}
