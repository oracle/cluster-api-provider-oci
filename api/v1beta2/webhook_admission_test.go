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

package v1beta2

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestCustomWebhookHandlersProcessSerializedAdmissionRequests(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("add v1beta2 types to scheme: %v", err)
	}

	tests := []struct {
		name      string
		object    runtime.Object
		defaulter admission.CustomDefaulter
		validator admission.CustomValidator
	}{
		{
			name:      "OCICluster",
			object:    &OCICluster{},
			defaulter: &OCIClusterWebhook{},
			validator: &OCIClusterWebhook{},
		},
		{
			name:      "OCIManagedCluster",
			object:    &OCIManagedCluster{},
			defaulter: &OCIManagedClusterWebhook{},
			validator: &OCIManagedClusterWebhook{},
		},
		{
			name:      "OCIManagedControlPlane",
			object:    &OCIManagedControlPlane{},
			defaulter: &OCIManagedControlPlaneWebhook{},
			validator: &OCIManagedControlPlaneWebhook{},
		},
		{
			name:      "OCIMachineTemplate",
			object:    &OCIMachineTemplate{},
			validator: &OCIMachineTemplateWebhook{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := serializedWebhookObject(t, scheme, tt.object)

			if tt.defaulter != nil {
				response := admission.WithCustomDefaulter(scheme, tt.object, tt.defaulter).Handle(context.Background(), admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object:    runtime.RawExtension{Raw: raw},
					},
				})
				if !response.Allowed {
					t.Fatalf("defaulting admission request was denied: %#v", response.Result)
				}
			}

			response := admission.WithCustomValidator(scheme, tt.object, tt.validator).Handle(context.Background(), admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Object:    runtime.RawExtension{Raw: raw},
				},
			})
			if response.Result == nil || response.Result.Code == http.StatusBadRequest || response.Result.Code >= http.StatusInternalServerError {
				t.Fatalf("validating admission request was not processed: %#v", response.Result)
			}
		})
	}
}

func serializedWebhookObject(t *testing.T, scheme *runtime.Scheme, object runtime.Object) []byte {
	t.Helper()

	gvks, _, err := scheme.ObjectKinds(object)
	if err != nil {
		t.Fatalf("find object kind: %v", err)
	}
	object.GetObjectKind().SetGroupVersionKind(gvks[0])

	raw, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("serialize admission object: %v", err)
	}
	return raw
}
