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

package controllers

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/base/mock_base"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine/mock_containerengine"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)


func TestControlPlaneReconciliation(t *testing.T) {
	var (
		r        OCIManagedClusterControlPlaneReconciler
		mockCtrl *gomock.Controller
		req      reconcile.Request
		recorder *record.FakeRecorder
	)

	setup := func(t *testing.T, g *WithT) {
		mockCtrl = gomock.NewController(t)
		recorder = record.NewFakeRecorder(2)
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}
	notReadyCluster := &infrastructurev1beta2.OCIManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oci-cluster",
			Namespace: "test",
		},
		Status: infrastructurev1beta2.OCIManagedClusterStatus{
			Ready: false,
		},
	}
	tests := []struct {
		name             string
		errorExpected    bool
		objects          []client.Object
		expectedEvent    string
		eventNotExpected string
	}{
		{
			name:          "control plane does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret()},
		},
		{
			name:          "no owner reference",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getControlPlanePoolWithNoOwner()},
			expectedEvent: "OwnerRefNotSet",
		},
		{
			name:          "cluster does not exist",
			errorExpected: true,
			objects:       []client.Object{getSecret(), getOCIManagedControlPlane()},
		},
		{
			name:             "paused cluster",
			errorExpected:    false,
			objects:          []client.Object{getSecret(), getOCIManagedControlPlane(), getPausedCluster()},
			eventNotExpected: "ClusterDoesNotExist",
		},
		{
			name:          "oci managedcluster does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOCIManagedControlPlane(), getCluster()},
			expectedEvent: "ClusterNotAvailable",
		},
		{
			name:          "oci managedcluster is not ready",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOCIManagedControlPlane(), getCluster(), notReadyCluster},
			expectedEvent: "ClusterInfrastructureNotReady",
		},
	}

	clientProvider, err := scope.MockNewClientProvider(scope.MockOCIClients{})
	if err != nil {
		t.Error(err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)

			client := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(tc.objects...).Build()
			r = OCIManagedClusterControlPlaneReconciler{
				Client:         client,
				Scheme:         setupScheme(),
				Recorder:       recorder,
				ClientProvider: clientProvider,
				Region:         MockTestRegion,
			}
			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "test",
					Name:      "test",
				},
			}

			_, err := r.Reconcile(context.Background(), req)
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
			if tc.expectedEvent != "" {
				g.Eventually(recorder.Events).Should(Receive(ContainSubstring(tc.expectedEvent)))
			}
			if tc.eventNotExpected != "" {
				g.Eventually(recorder.Events).ShouldNot(Receive(ContainSubstring(tc.eventNotExpected)))
			}
		})
	}
}

func TestControlPlaneReconciliationFunction(t *testing.T) {
	var (
		r                      OCIManagedClusterControlPlaneReconciler
		mockCtrl               *gomock.Controller
		recorder               *record.FakeRecorder
		ociManagedControlPlane *infrastructurev1beta2.OCIManagedControlPlane
		okeClient              *mock_containerengine.MockClient
		ms                     *scope.ManagedControlPlaneScope
		baseClient             *mock_base.MockBaseClient
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

	clientConfig := api.Config{
		Clusters: map[string]*api.Cluster{
			"test": {
				Server:                   "http://localhost:6443",
				CertificateAuthorityData: []byte{},
			},
		},
		Contexts: map[string]*api.Context{
			"test-context": {
				Cluster: "test",
			},
		},
		CurrentContext: "test-context",
	}
	config, _ := clientcmd.Write(clientConfig)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(getSecret(), getBootstrapSecret()).Build()
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		baseClient = mock_base.NewMockBaseClient(mockCtrl)
		ociManagedControlPlane = getOCIManagedControlPlane()
		ociCluster := getOCIManagedClusterWithOwner()
		ociClusterAccess := scope.OCIManagedCluster{
			OCIManagedCluster: ociCluster,
		}
		ms, err = scope.NewManagedControlPlaneScope(scope.ManagedControlPlaneScopeParams{
			ContainerEngineClient:  okeClient,
			OCIClusterAccessor:     ociClusterAccess,
			Cluster:                getCluster(),
			Client:                 client,
			OCIManagedControlPlane: ociManagedControlPlane,
			BaseClient:             baseClient,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIManagedClusterControlPlaneReconciler{
			Client:   client,
			Scheme:   setupScheme(),
			Recorder: recorder,
		}
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}
	tests := []struct {
		name                    string
		errorExpected           bool
		expectedEvent           string
		eventNotExpected        string
		conditionAssertion      []conditionAssertion
		testSpecificSetup       func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient)
		expectedFailureMessages []string
	}{
		{
			name:               "control plane in creating state",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.ControlPlaneReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrastructurev1beta2.ControlPlaneNotReadyReason}},
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:             common.String("test"),
							LifecycleState: oke.ClusterLifecycleStateCreating,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:               "control plane create",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.ControlPlaneReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrastructurev1beta2.ControlPlaneNotReadyReason}},
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				ociManagedControlPlane.Spec.ID = nil
				okeClient.EXPECT().ListClusters(gomock.Any(), gomock.Any()).
					Return(oke.ListClustersResponse{}, nil)
				okeClient.EXPECT().CreateCluster(gomock.Any(), gomock.Any()).
					Return(oke.CreateClusterResponse{OpcWorkRequestId: common.String("wr-id")}, nil)
				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Any()).
					Return(oke.GetWorkRequestResponse{WorkRequest: oke.WorkRequest{
						Resources: []oke.WorkRequestResource{
							{
								Identifier: common.String("cluster"),
								EntityType: common.String("cluster"),
							},
						},
					}}, nil)
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Any()).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:             common.String("cluster"),
							LifecycleState: oke.ClusterLifecycleStateCreating,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:               "control plane is created, no update",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.ControlPlaneReadyCondition, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:                common.String("test"),
							Name:              common.String("test"),
							Type:              oke.ClusterTypeBasicCluster,
							CompartmentId:     common.String("test-compartment"),
							VcnId:             common.String("vcn-id"),
							KubernetesVersion: common.String("v1.24.5"),
							FreeformTags:      tags,
							LifecycleState:    oke.ClusterLifecycleStateActive,
							EndpointConfig: &oke.ClusterEndpointConfig{
								SubnetId:          common.String("subnet-id"),
								NsgIds:            []string{"nsg-id"},
								IsPublicIpEnabled: common.Bool(true),
							},
							Endpoints: &oke.ClusterEndpoints{
								PublicEndpoint:  common.String("public"),
								PrivateEndpoint: common.String("private"),
							},
							ClusterPodNetworkOptions: []oke.ClusterPodNetworkOptionDetails{
								oke.FlannelOverlayClusterPodNetworkOptionDetails{},
							},
							Options: &oke.ClusterCreateOptions{
								ServiceLbSubnetIds: []string{"lb-subnet-id"},
								KubernetesNetworkConfig: &oke.KubernetesNetworkConfig{
									PodsCidr:     common.String("1.2.3.4/5"),
									ServicesCidr: common.String("5.6.7.8/9"),
								},
								AddOns: &oke.AddOnOptions{
									IsKubernetesDashboardEnabled: common.Bool(true),
									IsTillerEnabled:              common.Bool(false),
								},
								AdmissionControllerOptions: &oke.AdmissionControllerOptions{
									IsPodSecurityPolicyEnabled: common.Bool(true),
								},
								PersistentVolumeConfig: &oke.PersistentVolumeConfigDetails{
									FreeformTags: tags,
								},
								ServiceLbConfig: &oke.ServiceLbConfigDetails{
									FreeformTags: tags,
								},
							},
							ImagePolicyConfig: &oke.ImagePolicyConfig{
								IsPolicyEnabled: common.Bool(true),
								KeyDetails: []oke.KeyDetails{{
									KmsKeyId: common.String("kms-key-id"),
								}},
							},
							KmsKeyId: common.String("etcd-kms-key-id"),
						},
					}, nil)

				okeClient.EXPECT().CreateKubeconfig(gomock.Any(), gomock.Eq(oke.CreateKubeconfigRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.CreateKubeconfigResponse{
						Content: io.NopCloser(strings.NewReader(string(config))),
					}, nil)
				baseClient.EXPECT().GenerateToken(gomock.Any(), gomock.Eq("test")).
					Return("secret-token", nil)
			},
		},
		{
			name:               "control plane in created, update",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.ControlPlaneReadyCondition, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Name:              common.String("test"),
							Id:                common.String("test"),
							CompartmentId:     common.String("test-compartment"),
							VcnId:             common.String("vcn-id"),
							KubernetesVersion: common.String("v1.23.5"),
							FreeformTags:      tags,
							LifecycleState:    oke.ClusterLifecycleStateActive,
							EndpointConfig: &oke.ClusterEndpointConfig{
								SubnetId:          common.String("subnet-id"),
								NsgIds:            []string{"nsg-id"},
								IsPublicIpEnabled: common.Bool(true),
							},
							Endpoints: &oke.ClusterEndpoints{
								PublicEndpoint:  common.String("public"),
								PrivateEndpoint: common.String("private"),
							},
							ClusterPodNetworkOptions: []oke.ClusterPodNetworkOptionDetails{
								oke.FlannelOverlayClusterPodNetworkOptionDetails{},
							},
							Options: &oke.ClusterCreateOptions{
								ServiceLbSubnetIds: []string{"lb-subnet-id"},
								KubernetesNetworkConfig: &oke.KubernetesNetworkConfig{
									PodsCidr:     common.String("1.2.3.4/5"),
									ServicesCidr: common.String("5.6.7.8/9"),
								},
								AddOns: &oke.AddOnOptions{
									IsKubernetesDashboardEnabled: common.Bool(true),
									IsTillerEnabled:              common.Bool(false),
								},
								AdmissionControllerOptions: &oke.AdmissionControllerOptions{
									IsPodSecurityPolicyEnabled: common.Bool(true),
								},
								PersistentVolumeConfig: &oke.PersistentVolumeConfigDetails{
									FreeformTags: tags,
								},
								ServiceLbConfig: &oke.ServiceLbConfigDetails{
									FreeformTags: tags,
								},
							},
							ImagePolicyConfig: &oke.ImagePolicyConfig{
								IsPolicyEnabled: common.Bool(true),
								KeyDetails: []oke.KeyDetails{{
									KmsKeyId: common.String("kms-key-id"),
								}},
							},
							KmsKeyId: common.String("etcd-kms-key-id"),
						},
					}, nil)
				okeClient.EXPECT().UpdateCluster(gomock.Any(), gomock.Any()).
					Return(oke.UpdateClusterResponse{}, nil)

				okeClient.EXPECT().CreateKubeconfig(gomock.Any(), gomock.Eq(oke.CreateKubeconfigRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.CreateKubeconfigResponse{
						Content: io.NopCloser(strings.NewReader(string(config))),
					}, nil)
				baseClient.EXPECT().GenerateToken(gomock.Any(), gomock.Eq("test")).
					Return("secret-token", nil)
			},
		},
		{
			name:               "control plane in error state",
			errorExpected:      true,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.ControlPlaneReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.ControlPlaneProvisionFailedReason}},
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:             common.String("test"),
							LifecycleState: oke.ClusterLifecycleStateFailed,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:          "control plane in update state",
			errorExpected: false,
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:             common.String("test"),
							LifecycleState: oke.ClusterLifecycleStateUpdating,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, okeClient)
			ctx := context.Background()
			_, err := r.reconcile(ctx, ms, ociManagedControlPlane)
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
			if tc.expectedEvent != "" {
				g.Eventually(recorder.Events).Should(Receive(ContainSubstring(tc.expectedEvent)))
			}
			if len(tc.conditionAssertion) > 0 {
				expectControlPlaneConditions(g, ociManagedControlPlane, tc.conditionAssertion)
			}
		})
	}
}

func TestControlPlaneDeletionFunction(t *testing.T) {
	var (
		r                      OCIManagedClusterControlPlaneReconciler
		mockCtrl               *gomock.Controller
		recorder               *record.FakeRecorder
		ociManagedControlPlane *infrastructurev1beta2.OCIManagedControlPlane
		okeClient              *mock_containerengine.MockClient
		ms                     *scope.ManagedControlPlaneScope
		baseClient             *mock_base.MockBaseClient
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(getSecret()).Build()
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		baseClient = mock_base.NewMockBaseClient(mockCtrl)
		ociManagedControlPlane = getOCIManagedControlPlane()
		ociCluster := getOCIManagedClusterWithOwner()
		ociClusterAccess := scope.OCIManagedCluster{
			OCIManagedCluster: ociCluster,
		}
		ms, err = scope.NewManagedControlPlaneScope(scope.ManagedControlPlaneScopeParams{
			ContainerEngineClient:  okeClient,
			OCIClusterAccessor:     ociClusterAccess,
			Cluster:                getCluster(),
			Client:                 client,
			OCIManagedControlPlane: ociManagedControlPlane,
			BaseClient:             baseClient,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIManagedClusterControlPlaneReconciler{
			Client:   client,
			Scheme:   setupScheme(),
			Recorder: recorder,
		}
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}
	tests := []struct {
		name                    string
		errorExpected           bool
		expectedEvent           string
		eventNotExpected        string
		conditionAssertion      []conditionAssertion
		testSpecificSetup       func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient)
		expectedFailureMessages []string
	}{
		{
			name:               "control plane to be deleted",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.ControlPlaneReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrastructurev1beta2.ControlPlaneDeletionInProgress}},
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:             common.String("test"),
							LifecycleState: oke.ClusterLifecycleStateActive,
							FreeformTags:   tags,
						},
					}, nil)
				okeClient.EXPECT().DeleteCluster(gomock.Any(), gomock.Eq(oke.DeleteClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.DeleteClusterResponse{}, nil)
			},
		},
		{
			name:               "control plane not found",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.ControlPlaneNotFoundReason, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:               "control plane deleting",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.ControlPlaneReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrastructurev1beta2.ControlPlaneDeletionInProgress}},
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:             common.String("test"),
							LifecycleState: oke.ClusterLifecycleStateDeleting,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:               "control plane deleted",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.ControlPlaneReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrastructurev1beta2.ControlPlaneDeletedReason}},
			testSpecificSetup: func(controlPlaneScope *scope.ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:             common.String("test"),
							LifecycleState: oke.ClusterLifecycleStateDeleted,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, okeClient)
			ctx := context.Background()
			_, err := r.reconcileDelete(ctx, ms, ociManagedControlPlane)
			if len(tc.conditionAssertion) > 0 {
				expectControlPlaneConditions(g, ociManagedControlPlane, tc.conditionAssertion)
			}
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
			if tc.expectedEvent != "" {
				g.Eventually(recorder.Events).Should(Receive(ContainSubstring(tc.expectedEvent)))
			}
		})
	}
}

func getControlPlanePoolWithNoOwner() *infrastructurev1beta2.OCIManagedControlPlane {
	ociControlplane := getOCIManagedControlPlane()
	ociControlplane.OwnerReferences = []metav1.OwnerReference{}
	return ociControlplane
}

func getOCIManagedControlPlane() *infrastructurev1beta2.OCIManagedControlPlane {
	return &infrastructurev1beta2.OCIManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			UID:       "uid",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "test-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "test-cluster",
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
				{
					Name:       "test",
					Kind:       "MachinePool",
					APIVersion: expclusterv1.GroupVersion.String(),
				},
			},
		},
		Spec: infrastructurev1beta2.OCIManagedControlPlaneSpec{
			ID: common.String("test"),
			ClusterPodNetworkOptions: []infrastructurev1beta2.ClusterPodNetworkOptions{
				{
					CniType: infrastructurev1beta2.FlannelCNI,
				},
			},
			ImagePolicyConfig: &infrastructurev1beta2.ImagePolicyConfig{
				IsPolicyEnabled: common.Bool(true),
				KeyDetails: []infrastructurev1beta2.KeyDetails{{
					KmsKeyId: common.String("kms-key-id"),
				}},
			},
			ClusterOption: infrastructurev1beta2.ClusterOptions{
				AdmissionControllerOptions: &infrastructurev1beta2.AdmissionControllerOptions{
					IsPodSecurityPolicyEnabled: common.Bool(true),
				},
				AddOnOptions: &infrastructurev1beta2.AddOnOptions{
					IsKubernetesDashboardEnabled: common.Bool(true),
					IsTillerEnabled:              common.Bool(false),
				},
			},
			KmsKeyId: common.String("etcd-kms-key-id"),
			Version:  common.String("v1.24.5"),
		},
	}
}

func expectControlPlaneConditions(g *WithT, m *infrastructurev1beta2.OCIManagedControlPlane, expected []conditionAssertion) {
	g.Expect(len(m.Status.Conditions)).To(BeNumerically(">=", len(expected)), "number of conditions")
	for _, c := range expected {
		actual := conditions.Get(m, c.conditionType)
		g.Expect(actual).To(Not(BeNil()))
		g.Expect(actual.Type).To(Equal(c.conditionType))
		g.Expect(actual.Status).To(Equal(c.status))
		g.Expect(actual.Severity).To(Equal(c.severity))
		g.Expect(actual.Reason).To(Equal(c.reason))
	}
}

func getBootstrapSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-self-managed",
			Namespace: "test",
		},
		Data: map[string][]byte{
			"value": []byte("test"),
		},
	}
}
