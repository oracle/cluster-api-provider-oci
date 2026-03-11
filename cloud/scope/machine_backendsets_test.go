package scope

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer/mock_nlb"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/workrequests/mock_workrequests"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNLBCreateMembershipAcrossBackendSets(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	nlbClient := mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
	wrClient := mock_workrequests.NewMockClient(mockCtrl)
	client := fake.NewClientBuilder().Build()
	ociCluster := &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{UID: "uid"},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			NetworkSpec: infrastructurev1beta2.NetworkSpec{
				APIServerLB: infrastructurev1beta2.LoadBalancer{
					LoadBalancerId: common.String("nlbid"),
					NLBSpec: infrastructurev1beta2.NLBSpec{
						BackendSets: []infrastructurev1beta2.NLBBackendSet{
							{Name: "set-a"},
							{Name: "set-b"},
						},
					},
				},
			},
		},
	}
	ociCluster.Spec.ControlPlaneEndpoint.Port = 6443

	ms, err := NewMachineScope(MachineScopeParams{
		NetworkLoadBalancerClient: nlbClient,
		WorkRequestsClient:        wrClient,
		OCIMachine: &infrastructurev1beta2.OCIMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "test", UID: "uid"},
		},
		Machine: &clusterv1.Machine{},
		Cluster: &clusterv1.Cluster{},
		OCIClusterAccessor: OCISelfManagedCluster{
			OCICluster: ociCluster,
		},
		Client: client,
	})
	g.Expect(err).To(BeNil())
	ms.OCIMachine.Status.Addresses = []clusterv1beta1.MachineAddress{{
		Type:    clusterv1beta1.MachineInternalIP,
		Address: "1.1.1.1",
	}}

	nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: common.String("nlbid"),
	})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
		NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
			BackendSets: map[string]networkloadbalancer.BackendSet{
				"set-a": {Name: common.String("set-a"), Backends: []networkloadbalancer.Backend{}},
				"set-b": {Name: common.String("set-b"), Backends: []networkloadbalancer.Backend{}},
			},
		},
	}, nil)

	nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(networkloadbalancer.CreateBackendRequest{
		NetworkLoadBalancerId: common.String("nlbid"),
		BackendSetName:        common.String("set-a"),
		CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
			IpAddress: common.String("1.1.1.1"),
			Port:      common.Int(6443),
			Name:      common.String(fmt.Sprintf("test-%s", stableShortHash("set-a"))),
		},
		OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s-%s", "create-backend", "uid", stableShortHash("set-a:6443")),
	})).Return(networkloadbalancer.CreateBackendResponse{OpcWorkRequestId: common.String("wrid-a")}, nil)
	nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
		WorkRequestId: common.String("wrid-a"),
	})).Return(networkloadbalancer.GetWorkRequestResponse{
		WorkRequest: networkloadbalancer.WorkRequest{Status: networkloadbalancer.OperationStatusSucceeded},
	}, nil)

	nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(networkloadbalancer.CreateBackendRequest{
		NetworkLoadBalancerId: common.String("nlbid"),
		BackendSetName:        common.String("set-b"),
		CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
			IpAddress: common.String("1.1.1.1"),
			Port:      common.Int(6443),
			Name:      common.String(fmt.Sprintf("test-%s", stableShortHash("set-b"))),
		},
		OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s-%s", "create-backend", "uid", stableShortHash("set-b:6443")),
	})).Return(networkloadbalancer.CreateBackendResponse{OpcWorkRequestId: common.String("wrid-b")}, nil)
	nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
		WorkRequestId: common.String("wrid-b"),
	})).Return(networkloadbalancer.GetWorkRequestResponse{
		WorkRequest: networkloadbalancer.WorkRequest{Status: networkloadbalancer.OperationStatusSucceeded},
	}, nil)

	g.Expect(ms.ReconcileCreateInstanceOnLB(context.Background())).To(Succeed())
}

func TestNLBDeleteMembershipFromAllBackendSets(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	nlbClient := mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
	wrClient := mock_workrequests.NewMockClient(mockCtrl)
	client := fake.NewClientBuilder().Build()
	ociCluster := &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{UID: "uid"},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			NetworkSpec: infrastructurev1beta2.NetworkSpec{
				APIServerLB: infrastructurev1beta2.LoadBalancer{
					LoadBalancerId: common.String("nlbid"),
					NLBSpec: infrastructurev1beta2.NLBSpec{
						BackendSets: []infrastructurev1beta2.NLBBackendSet{
							{Name: "set-a"},
						},
					},
				},
			},
		},
	}
	ociCluster.Spec.ControlPlaneEndpoint.Port = 6443

	ms, err := NewMachineScope(MachineScopeParams{
		NetworkLoadBalancerClient: nlbClient,
		WorkRequestsClient:        wrClient,
		OCIMachine: &infrastructurev1beta2.OCIMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "test", UID: "uid"},
		},
		Machine: &clusterv1.Machine{},
		Cluster: &clusterv1.Cluster{},
		OCIClusterAccessor: OCISelfManagedCluster{
			OCICluster: ociCluster,
		},
		Client: client,
	})
	g.Expect(err).To(BeNil())

	nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: common.String("nlbid"),
	})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
		NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
			BackendSets: map[string]networkloadbalancer.BackendSet{
				"set-a":   {Name: common.String("set-a"), Backends: []networkloadbalancer.Backend{{Name: common.String("test")}}},
				"set-old": {Name: common.String("set-old"), Backends: []networkloadbalancer.Backend{{Name: common.String("test")}}},
			},
		},
	}, nil)

	seen := map[string]bool{}
	nlbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
		func(_ context.Context, req networkloadbalancer.DeleteBackendRequest) (networkloadbalancer.DeleteBackendResponse, error) {
			if req.NetworkLoadBalancerId == nil || *req.NetworkLoadBalancerId != "nlbid" {
				t.Fatalf("unexpected load balancer id: %#v", req.NetworkLoadBalancerId)
			}
			if req.BackendName == nil || *req.BackendName != "test" {
				t.Fatalf("unexpected backend name: %#v", req.BackendName)
			}
			if req.BackendSetName == nil {
				t.Fatalf("backend set name is nil")
			}
			name := *req.BackendSetName
			if name != "set-a" && name != "set-old" {
				t.Fatalf("unexpected backend set name: %q", name)
			}
			seen[name] = true
			return networkloadbalancer.DeleteBackendResponse{
				OpcWorkRequestId: common.String("wrid-" + name),
			}, nil
		},
	)
	nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
		func(_ context.Context, req networkloadbalancer.GetWorkRequestRequest) (networkloadbalancer.GetWorkRequestResponse, error) {
			if req.WorkRequestId == nil {
				t.Fatalf("work request id is nil")
			}
			return networkloadbalancer.GetWorkRequestResponse{
				WorkRequest: networkloadbalancer.WorkRequest{Status: networkloadbalancer.OperationStatusSucceeded},
			}, nil
		},
	)

	g.Expect(ms.ReconcileDeleteInstanceOnLB(context.Background())).To(Succeed())
	g.Expect(seen["set-a"]).To(BeTrue())
	g.Expect(seen["set-old"]).To(BeTrue())
}

func TestNLBCreateMembership_DefaultAndSecondaryBackendSetNaming(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	nlbClient := mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
	wrClient := mock_workrequests.NewMockClient(mockCtrl)
	client := fake.NewClientBuilder().Build()
	secondaryPort := int32(9345)
	ociCluster := &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{UID: "uid"},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			NetworkSpec: infrastructurev1beta2.NetworkSpec{
				APIServerLB: infrastructurev1beta2.LoadBalancer{
					LoadBalancerId: common.String("nlbid"),
					NLBSpec: infrastructurev1beta2.NLBSpec{
						BackendSets: []infrastructurev1beta2.NLBBackendSet{
							{Name: APIServerLBBackendSetName},
							{Name: "apiserver-lb-backendset-2", ListenerPort: &secondaryPort},
						},
					},
				},
			},
		},
	}
	ociCluster.Spec.ControlPlaneEndpoint.Port = 6443

	ms, err := NewMachineScope(MachineScopeParams{
		NetworkLoadBalancerClient: nlbClient,
		WorkRequestsClient:        wrClient,
		OCIMachine: &infrastructurev1beta2.OCIMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "test", UID: "uid"},
		},
		Machine: &clusterv1.Machine{},
		Cluster: &clusterv1.Cluster{},
		OCIClusterAccessor: OCISelfManagedCluster{
			OCICluster: ociCluster,
		},
		Client: client,
	})
	g.Expect(err).To(BeNil())
	ms.OCIMachine.Status.Addresses = []clusterv1beta1.MachineAddress{{
		Type:    clusterv1beta1.MachineInternalIP,
		Address: "1.1.1.1",
	}}

	nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: common.String("nlbid"),
	})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
		NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
			BackendSets: map[string]networkloadbalancer.BackendSet{
				APIServerLBBackendSetName:   {Name: common.String(APIServerLBBackendSetName), Backends: []networkloadbalancer.Backend{}},
				"apiserver-lb-backendset-2": {Name: common.String("apiserver-lb-backendset-2"), Backends: []networkloadbalancer.Backend{}},
			},
		},
	}, nil)

	nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(networkloadbalancer.CreateBackendRequest{
		NetworkLoadBalancerId: common.String("nlbid"),
		BackendSetName:        common.String(APIServerLBBackendSetName),
		CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
			IpAddress: common.String("1.1.1.1"),
			Port:      common.Int(6443),
			Name:      common.String("test"),
		},
		OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s-%s", "create-backend", "uid", stableShortHash(APIServerLBBackendSetName+":6443")),
	})).Return(networkloadbalancer.CreateBackendResponse{OpcWorkRequestId: common.String("wrid-default")}, nil)
	nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
		WorkRequestId: common.String("wrid-default"),
	})).Return(networkloadbalancer.GetWorkRequestResponse{
		WorkRequest: networkloadbalancer.WorkRequest{Status: networkloadbalancer.OperationStatusSucceeded},
	}, nil)

	nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(networkloadbalancer.CreateBackendRequest{
		NetworkLoadBalancerId: common.String("nlbid"),
		BackendSetName:        common.String("apiserver-lb-backendset-2"),
		CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
			IpAddress: common.String("1.1.1.1"),
			Port:      common.Int(9345),
			Name:      common.String(fmt.Sprintf("test-%s", stableShortHash("apiserver-lb-backendset-2"))),
		},
		OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s-%s", "create-backend", "uid", stableShortHash("apiserver-lb-backendset-2:9345")),
	})).Return(networkloadbalancer.CreateBackendResponse{OpcWorkRequestId: common.String("wrid-secondary")}, nil)
	nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(networkloadbalancer.GetWorkRequestRequest{
		WorkRequestId: common.String("wrid-secondary"),
	})).Return(networkloadbalancer.GetWorkRequestResponse{
		WorkRequest: networkloadbalancer.WorkRequest{Status: networkloadbalancer.OperationStatusSucceeded},
	}, nil)

	g.Expect(ms.ReconcileCreateInstanceOnLB(context.Background())).To(Succeed())
}
