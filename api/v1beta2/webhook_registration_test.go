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

package v1beta2_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	infrastructurev1beta2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
)

func TestWebhookRegistrationServesAdmissionReviews(t *testing.T) {
	kubernetesVersion := "v1.35.0"
	invalidateMachineTemplate := func(object runtime.Object) string {
		object.(*infrastructurev1beta2.OCIMachineTemplate).Spec.Template.Spec.Shape = ""
		return "spec.template.spec.shape"
	}
	invalidateManagedMachinePool := func(object runtime.Object) string {
		version := "not-a-version"
		object.(*infrastructurev1beta2exp.OCIManagedMachinePool).Spec.Version = &version
		return "spec.version"
	}
	tests := []struct {
		name                  string
		object                runtime.Object
		kind                  string
		setup                 func(ctrl.Manager) error
		wantAdmissionWebhooks int
		defaultPath           []string
		wantDefault           interface{}
		invalidCreate         func(runtime.Object) string
		invalidUpdate         func(runtime.Object) string
	}{
		{
			name: "OCICluster",
			object: &infrastructurev1beta2.OCICluster{
				Spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId:         "ocid1.compartment.oc1..test",
					OCIResourceIdentifier: "registration-test",
				},
			},
			kind: "OCICluster",
			setup: func(mgr ctrl.Manager) error {
				return (&infrastructurev1beta2.OCICluster{}).SetupWebhookWithManager(mgr)
			},
			wantAdmissionWebhooks: 2,
			defaultPath:           []string{"spec", "networkSpec", "apiServerLoadBalancer", "networkVisibility"},
			wantDefault:           string(infrastructurev1beta2.LBNetworkVisibilityInherited),
			invalidCreate: func(object runtime.Object) string {
				object.(*infrastructurev1beta2.OCICluster).Spec.CompartmentId = "invalid"
				return "spec.compartmentId"
			},
			invalidUpdate: func(object runtime.Object) string {
				object.(*infrastructurev1beta2.OCICluster).Spec.Region = "us-ashburn-1"
				return "spec.region"
			},
		},
		{
			name: "OCIManagedCluster",
			object: &infrastructurev1beta2.OCIManagedCluster{
				Spec: infrastructurev1beta2.OCIManagedClusterSpec{
					CompartmentId:         "ocid1.compartment.oc1..test",
					OCIResourceIdentifier: "registration-test",
				},
			},
			kind: "OCIManagedCluster",
			setup: func(mgr ctrl.Manager) error {
				return (&infrastructurev1beta2.OCIManagedCluster{}).SetupWebhookWithManager(mgr)
			},
			wantAdmissionWebhooks: 2,
			defaultPath:           []string{"spec", "networkSpec", "vcn", "cidr"},
			wantDefault:           infrastructurev1beta2.VcnDefaultCidr,
			invalidCreate: func(object runtime.Object) string {
				object.(*infrastructurev1beta2.OCIManagedCluster).Spec.CompartmentId = "invalid"
				return "spec.compartmentId"
			},
			invalidUpdate: func(object runtime.Object) string {
				object.(*infrastructurev1beta2.OCIManagedCluster).Spec.Region = "us-ashburn-1"
				return "spec.region"
			},
		},
		{
			name:   "OCIManagedControlPlane",
			object: &infrastructurev1beta2.OCIManagedControlPlane{},
			kind:   "OCIManagedControlPlane",
			setup: func(mgr ctrl.Manager) error {
				return (&infrastructurev1beta2.OCIManagedControlPlane{}).SetupWebhookWithManager(mgr)
			},
			wantAdmissionWebhooks: 2,
			defaultPath:           []string{"spec", "clusterPodNetworkOptions"},
			wantDefault:           []interface{}{map[string]interface{}{"cniType": string(infrastructurev1beta2.VCNNativeCNI)}},
			invalidCreate: func(object runtime.Object) string {
				object.(metav1.Object).SetName(strings.Repeat("a", 32))
				return "Name"
			},
			// ValidateUpdate currently accepts all decoded objects.
		},
		{
			name: "OCIMachineTemplate",
			object: &infrastructurev1beta2.OCIMachineTemplate{
				Spec: infrastructurev1beta2.OCIMachineTemplateSpec{
					Template: infrastructurev1beta2.OCIMachineTemplateResource{
						Spec: infrastructurev1beta2.OCIMachineSpec{Shape: "VM.Standard.E4.Flex"},
					},
				},
			},
			kind: "OCIMachineTemplate",
			setup: func(mgr ctrl.Manager) error {
				return (&infrastructurev1beta2.OCIMachineTemplate{}).SetupWebhookWithManager(mgr)
			},
			wantAdmissionWebhooks: 1,
			invalidCreate:         invalidateMachineTemplate,
			invalidUpdate:         invalidateMachineTemplate,
		},
		{
			name: "OCIManagedMachinePool",
			object: &infrastructurev1beta2exp.OCIManagedMachinePool{
				Spec: infrastructurev1beta2exp.OCIManagedMachinePoolSpec{Version: &kubernetesVersion},
			},
			kind: "OCIManagedMachinePool",
			setup: func(mgr ctrl.Manager) error {
				return (&infrastructurev1beta2exp.OCIManagedMachinePool{}).SetupWebhookWithManager(mgr)
			},
			wantAdmissionWebhooks: 2,
			defaultPath:           []string{"spec", "nodePoolNodeConfig", "nodePoolPodNetworkOptionDetails", "cniType"},
			wantDefault:           string(infrastructurev1beta2.VCNNativeCNI),
			invalidCreate:         invalidateManagedMachinePool,
			invalidUpdate:         invalidateManagedMachinePool,
		},
		{
			name: "OCIVirtualMachinePool",
			// Create and update validators currently accept all decoded objects.
			object: &infrastructurev1beta2exp.OCIVirtualMachinePool{},
			kind:   "OCIVirtualMachinePool",
			setup: func(mgr ctrl.Manager) error {
				return (&infrastructurev1beta2exp.OCIVirtualMachinePool{}).SetupWebhookWithManager(mgr)
			},
			wantAdmissionWebhooks: 2,
			defaultPath:           []string{"spec", "podConfiguration", "shape"},
			wantDefault:           "Pod.Standard.E4.Flex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := infrastructurev1beta2.AddToScheme(scheme); err != nil {
				t.Fatalf("add infrastructure API types to scheme: %v", err)
			}
			if err := infrastructurev1beta2exp.AddToScheme(scheme); err != nil {
				t.Fatalf("add experimental infrastructure API types to scheme: %v", err)
			}

			server := newPanicPropagatingWebhookServer()
			mgr := &webhookTestManager{
				scheme:   scheme,
				server:   server,
				registry: conversion.NewRegistry(),
			}
			if err := tt.setup(mgr); err != nil {
				t.Fatalf("register production webhook: %v", err)
			}
			if len(server.admissionPaths) != tt.wantAdmissionWebhooks {
				t.Fatalf("registered admission webhooks = %d, want %d", len(server.admissionPaths), tt.wantAdmissionWebhooks)
			}

			gvk := schema.GroupVersionKind{
				Group:   infrastructurev1beta2.GroupVersion.Group,
				Version: infrastructurev1beta2.GroupVersion.Version,
				Kind:    tt.kind,
			}
			object := tt.object.DeepCopyObject()
			object.GetObjectKind().SetGroupVersionKind(gvk)
			object.(metav1.Object).SetName("registration-test")
			objectJSON, err := json.Marshal(object)
			if err != nil {
				t.Fatalf("marshal admission object: %v", err)
			}
			updatedObject := object.DeepCopyObject()
			updatedObject.(metav1.Object).SetLabels(map[string]string{"registration-test": "updated"})
			updatedJSON, err := json.Marshal(updatedObject)
			if err != nil {
				t.Fatalf("marshal updated admission object: %v", err)
			}
			malformedJSON := []byte(fmt.Sprintf(`{"apiVersion":%q,"kind":%q,"spec":"invalid"}`, gvk.GroupVersion().String(), gvk.Kind))

			for _, path := range server.admissionPaths {
				t.Run(path, func(t *testing.T) {
					type admissionCase struct {
						name       string
						operation  admissionv1.Operation
						objectJSON []byte
						wantCode   int32
						wantField  string
					}
					cases := []admissionCase{
						{name: "valid-create", operation: admissionv1.Create, objectJSON: objectJSON, wantCode: http.StatusOK},
						{name: "valid-update", operation: admissionv1.Update, objectJSON: updatedJSON, wantCode: http.StatusOK},
						{name: "malformed-create", operation: admissionv1.Create, objectJSON: malformedJSON, wantCode: http.StatusBadRequest},
						{name: "malformed-update", operation: admissionv1.Update, objectJSON: malformedJSON, wantCode: http.StatusBadRequest},
					}
					if strings.HasPrefix(path, "/validate-") {
						for _, invalid := range []struct {
							operation admissionv1.Operation
							mutate    func(runtime.Object) string
						}{
							{admissionv1.Create, tt.invalidCreate},
							{admissionv1.Update, tt.invalidUpdate},
						} {
							if invalid.mutate == nil {
								continue
							}
							invalidObject := object.DeepCopyObject()
							field := invalid.mutate(invalidObject)
							invalidJSON, err := json.Marshal(invalidObject)
							if err != nil {
								t.Fatalf("marshal invalid admission object: %v", err)
							}
							cases = append(cases, admissionCase{
								name:       "invalid-" + strings.ToLower(string(invalid.operation)),
								operation:  invalid.operation,
								objectJSON: invalidJSON,
								wantCode:   http.StatusUnprocessableEntity,
								wantField:  field,
							})
						}
					}
					for _, tc := range cases {
						t.Run(tc.name, func(t *testing.T) {
							request := admissionv1.AdmissionRequest{
								UID:       types.UID("webhook-registration-test"),
								Kind:      metav1.GroupVersionKind(gvk),
								Resource:  metav1.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: strings.ToLower(gvk.Kind) + "s"},
								Operation: tc.operation,
								Object:    runtime.RawExtension{Raw: tc.objectJSON},
							}
							if tc.operation == admissionv1.Update {
								request.OldObject = runtime.RawExtension{Raw: objectJSON}
							}
							response := serveAdmissionReview(t, server, path, request)
							wantAllowed := tc.wantCode == http.StatusOK
							if response.Allowed != wantAllowed {
								t.Fatalf("admission allowed = %t, want %t; result: %+v", response.Allowed, wantAllowed, response.Result)
							}
							if response.Result == nil || response.Result.Code != tc.wantCode {
								t.Fatalf("admission result = %+v, want code %d", response.Result, tc.wantCode)
							}
							if tc.wantField != "" {
								if response.Result.Reason != metav1.StatusReasonInvalid || response.Result.Details == nil {
									t.Fatalf("admission result = %+v, want Invalid status with field details", response.Result)
								}
								found := false
								for _, cause := range response.Result.Details.Causes {
									if cause.Field == tc.wantField {
										found = true
									}
								}
								if !found {
									t.Fatalf("admission causes = %+v, want field %q", response.Result.Details.Causes, tc.wantField)
								}
							}
							if wantAllowed && strings.HasPrefix(path, "/mutate-") {
								if response.PatchType == nil || *response.PatchType != admissionv1.PatchTypeJSONPatch {
									t.Fatalf("admission patch type = %v, want JSONPatch", response.PatchType)
								}
								patch, err := jsonpatch.DecodePatch(response.Patch)
								if err != nil {
									t.Fatalf("decode defaulting patch: %v", err)
								}
								defaultedJSON, err := patch.Apply(tc.objectJSON)
								if err != nil {
									t.Fatalf("apply defaulting patch: %v", err)
								}
								var defaulted unstructured.Unstructured
								if err := json.Unmarshal(defaultedJSON, &defaulted); err != nil {
									t.Fatalf("unmarshal defaulted object: %v", err)
								}
								got, found, err := unstructured.NestedFieldNoCopy(defaulted.Object, tt.defaultPath...)
								if err != nil || !found || !reflect.DeepEqual(got, tt.wantDefault) {
									t.Fatalf("default at %v = %#v (found %t, error %v), want %#v", tt.defaultPath, got, found, err, tt.wantDefault)
								}
							}
						})
					}
				})
			}
		})
	}
}

func serveAdmissionReview(t *testing.T, server webhook.Server, path string, request admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	t.Helper()
	review := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{APIVersion: admissionv1.SchemeGroupVersion.String(), Kind: "AdmissionReview"},
		Request:  &request,
	}
	reviewJSON, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("marshal AdmissionReview: %v", err)
	}
	httpRequest := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(reviewJSON))
	httpRequest.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.WebhookMux().ServeHTTP(response, httpRequest)
	if response.Code != http.StatusOK {
		t.Fatalf("AdmissionReview status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var responseReview admissionv1.AdmissionReview
	if err := json.Unmarshal(response.Body.Bytes(), &responseReview); err != nil {
		t.Fatalf("unmarshal AdmissionReview response: %v", err)
	}
	if responseReview.Response == nil {
		t.Fatal("AdmissionReview response is nil")
	}
	if responseReview.Response.UID != request.UID {
		t.Fatalf("AdmissionReview response UID = %q, want %q", responseReview.Response.UID, request.UID)
	}
	return responseReview.Response
}

type webhookTestManager struct {
	ctrl.Manager
	scheme   *runtime.Scheme
	server   webhook.Server
	registry conversion.Registry
}

func (m *webhookTestManager) GetConfig() *rest.Config {
	return &rest.Config{}
}

func (m *webhookTestManager) GetScheme() *runtime.Scheme {
	return m.scheme
}

func (m *webhookTestManager) GetWebhookServer() webhook.Server {
	return m.server
}

func (m *webhookTestManager) GetConverterRegistry() conversion.Registry {
	return m.registry
}

type panicPropagatingWebhookServer struct {
	webhook.Server
	admissionPaths []string
}

func newPanicPropagatingWebhookServer() *panicPropagatingWebhookServer {
	return &panicPropagatingWebhookServer{
		Server: webhook.NewServer(webhook.Options{WebhookMux: http.NewServeMux()}),
	}
}

func (s *panicPropagatingWebhookServer) Register(path string, handler http.Handler) {
	if admissionWebhook, ok := handler.(*admission.Webhook); ok {
		handler = admissionWebhook.WithRecoverPanic(false)
		s.admissionPaths = append(s.admissionPaths, path)
	}
	s.Server.Register(path, handler)
}
