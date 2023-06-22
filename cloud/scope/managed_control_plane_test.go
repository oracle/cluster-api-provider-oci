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
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/base/mock_base"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine/mock_containerengine"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestControlPlaneReconciliation(t *testing.T) {
	var (
		cs        *ManagedControlPlaneScope
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
		ociClusterAccessor := OCIManagedCluster{
			&infrav2exp.OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					UID: "cluster_uid",
				},
				Spec: infrav2exp.OCIManagedClusterSpec{
					CompartmentId: "test-compartment",
					DefinedTags:   definedTags,
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							ID: common.String("vcn-id"),
							Subnets: []*infrastructurev1beta2.Subnet{
								{
									Role: infrastructurev1beta2.ControlPlaneEndpointRole,
									ID:   common.String("subnet-id"),
									Type: infrastructurev1beta2.Public,
								},
								{
									Role: infrastructurev1beta2.ServiceLoadBalancerRole,
									ID:   common.String("lb-subnet-id"),
								},
							},
							NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
								List: []*infrastructurev1beta2.NSG{
									{
										Role: infrastructurev1beta2.ControlPlaneEndpointRole,
										ID:   common.String("nsg-id"),
									},
								},
							},
						},
					},
				},
			},
		}
		ociClusterAccessor.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
		cs, err = NewManagedControlPlaneScope(ManagedControlPlaneScopeParams{
			ContainerEngineClient: okeClient,
			OCIManagedControlPlane: &infrav2exp.OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrav2exp.OCIManagedControlPlaneSpec{},
			},
			OCIClusterAccessor: ociClusterAccessor,
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"5.6.7.8/9"},
						},
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"1.2.3.4/5"},
						},
					},
				},
			},
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
		testSpecificSetup   func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient)
	}{
		{
			name:          "control plane exists",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Spec.ID = common.String("test")
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:           common.String("test"),
							FreeformTags: tags,
						},
					}, nil)
			},
		},
		{
			name:          "control plane list",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				okeClient.EXPECT().ListClusters(gomock.Any(), gomock.Eq(oke.ListClustersRequest{
					CompartmentId: common.String("test-compartment"),
					Name:          common.String("test"),
				})).
					Return(oke.ListClustersResponse{
						Items: []oke.ClusterSummary{
							{
								Id:           common.String("test"),
								FreeformTags: tags,
							},
						},
					}, nil)
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("test"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:           common.String("test"),
							FreeformTags: tags,
						},
					}, nil)
			},
		},
		{
			name:          "control plane create all params",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Spec = infrav2exp.OCIManagedControlPlaneSpec{
					ClusterPodNetworkOptions: []infrav2exp.ClusterPodNetworkOptions{
						{
							CniType: infrav2exp.FlannelCNI,
						},
						{
							CniType: infrav2exp.VCNNativeCNI,
						},
					},
					ImagePolicyConfig: &infrav2exp.ImagePolicyConfig{
						IsPolicyEnabled: common.Bool(true),
						KeyDetails: []infrav2exp.KeyDetails{{
							KmsKeyId: common.String("kms-key-id"),
						}},
					},
					ClusterOption: infrav2exp.ClusterOptions{
						AdmissionControllerOptions: &infrav2exp.AdmissionControllerOptions{
							IsPodSecurityPolicyEnabled: common.Bool(true),
						},
						AddOnOptions: &infrav2exp.AddOnOptions{
							IsKubernetesDashboardEnabled: common.Bool(true),
							IsTillerEnabled:              common.Bool(false),
						},
					},
					KmsKeyId: common.String("etcd-kms-key-id"),
					Version:  common.String("v1.24.5"),
				}
				okeClient.EXPECT().ListClusters(gomock.Any(), gomock.Eq(oke.ListClustersRequest{
					CompartmentId: common.String("test-compartment"),
					Name:          common.String("test"),
				})).
					Return(oke.ListClustersResponse{}, nil)
				okeClient.EXPECT().CreateCluster(gomock.Any(), gomock.Eq(oke.CreateClusterRequest{
					CreateClusterDetails: oke.CreateClusterDetails{
						Name:              common.String("test"),
						CompartmentId:     common.String("test-compartment"),
						VcnId:             common.String("vcn-id"),
						KubernetesVersion: common.String("v1.24.5"),
						FreeformTags:      tags,
						DefinedTags:       definedTagsInterface,
						EndpointConfig: &oke.CreateClusterEndpointConfigDetails{
							SubnetId:          common.String("subnet-id"),
							NsgIds:            []string{"nsg-id"},
							IsPublicIpEnabled: common.Bool(true),
						},
						ClusterPodNetworkOptions: []oke.ClusterPodNetworkOptionDetails{
							oke.FlannelOverlayClusterPodNetworkOptionDetails{},
							oke.OciVcnIpNativeClusterPodNetworkOptionDetails{},
						},
						Options: &oke.ClusterCreateOptions{
							ServiceLbSubnetIds: []string{"lb-subnet-id"},
							KubernetesNetworkConfig: &oke.KubernetesNetworkConfig{
								PodsCidr:     common.String("1.2.3.4/5"),
								ServicesCidr: common.String("5.6.7.8/9"),
							},
							AddOns: &oke.AddOnOptions{
								IsKubernetesDashboardEnabled: common.Bool(true),
								IsTillerEnabled:              common.Bool(false),
							},
							AdmissionControllerOptions: &oke.AdmissionControllerOptions{
								IsPodSecurityPolicyEnabled: common.Bool(true),
							},
							PersistentVolumeConfig: &oke.PersistentVolumeConfigDetails{
								FreeformTags: tags,
								DefinedTags:  definedTagsInterface,
							},
							ServiceLbConfig: &oke.ServiceLbConfigDetails{
								FreeformTags: tags,
								DefinedTags:  definedTagsInterface,
							},
						},
						ImagePolicyConfig: &oke.CreateImagePolicyConfigDetails{
							IsPolicyEnabled: common.Bool(true),
							KeyDetails: []oke.KeyDetails{{
								KmsKeyId: common.String("kms-key-id"),
							}},
						},
						KmsKeyId: common.String("etcd-kms-key-id"),
					},
					OpcRetryToken: common.String("create-oke-resource_uid"),
				})).
					Return(oke.CreateClusterResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)

				okeClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(oke.GetWorkRequestRequest{
					WorkRequestId: common.String("opc-work-request-id"),
				})).
					Return(oke.GetWorkRequestResponse{
						WorkRequest: oke.WorkRequest{
							Resources: []oke.WorkRequestResource{
								{
									Identifier: common.String("oke-cluster-id"),
									EntityType: common.String("cluster"),
								},
							},
						},
					}, nil)
				okeClient.EXPECT().GetCluster(gomock.Any(), gomock.Eq(oke.GetClusterRequest{
					ClusterId: common.String("oke-cluster-id"),
				})).
					Return(oke.GetClusterResponse{
						Cluster: oke.Cluster{
							Id:           common.String("oke-cluster-id"),
							FreeformTags: tags,
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
			tc.testSpecificSetup(cs, okeClient)
			_, err := cs.GetOrCreateControlPlane(context.Background())
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

func TestControlPlaneUpdation(t *testing.T) {
	var (
		cs        *ManagedControlPlaneScope
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
		ociClusterAccessor := OCIManagedCluster{
			&infrav2exp.OCIManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					UID: "cluster_uid",
				},
				Spec: infrav2exp.OCIManagedClusterSpec{
					CompartmentId: "test-compartment",
					DefinedTags:   definedTags,
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							ID: common.String("vcn-id"),
							Subnets: []*infrastructurev1beta2.Subnet{
								{
									Role: infrastructurev1beta2.ControlPlaneEndpointRole,
									ID:   common.String("subnet-id"),
									Type: infrastructurev1beta2.Public,
								},
								{
									Role: infrastructurev1beta2.ServiceLoadBalancerRole,
									ID:   common.String("lb-subnet-id"),
								},
							},
							NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
								List: []*infrastructurev1beta2.NSG{
									{
										Role: infrastructurev1beta2.ControlPlaneEndpointRole,
										ID:   common.String("nsg-id"),
									},
								},
							},
						},
					},
				},
			},
		}
		ociClusterAccessor.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
		cs, err = NewManagedControlPlaneScope(ManagedControlPlaneScopeParams{
			ContainerEngineClient: okeClient,
			OCIManagedControlPlane: &infrav2exp.OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrav2exp.OCIManagedControlPlaneSpec{},
			},
			OCIClusterAccessor: ociClusterAccessor,
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"5.6.7.8/9"},
						},
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"1.2.3.4/5"},
						},
					},
				},
			},
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
		okeCluster          oke.Cluster
		testSpecificSetup   func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient)
	}{
		{
			name:          "control plane no change",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Spec = infrav2exp.OCIManagedControlPlaneSpec{
					ClusterPodNetworkOptions: []infrav2exp.ClusterPodNetworkOptions{
						{
							CniType: infrav2exp.FlannelCNI,
						},
					},
					ImagePolicyConfig: &infrav2exp.ImagePolicyConfig{
						IsPolicyEnabled: common.Bool(true),
						KeyDetails: []infrav2exp.KeyDetails{{
							KmsKeyId: common.String("kms-key-id"),
						}},
					},
					ClusterOption: infrav2exp.ClusterOptions{
						AdmissionControllerOptions: &infrav2exp.AdmissionControllerOptions{
							IsPodSecurityPolicyEnabled: common.Bool(true),
						},
						AddOnOptions: &infrav2exp.AddOnOptions{
							IsKubernetesDashboardEnabled: common.Bool(true),
							IsTillerEnabled:              common.Bool(false),
						},
					},
					KmsKeyId: common.String("etcd-kms-key-id"),
					Version:  common.String("v1.24.5"),
				}
			},
			okeCluster: oke.Cluster{
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				VcnId:             common.String("vcn-id"),
				KubernetesVersion: common.String("v1.24.5"),
				Type:              oke.ClusterTypeBasicCluster,
				FreeformTags:      tags,
				DefinedTags:       definedTagsInterface,
				EndpointConfig: &oke.ClusterEndpointConfig{
					SubnetId:          common.String("subnet-id"),
					NsgIds:            []string{"nsg-id"},
					IsPublicIpEnabled: common.Bool(true),
				},
				ClusterPodNetworkOptions: []oke.ClusterPodNetworkOptionDetails{
					oke.FlannelOverlayClusterPodNetworkOptionDetails{},
				},
				Options: &oke.ClusterCreateOptions{
					ServiceLbSubnetIds: []string{"lb-subnet-id"},
					KubernetesNetworkConfig: &oke.KubernetesNetworkConfig{
						PodsCidr:     common.String("1.2.3.4/5"),
						ServicesCidr: common.String("5.6.7.8/9"),
					},
					AddOns: &oke.AddOnOptions{
						IsKubernetesDashboardEnabled: common.Bool(true),
						IsTillerEnabled:              common.Bool(false),
					},
					AdmissionControllerOptions: &oke.AdmissionControllerOptions{
						IsPodSecurityPolicyEnabled: common.Bool(true),
					},
					PersistentVolumeConfig: &oke.PersistentVolumeConfigDetails{
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
					},
					ServiceLbConfig: &oke.ServiceLbConfigDetails{
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
					},
				},
				ImagePolicyConfig: &oke.ImagePolicyConfig{
					IsPolicyEnabled: common.Bool(true),
					KeyDetails: []oke.KeyDetails{{
						KmsKeyId: common.String("kms-key-id"),
					}},
				},
				KmsKeyId: common.String("etcd-kms-key-id"),
			},
		},
		{
			name:          "control plane change",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Spec = infrav2exp.OCIManagedControlPlaneSpec{
					ClusterPodNetworkOptions: []infrav2exp.ClusterPodNetworkOptions{
						{
							CniType: infrav2exp.FlannelCNI,
						},
						{
							CniType: infrav2exp.VCNNativeCNI,
						},
					},
					ImagePolicyConfig: &infrav2exp.ImagePolicyConfig{
						IsPolicyEnabled: common.Bool(true),
						KeyDetails: []infrav2exp.KeyDetails{{
							KmsKeyId: common.String("new-kms-key-id"),
						}},
					},
					ClusterOption: infrav2exp.ClusterOptions{
						AdmissionControllerOptions: &infrav2exp.AdmissionControllerOptions{
							IsPodSecurityPolicyEnabled: common.Bool(true),
						},
						AddOnOptions: &infrav2exp.AddOnOptions{
							IsKubernetesDashboardEnabled: common.Bool(true),
							IsTillerEnabled:              common.Bool(false),
						},
					},
					KmsKeyId: common.String("etcd-kms-key-id"),
					Version:  common.String("v1.24.5"),
				}

				okeClient.EXPECT().UpdateCluster(gomock.Any(), gomock.Eq(oke.UpdateClusterRequest{
					ClusterId: common.String("id"),
					UpdateClusterDetails: oke.UpdateClusterDetails{
						Name:              common.String("test"),
						KubernetesVersion: common.String("v1.24.5"),
						Options: &oke.UpdateClusterOptionsDetails{
							AdmissionControllerOptions: &oke.AdmissionControllerOptions{
								IsPodSecurityPolicyEnabled: common.Bool(true),
							},
						},
						ImagePolicyConfig: &oke.UpdateImagePolicyConfigDetails{
							IsPolicyEnabled: common.Bool(true),
							KeyDetails: []oke.KeyDetails{{
								KmsKeyId: common.String("new-kms-key-id"),
							}},
						},
					},
				})).
					Return(oke.UpdateClusterResponse{
						OpcWorkRequestId: common.String("opc-work-request-id"),
					}, nil)
			},
			okeCluster: oke.Cluster{
				Id:                common.String("id"),
				Name:              common.String("test"),
				CompartmentId:     common.String("test-compartment"),
				VcnId:             common.String("vcn-id"),
				KubernetesVersion: common.String("v1.24.5"),
				FreeformTags:      tags,
				DefinedTags:       definedTagsInterface,
				EndpointConfig: &oke.ClusterEndpointConfig{
					SubnetId:          common.String("subnet-id"),
					NsgIds:            []string{"nsg-id"},
					IsPublicIpEnabled: common.Bool(true),
				},
				ClusterPodNetworkOptions: []oke.ClusterPodNetworkOptionDetails{
					oke.FlannelOverlayClusterPodNetworkOptionDetails{},
					oke.OciVcnIpNativeClusterPodNetworkOptionDetails{},
				},
				Options: &oke.ClusterCreateOptions{
					ServiceLbSubnetIds: []string{"lb-subnet-id"},
					KubernetesNetworkConfig: &oke.KubernetesNetworkConfig{
						PodsCidr:     common.String("1.2.3.4/5"),
						ServicesCidr: common.String("5.6.7.8/9"),
					},
					AddOns: &oke.AddOnOptions{
						IsKubernetesDashboardEnabled: common.Bool(true),
						IsTillerEnabled:              common.Bool(false),
					},
					AdmissionControllerOptions: &oke.AdmissionControllerOptions{
						IsPodSecurityPolicyEnabled: common.Bool(true),
					},
					PersistentVolumeConfig: &oke.PersistentVolumeConfigDetails{
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
					},
					ServiceLbConfig: &oke.ServiceLbConfigDetails{
						FreeformTags: tags,
						DefinedTags:  definedTagsInterface,
					},
				},
				ImagePolicyConfig: &oke.ImagePolicyConfig{
					IsPolicyEnabled: common.Bool(true),
					KeyDetails: []oke.KeyDetails{{
						KmsKeyId: common.String("kms-key-id"),
					}},
				},
				KmsKeyId: common.String("etcd-kms-key-id"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, okeClient)
			_, err := cs.UpdateControlPlane(context.Background(), &tc.okeCluster)
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

func TestControlPlaneKubeconfigReconcile(t *testing.T) {
	var (
		cs         *ManagedControlPlaneScope
		mockCtrl   *gomock.Controller
		okeClient  *mock_containerengine.MockClient
		baseClient *mock_base.MockBaseClient
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		baseClient = mock_base.NewMockBaseClient(mockCtrl)
		ociClusterAccessor := OCIManagedCluster{
			&infrav2exp.OCIManagedCluster{},
		}
		ociClusterAccessor.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
		cs, err = NewManagedControlPlaneScope(ManagedControlPlaneScopeParams{
			ContainerEngineClient: okeClient,
			BaseClient:            baseClient,
			OCIManagedControlPlane: &infrav2exp.OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrav2exp.OCIManagedControlPlaneSpec{},
			},
			OCIClusterAccessor: ociClusterAccessor,
			Cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			},
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
		okeCluster          oke.Cluster
		testSpecificSetup   func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient)
	}{
		{
			name:          "kubeconfig reconcile without secret",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				client := fake.NewClientBuilder().WithObjects().Build()
				cs.client = client
				clientConfig := api.Config{
					Clusters: map[string]*api.Cluster{
						"test-cluster": {
							Server:                   "http://localhost:6443",
							CertificateAuthorityData: []byte{},
						},
					},
					Contexts: map[string]*api.Context{
						"test-context": {
							Cluster: "test-cluster",
						},
					},
					CurrentContext: "test-context",
				}
				config, _ := clientcmd.Write(clientConfig)
				r := io.NopCloser(strings.NewReader(string(config)))
				okeClient.EXPECT().CreateKubeconfig(gomock.Any(), gomock.Eq(oke.CreateKubeconfigRequest{
					ClusterId: common.String("id"),
				})).
					Return(oke.CreateKubeconfigResponse{
						Content: r,
					}, nil)
				baseClient.EXPECT().GenerateToken(gomock.Any(), gomock.Eq("id")).
					Return("secret-token", nil)
			},
			okeCluster: oke.Cluster{
				Id:   common.String("id"),
				Name: common.String("test"),
			},
		},

		{
			name:          "kubeconfig reconcile with secret",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				clientConfig := api.Config{
					Clusters: map[string]*api.Cluster{
						"test-cluster": {
							Server:                   "http://localhost:6443",
							CertificateAuthorityData: []byte{},
						},
					},
					Contexts: map[string]*api.Context{
						"test-context": {
							Cluster: "test-cluster",
						},
					},
					AuthInfos: map[string]*api.AuthInfo{
						"test-capi-admin": {
							Token: "old-token",
						},
					},
					CurrentContext: "test-context",
				}
				config, _ := clientcmd.Write(clientConfig)
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-kubeconfig",
						Namespace: "test",
					},
					Data: map[string][]byte{
						secret.KubeconfigDataName: config,
					},
				}
				client := fake.NewClientBuilder().WithObjects(secret).Build()
				cs.client = client

				baseClient.EXPECT().GenerateToken(gomock.Any(), gomock.Eq("id")).
					Return("secret-token", nil)
			},
			okeCluster: oke.Cluster{
				Id:   common.String("id"),
				Name: common.String("test"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, okeClient)
			err := cs.ReconcileKubeconfig(context.Background(), &tc.okeCluster)
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

func TestAddonReconcile(t *testing.T) {
	var (
		cs         *ManagedControlPlaneScope
		mockCtrl   *gomock.Controller
		okeClient  *mock_containerengine.MockClient
		baseClient *mock_base.MockBaseClient
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		okeClient = mock_containerengine.NewMockClient(mockCtrl)
		baseClient = mock_base.NewMockBaseClient(mockCtrl)
		ociClusterAccessor := OCIManagedCluster{
			&infrav2exp.OCIManagedCluster{},
		}
		ociClusterAccessor.OCIManagedCluster.Spec.OCIResourceIdentifier = "resource_uid"
		cs, err = NewManagedControlPlaneScope(ManagedControlPlaneScopeParams{
			ContainerEngineClient: okeClient,
			BaseClient:            baseClient,
			OCIManagedControlPlane: &infrav2exp.OCIManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			OCIClusterAccessor: ociClusterAccessor,
			Cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			},
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
		okeCluster          oke.Cluster
		matchStatus         map[string]infrav2exp.AddonStatus
		testSpecificSetup   func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient)
	}{
		{
			name:          "install addon",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Spec.Addons = []infrav2exp.Addon{
					{
						Name: common.String("dashboard"),
					},
				}
				okeClient.EXPECT().GetAddon(gomock.Any(), gomock.Eq(oke.GetAddonRequest{
					ClusterId: common.String("id"),
					AddonName: common.String("dashboard"),
				})).
					Return(oke.GetAddonResponse{}, ociutil.ErrNotFound)
				okeClient.EXPECT().InstallAddon(gomock.Any(), gomock.Eq(oke.InstallAddonRequest{
					ClusterId: common.String("id"),
					InstallAddonDetails: oke.InstallAddonDetails{
						AddonName: common.String("dashboard"),
					},
				})).
					Return(oke.InstallAddonResponse{}, nil)
			},
			okeCluster: oke.Cluster{
				Id:   common.String("id"),
				Name: common.String("test"),
			},
		},
		{
			name:          "install addon with config and version",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Spec.Addons = []infrav2exp.Addon{
					{
						Name:    common.String("dashboard"),
						Version: common.String("v0.1.0"),
						Configurations: []infrav2exp.AddonConfiguration{
							{
								Key:   common.String("k1"),
								Value: common.String("v1"),
							},
							{
								Key:   common.String("k2"),
								Value: common.String("v2"),
							},
						},
					},
				}
				cs.OCIManagedControlPlane.Status.AddonStatus = nil
				okeClient.EXPECT().GetAddon(gomock.Any(), gomock.Eq(oke.GetAddonRequest{
					ClusterId: common.String("id"),
					AddonName: common.String("dashboard"),
				})).
					Return(oke.GetAddonResponse{}, ociutil.ErrNotFound)
				okeClient.EXPECT().InstallAddon(gomock.Any(), gomock.Eq(oke.InstallAddonRequest{
					ClusterId: common.String("id"),
					InstallAddonDetails: oke.InstallAddonDetails{
						AddonName: common.String("dashboard"),
						Version:   common.String("v0.1.0"),
						Configurations: []oke.AddonConfiguration{
							{
								Key:   common.String("k1"),
								Value: common.String("v1"),
							},
							{
								Key:   common.String("k2"),
								Value: common.String("v2"),
							},
						},
					},
				})).
					Return(oke.InstallAddonResponse{}, nil)
			},
			okeCluster: oke.Cluster{
				Id:   common.String("id"),
				Name: common.String("test"),
			},
		},
		{
			name:          "update addon",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Spec.Addons = []infrav2exp.Addon{
					{
						Name: common.String("dashboard"),
						Configurations: []infrav2exp.AddonConfiguration{
							{
								Key:   common.String("k1"),
								Value: common.String("v1"),
							},
							{
								Key:   common.String("k2"),
								Value: common.String("v2"),
							},
						},
					},
				}
				cs.OCIManagedControlPlane.Status.AddonStatus = nil
				okeClient.EXPECT().GetAddon(gomock.Any(), gomock.Eq(oke.GetAddonRequest{
					ClusterId: common.String("id"),
					AddonName: common.String("dashboard"),
				})).
					Return(oke.GetAddonResponse{
						Addon: oke.Addon{
							Name: common.String("dashboard"),
						},
					}, nil)
				okeClient.EXPECT().UpdateAddon(gomock.Any(), gomock.Eq(oke.UpdateAddonRequest{
					ClusterId: common.String("id"),
					AddonName: common.String("dashboard"),
					UpdateAddonDetails: oke.UpdateAddonDetails{
						Configurations: []oke.AddonConfiguration{
							{
								Key:   common.String("k1"),
								Value: common.String("v1"),
							},
							{
								Key:   common.String("k2"),
								Value: common.String("v2"),
							},
						},
					},
				})).
					Return(oke.UpdateAddonResponse{}, nil)
			},
			okeCluster: oke.Cluster{
				Id:   common.String("id"),
				Name: common.String("test"),
			},
		},
		{
			name:          "delete addon",
			errorExpected: false,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Status.AddonStatus = map[string]infrav2exp.AddonStatus{
					"dashboard": {
						LifecycleState: common.String("ACTIVE"),
					},
				}
				okeClient.EXPECT().GetAddon(gomock.Any(), gomock.Eq(oke.GetAddonRequest{
					ClusterId: common.String("id"),
					AddonName: common.String("dashboard"),
				})).
					Return(oke.GetAddonResponse{
						Addon: oke.Addon{
							Name:           common.String("dashboard"),
							LifecycleState: oke.AddonLifecycleStateActive,
						},
					}, nil)
				okeClient.EXPECT().DisableAddon(gomock.Any(), gomock.Eq(oke.DisableAddonRequest{
					ClusterId:             common.String("id"),
					AddonName:             common.String("dashboard"),
					IsRemoveExistingAddOn: common.Bool(true),
				})).
					Return(oke.DisableAddonResponse{}, nil)
			},
			okeCluster: oke.Cluster{
				Id:   common.String("id"),
				Name: common.String("test"),
			},
		},
		{
			name:          "install addon error",
			errorExpected: true,
			matchError:    errors.New("install error"),
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Spec.Addons = []infrav2exp.Addon{
					{
						Name: common.String("dashboard"),
					},
				}
				okeClient.EXPECT().GetAddon(gomock.Any(), gomock.Eq(oke.GetAddonRequest{
					ClusterId: common.String("id"),
					AddonName: common.String("dashboard"),
				})).
					Return(oke.GetAddonResponse{}, ociutil.ErrNotFound)
				okeClient.EXPECT().InstallAddon(gomock.Any(), gomock.Eq(oke.InstallAddonRequest{
					ClusterId: common.String("id"),
					InstallAddonDetails: oke.InstallAddonDetails{
						AddonName: common.String("dashboard"),
					},
				})).
					Return(oke.InstallAddonResponse{}, errors.New("install error"))
			},
			okeCluster: oke.Cluster{
				Id:   common.String("id"),
				Name: common.String("test"),
			},
		},
		{
			name:          "addon status error",
			errorExpected: true,
			testSpecificSetup: func(cs *ManagedControlPlaneScope, okeClient *mock_containerengine.MockClient) {
				cs.OCIManagedControlPlane.Spec.Addons = []infrav2exp.Addon{
					{
						Name: common.String("dashboard"),
					},
				}
				okeClient.EXPECT().GetAddon(gomock.Any(), gomock.Eq(oke.GetAddonRequest{
					ClusterId: common.String("id"),
					AddonName: common.String("dashboard"),
				})).
					Return(oke.GetAddonResponse{
						Addon: oke.Addon{
							Name:           common.String("dashboard"),
							LifecycleState: oke.AddonLifecycleStateNeedsAttention,
							AddonError: &oke.AddonError{
								Code:    common.String("32"),
								Message: common.String("error"),
								Status:  common.String("status"),
							},
						},
					}, nil)
				okeClient.EXPECT().UpdateAddon(gomock.Any(), gomock.Eq(oke.UpdateAddonRequest{
					ClusterId:          common.String("id"),
					AddonName:          common.String("dashboard"),
					UpdateAddonDetails: oke.UpdateAddonDetails{},
				})).
					Return(oke.UpdateAddonResponse{}, nil)
			},
			okeCluster: oke.Cluster{
				Id:   common.String("id"),
				Name: common.String("test"),
			},
			matchStatus: map[string]infrav2exp.AddonStatus{
				"dashboard": {
					LifecycleState: common.String("NEEDS_ATTENTION"),
					AddonError: &infrav2exp.AddonError{
						Code:    common.String("32"),
						Message: common.String("error"),
						Status:  common.String("status"),
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(cs, okeClient)
			err := cs.ReconcileAddons(context.Background(), &tc.okeCluster)
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
			if tc.matchStatus != nil {
				g.Expect(cs.OCIManagedControlPlane.Status.AddonStatus).To(Equal(tc.matchStatus))
			}
		})
	}
}
