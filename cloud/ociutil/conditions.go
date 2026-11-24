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

package ociutil

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
)

// MarkConditionTrue sets the given condition to True using standard metav1.Condition.
func MarkConditionTrue(target conditions.Setter, conditionType string) {
	conditions.Set(target, metav1.Condition{
		Type:   conditionType,
		Status: metav1.ConditionTrue,
		Reason: conditionType,
	})
}

// MarkConditionFalse sets the given condition to False with a reason and optional formatted message.
func MarkConditionFalse(target conditions.Setter, conditionType, reason, messageFormat string, messageArgs ...any) {
	msg := ""
	if messageFormat != "" {
		msg = fmt.Sprintf(messageFormat, messageArgs...)
	}
	conditions.Set(target, metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: msg,
	})
}

// MarkConditionUnknown sets the given condition to Unknown with a reason and optional formatted message.
func MarkConditionUnknown(target conditions.Setter, conditionType, reason, messageFormat string, messageArgs ...any) {
	msg := ""
	if messageFormat != "" {
		msg = fmt.Sprintf(messageFormat, messageArgs...)
	}
	conditions.Set(target, metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionUnknown,
		Reason:  reason,
		Message: msg,
	})
}

// SetReadySummary computes and sets the Ready condition from the provided condition types.
func SetReadySummary(target conditions.Setter, conditionTypes ...string) {
	SetSummaryCondition(target, clusterv1.ReadyCondition, conditionTypes...)
}

// SetSummaryCondition computes and sets targetConditionType from the provided condition types.
func SetSummaryCondition(target conditions.Setter, targetConditionType string, conditionTypes ...string) {
	if len(conditionTypes) == 0 {
		return
	}
	_ = conditions.SetSummaryCondition(target, target, targetConditionType, conditions.ForConditionTypes(conditionTypes))
}
