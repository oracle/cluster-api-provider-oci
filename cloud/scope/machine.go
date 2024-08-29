/*
 Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/url"
	"strconv"

	"github.com/go-logr/logr"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute"
	lb "github.com/oracle/cluster-api-provider-oci/cloud/services/loadbalancer"
	nlb "github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"
	wr "github.com/oracle/cluster-api-provider-oci/cloud/services/workrequests"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiUtil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const OCIMachineKind = "OCIMachine"

// MachineScopeParams defines the params need to create a new MachineScope
type MachineScopeParams struct {
	Logger                    *logr.Logger
	Cluster                   *clusterv1.Cluster
	Machine                   *clusterv1.Machine
	Client                    client.Client
	ComputeClient             compute.ComputeClient
	OCIClusterAccessor        OCIClusterAccessor
	OCIMachine                *infrastructurev1beta2.OCIMachine
	VCNClient                 vcn.Client
	NetworkLoadBalancerClient nlb.NetworkLoadBalancerClient
	LoadBalancerClient        lb.LoadBalancerClient
	WorkRequestsClient        wr.Client
}

type MachineScope struct {
	*logr.Logger
	Client                    client.Client
	patchHelper               *patch.Helper
	Cluster                   *clusterv1.Cluster
	Machine                   *clusterv1.Machine
	ComputeClient             compute.ComputeClient
	OCIClusterAccessor        OCIClusterAccessor
	OCIMachine                *infrastructurev1beta2.OCIMachine
	VCNClient                 vcn.Client
	NetworkLoadBalancerClient nlb.NetworkLoadBalancerClient
	LoadBalancerClient        lb.LoadBalancerClient
	WorkRequestsClient        wr.Client
}

// NewMachineScope creates a MachineScope given the MachineScopeParams
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Machine == nil {
		return nil, errors.New("failed to generate new scope from nil Machine")
	}
	if params.OCIClusterAccessor == nil {
		return nil, errors.New("failed to generate new scope from nil OCICluster")
	}

	if params.Logger == nil {
		log := klogr.New()
		params.Logger = &log
	}
	helper, err := patch.NewHelper(params.OCIMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &MachineScope{
		Logger:                    params.Logger,
		Client:                    params.Client,
		ComputeClient:             params.ComputeClient,
		Cluster:                   params.Cluster,
		OCIClusterAccessor:        params.OCIClusterAccessor,
		patchHelper:               helper,
		Machine:                   params.Machine,
		OCIMachine:                params.OCIMachine,
		VCNClient:                 params.VCNClient,
		NetworkLoadBalancerClient: params.NetworkLoadBalancerClient,
		LoadBalancerClient:        params.LoadBalancerClient,
		WorkRequestsClient:        params.WorkRequestsClient,
	}, nil
}

// GetOrCreateMachine will get machine instance or create if the instances doesn't exist
func (m *MachineScope) GetOrCreateMachine(ctx context.Context) (*core.Instance, error) {
	instance, err := m.GetMachine(ctx)
	if err != nil {
		return nil, err
	}
	if instance != nil {
		m.Logger.Info("Found an existing instance")
		return instance, nil
	}
	m.Logger.Info("Creating machine with name", "machine-name", m.OCIMachine.GetName())

	cloudInitData, err := m.GetBootstrapData()
	if err != nil {
		return nil, err
	}

	shapeConfig := core.LaunchInstanceShapeConfigDetails{}
	if (m.OCIMachine.Spec.ShapeConfig != infrastructurev1beta2.ShapeConfig{}) {
		ocpuString := m.OCIMachine.Spec.ShapeConfig.Ocpus
		if ocpuString != "" {
			ocpus, err := strconv.ParseFloat(ocpuString, 32)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("ocpus provided %s is not a valid floating point",
					ocpuString))
			}
			shapeConfig.Ocpus = common.Float32(float32(ocpus))
		}

		memoryInGBsString := m.OCIMachine.Spec.ShapeConfig.MemoryInGBs
		if memoryInGBsString != "" {
			memoryInGBs, err := strconv.ParseFloat(memoryInGBsString, 32)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("memoryInGBs provided %s is not a valid floating point",
					memoryInGBsString))
			}
			shapeConfig.MemoryInGBs = common.Float32(float32(memoryInGBs))
		}
		baselineOcpuOptString := m.OCIMachine.Spec.ShapeConfig.BaselineOcpuUtilization
		if baselineOcpuOptString != "" {
			value, err := ociutil.GetBaseLineOcpuOptimizationEnum(baselineOcpuOptString)
			if err != nil {
				return nil, err
			}
			shapeConfig.BaselineOcpuUtilization = value
		}
	}
	sourceDetails := core.InstanceSourceViaImageDetails{
		ImageId: common.String(m.OCIMachine.Spec.ImageId),
	}
	if m.OCIMachine.Spec.BootVolumeSizeInGBs != "" {
		bootVolumeSizeInGBsString := m.OCIMachine.Spec.BootVolumeSizeInGBs
		if bootVolumeSizeInGBsString != "" {
			bootVolumeSizeInGBs, err := strconv.ParseFloat(bootVolumeSizeInGBsString, 64)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("bootVolumeSizeInGBs provided %s is not a valid floating point",
					bootVolumeSizeInGBsString))
			}
			sourceDetails.BootVolumeSizeInGBs = common.Int64(int64(bootVolumeSizeInGBs))
		}
	}
	if m.OCIMachine.Spec.InstanceSourceViaImageDetails != nil {
		sourceDetails.KmsKeyId = m.OCIMachine.Spec.InstanceSourceViaImageDetails.KmsKeyId
		sourceDetails.BootVolumeVpusPerGB = m.OCIMachine.Spec.InstanceSourceViaImageDetails.BootVolumeVpusPerGB
	}

	subnetId := m.OCIMachine.Spec.NetworkDetails.SubnetId
	if subnetId == nil {
		if m.IsControlPlane() {
			subnetId = m.getGetControlPlaneMachineSubnet()
		} else {
			subnetId = m.getWorkerMachineSubnet()
		}
	}

	var nsgIds []string
	machineNsgIds := m.OCIMachine.Spec.NetworkDetails.NSGIds
	nsgId := m.OCIMachine.Spec.NetworkDetails.NSGId
	if machineNsgIds != nil && len(machineNsgIds) > 0 {
		nsgIds = machineNsgIds
	} else if nsgId != nil {
		nsgIds = []string{*nsgId}
	} else {
		if m.IsControlPlane() {
			nsgIds = m.getGetControlPlaneMachineNSGs()
		} else {
			nsgIds = m.getWorkerMachineNSGs()
		}
	}

	failureDomain := m.Machine.Spec.FailureDomain
	var faultDomain string
	var availabilityDomain string
	if failureDomain != nil {
		failureDomainIndex, err := strconv.Atoi(*failureDomain)
		if err != nil {
			m.Logger.Error(err, "Failure Domain is not a valid integer")
			return nil, errors.Wrap(err, "invalid failure domain parameter, must be a valid integer")
		}
		m.Logger.Info("Failure Domain being used", "failure-domain", failureDomainIndex)
		if failureDomainIndex < 1 || failureDomainIndex > 3 {
			err = errors.New("failure domain should be a value between 1 and 3")
			m.Logger.Error(err, "Failure domain should be a value between 1 and 3")
			return nil, err
		}
		faultDomain = m.OCIClusterAccessor.GetFailureDomains()[*failureDomain].Attributes[FaultDomain]
		availabilityDomain = m.OCIClusterAccessor.GetFailureDomains()[*failureDomain].Attributes[AvailabilityDomain]
	} else {
		randomFailureDomain, err := rand.Int(rand.Reader, big.NewInt(3))
		if err != nil {
			m.Logger.Error(err, "Failed to generate random failure domain")
			return nil, err
		}
		// the random number generated is between zero and two, whereas we need a number between one and three
		failureDomain = common.String(strconv.Itoa(int(randomFailureDomain.Int64()) + 1))
		availabilityDomain = m.OCIClusterAccessor.GetFailureDomains()[*failureDomain].Attributes[AvailabilityDomain]
	}

	metadata := m.OCIMachine.Spec.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["user_data"] = base64.StdEncoding.EncodeToString([]byte(cloudInitData))

	tags := m.getFreeFormTags()

	definedTags := ConvertMachineDefinedTags(m.OCIMachine.Spec.DefinedTags)

	launchDetails := core.LaunchInstanceDetails{DisplayName: common.String(m.OCIMachine.Name),
		SourceDetails: sourceDetails,
		CreateVnicDetails: &core.CreateVnicDetails{
			SubnetId:               subnetId,
			AssignPublicIp:         common.Bool(m.OCIMachine.Spec.NetworkDetails.AssignPublicIp),
			FreeformTags:           tags,
			DefinedTags:            definedTags,
			HostnameLabel:          m.OCIMachine.Spec.NetworkDetails.HostnameLabel,
			SkipSourceDestCheck:    m.OCIMachine.Spec.NetworkDetails.SkipSourceDestCheck,
			AssignPrivateDnsRecord: m.OCIMachine.Spec.NetworkDetails.AssignPrivateDnsRecord,
			DisplayName:            m.OCIMachine.Spec.NetworkDetails.DisplayName,
		},
		ComputeClusterId:               m.OCIMachine.Spec.ComputeClusterId,
		Metadata:                       metadata,
		Shape:                          common.String(m.OCIMachine.Spec.Shape),
		AvailabilityDomain:             common.String(availabilityDomain),
		CompartmentId:                  common.String(m.getCompartmentId()),
		IsPvEncryptionInTransitEnabled: common.Bool(m.OCIMachine.Spec.IsPvEncryptionInTransitEnabled),
		FreeformTags:                   tags,
		DefinedTags:                    definedTags,
		//		ExtendedMetadata:               m.OCIMachine.Spec.ExtendedMetadata,
		DedicatedVmHostId: m.OCIMachine.Spec.DedicatedVmHostId,
	}
	// Compute API does not behave well if the shape config is empty for fixed shapes
	// hence set it only if it non empty
	if (shapeConfig != core.LaunchInstanceShapeConfigDetails{}) {
		launchDetails.ShapeConfig = &shapeConfig
	}
	if faultDomain != "" {
		launchDetails.FaultDomain = common.String(faultDomain)
	}
	launchDetails.CreateVnicDetails.NsgIds = nsgIds
	if m.OCIMachine.Spec.CapacityReservationId != nil {
		launchDetails.CapacityReservationId = m.OCIMachine.Spec.CapacityReservationId
	}
	launchDetails.AgentConfig = m.getAgentConfig()
	launchDetails.LaunchOptions = m.getLaunchOptions()
	launchDetails.InstanceOptions = m.getInstanceOptions()
	launchDetails.AvailabilityConfig = m.getAvailabilityConfig()
	launchDetails.PreemptibleInstanceConfig = m.getPreemptibleInstanceConfig()
	launchDetails.PlatformConfig = m.getPlatformConfig()
	launchDetails.LaunchVolumeAttachments = m.getLaunchVolumeAttachments()
	req := core.LaunchInstanceRequest{LaunchInstanceDetails: launchDetails,
		OpcRetryToken: ociutil.GetOPCRetryToken(string(m.OCIMachine.UID))}
	resp, err := m.ComputeClient.LaunchInstance(ctx, req)
	if err != nil {
		return nil, err
	} else {
		return &resp.Instance, nil
	}
}

func (m *MachineScope) getFreeFormTags() map[string]string {
	tags := ociutil.BuildClusterTags(m.OCIClusterAccessor.GetOCIResourceIdentifier())
	// first use cluster level tags, then override with machine level tags
	if m.OCIClusterAccessor.GetFreeformTags() != nil {
		for k, v := range m.OCIClusterAccessor.GetFreeformTags() {
			tags[k] = v
		}
	}
	if m.OCIMachine.Spec.FreeformTags != nil {
		for k, v := range m.OCIMachine.Spec.FreeformTags {
			tags[k] = v
		}
	}
	return tags
}

// DeleteMachine terminates the instance using InstanceId from the OCIMachine spec and deletes the boot volume
func (m *MachineScope) DeleteMachine(ctx context.Context, instance *core.Instance) error {
	req := core.TerminateInstanceRequest{InstanceId: instance.Id,
		PreserveBootVolume:                 common.Bool(m.OCIMachine.Spec.PreserveBootVolume),
		PreserveDataVolumesCreatedAtLaunch: common.Bool(m.OCIMachine.Spec.PreserveDataVolumesCreatedAtLaunch),
	}
	_, err := m.ComputeClient.TerminateInstance(ctx, req)
	return err
}

// IsResourceCreatedByClusterAPI determines if the instance was created by the cluster using the
// tags created at instance launch.
func (m *MachineScope) IsResourceCreatedByClusterAPI(resourceFreeFormTags map[string]string) bool {
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(string(m.OCIClusterAccessor.GetOCIResourceIdentifier()))
	for k, v := range tagsAddedByClusterAPI {
		if resourceFreeFormTags[k] != v {
			return false
		}
	}
	return true
}

func (m *MachineScope) getMachineFromOCID(ctx context.Context, instanceID *string) (*core.Instance, error) {
	req := core.GetInstanceRequest{InstanceId: instanceID}

	// Send the request using the service client
	resp, err := m.ComputeClient.GetInstance(ctx, req)
	if err != nil {
		return nil, err
	}
	return &resp.Instance, nil
}

// GetMachineByDisplayName returns the machine from the compartment if there is a matching DisplayName,
// and it was created by the cluster
func (m *MachineScope) GetMachineByDisplayName(ctx context.Context, name string) (*core.Instance, error) {
	var page *string
	for {
		req := core.ListInstancesRequest{DisplayName: common.String(name),
			CompartmentId: common.String(m.getCompartmentId()), Page: page}
		resp, err := m.ComputeClient.ListInstances(ctx, req)
		if err != nil {
			return nil, err
		}
		if len(resp.Items) == 0 {
			return nil, nil
		}
		for _, instance := range resp.Items {
			if m.IsResourceCreatedByClusterAPI(instance.FreeformTags) {
				return &instance, nil
			}
		}

		if resp.OpcNextPage == nil {
			break
		} else {
			page = resp.OpcNextPage
		}
	}
	return nil, nil
}

// PatchObject persists the cluster configuration and status.
func (m *MachineScope) PatchObject(ctx context.Context) error {
	conditions.SetSummary(m.OCIMachine)
	return m.patchHelper.Patch(ctx, m.OCIMachine)
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *MachineScope) Close(ctx context.Context) error {
	return m.PatchObject(ctx)
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetBootstrapData() (string, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.Machine.Namespace, Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	if err := m.Client.Get(context.TODO(), key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for OCIMachine %s/%s", m.Machine.Namespace, m.Machine.Name)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}
	return string(value), nil
}

// Name returns the OCIMachine name.
func (m *MachineScope) Name() string {
	return m.OCIMachine.Name
}

// GetInstanceId returns the OCIMachine instance id.
func (m *MachineScope) GetInstanceId() *string {
	return m.OCIMachine.Spec.InstanceId
}

// SetReady sets the OCIMachine Ready Status.
func (m *MachineScope) SetReady() {
	m.OCIMachine.Status.Ready = true
}

// SetNotReady sets the OCIMachine Ready Status to false.
func (m *MachineScope) SetNotReady() {
	m.OCIMachine.Status.Ready = false
}

// IsReady returns the ready status of the machine.
func (m *MachineScope) IsReady() bool {
	return m.OCIMachine.Status.Ready
}

// SetFailureMessage sets the OCIMachine status error message.
func (m *MachineScope) SetFailureMessage(v error) {
	m.OCIMachine.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// SetFailureReason sets the OCIMachine status error reason.
func (m *MachineScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.OCIMachine.Status.FailureReason = &v
}

// GetMachine will attempt to get the machine instance by instance id, or display name if not instance id
func (m *MachineScope) GetMachine(ctx context.Context) (*core.Instance, error) {
	if m.GetInstanceId() != nil {
		return m.getMachineFromOCID(ctx, m.GetInstanceId())
	}
	instance, err := m.GetMachineByDisplayName(ctx, m.OCIMachine.Name)
	if err != nil {
		return nil, err
	}
	return instance, err
}

// GetMachineIPFromStatus returns the IP address from the OCIMachine's status if it is the Internal IP
func (m *MachineScope) GetMachineIPFromStatus() (string, error) {
	machine := m.OCIMachine
	if machine.Status.Addresses == nil || len(machine.Status.Addresses) == 0 {
		return "", errors.New("could not find machine IP Address in status object")
	}
	for _, ip := range machine.Status.Addresses {
		if ip.Type == clusterv1.MachineInternalIP {
			return ip.Address, nil
		}
	}
	return "", errors.New("could not find machine Internal IP Address in status object")
}

// GetInstanceIp returns the OCIMachine's instance IP from its primary VNIC attachment.
//
// See https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/managingVNICs.htm for more on VNICs
func (m *MachineScope) GetInstanceIp(ctx context.Context) (*string, error) {
	var page *string
	for {
		resp, err := m.ComputeClient.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
			InstanceId:    m.GetInstanceId(),
			CompartmentId: common.String(m.getCompartmentId()),
			Page:          page,
		})
		if err != nil {
			return nil, err
		}

		for _, attachment := range resp.Items {
			if attachment.LifecycleState != core.VnicAttachmentLifecycleStateAttached {
				m.Logger.Info("VNIC attachment is not in attached state", "vnicAttachmentID", *attachment.Id)
				continue
			}

			if attachment.VnicId == nil {
				// Should never happen but lets be extra cautious as field is non-mandatory in OCI API.
				m.Logger.Error(errors.New("VNIC attachment is attached but has no VNIC ID"), "vnicAttachmentID", *attachment.Id)
				continue
			}
			vnic, err := m.VCNClient.GetVnic(ctx, core.GetVnicRequest{
				VnicId: attachment.VnicId,
			})

			if err != nil {
				return nil, err
			}
			if vnic.IsPrimary != nil && *vnic.IsPrimary {
				return vnic.PrivateIp, nil
			}
		}

		if resp.OpcNextPage == nil {
			break
		} else {
			page = resp.OpcNextPage
		}
	}

	return nil, errors.New("primary VNIC not found")
}

// ReconcileCreateInstanceOnLB sets up backend sets for the load balancer
func (m *MachineScope) ReconcileCreateInstanceOnLB(ctx context.Context) error {
	instanceIp, err := m.GetMachineIPFromStatus()
	if err != nil {
		return err
	}
	loadbalancerId := m.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId
	m.Logger.Info("Private IP of the instance", "private-ip", instanceIp)
	m.Logger.Info("Control Plane load balancer", "id", loadbalancerId)

	// Check the load balancer type
	loadbalancerType := m.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerType
	// By default, the load balancer type is Network Load Balancer
	// Unless user specifies the load balancer type to be LBaaS
	if loadbalancerType == infrastructurev1beta2.LoadBalancerTypeLB {
		lb, err := m.LoadBalancerClient.GetLoadBalancer(ctx, loadbalancer.GetLoadBalancerRequest{
			LoadBalancerId: loadbalancerId,
		})
		if err != nil {
			return err
		}
		backendSet := lb.BackendSets[APIServerLBBackendSetName]
		// When creating a LB, there is no way to set the backend Name, default backend name is the instance IP and port
		// So we use default backend name instead of machine name
		backendName := instanceIp + ":" + strconv.Itoa(int(m.OCIClusterAccessor.GetControlPlaneEndpoint().Port))
		if !m.containsLBBackend(backendSet, backendName) {
			logger := m.Logger.WithValues("backend-set", *backendSet.Name)
			logger.Info("Checking work request status for create backend")
			// we always try to create the backend if it exists during a reconcile loop and wait for the work request
			// to complete. If there is a work request in progress, in the rare case, CAPOCI pod restarts during the
			// work request, the create backend call may throw an error which is ok, as reconcile will go into
			// an exponential backoff
			resp, err := m.LoadBalancerClient.CreateBackend(ctx, loadbalancer.CreateBackendRequest{
				LoadBalancerId: loadbalancerId,
				BackendSetName: backendSet.Name,
				CreateBackendDetails: loadbalancer.CreateBackendDetails{
					IpAddress: common.String(instanceIp),
					Port:      common.Int(int(m.OCIClusterAccessor.GetControlPlaneEndpoint().Port)),
				},
				OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", string(m.OCIMachine.UID)),
			})
			if err != nil {
				return err
			}
			m.OCIMachine.Status.CreateBackendWorkRequestId = *resp.OpcWorkRequestId
			logger.Info("Add instance to LB backend-set", "WorkRequestId", resp.OpcWorkRequestId)
			logger.Info("Waiting for LB work request to be complete")
			_, err = ociutil.AwaitLBWorkRequest(ctx, m.LoadBalancerClient, resp.OpcWorkRequestId)
			if err != nil {
				return err
			}
		}

	} else {
		lb, err := m.NetworkLoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: loadbalancerId,
		})
		if err != nil {
			return err
		}
		backendSet := lb.BackendSets[APIServerLBBackendSetName]
		if !m.containsNLBBackend(backendSet, m.Name()) {
			logger := m.Logger.WithValues("backend-set", *backendSet.Name)
			logger.Info("Checking work request status for create backend")
			// we always try to create the backend if it exists during a reconcile loop and wait for the work request
			// to complete. If there is a work request in progress, in the rare case, CAPOCI pod restarts during the
			// work request, the create backend call may throw an error which is ok, as reconcile will go into
			// an exponential backoff
			resp, err := m.NetworkLoadBalancerClient.CreateBackend(ctx, networkloadbalancer.CreateBackendRequest{
				NetworkLoadBalancerId: loadbalancerId,
				BackendSetName:        backendSet.Name,
				CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
					IpAddress: common.String(instanceIp),
					Port:      common.Int(int(m.OCIClusterAccessor.GetControlPlaneEndpoint().Port)),
					Name:      common.String(m.Name()),
				},
				OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", string(m.OCIMachine.UID)),
			})
			if err != nil {
				return err
			}
			m.OCIMachine.Status.CreateBackendWorkRequestId = *resp.OpcWorkRequestId
			logger.Info("Add instance to NLB backend-set", "WorkRequestId", resp.OpcWorkRequestId)
			logger.Info("Waiting for NLB work request to be complete")
			_, err = ociutil.AwaitNLBWorkRequest(ctx, m.NetworkLoadBalancerClient, resp.OpcWorkRequestId)
			if err != nil {
				return err
			}
			logger.Info("NLB Backend addition work request is complete")
		}
	}
	return nil
}

// ReconcileDeleteInstanceOnLB checks to make sure the instance is part of a backend set then deletes the backend
// on the NetworkLoadBalancer
//
// # It will await the Work Request completion before returning
//
// See https://docs.oracle.com/en-us/iaas/Content/NetworkLoadBalancer/BackendServers/backend_server_management.htm#BackendServerManagement
// for more info on Backend Server Management
func (m *MachineScope) ReconcileDeleteInstanceOnLB(ctx context.Context) error {
	loadbalancerId := m.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId
	// Check the load balancer type
	loadbalancerType := m.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerType
	if loadbalancerType == infrastructurev1beta2.LoadBalancerTypeLB {
		lb, err := m.LoadBalancerClient.GetLoadBalancer(ctx, loadbalancer.GetLoadBalancerRequest{
			LoadBalancerId: loadbalancerId,
		})
		if err != nil {
			if ociutil.IsNotFound(err) {
				m.Logger.Info("LB has been deleted", "lb", *loadbalancerId)
				return nil
			}
			return err
		}
		backendSet := lb.BackendSets[APIServerLBBackendSetName]
		// in case of delete from LB backend, if the instance does not have an IP, we consider
		// the instance to not have been added in first place and hence return nil
		if len(m.OCIMachine.Status.Addresses) <= 0 {
			m.Logger.Info("Instance does not have IP Address, hence ignoring LBaaS reconciliation on delete")
			return nil
		}
		instanceIp, err := m.GetMachineIPFromStatus()
		if err != nil {
			return err
		}
		backendName := instanceIp + ":" + strconv.Itoa(int(m.OCIClusterAccessor.GetControlPlaneEndpoint().Port))
		if m.containsLBBackend(backendSet, backendName) {
			logger := m.Logger.WithValues("backend-set", *backendSet.Name)
			// in OCI CLI, the colon in the backend name is replaced by %3A
			// replace the colon in the backend name by %3A to avoid the error in PCA
			escapedBackendName := url.QueryEscape(backendName)
			// we always try to delete the backend if it exists during a reconcile loop and wait for the work request
			// to complete. If there is a work request in progress, in the rare case, CAPOCI pod restarts during the
			// work request, the delete backend call may throw an error which is ok, as reconcile will go into
			// an exponential backoff
			resp, err := m.LoadBalancerClient.DeleteBackend(ctx, loadbalancer.DeleteBackendRequest{
				LoadBalancerId: loadbalancerId,
				BackendSetName: backendSet.Name,
				BackendName:    common.String(escapedBackendName),
			})
			if err != nil {
				logger.Error(err, "Delete instance from LB backend-set failed",
					"backendSetName", *backendSet.Name,
					"backendName", escapedBackendName,
				)
				return err
			}
			m.OCIMachine.Status.DeleteBackendWorkRequestId = *resp.OpcWorkRequestId
			logger.Info("Delete instance from LB backend-set", "WorkRequestId", resp.OpcWorkRequestId)
			logger.Info("Waiting for LB work request to be complete")
			_, err = ociutil.AwaitLBWorkRequest(ctx, m.LoadBalancerClient, resp.OpcWorkRequestId)
			if err != nil {
				return err
			}
			logger.Info("LB Backend addition work request is complete")
		}
	} else {
		lb, err := m.NetworkLoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: loadbalancerId,
		})
		if err != nil {
			if ociutil.IsNotFound(err) {
				m.Logger.Info("NLB has been deleted", "nlb", *loadbalancerId)
				return nil
			}
			return err
		}
		backendSet := lb.BackendSets[APIServerLBBackendSetName]
		if m.containsNLBBackend(backendSet, m.Name()) {
			logger := m.Logger.WithValues("backend-set", *backendSet.Name)
			// we always try to delete the backend if it exists during a reconcile loop and wait for the work request
			// to complete. If there is a work request in progress, in the rare case, CAPOCI pod restarts during the
			// work request, the delete backend call may throw an error which is ok, as reconcile will go into
			// an exponential backoff
			resp, err := m.NetworkLoadBalancerClient.DeleteBackend(ctx, networkloadbalancer.DeleteBackendRequest{
				NetworkLoadBalancerId: loadbalancerId,
				BackendSetName:        backendSet.Name,
				BackendName:           common.String(m.Name()),
			})
			if err != nil {
				return err
			}
			m.OCIMachine.Status.DeleteBackendWorkRequestId = *resp.OpcWorkRequestId
			logger.Info("Delete instance from LB backend-set", "WorkRequestId", resp.OpcWorkRequestId)
			logger.Info("Waiting for LB work request to be complete")
			_, err = ociutil.AwaitNLBWorkRequest(ctx, m.NetworkLoadBalancerClient, resp.OpcWorkRequestId)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MachineScope) containsNLBBackend(backendSet networkloadbalancer.BackendSet, backendName string) bool {
	for _, backend := range backendSet.Backends {
		if *backend.Name == backendName {
			m.Logger.Info("Instance present in the backend")
			return true
		}
	}
	return false
}

func (m *MachineScope) containsLBBackend(backendSet loadbalancer.BackendSet, backendName string) bool {
	for _, backend := range backendSet.Backends {
		if *backend.Name == backendName {
			m.Logger.Info("Instance present in the backend")
			return true
		}
	}
	return false
}

// IsControlPlane returns true if the machine is a control plane.
func (m *MachineScope) IsControlPlane() bool {
	return capiUtil.IsControlPlaneMachine(m.Machine)
}

func (m *MachineScope) getCompartmentId() string {
	if m.OCIMachine.Spec.CompartmentId != "" {
		return m.OCIMachine.Spec.CompartmentId
	}
	return m.OCIClusterAccessor.GetCompartmentId()
}

func (m *MachineScope) getGetControlPlaneMachineSubnet() *string {
	for _, subnet := range m.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets {
		if subnet.Role == infrastructurev1beta2.ControlPlaneRole {
			return subnet.ID
		}
	}
	return nil
}

func (m *MachineScope) getGetControlPlaneMachineNSGs() []string {
	nsgs := make([]string, 0)
	for _, nsg := range m.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List {
		if nsg.Role == infrastructurev1beta2.ControlPlaneRole {
			if nsg.ID != nil {
				nsgs = append(nsgs, *nsg.ID)
			}
		}
	}
	return nsgs
}

// getMachineSubnet iterates through the OCICluster Vcn subnets
// and returns the subnet ID if the name matches
func (m *MachineScope) getMachineSubnet(name string) (*string, error) {
	for _, subnet := range m.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets {
		if subnet.Name == name {
			return subnet.ID, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("Subnet with name %s not found for cluster %s", name, m.OCIClusterAccessor.GetName()))
}

func (m *MachineScope) getWorkerMachineSubnet() *string {
	for _, subnet := range m.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets {
		if subnet.Role == infrastructurev1beta2.WorkerRole {
			// if a subnet name is defined, use the correct subnet
			if m.OCIMachine.Spec.SubnetName != "" {
				if m.OCIMachine.Spec.SubnetName == subnet.Name {
					return subnet.ID
				}
			} else {
				return subnet.ID
			}
		}
	}
	return nil
}

func (m *MachineScope) getWorkerMachineNSGs() []string {
	if len(m.OCIMachine.Spec.NetworkDetails.NsgNames) > 0 {
		nsgs := make([]string, 0)
		for _, nsgName := range m.OCIMachine.Spec.NetworkDetails.NsgNames {
			for _, nsg := range m.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List {
				if nsg.Name == nsgName {
					if nsg.ID != nil {
						nsgs = append(nsgs, *nsg.ID)
					}
				}
			}
		}
		return nsgs
	} else {
		nsgs := make([]string, 0)
		for _, nsg := range m.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List {
			if nsg.Role == infrastructurev1beta2.WorkerRole {
				if nsg.ID != nil {
					nsgs = append(nsgs, *nsg.ID)
				}
			}
		}
		return nsgs
	}
}

func (m *MachineScope) getAgentConfig() *core.LaunchInstanceAgentConfigDetails {
	agentConfigSpec := m.OCIMachine.Spec.AgentConfig
	if agentConfigSpec != nil {
		agentConfig := &core.LaunchInstanceAgentConfigDetails{
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
				desiredState, _ := core.GetMappingInstanceAgentPluginConfigDetailsDesiredStateEnum(string(pluginConfigSpec.DesiredState))
				pluginConfigRequest.DesiredState = desiredState
				pluginConfigList[i] = pluginConfigRequest
			}
			agentConfig.PluginsConfig = pluginConfigList
		}
		return agentConfig
	}
	return nil
}

func (m *MachineScope) getLaunchOptions() *core.LaunchOptions {
	launcOptionsSpec := m.OCIMachine.Spec.LaunchOptions
	if launcOptionsSpec != nil {
		launchOptions := &core.LaunchOptions{
			IsConsistentVolumeNamingEnabled: launcOptionsSpec.IsConsistentVolumeNamingEnabled,
		}
		if launcOptionsSpec.BootVolumeType != "" {
			bootVolume, _ := core.GetMappingLaunchOptionsBootVolumeTypeEnum(string(launcOptionsSpec.BootVolumeType))
			launchOptions.BootVolumeType = bootVolume
		}
		if launcOptionsSpec.Firmware != "" {
			firmware, _ := core.GetMappingLaunchOptionsFirmwareEnum(string(launcOptionsSpec.Firmware))
			launchOptions.Firmware = firmware
		}
		if launcOptionsSpec.NetworkType != "" {
			networkType, _ := core.GetMappingLaunchOptionsNetworkTypeEnum(string(launcOptionsSpec.NetworkType))
			launchOptions.NetworkType = networkType
		}
		if launcOptionsSpec.RemoteDataVolumeType != "" {
			remoteVolumeType, _ := core.GetMappingLaunchOptionsRemoteDataVolumeTypeEnum(string(launcOptionsSpec.RemoteDataVolumeType))
			launchOptions.RemoteDataVolumeType = remoteVolumeType
		}
		return launchOptions
	}
	return nil
}

func (m *MachineScope) getInstanceOptions() *core.InstanceOptions {
	instanceOptionsSpec := m.OCIMachine.Spec.InstanceOptions
	if instanceOptionsSpec != nil {
		return &core.InstanceOptions{
			AreLegacyImdsEndpointsDisabled: instanceOptionsSpec.AreLegacyImdsEndpointsDisabled,
		}
	}
	return nil
}

func (m *MachineScope) getAvailabilityConfig() *core.LaunchInstanceAvailabilityConfigDetails {
	avalabilityConfigSpec := m.OCIMachine.Spec.AvailabilityConfig
	if avalabilityConfigSpec != nil {
		recoveryAction, _ := core.GetMappingLaunchInstanceAvailabilityConfigDetailsRecoveryActionEnum(string(avalabilityConfigSpec.RecoveryAction))
		return &core.LaunchInstanceAvailabilityConfigDetails{
			IsLiveMigrationPreferred: avalabilityConfigSpec.IsLiveMigrationPreferred,
			RecoveryAction:           recoveryAction,
		}
	}
	return nil
}

func (m *MachineScope) getPreemptibleInstanceConfig() *core.PreemptibleInstanceConfigDetails {
	preEmptibleInstanceConfigSpec := m.OCIMachine.Spec.PreemptibleInstanceConfig
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

func (m *MachineScope) getPlatformConfig() core.PlatformConfig {
	platformConfig := m.OCIMachine.Spec.PlatformConfig
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

func (m *MachineScope) getLaunchVolumeAttachments() []core.LaunchAttachVolumeDetails {
	volumeAttachmentsInSpec := m.OCIMachine.Spec.LaunchVolumeAttachment
	if len(volumeAttachmentsInSpec) < 0 {
		return nil
	}
	var volumes []core.LaunchAttachVolumeDetails

	for _, attachment := range volumeAttachmentsInSpec {
		if attachment.Type == infrastructurev1beta2.IscsiType {
			volumes = append(volumes, getIscsiVolumeAttachment(attachment.IscsiAttachment))
		}
	}
	return volumes
}

func getIscsiVolumeAttachment(attachment infrastructurev1beta2.LaunchIscsiVolumeAttachment) core.LaunchAttachVolumeDetails {
	volumeDetails := core.LaunchAttachIScsiVolumeDetails{
		Device:                       attachment.Device,
		DisplayName:                  attachment.DisplayName,
		IsShareable:                  attachment.IsShareable,
		IsReadOnly:                   attachment.IsReadOnly,
		VolumeId:                     attachment.VolumeId,
		UseChap:                      attachment.UseChap,
		IsAgentAutoIscsiLoginEnabled: attachment.IsAgentAutoIscsiLoginEnabled,
		EncryptionInTransitType:      getEncryptionType(attachment.EncryptionInTransitType),
		LaunchCreateVolumeDetails:    getLaunchCreateVolumeDetails(attachment.LaunchCreateVolumeFromAttributes),
	}
	return volumeDetails
}

func getLaunchCreateVolumeDetails(attributes infrastructurev1beta2.LaunchCreateVolumeFromAttributes) core.LaunchCreateVolumeFromAttributes {
	return core.LaunchCreateVolumeFromAttributes{
		SizeInGBs:     attributes.SizeInGBs,
		DisplayName:   attributes.DisplayName,
		CompartmentId: attributes.CompartmentId,
		KmsKeyId:      attributes.KmsKeyId,
		VpusPerGB:     attributes.VpusPerGB,
	}
}

func getEncryptionType(transitType infrastructurev1beta2.EncryptionInTransitTypeEnum) core.EncryptionInTransitTypeEnum {
	if transitType == "" {
		return ""
	}
	switch transitType {
	case infrastructurev1beta2.EncryptionInTransitTypeNone:
		return core.EncryptionInTransitTypeNone
	case infrastructurev1beta2.EncryptionInTransitTypeBmEncryptionInTransit:
		return core.EncryptionInTransitTypeBmEncryptionInTransit
	}
	return ""
}
