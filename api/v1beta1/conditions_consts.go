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

package v1beta1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// InstanceReadyCondition Ready indicates the instance is in a Running state.
	InstanceReadyCondition clusterv1.ConditionType = "InstanceReady"
	// InstanceNotFoundReason used when the instance couldn't be retrieved.
	InstanceNotFoundReason = "InstanceNotFound"
	// InstanceTerminatedReason instance is in a terminated state.
	InstanceTerminatedReason = "InstanceTerminated"
	// InstanceTerminatingReason instance is in terminating state.
	InstanceTerminatingReason = "InstanceTerminating"
	// InstanceNotReadyReason used when the instance is in a pending state.
	InstanceNotReadyReason = "InstanceNotReady"
	// InstanceProvisionStartedReason set when the provisioning of an instance started.
	InstanceProvisionStartedReason = "InstanceProvisionStarted"
	// InstanceProvisionFailedReason used for failures during instance provisioning.
	InstanceProvisionFailedReason = "InstanceProvisionFailed"
	// WaitingForClusterInfrastructureReason used when machine is waiting for cluster infrastructure to be ready before proceeding.
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	// WaitingForBootstrapDataReason used when machine is waiting for bootstrap data to be ready before proceeding.
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"
	// InstanceLBackendAdditionFailedReason used when addition to LB backend fails
	InstanceLBackendAdditionFailedReason = "BackendAdditionFailed"
	// ClusterReadyCondition Ready indicates the cluster is Ready.
	ClusterReadyCondition clusterv1.ConditionType = "ClusterReady"
	// VcnReconciliationFailedReason used when the vcn reconciliation is failed.
	VcnReconciliationFailedReason = "VcnReconciliationFailed"
	// DrgReconciliationFailedReason used when the DRG reconciliation fails.
	DrgReconciliationFailedReason = "DRGReconciliationFailed"
	// DRGVCNAttachmentReconciliationFailedReason used when the DRG VCN Attachment reconciliation fails.
	DRGVCNAttachmentReconciliationFailedReason = "DRGVCNAttachmentReconciliationFailed"
	// DRGRPCAttachmentReconciliationFailedReason used when the DRG RPC Attachment reconciliation fails.
	DRGRPCAttachmentReconciliationFailedReason = "DRGRPCAttachmentReconciliationFailed"
	// InternetGatewayReconciliationFailedReason used when the InternetGateway reconciliation is failed.
	InternetGatewayReconciliationFailedReason = "InternetGatewayReconciliationFailed"
	// NatGatewayReconciliationFailedReason used when the NatGateway reconciliation is failed.
	NatGatewayReconciliationFailedReason = "NatGatewayReconciliationFailed"
	// ServiceGatewayReconciliationFailedReason used when the ServiceGateway reconciliation is failed.
	ServiceGatewayReconciliationFailedReason = "ServiceGatewayReconciliationFailed"
	// NSGReconciliationFailedReason used when the NSG reconciliation is failed.
	NSGReconciliationFailedReason = "NSGReconciliationFailed"
	// RouteTableReconciliationFailedReason used when the RouteTable reconciliation is failed.
	RouteTableReconciliationFailedReason = "RouteTableReconciliationFailed"
	// SubnetReconciliationFailedReason used when the Subnet reconciliation is failed.
	SubnetReconciliationFailedReason = "SubnetReconciliationFailed"
	// SecurityListReconciliationFailedReason used when the SecurityList reconciliation is failed.
	SecurityListReconciliationFailedReason = "SecurityListReconciliationFailed"
	// APIServerLoadBalancerFailedReason used when the Subnet reconciliation is failed.
	APIServerLoadBalancerFailedReason = "APIServerLoadBalancerReconciliationFailed"
	// FailureDomainFailedReason used when the Subnet reconciliation is failed.
	FailureDomainFailedReason = "FailureDomainFailedReconciliationFailed"
	// InstanceLBBackendAdditionFailedReason used when addition to LB backend fails
	InstanceLBBackendAdditionFailedReason = "BackendAdditionFailed"
	// InstanceVnicAttachmentFailedReason used when attaching vnics to machine
	InstanceVnicAttachmentFailedReason = "VnicAttachmentFailed"
	// InstanceIPAddressNotFound used when IP address of the instance count not be found
	InstanceIPAddressNotFound = "InstanceIPAddressNotFound"
	// VcnEventReady used after reconciliation has completed successfully
	VcnEventReady = "VCNReady"
	// DrgEventReady used after reconciliation has completed successfully
	DrgEventReady = "DRGReady"
	// DRGVCNAttachmentEventReady used after reconciliation has completed successfully
	DRGVCNAttachmentEventReady = "DRGVCNAttachmentEventReady"
	// DRGRPCAttachmentEventReady used after reconciliation has completed successfully
	DRGRPCAttachmentEventReady = "DRGRPCAttachmentEventReady"
	// InternetGatewayEventReady used after reconciliation has completed successfully
	InternetGatewayEventReady = "InternetGatewayReady"
	// NatEventReady used after reconciliation has completed successfully
	NatEventReady = "NATReady"
	// ServiceGatewayEventReady used after reconciliation has completed successfully
	ServiceGatewayEventReady = "ServiceGatewayReady"
	// NetworkSecurityEventReady used after reconciliation has completed successfully
	NetworkSecurityEventReady = "NetworkSecurityReady"
	// RouteTableEventReady used after reconciliation has completed successfully
	RouteTableEventReady = "RouteTableReady"
	// SubnetEventReady used after reconciliation has completed successfully
	SubnetEventReady = "SubnetReady"
	// InstanceVnicAttachmentReady used after reconciliation has been completed successfully
	InstanceVnicAttachmentReady = "VnicAttachmentReady"
	// ApiServerLoadBalancerEventReady used after reconciliation has completed successfully
	ApiServerLoadBalancerEventReady = "APIServerLoadBalancerReady"
	// FailureDomainEventReady used after reconciliation has completed successfully
	FailureDomainEventReady = "FailureDomainsReady"
)
