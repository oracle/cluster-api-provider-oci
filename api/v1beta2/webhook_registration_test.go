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
	tests := []struct {
		name                  string
		object                runtime.Object
		kind                  string
		setup                 func(ctrl.Manager) error
		wantAdmissionWebhooks int
		defaultPath           []string
		wantDefault           interface{}
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
		},
		{
			name:   "OCIVirtualMachinePool",
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

			for _, path := range server.admissionPaths {
				t.Run(path, func(t *testing.T) {
					uid := types.UID("webhook-registration-test")
					review := admissionv1.AdmissionReview{
						TypeMeta: metav1.TypeMeta{APIVersion: admissionv1.SchemeGroupVersion.String(), Kind: "AdmissionReview"},
						Request: &admissionv1.AdmissionRequest{
							UID:       uid,
							Kind:      metav1.GroupVersionKind(gvk),
							Resource:  metav1.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: strings.ToLower(gvk.Kind) + "s"},
							Operation: admissionv1.Create,
							Object:    runtime.RawExtension{Raw: objectJSON},
							OldObject: runtime.RawExtension{Raw: objectJSON},
						},
					}
					reviewJSON, err := json.Marshal(review)
					if err != nil {
						t.Fatalf("marshal AdmissionReview: %v", err)
					}

					request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(reviewJSON))
					request.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					server.WebhookMux().ServeHTTP(response, request)

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
					if responseReview.Response.UID != uid {
						t.Fatalf("AdmissionReview response UID = %q, want %q", responseReview.Response.UID, uid)
					}
					if !responseReview.Response.Allowed {
						t.Fatalf("admission denied: %+v", responseReview.Response.Result)
					}
					if responseReview.Response.Result == nil || responseReview.Response.Result.Code != http.StatusOK {
						t.Fatalf("admission result = %+v, want code %d", responseReview.Response.Result, http.StatusOK)
					}
					if strings.HasPrefix(path, "/mutate-") {
						if responseReview.Response.PatchType == nil || *responseReview.Response.PatchType != admissionv1.PatchTypeJSONPatch {
							t.Fatalf("admission patch type = %v, want JSONPatch", responseReview.Response.PatchType)
						}
						patch, err := jsonpatch.DecodePatch(responseReview.Response.Patch)
						if err != nil {
							t.Fatalf("decode defaulting patch: %v", err)
						}
						defaultedJSON, err := patch.Apply(objectJSON)
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
