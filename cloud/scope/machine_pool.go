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
	"reflect"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil/ptr"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/computemanagement"
	expinfra1 "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const OCIMachinePoolKind = "OCIMachinePool"

// MachinePoolScopeParams defines the params need to create a new MachineScope
type MachinePoolScopeParams struct {
	Logger                  *logr.Logger
	Cluster                 *clusterv1.Cluster
	MachinePool             *expclusterv1.MachinePool
	Client                  client.Client
	ComputeManagementClient computemanagement.Client
	OCIClusterAccessor      OCIClusterAccessor
	OCIMachinePool          *expinfra1.OCIMachinePool
}

type MachinePoolScope struct {
	*logr.Logger
	Client                  client.Client
	patchHelper             *patch.Helper
	Cluster                 *clusterv1.Cluster
	MachinePool             *expclusterv1.MachinePool
	ComputeManagementClient computemanagement.Client
	OCIClusterAccesor       OCIClusterAccessor
	OCIMachinePool          *expinfra1.OCIMachinePool
}

// NewMachinePoolScope creates a MachinePoolScope given the MachinePoolScopeParams
func NewMachinePoolScope(params MachinePoolScopeParams) (*MachinePoolScope, error) {
	if params.MachinePool == nil {
		return nil, errors.New("failed to generate new scope from nil MachinePool")
	}
	if params.OCIClusterAccessor == nil {
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
	params.OCIMachinePool.Status.InfrastructureMachineKind = "OCIMachinePoolMachine"
	return &MachinePoolScope{
		Logger:                  params.Logger,
		Client:                  params.Client,
		ComputeManagementClient: params.ComputeManagementClient,
		Cluster:                 params.Cluster,
		OCIClusterAccesor:       params.OCIClusterAccessor,
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
func (m *MachinePoolScope) GetInstanceConfigurationId() *string {
	return m.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId
}

// SetInstanceConfigurationIdStatus sets the MachinePool InstanceConfigurationId status.
func (m *MachinePoolScope) SetInstanceConfigurationIdStatus(id string) {
	m.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = &id
}

// SetFailureMessage sets the OCIMachine status error message.
func (m *MachinePoolScope) SetFailureMessage(v error) {
	m.OCIMachinePool.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// SetFailureReason sets the OCIMachine status error reason.
func (m *MachinePoolScope) SetFailureReason(v string) {
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
	for _, subnet := range ptr.ToSubnetSlice(m.OCIClusterAccesor.GetNetworkSpec().Vcn.Subnets) {
		if subnet.Role == infrastructurev1beta2.WorkerRole {
			return subnet.ID
		}
	}
	return nil
}

// ListMachinePoolInstances returns the list of instances belonging to an instance pool
func (m *MachinePoolScope) ListMachinePoolInstances(ctx context.Context) ([]core.InstanceSummary, error) {
	poolOcid := m.OCIMachinePool.Spec.OCID
	if poolOcid == nil {
		return nil, errors.New("OCIMachinePool OCID can't be empty")
	}

	req := core.ListInstancePoolInstancesRequest{
		CompartmentId:  common.String(m.OCIClusterAccesor.GetCompartmentId()),
		InstancePoolId: poolOcid,
	}

	var instanceSummaries []core.InstanceSummary
	listPoolInstances := func(ctx context.Context, request core.ListInstancePoolInstancesRequest) (core.ListInstancePoolInstancesResponse, error) {
		return m.ComputeManagementClient.ListInstancePoolInstances(ctx, request)
	}
	for resp, err := listPoolInstances(ctx, req); ; resp, err = listPoolInstances(ctx, req) {
		if err != nil {
			return nil, err
		}

		instanceSummaries = append(instanceSummaries, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		} else {
			req.Page = resp.OpcNextPage
		}
	}

	return instanceSummaries, nil
}

// SetListandSetMachinePoolInstances retrieves a machine pools instances and sets them in the ProviderIDList
func (m *MachinePoolScope) SetListandSetMachinePoolInstances(ctx context.Context) ([]infrav2exp.OCIMachinePoolMachine, error) {
	poolInstanceSummaries, err := m.ListMachinePoolInstances(ctx)
	if err != nil {
		return nil, err
	}
	machines := make([]infrav2exp.OCIMachinePoolMachine, 0)

	for _, instance := range poolInstanceSummaries {
		// deleted nodes should not be considered
		if strings.EqualFold(*instance.State, "TERMINATED") {
			continue
		}
		ready := false
		if strings.EqualFold(*instance.State, "RUNNING") {
			ready = true
		}
		machines = append(machines, infrav2exp.OCIMachinePoolMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name: *instance.DisplayName,
			},
			Spec: infrav2exp.OCIMachinePoolMachineSpec{
				OCID:         instance.Id,
				ProviderID:   common.String(m.OCIClusterAccesor.GetProviderID(*instance.Id)),
				InstanceName: instance.DisplayName,
				MachineType:  infrav2exp.SelfManaged,
			},
			Status: infrav2exp.OCIMachinePoolMachineStatus{
				Ready: ready,
			},
		})
	}
	return machines, nil
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
	for _, nsg := range ptr.ToNSGSlice(m.OCIClusterAccesor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List) {
		if nsg.Role == infrastructurev1beta2.WorkerRole {
			return nsg.ID
		}
	}
	return nil
}

func (m *MachinePoolScope) buildInstanceConfigurationShapeConfig() (core.InstanceConfigurationLaunchInstanceShapeConfigDetails, error) {
	shapeConfig := core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}
	shapeConfigSpec := m.OCIMachinePool.Spec.InstanceConfiguration.ShapeConfig
	if shapeConfigSpec != nil {
		if shapeConfigSpec.Ocpus != nil {
			ocpus, err := strconv.ParseFloat(*shapeConfigSpec.Ocpus, 32)
			if err != nil {
				return core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}, errors.New(fmt.Sprintf("ocpus provided %s is not a valid floating point",
					*shapeConfigSpec.Ocpus))
			}
			shapeConfig.Ocpus = common.Float32(float32(ocpus))
		}

		if shapeConfigSpec.MemoryInGBs != nil {
			memoryInGBs, err := strconv.ParseFloat(*shapeConfigSpec.MemoryInGBs, 32)
			if err != nil {
				return core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}, errors.New(fmt.Sprintf("memoryInGBs provided %s is not a valid floating point",
					*shapeConfigSpec.MemoryInGBs))
			}
			shapeConfig.MemoryInGBs = common.Float32(float32(memoryInGBs))
		}
		baselineOcpuOptString := shapeConfigSpec.BaselineOcpuUtilization
		if baselineOcpuOptString != "" {
			value, err := ociutil.GetInstanceConfigBaseLineOcpuOptimizationEnum(baselineOcpuOptString)
			if err != nil {
				return core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}, err
			}
			shapeConfig.BaselineOcpuUtilization = value
		}
		shapeConfig.Nvmes = shapeConfigSpec.Nvmes
	}

	return shapeConfig, nil
}

func (s *MachinePoolScope) BuildInstancePoolPlacement() ([]core.CreateInstancePoolPlacementConfigurationDetails, error) {
	var placements []core.CreateInstancePoolPlacementConfigurationDetails

	ads := s.OCIClusterAccesor.GetAvailabilityDomains()

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
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(string(s.OCIClusterAccesor.GetOCIResourceIdentifier()))
	for k, v := range tagsAddedByClusterAPI {
		if resourceFreeFormTags[k] != v {
			return false
		}
	}
	return true
}

// GetFreeFormTags gets the free form tags for the MachinePoolScope cluster and returns them
func (m *MachinePoolScope) GetFreeFormTags() map[string]string {
	tags := ociutil.BuildClusterTags(m.OCIClusterAccesor.GetOCIResourceIdentifier())
	if m.OCIClusterAccesor.GetFreeformTags() != nil {
		for k, v := range m.OCIClusterAccesor.GetFreeformTags() {
			tags[k] = v
		}
	}

	return tags
}

// ReconcileInstanceConfiguration works to try to reconcile the state of the instance configuration for the cluster
func (m *MachinePoolScope) ReconcileInstanceConfiguration(ctx context.Context) error {
	var instanceConfiguration *core.InstanceConfiguration
	instanceConfiguration, err := m.GetInstanceConfiguration(ctx)
	if err != nil {
		return err
	}
	freeFormTags := m.GetFreeFormTags()
	definedTags := make(map[string]map[string]interface{})
	if m.OCIClusterAccesor.GetDefinedTags() != nil {
		for ns, mapNs := range m.OCIClusterAccesor.GetDefinedTags() {
			mapValues := make(map[string]interface{})
			for k, v := range mapNs {
				mapValues[k] = v
			}
			definedTags[ns] = mapValues
		}
	}
	instanceConfigurationSpec := m.OCIMachinePool.Spec.InstanceConfiguration
	if instanceConfiguration == nil {
		m.Info("Create new instance configuration")

		launchDetails, err := m.getLaunchInstanceDetails(instanceConfigurationSpec, freeFormTags, definedTags)
		if err != nil {
			return err
		}
		err = m.createInstanceConfiguration(ctx, launchDetails, freeFormTags, definedTags)
		if err != nil {
			return err
		}
		return m.PatchObject(ctx)
	} else {
		computeDetails, ok := instanceConfiguration.InstanceDetails.(core.ComputeInstanceDetails)
		if ok {
			launchDetailsActual := computeDetails.LaunchDetails
			launchDetailsSpec, err := m.getLaunchInstanceDetails(instanceConfigurationSpec, freeFormTags, definedTags)
			if err != nil {
				return err
			}
			// do not compare defined tags
			launchDetailsSpec.DefinedTags = nil
			launchDetailsActual.DefinedTags = nil

			if launchDetailsSpec.CreateVnicDetails != nil {
				launchDetailsSpec.CreateVnicDetails.DefinedTags = nil
			}
			if launchDetailsActual.CreateVnicDetails != nil {
				launchDetailsActual.CreateVnicDetails.DefinedTags = nil
			}
			launchDetailsActual.DisplayName = nil
			launchDetailsSpec.DisplayName = nil
			if !reflect.DeepEqual(launchDetailsSpec, launchDetailsActual) {
				m.Logger.Info("Machine pool", "spec", launchDetailsSpec)
				m.Logger.Info("Machine pool", "actual", launchDetailsActual)
				// created the launch details pec again as we may have removed certain fields for comparison purposes
				launchDetailsSpec, err := m.getLaunchInstanceDetails(instanceConfigurationSpec, freeFormTags, definedTags)
				if err != nil {
					return err
				}
				err = m.createInstanceConfiguration(ctx, launchDetailsSpec, freeFormTags, definedTags)
				if err != nil {
					return err
				}
				return m.PatchObject(ctx)
			}
		} else {
			m.Logger.Info("Could not read the instance details as ComputeInstanceDetails, please create a github " +
				"ticket in CAPOCI repository")
		}
	}
	return nil
}

func (m *MachinePoolScope) createInstanceConfiguration(ctx context.Context, launchDetails *core.InstanceConfigurationLaunchInstanceDetails, freeFormTags map[string]string, definedTags map[string]map[string]interface{}) error {
	launchInstanceDetails := core.ComputeInstanceDetails{
		LaunchDetails: launchDetails,
	}
	req := core.CreateInstanceConfigurationRequest{
		CreateInstanceConfiguration: core.CreateInstanceConfigurationDetails{
			CompartmentId:   common.String(m.OCIClusterAccesor.GetCompartmentId()),
			DisplayName:     common.String(fmt.Sprintf("%s-%s", m.OCIMachinePool.GetName(), m.OCIMachinePool.ResourceVersion)),
			FreeformTags:    freeFormTags,
			DefinedTags:     definedTags,
			InstanceDetails: launchInstanceDetails,
		},
	}

	resp, err := m.ComputeManagementClient.CreateInstanceConfiguration(ctx, req)
	if err != nil {
		conditions.MarkFalse(m.MachinePool, infrav2exp.LaunchTemplateReadyCondition, infrav2exp.LaunchTemplateCreateFailedReason, clusterv1.ConditionSeverityError, err.Error())
		m.Info("failed to create instance configuration")
		return err
	}

	m.SetInstanceConfigurationIdStatus(*resp.Id)
	return nil
}

func (m *MachinePoolScope) getLaunchInstanceDetails(instanceConfigurationSpec infrav2exp.InstanceConfiguration, freeFormTags map[string]string, definedTags map[string]map[string]interface{}) (*core.InstanceConfigurationLaunchInstanceDetails, error) {
	metadata := instanceConfigurationSpec.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	cloudInitData, err := m.GetBootstrapData()
	if err != nil {
		return nil, err
	}
	metadata["user_data"] = base64.StdEncoding.EncodeToString([]byte(cloudInitData))

	launchDetails := &core.InstanceConfigurationLaunchInstanceDetails{
		CompartmentId:     common.String(m.OCIClusterAccesor.GetCompartmentId()),
		DisplayName:       common.String(m.OCIMachinePool.GetName()),
		Shape:             common.String(*m.OCIMachinePool.Spec.InstanceConfiguration.Shape),
		Metadata:          metadata,
		DedicatedVmHostId: instanceConfigurationSpec.DedicatedVmHostId,
		FreeformTags:      freeFormTags,
		DefinedTags:       definedTags,
	}

	if instanceConfigurationSpec.CapacityReservationId != nil {
		launchDetails.CapacityReservationId = instanceConfigurationSpec.CapacityReservationId
	}
	launchDetails.CreateVnicDetails = m.getVnicDetails(instanceConfigurationSpec, freeFormTags, definedTags)
	launchDetails.SourceDetails = m.getInstanceConfigurationInstanceSourceViaImageDetail()
	launchDetails.AgentConfig = m.getAgentConfig()
	launchDetails.LaunchOptions = m.getLaunchOptions()
	launchDetails.InstanceOptions = m.getInstanceOptions()
	launchDetails.AvailabilityConfig = m.getAvailabilityConfig()
	launchDetails.PreemptibleInstanceConfig = m.getPreemptibleInstanceConfig()
	launchDetails.PlatformConfig = m.getPlatformConfig()

	shapeConfig, err := m.buildInstanceConfigurationShapeConfig()
	if err != nil {
		conditions.MarkFalse(m.MachinePool, infrav2exp.LaunchTemplateReadyCondition, infrav2exp.LaunchTemplateCreateFailedReason, clusterv1.ConditionSeverityError, err.Error())
		m.Info("failed to create instance configuration due to shape config")
		return nil, err
	}
	if (shapeConfig != core.InstanceConfigurationLaunchInstanceShapeConfigDetails{}) {
		launchDetails.ShapeConfig = &shapeConfig
	}
	return launchDetails, nil
}

// ListInstancePoolSummaries list the core.InstancePoolSummary for the given core.ListInstancePoolsRequest
func (m *MachinePoolScope) ListInstancePoolSummaries(ctx context.Context, req core.ListInstancePoolsRequest) ([]core.InstancePoolSummary, error) {
	listInstancePools := func(ctx context.Context, request core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
		return m.ComputeManagementClient.ListInstancePools(ctx, request)
	}

	var instancePoolSummaries []core.InstancePoolSummary
	for resp, err := listInstancePools(ctx, req); ; resp, err = listInstancePools(ctx, req) {
		if err != nil {
			return instancePoolSummaries, errors.Wrapf(err, "failed to query OCIMachinePool by name")
		}

		instancePoolSummaries = append(instancePoolSummaries, resp.Items...)

		if resp.OpcNextPage == nil {
			// no more pages
			break
		} else {
			req.Page = resp.OpcNextPage
		}
	}

	return instancePoolSummaries, nil
}

// FindInstancePool attempts to find the instance pool by name and checks to make sure
// the instance pool was created by the cluster before returning the correct pool
// nolint:nilnil
func (m *MachinePoolScope) FindInstancePool(ctx context.Context) (*core.InstancePool, error) {
	if m.OCIMachinePool.Spec.OCID != nil {
		response, err := m.ComputeManagementClient.GetInstancePool(ctx, core.GetInstancePoolRequest{
			InstancePoolId: m.OCIMachinePool.Spec.OCID,
		})
		if err != nil {
			return nil, err
		}
		return &response.InstancePool, nil
	}

	// We have to first list the pools to get the instance pool.
	// List returns InstancePoolSummary which lacks some details of InstancePool
	reqList := core.ListInstancePoolsRequest{
		CompartmentId: common.String(m.OCIClusterAccesor.GetCompartmentId()),
		DisplayName:   common.String(m.OCIMachinePool.GetName()),
	}

	instancePoolSummaries, err := m.ListInstancePoolSummaries(ctx, reqList)
	if err != nil {
		return nil, err
	}

	var instancePoolSummary *core.InstancePoolSummary
	for i, summary := range instancePoolSummaries {
		if m.IsResourceCreatedByClusterAPI(summary.FreeformTags) {
			instancePoolSummary = &instancePoolSummaries[i]
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
	if m.GetInstanceConfigurationId() == nil {
		return nil, errors.New("OCIMachinePool has no InstanceConfigurationId")
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
			CompartmentId:           common.String(m.OCIClusterAccesor.GetCompartmentId()),
			InstanceConfigurationId: m.GetInstanceConfigurationId(),
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
func (m *MachinePoolScope) UpdatePool(ctx context.Context, instancePool *core.InstancePool) (*core.InstancePool, error) {
	if instancePoolNeedsUpdates(m, instancePool) {
		m.Info("Updating instance pool")
		replicas := 0
		if m.MachinePool.Spec.Replicas != nil {
			replicas = int(*m.MachinePool.Spec.Replicas)
		}
		req := core.UpdateInstancePoolRequest{InstancePoolId: instancePool.Id,
			UpdateInstancePoolDetails: core.UpdateInstancePoolDetails{
				Size:                    common.Int(replicas),
				InstanceConfigurationId: m.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId,
			},
		}
		resp, err := m.ComputeManagementClient.UpdateInstancePool(ctx, req)
		if err != nil {
			return nil, errors.Wrap(err, "unable to update instance pool")
		}
		m.Info("Successfully updated instance pool")
		return &resp.InstancePool, nil
	}
	return instancePool, nil
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
	instanePoolSize := 0
	machinePoolReplicas := 0
	if machinePoolScope.MachinePool.Spec.Replicas != nil {
		machinePoolReplicas = int(*machinePoolScope.MachinePool.Spec.Replicas)
	}

	if instancePool.Size != nil {
		instanePoolSize = *instancePool.Size
	}
	if machinePoolReplicas != instanePoolSize {
		return true
	} else if !(reflect.DeepEqual(machinePoolScope.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId, instancePool.InstanceConfigurationId)) {
		return true
	}
	return false
}

func (m *MachinePoolScope) getAgentConfig() *core.InstanceConfigurationLaunchInstanceAgentConfigDetails {
	agentConfigSpec := m.OCIMachinePool.Spec.InstanceConfiguration.AgentConfig
	if agentConfigSpec != nil {
		agentConfig := &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{
			IsMonitoringDisabled:  agentConfigSpec.IsMonitoringDisabled,
			IsManagementDisabled:  agentConfigSpec.IsManagementDisabled,
			AreAllPluginsDisabled: agentConfigSpec.AreAllPluginsDisabled,
		}
		if len(agentConfigSpec.PluginsConfig) > 0 {
			pluginConfigList := make([]core.InstanceAgentPluginConfigDetails, len(agentConfigSpec.PluginsConfig))
			for i, pluginConfigSpec := range agentConfigSpec.PluginsConfig {
				pluginConfigRequest := core.InstanceAgentPluginConfigDetails{
					Name: pluginConfigSpec.Name,
				}
				desiredState, ok := core.GetMappingInstanceAgentPluginConfigDetailsDesiredStateEnum(string(pluginConfigSpec.DesiredState))
				if ok {
					pluginConfigRequest.DesiredState = desiredState
				}
				pluginConfigList[i] = pluginConfigRequest
			}
			agentConfig.PluginsConfig = pluginConfigList
		}
		return agentConfig
	}
	return nil
}

func (m *MachinePoolScope) getLaunchOptions() *core.InstanceConfigurationLaunchOptions {
	launcOptionsSpec := m.OCIMachinePool.Spec.InstanceConfiguration.LaunchOptions
	if launcOptionsSpec != nil {
		launchOptions := &core.InstanceConfigurationLaunchOptions{
			IsConsistentVolumeNamingEnabled: launcOptionsSpec.IsConsistentVolumeNamingEnabled,
		}
		if launcOptionsSpec.BootVolumeType != "" {
			bootVolume, _ := core.GetMappingInstanceConfigurationLaunchOptionsBootVolumeTypeEnum(string(launcOptionsSpec.BootVolumeType))
			launchOptions.BootVolumeType = bootVolume
		}
		if launcOptionsSpec.Firmware != "" {
			firmware, _ := core.GetMappingInstanceConfigurationLaunchOptionsFirmwareEnum(string(launcOptionsSpec.Firmware))
			launchOptions.Firmware = firmware
		}
		if launcOptionsSpec.NetworkType != "" {
			networkType, _ := core.GetMappingInstanceConfigurationLaunchOptionsNetworkTypeEnum(string(launcOptionsSpec.NetworkType))
			launchOptions.NetworkType = networkType
		}
		if launcOptionsSpec.RemoteDataVolumeType != "" {
			remoteVolumeType, _ := core.GetMappingInstanceConfigurationLaunchOptionsRemoteDataVolumeTypeEnum(string(launcOptionsSpec.RemoteDataVolumeType))
			launchOptions.RemoteDataVolumeType = remoteVolumeType
		}
		return launchOptions
	}
	return nil
}

func (m *MachinePoolScope) getInstanceOptions() *core.InstanceConfigurationInstanceOptions {
	instanceOptionsSpec := m.OCIMachinePool.Spec.InstanceConfiguration.InstanceOptions
	if instanceOptionsSpec != nil {
		return &core.InstanceConfigurationInstanceOptions{
			AreLegacyImdsEndpointsDisabled: instanceOptionsSpec.AreLegacyImdsEndpointsDisabled,
		}
	}
	return nil
}

func (m *MachinePoolScope) getInstanceConfigurationInstanceSourceViaImageDetail() core.InstanceConfigurationInstanceSourceViaImageDetails {
	sourceConfig := m.OCIMachinePool.Spec.InstanceConfiguration.InstanceSourceViaImageDetails
	if sourceConfig != nil {
		return core.InstanceConfigurationInstanceSourceViaImageDetails{
			ImageId:             sourceConfig.ImageId,
			BootVolumeVpusPerGB: sourceConfig.BootVolumeVpusPerGB,
			BootVolumeSizeInGBs: sourceConfig.BootVolumeSizeInGBs,
		}
	}
	return core.InstanceConfigurationInstanceSourceViaImageDetails{}
}

func (m *MachinePoolScope) getAvailabilityConfig() *core.InstanceConfigurationAvailabilityConfig {
	avalabilityConfigSpec := m.OCIMachinePool.Spec.InstanceConfiguration.AvailabilityConfig
	if avalabilityConfigSpec != nil {
		recoveryAction, _ := core.GetMappingInstanceConfigurationAvailabilityConfigRecoveryActionEnum(string(avalabilityConfigSpec.RecoveryAction))
		return &core.InstanceConfigurationAvailabilityConfig{
			RecoveryAction: recoveryAction,
		}
	}
	return nil
}

func (m *MachinePoolScope) getPreemptibleInstanceConfig() *core.PreemptibleInstanceConfigDetails {
	preEmptibleInstanceConfigSpec := m.OCIMachinePool.Spec.InstanceConfiguration.PreemptibleInstanceConfig
	if preEmptibleInstanceConfigSpec != nil {
		preemptibleInstanceConfig := &core.PreemptibleInstanceConfigDetails{}
		if preEmptibleInstanceConfigSpec.TerminatePreemptionAction != nil {
			preemptibleInstanceConfig.PreemptionAction = core.TerminatePreemptionAction{
				PreserveBootVolume: preEmptibleInstanceConfigSpec.TerminatePreemptionAction.PreserveBootVolume,
			}
		}
		return preemptibleInstanceConfig
	}
	return nil
}

func (m *MachinePoolScope) getPlatformConfig() core.PlatformConfig {
	platformConfig := m.OCIMachinePool.Spec.InstanceConfiguration.PlatformConfig
	if platformConfig != nil {
		switch platformConfig.PlatformConfigType {
		case infrastructurev1beta2.PlatformConfigTypeAmdRomeBmGpu:
			numaNodesPerSocket, _ := core.GetMappingAmdRomeBmGpuPlatformConfigNumaNodesPerSocketEnum(string(platformConfig.AmdRomeBmGpuPlatformConfig.NumaNodesPerSocket))
			return core.AmdRomeBmGpuPlatformConfig{
				IsSecureBootEnabled:                      platformConfig.AmdRomeBmGpuPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled:           platformConfig.AmdRomeBmGpuPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:                    platformConfig.AmdRomeBmGpuPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:                platformConfig.AmdRomeBmGpuPlatformConfig.IsMemoryEncryptionEnabled,
				IsSymmetricMultiThreadingEnabled:         platformConfig.AmdRomeBmGpuPlatformConfig.IsSymmetricMultiThreadingEnabled,
				IsAccessControlServiceEnabled:            platformConfig.AmdRomeBmGpuPlatformConfig.IsAccessControlServiceEnabled,
				AreVirtualInstructionsEnabled:            platformConfig.AmdRomeBmGpuPlatformConfig.AreVirtualInstructionsEnabled,
				IsInputOutputMemoryManagementUnitEnabled: platformConfig.AmdRomeBmGpuPlatformConfig.IsInputOutputMemoryManagementUnitEnabled,
				NumaNodesPerSocket:                       numaNodesPerSocket,
			}
		case infrastructurev1beta2.PlatformConfigTypeAmdRomeBm:
			numaNodesPerSocket, _ := core.GetMappingAmdRomeBmPlatformConfigNumaNodesPerSocketEnum(string(platformConfig.AmdRomeBmPlatformConfig.NumaNodesPerSocket))
			return core.AmdRomeBmPlatformConfig{
				IsSecureBootEnabled:                      platformConfig.AmdRomeBmPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled:           platformConfig.AmdRomeBmPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:                    platformConfig.AmdRomeBmPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:                platformConfig.AmdRomeBmPlatformConfig.IsMemoryEncryptionEnabled,
				IsSymmetricMultiThreadingEnabled:         platformConfig.AmdRomeBmPlatformConfig.IsSymmetricMultiThreadingEnabled,
				IsAccessControlServiceEnabled:            platformConfig.AmdRomeBmPlatformConfig.IsAccessControlServiceEnabled,
				AreVirtualInstructionsEnabled:            platformConfig.AmdRomeBmPlatformConfig.AreVirtualInstructionsEnabled,
				IsInputOutputMemoryManagementUnitEnabled: platformConfig.AmdRomeBmPlatformConfig.IsInputOutputMemoryManagementUnitEnabled,
				PercentageOfCoresEnabled:                 platformConfig.AmdRomeBmPlatformConfig.PercentageOfCoresEnabled,
				NumaNodesPerSocket:                       numaNodesPerSocket,
			}
		case infrastructurev1beta2.PlatformConfigTypeIntelIcelakeBm:
			numaNodesPerSocket, _ := core.GetMappingIntelIcelakeBmPlatformConfigNumaNodesPerSocketEnum(string(platformConfig.IntelIcelakeBmPlatformConfig.NumaNodesPerSocket))
			return core.IntelIcelakeBmPlatformConfig{
				IsSecureBootEnabled:                      platformConfig.IntelIcelakeBmPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled:           platformConfig.IntelIcelakeBmPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:                    platformConfig.IntelIcelakeBmPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:                platformConfig.IntelIcelakeBmPlatformConfig.IsMemoryEncryptionEnabled,
				IsSymmetricMultiThreadingEnabled:         platformConfig.IntelIcelakeBmPlatformConfig.IsSymmetricMultiThreadingEnabled,
				PercentageOfCoresEnabled:                 platformConfig.IntelIcelakeBmPlatformConfig.PercentageOfCoresEnabled,
				IsInputOutputMemoryManagementUnitEnabled: platformConfig.IntelIcelakeBmPlatformConfig.IsInputOutputMemoryManagementUnitEnabled,
				NumaNodesPerSocket:                       numaNodesPerSocket,
			}
		case infrastructurev1beta2.PlatformConfigTypeAmdvm:
			return core.AmdVmPlatformConfig{
				IsSecureBootEnabled:            platformConfig.AmdVmPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled: platformConfig.AmdVmPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:          platformConfig.AmdVmPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:      platformConfig.AmdVmPlatformConfig.IsMemoryEncryptionEnabled,
			}
		case infrastructurev1beta2.PlatformConfigTypeIntelVm:
			return core.IntelVmPlatformConfig{
				IsSecureBootEnabled:            platformConfig.IntelVmPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled: platformConfig.IntelVmPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:          platformConfig.IntelVmPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:      platformConfig.IntelVmPlatformConfig.IsMemoryEncryptionEnabled,
			}
		case infrastructurev1beta2.PlatformConfigTypeIntelSkylakeBm:
			return core.IntelSkylakeBmPlatformConfig{
				IsSecureBootEnabled:            platformConfig.IntelSkylakeBmPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled: platformConfig.IntelSkylakeBmPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:          platformConfig.IntelSkylakeBmPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:      platformConfig.IntelSkylakeBmPlatformConfig.IsMemoryEncryptionEnabled,
			}
		case infrastructurev1beta2.PlatformConfigTypeAmdMilanBm:
			numaNodesPerSocket, _ := core.GetMappingAmdMilanBmPlatformConfigNumaNodesPerSocketEnum(string(platformConfig.AmdMilanBmPlatformConfig.NumaNodesPerSocket))
			return core.AmdMilanBmPlatformConfig{
				IsSecureBootEnabled:                      platformConfig.AmdMilanBmPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled:           platformConfig.AmdMilanBmPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:                    platformConfig.AmdMilanBmPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:                platformConfig.AmdMilanBmPlatformConfig.IsMemoryEncryptionEnabled,
				IsSymmetricMultiThreadingEnabled:         platformConfig.AmdMilanBmPlatformConfig.IsSymmetricMultiThreadingEnabled,
				IsAccessControlServiceEnabled:            platformConfig.AmdMilanBmPlatformConfig.IsAccessControlServiceEnabled,
				AreVirtualInstructionsEnabled:            platformConfig.AmdMilanBmPlatformConfig.AreVirtualInstructionsEnabled,
				IsInputOutputMemoryManagementUnitEnabled: platformConfig.AmdMilanBmPlatformConfig.IsInputOutputMemoryManagementUnitEnabled,
				PercentageOfCoresEnabled:                 platformConfig.AmdMilanBmPlatformConfig.PercentageOfCoresEnabled,
				NumaNodesPerSocket:                       numaNodesPerSocket,
			}
		default:
		}
	}
	return nil
}

func (m *MachinePoolScope) getWorkerMachineNSGs() []string {
	instanceVnicConfiguration := m.OCIMachinePool.Spec.InstanceConfiguration.InstanceVnicConfiguration
	if instanceVnicConfiguration != nil && len(instanceVnicConfiguration.NsgNames) > 0 {
		nsgs := make([]string, 0)
		for _, nsgName := range instanceVnicConfiguration.NsgNames {
			for _, nsg := range ptr.ToNSGSlice(m.OCIClusterAccesor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List) {
				if nsg.Name == nsgName {
					nsgs = append(nsgs, ptr.ToString(nsg.ID))
				}
			}
		}
		return nsgs
	} else {
		nsgs := make([]string, 0)
		for _, nsg := range ptr.ToNSGSlice(m.OCIClusterAccesor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List) {
			if nsg.Role == infrastructurev1beta2.WorkerRole {
				nsgs = append(nsgs, ptr.ToString(nsg.ID))
			}
		}
		return nsgs
	}
}

// GetInstanceConfiguration returns the instance configuration associated with the instance pool
// nolint:nilnil
func (m *MachinePoolScope) GetInstanceConfiguration(ctx context.Context) (*core.InstanceConfiguration, error) {
	instanceConfigurationId := m.GetInstanceConfigurationId()
	if instanceConfigurationId != nil {
		return m.getInstanceConfigurationFromOCID(ctx, instanceConfigurationId)
	}

	ids, err := m.getInstanceConfigurationsFromDisplayNameSortedTimeCreateDescending(ctx, m.OCIMachinePool.GetName())
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		return m.getInstanceConfigurationFromOCID(ctx, ids[0])
	}
	return nil, nil
}

func (m *MachinePoolScope) CleanupInstanceConfiguration(ctx context.Context, instancePool *core.InstancePool) error {
	m.Info("Cleaning up unused instance configurations")
	ids, err := m.getInstanceConfigurationsFromDisplayNameSortedTimeCreateDescending(ctx, m.OCIMachinePool.GetName())
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		m.Info(fmt.Sprintf("Number of instance configurations found are %d", len(ids)))
		for _, id := range ids {
			// if instance pool is nil, delete all configurations associated with the pool
			if instancePool == nil || !reflect.DeepEqual(id, instancePool.InstanceConfigurationId) {
				req := core.DeleteInstanceConfigurationRequest{InstanceConfigurationId: id}
				if _, err := m.ComputeManagementClient.DeleteInstanceConfiguration(ctx, req); err != nil {
					return errors.Wrap(err, "failed to delete expired instance configuration")
				}
			}
		}
	}
	return nil
}

func (m *MachinePoolScope) getInstanceConfigurationFromOCID(ctx context.Context, instanceConfigurationId *string) (*core.InstanceConfiguration, error) {
	req := core.GetInstanceConfigurationRequest{InstanceConfigurationId: instanceConfigurationId}
	instanceConfiguration, err := m.ComputeManagementClient.GetInstanceConfiguration(ctx, req)
	if err != nil {
		return nil, err
	}
	return &instanceConfiguration.InstanceConfiguration, nil
}

func (m *MachinePoolScope) getInstanceConfigurationsFromDisplayNameSortedTimeCreateDescending(ctx context.Context, displayName string) ([]*string, error) {
	listInstanceConfiguration := func(ctx context.Context, request core.ListInstanceConfigurationsRequest) (core.ListInstanceConfigurationsResponse, error) {
		return m.ComputeManagementClient.ListInstanceConfigurations(ctx, request)
	}

	req := core.ListInstanceConfigurationsRequest{
		CompartmentId: common.String(m.OCIClusterAccesor.GetCompartmentId()),
		SortBy:        core.ListInstanceConfigurationsSortByTimecreated,
		SortOrder:     core.ListInstanceConfigurationsSortOrderDesc,
	}
	ids := make([]*string, 0)
	for {
		resp, err := listInstanceConfiguration(ctx, req)

		if err != nil {
			return nil, errors.Wrapf(err, "failed to query InstanceConfiguration by name")
		}

		for _, instanceConfiguration := range resp.Items {
			if strings.HasPrefix(*instanceConfiguration.DisplayName, displayName) &&
				m.IsResourceCreatedByClusterAPI(instanceConfiguration.FreeformTags) {
				ids = append(ids, instanceConfiguration.Id)
			}
		}
		if resp.OpcNextPage == nil {
			// no more pages
			break
		} else {
			req.Page = resp.OpcNextPage
		}
	}

	return ids, nil
}

func (m *MachinePoolScope) getVnicDetails(instanceConfigurationSpec infrav2exp.InstanceConfiguration, freeFormTags map[string]string, definedTags map[string]map[string]interface{}) *core.InstanceConfigurationCreateVnicDetails {
	subnetId := m.GetWorkerMachineSubnet()
	createVnicDetails := core.InstanceConfigurationCreateVnicDetails{
		SubnetId:     subnetId,
		FreeformTags: freeFormTags,
		DefinedTags:  definedTags,
		NsgIds:       m.getWorkerMachineNSGs(),
	}
	if instanceConfigurationSpec.InstanceVnicConfiguration != nil {
		createVnicDetails.AssignPublicIp = &instanceConfigurationSpec.InstanceVnicConfiguration.AssignPublicIp
		createVnicDetails.HostnameLabel = instanceConfigurationSpec.InstanceVnicConfiguration.HostnameLabel
		createVnicDetails.SkipSourceDestCheck = instanceConfigurationSpec.InstanceVnicConfiguration.SkipSourceDestCheck
		createVnicDetails.AssignPrivateDnsRecord = instanceConfigurationSpec.InstanceVnicConfiguration.AssignPrivateDnsRecord
		createVnicDetails.DisplayName = instanceConfigurationSpec.InstanceVnicConfiguration.DisplayName
	}
	return &createVnicDetails
}
