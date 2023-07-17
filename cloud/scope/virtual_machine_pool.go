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
	"reflect"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OCIVirtualMachinePoolKind = "OCIVirtualMachinePool"
)

// VirtualMachinePoolScopeParams defines the params need to create a new VirtualMachinePoolScope
type VirtualMachinePoolScopeParams struct {
	Logger                  *logr.Logger
	Cluster                 *clusterv1.Cluster
	MachinePool             *expclusterv1.MachinePool
	Client                  client.Client
	ComputeManagementClient computemanagement.Client
	OCIManagedCluster       *infrastructurev1beta2.OCIManagedCluster
	OCIManagedControlPlane  *infrastructurev1beta2.OCIManagedControlPlane
	OCIVirtualMachinePool   *expinfra1.OCIVirtualMachinePool
	ContainerEngineClient   containerengine.Client
}

type VirtualMachinePoolScope struct {
	*logr.Logger
	Client                  client.Client
	patchHelper             *patch.Helper
	Cluster                 *clusterv1.Cluster
	MachinePool             *expclusterv1.MachinePool
	ComputeManagementClient computemanagement.Client
	OCIManagedCluster       *infrastructurev1beta2.OCIManagedCluster
	OCIVirtualMachinePool   *expinfra1.OCIVirtualMachinePool
	ContainerEngineClient   containerengine.Client
	OCIManagedControlPlane  *infrastructurev1beta2.OCIManagedControlPlane
}

// NewVirtualMachinePoolScope creates a VirtualMachinePoolScope given the VirtualMachinePoolScopeParams
func NewVirtualMachinePoolScope(params VirtualMachinePoolScopeParams) (*VirtualMachinePoolScope, error) {
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
	helper, err := patch.NewHelper(params.OCIVirtualMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	params.OCIVirtualMachinePool.Status.InfrastructureMachineKind = "OCIMachinePoolMachine"
	return &VirtualMachinePoolScope{
		Logger:                  params.Logger,
		Client:                  params.Client,
		ComputeManagementClient: params.ComputeManagementClient,
		Cluster:                 params.Cluster,
		OCIManagedCluster:       params.OCIManagedCluster,
		patchHelper:             helper,
		MachinePool:             params.MachinePool,
		OCIVirtualMachinePool:   params.OCIVirtualMachinePool,
		ContainerEngineClient:   params.ContainerEngineClient,
		OCIManagedControlPlane:  params.OCIManagedControlPlane,
	}, nil
}

// PatchObject persists the cluster configuration and status.
func (m *VirtualMachinePoolScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.OCIVirtualMachinePool)
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *VirtualMachinePoolScope) Close(ctx context.Context) error {
	return m.PatchObject(ctx)
}

// SetFailureReason sets the OCIMachine status error reason.
func (m *VirtualMachinePoolScope) SetFailureReason(v capierrors.MachinePoolStatusFailure) {
	m.OCIVirtualMachinePool.Status.FailureReason = &v
}

func (m *VirtualMachinePoolScope) SetReplicaCount(count int32) {
	m.OCIVirtualMachinePool.Status.Replicas = count
}

// GetWorkerMachineSubnet returns the WorkerRole core.Subnet id for the cluster
func (m *VirtualMachinePoolScope) GetWorkerMachineSubnet() *string {
	for _, subnet := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == infrastructurev1beta2.WorkerRole {
			return subnet.ID
		}
	}
	return nil
}

// ListandSetMachinePoolInstances retrieves a virtual node pools instances and sets them in the ProviderIDList
func (m *VirtualMachinePoolScope) ListandSetMachinePoolInstances(ctx context.Context, nodePool *oke.VirtualNodePool) ([]infrav2exp.OCIMachinePoolMachine, error) {
	machines := make([]infrav2exp.OCIMachinePoolMachine, 0)
	var page *string
	for {
		reqList := oke.ListVirtualNodesRequest{
			VirtualNodePoolId: nodePool.Id,
			Page:              page,
		}

		response, err := m.ContainerEngineClient.ListVirtualNodes(ctx, reqList)
		if err != nil {
			return machines, err
		}
		for _, node := range response.Items {
			// deleted nodes should not be considered
			if node.LifecycleState == oke.VirtualNodeLifecycleStateDeleted ||
				node.LifecycleState == oke.VirtualNodeLifecycleStateFailed {
				continue
			}
			ready := false
			if node.LifecycleState == oke.VirtualNodeLifecycleStateActive {
				ready = true
			}
			machines = append(machines, infrav2exp.OCIMachinePoolMachine{
				Spec: infrav2exp.OCIMachinePoolMachineSpec{
					OCID:         node.Id,
					ProviderID:   node.Id,
					InstanceName: node.DisplayName,
				},
				Status: infrav2exp.OCIMachinePoolMachineStatus{
					Ready: ready,
				},
			})
		}
		if response.OpcNextPage == nil {
			break
		} else {
			page = response.OpcNextPage
		}
	}
	return machines, nil
}

// IsResourceCreatedByClusterAPI determines if the virtual node pool was created by the cluster using the
// tags created at virtual node pool launch.
func (m *VirtualMachinePoolScope) IsResourceCreatedByClusterAPI(resourceFreeFormTags map[string]string) bool {
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(m.OCIManagedCluster.Spec.OCIResourceIdentifier)
	for k, v := range tagsAddedByClusterAPI {
		if resourceFreeFormTags[k] != v {
			return false
		}
	}
	return true
}

// FindVirtualNodePool attempts to find the node pool by id if the id exists or by name. It checks to make sure
// the node pool was created by the cluster before returning the correct pool
func (m *VirtualMachinePoolScope) FindVirtualNodePool(ctx context.Context) (*oke.VirtualNodePool, error) {
	if m.OCIVirtualMachinePool.Spec.ID != nil {
		response, err := m.ContainerEngineClient.GetVirtualNodePool(ctx, oke.GetVirtualNodePoolRequest{
			VirtualNodePoolId: m.OCIVirtualMachinePool.Spec.ID,
		})
		if err != nil {
			return nil, err
		}
		if m.IsResourceCreatedByClusterAPI(response.FreeformTags) {
			return &response.VirtualNodePool, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}

	var page *string
	for {
		reqList := oke.ListVirtualNodePoolsRequest{
			CompartmentId: common.String(m.OCIManagedCluster.Spec.CompartmentId),
			ClusterId:     m.OCIManagedControlPlane.Spec.ID,
			Name:          common.String(m.getNodePoolName()),
			Page:          page,
		}

		response, err := m.ContainerEngineClient.ListVirtualNodePools(ctx, reqList)
		if err != nil {
			return nil, err
		}
		for _, i := range response.Items {
			if m.IsResourceCreatedByClusterAPI(i.FreeformTags) {
				return m.getOKEVirtualNodePoolFromOCID(ctx, i.Id)
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

func (m *VirtualMachinePoolScope) getNodePoolName() string {
	return m.OCIVirtualMachinePool.GetName()
}

// CreateVirtualNodePool attempts to create a virtual node pool
func (m *VirtualMachinePoolScope) CreateVirtualNodePool(ctx context.Context) (*oke.VirtualNodePool, error) {
	m.Info("Creating Node Pool")

	placementConfigs := m.OCIVirtualMachinePool.Spec.PlacementConfigs
	if len(placementConfigs) == 0 {
		placementConfigs = make([]expinfra1.VirtualNodepoolPlacementConfig, 0)
		workerSubnets := m.getWorkerMachineSubnets()
		if len(workerSubnets) == 0 {
			return nil, errors.New("worker subnets are not specified")
		}
		adMap := m.OCIManagedCluster.Spec.AvailabilityDomains
		for k, v := range adMap {
			placementConfigs = append(placementConfigs, expinfra1.VirtualNodepoolPlacementConfig{
				AvailabilityDomain: common.String(k),
				FaultDomains:       v.FaultDomains,
				SubnetName:         common.String(workerSubnets[0]),
			})
		}
		m.OCIVirtualMachinePool.Spec.PlacementConfigs = placementConfigs
	}
	if len(m.OCIVirtualMachinePool.Spec.NsgNames) == 0 {
		m.OCIVirtualMachinePool.Spec.NsgNames = m.getWorkerMachineNSGList()
	}
	placementConfig, err := m.buildPlacementConfig(placementConfigs)
	if err != nil {
		return nil, err
	}
	podConfiguration := m.getPodConfiguration()

	nodePoolDetails := oke.CreateVirtualNodePoolDetails{
		CompartmentId:            common.String(m.OCIManagedCluster.Spec.CompartmentId),
		Size:                     common.Int(int(*m.MachinePool.Spec.Replicas)),
		ClusterId:                m.OCIManagedControlPlane.Spec.ID,
		DisplayName:              common.String(m.getNodePoolName()),
		PlacementConfigurations:  placementConfig,
		FreeformTags:             m.getFreeFormTags(),
		DefinedTags:              m.getDefinedTags(),
		NsgIds:                   m.getWorkerMachineNSGs(),
		InitialVirtualNodeLabels: m.getInitialNodeKeyValuePairs(),
		PodConfiguration:         podConfiguration,
		Taints:                   m.getTaints(),
		VirtualNodeTags: &oke.VirtualNodeTags{
			FreeformTags: m.getFreeFormTags(),
			DefinedTags:  m.getDefinedTags(),
		},
	}
	nodePoolDetails.InitialVirtualNodeLabels = m.getInitialNodeKeyValuePairs()

	req := oke.CreateVirtualNodePoolRequest{
		CreateVirtualNodePoolDetails: nodePoolDetails,
	}
	response, err := m.ContainerEngineClient.CreateVirtualNodePool(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create OCIVirtualMachinePool")
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
		if *resource.EntityType == "VirtualNodePool" {
			nodePoolId = resource.Identifier
		}
	}
	m.Logger.Info("Work request affected resources", "resources", resources)
	if nodePoolId == nil {
		return nil, errors.New(fmt.Sprintf("node pool ws not created with the request, please create a "+
			"support ticket with opc-request-id %s", *wrResponse.OpcRequestId))
	}
	m.OCIVirtualMachinePool.Spec.ID = nodePoolId
	m.Info("Created Node Pool", "id", nodePoolId)
	return m.getOKEVirtualNodePoolFromOCID(ctx, nodePoolId)
}

func (m *VirtualMachinePoolScope) getOKEVirtualNodePoolFromOCID(ctx context.Context, nodePoolId *string) (*oke.VirtualNodePool, error) {
	req := oke.GetVirtualNodePoolRequest{VirtualNodePoolId: nodePoolId}

	// Send the request using the service client
	resp, err := m.ContainerEngineClient.GetVirtualNodePool(ctx, req)
	if err != nil {
		return nil, err
	}
	return &resp.VirtualNodePool, nil
}

// DeleteVirtualNodePool terminates a virtual nodepool
func (m *VirtualMachinePoolScope) DeleteVirtualNodePool(ctx context.Context, nodePool *oke.VirtualNodePool) error {
	m.Info("Terminating node pool", "id", nodePool.Id)
	req := oke.DeleteVirtualNodePoolRequest{VirtualNodePoolId: nodePool.Id}
	if _, err := m.ContainerEngineClient.DeleteVirtualNodePool(ctx, req); err != nil {
		return err
	}

	return nil
}

func (m *VirtualMachinePoolScope) getDefinedTags() map[string]map[string]interface{} {
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

func (m *VirtualMachinePoolScope) getFreeFormTags() map[string]string {
	tags := ociutil.BuildClusterTags(m.OCIManagedCluster.Spec.OCIResourceIdentifier)
	if m.OCIManagedCluster.Spec.FreeformTags != nil {
		for k, v := range m.OCIManagedCluster.Spec.FreeformTags {
			tags[k] = v
		}
	}

	return tags
}

func (m *VirtualMachinePoolScope) getWorkerMachineSubnets() []string {
	subnetList := make([]string, 0)
	for _, subnet := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == infrastructurev1beta2.WorkerRole {
			subnetList = append(subnetList, subnet.Name)
		}
	}
	return subnetList
}

func (m *VirtualMachinePoolScope) getWorkerMachineNSGs() []string {
	nsgList := make([]string, 0)
	specNsgNames := m.OCIVirtualMachinePool.Spec.NsgNames
	if len(specNsgNames) > 0 {
		for _, nsgName := range specNsgNames {
			for _, nsg := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
				if nsg.Name == nsgName {
					nsgList = append(nsgList, *nsg.ID)
				}
			}
		}
	} else {
		for _, nsg := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
			if nsg.Role == infrastructurev1beta2.WorkerRole {
				nsgList = append(nsgList, *nsg.ID)
			}
		}
	}
	return nsgList
}

func (m *VirtualMachinePoolScope) getWorkerMachineNSGList() []string {
	nsgList := make([]string, 0)
	for _, nsg := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
		if nsg.Role == infrastructurev1beta2.WorkerRole {
			nsgList = append(nsgList, nsg.Name)
		}
	}
	return nsgList
}

func (m *VirtualMachinePoolScope) getPodNSGs(nsgs []string) []string {
	nsgList := make([]string, 0)
	if len(nsgs) > 0 {
		for _, nsgName := range nsgs {
			for _, nsg := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
				if nsg.Name == nsgName {
					nsgList = append(nsgList, *nsg.ID)
				}
			}
		}
	}
	return nsgList
}

func (m *VirtualMachinePoolScope) buildPlacementConfig(configs []expinfra1.VirtualNodepoolPlacementConfig) ([]oke.PlacementConfiguration, error) {
	placementConfigs := make([]oke.PlacementConfiguration, 0)
	for _, config := range configs {
		subnetId := m.getSubnet(config.SubnetName)
		if subnetId == nil {
			return nil, errors.New(fmt.Sprintf("worker subnet with name %s is not present in spec",
				*config.SubnetName))
		}
		placementConfigs = append(placementConfigs, oke.PlacementConfiguration{
			AvailabilityDomain: config.AvailabilityDomain,
			SubnetId:           subnetId,
			FaultDomain:        config.FaultDomains,
		})
	}
	return placementConfigs, nil
}

func (m *VirtualMachinePoolScope) getInitialNodeKeyValuePairs() []oke.InitialVirtualNodeLabel {
	keyValues := make([]oke.InitialVirtualNodeLabel, 0)
	for _, kv := range m.OCIVirtualMachinePool.Spec.InitialVirtualNodeLabels {
		keyValues = append(keyValues, oke.InitialVirtualNodeLabel{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}
	return keyValues
}

func (m *VirtualMachinePoolScope) getTaints() []oke.Taint {
	taints := make([]oke.Taint, 0)
	for _, taint := range m.OCIVirtualMachinePool.Spec.Taints {
		taints = append(taints, oke.Taint{
			Key:    taint.Key,
			Value:  taint.Value,
			Effect: taint.Effect,
		})
	}
	return taints
}

func (m *VirtualMachinePoolScope) getSubnet(name *string) *string {
	for _, subnet := range m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Name == *name {
			return subnet.ID
		}
	}
	return nil
}

// UpdateVirtualNodePool updates a virtual node pool, if needed, based on updated spec
func (m *VirtualMachinePoolScope) UpdateVirtualNodePool(ctx context.Context, pool *oke.VirtualNodePool) (bool, error) {
	spec := m.OCIVirtualMachinePool.Spec.DeepCopy()
	setVirtualMachinePoolSpecDefaults(spec)
	nodePoolSizeUpdateRequired := false
	// if replicas is not managed by cluster autoscaler and if the number of nodes in the spec is not equal to number set in the node pool
	// update the node pool
	if !annotations.ReplicasManagedByExternalAutoscaler(m.MachinePool) && (*m.MachinePool.Spec.Replicas != int32(*pool.Size)) {
		nodePoolSizeUpdateRequired = true
	}
	actual := m.getSpecFromAPIObject(pool)
	if !reflect.DeepEqual(spec, actual) ||
		m.getNodePoolName() != *pool.DisplayName || nodePoolSizeUpdateRequired {
		m.Logger.Info("Updating virtual node pool")
		// printing json specs will help debug problems when there are spurious/unwanted updates
		jsonSpec, err := json.Marshal(*spec)
		if err != nil {
			return false, err
		}
		jsonActual, err := json.Marshal(*actual)
		if err != nil {
			return false, err
		}
		m.Logger.Info("Virtual Node pool", "spec", jsonSpec, "actual", jsonActual)
		if len(m.OCIVirtualMachinePool.Spec.NsgNames) == 0 {
			m.OCIVirtualMachinePool.Spec.NsgNames = m.getWorkerMachineNSGList()
		}
		placementConfigs := m.OCIVirtualMachinePool.Spec.PlacementConfigs
		placementConfig, err := m.buildPlacementConfig(placementConfigs)
		if err != nil {
			return false, err
		}
		podConfiguration := m.getPodConfiguration()

		nodePoolDetails := oke.UpdateVirtualNodePoolDetails{
			Size:                     common.Int(int(*m.MachinePool.Spec.Replicas)),
			DisplayName:              common.String(m.getNodePoolName()),
			PlacementConfigurations:  placementConfig,
			NsgIds:                   m.getWorkerMachineNSGs(),
			InitialVirtualNodeLabels: m.getInitialNodeKeyValuePairs(),
			PodConfiguration:         podConfiguration,
			Taints:                   m.getTaints(),
		}

		nodePoolDetails.InitialVirtualNodeLabels = m.getInitialNodeKeyValuePairs()

		req := oke.UpdateVirtualNodePoolRequest{
			VirtualNodePoolId:            pool.Id,
			UpdateVirtualNodePoolDetails: nodePoolDetails,
		}
		_, err = m.ContainerEngineClient.UpdateVirtualNodePool(ctx, req)
		if err != nil {
			return false, errors.Wrapf(err, "failed to update Virtual Node Pool")
		}

		m.Info("Updated virtual node pool")
		return true, nil
	} else {
		m.Info("No reconciliation needed for virtual node pool")
	}
	return false, nil
}

// setVirtualMachinePoolSpecDefaults sets the defaults in the spec as returned by OKE API. We need to set defaults here rather than webhook as
// there is a chance user will edit the cluster
func setVirtualMachinePoolSpecDefaults(spec *infrav2exp.OCIVirtualMachinePoolSpec) {
	spec.ProviderIDList = nil
	spec.ProviderID = nil
}

func (m *VirtualMachinePoolScope) getSpecFromAPIObject(pool *oke.VirtualNodePool) *expinfra1.OCIVirtualMachinePoolSpec {
	spec := expinfra1.OCIVirtualMachinePoolSpec{
		ID: pool.Id,
	}
	if pool.PodConfiguration != nil {
		podConfiguration := expinfra1.PodConfig{
			Shape: pool.PodConfiguration.Shape,
		}
		podConfiguration.NsgNames = GetNsgNamesFromId(pool.PodConfiguration.NsgIds, m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List)
		podConfiguration.SubnetName = common.String(GetSubnetNameFromId(pool.PodConfiguration.SubnetId, m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets))
		spec.PodConfiguration = podConfiguration
	}
	spec.PlacementConfigs = m.buildPlacementConfigFromActual(pool.PlacementConfigurations)
	spec.NsgNames = GetNsgNamesFromId(pool.NsgIds, m.OCIManagedCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List)
	spec.InitialVirtualNodeLabels = getInitialVirtualNodeLabels(pool.InitialVirtualNodeLabels)
	spec.Taints = getTaints(pool.Taints)
	return &spec
}

func getInitialVirtualNodeLabels(labels []oke.InitialVirtualNodeLabel) []expinfra1.KeyValue {
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

func getTaints(taints []oke.Taint) []expinfra1.Taint {
	if len(taints) == 0 {
		return nil
	}
	kv := make([]expinfra1.Taint, 0)
	for _, l := range taints {
		kv = append(kv, expinfra1.Taint{
			Key:    l.Key,
			Value:  l.Value,
			Effect: l.Effect,
		})
	}
	return kv
}

func (m *VirtualMachinePoolScope) buildPlacementConfigFromActual(actualConfigs []oke.PlacementConfiguration) []expinfra1.VirtualNodepoolPlacementConfig {
	configs := make([]expinfra1.VirtualNodepoolPlacementConfig, 0)
	for _, config := range actualConfigs {
		configs = append(configs, expinfra1.VirtualNodepoolPlacementConfig{
			AvailabilityDomain: config.AvailabilityDomain,
			FaultDomains:       config.FaultDomain,
			SubnetName:         common.String(GetSubnetNameFromId(config.SubnetId, m.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets)),
		})
	}
	return configs
}

func (m *VirtualMachinePoolScope) getPodConfiguration() *oke.PodConfiguration {
	subnetId := m.getSubnet(m.OCIVirtualMachinePool.Spec.PodConfiguration.SubnetName)
	podConfig := m.OCIVirtualMachinePool.Spec.PodConfiguration
	return &oke.PodConfiguration{
		SubnetId: subnetId,
		Shape:    podConfig.Shape,
		NsgIds:   m.getPodNSGs(podConfig.NsgNames),
	}
}
