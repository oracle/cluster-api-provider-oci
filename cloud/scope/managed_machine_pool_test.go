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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManagedMachinePoolCreate(t *testing.T) {
	var (
		ms        *ManagedMachinePoolScope
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

		ms, err = NewManagedMachinePoolScope(ManagedMachinePoolScopeParams{
			ContainerEngineClient: okeClient,
			OCIManagedControlPlane: &infrastructurev1beta2.OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrastructurev1beta2.OCIManagedControlPlaneSpec{
					ID: common.String("cluster-id"),
				},
			},
			OCIManagedMachinePool: &infrav2exp.OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrav2exp.OCIManagedMachinePoolSpec{
					Version: common.String("v1.24.5"),
				},
			},
			OCIManagedCluster: ociManagedCluster,
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{},
			},
			MachinePool: &expclusterv1.MachinePool{
				Spec: expclusterv1.MachinePoolSpec{
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
		testSpecificSetup   func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient)
	}{
		{
			name:          "nodepool create all",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						ImageId:             common.String("test-image-id"),
						BootVolumeSizeInGBs: common.Int64(75),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(25),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
				okeClient.EXPECT().CreateNodePool(gomock.Any(), gomock.Eq(oke.CreateNodePoolRequest{
					CreateNodePoolDetails: oke.CreateNodePoolDetails{
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
						NodeShapeConfig: &oke.CreateNodeShapeConfigDetails{
							Ocpus:       common.Float32(2),
							MemoryInGBs: common.Float32(16),
						},
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId:             common.String("test-image-id"),
							BootVolumeSizeInGBs: common.Int64(75),
						},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
						SshPublicKey: common.String("test-ssh-public-key"),
						NodeConfigDetails: &oke.CreateNodePoolNodeConfigDetails{
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
							DefinedTags:                    definedTagsInterface,
							NodePoolPodNetworkOptionDetails: oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
								PodSubnetIds:   []string{"pod-subnet-id"},
								MaxPodsPerNode: common.Int(25),
								PodNsgIds:      []string{"pod-nsg-id"},
							},
						},
						NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
							EvictionGraceDuration:           common.String("PT30M"),
							IsForceDeleteAfterGraceDuration: common.Bool(true),
						},
					},
				})).
					Return(oke.CreateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)

				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(oke.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-work-request-id"),
				})).
					Return(oke.GetWorkRequestResponse{
						WorkRequest: oke.WorkRequest{
							Resources: []oke.WorkRequestResource{
								{
									Identifier: common.String("oke-np-id"),
									EntityType: common.String("nodepool"),
								},
							},
						},
					}, nil)
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("oke-np-id"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:           common.String("oke-np-id"),
							FreeformTags: tags,
						},
					}, nil)
			},
		},
		{
			name:          "nodepool lookup image",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						BootVolumeSizeInGBs: common.Int64(75),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(25),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
				okeClient.EXPECT().GetNodePoolOptions(gomock.Any(), gomock.Eq(oke.GetNodePoolOptionsRequest{
					CompartmentId:    common.String("test-compartment"),
					NodePoolOptionId: common.String("all"),
				})).
					Return(oke.GetNodePoolOptionsResponse{
						NodePoolOptions: oke.NodePoolOptions{
							Sources: []oke.NodeSourceOption{
								oke.NodeSourceViaImageOption{
									SourceName: common.String("Oracle-Linux-8.6-aarch64-2022.12.15-0-OKE-1.24.5-543"),
									ImageId:    common.String("image-id-1"),
								},
								oke.NodeSourceViaImageOption{
									SourceName: common.String("Oracle-Linux-8.6-Gen2-GPU-2022.12.16-0-OKE-1.24.5-543"),
									ImageId:    common.String("image-id-2"),
								},
								oke.NodeSourceViaImageOption{
									SourceName: common.String("Oracle-Linux-8.6-2022.12.15-0-OKE-1.24.5-543"),
									ImageId:    common.String("image-id-3"),
								},
							},
						},
					}, nil)
				okeClient.EXPECT().CreateNodePool(gomock.Any(), gomock.Eq(oke.CreateNodePoolRequest{
					CreateNodePoolDetails: oke.CreateNodePoolDetails{
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
						NodeShapeConfig: &oke.CreateNodeShapeConfigDetails{
							Ocpus:       common.Float32(2),
							MemoryInGBs: common.Float32(16),
						},
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId:             common.String("image-id-3"),
							BootVolumeSizeInGBs: common.Int64(75),
						},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
						SshPublicKey: common.String("test-ssh-public-key"),
						NodeConfigDetails: &oke.CreateNodePoolNodeConfigDetails{
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
							DefinedTags:                    definedTagsInterface,
							NodePoolPodNetworkOptionDetails: oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
								PodSubnetIds:   []string{"pod-subnet-id"},
								MaxPodsPerNode: common.Int(25),
								PodNsgIds:      []string{"pod-nsg-id"},
							},
						},
						NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
							EvictionGraceDuration:           common.String("PT30M"),
							IsForceDeleteAfterGraceDuration: common.Bool(true),
						},
					},
				})).
					Return(oke.CreateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)

				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(oke.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-work-request-id"),
				})).
					Return(oke.GetWorkRequestResponse{
						WorkRequest: oke.WorkRequest{
							Resources: []oke.WorkRequestResource{
								{
									Identifier: common.String("oke-np-id"),
									EntityType: common.String("nodepool"),
								},
							},
						},
					}, nil)
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("oke-np-id"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:           common.String("oke-np-id"),
							FreeformTags: tags,
						},
					}, nil)
			},
		},
		{
			name:          "nodepool lookup image arm",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape-A1",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						BootVolumeSizeInGBs: common.Int64(75),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(25),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
				okeClient.EXPECT().GetNodePoolOptions(gomock.Any(), gomock.Eq(oke.GetNodePoolOptionsRequest{
					CompartmentId:    common.String("test-compartment"),
					NodePoolOptionId: common.String("all"),
				})).
					Return(oke.GetNodePoolOptionsResponse{
						NodePoolOptions: oke.NodePoolOptions{
							Sources: []oke.NodeSourceOption{
								oke.NodeSourceViaImageOption{
									SourceName: common.String("Oracle-Linux-8.6-aarch64-2022.12.15-0-OKE-1.24.5-543"),
									ImageId:    common.String("image-id-1"),
								},
								oke.NodeSourceViaImageOption{
									SourceName: common.String("Oracle-Linux-8.6-Gen2-GPU-2022.12.16-0-OKE-1.24.5-543"),
									ImageId:    common.String("image-id-2"),
								},
								oke.NodeSourceViaImageOption{
									SourceName: common.String("Oracle-Linux-8.6-2022.12.15-0-OKE-1.24.5-543"),
									ImageId:    common.String("image-id-3"),
								},
							},
						},
					}, nil)
				okeClient.EXPECT().CreateNodePool(gomock.Any(), gomock.Eq(oke.CreateNodePoolRequest{
					CreateNodePoolDetails: oke.CreateNodePoolDetails{
						ClusterId:         common.String("cluster-id"),
						Name:              common.String("test"),
						CompartmentId:     common.String("test-compartment"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeMetadata:      map[string]string{"key1": "value1"},
						InitialNodeLabels: []oke.KeyValue{{
							Key:   common.String("key"),
							Value: common.String("value"),
						}},
						NodeShape: common.String("test-shape-A1"),
						NodeShapeConfig: &oke.CreateNodeShapeConfigDetails{
							Ocpus:       common.Float32(2),
							MemoryInGBs: common.Float32(16),
						},
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId:             common.String("image-id-1"),
							BootVolumeSizeInGBs: common.Int64(75),
						},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
						SshPublicKey: common.String("test-ssh-public-key"),
						NodeConfigDetails: &oke.CreateNodePoolNodeConfigDetails{
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
							DefinedTags:                    definedTagsInterface,
							NodePoolPodNetworkOptionDetails: oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
								PodSubnetIds:   []string{"pod-subnet-id"},
								MaxPodsPerNode: common.Int(25),
								PodNsgIds:      []string{"pod-nsg-id"},
							},
						},
						NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
							EvictionGraceDuration:           common.String("PT30M"),
							IsForceDeleteAfterGraceDuration: common.Bool(true),
						},
					},
				})).
					Return(oke.CreateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)

				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(oke.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-work-request-id"),
				})).
					Return(oke.GetWorkRequestResponse{
						WorkRequest: oke.WorkRequest{
							Resources: []oke.WorkRequestResource{
								{
									Identifier: common.String("oke-np-id"),
									EntityType: common.String("nodepool"),
								},
							},
						},
					}, nil)
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("oke-np-id"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:           common.String("oke-np-id"),
							FreeformTags: tags,
						},
					}, nil)
			},
		},
		{
			name:                "nodepool lookup image - error as image lookup failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("could not lookup nodepool image id from nodepool options"),
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape-A1",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						BootVolumeSizeInGBs: common.Int64(75),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(25),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
				okeClient.EXPECT().GetNodePoolOptions(gomock.Any(), gomock.Eq(oke.GetNodePoolOptionsRequest{
					CompartmentId:    common.String("test-compartment"),
					NodePoolOptionId: common.String("all"),
				})).
					Return(oke.GetNodePoolOptionsResponse{
						NodePoolOptions: oke.NodePoolOptions{
							Sources: []oke.NodeSourceOption{
								oke.NodeSourceViaImageOption{
									SourceName: common.String("Oracle-Linux-8.6-aarch64-2022.12.15-0-OKE-1.25.5-543"),
									ImageId:    common.String("image-id-1"),
								},
								oke.NodeSourceViaImageOption{
									SourceName: common.String("Oracle-Linux-8.6-Gen2-GPU-2022.12.16-0-OKE-1.25.5-543"),
									ImageId:    common.String("image-id-2"),
								},
								oke.NodeSourceViaImageOption{
									SourceName: common.String("Oracle-Linux-8.6-2022.12.15-0-OKE-1.25.5-543"),
									ImageId:    common.String("image-id-3"),
								},
							},
						},
					}, nil)
			},
		},
		{
			name:          "nodepool default placement",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						ImageId: common.String("test-image-id"),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						NsgNames:                       []string{"worker-nsg"},
						KmsKeyId:                       common.String("kms-key-id"),
						IsPvEncryptionInTransitEnabled: common.Bool(true),
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(15),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
				okeClient.EXPECT().CreateNodePool(gomock.Any(), gomock.Eq(oke.CreateNodePoolRequest{
					CreateNodePoolDetails: oke.CreateNodePoolDetails{
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
						NodeShapeConfig: &oke.CreateNodeShapeConfigDetails{
							Ocpus:       common.Float32(2),
							MemoryInGBs: common.Float32(16),
						},
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId: common.String("test-image-id"),
						},
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
						SshPublicKey: common.String("test-ssh-public-key"),
						NodeConfigDetails: &oke.CreateNodePoolNodeConfigDetails{
							Size: common.Int(3),
							PlacementConfigs: []oke.NodePoolPlacementConfigDetails{
								{
									AvailabilityDomain: common.String("ad-1"),
									SubnetId:           common.String("subnet-id"),
									FaultDomains:       []string{"fd-5", "fd-6"},
								},
							},
							NsgIds:                         []string{"nsg-id"},
							KmsKeyId:                       common.String("kms-key-id"),
							IsPvEncryptionInTransitEnabled: common.Bool(true),
							FreeformTags:                   tags,
							DefinedTags:                    definedTagsInterface,
							NodePoolPodNetworkOptionDetails: oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
								PodSubnetIds:   []string{"pod-subnet-id"},
								MaxPodsPerNode: common.Int(15),
								PodNsgIds:      []string{"pod-nsg-id"},
							},
						},
						NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
							EvictionGraceDuration:           common.String("PT30M"),
							IsForceDeleteAfterGraceDuration: common.Bool(true),
						},
					},
				})).
					Return(oke.CreateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)

				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(oke.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-work-request-id"),
				})).
					Return(oke.GetWorkRequestResponse{
						WorkRequest: oke.WorkRequest{
							Resources: []oke.WorkRequestResource{
								{
									Identifier: common.String("oke-np-id"),
									EntityType: common.String("nodepool"),
								},
							},
						},
					}, nil)
				okeClient.EXPECT().GetNodePool(gomock.Any(), gomock.Eq(oke.GetNodePoolRequest{
					NodePoolId: common.String("oke-np-id"),
				})).
					Return(oke.GetNodePoolResponse{
						NodePool: oke.NodePool{
							Id:           common.String("oke-np-id"),
							FreeformTags: tags,
						},
					}, nil)
			},
		},
		{
			name:          "nodepool no worker subnets",
			errorExpected: true,
			matchError:    errors.New("worker subnets are not specified"),
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedCluster.Spec.NetworkSpec.Vcn.Subnets = []*infrastructurev1beta2.Subnet{}
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{}
			},
		},
		{
			name:          "nodepool no worker subnets",
			errorExpected: true,
			matchError: errors.New(fmt.Sprintf("worker subnet %s is not present in placementConfigs spec",
				"worker-subnet-invalid")),
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
							{
								AvailabilityDomain:    common.String("test-ad"),
								SubnetName:            common.String("worker-subnet-invalid"),
								CapacityReservationId: common.String("cap-id"),
								FaultDomains:          []string{"fd-1", "fd-2"},
							},
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
			_, err := ms.CreateNodePool(context.Background())
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

func TestManagedMachinePoolUpdate(t *testing.T) {
	var (
		ms        *ManagedMachinePoolScope
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

		ms, err = NewManagedMachinePoolScope(ManagedMachinePoolScopeParams{
			ContainerEngineClient: okeClient,
			OCIManagedControlPlane: &infrastructurev1beta2.OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrastructurev1beta2.OCIManagedControlPlaneSpec{
					ID: common.String("cluster-id"),
				},
			},
			OCIManagedMachinePool: &infrav2exp.OCIManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrav2exp.OCIManagedMachinePoolSpec{
					Version: common.String("v1.24.5"),
				},
			},
			OCIManagedCluster: ociManagedCluster,
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{},
			},
			MachinePool: &expclusterv1.MachinePool{
				Spec: expclusterv1.MachinePoolSpec{
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
		nodePool            oke.NodePool
		testSpecificSetup   func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient)
	}{
		{
			name:          "nodepool no change",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						ImageId: common.String("test-image-id"),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames: []string{"pod-subnet"},
								NSGNames:    []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
			},
			nodePool: oke.NodePool{
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
				DefinedTags:  definedTagsInterface,
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
					DefinedTags:                    definedTagsInterface,
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
		},
		{
			name:          "update due to change in replica size",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				newReplicas := int32(4)
				ms.MachinePool.Spec.Replicas = &newReplicas
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					ID:           common.String("node-pool-id"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						ImageId: common.String("test-image-id"),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(31),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeMetadata:      map[string]string{"key1": "value1"},
						InitialNodeLabels: []oke.KeyValue{{
							Key:   common.String("key"),
							Value: common.String("value"),
						}},
						NodeShape: common.String("test-shape"),
						NodeShapeConfig: &oke.UpdateNodeShapeConfigDetails{
							Ocpus:       common.Float32(2),
							MemoryInGBs: common.Float32(16),
						},
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId: common.String("test-image-id"),
						},
						SshPublicKey: common.String("test-ssh-public-key"),
						NodeConfigDetails: &oke.UpdateNodePoolNodeConfigDetails{
							Size:                           common.Int(4),
							NsgIds:                         []string{"nsg-id"},
							KmsKeyId:                       common.String("kms-key-id"),
							IsPvEncryptionInTransitEnabled: common.Bool(true),
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
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
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
				DefinedTags:  definedTagsInterface,
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
					DefinedTags:                    definedTagsInterface,
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
		},
		{
			name:          "no update due to change in replica size as annotation is set",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.MachinePool.Annotations = make(map[string]string)
				ms.MachinePool.Annotations[clusterv1.ReplicasManagedByAnnotation] = ""
				newReplicas := int32(4)
				ms.MachinePool.Spec.Replicas = &newReplicas
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					ID:           common.String("node-pool-id"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						ImageId: common.String("test-image-id"),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(31),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
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
				DefinedTags:  definedTagsInterface,
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
					DefinedTags:                    definedTagsInterface,
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
		},
		{
			name:          "update due to change in k8s version",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					ID:           common.String("node-pool-id"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						ImageId: common.String("test-image-id"),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(31),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeMetadata:      map[string]string{"key1": "value1"},
						InitialNodeLabels: []oke.KeyValue{{
							Key:   common.String("key"),
							Value: common.String("value"),
						}},
						NodeShape: common.String("test-shape"),
						NodeShapeConfig: &oke.UpdateNodeShapeConfigDetails{
							Ocpus:       common.Float32(2),
							MemoryInGBs: common.Float32(16),
						},
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId: common.String("test-image-id"),
						},
						SshPublicKey: common.String("test-ssh-public-key"),
						NodeConfigDetails: &oke.UpdateNodePoolNodeConfigDetails{
							NsgIds:                         []string{"nsg-id"},
							KmsKeyId:                       common.String("kms-key-id"),
							IsPvEncryptionInTransitEnabled: common.Bool(true),
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
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
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
				DefinedTags:  definedTagsInterface,
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
					DefinedTags:                    definedTagsInterface,
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
		},
		{
			name:          "update due to change in placement config",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					ID:           common.String("node-pool-id"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						ImageId: common.String("test-image-id"),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(31),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeMetadata:      map[string]string{"key1": "value1"},
						InitialNodeLabels: []oke.KeyValue{{
							Key:   common.String("key"),
							Value: common.String("value"),
						}},
						NodeShape: common.String("test-shape"),
						NodeShapeConfig: &oke.UpdateNodeShapeConfigDetails{
							Ocpus:       common.Float32(2),
							MemoryInGBs: common.Float32(16),
						},
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId: common.String("test-image-id"),
						},
						SshPublicKey: common.String("test-ssh-public-key"),
						NodeConfigDetails: &oke.UpdateNodePoolNodeConfigDetails{
							PlacementConfigs: []oke.NodePoolPlacementConfigDetails{
								{
									AvailabilityDomain:    common.String("test-ad"),
									CapacityReservationId: common.String("cap-id"),
									SubnetId:              common.String("subnet-id"),
									FaultDomains:          []string{"fd-1", "fd-2"},
								},
							},
							NsgIds:                         []string{"nsg-id"},
							KmsKeyId:                       common.String("kms-key-id"),
							IsPvEncryptionInTransitEnabled: common.Bool(true),
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
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
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
				DefinedTags:  definedTagsInterface,
				SshPublicKey: common.String("test-ssh-public-key"),
				NodeConfigDetails: &oke.NodePoolNodeConfigDetails{
					Size: common.Int(3),
					PlacementConfigs: []oke.NodePoolPlacementConfigDetails{
						{
							AvailabilityDomain:    common.String("test-ad"),
							SubnetId:              common.String("subnet-id-to-be-changed"),
							CapacityReservationId: common.String("cap-id"),
							FaultDomains:          []string{"fd-1", "fd-2"},
						},
					},
					NsgIds:                         []string{"nsg-id"},
					KmsKeyId:                       common.String("kms-key-id"),
					IsPvEncryptionInTransitEnabled: common.Bool(true),
					FreeformTags:                   tags,
					DefinedTags:                    definedTagsInterface,
					NodePoolPodNetworkOptionDetails: oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
						PodSubnetIds:   []string{"pod-subnet-id"},
						MaxPodsPerNode: common.Int(15),
						PodNsgIds:      []string{"pod-nsg-id"},
					},
				},
				NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
					EvictionGraceDuration:           common.String("PT30M"),
					IsForceDeleteAfterGraceDuration: common.Bool(true),
				},
			},
		},
		{
			name:          "update due to change in name",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Name = "changed"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					ID:           common.String("node-pool-id"),
					NodeMetadata: map[string]string{"key1": "value1"},
					InitialNodeLabels: []infrav2exp.KeyValue{{
						Key:   common.String("key"),
						Value: common.String("value"),
					}},
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("2"),
						MemoryInGBs: common.String("16"),
					},
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						ImageId: common.String("test-image-id"),
					},
					SshPublicKey: "test-ssh-public-key",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						PlacementConfigs: []infrav2exp.PlacementConfig{
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
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},
								MaxPodsPerNode: common.Int(31),
								NSGNames:       []string{"pod-nsg"},
							},
						},
					},
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration:           common.String("PT30M"),
						IsForceDeleteAfterGraceDuration: common.Bool(true),
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("changed"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeMetadata:      map[string]string{"key1": "value1"},
						InitialNodeLabels: []oke.KeyValue{{
							Key:   common.String("key"),
							Value: common.String("value"),
						}},
						NodeShape: common.String("test-shape"),
						NodeShapeConfig: &oke.UpdateNodeShapeConfigDetails{
							Ocpus:       common.Float32(2),
							MemoryInGBs: common.Float32(16),
						},
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId: common.String("test-image-id"),
						},
						SshPublicKey: common.String("test-ssh-public-key"),
						NodeConfigDetails: &oke.UpdateNodePoolNodeConfigDetails{
							NsgIds:                         []string{"nsg-id"},
							KmsKeyId:                       common.String("kms-key-id"),
							IsPvEncryptionInTransitEnabled: common.Bool(true),
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
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
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
				DefinedTags:  definedTagsInterface,
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
					DefinedTags:                    definedTagsInterface,
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
		},
		{
			name:          "no update when unspecified fields differ from OCI",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				// Spec only specifies a few fields - unspecified fields should not trigger updates
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus: common.String("2"), // Only specify Ocpus, not MemoryInGBs
					},
				}
				// OCI has different values for unspecified fields - should not trigger update
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodeShapeConfig: &oke.NodeShapeConfig{
					Ocpus:       common.Float32(2),
					MemoryInGBs: common.Float32(32), // Different from spec
				},
				NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
					EvictionGraceDuration:           common.String("PT60M"), // Not in spec
					IsForceDeleteAfterGraceDuration: common.Bool(false),     // Not in spec
				},
				NodePoolCyclingDetails: &oke.NodePoolCyclingDetails{
					IsNodeCyclingEnabled: common.Bool(false),   // Not in spec
					MaximumSurge:         common.String("25%"), // Not in spec
					MaximumUnavailable:   common.String("25%"), // Not in spec
				},
			},
		},
		{
			name:          "update when user-specified NodeEvictionNodePoolSettings differ",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration: common.String("PT30M"), // User specified
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
							EvictionGraceDuration: common.String("PT30M"),
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
					EvictionGraceDuration:           common.String("PT60M"), // Different from spec
					IsForceDeleteAfterGraceDuration: common.Bool(false),     // Different but not in spec
				},
			},
		},
		{
			name:          "update when user-specified NodePoolCyclingDetails differ",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodePoolCyclingDetails: &infrav2exp.NodePoolCyclingDetails{
						IsNodeCyclingEnabled: common.Bool(true),    // User specified
						MaximumSurge:         common.String("20%"), // User specified
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						NodePoolCyclingDetails: &oke.NodePoolCyclingDetails{
							IsNodeCyclingEnabled: common.Bool(true),
							MaximumSurge:         common.String("20%"),
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodePoolCyclingDetails: &oke.NodePoolCyclingDetails{
					IsNodeCyclingEnabled: common.Bool(false),   // Different from spec
					MaximumSurge:         common.String("25%"), // Different from spec
					MaximumUnavailable:   common.String("30%"), // Different but not in spec
				},
			},
		},
		{
			name:          "no update when OCI has nil values for unspecified fields",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				// Spec specifies some fields but OCI has nil for others - should not trigger update
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						EvictionGraceDuration: common.String("PT30M"), // User specified
					},
				}
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
					EvictionGraceDuration:           common.String("PT30M"), // Same as spec
					IsForceDeleteAfterGraceDuration: nil,                    // nil in OCI, not specified in spec
				},
				NodePoolCyclingDetails: nil, // nil in OCI, not specified in spec
			},
		},
		{
			name:          "update when OCI nil values differ from user-specified values",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodeEvictionNodePoolSettings: &infrav2exp.NodeEvictionNodePoolSettings{
						IsForceDeleteAfterGraceDuration: common.Bool(true), // User wants to set this
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
							IsForceDeleteAfterGraceDuration: common.Bool(true),
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodeEvictionNodePoolSettings: &oke.NodeEvictionNodePoolSettings{
					EvictionGraceDuration:           nil, // nil in OCI
					IsForceDeleteAfterGraceDuration: nil, // nil in OCI, user wants to set it
				},
			},
		},
		{
			name:          "update with order-independent NSG comparison",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						NsgNames: []string{"worker-nsg", "pod-nsg"}, // Order in spec
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						NodeConfigDetails: &oke.UpdateNodePoolNodeConfigDetails{
							NsgIds: []string{"nsg-id", "pod-nsg-id"}, // Should be updated to spec order
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodeConfigDetails: &oke.NodePoolNodeConfigDetails{
					NsgIds: []string{"pod-nsg-id", "nsg-id"}, // Different order in OCI
				},
			},
		},
		{
			name:          "no update when NSG lists are equivalent regardless of order",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						NsgNames: []string{"worker-nsg", "pod-nsg"}, // Order in spec
					},
				}
				// No UpdateNodePool call expected - lists are equivalent
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodeConfigDetails: &oke.NodePoolNodeConfigDetails{
					NsgIds: []string{"nsg-id", "pod-nsg-id"}, // Same elements, different order
				},
			},
		},
		{
			name:          "update when user specifies different Kubernetes version",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.25.0"), // User specifies v1.25.0
					NodeShape: "test-shape",
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.25.0"), // Should update to v1.25.0
						NodeShape:         common.String("test-shape"),
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"), // OCI has v1.24.5
				NodeShape:         common.String("test-shape"),
			},
		},
		{
			name:          "update when user specifies different node shape",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "VM.Standard.E4.Flex", // User specifies different shape
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("VM.Standard.E4.Flex"), // Should update shape
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("VM.Standard.E3.Flex"), // OCI has different shape
			},
		},
		{
			name:          "update when user specifies different node shape config",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodeShapeConfig: &infrav2exp.NodeShapeConfig{
						Ocpus:       common.String("4"),  // User specifies 4 OCPUs
						MemoryInGBs: common.String("32"), // User specifies 32GB memory
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						NodeShapeConfig: &oke.UpdateNodeShapeConfigDetails{
							Ocpus:       common.Float32(4),  // Should update to 4 OCPUs
							MemoryInGBs: common.Float32(32), // Should update to 32GB
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodeShapeConfig: &oke.NodeShapeConfig{
					Ocpus:       common.Float32(2),  // OCI has 2 OCPUs
					MemoryInGBs: common.Float32(16), // OCI has 16GB memory
				},
			},
		},
		{
			name:          "update when user specifies different SSH key",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:      common.String("v1.24.5"),
					NodeShape:    "test-shape",
					SshPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ new-ssh-key", // User specifies new key
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						SshPublicKey:      common.String("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ new-ssh-key"), // Should update SSH key
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				SshPublicKey:      common.String("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ old-ssh-key"), // OCI has old key
			},
		},
		{
			name:          "update when user specifies different metadata",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodeMetadata: map[string]string{
						"user.key1": "new-value1", // User specifies new value
						"user.key2": "value2",     // User specifies new key
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						NodeMetadata: map[string]string{
							"user.key1": "new-value1", // Should update metadata
							"user.key2": "value2",
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodeMetadata: map[string]string{
					"user.key1": "old-value1", // OCI has old value
					"oci.key":   "oci-value",  // OCI has extra key (should be preserved)
				},
			},
		},
		{
			name:          "update when user specifies different initial node labels",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					InitialNodeLabels: []infrav2exp.KeyValue{
						{Key: common.String("app"), Value: common.String("web")},  // User specifies new label
						{Key: common.String("env"), Value: common.String("prod")}, // User specifies new label
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						InitialNodeLabels: []oke.KeyValue{
							{Key: common.String("app"), Value: common.String("web")}, // Should update labels
							{Key: common.String("env"), Value: common.String("prod")},
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				InitialNodeLabels: []oke.KeyValue{
					{Key: common.String("app"), Value: common.String("api")}, // OCI has different value
				},
			},
		},
		{
			name:          "update when user specifies different pod network config",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
					NodePoolNodeConfig: &infrav2exp.NodePoolNodeConfig{
						NodePoolPodNetworkOptionDetails: &infrav2exp.NodePoolPodNetworkOptionDetails{
							CniType: infrastructurev1beta2.VCNNativeCNI,
							VcnIpNativePodNetworkOptions: infrav2exp.VcnIpNativePodNetworkOptions{
								SubnetNames:    []string{"pod-subnet"},           // User specifies different subnet
								MaxPodsPerNode: common.Int(50),                   // User specifies different max pods
								NSGNames:       []string{"pod-nsg", "extra-nsg"}, // User specifies different NSGs
							},
						},
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						NodeConfigDetails: &oke.UpdateNodePoolNodeConfigDetails{
							NodePoolPodNetworkOptionDetails: oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
								PodSubnetIds:   []string{"pod-subnet-id"},              // Should update to new subnet
								MaxPodsPerNode: common.Int(50),                         // Should update max pods
								PodNsgIds:      []string{"pod-nsg-id", "extra-nsg-id"}, // Should update NSGs
							},
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				NodeConfigDetails: &oke.NodePoolNodeConfigDetails{
					NodePoolPodNetworkOptionDetails: oke.OciVcnIpNativeNodePoolPodNetworkOptionDetails{
						PodSubnetIds:   []string{"old-pod-subnet-id"}, // OCI has old subnet
						MaxPodsPerNode: common.Int(31),                // OCI has old max pods
						PodNsgIds:      []string{"old-nsg-id"},        // OCI has old NSGs
					},
				},
			},
		},
		{
			name:          "update when user specifies different freeform tags",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedCluster.Spec.FreeformTags = map[string]string{
					"user.tag1": "new-value1", // User specifies new tag value
					"user.tag2": "value2",     // User specifies new tag
				}
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "test-shape",
				}
				expectedTags := map[string]string{
					ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
					ociutil.ClusterResourceIdentifier: "resource_uid",
					"user.tag1":                       "new-value1",
					"user.tag2":                       "value2",
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("test-shape"),
						FreeformTags:      expectedTags, // Should update to new tags
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("test-shape"),
				FreeformTags: map[string]string{
					ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
					ociutil.ClusterResourceIdentifier: "resource_uid",
					"user.tag1":                       "old-value1", // OCI has old tag value
				},
			},
		},
		{
			name:          "update when user specifies different node source image",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.24.5"),
					NodeShape: "VM.Standard.E3.Flex",
					NodeSourceViaImage: &infrav2exp.NodeSourceViaImage{
						ImageId:             common.String("new-image-id"),
						BootVolumeSizeInGBs: common.Int64(100),
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						NodeShape:         common.String("VM.Standard.E3.Flex"),
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId:             common.String("new-image-id"),
							BootVolumeSizeInGBs: common.Int64(100),
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"),
				NodeShape:         common.String("VM.Standard.E3.Flex"),
				NodeSourceDetails: oke.NodeSourceViaImageDetails{
					ImageId:             common.String("old-image-id"), // Different image
					BootVolumeSizeInGBs: common.Int64(75),              // Different size
				},
			},
		},
		{
			name:          "test cycling update trigger - version change with cycling enabled",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedMachinePoolScope, okeClient *mock_containerengine.MockClient) {
				ms.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
				ms.MachinePool.Annotations = make(map[string]string)
				ms.OCIManagedMachinePool.Spec = infrav2exp.OCIManagedMachinePoolSpec{
					Version:   common.String("v1.25.0"), // Version change
					NodeShape: "test-shape",
					NodePoolCyclingDetails: &infrav2exp.NodePoolCyclingDetails{
						IsNodeCyclingEnabled: common.Bool(true), // Cycling enabled
						MaximumSurge:         common.String("25%"),
						MaximumUnavailable:   common.String("25%"),
					},
				}
				okeClient.EXPECT().UpdateNodePool(gomock.Any(), gomock.Eq(oke.UpdateNodePoolRequest{
					NodePoolId: common.String("node-pool-id"),
					UpdateNodePoolDetails: oke.UpdateNodePoolDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.25.0"), // Version updated
						NodeShape:         common.String("test-shape"),
						NodeSourceDetails: &oke.NodeSourceViaImageDetails{
							ImageId:             common.String("test-image-id"),
							BootVolumeSizeInGBs: common.Int64(75),
						},
						NodePoolCyclingDetails: &oke.NodePoolCyclingDetails{
							IsNodeCyclingEnabled: common.Bool(true),    // Cycling enabled
							MaximumSurge:         common.String("25%"), // Same values
							MaximumUnavailable:   common.String("25%"), // Same values
						},
					},
				})).
					Return(oke.UpdateNodePoolResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			nodePool: oke.NodePool{
				ClusterId:         common.String("cluster-id"),
				Id:                common.String("node-pool-id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				KubernetesVersion: common.String("v1.24.5"), // Current version
				NodeShape:         common.String("test-shape"),
				NodePoolCyclingDetails: &oke.NodePoolCyclingDetails{
					IsNodeCyclingEnabled: common.Bool(true),    // Cycling enabled
					MaximumSurge:         common.String("25%"), // Same values
					MaximumUnavailable:   common.String("25%"), // Same values
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, okeClient)
			_, err := ms.UpdateNodePool(context.Background(), &tc.nodePool)
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
