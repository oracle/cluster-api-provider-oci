/*
 Copyright (c) 2022 Oracle and/or its affiliates.

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

package scope

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine/mock_containerengine"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestVirtualMachinePoolCreate(t *testing.T) {
	var (
		ms        *VirtualMachinePoolScope
		mockCtrl  *gomock.Controller
		okeClient *mock_containerengine.MockClient
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

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

	definedTagsInterface := make(map[string]map[string]interface{})
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
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		ociManagedCluster := &infrastructurev1beta2.OCIManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "cluster_uid",
			},
			Spec: infrastructurev1beta2.OCIManagedClusterSpec{
				CompartmentId: "test-compartment",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn-id"),
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.WorkerRole,
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
									Role: infrastructurev1beta2.WorkerRole,
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
		client := fake.NewClientBuilder().WithObjects().Build()
		size := int32(3)

		ms, err = NewVirtualMachinePoolScope(VirtualMachinePoolScopeParams{
			ContainerEngineClient: okeClient,
			OCIManagedControlPlane: &infrastructurev1beta2.OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrastructurev1beta2.OCIManagedControlPlaneSpec{
					ID: common.String("cluster-id"),
				},
			},
			OCIVirtualMachinePool: &infrav2exp.OCIVirtualMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrav2exp.OCIVirtualMachinePoolSpec{},
			},
			OCIManagedCluster: ociManagedCluster,
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{},
			},
			MachinePool: &clusterv1.MachinePool{
				Spec: clusterv1.MachinePoolSpec{
					Replicas: &size,
				},
			},
			Client: client,
		})
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name                string
		errorExpected       bool
		objects             []client.Object
		expectedEvent       string
		eventNotExpected    string
		matchError          error
		errorSubStringMatch bool
		testSpecificSetup   func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient)
	}{
		{
			name:          "virtual nodepool create all",
			errorExpected: false,
			testSpecificSetup: func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIVirtualMachinePool.Spec = infrav2exp.OCIVirtualMachinePoolSpec{
					NsgNames: []string{"worker-nsg"},
					InitialVirtualNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					Taints: []infrav2exp.Taint{{
						Key:    common.String("key"),
						Value:  common.String("value"),
						Effect: common.String("effect"),
					}},
					PodConfiguration: infrav2exp.PodConfig{
						NsgNames:   []string{"pod-nsg"},
						Shape:      common.String("pod-shape"),
						SubnetName: common.String("pod-subnet"),
					},
					PlacementConfigs: []infrav2exp.VirtualNodepoolPlacementConfig{
						{
							AvailabilityDomain: common.String("test-ad"),
							SubnetName:         common.String("worker-subnet"),
							FaultDomains:       []string{"fd-1", "fd-2"},
						},
					},
				}
				okeClient.EXPECT().CreateVirtualNodePool(gomock.Any(), gomock.Eq(oke.CreateVirtualNodePoolRequest{
					CreateVirtualNodePoolDetails: oke.CreateVirtualNodePoolDetails{
						ClusterId:     common.String("cluster-id"),
						DisplayName:   common.String("test"),
						CompartmentId: common.String("test-compartment"),
						InitialVirtualNodeLabels: []oke.InitialVirtualNodeLabel{{
							Key:   common.String("key"),
							Value: common.String("value")}},
						Taints: []oke.Taint{{
							Key:    common.String("key"),
							Value:  common.String("value"),
							Effect: common.String("effect")}},
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
						NsgIds:       []string{"nsg-id"},
						Size:         common.Int(3),
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
						VirtualNodeTags: &oke.VirtualNodeTags{
							FreeformTags: tags,
							DefinedTags:  definedTagsInterface,
						},
					},
				})).
					Return(oke.CreateVirtualNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)

				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(oke.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-work-request-id"),
				})).
					Return(oke.GetWorkRequestResponse{
						WorkRequest: oke.WorkRequest{
							Resources: []oke.WorkRequestResource{
								{
									Identifier: common.String("oke-virtual-np-id"),
									EntityType: common.String("VirtualNodePool"),
								},
							},
						},
					}, nil)
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("oke-virtual-np-id"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:           common.String("oke-np-id"),
							FreeformTags: tags,
						},
					}, nil)
			},
		},
		{
			name:          "virtual nodepool default placement",
			errorExpected: false,
			testSpecificSetup: func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIVirtualMachinePool.Spec = infrav2exp.OCIVirtualMachinePoolSpec{
					NsgNames: []string{"worker-nsg"},
					InitialVirtualNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					Taints: []infrav2exp.Taint{{
						Key:    common.String("key"),
						Value:  common.String("value"),
						Effect: common.String("effect"),
					}},
					PodConfiguration: infrav2exp.PodConfig{
						NsgNames:   []string{"pod-nsg"},
						Shape:      common.String("pod-shape"),
						SubnetName: common.String("pod-subnet"),
					},
				}
				okeClient.EXPECT().CreateVirtualNodePool(gomock.Any(), gomock.Eq(oke.CreateVirtualNodePoolRequest{
					CreateVirtualNodePoolDetails: oke.CreateVirtualNodePoolDetails{
						ClusterId:     common.String("cluster-id"),
						DisplayName:   common.String("test"),
						CompartmentId: common.String("test-compartment"),
						InitialVirtualNodeLabels: []oke.InitialVirtualNodeLabel{{
							Key:   common.String("key"),
							Value: common.String("value")}},
						Taints: []oke.Taint{{
							Key:    common.String("key"),
							Value:  common.String("value"),
							Effect: common.String("effect")}},
						PlacementConfigurations: []oke.PlacementConfiguration{
							{
								AvailabilityDomain: common.String("ad-1"),
								SubnetId:           common.String("subnet-id"),
								FaultDomain:        []string{"fd-5", "fd-6"},
							},
						},
						PodConfiguration: &oke.PodConfiguration{
							NsgIds:   []string{"pod-nsg-id"},
							Shape:    common.String("pod-shape"),
							SubnetId: common.String("pod-subnet-id"),
						},
						Size:         common.Int(3),
						NsgIds:       []string{"nsg-id"},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
						VirtualNodeTags: &oke.VirtualNodeTags{
							FreeformTags: tags,
							DefinedTags:  definedTagsInterface,
						},
					},
				})).
					Return(oke.CreateVirtualNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)

				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(oke.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-work-request-id"),
				})).
					Return(oke.GetWorkRequestResponse{
						WorkRequest: oke.WorkRequest{
							Resources: []oke.WorkRequestResource{
								{
									Identifier: common.String("oke-virtual-np-id"),
									EntityType: common.String("VirtualNodePool"),
								},
							},
						},
					}, nil)
				okeClient.EXPECT().GetVirtualNodePool(gomock.Any(), gomock.Eq(oke.GetVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("oke-virtual-np-id"),
				})).
					Return(oke.GetVirtualNodePoolResponse{
						VirtualNodePool: oke.VirtualNodePool{
							Id:           common.String("oke-np-id"),
							FreeformTags: tags,
						},
					}, nil)
			},
		},
		{
			name:          "virtual nodepool no worker subnets",
			errorExpected: true,
			matchError:    errors.New("worker subnets are not specified"),
			testSpecificSetup: func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets = []*infrastructurev1beta2.Subnet{}
				ms.OCIVirtualMachinePool.Spec = infrav2exp.OCIVirtualMachinePoolSpec{}
			},
		},
		{
			name:          "virtual nodepool no worker subnets",
			errorExpected: true,
			matchError: errors.New(fmt.Sprintf("worker subnet with name %s is not present in spec",
				"worker-subnet-invalid")),
			testSpecificSetup: func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIVirtualMachinePool.Spec = infrav2exp.OCIVirtualMachinePoolSpec{
					PlacementConfigs: []infrav2exp.VirtualNodepoolPlacementConfig{
						{
							AvailabilityDomain: common.String("test-ad"),
							SubnetName:         common.String("worker-subnet-invalid"),
							FaultDomains:       []string{"fd-1", "fd-2"},
						},
					},
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, okeClient)
			_, err := ms.CreateVirtualNodePool(context.Background())
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
				if tc.errorSubStringMatch {
					g.Expect(err.Error()).To(ContainSubstring(tc.matchError.Error()))
				} else {
					g.Expect(err.Error()).To(Equal(tc.matchError.Error()))
				}
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestVirtualMachinePoolUpdate(t *testing.T) {
	var (
		ms        *VirtualMachinePoolScope
		mockCtrl  *gomock.Controller
		okeClient *mock_containerengine.MockClient
	)

	tags := make(map[string]string)
	tags[ociutil.CreatedBy] = ociutil.OCIClusterAPIProvider
	tags[ociutil.ClusterResourceIdentifier] = "resource_uid"

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

	definedTagsInterface := make(map[string]map[string]interface{})
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
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		ociManagedCluster := &infrastructurev1beta2.OCIManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "cluster_uid",
			},
			Spec: infrastructurev1beta2.OCIManagedClusterSpec{
				CompartmentId: "test-compartment",
				DefinedTags:   definedTags,
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn-id"),
						Subnets: []*infrastructurev1beta2.Subnet{
							{
								Role: infrastructurev1beta2.WorkerRole,
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
									Role: infrastructurev1beta2.WorkerRole,
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
		client := fake.NewClientBuilder().WithObjects().Build()
		size := int32(3)

		ms, err = NewVirtualMachinePoolScope(VirtualMachinePoolScopeParams{
			ContainerEngineClient: okeClient,
			OCIManagedControlPlane: &infrastructurev1beta2.OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrastructurev1beta2.OCIManagedControlPlaneSpec{
					ID: common.String("cluster-id"),
				},
			},
			OCIVirtualMachinePool: &infrav2exp.OCIVirtualMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrav2exp.OCIVirtualMachinePoolSpec{},
			},
			OCIManagedCluster: ociManagedCluster,
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{},
			},
			MachinePool: &clusterv1.MachinePool{
				Spec: clusterv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{},
					Replicas: &size,
				},
			},
			Client: client,
		})
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name                string
		errorExpected       bool
		objects             []client.Object
		expectedEvent       string
		eventNotExpected    string
		matchError          error
		errorSubStringMatch bool
		nodePool            oke.VirtualNodePool
		testSpecificSetup   func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient)
	}{
		{
			name:          "virtual nodepool no change",
			errorExpected: false,
			testSpecificSetup: func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIVirtualMachinePool.Spec = infrav2exp.OCIVirtualMachinePoolSpec{
					ID:       common.String("test"),
					NsgNames: []string{"worker-nsg"},
					InitialVirtualNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					Taints: []infrav2exp.Taint{{
						Key:    common.String("key"),
						Value:  common.String("value"),
						Effect: common.String("effect"),
					}},
					PodConfiguration: infrav2exp.PodConfig{
						NsgNames:   []string{"pod-nsg"},
						Shape:      common.String("pod-shape"),
						SubnetName: common.String("pod-subnet"),
					},
					PlacementConfigs: []infrav2exp.VirtualNodepoolPlacementConfig{
						{
							AvailabilityDomain: common.String("test-ad"),
							SubnetName:         common.String("worker-subnet"),
							FaultDomains:       []string{"fd-1", "fd-2"},
						},
					},
				}
			},
			nodePool: oke.VirtualNodePool{
				Id:                common.String("test"),
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
		},
		{
			name:          "update due to change in replica size",
			errorExpected: false,
			testSpecificSetup: func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				newReplicas := int32(4)
				ms.MachinePool.Spec.Replicas = &newReplicas
				ms.OCIVirtualMachinePool.Spec = infrav2exp.OCIVirtualMachinePoolSpec{
					ID:       common.String("test"),
					NsgNames: []string{"worker-nsg"},
					InitialVirtualNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					Taints: []infrav2exp.Taint{{
						Key:    common.String("key"),
						Value:  common.String("value"),
						Effect: common.String("effect"),
					}},
					PodConfiguration: infrav2exp.PodConfig{
						NsgNames:   []string{"pod-nsg"},
						Shape:      common.String("pod-shape"),
						SubnetName: common.String("pod-subnet"),
					},
					PlacementConfigs: []infrav2exp.VirtualNodepoolPlacementConfig{
						{
							AvailabilityDomain: common.String("test-ad"),
							SubnetName:         common.String("worker-subnet"),
							FaultDomains:       []string{"fd-1", "fd-2"},
						},
					},
				}
				okeClient.EXPECT().UpdateVirtualNodePool(gomock.Any(), gomock.Eq(oke.UpdateVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
					UpdateVirtualNodePoolDetails: oke.UpdateVirtualNodePoolDetails{
						DisplayName: common.String("test"),
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
						Size: common.Int(4),
					},
				})).
					Return(oke.UpdateVirtualNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.VirtualNodePool{
				Id:                common.String("test"),
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
		},
		{
			name:          "no update due to change in replica size as annotation is set",
			errorExpected: false,
			testSpecificSetup: func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.MachinePool.Annotations = make(map[string]string)
				ms.MachinePool.Annotations[clusterv1beta1.ReplicasManagedByAnnotation] = ""
				newReplicas := int32(4)
				ms.MachinePool.Spec.Replicas = &newReplicas
				ms.OCIVirtualMachinePool.Spec = infrav2exp.OCIVirtualMachinePoolSpec{
					ID:       common.String("test"),
					NsgNames: []string{"worker-nsg"},
					InitialVirtualNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					Taints: []infrav2exp.Taint{{
						Key:    common.String("key"),
						Value:  common.String("value"),
						Effect: common.String("effect"),
					}},
					PodConfiguration: infrav2exp.PodConfig{
						NsgNames:   []string{"pod-nsg"},
						Shape:      common.String("pod-shape"),
						SubnetName: common.String("pod-subnet"),
					},
					PlacementConfigs: []infrav2exp.VirtualNodepoolPlacementConfig{
						{
							AvailabilityDomain: common.String("test-ad"),
							SubnetName:         common.String("worker-subnet"),
							FaultDomains:       []string{"fd-1", "fd-2"},
						},
					},
				}
			},
			nodePool: oke.VirtualNodePool{
				Id:                common.String("test"),
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
		},
		{
			name:          "update due to change in placement config",
			errorExpected: false,
			testSpecificSetup: func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIVirtualMachinePool.Spec = infrav2exp.OCIVirtualMachinePoolSpec{
					ID:       common.String("test"),
					NsgNames: []string{"worker-nsg"},
					InitialVirtualNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					Taints: []infrav2exp.Taint{{
						Key:    common.String("key"),
						Value:  common.String("value"),
						Effect: common.String("effect"),
					}},
					PodConfiguration: infrav2exp.PodConfig{
						NsgNames:   []string{"pod-nsg"},
						Shape:      common.String("pod-shape"),
						SubnetName: common.String("pod-subnet"),
					},
					PlacementConfigs: []infrav2exp.VirtualNodepoolPlacementConfig{
						{
							AvailabilityDomain: common.String("test-ad"),
							SubnetName:         common.String("worker-subnet"),
							FaultDomains:       []string{"fd-1", "fd-2", "fd-3"},
						},
					},
				}
				okeClient.EXPECT().UpdateVirtualNodePool(gomock.Any(), gomock.Eq(oke.UpdateVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
					UpdateVirtualNodePoolDetails: oke.UpdateVirtualNodePoolDetails{
						DisplayName: common.String("test"),
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
								FaultDomain:        []string{"fd-1", "fd-2", "fd-3"},
							},
						},
						NsgIds: []string{"nsg-id"},
						PodConfiguration: &oke.PodConfiguration{
							NsgIds:   []string{"pod-nsg-id"},
							Shape:    common.String("pod-shape"),
							SubnetId: common.String("pod-subnet-id"),
						},
						Size: common.Int(3),
					},
				})).
					Return(oke.UpdateVirtualNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.VirtualNodePool{
				Id:                common.String("test"),
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
		},
		{
			name:          "update due to change in name",
			errorExpected: false,
			testSpecificSetup: func(cs *VirtualMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIVirtualMachinePool.Name = "changed"
				ms.OCIVirtualMachinePool.Spec = infrav2exp.OCIVirtualMachinePoolSpec{
					ID:       common.String("test"),
					NsgNames: []string{"worker-nsg"},
					InitialVirtualNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					Taints: []infrav2exp.Taint{{
						Key:    common.String("key"),
						Value:  common.String("value"),
						Effect: common.String("effect"),
					}},
					PodConfiguration: infrav2exp.PodConfig{
						NsgNames:   []string{"pod-nsg"},
						Shape:      common.String("pod-shape"),
						SubnetName: common.String("pod-subnet"),
					},
					PlacementConfigs: []infrav2exp.VirtualNodepoolPlacementConfig{
						{
							AvailabilityDomain: common.String("test-ad"),
							SubnetName:         common.String("worker-subnet"),
							FaultDomains:       []string{"fd-1", "fd-2"},
						},
					},
				}
				okeClient.EXPECT().UpdateVirtualNodePool(gomock.Any(), gomock.Eq(oke.UpdateVirtualNodePoolRequest{
					VirtualNodePoolId: common.String("test"),
					UpdateVirtualNodePoolDetails: oke.UpdateVirtualNodePoolDetails{
						DisplayName: common.String("changed"),
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
						Size: common.Int(3),
					},
				})).
					Return(oke.UpdateVirtualNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.VirtualNodePool{
				Id:                common.String("test"),
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
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, okeClient)
			_, err := ms.UpdateVirtualNodePool(context.Background(), &tc.nodePool)
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
				if tc.errorSubStringMatch {
					g.Expect(err.Error()).To(ContainSubstring(tc.matchError.Error()))
				} else {
					g.Expect(err.Error()).To(Equal(tc.matchError.Error()))
				}
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}
