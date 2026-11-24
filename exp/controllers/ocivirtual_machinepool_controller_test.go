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
	infrastructurev1beta3 "github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine/mock_containerengine"
	infrav3exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta3"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestVirtualMachinePoolReconciliation(t *testing.T) {
	var (
		r        OCIVirtualMachinePoolReconciler
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
			objects:       []client.Object{getSecret(), getOCIVirtualMachinePoolWithNoOwner()},
			expectedEvent: "OwnerRefNotSet",
		},
		{
			name:          "cluster does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOCIVirtualMachinePool(), getMachinePool()},
			expectedEvent: "ClusterDoesNotExist",
		},
		{
			name:             "paused cluster",
			errorExpected:    false,
			objects:          []client.Object{getSecret(), getOCIVirtualMachinePool(), getMachinePool(), getPausedCluster()},
			eventNotExpected: "ClusterDoesNotExist",
		},
		{
			name:          "ocimanagedcluster does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOCIVirtualMachinePool(), getMachinePool(), getCluster()},
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

			client := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(tc.objects...).Build()
			r = OCIVirtualMachinePoolReconciler{
				Client:         client,
				Scheme:         testScheme(),
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

func TestNormalReconciliationFunctionForVirtualMP(t *testing.T) {
	var (
		r                     OCIVirtualMachinePoolReconciler
		mockCtrl              *gomock.Controller
		recorder              *record.FakeRecorder
		ociVirtualMachinePool *infrav3exp.OCIVirtualMachinePool
		okeClient             *mock_containerengine.MockClient
		ms                    *scope.VirtualMachinePoolScope
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(getSecret()).Build()
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		machinePool := getMachinePool()
		ociVirtualMachinePool = getOCIVirtualMachinePool()
		ociCluster := getOCIManagedClusterWithOwner()
		ociManagedControlPlane := infrastructurev1beta3.OCIManagedControlPlane{
			Spec: infrastructurev1beta3.OCIManagedControlPlaneSpec{
				ID: common.String("cluster-id"),
			},
			Status: infrastructurev1beta3.OCIManagedControlPlaneStatus{
				Initialization: infrastructurev1beta3.OCIManagedControlPlaneInitializationStatus{
					ControlPlaneInitialized: common.Bool(true),
				},
			},
		}
		ms, err = scope.NewVirtualMachinePoolScope(scope.VirtualMachinePoolScopeParams{
			ContainerEngineClient:  okeClient,
			OCIManagedCluster:      ociCluster,
			Cluster:                getCluster(),
			Client:                 client,
			OCIVirtualMachinePool:  ociVirtualMachinePool,
			MachinePool:            machinePool,
			OCIManagedControlPlane: &ociManagedControlPlane,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIVirtualMachinePoolReconciler{
			Client:   client,
			Scheme:   testScheme(),
			Recorder: recorder,
		}
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}
	type test struct {
		name                    string
		errorExpected           bool
		expectedEvent           string
		eventNotExpected        string
		conditionAssertion      []conditionAssertion
		testSpecificSetup       func(t *test, machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient)
		expectedFailureMessages []string
		createPoolMachines      []infrav3exp.OCIMachinePoolMachine
		deletePoolMachines      []clusterv1.Machine
		validate                func(g *WithT, t *test)
	}
	tests := []test{
		{
			name:               "virtual node pool in creating state",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionFalse, infrav3exp.VirtualNodePoolNotReadyReason}},
			testSpecificSetup: func(t *test, machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:             common.String("test"),
							LifecycleState: oke.VirtualNodePoolLifecycleStateCreating,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:               "virtual node pool create",
			errorExpected:      false,
			expectedEvent:      "Created new Virtual Node Pool: test",
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionFalse, infrav3exp.VirtualNodePoolNotReadyReason}},
			testSpecificSetup: func(t *test, machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ociVirtualMachinePool.Spec.ID = nil
				okeClient.EXPECT().ListVirtualNodePools(gomock.Any(), gomock.Any()).
					Return(oke.ListVirtualNodePoolsResponse{}, nil)
				okeClient.EXPECT().CreateVirtualNodePool(gomock.Any(), gomock.Any()).
					Return(oke.CreateVirtualNodePoolResponse{OpcWorkRequestId: common.String("wr-id")}, nil)
				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Any()).
					Return(oke.GetWorkRequestResponse{WorkRequest: oke.WorkRequest{
						Resources: []oke.WorkRequestResource{
							{
								Identifier: common.String("virtual-node-pool"),
								EntityType: common.String("virtualnodepool"),
							},
						},
					}}, nil)
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Any()).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:             common.String("test"),
							LifecycleState: oke.VirtualNodePoolLifecycleStateCreating,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:               "virtual node pool is created, no update",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionTrue, ""}},
			testSpecificSetup: func(t *test, machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				r.Client = interceptor.NewClient(fake.NewClientBuilder().WithScheme(testScheme()).WithStatusSubresource(&infrav3exp.OCIMachinePoolMachine{}).WithObjects(getSecret(), ociVirtualMachinePool).Build(), interceptor.Funcs{
					Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
						if err := client.Create(ctx, obj, opts...); err != nil {
							return err
						}
						m := obj.(*infrav3exp.OCIMachinePoolMachine)
						t.createPoolMachines = append(t.createPoolMachines, *m)
						return nil
					},
				})
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:                common.String("id"),
							LifecycleState:    oke.VirtualNodePoolLifecycleStateActive,
							ClusterId:         common.String("cluster-id"),
							DisplayName:       common.String("test"),
							CompartmentId:     common.String("test-compartment"),
							KubernetesVersion: common.String("v1.24.5"),
							InitialVirtualNodeLabels: []oke.InitialVirtualNodeLabel{{
								Key:   common.String("key"),
								Value: common.String("value"),
							}},
							Taints: []oke.Taint{{
								Key:    common.String("key"),
								Value:  common.String("value"),
								Effect: common.String("effect"),
							}},
							PlacementConfigurations: []oke.PlacementConfiguration{
								{
									AvailabilityDomain: common.String("test-ad"),
									SubnetId:           common.String("subnet-id"),
									FaultDomain:        []string{"fd-1", "fd-2"},
								},
							},
							NsgIds: []string{"nsg-id"},
							PodConfiguration: &oke.PodConfiguration{
								NsgIds:   []string{"pod-nsg-id"},
								Shape:    common.String("pod-shape"),
								SubnetId: common.String("pod-subnet-id"),
							},
							Size:         common.Int(3),
							FreeformTags: tags,
						},
					}, nil)

				okeClient.EXPECT().ListVirtualNodes(gomock.Any(), gomock.Eq(oke.ListVirtualNodesRequest{
					VirtualNodePoolId: common.String("id"),
				})).Return(oke.ListVirtualNodesResponse{
					Items: []oke.VirtualNodeSummary{
						{
							Id:             common.String("id-1"),
							LifecycleState: oke.VirtualNodeLifecycleStateActive,
							DisplayName:    common.String("name-1"),
						},
					},
				}, nil)
			},
			validate: func(g *WithT, t *test) {
				g.Expect(len(t.createPoolMachines)).To(Equal(1))
				machine := t.createPoolMachines[0]
				g.Expect(machine.Spec.MachineType).To(Equal(infrav3exp.Virtual))
				g.Expect(*machine.Spec.InstanceName).To(Equal("name-1"))
				g.Expect(*machine.Spec.ProviderID).To(Equal("id-1"))
				g.Expect(*machine.Spec.OCID).To(Equal("id-1"))

				g.Expect(ms.OCIVirtualMachinePool.Status.NodepoolLifecycleState).To(Equal("ACTIVE"))
			},
		},
		{
			name:               "delete unwanted machinepool machine",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionTrue, ""}},
			testSpecificSetup: func(t *test, machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				fakeClient := fake.NewClientBuilder().WithScheme(testScheme()).WithStatusSubresource(&infrav3exp.OCIMachinePoolMachine{}).WithObjects(&infrav3exp.OCIMachinePoolMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:     "test-cluster",
							clusterv1.MachinePoolNameLabel: "test",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "Machine",
								Name:       "test",
								APIVersion: clusterv1.GroupVersion.String(),
							},
						},
					},
					Spec: infrav3exp.OCIMachinePoolMachineSpec{
						OCID:         common.String("id-2"),
						InstanceName: common.String("name-2"),
						ProviderID:   common.String("id-2"),
						MachineType:  infrav3exp.Managed,
					},
				}, &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:     "test-cluster",
							clusterv1.MachinePoolNameLabel: "test",
						},
					},
					Spec: clusterv1.MachineSpec{},
				}).Build()
				r.Client = interceptor.NewClient(fakeClient, interceptor.Funcs{
					Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
						if err := client.Create(ctx, obj, opts...); err != nil {
							return err
						}
						m := obj.(*infrav3exp.OCIMachinePoolMachine)
						t.createPoolMachines = append(t.createPoolMachines, *m)
						return nil
					},
					Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						m := obj.(*clusterv1.Machine)
						t.deletePoolMachines = append(t.deletePoolMachines, *m)
						return nil
					},
				})
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:                common.String("id"),
							LifecycleState:    oke.VirtualNodePoolLifecycleStateActive,
							ClusterId:         common.String("cluster-id"),
							DisplayName:       common.String("test"),
							CompartmentId:     common.String("test-compartment"),
							KubernetesVersion: common.String("v1.24.5"),
							InitialVirtualNodeLabels: []oke.InitialVirtualNodeLabel{{
								Key:   common.String("key"),
								Value: common.String("value"),
							}},
							Taints: []oke.Taint{{
								Key:    common.String("key"),
								Value:  common.String("value"),
								Effect: common.String("effect"),
							}},
							PlacementConfigurations: []oke.PlacementConfiguration{
								{
									AvailabilityDomain: common.String("test-ad"),
									SubnetId:           common.String("subnet-id"),
									FaultDomain:        []string{"fd-1", "fd-2"},
								},
							},
							NsgIds: []string{"nsg-id"},
							PodConfiguration: &oke.PodConfiguration{
								NsgIds:   []string{"pod-nsg-id"},
								Shape:    common.String("pod-shape"),
								SubnetId: common.String("pod-subnet-id"),
							},
							Size:         common.Int(3),
							FreeformTags: tags,
						},
					}, nil)

				okeClient.EXPECT().ListVirtualNodes(gomock.Any(), gomock.Eq(oke.ListVirtualNodesRequest{
					VirtualNodePoolId: common.String("id"),
				})).Return(oke.ListVirtualNodesResponse{
					Items: []oke.VirtualNodeSummary{
						{
							Id:             common.String("id-1"),
							LifecycleState: oke.VirtualNodeLifecycleStateActive,
							DisplayName:    common.String("name-1"),
						},
					},
				}, nil)
			},
			validate: func(g *WithT, t *test) {
				g.Expect(len(t.createPoolMachines)).To(Equal(1))
				machine := t.createPoolMachines[0]
				g.Expect(machine.Spec.MachineType).To(Equal(infrav3exp.Virtual))
				g.Expect(*machine.Spec.InstanceName).To(Equal("name-1"))
				g.Expect(*machine.Spec.ProviderID).To(Equal("id-1"))
				g.Expect(*machine.Spec.OCID).To(Equal("id-1"))

				g.Expect(len(t.deletePoolMachines)).To(Equal(1))
				deleteMachine := t.deletePoolMachines[0]
				g.Expect(deleteMachine.Name).To(Equal("test"))
			},
		},
		{
			name:               "virtual node pool in created, update",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionTrue, ""}},
			testSpecificSetup: func(t *test, machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:                common.String("id"),
							LifecycleState:    oke.VirtualNodePoolLifecycleStateActive,
							ClusterId:         common.String("cluster-id"),
							DisplayName:       common.String("test"),
							CompartmentId:     common.String("test-compartment"),
							KubernetesVersion: common.String("v1.23.5"),
							InitialVirtualNodeLabels: []oke.InitialVirtualNodeLabel{{
								Key:   common.String("key"),
								Value: common.String("value"),
							}},
							Taints: []oke.Taint{{
								Key:    common.String("key"),
								Value:  common.String("value"),
								Effect: common.String("effect"),
							}},
							PlacementConfigurations: []oke.PlacementConfiguration{
								{
									AvailabilityDomain: common.String("test-ad"),
									SubnetId:           common.String("subnet-id"),
									FaultDomain:        []string{"fd-1", "fd-2"},
								},
							},
							PodConfiguration: &oke.PodConfiguration{
								NsgIds:   []string{"pod-nsg-id"},
								Shape:    common.String("pod-shape"),
								SubnetId: common.String("pod-subnet-id"),
							},
							Size:         common.Int(3),
							FreeformTags: tags,
						},
					}, nil)
				okeClient.EXPECT().ListVirtualNodes(gomock.Any(), gomock.Eq(oke.ListVirtualNodesRequest{
					VirtualNodePoolId: common.String("id"),
				})).Return(oke.ListVirtualNodesResponse{}, nil)
				okeClient.EXPECT().UpdateVirtualNodePool(gomock.Any(), gomock.Any()).
					Return(oke.UpdateVirtualNodePoolResponse{}, nil)
			},
		},
		{
			name:                    "virtual node pool in error state",
			errorExpected:           true,
			expectedFailureMessages: []string{"Virtual Node Pool status FAILED is unexpected"},
			conditionAssertion:      []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionFalse, infrav3exp.VirtualNodePoolProvisionFailedReason}},
			testSpecificSetup: func(t *test, machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:             common.String("test"),
							LifecycleState: oke.VirtualNodePoolLifecycleStateFailed,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:          "virtual node pool in update state",
			errorExpected: false,
			testSpecificSetup: func(t *test, machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:             common.String("test"),
							LifecycleState: oke.VirtualNodePoolLifecycleStateUpdating,
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
			tc.testSpecificSetup(&tc, ms, okeClient)
			ctx := context.Background()
			_, err := r.reconcileNormal(ctx, log.FromContext(ctx), ms)
			if len(tc.conditionAssertion) > 0 {
				expectVMPConditions(g, ociVirtualMachinePool, tc.conditionAssertion)
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
				g.Expect(tc.expectedFailureMessages).To(Equal(ms.OCIVirtualMachinePool.Status.FailureMessages))
			}
			if tc.validate != nil {
				tc.validate(g, &tc)
			}
		})
	}
}

func TestVMPDeletionFunction(t *testing.T) {
	var (
		r                     OCIVirtualMachinePoolReconciler
		mockCtrl              *gomock.Controller
		recorder              *record.FakeRecorder
		ociVirtualMachinePool *infrav3exp.OCIVirtualMachinePool
		okeClient             *mock_containerengine.MockClient
		ms                    *scope.VirtualMachinePoolScope
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(getSecret()).Build()
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		machinePool := getMachinePool()
		ociVirtualMachinePool = getOCIVirtualMachinePool()
		ociCluster := getOCIManagedClusterWithOwner()
		ociManagedControlPlane := infrastructurev1beta3.OCIManagedControlPlane{
			Spec: infrastructurev1beta3.OCIManagedControlPlaneSpec{
				ID: common.String("cluster-id"),
			},
			Status: infrastructurev1beta3.OCIManagedControlPlaneStatus{
				Initialization: infrastructurev1beta3.OCIManagedControlPlaneInitializationStatus{
					ControlPlaneInitialized: common.Bool(true),
				},
			},
		}
		ms, err = scope.NewVirtualMachinePoolScope(scope.VirtualMachinePoolScopeParams{
			ContainerEngineClient:  okeClient,
			OCIManagedCluster:      ociCluster,
			Cluster:                getCluster(),
			Client:                 client,
			OCIVirtualMachinePool:  ociVirtualMachinePool,
			MachinePool:            machinePool,
			OCIManagedControlPlane: &ociManagedControlPlane,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIVirtualMachinePoolReconciler{
			Client:   client,
			Scheme:   testScheme(),
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
		testSpecificSetup       func(machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient)
		expectedFailureMessages []string
	}{
		{
			name:               "virtual node pool to be deleted",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionFalse, infrav3exp.VirtualNodePoolDeletionInProgress}},
			testSpecificSetup: func(machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:             common.String("test"),
							LifecycleState: oke.VirtualNodePoolLifecycleStateActive,
							FreeformTags:   tags,
						},
					}, nil)
				okeClient.EXPECT().DeleteVirtualNodePool(gomock.Any(), gomock.Eq(oke.DeleteVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.DeleteVirtualNodePoolResponse{}, nil)
			},
		},
		{
			name:               "virtual node pool not found",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionFalse, infrav3exp.VirtualNodePoolNotFoundReason}},
			testSpecificSetup: func(machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:               "virtual node pool",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionFalse, infrav3exp.VirtualNodePoolNotFoundReason}},
			testSpecificSetup: func(machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{}, ociutil.ErrNotFound)
			},
		},
		{
			name:               "virtual node pool deleting",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionFalse, infrav3exp.VirtualNodePoolDeletionInProgress}},
			testSpecificSetup: func(machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:             common.String("test"),
							LifecycleState: oke.VirtualNodePoolLifecycleStateDeleting,
							FreeformTags:   tags,
						},
					}, nil)
			},
		},
		{
			name:               "virtual node pool deleted",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav3exp.VirtualNodePoolReadyCondition, metav1.ConditionFalse, infrav3exp.VirtualNodePoolDeletedReason}},
			testSpecificSetup: func(machinePoolScope *scope.VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:             common.String("test"),
							LifecycleState: oke.VirtualNodePoolLifecycleStateDeleted,
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
				expectVMPConditions(g, ociVirtualMachinePool, tc.conditionAssertion)
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
				g.Expect(tc.expectedFailureMessages).To(Equal(ms.OCIVirtualMachinePool.Status.FailureMessages))
			}
		})
	}
}

func getOCIVirtualMachinePoolWithNoOwner() *infrav3exp.OCIVirtualMachinePool {
	ociMachine := getOCIVirtualMachinePool()
	ociMachine.OwnerReferences = []metav1.OwnerReference{}
	return ociMachine
}

func getOCIVirtualMachinePool() *infrav3exp.OCIVirtualMachinePool {
	return &infrav3exp.OCIVirtualMachinePool{
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
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
		Spec: infrav3exp.OCIVirtualMachinePoolSpec{
			ID:       common.String("test"),
			NsgNames: []string{"worker-nsg"},
			InitialVirtualNodeLabels: []infrav3exp.KeyValue{{
				Key:   common.String("key"),
				Value: common.String("value"),
			}},
			Taints: []infrav3exp.Taint{{
				Key:    common.String("key"),
				Value:  common.String("value"),
				Effect: common.String("effect"),
			}},
			PodConfiguration: infrav3exp.PodConfig{
				NsgNames:   []string{"pod-nsg"},
				Shape:      common.String("pod-shape"),
				SubnetName: common.String("pod-subnet"),
			},
			PlacementConfigs: []infrav3exp.VirtualNodepoolPlacementConfig{
				{
					AvailabilityDomain: common.String("test-ad"),
					SubnetName:         common.String("worker-subnet"),
					FaultDomains:       []string{"fd-1", "fd-2"},
				},
			},
		},
	}
}

func expectVMPConditions(g *WithT, m *infrav3exp.OCIVirtualMachinePool, expected []conditionAssertion) {
	g.Expect(len(m.Status.Conditions)).To(BeNumerically(">=", len(expected)), "number of conditions")
	for _, c := range expected {
		actual := conditions.Get(m, c.conditionType)
		g.Expect(actual).To(Not(BeNil()))
		g.Expect(actual.Type).To(Equal(c.conditionType))
		g.Expect(actual.Status).To(Equal(c.status))
		if c.reason != "" {
			g.Expect(actual.Reason).To(Equal(c.reason))
		} else if c.status == metav1.ConditionTrue {
			g.Expect(actual.Reason).To(Equal(c.conditionType))
		}
	}
}

func getOCIManagedClusterWithOwner() *infrastructurev1beta3.OCIManagedCluster {
	ociCluster := getOCIManagedClusterWithNoOwner()
	ociCluster.OwnerReferences = []metav1.OwnerReference{
		{
			Name:       "test-cluster",
			Kind:       "Cluster",
			APIVersion: clusterv1.GroupVersion.String(),
		},
	}
	return ociCluster
}

func getOCIManagedClusterWithNoOwner() *infrastructurev1beta3.OCIManagedCluster {
	ociCluster := &infrastructurev1beta3.OCIManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test",
		},
		Spec: infrastructurev1beta3.OCIManagedClusterSpec{
			CompartmentId: "test",
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Port: 6443,
			},
			OCIResourceIdentifier: "resource_uid",
			NetworkSpec: infrastructurev1beta3.NetworkSpec{
				Vcn: infrastructurev1beta3.VCN{
					ID: common.String("vcn-id"),
					Subnets: []*infrastructurev1beta3.Subnet{
						{
							Role: infrastructurev1beta3.ControlPlaneEndpointRole,
							ID:   common.String("subnet-id"),
							Type: infrastructurev1beta3.Private,
							Name: "worker-subnet",
						},
						{
							Role: infrastructurev1beta3.PodRole,
							ID:   common.String("pod-subnet-id"),
							Type: infrastructurev1beta3.Private,
							Name: "pod-subnet",
						},
					},
					NetworkSecurityGroup: infrastructurev1beta3.NetworkSecurityGroup{
						List: []*infrastructurev1beta3.NSG{
							{
								Role: infrastructurev1beta3.ControlPlaneEndpointRole,
								ID:   common.String("nsg-id"),
								Name: "worker-nsg",
							},
							{
								Role: infrastructurev1beta3.PodRole,
								ID:   common.String("pod-nsg-id"),
								Name: "pod-nsg",
							},
						},
					},
				},
			},
			AvailabilityDomains: map[string]infrastructurev1beta3.OCIAvailabilityDomain{
				"ad-1": {
					Name:         "ad-1",
					FaultDomains: []string{"fd-5", "fd-6"},
				},
			},
		},
	}
	ociCluster.OwnerReferences = []metav1.OwnerReference{}
	return ociCluster
}
