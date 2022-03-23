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
	"encoding/base64"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"

	"github.com/go-logr/logr"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute"
	nlb "github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/oracle/oci-go-sdk/v63/networkloadbalancer"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiUtil "sigs.k8s.io/cluster-api/util"
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
	OCICluster                *infrastructurev1beta1.OCICluster
	OCIMachine                *infrastructurev1beta1.OCIMachine
	VCNClient                 vcn.Client
	NetworkLoadBalancerClient nlb.NetworkLoadBalancerClient
}

type MachineScope struct {
	*logr.Logger
	Client                    client.Client
	patchHelper               *patch.Helper
	Cluster                   *clusterv1.Cluster
	Machine                   *clusterv1.Machine
	ComputeClient             compute.ComputeClient
	OCICluster                *infrastructurev1beta1.OCICluster
	OCIMachine                *infrastructurev1beta1.OCIMachine
	VCNClient                 vcn.Client
	NetworkLoadBalancerClient nlb.NetworkLoadBalancerClient
}

// NewMachineScope creates a MachineScope given the MachineScopeParams
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Machine == nil {
		return nil, errors.New("failed to generate new scope from nil Machine")
	}
	if params.OCICluster == nil {
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
		OCICluster:                params.OCICluster,
		patchHelper:               helper,
		Machine:                   params.Machine,
		OCIMachine:                params.OCIMachine,
		VCNClient:                 params.VCNClient,
		NetworkLoadBalancerClient: params.NetworkLoadBalancerClient,
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
	if (m.OCIMachine.Spec.ShapeConfig != infrastructurev1beta1.ShapeConfig{}) {
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

	subnetId := m.OCIMachine.Spec.NetworkDetails.SubnetId
	if subnetId == nil {
		if m.IsControlPlane() {
			subnetId = m.getGetControlPlaneMachineSubnet()
		} else {
			subnetId = m.getWorkerMachineSubnet()
		}
	}

	nsgId := m.OCIMachine.Spec.NetworkDetails.NSGId
	if nsgId == nil {
		if m.IsControlPlane() {
			nsgId = m.getGetControlPlaneMachineNSG()
		} else {
			nsgId = m.getWorkerMachineNSG()
		}
	}

	failureDomain := m.Machine.Spec.FailureDomain
	if failureDomain == nil {
		m.Logger.Info("Failure Domain is not set in the machine spec, setting it to a random value from 1 to 3")
		rand.Seed(time.Now().UnixNano())
		// rand.Intn(3) will produce a random number from 0(inclusive) to 3(exclusive)
		// ee add one to get a number from 1 to 3
		failureDomain = common.String(strconv.Itoa(rand.Intn(3) + 1))
	}
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
	metadata := m.OCIMachine.Spec.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["user_data"] = base64.StdEncoding.EncodeToString([]byte(cloudInitData))

	tags := m.getFreeFormTags(*m.OCICluster)

	definedTags := make(map[string]map[string]interface{})
	if m.OCIMachine.Spec.DefinedTags != nil {
		for ns, mapNs := range m.OCIMachine.Spec.DefinedTags {
			mapValues := make(map[string]interface{})
			for k, v := range mapNs {
				mapValues[k] = v
			}
			definedTags[ns] = mapValues
		}
	}

	availabilityDomain := m.OCICluster.Status.FailureDomains[*failureDomain].Attributes[AvailabilityDomain]
	faultDomain := m.OCICluster.Status.FailureDomains[*failureDomain].Attributes[FaultDomain]
	launchDetails := core.LaunchInstanceDetails{DisplayName: common.String(m.OCIMachine.Name),
		SourceDetails: sourceDetails,
		CreateVnicDetails: &core.CreateVnicDetails{
			SubnetId:       subnetId,
			AssignPublicIp: common.Bool(m.OCIMachine.Spec.NetworkDetails.AssignPublicIp),
		},
		Metadata:                       metadata,
		Shape:                          common.String(m.OCIMachine.Spec.Shape),
		AvailabilityDomain:             common.String(availabilityDomain),
		CompartmentId:                  common.String(m.getCompartmentId()),
		IsPvEncryptionInTransitEnabled: common.Bool(m.OCIMachine.Spec.IsPvEncryptionInTransitEnabled),
		FreeformTags:                   tags,
		DefinedTags:                    definedTags,
	}
	// Compute API does not behave well if the shape config is empty for fixed shapes
	// hence set it only if it non empty
	if (shapeConfig != core.LaunchInstanceShapeConfigDetails{}) {
		launchDetails.ShapeConfig = &shapeConfig
	}
	if faultDomain != "" {
		launchDetails.FaultDomain = common.String(faultDomain)
	}
	if nsgId != nil {
		launchDetails.CreateVnicDetails.NsgIds = []string{*nsgId}
	}
	req := core.LaunchInstanceRequest{LaunchInstanceDetails: launchDetails,
		OpcRetryToken: ociutil.GetOPCRetryToken(string(m.OCIMachine.UID))}
	resp, err := m.ComputeClient.LaunchInstance(ctx, req)
	if err != nil {
		return nil, err
	} else {
		return &resp.Instance, nil
	}
}

func (m *MachineScope) getFreeFormTags(ociCluster infrastructurev1beta1.OCICluster) map[string]string {
	tags := ociutil.BuildClusterTags(string(ociCluster.UID))
	// first use cluster level tags, then override with machine level tags
	if ociCluster.Spec.FreeformTags != nil {
		for k, v := range ociCluster.Spec.FreeformTags {
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
func (m *MachineScope) DeleteMachine(ctx context.Context) error {
	req := core.TerminateInstanceRequest{InstanceId: m.OCIMachine.Spec.InstanceId,
		PreserveBootVolume: common.Bool(false)}
	_, err := m.ComputeClient.TerminateInstance(ctx, req)
	return err
}

// IsResourceCreatedByClusterAPI determines if the instance was created by the cluster using the
// tags created at instance launch.
func (s *MachineScope) IsResourceCreatedByClusterAPI(resourceFreeFormTags map[string]string) bool {
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(string(s.OCICluster.UID))
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
	req := core.ListInstancesRequest{DisplayName: common.String(name),
		CompartmentId: common.String(m.getCompartmentId())}
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
	return nil, nil
}

// PatchObject persists the cluster configuration and status.
func (m *MachineScope) PatchObject(ctx context.Context) error {
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

		if page = resp.OpcNextPage; resp.OpcNextPage == nil {
			break
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
	loadbalancerId := m.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId
	m.Logger.Info("Private IP of the instance", "private-ip", instanceIp)
	m.Logger.Info("Control Plane load balancer", "id", loadbalancerId)
	lb, err := m.NetworkLoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: loadbalancerId,
	})
	if err != nil {
		return err
	}
	backendSet := lb.BackendSets[APIServerLBBackendSetName]
	if !m.containsBackend(backendSet, m.Name()) {
		logger := m.Logger.WithValues("backend-set", *backendSet.Name)
		logger.Info("Checking work request status for create backend")
		workRequest := m.OCIMachine.Status.CreateBackendWorkRequestId
		if workRequest != "" {
			_, err = ociutil.AwaitLBWorkRequest(ctx, m.NetworkLoadBalancerClient, &workRequest)
			if err != nil {
				return err
			}
		} else {
			resp, err := m.NetworkLoadBalancerClient.CreateBackend(ctx, networkloadbalancer.CreateBackendRequest{
				NetworkLoadBalancerId: loadbalancerId,
				BackendSetName:        backendSet.Name,
				CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
					IpAddress: common.String(instanceIp),
					Port:      common.Int(int(m.OCICluster.Spec.ControlPlaneEndpoint.Port)),
					Name:      common.String(m.Name()),
				},
				OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", string(m.OCIMachine.UID)),
			})
			if err != nil {
				return err
			}
			m.OCIMachine.Status.CreateBackendWorkRequestId = *resp.OpcWorkRequestId
			logger.Info("Add instance to LB backend-set", "WorkRequestId", resp.OpcWorkRequestId)
			logger.Info("Waiting for LB work request to be complete")
			_, err = ociutil.AwaitLBWorkRequest(ctx, m.NetworkLoadBalancerClient, resp.OpcWorkRequestId)
			if err != nil {
				return err
			}
			logger.Info("LB Backend addition work request is complete")
		}
	}
	return nil
}

// ReconcileDeleteInstanceOnLB checks to make sure the instance is part of a backend set then deletes the backend
// on the NetworkLoadBalancer
//
// It will await the Work Request completion before returning
//
// See https://docs.oracle.com/en-us/iaas/Content/NetworkLoadBalancer/BackendServers/backend_server_management.htm#BackendServerManagement
// for more info on Backend Server Management
func (m *MachineScope) ReconcileDeleteInstanceOnLB(ctx context.Context) error {
	loadbalancerId := m.OCICluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId
	lb, err := m.NetworkLoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: loadbalancerId,
	})
	if err != nil {
		return err
	}
	backendSet := lb.BackendSets[APIServerLBBackendSetName]
	if m.containsBackend(backendSet, m.Name()) {
		logger := m.Logger.WithValues("backend-set", *backendSet.Name)
		workRequest := m.OCIMachine.Status.DeleteBackendWorkRequestId
		if workRequest != "" {
			_, err = ociutil.AwaitLBWorkRequest(ctx, m.NetworkLoadBalancerClient, &workRequest)
			if err != nil {
				return err
			}
		} else {
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
			_, err = ociutil.AwaitLBWorkRequest(ctx, m.NetworkLoadBalancerClient, resp.OpcWorkRequestId)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MachineScope) containsBackend(backendSet networkloadbalancer.BackendSet, backendName string) bool {
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
	return m.OCICluster.Spec.CompartmentId
}

func (m *MachineScope) getGetControlPlaneMachineSubnet() *string {
	for _, subnet := range m.OCICluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == infrastructurev1beta1.ControlPlaneRole {
			return subnet.ID
		}
	}
	return nil
}

func (m *MachineScope) getGetControlPlaneMachineNSG() *string {
	for _, nsg := range m.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
		if nsg.Role == infrastructurev1beta1.ControlPlaneRole {
			return nsg.ID
		}
	}
	return nil
}

func (m *MachineScope) getWorkerMachineSubnet() *string {
	for _, subnet := range m.OCICluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == infrastructurev1beta1.WorkerRole {
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

func (m *MachineScope) getWorkerMachineNSG() *string {
	for _, nsg := range m.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
		if nsg.Role == infrastructurev1beta1.WorkerRole {
			// if an NSG name is defined, use the correct NSG
			if m.OCIMachine.Spec.NSGName != "" {
				if m.OCIMachine.Spec.NSGName == nsg.Name {
					return nsg.ID
				}
			} else {
				return nsg.ID
			}
		}
	}
	return nil
}
