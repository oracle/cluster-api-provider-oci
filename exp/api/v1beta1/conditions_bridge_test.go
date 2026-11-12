/*
Copyright (c) 2026 Oracle and/or its affiliates.

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

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

type conditionBridgeObject interface {
	runtime.Object
	GetV1Beta2Conditions() []metav1.Condition
	SetV1Beta2Conditions([]metav1.Condition)
}

type conditionBridgeHub interface {
	conditionBridgeObject
	conversion.Hub
}

type conditionBridgeSpoke interface {
	conditionBridgeObject
	conversion.Convertible
}

type conditionBridgeFixture struct {
	name  string
	hub   func() conditionBridgeHub
	spoke func() conditionBridgeSpoke
}

func conditionBridgeFixtures() []conditionBridgeFixture {
	return []conditionBridgeFixture{
		{"OCIMachinePool", func() conditionBridgeHub { return &v1beta2.OCIMachinePool{} }, func() conditionBridgeSpoke { return &OCIMachinePool{} }},
		{"OCIManagedMachinePool", func() conditionBridgeHub { return &v1beta2.OCIManagedMachinePool{} }, func() conditionBridgeSpoke { return &OCIManagedMachinePool{} }},
		{"OCIVirtualMachinePool", func() conditionBridgeHub { return &v1beta2.OCIVirtualMachinePool{} }, func() conditionBridgeSpoke { return &OCIVirtualMachinePool{} }},
		{"OCIMachinePoolMachine", func() conditionBridgeHub { return &v1beta2.OCIMachinePoolMachine{} }, func() conditionBridgeSpoke { return &OCIMachinePoolMachine{} }},
	}
}

func TestV1Beta2ConditionsAccessors(t *testing.T) {
	for _, fixture := range conditionBridgeFixtures() {
		objects := map[string]conditionBridgeObject{"v1beta1": fixture.spoke(), "v1beta2": fixture.hub()}
		for version, object := range objects {
			t.Run(fixture.name+"/"+version, func(t *testing.T) {
				if got := object.GetV1Beta2Conditions(); got != nil {
					t.Fatalf("unset conditions = %#v, want nil", got)
				}
				setConditionBridgeLegacyStatus(t, object)
				legacy := conditionBridgeLegacyStatus(t, object)
				conditions := conditionBridgeConditions()
				object.SetV1Beta2Conditions(conditions)
				if !reflect.DeepEqual(object.GetV1Beta2Conditions(), conditions) {
					t.Fatal("standard conditions were not set")
				}
				copied := object.DeepCopyObject().(conditionBridgeObject)
				copied.GetV1Beta2Conditions()[0].Message = "changed in copy"
				if !reflect.DeepEqual(object.GetV1Beta2Conditions(), conditions) {
					t.Fatal("deep copy shares standard conditions with the original")
				}
				object.SetV1Beta2Conditions(nil)
				if object.GetV1Beta2Conditions() != nil {
					t.Fatal("clearing standard conditions did not return nil")
				}
				if !reflect.DeepEqual(conditionBridgeLegacyStatus(t, object), legacy) {
					t.Fatal("standard condition accessors changed legacy status")
				}
			})
		}
	}
}

func TestV1Beta2ConditionsConversion(t *testing.T) {
	for _, fixture := range conditionBridgeFixtures() {
		for _, state := range []string{"absent", "empty", "populated"} {
			t.Run(fixture.name+"/"+state, func(t *testing.T) {
				hub, spoke := fixture.hub(), fixture.spoke()
				setConditionBridgeLegacyStatus(t, hub)
				if state == "empty" {
					hub.SetV1Beta2Conditions(nil)
				} else if state == "populated" {
					hub.SetV1Beta2Conditions(conditionBridgeConditions())
				}
				wantStatus := conditionBridgeStatus(t, hub)
				if err := spoke.ConvertFrom(hub); err != nil {
					t.Fatal(err)
				}
				if got := conditionBridgeStatus(t, spoke); !reflect.DeepEqual(got, wantStatus) {
					t.Fatalf("down-conversion changed status: got %#v, want %#v", got, wantStatus)
				}

				// The typed legacy-view edit must win over the older snapshot in conversion-data.
				if state == "populated" {
					edited := conditionBridgeConditions()
					edited[0].Status = metav1.ConditionFalse
					edited[0].ObservedGeneration = 4
					edited[0].Reason = "NetworkUnavailable"
					edited[0].Message = "updated through v1beta1"
					spoke.SetV1Beta2Conditions(edited)
				}
				wantStatus = conditionBridgeStatus(t, spoke)
				restoredHub := fixture.hub()
				if err := spoke.ConvertTo(restoredHub); err != nil {
					t.Fatal(err)
				}
				if got := conditionBridgeStatus(t, restoredHub); !reflect.DeepEqual(got, wantStatus) {
					t.Fatalf("up-conversion changed status: got %#v, want %#v", got, wantStatus)
				}
				if restoredHub.(metav1.Object).GetUID() != hub.(metav1.Object).GetUID() {
					t.Fatal("conversion changed object identity")
				}

				// Exercise a spoke-originated write without a preservation annotation as well.
				freshSpoke := fixture.spoke()
				setConditionBridgeLegacyStatus(t, freshSpoke)
				if state != "absent" {
					freshSpoke.SetV1Beta2Conditions(spoke.GetV1Beta2Conditions())
				}
				wantStatus = conditionBridgeStatus(t, freshSpoke)
				freshHub := fixture.hub()
				if err := freshSpoke.ConvertTo(freshHub); err != nil {
					t.Fatal(err)
				}
				restoredSpoke := fixture.spoke()
				if err := restoredSpoke.ConvertFrom(freshHub); err != nil {
					t.Fatal(err)
				}
				if got := conditionBridgeStatus(t, restoredSpoke); !reflect.DeepEqual(got, wantStatus) {
					t.Fatalf("spoke round trip changed status: got %#v, want %#v", got, wantStatus)
				}
			})
		}
	}
}

func conditionBridgeConditions() []metav1.Condition {
	return []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue, ObservedGeneration: 3, LastTransitionTime: metav1.NewTime(time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)), Reason: "Ready", Message: "infrastructure ready"},
		{Type: "ProviderSpecificReady", Status: metav1.ConditionUnknown, ObservedGeneration: 2, LastTransitionTime: metav1.NewTime(time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)), Reason: "Observing", Message: "waiting for inventory"},
	}
}

func setConditionBridgeLegacyStatus(t *testing.T, object runtime.Object) {
	t.Helper()
	data := []byte(`{"metadata":{"uid":"existing-object"},"status":{"ready":true,"initialized":true,"conditions":[{"type":"LegacyReady","status":"False","severity":"Warning","lastTransitionTime":"2026-11-11T00:00:00Z","reason":"LegacyReason","message":"legacy diagnostics"}]}}`)
	if err := json.Unmarshal(data, object); err != nil {
		t.Fatal(err)
	}
}

func conditionBridgeStatus(t *testing.T, object runtime.Object) map[string]interface{} {
	t.Helper()
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded["status"].(map[string]interface{})
}

func conditionBridgeLegacyStatus(t *testing.T, object runtime.Object) map[string]interface{} {
	t.Helper()
	status := conditionBridgeStatus(t, object)
	delete(status, "v1beta2")
	return status
}
