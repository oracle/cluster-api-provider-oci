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
	mock_scope "github.com/oracle/cluster-api-provider-oci/cloud/scope/mocks"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestOCIManagedClusterReconciler_Reconcile(t *testing.T) {
	var (
		r        OCIManagedClusterReconciler
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
		name          string
		objects       []client.Object
		expectedEvent string
	}{
		{
			name:    "cluster does not exist",
			objects: []client.Object{getSecret()},
		},
		{
			name:          "no owner reference",
			objects:       []client.Object{getSecret(), getOCIManagedClusterWithNoOwner()},
			expectedEvent: "OwnerRefNotSet",
		},
		{
			name:          "cluster is paused",
			objects:       []client.Object{getSecret(), getOCIManagedClusterWithOwner(), getPausedInfraCluster()},
			expectedEvent: "ClusterPaused",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)

			client := fake.NewClientBuilder().WithObjects(tc.objects...).Build()
			r = OCIManagedClusterReconciler{
				Client:   client,
				Scheme:   scheme.Scheme,
				Recorder: recorder,
				Region:   MockTestRegion,
			}
			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "test",
					Name:      "test-cluster",
				},
			}

			_, err := r.Reconcile(context.Background(), req)
			g.Expect(err).To(BeNil())

			if tc.expectedEvent != "" {
				g.Eventually(recorder.Events).Should(Receive(ContainSubstring(tc.expectedEvent)))
			}
		})
	}
}

func TestOCIManagedClusterReconciler_reconcile(t *testing.T) {
	var (
		r          OCIManagedClusterReconciler
		mockCtrl   *gomock.Controller
		recorder   *record.FakeRecorder
		ociCluster *infrastructurev1beta2.OCIManagedCluster
		cluster    *clusterv1.Cluster
		cs         *mock_scope.MockClusterScopeClient
	)

	setup := func(t *testing.T, g *WithT) {
		mockCtrl = gomock.NewController(t)
		cs = mock_scope.NewMockClusterScopeClient(mockCtrl)
		ociCluster = &infrastructurev1beta2.OCIManagedCluster{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       infrastructurev1beta2.OCIManagedClusterSpec{},
			Status:     infrastructurev1beta2.OCIManagedClusterStatus{},
		}
		cluster = &clusterv1beta2.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test",
			},
			Spec: clusterv1beta2.ClusterSpec{
				ControlPlaneRef: clusterv1beta2.ContractVersionedObjectReference{
					Name: "test",
				},
				Paused: ptr.To(true),
			},
		}
		controlPlane := infrastructurev1beta2.OCIManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		}

		recorder = record.NewFakeRecorder(20)
		client := fake.NewClientBuilder().WithObjects(getSecret(), &controlPlane).Build()
		r = OCIManagedClusterReconciler{
			Client:   client,
			Scheme:   scheme.Scheme,
			Recorder: recorder,
			Region:   MockTestRegion,
		}
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}
	tests := []struct {
		name               string
		errorExpected      bool
		expectedEvent      string
		eventNotExpected   string
		conditionAssertion conditionAssertion
		testSpecificSetup  func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster)
	}{
		{
			name:               "all success",
			expectedEvent:      infrastructurev1beta2.DRGRPCAttachmentEventReady,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionTrue, "", ""},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNSG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileRouteTable(context.Background()).Return(nil)
				cs.EXPECT().ReconcileSubnet(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().ReconcileFailureDomains(context.Background()).Return(nil)
			},
		},
		{
			name:               "drg reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.DrgEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.DrgReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "vcn reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.VcnEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.VcnReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "internet gateway reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.InternetGatewayEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.InternetGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "nat gateway reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.NatEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.NatGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "service gateway reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.ServiceGatewayEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.ServiceGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileServiceGateway(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "nsg reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.NetworkSecurityEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.NSGReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNSG(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "route table reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.RouteTableEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.RouteTableReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNSG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileRouteTable(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "subnet reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.SubnetEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.SubnetReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNSG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileRouteTable(context.Background()).Return(nil)
				cs.EXPECT().ReconcileSubnet(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "failure domain reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.FailureDomainEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.FailureDomainFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNSG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileRouteTable(context.Background()).Return(nil)
				cs.EXPECT().ReconcileSubnet(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().ReconcileFailureDomains(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "drg vcn attachment reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.DRGVCNAttachmentEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.DRGVCNAttachmentReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNSG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileRouteTable(context.Background()).Return(nil)
				cs.EXPECT().ReconcileSubnet(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRGVCNAttachment(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "drg rpc attachment reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta2.DRGRPCAttachmentEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.DRGRPCAttachmentReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().SetRegionCode(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNSG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileRouteTable(context.Background()).Return(nil)
				cs.EXPECT().ReconcileSubnet(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().ReconcileDRGRPCAttachment(context.Background()).Return(errors.New("some error"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, ociCluster)
			ctx := context.Background()
			_, err := r.reconcile(ctx, log.FromContext(ctx), cs, ociCluster, cluster)
			actual := conditions.Get(ociCluster, tc.conditionAssertion.conditionType)
			g.Expect(actual).To(Not(BeNil()))
			g.Expect(actual.Type).To(Equal(tc.conditionAssertion.conditionType))
			g.Expect(actual.Status).To(Equal(tc.conditionAssertion.status))
			g.Expect(actual.Severity).To(Equal(tc.conditionAssertion.severity))
			g.Expect(actual.Reason).To(Equal(tc.conditionAssertion.reason))

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

func TestOCIManagedClusterReconciler_reconcileDelete(t *testing.T) {
	var (
		r          OCIManagedClusterReconciler
		mockCtrl   *gomock.Controller
		recorder   *record.FakeRecorder
		ociCluster *infrastructurev1beta2.OCIManagedCluster
		cs         *mock_scope.MockClusterScopeClient
	)

	setup := func(t *testing.T, g *WithT) {
		mockCtrl = gomock.NewController(t)
		cs = mock_scope.NewMockClusterScopeClient(mockCtrl)
		ociCluster = &infrastructurev1beta2.OCIManagedCluster{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       infrastructurev1beta2.OCIManagedClusterSpec{},
			Status:     infrastructurev1beta2.OCIManagedClusterStatus{},
		}
		recorder = record.NewFakeRecorder(10)
		client := fake.NewClientBuilder().WithObjects(getSecret()).Build()
		r = OCIManagedClusterReconciler{
			Client:   client,
			Scheme:   scheme.Scheme,
			Recorder: recorder,
		}
		//cs.EXPECT().GetOCIClusterAccessor().Return(ociClusterAccessor)
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}
	tests := []struct {
		name               string
		errorExpected      bool
		expectedEvent      string
		conditionAssertion conditionAssertion
		testSpecificSetup  func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster)
	}{
		{
			name: "all success",
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(nil)
				cs.EXPECT().DeleteSubnets(context.Background()).Return(nil)
				cs.EXPECT().DeleteRouteTables(context.Background()).Return(nil)
				cs.EXPECT().DeleteSecurityLists(context.Background()).Return(nil)
				cs.EXPECT().DeleteServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteNatGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteVCN(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRG(context.Background()).Return(nil)
			},
		},
		{
			name:               "drg rpc delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.DRGRPCAttachmentReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "drg vcn attachment delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.DRGVCNAttachmentReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "nsg delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.NSGReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "subnet delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.SubnetReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(nil)
				cs.EXPECT().DeleteSubnets(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "route table delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.RouteTableReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(nil)
				cs.EXPECT().DeleteSubnets(context.Background()).Return(nil)
				cs.EXPECT().DeleteRouteTables(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "security list delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.SecurityListReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(nil)
				cs.EXPECT().DeleteSubnets(context.Background()).Return(nil)
				cs.EXPECT().DeleteRouteTables(context.Background()).Return(nil)
				cs.EXPECT().DeleteSecurityLists(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "service gateway delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.ServiceGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(nil)
				cs.EXPECT().DeleteSubnets(context.Background()).Return(nil)
				cs.EXPECT().DeleteRouteTables(context.Background()).Return(nil)
				cs.EXPECT().DeleteSecurityLists(context.Background()).Return(nil)
				cs.EXPECT().DeleteServiceGateway(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "nat gateway delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.NatGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(nil)
				cs.EXPECT().DeleteSubnets(context.Background()).Return(nil)
				cs.EXPECT().DeleteRouteTables(context.Background()).Return(nil)
				cs.EXPECT().DeleteSecurityLists(context.Background()).Return(nil)
				cs.EXPECT().DeleteServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteNatGateway(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "internet gateway delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.InternetGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(nil)
				cs.EXPECT().DeleteSubnets(context.Background()).Return(nil)
				cs.EXPECT().DeleteRouteTables(context.Background()).Return(nil)
				cs.EXPECT().DeleteSecurityLists(context.Background()).Return(nil)
				cs.EXPECT().DeleteServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteNatGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteInternetGateway(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "delete vcn failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.VcnReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(nil)
				cs.EXPECT().DeleteSubnets(context.Background()).Return(nil)
				cs.EXPECT().DeleteRouteTables(context.Background()).Return(nil)
				cs.EXPECT().DeleteSecurityLists(context.Background()).Return(nil)
				cs.EXPECT().DeleteServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteNatGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteVCN(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "delete drg failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta2.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta2.DrgReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta2.OCIManagedCluster) {
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(nil)
				cs.EXPECT().DeleteSubnets(context.Background()).Return(nil)
				cs.EXPECT().DeleteRouteTables(context.Background()).Return(nil)
				cs.EXPECT().DeleteSecurityLists(context.Background()).Return(nil)
				cs.EXPECT().DeleteServiceGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteNatGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().DeleteVCN(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRG(context.Background()).Return(errors.New("some error"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, ociCluster)
			ctx := context.Background()
			_, err := r.reconcileDelete(ctx, log.FromContext(ctx), cs, ociCluster)
			actual := conditions.Get(ociCluster, tc.conditionAssertion.conditionType)
			if tc.conditionAssertion != (conditionAssertion{}) {
				g.Expect(actual).To(Not(BeNil()))
				g.Expect(actual.Type).To(Equal(tc.conditionAssertion.conditionType))
				g.Expect(actual.Status).To(Equal(tc.conditionAssertion.status))
				g.Expect(actual.Severity).To(Equal(tc.conditionAssertion.severity))
				g.Expect(actual.Reason).To(Equal(tc.conditionAssertion.reason))
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

func getOCIManagedClusterWithNoOwner() *infrastructurev1beta2.OCIManagedCluster {
	ociCluster := &infrastructurev1beta2.OCIManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test",
		},
		Spec: infrastructurev1beta2.OCIManagedClusterSpec{
			CompartmentId: "test",
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
							Role: infrastructurev1beta2.PodRole,
							ID:   common.String("pod-subnet-id"),
							Type: infrastructurev1beta2.Private,
							Name: "pod-subnet",
						},
					},
					NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
						List: []*infrastructurev1beta2.NSG{
							{
								Role: infrastructurev1beta2.ControlPlaneEndpointRole,
								ID:   common.String("nsg-id"),
								Name: "worker-nsg",
							},
							{
								Role: infrastructurev1beta2.PodRole,
								ID:   common.String("pod-nsg-id"),
								Name: "pod-nsg",
							},
						},
					},
				},
			},
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

func getOCIManagedClusterWithOwner() *infrastructurev1beta2.OCIManagedCluster {
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
