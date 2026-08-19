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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"

	"github.com/go-logr/logr"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/hash"
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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OCIMachinePoolKind                  = "OCIMachinePool"
	InstanceConfigurationHashAnnotation = "oci.oraclecloud.com/instance-configuration-hash"
	BootstrapDataHashAnnotation         = "oci.oraclecloud.com/bootstrap-data-hash"
	InstancePoolUpdateAttemptAnnotation = "oci.oraclecloud.com/instance-pool-update-attempt"

	instancePoolUpdateAttemptSchemaVersion = 1
	instancePoolUpdateAttemptTimeout       = time.Hour
)

type instancePoolUpdatePhase string

const (
	instancePoolUpdatePhasePrepared  instancePoolUpdatePhase = "Prepared"
	instancePoolUpdatePhaseSubmitted instancePoolUpdatePhase = "Submitted"
)

// InstancePoolUpdateOutcome describes the transition made while reconciling
// one durable asynchronous OCI instance-pool update.
type InstancePoolUpdateOutcome string

const (
	InstancePoolUpdateNoChange      InstancePoolUpdateOutcome = "NoChange"
	InstancePoolUpdateSubmitted     InstancePoolUpdateOutcome = "Submitted"
	InstancePoolUpdateWaiting       InstancePoolUpdateOutcome = "Waiting"
	InstancePoolUpdateConverged     InstancePoolUpdateOutcome = "Converged"
	InstancePoolUpdateRetryRequired InstancePoolUpdateOutcome = "RetryRequired"
)

// MachinePoolScopeParams defines the params need to create a new MachineScope
type MachinePoolScopeParams struct {
	Logger                  *logr.Logger
	Cluster                 *clusterv1.Cluster
	MachinePool             *clusterv1.MachinePool
	Client                  client.Client
	ComputeManagementClient computemanagement.Client
	InstancePoolETag        *string
	OCIClusterAccessor      OCIClusterAccessor
	OCIMachinePool          *expinfra1.OCIMachinePool
	Recorder                record.EventRecorder
}

type MachinePoolScope struct {
	*logr.Logger
	Client                  client.Client
	patchHelper             *v1beta1patch.Helper
	Cluster                 *clusterv1.Cluster
	MachinePool             *clusterv1.MachinePool
	ComputeManagementClient computemanagement.Client
	InstancePoolETag        *string
	OCIClusterAccesor       OCIClusterAccessor
	OCIMachinePool          *expinfra1.OCIMachinePool
	Recorder                record.EventRecorder
}

// instancePoolUpdateAttempt is the durable identity of one logical OCI update.
// The complete canonical target is persisted so a changed desired state cannot
// replace an operation whose completion has not yet been observed.
type instancePoolUpdateAttempt struct {
	Version     int                      `json:"version"`
	Target      instancePoolUpdateTarget `json:"target"`
	Fingerprint string                   `json:"fingerprint"`
	RetryToken  string                   `json:"retryToken"`
	Phase       instancePoolUpdatePhase  `json:"phase"`
	StartedAt   time.Time                `json:"startedAt"`
	IfMatch     string                   `json:"ifMatch,omitempty"`
}

type instancePoolUpdateTarget struct {
	Size                         *int                                `json:"size,omitempty"`
	InstanceConfigurationID      *string                             `json:"instanceConfigurationId,omitempty"`
	FreeformTags                 map[string]string                   `json:"freeformTags"`
	PlacementConfigurations      []instancePoolUpdatePlacementTarget `json:"placementConfigurations,omitempty"`
	InstanceDisplayNameFormatter *string                             `json:"instanceDisplayNameFormatter,omitempty"`
	InstanceHostnameFormatter    *string                             `json:"instanceHostnameFormatter,omitempty"`
}

type instancePoolUpdatePlacementTarget struct {
	AvailabilityDomain *string                                    `json:"availabilityDomain,omitempty"`
	FaultDomains       []string                                   `json:"faultDomains,omitempty"`
	PrimarySubnetID    *string                                    `json:"primarySubnetId,omitempty"`
	PrimaryVNICSubnet  *instancePoolUpdatePrimaryVNICSubnetTarget `json:"primaryVnicSubnet,omitempty"`
}

type instancePoolUpdatePrimaryVNICSubnetTarget struct {
	SubnetID       *string `json:"subnetId,omitempty"`
	IsAssignIPv6IP *bool   `json:"isAssignIpv6Ip,omitempty"`
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
	helper, err := v1beta1patch.NewHelper(params.OCIMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	params.OCIMachinePool.Status.InfrastructureMachineKind = "OCIMachinePoolMachine"
	return &MachinePoolScope{
		Logger:                  params.Logger,
		Client:                  params.Client,
		ComputeManagementClient: params.ComputeManagementClient,
		InstancePoolETag:        params.InstancePoolETag,
		Cluster:                 params.Cluster,
		OCIClusterAccesor:       params.OCIClusterAccessor,
		patchHelper:             helper,
		MachinePool:             params.MachinePool,
		OCIMachinePool:          params.OCIMachinePool,
		Recorder:                params.Recorder,
	}, nil
}

func (m *MachinePoolScope) getInstanceConfigurationHashAnnotation() string {
	if m.OCIMachinePool.Annotations == nil {
		return ""
	}
	return m.OCIMachinePool.Annotations[InstanceConfigurationHashAnnotation]
}

func (m *MachinePoolScope) setInstanceConfigurationHashAnnotation(h string) {
	if m.OCIMachinePool.Annotations == nil {
		m.OCIMachinePool.Annotations = map[string]string{}
	}
	m.OCIMachinePool.Annotations[InstanceConfigurationHashAnnotation] = h
}

func (m *MachinePoolScope) getBootstrapDataHashAnnotation() string {
	if m.OCIMachinePool.Annotations == nil {
		return ""
	}
	return m.OCIMachinePool.Annotations[BootstrapDataHashAnnotation]
}

func (m *MachinePoolScope) setBootstrapDataHashAnnotation(h string) {
	if m.OCIMachinePool.Annotations == nil {
		m.OCIMachinePool.Annotations = map[string]string{}
	}
	m.OCIMachinePool.Annotations[BootstrapDataHashAnnotation] = h
}

func newInstancePoolUpdateAttempt(details core.UpdateInstancePoolDetails, ifMatch *string) (*instancePoolUpdateAttempt, error) {
	target := newInstancePoolUpdateTarget(details)
	fingerprint, err := target.fingerprint()
	if err != nil {
		return nil, err
	}
	return &instancePoolUpdateAttempt{
		Version:     instancePoolUpdateAttemptSchemaVersion,
		Target:      target,
		Fingerprint: fingerprint,
		RetryToken:  ptr.ToString(ociutil.GetOPCRetryToken("update-instance-pool-%s", string(uuid.NewUUID()))),
		Phase:       instancePoolUpdatePhasePrepared,
		StartedAt:   time.Now().UTC(),
		IfMatch:     ptr.ToString(ifMatch),
	}, nil
}

func newInstancePoolUpdateTarget(details core.UpdateInstancePoolDetails) instancePoolUpdateTarget {
	target := instancePoolUpdateTarget{
		Size:                         details.Size,
		InstanceConfigurationID:      details.InstanceConfigurationId,
		FreeformTags:                 cloneStringMap(details.FreeformTags),
		InstanceDisplayNameFormatter: details.InstanceDisplayNameFormatter,
		InstanceHostnameFormatter:    details.InstanceHostnameFormatter,
	}
	if details.PlacementConfigurations != nil {
		target.PlacementConfigurations = make([]instancePoolUpdatePlacementTarget, 0, len(details.PlacementConfigurations))
		for _, placement := range details.PlacementConfigurations {
			targetPlacement := instancePoolUpdatePlacementTarget{
				AvailabilityDomain: placement.AvailabilityDomain,
				FaultDomains:       sortedStrings(placement.FaultDomains),
				PrimarySubnetID:    placement.PrimarySubnetId,
			}
			if placement.PrimaryVnicSubnets != nil {
				targetPlacement.PrimaryVNICSubnet = &instancePoolUpdatePrimaryVNICSubnetTarget{
					SubnetID:       placement.PrimaryVnicSubnets.SubnetId,
					IsAssignIPv6IP: placement.PrimaryVnicSubnets.IsAssignIpv6Ip,
				}
			}
			target.PlacementConfigurations = append(target.PlacementConfigurations, targetPlacement)
		}
		sort.SliceStable(target.PlacementConfigurations, func(i, j int) bool {
			return ptr.ToString(target.PlacementConfigurations[i].AvailabilityDomain) < ptr.ToString(target.PlacementConfigurations[j].AvailabilityDomain)
		})
	}
	return target
}

func (t instancePoolUpdateTarget) updateDetails() core.UpdateInstancePoolDetails {
	details := core.UpdateInstancePoolDetails{
		Size:                         t.Size,
		InstanceConfigurationId:      t.InstanceConfigurationID,
		FreeformTags:                 cloneStringMap(t.FreeformTags),
		InstanceDisplayNameFormatter: t.InstanceDisplayNameFormatter,
		InstanceHostnameFormatter:    t.InstanceHostnameFormatter,
	}
	if t.PlacementConfigurations != nil {
		details.PlacementConfigurations = make([]core.UpdateInstancePoolPlacementConfigurationDetails, 0, len(t.PlacementConfigurations))
		for _, placement := range t.PlacementConfigurations {
			updatePlacement := core.UpdateInstancePoolPlacementConfigurationDetails{
				AvailabilityDomain: placement.AvailabilityDomain,
				FaultDomains:       append([]string(nil), placement.FaultDomains...),
				PrimarySubnetId:    placement.PrimarySubnetID,
			}
			if placement.PrimaryVNICSubnet != nil {
				updatePlacement.PrimaryVnicSubnets = &core.InstancePoolPlacementPrimarySubnet{
					SubnetId:       placement.PrimaryVNICSubnet.SubnetID,
					IsAssignIpv6Ip: placement.PrimaryVNICSubnet.IsAssignIPv6IP,
				}
			}
			details.PlacementConfigurations = append(details.PlacementConfigurations, updatePlacement)
		}
	}
	return details
}

func (t instancePoolUpdateTarget) fingerprint() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (t instancePoolUpdateTarget) matches(instancePool *core.InstancePool) bool {
	if instancePool == nil {
		return false
	}
	if t.Size != nil && !intPointersEqual(t.Size, instancePool.Size) {
		return false
	}
	if t.InstanceConfigurationID != nil && !ptr.StringEqual(t.InstanceConfigurationID, instancePool.InstanceConfigurationId) {
		return false
	}
	if !stringMapsEqual(t.FreeformTags, instancePool.FreeformTags) {
		return false
	}
	if t.PlacementConfigurations != nil && instancePoolPlacementNeedsUpdate(instancePool.PlacementConfigurations, t.updateDetails().PlacementConfigurations) {
		return false
	}
	if t.InstanceDisplayNameFormatter != nil && !instancePoolFormattersEqual(t.InstanceDisplayNameFormatter, instancePool.InstanceDisplayNameFormatter) {
		return false
	}
	if t.InstanceHostnameFormatter != nil && !instancePoolFormattersEqual(t.InstanceHostnameFormatter, instancePool.InstanceHostnameFormatter) {
		return false
	}
	return true
}

func (m *MachinePoolScope) getInstancePoolUpdateAttempt() (*instancePoolUpdateAttempt, error) {
	if m.OCIMachinePool.Annotations == nil || m.OCIMachinePool.Annotations[InstancePoolUpdateAttemptAnnotation] == "" {
		return nil, nil
	}
	attempt := &instancePoolUpdateAttempt{}
	if err := json.Unmarshal([]byte(m.OCIMachinePool.Annotations[InstancePoolUpdateAttemptAnnotation]), attempt); err != nil {
		return nil, errors.Wrap(err, "decode instance pool update attempt")
	}
	if attempt.Version != instancePoolUpdateAttemptSchemaVersion {
		return nil, errors.Errorf("unsupported instance pool update attempt schema version %d; verify the OCI operation is terminal before removing annotation %q", attempt.Version, InstancePoolUpdateAttemptAnnotation)
	}
	if attempt.RetryToken == "" || attempt.StartedAt.IsZero() {
		return nil, errors.New("invalid instance pool update attempt")
	}
	if attempt.Phase != instancePoolUpdatePhasePrepared && attempt.Phase != instancePoolUpdatePhaseSubmitted {
		return nil, errors.Errorf("invalid instance pool update phase %q", attempt.Phase)
	}
	fingerprint, err := attempt.Target.fingerprint()
	if err != nil {
		return nil, errors.Wrap(err, "fingerprint instance pool update attempt")
	}
	if fingerprint != attempt.Fingerprint {
		return nil, errors.New("instance pool update attempt fingerprint does not match its target")
	}
	return attempt, nil
}

func (m *MachinePoolScope) setInstancePoolUpdateAttempt(ctx context.Context, attempt *instancePoolUpdateAttempt) error {
	data, err := json.Marshal(attempt)
	if err != nil {
		return errors.Wrap(err, "encode instance pool update attempt")
	}
	expected := ""
	if m.OCIMachinePool.Annotations != nil {
		expected = m.OCIMachinePool.Annotations[InstancePoolUpdateAttemptAnnotation]
	}
	if err := m.transitionInstancePoolUpdateAttempt(ctx, expected, string(data)); err != nil {
		return errors.Wrap(err, "persist instance pool update attempt")
	}
	return nil
}

func (m *MachinePoolScope) clearInstancePoolUpdateAttempt(ctx context.Context) error {
	if !m.HasPendingInstancePoolUpdate() {
		return nil
	}
	expected := m.OCIMachinePool.Annotations[InstancePoolUpdateAttemptAnnotation]
	if err := m.transitionInstancePoolUpdateAttempt(ctx, expected, ""); err != nil {
		return errors.Wrap(err, "clear completed instance pool update attempt")
	}
	return nil
}

// transitionInstancePoolUpdateAttempt changes the durable attempt only when it
// still has the value observed by this reconciliation. The resource-version
// precondition prevents overlapping reconcilers from replacing or clearing a
// newer attempt.
func (m *MachinePoolScope) transitionInstancePoolUpdateAttempt(ctx context.Context, expected, desired string) error {
	// Persist changes already staged on the scope before rebasing its patch
	// helper around the independently guarded annotation transition.
	if err := m.PatchObject(ctx); err != nil {
		return errors.Wrap(err, "persist OCIMachinePool before update-attempt transition")
	}

	latest := &infrav2exp.OCIMachinePool{}
	if err := m.Client.Get(ctx, client.ObjectKeyFromObject(m.OCIMachinePool), latest); err != nil {
		return errors.Wrap(err, "get latest OCIMachinePool update attempt")
	}
	actual := ""
	if latest.Annotations != nil {
		actual = latest.Annotations[InstancePoolUpdateAttemptAnnotation]
	}
	if actual != expected {
		return errors.New("instance pool update attempt changed concurrently")
	}

	before := latest.DeepCopy()
	if desired == "" {
		delete(latest.Annotations, InstancePoolUpdateAttemptAnnotation)
	} else {
		if latest.Annotations == nil {
			latest.Annotations = map[string]string{}
		}
		latest.Annotations[InstancePoolUpdateAttemptAnnotation] = desired
	}
	patch := client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})
	if err := m.Client.Patch(ctx, latest, patch); err != nil {
		return errors.Wrap(err, "patch OCIMachinePool update attempt")
	}

	// Keep the scope object aligned with the successful transition. Carrying
	// forward the resource version also prevents the scope's final patch from
	// overwriting an attempt written by a later reconciliation.
	m.OCIMachinePool.SetResourceVersion(latest.GetResourceVersion())
	if desired == "" {
		delete(m.OCIMachinePool.Annotations, InstancePoolUpdateAttemptAnnotation)
	} else {
		if m.OCIMachinePool.Annotations == nil {
			m.OCIMachinePool.Annotations = map[string]string{}
		}
		m.OCIMachinePool.Annotations[InstancePoolUpdateAttemptAnnotation] = desired
	}
	helper, err := v1beta1patch.NewHelper(m.OCIMachinePool, m.Client)
	if err != nil {
		return errors.Wrap(err, "rebase OCIMachinePool patch helper after update-attempt transition")
	}
	m.patchHelper = helper
	return nil
}

func (m *MachinePoolScope) submitInstancePoolUpdateAttempt(ctx context.Context, instancePool *core.InstancePool, attempt *instancePoolUpdateAttempt) (InstancePoolUpdateOutcome, error) {
	request := core.UpdateInstancePoolRequest{
		InstancePoolId:            instancePool.Id,
		UpdateInstancePoolDetails: attempt.Target.updateDetails(),
		OpcRetryToken:             common.String(attempt.RetryToken),
	}
	if attempt.IfMatch != "" {
		request.IfMatch = common.String(attempt.IfMatch)
	}

	_, err := m.ComputeManagementClient.UpdateInstancePool(ctx, request)
	if err != nil {
		if serviceErr, ok := common.IsServiceError(err); ok {
			switch status := serviceErr.GetHTTPStatusCode(); {
			case status == http.StatusPreconditionFailed:
				// OCI rejected the stale If-Match value, so this request was not
				// accepted. Discard its token and rebuild from fresh readback.
				if clearErr := m.clearInstancePoolUpdateAttempt(ctx); clearErr != nil {
					return InstancePoolUpdateNoChange, clearErr
				}
				m.Info("Instance pool update rejected because its ETag was stale; retrying from fresh readback")
				return InstancePoolUpdateRetryRequired, nil
			case status >= http.StatusBadRequest && status < http.StatusInternalServerError &&
				status != http.StatusRequestTimeout && status != http.StatusConflict && status != http.StatusTooManyRequests:
				if clearErr := m.clearInstancePoolUpdateAttempt(ctx); clearErr != nil {
					return InstancePoolUpdateNoChange, clearErr
				}
			}
		}
		return InstancePoolUpdateNoChange, errors.Wrap(err, "unable to update instance pool")
	}

	attempt.Phase = instancePoolUpdatePhaseSubmitted
	attempt.StartedAt = time.Now().UTC()
	if err := m.setInstancePoolUpdateAttempt(ctx, attempt); err != nil {
		return InstancePoolUpdateNoChange, err
	}
	m.Info("Successfully submitted instance pool update", "fingerprint", attempt.Fingerprint)
	return InstancePoolUpdateSubmitted, nil
}

// HasPendingInstancePoolUpdate reports whether an OCI update has been persisted
// but has not yet been proven complete by fresh readback.
func (m *MachinePoolScope) HasPendingInstancePoolUpdate() bool {
	return m.OCIMachinePool.Annotations != nil && m.OCIMachinePool.Annotations[InstancePoolUpdateAttemptAnnotation] != ""
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stringMapsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		rightValue, ok := right[key]
		if !ok || rightValue != value {
			return false
		}
	}
	return true
}

func intPointersEqual(left, right *int) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func sortedStrings(values []string) []string {
	if values == nil {
		return nil
	}
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
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

// SyncReplicasFromInstancePool updates the owner MachinePool spec to match the
// observed OCI instance pool size when replicas are externally managed.
func (m *MachinePoolScope) SyncReplicasFromInstancePool(ctx context.Context, instancePool *core.InstancePool) error {
	if !annotations.ReplicasManagedByExternalAutoscaler(m.MachinePool) {
		return nil
	}
	if instancePool == nil || instancePool.Size == nil {
		m.Info("Synced MachinePool instancePool or size is nil.")
		return nil
	}

	observedReplicas := int32(*instancePool.Size)
	if m.MachinePool.Spec.Replicas != nil && *m.MachinePool.Spec.Replicas == observedReplicas {
		return nil
	}

	helper, err := v1beta1patch.NewHelper(m.MachinePool, m.Client)
	if err != nil {
		return errors.Wrap(err, "failed to init machinepool patch helper")
	}

	m.MachinePool.Spec.Replicas = pointer.Int32(observedReplicas)
	if err := helper.Patch(ctx, m.MachinePool); err != nil {
		return errors.Wrap(err, "failed to patch machinepool replicas from observed instance pool size")
	}

	m.Info("Synced MachinePool replicas from observed instance pool size", "replicas", observedReplicas)
	return nil
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
		shapeConfig.Vcpus = shapeConfigSpec.Vcpus
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
				faultDomains := ad.FaultDomains
				if len(specPlacment.FaultDomains) > 0 {
					faultDomains = specPlacment.FaultDomains
				}
				placement := core.CreateInstancePoolPlacementConfigurationDetails{
					AvailabilityDomain: common.String(ad.Name),
					PrimarySubnetId:    s.GetWorkerMachineSubnet(),
					FaultDomains:       faultDomains,
				}
				if specPlacment.PrimaryVnicSubnets != nil {
					if primaryVnicSubnets := buildPrimaryVnicSubnets(specPlacment.PrimaryVnicSubnets, s.GetWorkerMachineSubnet()); primaryVnicSubnets != nil {
						placement.PrimarySubnetId = nil
						placement.PrimaryVnicSubnets = primaryVnicSubnets
					}
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
	sort.SliceStable(placements, func(i, j int) bool {
		return ptr.ToString(placements[i].AvailabilityDomain) < ptr.ToString(placements[j].AvailabilityDomain)
	})

	return placements, nil
}

func (s *MachinePoolScope) buildUpdateInstancePoolPlacement() ([]core.UpdateInstancePoolPlacementConfigurationDetails, error) {
	createPlacements, err := s.BuildInstancePoolPlacement()
	if err != nil {
		return nil, err
	}

	updatePlacements := make([]core.UpdateInstancePoolPlacementConfigurationDetails, 0, len(createPlacements))
	for _, placement := range createPlacements {
		updatePlacements = append(updatePlacements, core.UpdateInstancePoolPlacementConfigurationDetails{
			AvailabilityDomain: placement.AvailabilityDomain,
			FaultDomains:       placement.FaultDomains,
			PrimarySubnetId:    placement.PrimarySubnetId,
			PrimaryVnicSubnets: placement.PrimaryVnicSubnets,
		})
	}
	return updatePlacements, nil
}

func buildPrimaryVnicSubnets(spec *infrav2exp.InstancePoolPlacementPrimarySubnet, defaultSubnetID *string) *core.InstancePoolPlacementPrimarySubnet {
	if spec == nil {
		return nil
	}
	subnetID := spec.SubnetId
	if subnetID == nil && defaultSubnetID != nil {
		subnetID = defaultSubnetID
	}
	if subnetID == nil {
		return nil
	}
	return &core.InstancePoolPlacementPrimarySubnet{
		SubnetId:       subnetID,
		IsAssignIpv6Ip: spec.IsAssignIpv6Ip,
	}
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
	tags := make(map[string]string)
	for k, v := range m.OCIClusterAccesor.GetFreeformTags() {
		tags[k] = v
	}
	for k, v := range m.OCIMachinePool.Spec.InstanceConfiguration.FreeformTags {
		tags[k] = v
	}
	// Ownership tags must be applied last. Resource discovery and cleanup depend
	// on these values matching the controller-owned cluster identifiers, so users
	// cannot override them through cluster or machine pool freeform tags.
	for k, v := range ociutil.BuildClusterTags(m.OCIClusterAccesor.GetOCIResourceIdentifier()) {
		tags[k] = v
	}

	return tags
}

// ReconcileInstanceConfiguration reconciles the InstanceConfiguration resource.
//
// Infrastructure config (shape, image, networking, etc.) and bootstrap data
// (user_data from the bootstrap secret) are tracked as separate change signals.
// This avoids conflating OCI-returned field defaults with real bootstrap changes.
func (m *MachinePoolScope) ReconcileInstanceConfiguration(ctx context.Context, _ *core.InstancePool) error {
	// Fetch existing IC
	instanceConfiguration, err := m.GetInstanceConfiguration(ctx)
	if err != nil {
		return err
	}

	// Build desired launch details (includes everything: config + user_data)
	freeFormTags := m.GetFreeFormTags()
	definedTags := m.getDefinedTags()

	instanceConfigurationSpec := m.OCIMachinePool.Spec.InstanceConfiguration
	desiredLaunch, err := m.getLaunchInstanceDetails(instanceConfigurationSpec, freeFormTags, definedTags)
	if err != nil {
		return err
	}

	// Compute config hash (excludes user_data — tracked separately)
	desiredConfigHash, err := hash.ComputeHash(desiredLaunch)
	if err != nil {
		return errors.Wrap(err, "compute desired instance config hash")
	}

	// Compute bootstrap data hash (user_data only)
	desiredBootstrapHash := hash.ComputeUserDataHash(desiredLaunch.Metadata)

	m.Logger.V(1).Info("InstanceConfig desired hashes",
		"configHash", desiredConfigHash,
		"bootstrapHash", desiredBootstrapHash)

	// If no IC exists, create a new one
	if instanceConfiguration == nil {
		m.Info("No existing instance configuration, creating a new one")
		if err := m.createInstanceConfiguration(ctx, desiredLaunch, freeFormTags, definedTags, desiredConfigHash); err != nil {
			return err
		}
		m.setInstanceConfigurationHashAnnotation(desiredConfigHash)
		m.setBootstrapDataHashAnnotation(desiredBootstrapHash)
		if err := m.PatchObject(ctx); err != nil {
			return err
		}
		return nil
	}

	// Compute actual config hash from OCI
	computeDetails, ok := instanceConfiguration.InstanceDetails.(core.ComputeInstanceDetails)
	if !ok {
		m.Info("InstanceDetails not ComputeInstanceDetails, skipping hash compare")
		return nil
	}
	actualLaunch := computeDetails.LaunchDetails
	actualConfigHash, err := hash.ComputeComparableHash(actualLaunch, desiredLaunch)
	if err != nil {
		return errors.Wrap(err, "compute actual instance config hash")
	}

	m.Logger.V(1).Info("InstanceConfig actual config hash", "hash", actualConfigHash)

	actualBootstrapHash := hash.ComputeUserDataHash(actualLaunch.Metadata)
	storedBootstrapHash := m.getBootstrapDataHashAnnotation()

	// Backfill annotations on first reconciliation
	storedConfigHash := m.getInstanceConfigurationHashAnnotation()
	hadStoredConfigHash := storedConfigHash != ""
	hadStoredBootstrapHash := storedBootstrapHash != ""
	needsAnnotationPatch := false
	if storedConfigHash == "" {
		m.Info("No stored config hash annotation, backfilling", "actualConfigHash", actualConfigHash)
		m.setInstanceConfigurationHashAnnotation(actualConfigHash)
		storedConfigHash = actualConfigHash
		needsAnnotationPatch = true
	}
	if storedBootstrapHash == "" {
		m.Info("No stored bootstrap hash annotation, backfilling", "actualBootstrapHash", actualBootstrapHash)
		m.setBootstrapDataHashAnnotation(actualBootstrapHash)
		storedBootstrapHash = actualBootstrapHash
		needsAnnotationPatch = true
	}
	if needsAnnotationPatch {
		if err := m.PatchObject(ctx); err != nil {
			return err
		}
	}

	// Evaluate change signals
	//
	// Both signals compare desired vs actual (fetched from OCI):
	//
	//   configChanged:     desired config hash  vs  actual config hash (projected)
	//   bootstrapChanged: desired user_data hash  vs  actual user_data hash
	//
	// Config uses ComputeComparableHash which filters out OCI-returned scalar
	// defaults (e.g. ShapeConfig.MemoryInGBs on flex shapes) that would otherwise
	// cause continuous recreates (issue #509). Tag maps are compared as part of
	// the launch behavior so tag additions, changes, and removals recreate the
	// immutable instance configuration.
	//
	// Bootstrap compares OCI actual vs desired. We still classify kubeadm
	// discovery-token-only drift separately for observability, but bootstrap
	// drift always creates a new InstanceConfiguration so future replacements
	// don't launch with stale join configuration.
	//
	// The bootstrap hash annotation tracks the currently active IC's raw
	// user_data hash for observability and upgrade backfill.
	desiredBootstrapHashIgnoringToken := hash.ComputeUserDataHashIgnoringKubeadmToken(desiredLaunch.Metadata)
	actualBootstrapHashIgnoringToken := hash.ComputeUserDataHashIgnoringKubeadmToken(actualLaunch.Metadata)
	// Trust the stored desired config hash only after both hash annotations exist;
	// partial backfill can otherwise force a recreate unrelated to a spec change.
	desiredConfigChanged := hadStoredConfigHash && hadStoredBootstrapHash && storedConfigHash != desiredConfigHash
	configChanged := desiredConfigHash != actualConfigHash || desiredConfigChanged
	bootstrapChanged := desiredBootstrapHash != actualBootstrapHash
	tokenOnlyBootstrapChanged := bootstrapChanged && desiredBootstrapHashIgnoringToken == actualBootstrapHashIgnoringToken

	m.Logger.V(1).Info("InstanceConfig bootstrap hashes",
		"desired", desiredBootstrapHash,
		"actual", actualBootstrapHash,
		"desiredIgnoringToken", desiredBootstrapHashIgnoringToken,
		"actualIgnoringToken", actualBootstrapHashIgnoringToken)

	if !configChanged && !bootstrapChanged {
		m.Info("Instance configuration is up-to-date, no recreate",
			"configHash", desiredConfigHash,
			"bootstrapHash", desiredBootstrapHash)
		// Keep annotations consistent for observability
		needsAnnotationUpdate := false
		if storedConfigHash != actualConfigHash {
			m.setInstanceConfigurationHashAnnotation(actualConfigHash)
			needsAnnotationUpdate = true
		}
		if storedBootstrapHash != actualBootstrapHash {
			m.setBootstrapDataHashAnnotation(actualBootstrapHash)
			needsAnnotationUpdate = true
		}
		if needsAnnotationUpdate {
			return m.PatchObject(ctx)
		}
		return nil
	}

	// At least one signal changed, create new IC
	m.Info("creating new version for instance configuration",
		"needsUpdate", configChanged,
		"userDataHashChanged", bootstrapChanged,
		"tokenOnlyBootstrapChanged", tokenOnlyBootstrapChanged,
		"desiredConfigHash", desiredConfigHash,
		"actualConfigHash", actualConfigHash,
		"desiredBootstrapHash", desiredBootstrapHash,
		"actualBootstrapHash", actualBootstrapHash)

	if err := m.createInstanceConfiguration(ctx, desiredLaunch, freeFormTags, definedTags, desiredConfigHash); err != nil {
		return err
	}
	m.setInstanceConfigurationHashAnnotation(desiredConfigHash)
	m.setBootstrapDataHashAnnotation(desiredBootstrapHash)
	if err := m.PatchObject(ctx); err != nil {
		return err
	}

	return nil
}

// getDefinedTags builds the defined tags map for InstanceConfiguration creation.
func (m *MachinePoolScope) getDefinedTags() map[string]map[string]interface{} {
	definedTags := make(map[string]map[string]interface{})
	for ns, mapNs := range m.OCIClusterAccesor.GetDefinedTags() {
		mapValues := make(map[string]interface{})
		for k, v := range mapNs {
			mapValues[k] = v
		}
		definedTags[ns] = mapValues
	}
	for ns, mapNs := range m.OCIMachinePool.Spec.InstanceConfiguration.DefinedTags {
		mapValues, ok := definedTags[ns]
		if !ok {
			mapValues = make(map[string]interface{})
			definedTags[ns] = mapValues
		}
		for k, v := range mapNs {
			mapValues[k] = v
		}
	}
	return definedTags
}

// createInstanceConfiguration creates a new OCI InstanceConfiguration
func (m *MachinePoolScope) createInstanceConfiguration(
	ctx context.Context,
	launchDetails *core.InstanceConfigurationLaunchInstanceDetails,
	freeFormTags map[string]string,
	definedTags map[string]map[string]any,
	desiredHash string,
) error {
	launchInstanceDetails := core.ComputeInstanceDetails{
		LaunchDetails: launchDetails,
	}

	// Use hash suffix to avoid creating a new IC every time ResourceVersion changes.
	// Take first 10 characters of hash for uniqueness while keeping display name reasonably short.
	suffix := desiredHash
	if len(suffix) > 10 {
		suffix = suffix[:10]
	}

	displayName := fmt.Sprintf("%s-%s", m.OCIMachinePool.GetName(), suffix)

	req := core.CreateInstanceConfigurationRequest{
		CreateInstanceConfiguration: core.CreateInstanceConfigurationDetails{
			CompartmentId:   common.String(m.OCIClusterAccesor.GetCompartmentId()),
			DisplayName:     common.String(displayName),
			FreeformTags:    freeFormTags,
			DefinedTags:     definedTags,
			InstanceDetails: launchInstanceDetails,
		},
	}

	resp, err := m.ComputeManagementClient.CreateInstanceConfiguration(ctx, req)
	if err != nil {
		v1beta1conditions.MarkFalse(m.OCIMachinePool, infrav2exp.LaunchTemplateReadyCondition, infrav2exp.LaunchTemplateCreateFailedReason, clusterv1beta1.ConditionSeverityError, "%s", err.Error())
		m.Info("failed to create instance configuration")
		return err
	}

	m.Info("Created instance configuration", "id", ptr.ToString(resp.Id), "displayName", displayName)
	m.SetInstanceConfigurationIdStatus(*resp.Id)
	return nil
}

func (m *MachinePoolScope) getLaunchInstanceDetails(instanceConfigurationSpec infrav2exp.InstanceConfiguration, freeFormTags map[string]string, definedTags map[string]map[string]interface{}) (*core.InstanceConfigurationLaunchInstanceDetails, error) {
	metadata := copyStringMap(instanceConfigurationSpec.Metadata)
	cloudInitData, err := m.GetBootstrapData()
	if err != nil {
		return nil, err
	}
	metadata["user_data"] = base64.StdEncoding.EncodeToString([]byte(cloudInitData))

	extendedMetadata, err := ConvertMachineExtendedMetadata(instanceConfigurationSpec.ExtendedMetadata)
	if err != nil {
		return nil, err
	}

	launchDetails := &core.InstanceConfigurationLaunchInstanceDetails{
		CompartmentId:           common.String(m.OCIClusterAccesor.GetCompartmentId()),
		ClusterPlacementGroupId: instanceConfigurationSpec.ClusterPlacementGroupId,
		DisplayName:             common.String(m.OCIMachinePool.GetName()),
		Shape:                   common.String(*m.OCIMachinePool.Spec.InstanceConfiguration.Shape),
		Metadata:                metadata,
		ExtendedMetadata:        extendedMetadata,
		DedicatedVmHostId:       instanceConfigurationSpec.DedicatedVmHostId,
		FreeformTags:            freeFormTags,
		DefinedTags:             definedTags,
		IpxeScript:              instanceConfigurationSpec.IpxeScript,
	}
	licensingConfigs, err := buildLaunchInstanceLicensingConfigs(instanceConfigurationSpec.LicensingConfigs)
	if err != nil {
		return nil, err
	}
	launchDetails.LicensingConfigs = licensingConfigs

	if instanceConfigurationSpec.CapacityReservationId != nil {
		launchDetails.CapacityReservationId = instanceConfigurationSpec.CapacityReservationId
	}
	if instanceConfigurationSpec.IsPvEncryptionInTransitEnabled != nil {
		launchDetails.IsPvEncryptionInTransitEnabled = instanceConfigurationSpec.IsPvEncryptionInTransitEnabled
	}
	if instanceConfigurationSpec.LaunchMode != "" {
		launchMode, ok := core.GetMappingInstanceConfigurationLaunchInstanceDetailsLaunchModeEnum(string(instanceConfigurationSpec.LaunchMode))
		if !ok {
			return nil, errors.Errorf("unsupported launch mode %q, valid values: %s", instanceConfigurationSpec.LaunchMode,
				strings.Join(core.GetInstanceConfigurationLaunchInstanceDetailsLaunchModeEnumStringValues(), ", "))
		}
		launchDetails.LaunchMode = launchMode
	}
	if instanceConfigurationSpec.PreferredMaintenanceAction != "" {
		preferredMaintenanceAction, ok := core.GetMappingInstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnum(string(instanceConfigurationSpec.PreferredMaintenanceAction))
		if !ok {
			return nil, errors.Errorf("unsupported preferred maintenance action %q, valid values: %s", instanceConfigurationSpec.PreferredMaintenanceAction,
				strings.Join(core.GetInstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionEnumStringValues(), ", "))
		}
		launchDetails.PreferredMaintenanceAction = preferredMaintenanceAction
	}
	launchDetails.CreateVnicDetails = m.getVnicDetails(instanceConfigurationSpec, freeFormTags, definedTags)
	launchDetails.SourceDetails = m.getInstanceConfigurationInstanceSourceViaImageDetail()
	launchDetails.AgentConfig = m.getAgentConfig()
	launchDetails.LaunchOptions = m.getLaunchOptions()
	launchDetails.InstanceOptions = m.getInstanceOptions()
	launchDetails.AvailabilityConfig = m.getAvailabilityConfig()
	launchDetails.PreemptibleInstanceConfig = m.getPreemptibleInstanceConfig()
	if err := validatePlatformConfig(instanceConfigurationSpec.PlatformConfig); err != nil {
		return nil, err
	}
	launchDetails.PlatformConfig = m.getPlatformConfig()

	shapeConfig, err := m.buildInstanceConfigurationShapeConfig()
	if err != nil {
		v1beta1conditions.MarkFalse(m.OCIMachinePool, infrav2exp.LaunchTemplateReadyCondition, infrav2exp.LaunchTemplateCreateFailedReason, clusterv1beta1.ConditionSeverityError, "%s", err.Error())
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
		m.InstancePoolETag = response.Etag
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
	m.InstancePoolETag = respGet.Etag

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

			PlacementConfigurations:      placements,
			FreeformTags:                 tags,
			InstanceDisplayNameFormatter: m.OCIMachinePool.Spec.InstanceDisplayNameFormatter,
			InstanceHostnameFormatter:    m.OCIMachinePool.Spec.InstanceHostnameFormatter,
		},
	}
	instancePool, err := m.ComputeManagementClient.CreateInstancePool(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create OCIMachinePool")
	}
	m.Info("Created Instance Pool", "id", instancePool.Id)

	return &instancePool.InstancePool, nil
}

// UpdatePool advances one durable asynchronous OCI instance-pool update. It
// serializes changed desired states and requires fresh OCI readback before a
// submitted operation is considered converged.
func (m *MachinePoolScope) UpdatePool(ctx context.Context, instancePool *core.InstancePool) (InstancePoolUpdateOutcome, error) {
	converged := false
	attempt, err := m.getInstancePoolUpdateAttempt()
	if err != nil {
		return InstancePoolUpdateNoChange, err
	}
	if attempt != nil {
		if instancePool.LifecycleState == core.InstancePoolLifecycleStateRunning && attempt.Target.matches(instancePool) {
			if err := m.clearInstancePoolUpdateAttempt(ctx); err != nil {
				return InstancePoolUpdateNoChange, err
			}
			converged = true
		} else {
			if attempt.Phase == instancePoolUpdatePhaseSubmitted &&
				time.Since(attempt.StartedAt) >= instancePoolUpdateAttemptTimeout {
				return InstancePoolUpdateWaiting, errors.Errorf("instance pool update %s has not converged after %s while OCI lifecycle state is %q", attempt.Fingerprint, instancePoolUpdateAttemptTimeout, instancePool.LifecycleState)
			}
			if attempt.Phase == instancePoolUpdatePhasePrepared {
				if instancePool.LifecycleState != core.InstancePoolLifecycleStateRunning {
					return InstancePoolUpdateWaiting, nil
				}
				return m.submitInstancePoolUpdateAttempt(ctx, instancePool, attempt)
			}
			return InstancePoolUpdateWaiting, nil
		}
	}
	if instancePool.LifecycleState != core.InstancePoolLifecycleStateRunning {
		if converged {
			return InstancePoolUpdateConverged, nil
		}
		return InstancePoolUpdateNoChange, nil
	}

	placementConfigurations, err := m.buildUpdateInstancePoolPlacement()
	if err != nil {
		return InstancePoolUpdateNoChange, errors.Wrapf(err, "unable to build instance pool placements")
	}
	comparePlacements := len(m.OCIMachinePool.Spec.PlacementDetails) > 0 ||
		instancePoolHasPrimaryVnicSubnets(instancePool.PlacementConfigurations) ||
		(len(instancePool.PlacementConfigurations) > 0 && len(placementConfigurations) > 0)
	placementNeedsUpdate := comparePlacements && instancePoolPlacementNeedsUpdate(instancePool.PlacementConfigurations, placementConfigurations)
	if instancePoolNeedsUpdates(m, instancePool, placementNeedsUpdate) {
		m.Info("Updating instance pool")
		replicas := 0
		if m.MachinePool.Spec.Replicas != nil {
			replicas = int(*m.MachinePool.Spec.Replicas)
		}

		updateDetails := core.UpdateInstancePoolDetails{
			InstanceConfigurationId: m.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId,
			FreeformTags:            m.GetFreeFormTags(),
		}
		if !annotations.ReplicasManagedByExternalAutoscaler(m.MachinePool) {
			updateDetails.Size = common.Int(replicas)
		}
		if placementNeedsUpdate {
			updateDetails.PlacementConfigurations = placementConfigurations
		}
		updateDetails.InstanceDisplayNameFormatter = instancePoolFormatterUpdateValue(
			m.OCIMachinePool.Spec.InstanceDisplayNameFormatter,
			instancePool.InstanceDisplayNameFormatter,
		)
		updateDetails.InstanceHostnameFormatter = instancePoolFormatterUpdateValue(
			m.OCIMachinePool.Spec.InstanceHostnameFormatter,
			instancePool.InstanceHostnameFormatter,
		)

		attempt, err := newInstancePoolUpdateAttempt(updateDetails, m.InstancePoolETag)
		if err != nil {
			return InstancePoolUpdateNoChange, errors.Wrap(err, "build instance pool update attempt")
		}
		if err := m.setInstancePoolUpdateAttempt(ctx, attempt); err != nil {
			return InstancePoolUpdateNoChange, err
		}
		return m.submitInstancePoolUpdateAttempt(ctx, instancePool, attempt)
	}
	if converged {
		return InstancePoolUpdateConverged, nil
	}
	return InstancePoolUpdateNoChange, nil
}

func instancePoolHasPrimaryVnicSubnets(placements []core.InstancePoolPlacementConfiguration) bool {
	for _, placement := range placements {
		if placement.PrimaryVnicSubnets != nil {
			return true
		}
	}
	return false
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
func instancePoolNeedsUpdates(machinePoolScope *MachinePoolScope, instancePool *core.InstancePool, placementNeedsUpdate bool) bool {
	instanePoolSize := 0
	machinePoolReplicas := 0
	if machinePoolScope.MachinePool.Spec.Replicas != nil {
		machinePoolReplicas = int(*machinePoolScope.MachinePool.Spec.Replicas)
	}

	if instancePool.Size != nil {
		instanePoolSize = *instancePool.Size
	}
	if !annotations.ReplicasManagedByExternalAutoscaler(machinePoolScope.MachinePool) && machinePoolReplicas != instanePoolSize {
		return true
	} else if !ptr.StringEqual(machinePoolScope.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId, instancePool.InstanceConfigurationId) {
		return true
	} else if !instancePoolFormattersEqual(machinePoolScope.OCIMachinePool.Spec.InstanceDisplayNameFormatter, instancePool.InstanceDisplayNameFormatter) {
		return true
	} else if !instancePoolFormattersEqual(machinePoolScope.OCIMachinePool.Spec.InstanceHostnameFormatter, instancePool.InstanceHostnameFormatter) {
		return true
	}
	return placementNeedsUpdate
}

func instancePoolFormatterUpdateValue(desired, actual *string) *string {
	if desired == nil && ptr.ToString(actual) != "" {
		return common.String("")
	}
	return desired
}

func instancePoolFormattersEqual(desired, actual *string) bool {
	return ptr.ToString(desired) == ptr.ToString(actual)
}

func instancePoolPlacementNeedsUpdate(actual []core.InstancePoolPlacementConfiguration, desired []core.UpdateInstancePoolPlacementConfigurationDetails) bool {
	if len(actual) != len(desired) {
		return true
	}

	actualByAvailabilityDomain := make(map[string]core.InstancePoolPlacementConfiguration, len(actual))
	for _, placement := range actual {
		if placement.AvailabilityDomain == nil {
			return true
		}
		availabilityDomain := *placement.AvailabilityDomain
		if _, ok := actualByAvailabilityDomain[availabilityDomain]; ok {
			return true
		}
		actualByAvailabilityDomain[availabilityDomain] = placement
	}

	for _, desiredPlacement := range desired {
		if desiredPlacement.AvailabilityDomain == nil {
			return true
		}

		actualPlacement, ok := actualByAvailabilityDomain[*desiredPlacement.AvailabilityDomain]
		if !ok {
			return true
		}
		if desiredPlacement.PrimarySubnetId != nil && !ptr.StringEqual(actualPlacement.PrimarySubnetId, desiredPlacement.PrimarySubnetId) {
			return true
		}
		if !samePrimaryVnicSubnets(actualPlacement.PrimaryVnicSubnets, desiredPlacement.PrimaryVnicSubnets) {
			return true
		}
		if !sameStringSet(actualPlacement.FaultDomains, desiredPlacement.FaultDomains) {
			return true
		}
	}

	return false
}

// samePrimaryVnicSubnets compares desired vs actual PrimaryVnicSubnets semantically
// rather than with reflect.DeepEqual, because OCI may normalize IsAssignIpv6Ip nil→false
// or return IPv6 CIDR pairs in a different order, causing spurious drift detection.
// When desired is nil the user did not set the field, so any OCI value is acceptable.
func samePrimaryVnicSubnets(actual, desired *core.InstancePoolPlacementPrimarySubnet) bool {
	if desired == nil {
		return true
	}
	if actual == nil {
		return false
	}
	if ptr.ToString(actual.SubnetId) != ptr.ToString(desired.SubnetId) {
		return false
	}
	desiredIpv6 := desired.IsAssignIpv6Ip != nil && *desired.IsAssignIpv6Ip
	actualIpv6 := actual.IsAssignIpv6Ip != nil && *actual.IsAssignIpv6Ip
	if desiredIpv6 != actualIpv6 {
		return false
	}
	return true
}

// InstancePoolUsesDesiredInstanceConfiguration reports whether the actual
// InstancePool has switched to the desired backing InstanceConfiguration and
// any MachinePool-level formatter fields that are updated on the InstancePool.
func (m *MachinePoolScope) InstancePoolUsesDesiredInstanceConfiguration(instancePool *core.InstancePool) bool {
	if instancePool == nil {
		return true
	}
	desiredID := m.GetInstanceConfigurationId()
	return desiredID != nil &&
		ptr.StringEqual(desiredID, instancePool.InstanceConfigurationId) &&
		instancePoolFormattersEqual(m.OCIMachinePool.Spec.InstanceDisplayNameFormatter, instancePool.InstanceDisplayNameFormatter) &&
		instancePoolFormattersEqual(m.OCIMachinePool.Spec.InstanceHostnameFormatter, instancePool.InstanceHostnameFormatter)
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
			KmsKeyId:            sourceConfig.KmsKeyId,
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
			IsLiveMigrationPreferred: avalabilityConfigSpec.IsLiveMigrationPreferred,
			RecoveryAction:           recoveryAction,
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
				IsSecureBootEnabled:              platformConfig.AmdVmPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled:   platformConfig.AmdVmPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:            platformConfig.AmdVmPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:        platformConfig.AmdVmPlatformConfig.IsMemoryEncryptionEnabled,
				IsSymmetricMultiThreadingEnabled: platformConfig.AmdVmPlatformConfig.IsSymmetricMultiThreadingEnabled,
			}
		case infrastructurev1beta2.PlatformConfigTypeIntelVm:
			return core.IntelVmPlatformConfig{
				IsSecureBootEnabled:              platformConfig.IntelVmPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled:   platformConfig.IntelVmPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:            platformConfig.IntelVmPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:        platformConfig.IntelVmPlatformConfig.IsMemoryEncryptionEnabled,
				IsSymmetricMultiThreadingEnabled: platformConfig.IntelVmPlatformConfig.IsSymmetricMultiThreadingEnabled,
			}
		case infrastructurev1beta2.PlatformConfigTypeIntelSkylakeBm:
			numaNodesPerSocket, _ := core.GetMappingIntelSkylakeBmPlatformConfigNumaNodesPerSocketEnum(string(platformConfig.IntelSkylakeBmPlatformConfig.NumaNodesPerSocket))
			return core.IntelSkylakeBmPlatformConfig{
				IsSecureBootEnabled:                      platformConfig.IntelSkylakeBmPlatformConfig.IsSecureBootEnabled,
				IsTrustedPlatformModuleEnabled:           platformConfig.IntelSkylakeBmPlatformConfig.IsTrustedPlatformModuleEnabled,
				IsMeasuredBootEnabled:                    platformConfig.IntelSkylakeBmPlatformConfig.IsMeasuredBootEnabled,
				IsMemoryEncryptionEnabled:                platformConfig.IntelSkylakeBmPlatformConfig.IsMemoryEncryptionEnabled,
				IsSymmetricMultiThreadingEnabled:         platformConfig.IntelSkylakeBmPlatformConfig.IsSymmetricMultiThreadingEnabled,
				IsInputOutputMemoryManagementUnitEnabled: platformConfig.IntelSkylakeBmPlatformConfig.IsInputOutputMemoryManagementUnitEnabled,
				PercentageOfCoresEnabled:                 platformConfig.IntelSkylakeBmPlatformConfig.PercentageOfCoresEnabled,
				NumaNodesPerSocket:                       numaNodesPerSocket,
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

// CleanupInstanceConfiguration deletes unused InstanceConfigurations that are no longer referenced
// by the current MachinePool, keeping only the currently active one.
func (m *MachinePoolScope) CleanupInstanceConfiguration(ctx context.Context, instancePool *core.InstancePool) error {
	m.Info("Cleaning up unused instance configurations")

	if instancePool != nil && m.HasPendingInstancePoolUpdate() {
		m.Info("Deferring instance configuration cleanup while an instance pool update is pending")
		return nil
	}
	if !m.InstancePoolUsesDesiredInstanceConfiguration(instancePool) {
		m.Info("Deferring instance configuration cleanup until instance pool switch is observed",
			"desiredInstanceConfigurationId", ptr.ToString(m.GetInstanceConfigurationId()),
			"actualInstanceConfigurationId", ptr.ToString(instancePool.InstanceConfigurationId),
			"desiredInstanceDisplayNameFormatter", ptr.ToString(m.OCIMachinePool.Spec.InstanceDisplayNameFormatter),
			"actualInstanceDisplayNameFormatter", ptr.ToString(instancePool.InstanceDisplayNameFormatter),
			"desiredInstanceHostnameFormatter", ptr.ToString(m.OCIMachinePool.Spec.InstanceHostnameFormatter),
			"actualInstanceHostnameFormatter", ptr.ToString(instancePool.InstanceHostnameFormatter))
		return nil
	}

	ids, err := m.getInstanceConfigurationsFromDisplayNameSortedTimeCreateDescending(ctx, m.OCIMachinePool.GetName())
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	keepID := m.GetInstanceConfigurationId()
	if instancePool != nil && instancePool.InstanceConfigurationId != nil {
		keepID = instancePool.InstanceConfigurationId
	}

	keep := ptr.ToString(keepID)
	m.Info("Instance configuration cleanup", "found", len(ids), "keep", keep)

	for _, id := range ids {
		if ptr.ToString(id) == keep {
			continue
		}
		req := core.DeleteInstanceConfigurationRequest{InstanceConfigurationId: id}
		if _, err := m.ComputeManagementClient.DeleteInstanceConfiguration(ctx, req); err != nil {
			if m.Recorder != nil {
				m.Recorder.Eventf(m.OCIMachinePool, corev1.EventTypeWarning, "CleanupInstanceConfigurationFailed", "Failed to delete old instance configuration %q: %v", ptr.ToString(id), err)
			}
			return errors.Wrap(err, "failed to delete expired instance configuration")
		}
		m.Info("Deleted old instance configuration", "id", ptr.ToString(id))
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
	nsgIDs := m.getWorkerMachineNSGs()
	createVnicDetails := core.InstanceConfigurationCreateVnicDetails{
		SubnetId:     subnetId,
		FreeformTags: freeFormTags,
		DefinedTags:  definedTags,
		NsgIds:       nsgIDs,
	}
	if instanceConfigurationSpec.InstanceVnicConfiguration != nil {
		createVnicDetails.AssignIpv6Ip = common.Bool(instanceConfigurationSpec.InstanceVnicConfiguration.AssignIpv6Ip)
		createVnicDetails.AssignPublicIp = common.Bool(instanceConfigurationSpec.InstanceVnicConfiguration.AssignPublicIp)
		if instanceConfigurationSpec.InstanceVnicConfiguration.SubnetId != nil {
			createVnicDetails.SubnetId = instanceConfigurationSpec.InstanceVnicConfiguration.SubnetId
		}
		if len(instanceConfigurationSpec.InstanceVnicConfiguration.NSGIds) > 0 {
			createVnicDetails.NsgIds = append([]string(nil), instanceConfigurationSpec.InstanceVnicConfiguration.NSGIds...)
		} else if instanceConfigurationSpec.InstanceVnicConfiguration.NSGId != nil {
			createVnicDetails.NsgIds = []string{*instanceConfigurationSpec.InstanceVnicConfiguration.NSGId}
		}
		createVnicDetails.HostnameLabel = instanceConfigurationSpec.InstanceVnicConfiguration.HostnameLabel
		createVnicDetails.SkipSourceDestCheck = instanceConfigurationSpec.InstanceVnicConfiguration.SkipSourceDestCheck
		createVnicDetails.AssignPrivateDnsRecord = instanceConfigurationSpec.InstanceVnicConfiguration.AssignPrivateDnsRecord
		createVnicDetails.DisplayName = instanceConfigurationSpec.InstanceVnicConfiguration.DisplayName
	}
	return &createVnicDetails
}

func buildLaunchInstanceLicensingConfigs(spec []infrav2exp.LaunchInstanceLicensingConfig) ([]core.LaunchInstanceLicensingConfig, error) {
	if len(spec) == 0 {
		return nil, nil
	}
	configs := make([]core.LaunchInstanceLicensingConfig, 0, len(spec))
	for _, licensingConfig := range spec {
		switch licensingConfig.Type {
		case infrav2exp.LaunchInstanceLicensingConfigTypeEnum(core.LaunchInstanceLicensingConfigTypeWindows):
			licenseType, ok := core.GetMappingLaunchInstanceLicensingConfigLicenseTypeEnum(string(licensingConfig.LicenseType))
			if !ok {
				return nil, errors.Errorf("unsupported licensing config license type %q, valid values: %s", licensingConfig.LicenseType,
					strings.Join(core.GetLaunchInstanceLicensingConfigLicenseTypeEnumStringValues(), ", "))
			}
			configs = append(configs, core.LaunchInstanceWindowsLicensingConfig{
				LicenseType: licenseType,
			})
		default:
			return nil, errors.Errorf("unsupported licensing config type %q, valid values: %s", licensingConfig.Type,
				strings.Join(core.GetLaunchInstanceLicensingConfigTypeEnumStringValues(), ", "))
		}
	}
	return configs, nil
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return make(map[string]string)
	}
	out := make(map[string]string, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
