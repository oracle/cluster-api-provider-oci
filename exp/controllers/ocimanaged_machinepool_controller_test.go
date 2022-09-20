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
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine/mock_containerengine"
	infrav1exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestManagedMachinePoolReconciliation(t *testing.T) {
	var (
		r        OCIManagedMachinePoolReconciler
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
	tests := []struct {
		name             string
		errorExpected    bool
		objects          []client.Object
		expectedEvent    string
		eventNotExpected string
	}{
		{
			name:          "machine pool does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret()},
		},
		{
			name:          "no owner reference",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOCIManagedMachinePoolWithNoOwner()},
			expectedEvent: "OwnerRefNotSet",
		},
		{
			name:          "cluster does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOCIManagedMachinePool(), getMachinePool()},
			expectedEvent: "ClusterDoesNotExist",
		},
		{
			name:             "paused cluster",
			errorExpected:    false,
			objects:          []client.Object{getSecret(), getOCIManagedMachinePool(), getMachinePool(), getPausedCluster()},
			eventNotExpected: "ClusterDoesNotExist",
		},
		{
			name:          "ocimanagedcluster does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOCIManagedMachinePool(), getMachinePool(), getCluster()},
			expectedEvent: "ClusterNotAvailable",
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

			client := fake.NewClientBuilder().WithObjects(tc.objects...).Build()
			r = OCIManagedMachinePoolReconciler{
				Client:         client,
				Scheme:         runtime.NewScheme(),
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

func TestNormalReconciliationFunction(t *testing.T) {
	var (
		r                     OCIManagedMachinePoolReconciler
		mockCtrl              *gomock.Controller
		recorder              *record.FakeRecorder
		ociManagedMachinePool *infrav1exp.OCIManagedMachinePool
		okeClient             *mock_containerengine.MockClient
		ms                    *scope.ManagedMachinePoolScope
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().WithObjects(getSecret()).Build()
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		machinePool := getMachinePool()
		ociManagedMachinePool = getOCIManagedMachinePool()
		ociCluster := getOCIClusterWithOwner()
		ociManagedControlPlane := infrav1exp.OCIManagedControlPlane{
			Spec: infrav1exp.OCIManagedControlPlaneSpec{
				ID: common.String("cluster-id"),
			},
			Status: infrav1exp.OCIManagedControlPlaneStatus{
				Ready: true,
			},
		}
		ms, err = scope.NewManagedMachinePoolScope(scope.ManagedMachinePoolScopeParams{
			ContainerEngineClient:  okeClient,
			OCIManagedCluster:      ociCluster,
			Cluster:                getCluster(),
			Client:                 client,
			OCIManagedMachinePool:  ociManagedMachinePool,
			MachinePool:            machinePool,
			OCIManagedControlPlane: &ociManagedControlPlane,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIManagedMachinePoolReconciler{
			Client:   client,
			Scheme:   runtime.NewScheme(),
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
		testSpecificSetup       func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient)
		expectedFailureMessages []string
	}{
		{
			name:               "node pool in creating state",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav1exp.NodePoolReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrav1exp.NodePoolNotReadyReason}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:             common.String("test"),
							LifecycleState: oke.NodePoolLifecycleStateCreating,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:               "node pool create",
			errorExpected:      false,
			expectedEvent:      "Created new Node Pool: test",
			conditionAssertion: []conditionAssertion{{infrav1exp.NodePoolReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrav1exp.NodePoolNotReadyReason}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ociManagedMachinePool.Spec.ID = nil
				okeClient.EXPECT().ListNodePools(gomock.Any(), gomock.Any()).
					Return(oke.ListNodePoolsResponse{}, nil)
				okeClient.EXPECT().CreateNodePool(gomock.Any(), gomock.Any()).
					Return(oke.CreateNodePoolResponse{OpcWorkRequestId: common.String("wr-id")}, nil)
				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Any()).
					Return(oke.GetWorkRequestResponse{WorkRequest: oke.WorkRequest{
						Resources: []oke.WorkRequestResource{
							{
								Identifier: common.String("node-pool"),
								EntityType: common.String("nodepool"),
							},
						},
					}}, nil)
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Any()).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:             common.String("test"),
							LifecycleState: oke.NodePoolLifecycleStateCreating,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:               "node pool is created, no update",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav1exp.NodePoolReadyCondition, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:                common.String("id"),
							LifecycleState:    oke.NodePoolLifecycleStateActive,
							ClusterId:         common.String("cluster-id"),
							Name:              common.String("test"),
							CompartmentId:     common.String("test-compartment"),
							KubernetesVersion: common.String("v1.24.5"),
							NodeMetadata:      map[string]string{"key1": "value1"},
							InitialNodeLabels: []oke.KeyValue{{
								Key:   common.String("key"),
								Value: common.String("value"),
							}},
							NodeShape: common.String("test-shape"),
							NodeShapeConfig: &oke.NodeShapeConfig{
								Ocpus:       common.Float32(2),
								MemoryInGBs: common.Float32(16),
							},
							NodeSourceDetails: oke.NodeSourceViaImageDetails{
								ImageId: common.String("test-image-id"),
							},
							FreeformTags: tags,
							SshPublicKey: common.String("test-ssh-public-key"),
							NodeConfigDetails: &oke.NodePoolNodeConfigDetails{
								Size: common.Int(3),
								PlacementConfigs: []oke.NodePoolPlacementConfigDetails{
									{
										AvailabilityDomain:    common.String("test-ad"),
										SubnetId:              common.String("subnet-id"),
										CapacityReservationId: common.String("cap-id"),
										FaultDomains:          []string{"fd-1", "fd-2"},
									},
								},
								NsgIds:                         []string{"nsg-id"},
								KmsKeyId:                       common.String("kms-key-id"),
								IsPvEncryptionInTransitEnabled: common.Bool(true),
								FreeformTags:                   tags,
								NodePoolPodNetworkOptionDetails: oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
									PodSubnetIds:   []string{"pod-subnet-id"},
									MaxPodsPerNode: common.Int(31),
									PodNsgIds:      []string{"pod-nsg-id"},
								},
							},
							NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
								EvictionGraceDuration:           common.String("PT30M"),
								IsForceDeleteAfterGraceDuration: common.Bool(true),
							},
						},
					}, nil)
			},
		},
		{
			name:               "node pool in created, pdate",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav1exp.NodePoolReadyCondition, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:                common.String("id"),
							LifecycleState:    oke.NodePoolLifecycleStateActive,
							ClusterId:         common.String("cluster-id"),
							Name:              common.String("test"),
							CompartmentId:     common.String("test-compartment"),
							KubernetesVersion: common.String("v1.23.5"),
							NodeMetadata:      map[string]string{"key1": "value1"},
							InitialNodeLabels: []oke.KeyValue{{
								Key:   common.String("key"),
								Value: common.String("value"),
							}},
							NodeShape: common.String("test-shape"),
							NodeShapeConfig: &oke.NodeShapeConfig{
								Ocpus:       common.Float32(2),
								MemoryInGBs: common.Float32(16),
							},
							NodeSourceDetails: oke.NodeSourceViaImageDetails{
								ImageId: common.String("test-image-id"),
							},
							FreeformTags: tags,
							SshPublicKey: common.String("test-ssh-public-key"),
							NodeConfigDetails: &oke.NodePoolNodeConfigDetails{
								Size: common.Int(3),
								PlacementConfigs: []oke.NodePoolPlacementConfigDetails{
									{
										AvailabilityDomain:    common.String("test-ad"),
										SubnetId:              common.String("subnet-id"),
										CapacityReservationId: common.String("cap-id"),
										FaultDomains:          []string{"fd-1", "fd-2"},
									},
								},
								NsgIds:                         []string{"nsg-id"},
								KmsKeyId:                       common.String("kms-key-id"),
								IsPvEncryptionInTransitEnabled: common.Bool(true),
								FreeformTags:                   tags,
								NodePoolPodNetworkOptionDetails: oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
									PodSubnetIds:   []string{"pod-subnet-id"},
									MaxPodsPerNode: common.Int(31),
									PodNsgIds:      []string{"pod-nsg-id"},
								},
							},
							NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
								EvictionGraceDuration:           common.String("PT30M"),
								IsForceDeleteAfterGraceDuration: common.Bool(true),
							},
						},
					}, nil)
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Any()).
					Return(oke.UpdateNodePoolResponse{}, nil)
			},
		},
		{
			name:                    "node pool in error state",
			errorExpected:           true,
			expectedFailureMessages: []string{"test error!", "Node Pool status FAILED is unexpected"},
			conditionAssertion:      []conditionAssertion{{infrav1exp.NodePoolReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrav1exp.NodePoolProvisionFailedReason}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:             common.String("test"),
							LifecycleState: oke.NodePoolLifecycleStateFailed,
							FreeformTags:   tags,
							Nodes: []oke.Node{
								{
									NodeError: &oke.NodeError{
										Message: common.String("test error!"),
									},
								},
							},
						},
					}, nil)
			},
		},
		{
			name:          "node pool in update state",
			errorExpected: false,
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:             common.String("test"),
							LifecycleState: oke.NodePoolLifecycleStateUpdating,
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
			_, err := r.reconcileNormal(ctx, log.FromContext(ctx), ms)
			if len(tc.conditionAssertion) > 0 {
				expectConditions(g, ociManagedMachinePool, tc.conditionAssertion)
			}
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
			if tc.expectedEvent != "" {
				g.Eventually(recorder.Events).Should(Receive(ContainSubstring(tc.expectedEvent)))
			}
			if len(tc.expectedFailureMessages) > 0 {
				g.Expect(tc.expectedFailureMessages).To(Equal(ms.OCIManagedMachinePool.Status.FailureMessages))
			}
		})
	}
}

func TestDeletionFunction(t *testing.T) {
	var (
		r                     OCIManagedMachinePoolReconciler
		mockCtrl              *gomock.Controller
		recorder              *record.FakeRecorder
		ociManagedMachinePool *infrav1exp.OCIManagedMachinePool
		okeClient             *mock_containerengine.MockClient
		ms                    *scope.ManagedMachinePoolScope
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().WithObjects(getSecret()).Build()
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		machinePool := getMachinePool()
		ociManagedMachinePool = getOCIManagedMachinePool()
		ociCluster := getOCIClusterWithOwner()
		ociManagedControlPlane := infrav1exp.OCIManagedControlPlane{
			Spec: infrav1exp.OCIManagedControlPlaneSpec{
				ID: common.String("cluster-id"),
			},
			Status: infrav1exp.OCIManagedControlPlaneStatus{
				Ready: true,
			},
		}
		ms, err = scope.NewManagedMachinePoolScope(scope.ManagedMachinePoolScopeParams{
			ContainerEngineClient:  okeClient,
			OCIManagedCluster:      ociCluster,
			Cluster:                getCluster(),
			Client:                 client,
			OCIManagedMachinePool:  ociManagedMachinePool,
			MachinePool:            machinePool,
			OCIManagedControlPlane: &ociManagedControlPlane,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIManagedMachinePoolReconciler{
			Client:   client,
			Scheme:   runtime.NewScheme(),
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
		testSpecificSetup       func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient)
		expectedFailureMessages []string
	}{
		{
			name:               "node pool to be deleted",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav1exp.NodePoolReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrav1exp.NodePoolDeletionInProgress}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:             common.String("test"),
							LifecycleState: oke.NodePoolLifecycleStateActive,
							FreeformTags:   tags,
						},
					}, nil)
				okeClient.EXPECT().DeleteNodePool(gomock.Any(), gomock.Eq(oke.DeleteNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.DeleteNodePoolResponse{}, nil)
			},
		},
		{
			name:               "node pool not found",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav1exp.NodePoolNotFoundReason, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:               "node pool ",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav1exp.NodePoolNotFoundReason, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:               "node pool deleting",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav1exp.NodePoolReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrav1exp.NodePoolDeletionInProgress}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:             common.String("test"),
							LifecycleState: oke.NodePoolLifecycleStateDeleting,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:               "node pool deleted",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav1exp.NodePoolReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrav1exp.NodePoolDeletedReason}},
			testSpecificSetup: func(machinePoolScope *scope.ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("test"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:             common.String("test"),
							LifecycleState: oke.NodePoolLifecycleStateDeleted,
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
			_, err := r.reconcileDelete(ctx, ms)
			if len(tc.conditionAssertion) > 0 {
				expectConditions(g, ociManagedMachinePool, tc.conditionAssertion)
			}
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
			if tc.expectedEvent != "" {
				g.Eventually(recorder.Events).Should(Receive(ContainSubstring(tc.expectedEvent)))
			}
			if len(tc.expectedFailureMessages) > 0 {
				g.Expect(tc.expectedFailureMessages).To(Equal(ms.OCIManagedMachinePool.Status.FailureMessages))
			}
		})
	}
}

func getOCIManagedMachinePoolWithNoOwner() *infrav1exp.OCIManagedMachinePool {
	ociMachine := getOCIManagedMachinePool()
	ociMachine.OwnerReferences = []metav1.OwnerReference{}
	return ociMachine
}

func getOCIManagedMachinePool() *infrav1exp.OCIManagedMachinePool {
	return &infrav1exp.OCIManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			UID:       "uid",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "test-cluster",
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
		Spec: infrav1exp.OCIManagedMachinePoolSpec{
			ID:           common.String("test"),
			NodeMetadata: map[string]string{"key1": "value1"},
			InitialNodeLabels: []infrav1exp.KeyValue{{
				Key:   common.String("key"),
				Value: common.String("value"),
			}},
			Version:   common.String("v1.24.5"),
			NodeShape: "test-shape",
			NodeShapeConfig: &infrav1exp.NodeShapeConfig{
				Ocpus:       common.String("2"),
				MemoryInGBs: common.String("16"),
			},
			NodeSourceViaImage: &infrav1exp.NodeSourceViaImage{
				ImageId: common.String("test-image-id"),
			},
			SshPublicKey: "test-ssh-public-key",
			NodePoolNodeConfig: &infrav1exp.NodePoolNodeConfig{
				PlacementConfigs: []infrav1exp.PlacementConfig{
					{
						AvailabilityDomain:    common.String("test-ad"),
						SubnetName:            common.String("worker-subnet"),
						CapacityReservationId: common.String("cap-id"),
						FaultDomains:          []string{"fd-1", "fd-2"},
					},
				},
				NsgNames:                       []string{"worker-nsg"},
				KmsKeyId:                       common.String("kms-key-id"),
				IsPvEncryptionInTransitEnabled: common.Bool(true),
				NodePoolPodNetworkOptionDetails: &infrav1exp.NodePoolPodNetworkOptionDetails{
					CniType: infrav1exp.VCNNativeCNI,
					VcnIpNativePodNetworkOptions: infrav1exp.VcnIpNativePodNetworkOptions{
						SubnetNames:    []string{"pod-subnet"},
						MaxPodsPerNode: common.Int(31),
						NSGNames:       []string{"pod-nsg"},
					},
				},
			},
			NodeEvictionNodePoolSettings: &infrav1exp.NodeEvictionNodePoolSettings{
				EvictionGraceDuration:           common.String("PT30M"),
				IsForceDeleteAfterGraceDuration: common.Bool(true),
			},
		},
	}
}

func getMachinePool() *expclusterv1.MachinePool {
	replicas := int32(3)
	machinePool := &expclusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: expclusterv1.MachinePoolSpec{
			Replicas: &replicas,
			Template: clusterv1.MachineTemplateSpec{},
		},
	}
	return machinePool
}

func getCluster() *clusterv1.Cluster {
	infraRef := corev1.ObjectReference{
		Name: "oci-cluster",
	}
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &infraRef,
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}
}

func getPausedCluster() *clusterv1.Cluster {
	cluster := getCluster()
	cluster.Spec.Paused = true
	return cluster
}

func expectConditions(g *WithT, m *infrav1exp.OCIManagedMachinePool, expected []conditionAssertion) {
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
