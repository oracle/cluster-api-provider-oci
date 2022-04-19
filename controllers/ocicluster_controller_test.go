package controllers

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	mock_scope "github.com/oracle/cluster-api-provider-oci/cloud/scope/mocks"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	MockTestRegion = "us-austin-1"
)

func TestOCIClusterReconciler_Reconcile(t *testing.T) {
	var (
		r        OCIClusterReconciler
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
			objects:       []client.Object{getSecret(), getOciClusterWithNoOwner()},
			expectedEvent: "OwnerRefNotSet",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)

			client := fake.NewClientBuilder().WithObjects(tc.objects...).Build()
			r = OCIClusterReconciler{
				Client:   client,
				Scheme:   runtime.NewScheme(),
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

func TestOCIClusterReconciler_reconcile(t *testing.T) {
	var (
		r          OCIClusterReconciler
		mockCtrl   *gomock.Controller
		recorder   *record.FakeRecorder
		ociCluster *infrastructurev1beta1.OCICluster
		cs         *mock_scope.MockClusterScopeClient
	)

	setup := func(t *testing.T, g *WithT) {
		mockCtrl = gomock.NewController(t)
		cs = mock_scope.NewMockClusterScopeClient(mockCtrl)
		ociCluster = &infrastructurev1beta1.OCICluster{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       infrastructurev1beta1.OCIClusterSpec{},
			Status:     infrastructurev1beta1.OCIClusterStatus{},
		}
		recorder = record.NewFakeRecorder(20)
		client := fake.NewClientBuilder().WithObjects(getSecret()).Build()
		r = OCIClusterReconciler{
			Client:   client,
			Scheme:   runtime.NewScheme(),
			Recorder: recorder,
			Region:   MockTestRegion,
		}
		cs.EXPECT().GetOCICluster().Return(ociCluster)
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
		testSpecificSetup  func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster)
	}{
		{
			name:               "all success",
			expectedEvent:      infrastructurev1beta1.DRGRPCAttachmentEventReady,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionTrue, "", ""},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
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
				cs.EXPECT().ReconcileApiServerLB(context.Background()).Return(nil)
			},
		},
		{
			name:               "drg reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta1.DrgEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.DrgReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().ReconcileDRG(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "vcn reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta1.VcnEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.VcnReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "internet gateway reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta1.InternetGatewayEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.InternetGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "nat gateway reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta1.NatEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.NatGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().ReconcileDRG(context.Background()).Return(nil)
				cs.EXPECT().ReconcileVCN(context.Background()).Return(nil)
				cs.EXPECT().ReconcileInternetGateway(context.Background()).Return(nil)
				cs.EXPECT().ReconcileNatGateway(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "service gateway reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta1.ServiceGatewayEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.ServiceGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
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
			eventNotExpected:   infrastructurev1beta1.NetworkSecurityEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.NSGReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
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
			eventNotExpected:   infrastructurev1beta1.RouteTableEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.RouteTableReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
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
			eventNotExpected:   infrastructurev1beta1.SubnetEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.SubnetReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
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
			name:               "api server lb reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta1.ApiServerLoadBalancerEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.APIServerLoadBalancerFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
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
				cs.EXPECT().ReconcileApiServerLB(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "failure domain reconciliation failure",
			expectedEvent:      "ReconcileError",
			eventNotExpected:   infrastructurev1beta1.FailureDomainEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.FailureDomainFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
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
			eventNotExpected:   infrastructurev1beta1.DRGVCNAttachmentEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.DRGVCNAttachmentReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
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
			eventNotExpected:   infrastructurev1beta1.DRGRPCAttachmentEventReady,
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.DRGRPCAttachmentReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
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
		{
			name:               "skip vcn reconciliation",
			expectedEvent:      infrastructurev1beta1.ApiServerLoadBalancerEventReady,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionTrue, "", ""},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				ociCluster.Spec.NetworkSpec.SkipNetworkManagement = true
				cs.EXPECT().ReconcileFailureDomains(context.Background()).Return(nil)
				cs.EXPECT().ReconcileApiServerLB(context.Background()).Return(nil)
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
			_, err := r.reconcile(ctx, log.FromContext(ctx), cs)
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

func TestOCIClusterReconciler_reconcileDelete(t *testing.T) {
	var (
		r          OCIClusterReconciler
		mockCtrl   *gomock.Controller
		recorder   *record.FakeRecorder
		ociCluster *infrastructurev1beta1.OCICluster
		cs         *mock_scope.MockClusterScopeClient
	)

	setup := func(t *testing.T, g *WithT) {
		mockCtrl = gomock.NewController(t)
		cs = mock_scope.NewMockClusterScopeClient(mockCtrl)
		ociCluster = &infrastructurev1beta1.OCICluster{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       infrastructurev1beta1.OCIClusterSpec{},
			Status:     infrastructurev1beta1.OCIClusterStatus{},
		}
		recorder = record.NewFakeRecorder(10)
		client := fake.NewClientBuilder().WithObjects(getSecret()).Build()
		r = OCIClusterReconciler{
			Client:   client,
			Scheme:   runtime.NewScheme(),
			Recorder: recorder,
		}
		cs.EXPECT().GetOCICluster().Return(ociCluster)
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}
	tests := []struct {
		name               string
		errorExpected      bool
		expectedEvent      string
		conditionAssertion conditionAssertion
		testSpecificSetup  func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster)
	}{
		{
			name: "all success",
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.DRGRPCAttachmentReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "drg vcn attachment delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.DRGVCNAttachmentReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "api server lb delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.APIServerLoadBalancerFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "api server lb delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.APIServerLoadBalancerFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "nsg delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.NSGReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGRPCAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteDRGVCNAttachment(context.Background()).Return(nil)
				cs.EXPECT().DeleteNSGs(context.Background()).Return(errors.New("some error"))
			},
		},
		{
			name:               "subnet delete failure",
			expectedEvent:      "ReconcileError",
			errorExpected:      true,
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.SubnetReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.RouteTableReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.SecurityListReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.ServiceGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.NatGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.InternetGatewayReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.VcnReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
			conditionAssertion: conditionAssertion{infrastructurev1beta1.ClusterReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrastructurev1beta1.DrgReconciliationFailedReason},
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
		{
			name: "skip vcn deletion",
			testSpecificSetup: func(cs *mock_scope.MockClusterScopeClient, ociCluster *infrastructurev1beta1.OCICluster) {
				ociCluster.Spec.NetworkSpec.SkipNetworkManagement = true
				cs.EXPECT().DeleteApiServerLB(context.Background()).Return(nil)
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
			_, err := r.reconcileDelete(ctx, log.FromContext(ctx), cs)
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

func getOciClusterWithNoOwner() *infrastructurev1beta1.OCICluster {
	ociCluster := &infrastructurev1beta1.OCICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test",
		},
		Spec: infrastructurev1beta1.OCIClusterSpec{
			CompartmentId: "test",
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Port: 6443,
			},
		},
	}
	ociCluster.OwnerReferences = []metav1.OwnerReference{}
	return ociCluster
}
