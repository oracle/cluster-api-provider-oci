/*
Copyright (c) 2024 Oracle and/or its affiliates.

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

package conditions

import (
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// ConditionGetter interface for objects that have GetConditions method
type ConditionGetter interface {
	GetConditions() clusterv1beta1.Conditions
}

// ConditionSetter interface for objects that have SetConditions method
type ConditionSetter interface {
	ConditionGetter
	SetConditions(clusterv1beta1.Conditions)
}

// Get returns the condition with the given type, or nil if not found.
// This is an alias for GetCondition to match the CAPI conditions package API.
func Get(from ConditionGetter, t clusterv1beta1.ConditionType) *clusterv1beta1.Condition {
	return GetCondition(from, t)
}

// GetCondition returns the condition with the given type, or nil if not found
func GetCondition(from ConditionGetter, t clusterv1beta1.ConditionType) *clusterv1beta1.Condition {
	conditions := from.GetConditions()
	for i := range conditions {
		if conditions[i].Type == t {
			return &conditions[i]
		}
	}
	return nil
}

// SetCondition sets a condition on an object
func SetCondition(to ConditionSetter, condition *clusterv1beta1.Condition) {
	if condition == nil {
		return
	}

	conditions := to.GetConditions()
	exists := false
	for i := range conditions {
		if conditions[i].Type == condition.Type {
			// Update existing condition
			if conditions[i].Status != condition.Status {
				condition.LastTransitionTime = metav1.NewTime(time.Now().UTC().Truncate(time.Second))
			} else {
				condition.LastTransitionTime = conditions[i].LastTransitionTime
			}
			conditions[i] = *condition
			exists = true
			break
		}
	}
	if !exists {
		if condition.LastTransitionTime.IsZero() {
			condition.LastTransitionTime = metav1.NewTime(time.Now().UTC().Truncate(time.Second))
		}
		conditions = append(conditions, *condition)
	}
	to.SetConditions(conditions)
}

// MarkConditionTrue sets the condition to True
func MarkConditionTrue(to ConditionSetter, t clusterv1beta1.ConditionType) {
	SetCondition(to, &clusterv1beta1.Condition{
		Type:   t,
		Status: corev1.ConditionTrue,
	})
}

// MarkConditionFalse sets the condition to False
func MarkConditionFalse(to ConditionSetter, t clusterv1beta1.ConditionType, reason string, severity clusterv1beta1.ConditionSeverity, messageFormat string, messageArgs ...interface{}) {
	SetCondition(to, &clusterv1beta1.Condition{
		Type:     t,
		Status:   corev1.ConditionFalse,
		Severity: severity,
		Reason:   reason,
		Message:  fmt.Sprintf(messageFormat, messageArgs...),
	})
}

// MarkConditionUnknown sets the condition to Unknown
func MarkConditionUnknown(to ConditionSetter, t clusterv1beta1.ConditionType, reason string, messageFormat string, messageArgs ...interface{}) {
	SetCondition(to, &clusterv1beta1.Condition{
		Type:    t,
		Status:  corev1.ConditionUnknown,
		Reason:  reason,
		Message: fmt.Sprintf(messageFormat, messageArgs...),
	})
}

// SetSummaryCondition sets a Ready condition that summarizes all other conditions
func SetSummaryCondition(to ConditionSetter) {
	conditions := to.GetConditions()
	if len(conditions) == 0 {
		return
	}

	// Check if any condition is False or Unknown
	var falseConditions []clusterv1beta1.Condition
	var unknownConditions []clusterv1beta1.Condition

	for _, c := range conditions {
		if c.Type == clusterv1beta1.ReadyCondition {
			continue
		}
		switch c.Status {
		case corev1.ConditionFalse:
			falseConditions = append(falseConditions, c)
		case corev1.ConditionUnknown:
			unknownConditions = append(unknownConditions, c)
		}
	}

	// Determine summary status
	var summary *clusterv1beta1.Condition
	if len(falseConditions) > 0 {
		// Sort by severity (Error > Warning > Info)
		sort.Slice(falseConditions, func(i, j int) bool {
			return severityOrder(falseConditions[i].Severity) < severityOrder(falseConditions[j].Severity)
		})
		c := falseConditions[0]
		summary = &clusterv1beta1.Condition{
			Type:     clusterv1beta1.ReadyCondition,
			Status:   corev1.ConditionFalse,
			Severity: c.Severity,
			Reason:   c.Reason,
			Message:  c.Message,
		}
	} else if len(unknownConditions) > 0 {
		c := unknownConditions[0]
		summary = &clusterv1beta1.Condition{
			Type:    clusterv1beta1.ReadyCondition,
			Status:  corev1.ConditionUnknown,
			Reason:  c.Reason,
			Message: c.Message,
		}
	} else {
		summary = &clusterv1beta1.Condition{
			Type:   clusterv1beta1.ReadyCondition,
			Status: corev1.ConditionTrue,
		}
	}

	SetCondition(to, summary)
}

func severityOrder(s clusterv1beta1.ConditionSeverity) int {
	switch s {
	case clusterv1beta1.ConditionSeverityError:
		return 0
	case clusterv1beta1.ConditionSeverityWarning:
		return 1
	case clusterv1beta1.ConditionSeverityInfo:
		return 2
	default:
		return 3
	}
}
