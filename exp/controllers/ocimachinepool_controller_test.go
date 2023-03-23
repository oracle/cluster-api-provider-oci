/*
Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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

package controllers

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/computemanagement/mock_computemanagement"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
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

func TestMachinePoolReconciliation(t *testing.T) {
	var (
		r        OCIMachinePoolReconciler
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
			objects:       []client.Object{getSecret(), getMachinePoolWithNoOwner()},
			expectedEvent: "OwnerRefNotSet",
		},
		{
			name:          "cluster does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOCIMachinePool(), getMachinePool()},
			expectedEvent: "ClusterDoesNotExist",
		},
		{
			name:             "paused cluster",
			errorExpected:    false,
			objects:          []client.Object{getSecret(), getOCIMachinePool(), getMachinePool(), getPausedCluster()},
			eventNotExpected: "ClusterDoesNotExist",
		},
		{
			name:          "ocicluster does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOCIMachinePool(), getMachinePool(), getCluster()},
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
			r = OCIMachinePoolReconciler{
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

func getMachinePoolWithNoOwner() *infrav2exp.OCIMachinePool {
	ociMachinePool := getOCIMachinePool()
	ociMachinePool.OwnerReferences = []metav1.OwnerReference{}
	return ociMachinePool
}

func getOCIMachinePool() *infrav2exp.OCIMachinePool {
	return &infrav2exp.OCIMachinePool{
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
		Spec: infrav2exp.OCIMachinePoolSpec{},
	}
}

func TestReconciliationFunction(t *testing.T) {
	var (
		r                       OCIMachinePoolReconciler
		mockCtrl                *gomock.Controller
		recorder                *record.FakeRecorder
		ociMachinePool          *infrav2exp.OCIMachinePool
		ms                      *scope.MachinePoolScope
		computeManagementClient *mock_computemanagement.MockClient
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	definedTagsInterface := make(map[string]map[string]interface{})
	definedTags := map[string]map[string]string{
		"ns1": {
			"tag1": "foo",
			"tag2": "bar",
		},
		"ns2": {
			"tag1": "foo1",
			"tag2": "bar1",
		},
	}
	for ns, mapNs := range definedTags {
		mapValues := make(map[string]interface{})
		for k, v := range mapNs {
			mapValues[k] = v
		}
		definedTagsInterface[ns] = mapValues
	}
	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		computeManagementClient = mock_computemanagement.NewMockClient(mockCtrl)
		machinePool := getMachinePool()
		ociMachinePool = getOCIMachinePool()
		client := fake.NewClientBuilder().WithObjects(getSecret(), ociMachinePool).Build()
		ociCluster := getOCIClusterWithOwner()
		ms, err = scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
			ComputeManagementClient: computeManagementClient,
			OCICluster:              ociCluster,
			Cluster:                 getCluster(),
			Client:                  client,
			OCIMachinePool:          ociMachinePool,
			MachinePool:             machinePool,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIMachinePoolReconciler{
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
		testSpecificSetup       func(machinePoolScope *scope.MachinePoolScope, computeManagementClient *mock_computemanagement.MockClient)
		expectedFailureMessages []string
	}{
		{
			name:               "bootstrap data not ready",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav2exp.InstancePoolReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrastructurev1beta2.WaitingForBootstrapDataReason}},
			testSpecificSetup: func(machinePoolScope *scope.MachinePoolScope, computeManagementClient *mock_computemanagement.MockClient) {
			},
		},
		{
			name:               "instance pool create",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav2exp.LaunchTemplateReadyCondition, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(machinePoolScope *scope.MachinePoolScope, computeManagementClient *mock_computemanagement.MockClient) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
				}
				ms.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName = common.String("bootstrap")
				computeManagementClient.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
					InstanceConfigurationId: common.String("test"),
				})).
					Return(core.GetInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("test"),
							InstanceDetails: core.ComputeInstanceDetails{
								LaunchDetails: &core.InstanceConfigurationLaunchInstanceDetails{
									DefinedTags:   definedTagsInterface,
									FreeformTags:  tags,
									CompartmentId: common.String("test-compartment"),
									Shape:         common.String("test-shape"),
									CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
										FreeformTags: tags,
										NsgIds:       []string{"worker-nsg-id"},
										SubnetId:     common.String("worker-subnet-id"),
									},
									SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
									Metadata:      map[string]string{"user_data": "dGVzdA=="},
								},
							},
						},
					}, nil)

				computeManagementClient.EXPECT().ListInstancePools(gomock.Any(), gomock.Any()).
					Return(core.ListInstancePoolsResponse{}, nil)
				computeManagementClient.EXPECT().CreateInstancePool(gomock.Any(), gomock.Any()).
					Return(core.CreateInstancePoolResponse{
						InstancePool: core.InstancePool{
							LifecycleState: core.InstancePoolLifecycleStateProvisioning,
							Id:             common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:               "instance pool running",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav2exp.LaunchTemplateReadyCondition, corev1.ConditionTrue, "", ""}, {infrav2exp.InstancePoolReadyCondition, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(machinePoolScope *scope.MachinePoolScope, computeManagementClient *mock_computemanagement.MockClient) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
				}
				ms.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName = common.String("bootstrap")
				ms.OCIMachinePool.Spec.OCID = common.String("pool-id")
				computeManagementClient.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
					InstanceConfigurationId: common.String("test"),
				})).
					Return(core.GetInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("test"),
							InstanceDetails: core.ComputeInstanceDetails{
								LaunchDetails: &core.InstanceConfigurationLaunchInstanceDetails{
									DefinedTags:   definedTagsInterface,
									FreeformTags:  tags,
									CompartmentId: common.String("test-compartment"),
									Shape:         common.String("test-shape"),
									CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
										FreeformTags: tags,
										NsgIds:       []string{"worker-nsg-id"},
										SubnetId:     common.String("worker-subnet-id"),
									},
									SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
									Metadata:      map[string]string{"user_data": "dGVzdA=="},
								},
							},
						},
					}, nil)

				computeManagementClient.EXPECT().GetInstancePool(gomock.Any(), gomock.Any()).
					Return(core.GetInstancePoolResponse{
						InstancePool: core.InstancePool{
							LifecycleState:          core.InstancePoolLifecycleStateRunning,
							Id:                      common.String("pool-id"),
							InstanceConfigurationId: common.String("test"),
							Size:                    common.Int(3),
						},
					}, nil)
				computeManagementClient.EXPECT().ListInstancePoolInstances(gomock.Any(), gomock.Any()).
					Return(core.ListInstancePoolInstancesResponse{
						Items: []core.InstanceSummary{{
							Id:    common.String("id-1"),
							State: common.String("Running"),
						}},
					}, nil)
				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)
			},
		},
		{
			name:               "instance pool failed",
			errorExpected:      true,
			conditionAssertion: []conditionAssertion{{infrav2exp.LaunchTemplateReadyCondition, corev1.ConditionTrue, "", ""}, {infrav2exp.InstancePoolReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrav2exp.InstancePoolProvisionFailedReason}},
			testSpecificSetup: func(machinePoolScope *scope.MachinePoolScope, computeManagementClient *mock_computemanagement.MockClient) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
				}
				ms.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName = common.String("bootstrap")
				ms.OCIMachinePool.Spec.OCID = common.String("pool-id")
				computeManagementClient.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
					InstanceConfigurationId: common.String("test"),
				})).
					Return(core.GetInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("test"),
							InstanceDetails: core.ComputeInstanceDetails{
								LaunchDetails: &core.InstanceConfigurationLaunchInstanceDetails{
									DefinedTags:   definedTagsInterface,
									FreeformTags:  tags,
									CompartmentId: common.String("test-compartment"),
									Shape:         common.String("test-shape"),
									CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
										FreeformTags: tags,
										NsgIds:       []string{"worker-nsg-id"},
										SubnetId:     common.String("worker-subnet-id"),
									},
									SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
									Metadata:      map[string]string{"user_data": "dGVzdA=="},
								},
							},
						},
					}, nil)

				computeManagementClient.EXPECT().GetInstancePool(gomock.Any(), gomock.Any()).
					Return(core.GetInstancePoolResponse{
						InstancePool: core.InstancePool{
							LifecycleState:          core.InstancePoolLifecycleStateTerminated,
							Id:                      common.String("pool-id"),
							InstanceConfigurationId: common.String("test"),
							Size:                    common.Int(3),
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
			tc.testSpecificSetup(ms, computeManagementClient)
			ctx := context.Background()
			_, err := r.reconcileNormal(ctx, log.FromContext(ctx), ms)
			if len(tc.conditionAssertion) > 0 {
				expectMachinePoolConditions(g, ociMachinePool, tc.conditionAssertion)
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
				g.Expect(tc.expectedFailureMessages).To(Equal(ms.OCIMachinePool.Status.FailureMessage))
			}
		})
	}
}

func TestDeleteeconciliationFunction(t *testing.T) {
	var (
		r                       OCIMachinePoolReconciler
		mockCtrl                *gomock.Controller
		recorder                *record.FakeRecorder
		ociMachinePool          *infrav2exp.OCIMachinePool
		ms                      *scope.MachinePoolScope
		computeManagementClient *mock_computemanagement.MockClient
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"
	definedTagsInterface := make(map[string]map[string]interface{})
	definedTags := map[string]map[string]string{
		"ns1": {
			"tag1": "foo",
			"tag2": "bar",
		},
		"ns2": {
			"tag1": "foo1",
			"tag2": "bar1",
		},
	}
	for ns, mapNs := range definedTags {
		mapValues := make(map[string]interface{})
		for k, v := range mapNs {
			mapValues[k] = v
		}
		definedTagsInterface[ns] = mapValues
	}
	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		computeManagementClient = mock_computemanagement.NewMockClient(mockCtrl)
		machinePool := getMachinePool()
		ociMachinePool = getOCIMachinePool()
		client := fake.NewClientBuilder().WithObjects(getSecret(), ociMachinePool).Build()
		ociCluster := getOCIClusterWithOwner()
		ms, err = scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
			ComputeManagementClient: computeManagementClient,
			OCICluster:              ociCluster,
			Cluster:                 getCluster(),
			Client:                  client,
			OCIMachinePool:          ociMachinePool,
			MachinePool:             machinePool,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIMachinePoolReconciler{
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
		testSpecificSetup       func(machinePoolScope *scope.MachinePoolScope, computeManagementClient *mock_computemanagement.MockClient)
		expectedFailureMessages []string
	}{
		{
			name:               "instance pool running",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{},
			testSpecificSetup: func(machinePoolScope *scope.MachinePoolScope, computeManagementClient *mock_computemanagement.MockClient) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
				}
				ms.OCIMachinePool.Spec.OCID = common.String("pool-id")
				computeManagementClient.EXPECT().GetInstancePool(gomock.Any(), gomock.Any()).
					Return(core.GetInstancePoolResponse{
						InstancePool: core.InstancePool{
							LifecycleState:          core.InstancePoolLifecycleStateRunning,
							Id:                      common.String("pool-id"),
							InstanceConfigurationId: common.String("test"),
							Size:                    common.Int(3),
						},
					}, nil)
				computeManagementClient.EXPECT().TerminateInstancePool(gomock.Any(), gomock.Any()).
					Return(core.TerminateInstancePoolResponse{}, nil)
			},
		},
		{
			name:               "instance pool terminated",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrav2exp.InstancePoolReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrav2exp.InstancePoolDeletionInProgress}},
			testSpecificSetup: func(machinePoolScope *scope.MachinePoolScope, computeManagementClient *mock_computemanagement.MockClient) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
				}
				ms.OCIMachinePool.Spec.OCID = common.String("pool-id")
				computeManagementClient.EXPECT().GetInstancePool(gomock.Any(), gomock.Any()).
					Return(core.GetInstancePoolResponse{
						InstancePool: core.InstancePool{
							LifecycleState:          core.InstancePoolLifecycleStateTerminated,
							Id:                      common.String("pool-id"),
							InstanceConfigurationId: common.String("test"),
							Size:                    common.Int(3),
						},
					}, nil)
				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)
				computeManagementClient.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
					InstanceConfigurationId: common.String("test"),
				})).
					Return(core.GetInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("test"),
						},
					}, nil)
				computeManagementClient.EXPECT().DeleteInstanceConfiguration(gomock.Any(), gomock.Eq(core.DeleteInstanceConfigurationRequest{
					InstanceConfigurationId: common.String("test"),
				})).
					Return(core.DeleteInstanceConfigurationResponse{}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, computeManagementClient)
			ctx := context.Background()
			_, err := r.reconcileDelete(ctx, ms)
			if len(tc.conditionAssertion) > 0 {
				expectMachinePoolConditions(g, ociMachinePool, tc.conditionAssertion)
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
				g.Expect(tc.expectedFailureMessages).To(Equal(ms.OCIMachinePool.Status.FailureMessage))
			}
		})
	}
}

func getOCIClusterWithOwner() *infrastructurev1beta2.OCICluster {
	ociCluster := getOCIClusterWithNoOwner()
	ociCluster.OwnerReferences = []metav1.OwnerReference{
		{
			Name:       "test-cluster",
			Kind:       "Cluster",
			APIVersion: clusterv1.GroupVersion.String(),
		},
	}
	return ociCluster
}

func getOCIClusterWithNoOwner() *infrastructurev1beta2.OCICluster {
	ociCluster := &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test",
		},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			CompartmentId: "test-compartment",
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Port: 6443,
			},
			OCIResourceIdentifier: "resource_uid",
			NetworkSpec: infrastructurev1beta2.NetworkSpec{
				Vcn: infrastructurev1beta2.VCN{
					ID: common.String("vcn-id"),
					Subnets: []*infrastructurev1beta2.Subnet{
						{
							Role: infrastructurev1beta2.ControlPlaneEndpointRole,
							ID:   common.String("subnet-id"),
							Type: infrastructurev1beta2.Private,
							Name: "worker-subnet",
						},
						{
							Role: infrastructurev1beta2.WorkerRole,
							ID:   common.String("worker-subnet-id"),
							Type: infrastructurev1beta2.Private,
							Name: "worker-subnet",
						},
					},
					NetworkSecurityGroups: infrastructurev1beta2.NetworkSecurityGroups{
						NSGList: []*infrastructurev1beta2.NSG{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								ID:   common.String("nsg-id"),
								Name: "worker-nsg",
							},
							{
								Role: infrastructurev1beta2.WorkerRole,
								ID:   common.String("worker-nsg-id"),
								Name: "worker-nsg",
							},
						},
					},
				},
			},
		},
		Status: infrastructurev1beta2.OCIClusterStatus{
			AvailabilityDomains: map[string]infrastructurev1beta2.OCIAvailabilityDomain{
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

func expectMachinePoolConditions(g *WithT, m *infrav2exp.OCIMachinePool, expected []conditionAssertion) {
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
