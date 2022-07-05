/*
Copyright 2022 The Kubernetes Authors.

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

package v1beta1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// InstancePoolReadyCondition reports on current status of the Instance Pool. Ready indicates the group is provisioned.
	InstancePoolReadyCondition clusterv1.ConditionType = "InstancePoolReady"
	// InstancePoolNotFoundReason used when the Instance Pool couldn't be retrieved.
	InstancePoolNotFoundReason = "InstancePoolNotFound"
	// InstancePoolProvisionFailedReason used for failures during Instance Pool provisioning.
	InstancePoolProvisionFailedReason = "InstancePoolProvisionFailed"
	// InstancePoolDeletionInProgress Instance Pool is in a deletion in progress state.
	InstancePoolDeletionInProgress = "InstancePoolDeletionInProgress"
	// InstancePoolNotReadyReason used when the instance pool is in a pending state.
	InstancePoolNotReadyReason = "InstancePoolNotReady"

	// LaunchTemplateReadyCondition represents the status of an OCIachinePool's associated Instance Template.
	LaunchTemplateReadyCondition clusterv1.ConditionType = "LaunchTemplateReady"
	// LaunchTemplateNotFoundReason is used when an associated Launch Template can't be found.
	LaunchTemplateNotFoundReason = "LaunchTemplateNotFound"
	// LaunchTemplateCreateFailedReason used for failures during Launch Template creation.
	LaunchTemplateCreateFailedReason = "LaunchTemplateCreateFailed"
)
