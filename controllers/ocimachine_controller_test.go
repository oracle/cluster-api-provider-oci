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
	"errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute/mock_compute"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer/mock_nlb"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestMachineReconciliation(t *testing.T) {
	var (
		r        OCIMachineReconciler
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
			name:          "machine does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret()},
		},
		{
			name:          "no owner reference",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOciMachineWithNoOwner()},
			expectedEvent: "OwnerRefNotSet",
		},
		{
			name:          "cluster does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOciMachine(), getMachine()},
			expectedEvent: "ClusterDoesNotExist",
		},
		{
			name:             "paused cluster",
			errorExpected:    false,
			objects:          []client.Object{getSecret(), getOciMachine(), getMachine(), getPausedCluster()},
			eventNotExpected: "ClusterDoesNotExist",
		},
		{
			name:          "ocicluster does not exist",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOciMachine(), getMachine(), getCluster()},
			expectedEvent: "ClusterNotAvailable",
		},
		{
			name:          "bootstrap data not available",
			errorExpected: false,
			objects:       []client.Object{getSecret(), getOciMachine(), getMachine(), getCluster(), getOCICluster()},
			expectedEvent: infrastructurev1beta2.WaitingForBootstrapDataReason,
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

			client := fake.NewClientBuilder().WithStatusSubresource(tc.objects...).WithObjects(tc.objects...).Build()
			r = OCIMachineReconciler{
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
		r             OCIMachineReconciler
		mockCtrl      *gomock.Controller
		recorder      *record.FakeRecorder
		ociMachine    *infrastructurev1beta2.OCIMachine
		computeClient *mock_compute.MockComputeClient
		nlbClient     *mock_nlb.MockNetworkLoadBalancerClient
		vcnClient     *mock_vcn.MockClient
		ms            *scope.MachineScope
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().WithObjects(getSecret()).Build()
		computeClient = mock_compute.NewMockComputeClient(mockCtrl)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
		vcnClient = mock_vcn.NewMockClient(mockCtrl)
		machine := getMachine()
		ociMachine = getOciMachine()
		machine.Spec.Bootstrap.DataSecretName = common.String("bootstrap")
		ociCluster := getOCICluster()
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlbid")
		ms, err = scope.NewMachineScope(scope.MachineScopeParams{
			ComputeClient:             computeClient,
			NetworkLoadBalancerClient: nlbClient,
			VCNClient:                 vcnClient,
			OCIMachine:                ociMachine,
			Machine:                   machine,
			OCIClusterAccessor: scope.OCISelfManagedCluster{
				OCICluster: ociCluster,
			},
			Cluster: getCluster(),
			Client:  client,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIMachineReconciler{
			Client:   client,
			Scheme:   runtime.NewScheme(),
			Recorder: recorder,
		}
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}
	type test struct {
		name               string
		errorExpected      bool
		expectedEvent      string
		eventNotExpected   string
		conditionAssertion []conditionAssertion
		deleteMachines     []clusterv1.Machine
		testSpecificSetup  func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbclient *mock_nlb.MockNetworkLoadBalancerClient)
		validate           func(g *WithT, t *test, result ctrl.Result)
	}
	tests := []test{
		{
			name:               "instance in provisioning state",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrastructurev1beta2.InstanceNotReadyReason}},
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbclient *mock_nlb.MockNetworkLoadBalancerClient) {
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateProvisioning,
						},
					}, nil)
			},
		},
		{
			name:               "instance in running state",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbclient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
						},
					}, nil)
			},
		},
		{
			name:               "instance in running state, reconcile every 5 minutes",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbclient *mock_nlb.MockNetworkLoadBalancerClient) {
				if machineScope.Machine.ObjectMeta.Annotations == nil {
					machineScope.Machine.ObjectMeta.Annotations = make(map[string]string)
				}
				machineScope.Machine.ObjectMeta.Annotations[infrastructurev1beta2.DeleteMachineOnInstanceTermination] = ""
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
						},
					}, nil)
			},
		},
		{
			name:               "instance in running state, reconcile every 5 minutes",
			errorExpected:      false,
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionTrue, "", ""}},
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbclient *mock_nlb.MockNetworkLoadBalancerClient) {
				if machineScope.Machine.ObjectMeta.Annotations == nil {
					machineScope.Machine.ObjectMeta.Annotations = make(map[string]string)
				}
				machineScope.Machine.ObjectMeta.Annotations[infrastructurev1beta2.DeleteMachineOnInstanceTermination] = ""
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
						},
					}, nil)
			},
		},
		{
			name:               "instance in terminated state",
			errorExpected:      true,
			expectedEvent:      "invalid lifecycle state TERMINATED",
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.InstanceProvisionFailedReason}},
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbclient *mock_nlb.MockNetworkLoadBalancerClient) {
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateTerminated,
						},
					}, nil)
			},
		},
		{
			name:               "instance in terminated state, machine has to be deleted",
			errorExpected:      true,
			expectedEvent:      "invalid lifecycle state TERMINATED",
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.InstanceProvisionFailedReason}},
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbclient *mock_nlb.MockNetworkLoadBalancerClient) {
				fakeClient := fake.NewClientBuilder().WithObjects(getSecret()).Build()
				if machineScope.Machine.ObjectMeta.Annotations == nil {
					machineScope.Machine.ObjectMeta.Annotations = make(map[string]string)
				}
				machineScope.Machine.ObjectMeta.Annotations[infrastructurev1beta2.DeleteMachineOnInstanceTermination] = ""
				t.deleteMachines = make([]clusterv1.Machine, 0)
				machineScope.Client = interceptor.NewClient(fakeClient, interceptor.Funcs{
					Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						m := obj.(*clusterv1.Machine)
						t.deleteMachines = append(t.deleteMachines, *m)
						return nil
					},
				})
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateTerminated,
						},
					}, nil)
			},
			validate: func(g *WithT, t *test, result ctrl.Result) {
				g.Expect(len(t.deleteMachines)).To(Equal(1))
			},
		},
		{
			name:               "control plane backend vnic attachments call error",
			errorExpected:      true,
			expectedEvent:      "server error",
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.InstanceIPAddressNotFound}},
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbclient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.Machine.ObjectMeta.Labels = make(map[string]string)
				machineScope.Machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel] = "true"
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
						},
					}, nil)
				computeClient.EXPECT().ListVnicAttachments(gomock.Any(), gomock.Eq(core.ListVnicAttachmentsRequest{
					InstanceId:    common.String("test"),
					CompartmentId: common.String("test"),
					Page:          nil,
				})).
					Return(core.ListVnicAttachmentsResponse{}, errors.New("server error"))
			},
		},
		{
			name:          "control plane backend backend exists",
			errorExpected: false,
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.Machine.ObjectMeta.Labels = make(map[string]string)
				machineScope.Machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel] = "true"
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
						},
					}, nil)
				computeClient.EXPECT().ListVnicAttachments(gomock.Any(), gomock.Eq(core.ListVnicAttachmentsRequest{
					InstanceId:    common.String("test"),
					CompartmentId: common.String("test"),
					Page:          nil,
				})).
					Return(core.ListVnicAttachmentsResponse{
						Items: []core.VnicAttachment{
							{
								LifecycleState: core.VnicAttachmentLifecycleStateAttached,
								VnicId:         common.String("vnicid"),
							},
						},
					}, nil)
				vcnClient.EXPECT().GetVnic(gomock.Any(), gomock.Eq(core.GetVnicRequest{
					VnicId: common.String("vnicid"),
				})).
					Return(core.GetVnicResponse{
						Vnic: core.Vnic{
							IsPrimary: common.Bool(true),
							PrivateIp: common.String("1.1.1.1"),
						},
					}, nil)

				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							scope.APIServerLBBackendSetName: {
								Backends: []networkloadbalancer.Backend{
									{
										Name:      common.String("test"),
										IpAddress: common.String("1.1.1.1"),
									},
								},
							},
						},
					},
				}, nil)
			},
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionTrue, "", ""}},
		},
		{
			name:          "backend creation",
			errorExpected: false,
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.Machine.ObjectMeta.Labels = make(map[string]string)
				machineScope.Machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel] = "true"
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
						},
					}, nil)
				computeClient.EXPECT().ListVnicAttachments(gomock.Any(), gomock.Eq(core.ListVnicAttachmentsRequest{
					InstanceId:    common.String("test"),
					CompartmentId: common.String("test"),
					Page:          nil,
				})).
					Return(core.ListVnicAttachmentsResponse{
						Items: []core.VnicAttachment{
							{
								LifecycleState: core.VnicAttachmentLifecycleStateAttached,
								VnicId:         common.String("vnicid"),
							},
						},
					}, nil)
				vcnClient.EXPECT().GetVnic(gomock.Any(), gomock.Eq(core.GetVnicRequest{
					VnicId: common.String("vnicid"),
				})).
					Return(core.GetVnicResponse{
						Vnic: core.Vnic{
							IsPrimary: common.Bool(true),
							PrivateIp: common.String("1.1.1.1"),
						},
					}, nil)

				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							scope.APIServerLBBackendSetName: {
								Name:     common.String(scope.APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{},
							},
						},
					},
				}, nil)

				nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					networkloadbalancer.CreateBackendRequest{
						NetworkLoadBalancerId: common.String("nlbid"),
						BackendSetName:        common.String(scope.APIServerLBBackendSetName),
						CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
							Name:      common.String("test"),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(networkloadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					}}, nil)
			},
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionTrue, "", ""}},
		},
		{
			name:          "ip address exists",
			errorExpected: false,
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.Machine.ObjectMeta.Labels = make(map[string]string)
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				machineScope.Machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel] = "true"
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
						},
					}, nil)

				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							scope.APIServerLBBackendSetName: {
								Name:     common.String(scope.APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{},
							},
						},
					},
				}, nil)

				nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					networkloadbalancer.CreateBackendRequest{
						NetworkLoadBalancerId: common.String("nlbid"),
						BackendSetName:        common.String(scope.APIServerLBBackendSetName),
						CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
							Name:      common.String("test"),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(networkloadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					}}, nil)
			},
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionTrue, "", ""}},
		},
		{
			name:          "backend creation fails",
			errorExpected: true,
			testSpecificSetup: func(t *test, machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.Machine.ObjectMeta.Labels = make(map[string]string)
				machineScope.Machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel] = "true"
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
						},
					}, nil)
				computeClient.EXPECT().ListVnicAttachments(gomock.Any(), gomock.Eq(core.ListVnicAttachmentsRequest{
					InstanceId:    common.String("test"),
					CompartmentId: common.String("test"),
					Page:          nil,
				})).
					Return(core.ListVnicAttachmentsResponse{
						Items: []core.VnicAttachment{
							{
								LifecycleState: core.VnicAttachmentLifecycleStateAttached,
								VnicId:         common.String("vnicid"),
							},
						},
					}, nil)
				vcnClient.EXPECT().GetVnic(gomock.Any(), gomock.Eq(core.GetVnicRequest{
					VnicId: common.String("vnicid"),
				})).
					Return(core.GetVnicResponse{
						Vnic: core.Vnic{
							IsPrimary: common.Bool(true),
							PrivateIp: common.String("1.1.1.1"),
						},
					}, nil)

				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							scope.APIServerLBBackendSetName: {
								Name:     common.String(scope.APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{},
							},
						},
					},
				}, nil)

				nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					networkloadbalancer.CreateBackendRequest{
						NetworkLoadBalancerId: common.String("nlbid"),
						BackendSetName:        common.String(scope.APIServerLBBackendSetName),
						CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
							Name:      common.String("test"),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(networkloadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusFailed,
					}}, nil)
			},
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.InstanceLBBackendAdditionFailedReason}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(&tc, ms, computeClient, vcnClient, nlbClient)
			ctx := context.Background()
			result, err := r.reconcileNormal(ctx, log.FromContext(ctx), ms)
			if len(tc.conditionAssertion) > 0 {
				expectConditions(g, ociMachine, tc.conditionAssertion)
			}
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
			if tc.expectedEvent != "" {
				g.Eventually(recorder.Events).Should(Receive(ContainSubstring(tc.expectedEvent)))
			}
			if tc.validate != nil {
				tc.validate(g, &tc, result)
			}
		})
	}
}

func TestMachineReconciliationDelete(t *testing.T) {
	var (
		r             OCIMachineReconciler
		mockCtrl      *gomock.Controller
		recorder      *record.FakeRecorder
		ociMachine    *infrastructurev1beta2.OCIMachine
		computeClient *mock_compute.MockComputeClient
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		computeClient = mock_compute.NewMockComputeClient(mockCtrl)
		recorder = record.NewFakeRecorder(2)
		ociMachine = getOciMachine()
		now := metav1.NewTime(time.Now())
		ociMachine.DeletionTimestamp = &now
		controllerutil.AddFinalizer(ociMachine, infrastructurev1beta2.MachineFinalizer)
		client := fake.NewClientBuilder().WithObjects(getSecret(), getMachine(), ociMachine, getCluster(), getOCICluster()).Build()
		clientProvider, err := scope.MockNewClientProvider(scope.MockOCIClients{
			ComputeClient: computeClient,
		})
		if err != nil {
			t.Errorf("Expected %v to equal nil", err)
		}

		r = OCIMachineReconciler{
			Client:         client,
			Scheme:         runtime.NewScheme(),
			Recorder:       recorder,
			ClientProvider: clientProvider,
			Region:         scope.MockTestRegion,
		}
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	t.Run("OCIMachine Reconciliation", func(t *testing.T) {
		t.Run("instance terminated", func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "test",
					Name:      "test",
				},
			}

			computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
				InstanceId: common.String("test"),
			})).
				Return(core.GetInstanceResponse{
					Instance: core.Instance{
						Id:             common.String("test"),
						LifecycleState: core.InstanceLifecycleStateTerminated,
					},
				}, nil)

			_, err := r.Reconcile(context.Background(), req)
			// delete throws reconcile error when scope is closed
			g.Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})
}

func TestMachineReconciliationDeletionNormal(t *testing.T) {
	var (
		r             OCIMachineReconciler
		mockCtrl      *gomock.Controller
		recorder      *record.FakeRecorder
		ociMachine    *infrastructurev1beta2.OCIMachine
		computeClient *mock_compute.MockComputeClient
		nlbClient     *mock_nlb.MockNetworkLoadBalancerClient
		vcnClient     *mock_vcn.MockClient
		ms            *scope.MachineScope
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		now := metav1.NewTime(time.Now())
		ociMachine = getOciMachine()
		ociMachine.DeletionTimestamp = &now
		controllerutil.AddFinalizer(ociMachine, infrastructurev1beta2.MachineFinalizer)
		machine := getMachine()
		machine.Spec.Bootstrap.DataSecretName = common.String("bootstrap")
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().WithObjects(getSecret(), machine, ociMachine).Build()
		computeClient = mock_compute.NewMockComputeClient(mockCtrl)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
		vcnClient = mock_vcn.NewMockClient(mockCtrl)
		ociCluster := getOCICluster()
		ociCluster.UID = "uid"
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlbid")
		ms, err = scope.NewMachineScope(scope.MachineScopeParams{
			ComputeClient: computeClient,
			OCIMachine:    ociMachine,
			Machine:       machine,
			OCIClusterAccessor: scope.OCISelfManagedCluster{
				OCICluster: ociCluster,
			},
			Cluster:                   getCluster(),
			Client:                    client,
			NetworkLoadBalancerClient: nlbClient,
			VCNClient:                 vcnClient,
		})

		recorder = record.NewFakeRecorder(2)
		r = OCIMachineReconciler{
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
		name               string
		errorExpected      bool
		objects            []client.Object
		expectedEvent      string
		eventNotExpected   string
		conditionAssertion []conditionAssertion
		testSpecificSetup  func(machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbclient *mock_nlb.MockNetworkLoadBalancerClient)
	}{
		{
			name:               "instance in terminated state",
			errorExpected:      false,
			objects:            []client.Object{getSecret()},
			expectedEvent:      "InstanceTerminated",
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrastructurev1beta2.InstanceTerminatedReason}},
			testSpecificSetup: func(machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateTerminated,
						},
					}, nil)
			},
		},
		{
			name:               "instance in running, should be terminated",
			errorExpected:      false,
			objects:            []client.Object{getSecret()},
			expectedEvent:      "InstanceTerminating",
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrastructurev1beta2.InstanceTerminatingReason}},
			testSpecificSetup: func(machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
							FreeformTags: map[string]string{
								"CreatedBy":   "OCIClusterAPIProvider",
								"ClusterUUID": "uid"},
						},
					}, nil)
				computeClient.EXPECT().TerminateInstance(gomock.Any(), gomock.Eq(core.TerminateInstanceRequest{
					InstanceId:         common.String("test"),
					PreserveBootVolume: common.Bool(false),
				})).Return(core.TerminateInstanceResponse{}, nil)
			},
		},
		{
			name:               "lb backend is deleted",
			errorExpected:      false,
			objects:            []client.Object{getSecret()},
			expectedEvent:      "InstanceTerminating",
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrastructurev1beta2.InstanceTerminatingReason}},
			testSpecificSetup: func(machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.Machine.ObjectMeta.Labels = make(map[string]string)
				machineScope.Machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel] = "true"
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             common.String("test"),
							LifecycleState: core.InstanceLifecycleStateRunning,
							FreeformTags: map[string]string{
								"CreatedBy":   "OCIClusterAPIProvider",
								"ClusterUUID": "uid"},
						},
					}, nil)

				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							scope.APIServerLBBackendSetName: {
								Name: common.String(scope.APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{
									{
										Name:      common.String("test"),
										IpAddress: common.String("1.1.1.1"),
									},
								},
							},
						},
					},
				}, nil)

				nlbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Eq(
					networkloadbalancer.DeleteBackendRequest{
						NetworkLoadBalancerId: common.String("nlbid"),
						BackendSetName:        common.String(scope.APIServerLBBackendSetName),
						BackendName:           common.String("test"),
					})).Return(networkloadbalancer.DeleteBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					}}, nil)

				computeClient.EXPECT().TerminateInstance(gomock.Any(), gomock.Eq(core.TerminateInstanceRequest{
					InstanceId:         common.String("test"),
					PreserveBootVolume: common.Bool(false),
				})).Return(core.TerminateInstanceResponse{}, nil)
			},
		},
		{
			name:               "tombstone",
			errorExpected:      false,
			objects:            []client.Object{getSecret()},
			expectedEvent:      "OCIMachineRemovedFromLB",
			conditionAssertion: []conditionAssertion{{infrastructurev1beta2.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrastructurev1beta2.InstanceNotFoundReason}},
			testSpecificSetup: func(machineScope *scope.MachineScope, computeClient *mock_compute.MockComputeClient, vcnClient *mock_vcn.MockClient, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.Machine.ObjectMeta.Labels = make(map[string]string)
				machineScope.Machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel] = "true"
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{}, ociutil.ErrNotFound)

				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							scope.APIServerLBBackendSetName: {
								Name: common.String(scope.APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{
									{
										Name:      common.String("test"),
										IpAddress: common.String("1.1.1.1"),
									},
								},
							},
						},
					},
				}, nil)

				nlbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Eq(
					networkloadbalancer.DeleteBackendRequest{
						NetworkLoadBalancerId: common.String("nlbid"),
						BackendSetName:        common.String(scope.APIServerLBBackendSetName),
						BackendName:           common.String("test"),
					})).Return(networkloadbalancer.DeleteBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					}}, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, computeClient, vcnClient, nlbClient)
			ctx := context.Background()
			_, err := r.reconcileDelete(ctx, ms)
			if len(tc.conditionAssertion) > 0 {
				expectConditions(g, ociMachine, tc.conditionAssertion)
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

type conditionAssertion struct {
	conditionType clusterv1.ConditionType
	status        corev1.ConditionStatus
	severity      clusterv1.ConditionSeverity
	reason        string
}

func expectConditions(g *WithT, m *infrastructurev1beta2.OCIMachine, expected []conditionAssertion) {
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

func getSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("test"),
		},
	}
}

func getOciMachine() *infrastructurev1beta2.OCIMachine {
	return &infrastructurev1beta2.OCIMachine{
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
					Kind:       "Machine",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
		Spec: infrastructurev1beta2.OCIMachineSpec{
			InstanceId: common.String("test"),
		},
	}
}

func getOciMachineWithNoOwner() *infrastructurev1beta2.OCIMachine {
	ociMachine := getOciMachine()
	ociMachine.OwnerReferences = []metav1.OwnerReference{}
	return ociMachine
}

func getCluster() *clusterv1.Cluster {
	infraRef := corev1.ObjectReference{
		Name: "oci-cluster",
		Kind: "OCICluster",
	}
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &infraRef,
		},
	}
}

func getPausedCluster() *clusterv1.Cluster {
	cluster := getCluster()
	cluster.Spec.Paused = true
	return cluster
}

func getMachine() *clusterv1.Machine {
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}
	return machine
}

func getOCICluster() *infrastructurev1beta2.OCICluster {
	return &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oci-cluster",
			Namespace: "test",
		},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			CompartmentId: "test",
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Port: 6443,
			},
		},
	}
}
