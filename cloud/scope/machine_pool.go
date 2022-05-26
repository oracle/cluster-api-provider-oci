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
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/computemanagement"
	expinfra1 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	infrav1exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const OCIMachinePoolKind = "OCIMachinePool"

// MachineScopeParams defines the params need to create a new MachineScope
type MachinePoolScopeParams struct {
	Logger                  *logr.Logger
	Cluster                 *clusterv1.Cluster
	MachinePool             *expclusterv1.MachinePool
	Client                  client.Client
	ComputeManagementClient computemanagement.Client
	OCICluster              *infrastructurev1beta1.OCICluster
	OCIMachinePool          *expinfra1.OCIMachinePool
	OCIMachine              *infrastructurev1beta1.OCIMachine
}

type MachinePoolScope struct {
	*logr.Logger
	Client                  client.Client
	patchHelper             *patch.Helper
	Cluster                 *clusterv1.Cluster
	MachinePool             *expclusterv1.MachinePool
	ComputeManagementClient computemanagement.Client
	OCICluster              *infrastructurev1beta1.OCICluster
	OCIMachinePool          *expinfra1.OCIMachinePool
	OCIMachine              *infrastructurev1beta1.OCIMachine
}

// NewMachinePoolScope creates a MachinePoolScope given the MachinePoolScopeParams
func NewMachinePoolScope(params MachinePoolScopeParams) (*MachinePoolScope, error) {
	if params.MachinePool == nil {
		return nil, errors.New("failed to generate new scope from nil MachinePool")
	}
	if params.OCICluster == nil {
		return nil, errors.New("failed to generate new scope from nil OCICluster")
	}

	if params.Logger == nil {
		log := klogr.New()
		params.Logger = &log
	}
	helper, err := patch.NewHelper(params.OCIMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &MachinePoolScope{
		Logger:                  params.Logger,
		Client:                  params.Client,
		ComputeManagementClient: params.ComputeManagementClient,
		Cluster:                 params.Cluster,
		OCICluster:              params.OCICluster,
		patchHelper:             helper,
		MachinePool:             params.MachinePool,
		OCIMachinePool:          params.OCIMachinePool,
	}, nil
}

// PatchObject persists the cluster configuration and status.
func (m *MachinePoolScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.OCIMachinePool)
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *MachinePoolScope) Close(ctx context.Context) error {
	return m.PatchObject(ctx)
}

// HasFailed returns true when the OCIMachinePool's Failure reason or Failure message is populated.
func (m *MachinePoolScope) HasFailed() bool {
	return m.OCIMachinePool.Status.FailureReason != nil || m.OCIMachinePool.Status.FailureMessage != nil
}

// GetInstanceConfigurationId returns the MachinePoolScope instance configuration id.
func (m *MachinePoolScope) GetInstanceConfigurationId() string {
	return m.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId
}

// SetInstanceConfigurationIdStatus sets the MachinePool InstanceConfigurationId status.
func (m *MachinePoolScope) SetInstanceConfigurationIdStatus(id string) {
	m.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = id
}

// SetFailureMessage sets the OCIMachine status error message.
func (m *MachinePoolScope) SetFailureMessage(v error) {
	m.OCIMachinePool.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// SetFailureReason sets the OCIMachine status error reason.
func (m *MachinePoolScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.OCIMachinePool.Status.FailureReason = &v
}

// SetReady sets the OCIMachine Ready Status.
func (m *MachinePoolScope) SetReady() {
	m.OCIMachinePool.Status.Ready = true
}

func (m *MachinePoolScope) SetReplicaCount(count int32) {
	m.OCIMachinePool.Status.Replicas = count
}

// GetWorkerMachineSubnet returns the WorkerRole core.Subnet id for the cluster
func (m *MachinePoolScope) GetWorkerMachineSubnet() *string {
	for _, subnet := range m.OCICluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == infrastructurev1beta1.WorkerRole {
			return subnet.ID
		}
	}
	return nil
}

// ListMachinePoolInstances returns the WorkerRole core.Subnet id for the cluster
func (m *MachinePoolScope) ListMachinePoolInstances(ctx context.Context) ([]core.InstanceSummary, error) {
	poolOid := m.OCIMachinePool.Spec.OCID
	if len(poolOid) <= 0 {
		return nil, errors.New("OCIMachinePool OCID can't be empty")
	}

	req := core.ListInstancePoolInstancesRequest{
		CompartmentId:  common.String(m.OCICluster.Spec.CompartmentId),
		InstancePoolId: common.String(poolOid),
	}
	resp, err := m.ComputeManagementClient.ListInstancePoolInstances(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Items, nil
}

// SetListandSetMachinePoolInstances retrieves a machine pools instances and sets them in the ProviderIDList
func (m *MachinePoolScope) SetListandSetMachinePoolInstances(ctx context.Context) (int32, error) {
	poolInstanceSummaries, err := m.ListMachinePoolInstances(ctx)
	if err != nil {
		return 0, errors.New("Unable to list machine pool's instances")
	}
	providerIDList := make([]string, len(poolInstanceSummaries))

	for i, instance := range poolInstanceSummaries {
		if *instance.State == "Running" {
			providerIDList[i] = fmt.Sprintf("oci://%s", *instance.Id)
		}
	}

	m.OCIMachinePool.Spec.ProviderIDList = providerIDList

	return int32(len(providerIDList)), nil
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachinePoolScope) GetBootstrapData() (string, error) {
	if m.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked MachinePool's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.MachinePool.Namespace, Name: *m.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName}
	if err := m.Client.Get(context.TODO(), key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for OCIMachinePool %s/%s", m.MachinePool.Namespace, m.MachinePool.Name)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}
	return string(value), nil
}

// GetWorkerMachineNSG returns the worker role core.NetworkSecurityGroup id for the cluster
func (m *MachinePoolScope) GetWorkerMachineNSG() *string {
	for _, nsg := range m.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
		if nsg.Role == infrastructurev1beta1.WorkerRole {
			return nsg.ID
		}
	}
	return nil
}

// BuildInstanceConfigurationShapeConfig builds the core.InstanceConfigurationLaunchInstanceShapeConfigDetails based
// on the MachinePoolScope
func (m *MachinePoolScope) BuildInstanceConfigurationShapeConfig() (core.InstanceConfigurationLaunchInstanceShapeConfigDetails, error) {
	shapeConfig := core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}
	if (m.OCIMachinePool.Spec.ShapeConfig != expinfra1.ShapeConfig{}) {
		ocpuString := m.OCIMachinePool.Spec.ShapeConfig.Ocpus
		if ocpuString != "" {
			ocpus, err := strconv.ParseFloat(ocpuString, 32)
			if err != nil {
				return core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}, errors.New(fmt.Sprintf("ocpus provided %s is not a valid floating point",
					ocpuString))
			}
			shapeConfig.Ocpus = common.Float32(float32(ocpus))
		}

		memoryInGBsString := m.OCIMachinePool.Spec.ShapeConfig.MemoryInGBs
		if memoryInGBsString != "" {
			memoryInGBs, err := strconv.ParseFloat(memoryInGBsString, 32)
			if err != nil {
				return core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}, errors.New(fmt.Sprintf("memoryInGBs provided %s is not a valid floating point",
					memoryInGBsString))
			}
			shapeConfig.MemoryInGBs = common.Float32(float32(memoryInGBs))
		}
		baselineOcpuOptString := m.OCIMachinePool.Spec.ShapeConfig.BaselineOcpuUtilization
		if baselineOcpuOptString != "" {
			value, err := ociutil.GetInstanceConfigBaseLineOcpuOptimizationEnum(baselineOcpuOptString)
			if err != nil {
				return core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}, err
			}
			shapeConfig.BaselineOcpuUtilization = value
		}
	}

	return shapeConfig, nil
}

func (s *MachinePoolScope) BuildInstancePoolPlacement() ([]core.CreateInstancePoolPlacementConfigurationDetails, error) {
	var placements []core.CreateInstancePoolPlacementConfigurationDetails

	ads := s.OCICluster.Status.AvailabilityDomains

	specPlacementDetails := s.OCIMachinePool.Spec.PlacementDetails

	// make sure user doesn't specify 3 ads when there is only one available
	if len(specPlacementDetails) > len(ads) {
		errMsg := fmt.Sprintf("Cluster has %d ADs specified and the machine pools spec has %d", len(ads), len(specPlacementDetails))
		return nil, errors.New(errMsg)
	}

	// build placements from the user spec
	for _, ad := range ads {
		for _, specPlacment := range specPlacementDetails {
			if strings.HasSuffix(ad.Name, strconv.Itoa(specPlacment.AvailabilityDomain)) {
				placement := core.CreateInstancePoolPlacementConfigurationDetails{
					AvailabilityDomain: common.String(ad.Name),
					PrimarySubnetId:    s.GetWorkerMachineSubnet(),
					FaultDomains:       ad.FaultDomains,
				}
				s.Info("Adding machine placement for AD", "AD", ad.Name)
				placements = append(placements, placement)
			}
		}
	}

	// build placements if the user hasn't specified any
	if len(placements) <= 0 {
		for _, ad := range ads {
			placement := core.CreateInstancePoolPlacementConfigurationDetails{
				AvailabilityDomain: common.String(ad.Name),
				PrimarySubnetId:    s.GetWorkerMachineSubnet(),
				FaultDomains:       ad.FaultDomains,
			}
			placements = append(placements, placement)
		}
	}

	return placements, nil
}

// IsResourceCreatedByClusterAPI determines if the instance was created by the cluster using the
// tags created at instance launch.
func (s *MachinePoolScope) IsResourceCreatedByClusterAPI(resourceFreeFormTags map[string]string) bool {
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(string(s.OCICluster.GetOCIResourceIdentifier()))
	for k, v := range tagsAddedByClusterAPI {
		if resourceFreeFormTags[k] != v {
			return false
		}
	}
	return true
}

// GetFreeFormTags gets the free form tags for the MachinePoolScope cluster and returns them
func (m *MachinePoolScope) GetFreeFormTags() map[string]string {

	tags := ociutil.BuildClusterTags(m.OCICluster.GetOCIResourceIdentifier())
	if m.OCICluster.Spec.FreeformTags != nil {
		for k, v := range m.OCICluster.Spec.FreeformTags {
			tags[k] = v
		}
	}

	return tags
}

// ReconcileInstanceConfiguration works to try to reconcile the state of the instance configuration for the cluster
func (m *MachinePoolScope) ReconcileInstanceConfiguration(ctx context.Context) error {
	var instanceConfiguration *core.InstanceConfiguration
	instanceConfigurationId := m.GetInstanceConfigurationId()

	if len(instanceConfigurationId) > 0 {
		req := core.GetInstanceConfigurationRequest{InstanceConfigurationId: common.String(instanceConfigurationId)}
		instanceConfiguration, err := m.ComputeManagementClient.GetInstanceConfiguration(ctx, req)
		if err == nil {
			m.Info("instance configuration found", "InstanceConfigurationId", instanceConfiguration.Id)
			m.SetInstanceConfigurationIdStatus(instanceConfigurationId)
			return m.PatchObject(ctx)
		} else {
			return errors.Wrap(err, fmt.Sprintf("error getting instance configuration by id %s", instanceConfigurationId))
		}
	}

	if instanceConfiguration == nil {
		m.Info("Create new instance configuration")

		tags := m.GetFreeFormTags()

		cloudInitData, err := m.GetBootstrapData()
		if err != nil {
			return err
		}

		metadata := m.OCIMachinePool.Spec.Metadata
		if metadata == nil {
			metadata = make(map[string]string)
		}
		metadata["user_data"] = base64.StdEncoding.EncodeToString([]byte(cloudInitData))

		subnetId := m.GetWorkerMachineSubnet()
		nsgId := m.GetWorkerMachineNSG()

		launchInstanceDetails := core.ComputeInstanceDetails{
			LaunchDetails: &core.InstanceConfigurationLaunchInstanceDetails{
				CompartmentId: common.String(m.OCICluster.Spec.CompartmentId),
				DisplayName:   common.String(m.OCIMachinePool.GetName()),
				Shape:         common.String(m.OCIMachinePool.Spec.InstanceConfiguration.InstanceDetails.Shape),
				SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{
					ImageId: common.String(m.OCIMachinePool.Spec.ImageId),
				},
				Metadata: metadata,
				CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
					SubnetId:       subnetId,
					AssignPublicIp: common.Bool(m.OCIMachinePool.Spec.VNICAssignPublicIp),
					NsgIds:         []string{*nsgId},
				},
			},
		}

		shapeConfig, err := m.BuildInstanceConfigurationShapeConfig()
		if err != nil {
			conditions.MarkFalse(m.MachinePool, infrav1exp.LaunchTemplateReadyCondition, infrav1exp.LaunchTemplateCreateFailedReason, clusterv1.ConditionSeverityError, err.Error())
			m.Info("failed to create instance configuration due to shape config")
			return err
		}
		if (shapeConfig != core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}) {
			launchInstanceDetails.LaunchDetails.ShapeConfig = &shapeConfig
		}

		req := core.CreateInstanceConfigurationRequest{
			CreateInstanceConfiguration: core.CreateInstanceConfigurationDetails{
				CompartmentId:   common.String(m.OCICluster.Spec.CompartmentId),
				DisplayName:     common.String(m.OCIMachinePool.GetName()),
				FreeformTags:    tags,
				InstanceDetails: launchInstanceDetails,
			},
		}

		resp, err := m.ComputeManagementClient.CreateInstanceConfiguration(ctx, req)
		if err != nil {
			conditions.MarkFalse(m.MachinePool, infrav1exp.LaunchTemplateReadyCondition, infrav1exp.LaunchTemplateCreateFailedReason, clusterv1.ConditionSeverityError, err.Error())
			m.Info("failed to create instance configuration")
			return err
		}

		m.SetInstanceConfigurationIdStatus(*resp.Id)
		return m.PatchObject(ctx)
	}

	return nil
}

// FindInstancePool attempts to find the instance pool by name and checks to make sure
// the instance pool was created by the cluster before returning the correct pool
func (m *MachinePoolScope) FindInstancePool(ctx context.Context) (*core.InstancePool, error) {
	// We have to first list the pools to get the instance pool.
	// List returns InstancePoolSummary which lacks some details of InstancePool

	reqList := core.ListInstancePoolsRequest{
		CompartmentId: common.String(m.OCICluster.Spec.CompartmentId),
		DisplayName:   common.String(m.OCIMachinePool.GetName()),
	}
	respList, err := m.ComputeManagementClient.ListInstancePools(ctx, reqList)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query OCIMachinePool by name")
	}

	if len(respList.Items) <= 0 {
		m.Info("No machine pool found", "machinepool-name", m.OCIMachinePool.GetName())
		return nil, nil
	}

	var instancePoolSummary *core.InstancePoolSummary
	for _, i := range respList.Items {
		if m.IsResourceCreatedByClusterAPI(i.FreeformTags) {
			instancePoolSummary = &i
			break
		}
	}
	if instancePoolSummary == nil {
		m.Info("No machine pool found created by this cluster", "machinepool-name", m.OCIMachinePool.GetName())
		return nil, nil
	}

	reqGet := core.GetInstancePoolRequest{
		InstancePoolId: instancePoolSummary.Id,
	}
	respGet, err := m.ComputeManagementClient.GetInstancePool(ctx, reqGet)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query OCIMachinePool with id %s", *instancePoolSummary.Id)
	}

	if !m.IsResourceCreatedByClusterAPI(respGet.InstancePool.FreeformTags) {
		return nil, errors.Wrapf(err, "failed to query OCIMachinePool not created by this cluster.")
	}

	m.Info("Found existing instance pool", "id", instancePoolSummary.Id, "machinepool-name", m.OCIMachinePool.GetName())

	return &respGet.InstancePool, nil
}

// CreateInstancePool attempts to create an instance pool
func (m *MachinePoolScope) CreateInstancePool(ctx context.Context) (*core.InstancePool, error) {
	if m.GetInstanceConfigurationId() == "" {
		return nil, errors.New("OCIMachinePool has no InstanceConfigurationId for some reason")
	}

	tags := m.GetFreeFormTags()

	// build placements
	placements, err := m.BuildInstancePoolPlacement()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to build instance pool placements")
	}

	replicas := int(1)
	if m.MachinePool.Spec.Replicas != nil {
		replicas = int(*m.MachinePool.Spec.Replicas)
	}

	m.Info("Creating Instance Pool")
	req := core.CreateInstancePoolRequest{
		CreateInstancePoolDetails: core.CreateInstancePoolDetails{
			CompartmentId:           common.String(m.OCICluster.Spec.CompartmentId),
			InstanceConfigurationId: common.String(m.GetInstanceConfigurationId()),
			Size:                    common.Int(replicas),
			DisplayName:             common.String(m.OCIMachinePool.GetName()),

			PlacementConfigurations: placements,
			FreeformTags:            tags,
		},
	}
	instancePool, err := m.ComputeManagementClient.CreateInstancePool(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create OCIMachinePool")
	}
	m.Info("Created Instance Pool", "id", instancePool.Id)

	return &instancePool.InstancePool, nil
}

// UpdatePool attempts to update the instance pool
func (m *MachinePoolScope) UpdatePool(ctx context.Context, instancePool *core.InstancePool) error {

	if instancePoolNeedsUpdates(m, instancePool) {
		m.Info("updating instance pool")

		replicas := int(1)
		if m.MachinePool.Spec.Replicas != nil {
			replicas = int(*m.MachinePool.Spec.Replicas)
		}

		req := core.UpdateInstancePoolRequest{InstancePoolId: instancePool.Id,
			UpdateInstancePoolDetails: core.UpdateInstancePoolDetails{
				Size: common.Int(replicas),
			}}

		if _, err := m.ComputeManagementClient.UpdateInstancePool(ctx, req); err != nil {
			return errors.Wrap(err, "unable to update instance pool")
		}
	}

	return nil
}

func (m *MachinePoolScope) TerminateInstancePool(ctx context.Context, instancePool *core.InstancePool) error {
	m.Info("Terminating instance pool", "id", instancePool.Id, "lifecycleState", instancePool.LifecycleState)
	req := core.TerminateInstancePoolRequest{InstancePoolId: instancePool.Id}
	if _, err := m.ComputeManagementClient.TerminateInstancePool(ctx, req); err != nil {
		return err
	}

	return nil
}

// instancePoolNeedsUpdates compares incoming OCIMachinePool and compares against existing instance pool.
func instancePoolNeedsUpdates(machinePoolScope *MachinePoolScope, instancePool *core.InstancePool) bool {

	// Allow pool resize
	if machinePoolScope.MachinePool.Spec.Replicas != nil {
		if instancePool.Size == nil || int(*machinePoolScope.MachinePool.Spec.Replicas) != *instancePool.Size {
			return true
		}
	} else if instancePool.Size != nil {
		return true
	}

	// todo subnet diff

	return false
}
