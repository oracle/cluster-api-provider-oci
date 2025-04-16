package scope

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute/mock_compute"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeleteNpn(t *testing.T) {

	var (
		ms            *MachineScope
		mockCtrl      *gomock.Controller
		computeClient *mock_compute.MockComputeClient
		ociCluster    infrastructurev1beta2.OCICluster
		mockWlClient  client.Client
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-kubeconfig",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"value": []byte(`
apiVersion: v1
clusters:
- cluster:
    server: https://fake-server:6443
  name: fake-cluster
contexts:
- context:
    cluster: fake-cluster
    user: fake-user
  name: fake-context
current-context: fake-context
kind: Config
preferences: {}
users:
- name: fake-user
  user:
    token: eyfgjakbvka
`),
			},
		}

		mockCtrl = gomock.NewController(t)
		computeClient = mock_compute.NewMockComputeClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects(secret).Build()
		ociCluster = infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				OCIResourceIdentifier: "resource_uid",
			},
		}
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ms, err = NewMachineScope(MachineScopeParams{
			ComputeClient: computeClient,
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test",
				},
			},
			Machine: &clusterv1.Machine{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: common.String("bootstrap"),
					},
				},
			},
			Cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: v1beta1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "test",
					},
				},
			},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
		})
		ms.Machine.Namespace = "default"
		g.Expect(err).To(BeNil())
		mockWlClient = fake.NewClientBuilder().WithObjects(secret).Build()
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name              string
		expectedError     bool
		testSpecificSetup func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient, mockClient client.Client)
	}{
		{
			name: "successful deletion",
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient, mockWlClient client.Client) {
				machineScope.OCIMachine.Spec.InstanceId = common.String("ocid1.bsndhqhdiq")
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("ocid1.bsndhqhdiq"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id: common.String("ocid1.bsndhqhdiq"),
						},
					}, nil)
				npnCrCreate := &unstructured.Unstructured{}
				npnCrCreate.Object = map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "bsndhqhdiq",
					},
					"spec": map[string]interface{}{
						"id": "ocid1.bsndhqhdiq",
					},
				}
				npnCrCreate.SetGroupVersionKind(schema.GroupVersionKind{
					Version: "oci.oraclecloud.com/v1beta1",
					Kind:    "NativePodNetwork",
				})
				mockWlClient.Create(context.Background(), npnCrCreate)
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tt.testSpecificSetup(ms, computeClient, mockWlClient)

			err := ms.DeleteNpn(context.Background(), mockWlClient)
			if tt.expectedError {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestHasNpnCrd(t *testing.T) {

	var (
		ms            *MachineScope
		mockCtrl      *gomock.Controller
		computeClient *mock_compute.MockComputeClient
		ociCluster    infrastructurev1beta2.OCICluster
		mockWlClient  client.Client
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-kubeconfig",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"value": []byte(`
apiVersion: v1
clusters:
- cluster:
    server: https://fake-server:6443
  name: fake-cluster
contexts:
- context:
    cluster: fake-cluster
    user: fake-user
  name: fake-context
current-context: fake-context
kind: Config
preferences: {}
users:
- name: fake-user
  user:
    token: eyfgjakbvka
`),
			},
		}

		mockCtrl = gomock.NewController(t)
		computeClient = mock_compute.NewMockComputeClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects(secret).Build()
		ociCluster = infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				OCIResourceIdentifier: "resource_uid",
			},
		}
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ms, err = NewMachineScope(MachineScopeParams{
			ComputeClient: computeClient,
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test",
				},
			},
			Machine: &clusterv1.Machine{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: common.String("bootstrap"),
					},
				},
			},
			Cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: v1beta1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "test",
					},
				},
			},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
		})
		ms.Machine.Namespace = "default"
		g.Expect(err).To(BeNil())
		mockWlClient = fake.NewClientBuilder().WithObjects(secret).Build()
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name              string
		expectedError     bool
		expectedResult    bool
		testSpecificSetup func(machineScope *MachineScope, mockClient client.Client)
	}{
		{
			name:           "CRD exists",
			expectedResult: true,
			testSpecificSetup: func(machineScope *MachineScope, mockWlClient client.Client) {
				npnCrd := &unstructured.Unstructured{}
				npnCrd.SetGroupVersionKind(schema.GroupVersionKind{
					Version: "apiextensions.k8s.io/v1",
					Kind:    "CustomResourceDefinition",
				})
				npnCrd.SetName("nativepodnetworks.oci.oraclecloud.com")
				mockWlClient.Create(context.Background(), npnCrd)
			},
			expectedError: false,
		},
		{
			name:           "CRD does not exist",
			expectedResult: false,
			testSpecificSetup: func(machineScope *MachineScope, mockWlClient client.Client) {
				// No CRD is created in this case
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tt.testSpecificSetup(ms, mockWlClient)

			result, err := ms.HasNpnCrd(context.Background(), mockWlClient)
			if tt.expectedError {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(result).To(Equal(tt.expectedResult))
			}
		})
	}
}

func TestGetOrCreateNpn(t *testing.T) {

	var (
		ms            *MachineScope
		mockCtrl      *gomock.Controller
		computeClient *mock_compute.MockComputeClient
		ociCluster    infrastructurev1beta2.OCICluster
		mockWlClient  client.Client
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-kubeconfig",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"value": []byte(`
apiVersion: v1
clusters:
- cluster:
    server: https://fake-server:6443
  name: fake-cluster
contexts:
- context:
    cluster: fake-cluster
    user: fake-user
  name: fake-context
current-context: fake-context
kind: Config
preferences: {}
users:
- name: fake-user
  user:
    token: eyfgjakbvka
`),
			},
		}

		mockCtrl = gomock.NewController(t)
		computeClient = mock_compute.NewMockComputeClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects(secret).Build()
		ociCluster = infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				OCIResourceIdentifier: "resource_uid",
			},
		}
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ms, err = NewMachineScope(MachineScopeParams{
			ComputeClient: computeClient,
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test",
					MaxPodPerNode: 10,
					PodSubnetIds:  []string{"subnet1", "subnet2"},
					PodNSGIds:     []string{"nsg1", "nsg2"},
				},
			},
			Machine: &clusterv1.Machine{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: common.String("bootstrap"),
					},
				},
			},
			Cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: v1beta1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "test",
					},
				},
			},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
		})
		ms.Machine.Namespace = "default"
		g.Expect(err).To(BeNil())
		mockWlClient = fake.NewClientBuilder().WithObjects(secret).Build()
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name              string
		expectedError     bool
		testSpecificSetup func(machineScope *MachineScope, mockClient client.Client)
	}{
		{
			name: "NPN CR already exists",
			testSpecificSetup: func(machineScope *MachineScope, mockWlClient client.Client) {
				npnCr := &unstructured.Unstructured{}
				npnCr.SetGroupVersionKind(schema.GroupVersionKind{
					Version: "oci.oraclecloud.com/v1beta1",
					Kind:    "NativePodNetwork",
				})
				npnCr.SetName("bsndhqhdiq")
				mockWlClient.Create(context.Background(), npnCr)
				machineScope.OCIMachine.Spec.InstanceId = common.String("ocid1.bsndhqhdiq")
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("ocid1.bsndhqhdiq"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id: common.String("ocid1.bsndhqhdiq"),
						},
					}, nil)
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tt.testSpecificSetup(ms, mockWlClient)

			result, err := ms.GetOrCreateNpn(context.Background(), mockWlClient)
			if tt.expectedError {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(result).To(Not(BeNil()))
			}
		})
	}
}
