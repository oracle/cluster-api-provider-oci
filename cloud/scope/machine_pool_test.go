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
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/computemanagement/mock_computemanagement"
	infrav1exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInstanceConfigCreate(t *testing.T) {
	var (
		ms                      *MachinePoolScope
		mockCtrl                *gomock.Controller
		computeManagementClient *mock_computemanagement.MockClient
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
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"value": []byte("test"),
			},
		}
		computeManagementClient = mock_computemanagement.NewMockClient(mockCtrl)
		ociCluster := &infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "cluster_uid",
			},
			Spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId:         "test-compartment",
				DefinedTags:           definedTags,
				OCIResourceIdentifier: "resource_uid",
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID: common.String("vcn-id"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.WorkerRole,
								ID:   common.String("subnet-id"),
								Type: infrastructurev1beta1.Private,
								Name: "worker-subnet",
							},
						},
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
							{
								Role: infrastructurev1beta1.WorkerRole,
								ID:   common.String("nsg-id"),
								Name: "worker-nsg",
							},
						},
					},
				},
			},
			Status: infrastructurev1beta1.OCIClusterStatus{
				AvailabilityDomains: map[string]infrastructurev1beta1.OCIAvailabilityDomain{
					"ad-1": {
						Name:         "ad-1",
						FaultDomains: []string{"fd-5", "fd-6"},
					},
				},
			},
		}
		size := int32(3)
		machinePool := &infrav1exp.OCIMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test",
				ResourceVersion: "20",
			},
			Spec: infrav1exp.OCIMachinePoolSpec{},
		}
		client := fake.NewClientBuilder().WithObjects(secret, machinePool).Build()
		ms, err = NewMachinePoolScope(MachinePoolScopeParams{
			ComputeManagementClient: computeManagementClient,
			OCIMachinePool:          machinePool,
			OCICluster:              ociCluster,
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{},
			},
			MachinePool: &expclusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: expclusterv1.MachinePoolSpec{
					Replicas: &size,
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Bootstrap: clusterv1.Bootstrap{
								DataSecretName: common.String("bootstrap"),
							},
						},
					},
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
		testSpecificSetup   func(ms *MachinePoolScope)
	}{
		{
			name:          "instance config exists",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav1exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
				}
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
										NsgIds:       []string{"nsg-id"},
										SubnetId:     common.String("subnet-id"),
									},
									SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
									Metadata:      map[string]string{"user_data": "dGVzdA=="},
								},
							},
						},
					}, nil)

			},
		},
		{
			name:          "instance config create",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav1exp.InstanceConfiguration{
					Shape: common.String("test-shape"),
					ShapeConfig: &infrav1exp.ShapeConfig{
						Ocpus:                   common.String("2"),
						MemoryInGBs:             common.String("65"),
						BaselineOcpuUtilization: "BASELINE_1_1",
						Nvmes:                   common.Int(5),
					},
					InstanceVnicConfiguration: &infrastructurev1beta1.NetworkDetails{
						AssignPublicIp:         true,
						SubnetName:             "worker-subnet",
						SkipSourceDestCheck:    common.Bool(true),
						NsgNames:               []string{"worker-nsg"},
						HostnameLabel:          common.String("test"),
						DisplayName:            common.String("test-display"),
						AssignPrivateDnsRecord: common.Bool(true),
					},
					PlatformConfig: &infrastructurev1beta1.PlatformConfig{
						PlatformConfigType: infrastructurev1beta1.PlatformConfigTypeAmdvm,
						AmdVmPlatformConfig: infrastructurev1beta1.AmdVmPlatformConfig{
							IsMeasuredBootEnabled:          common.Bool(false),
							IsTrustedPlatformModuleEnabled: common.Bool(true),
							IsSecureBootEnabled:            common.Bool(true),
						},
					},
					AgentConfig: &infrastructurev1beta1.LaunchInstanceAgentConfig{
						IsMonitoringDisabled:  common.Bool(false),
						IsManagementDisabled:  common.Bool(true),
						AreAllPluginsDisabled: common.Bool(true),
						PluginsConfig: []infrastructurev1beta1.InstanceAgentPluginConfig{
							{
								Name:         common.String("test-plugin"),
								DesiredState: infrastructurev1beta1.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
							},
						},
					},
				}
				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Eq(core.CreateInstanceConfigurationRequest{
					CreateInstanceConfiguration: core.CreateInstanceConfigurationDetails{
						DefinedTags:   definedTagsInterface,
						DisplayName:   common.String("test-20"),
						FreeformTags:  tags,
						CompartmentId: common.String("test-compartment"),
						InstanceDetails: core.ComputeInstanceDetails{
							LaunchDetails: &core.InstanceConfigurationLaunchInstanceDetails{
								DefinedTags:   definedTagsInterface,
								FreeformTags:  tags,
								DisplayName:   common.String("test"),
								CompartmentId: common.String("test-compartment"),
								CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
									DefinedTags:            definedTagsInterface,
									FreeformTags:           tags,
									NsgIds:                 []string{"nsg-id"},
									AssignPublicIp:         common.Bool(true),
									SkipSourceDestCheck:    common.Bool(true),
									SubnetId:               common.String("subnet-id"),
									HostnameLabel:          common.String("test"),
									DisplayName:            common.String("test-display"),
									AssignPrivateDnsRecord: common.Bool(true),
								},
								PlatformConfig: core.AmdVmPlatformConfig{
									IsMeasuredBootEnabled:          common.Bool(false),
									IsTrustedPlatformModuleEnabled: common.Bool(true),
									IsSecureBootEnabled:            common.Bool(true),
								},
								Metadata: map[string]string{"user_data": "dGVzdA=="},
								Shape:    common.String("test-shape"),
								ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
									Ocpus:                   common.Float32(2),
									MemoryInGBs:             common.Float32(65),
									BaselineOcpuUtilization: "BASELINE_1_1",
									Nvmes:                   common.Int(5),
								},
								AgentConfig: &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{
									IsMonitoringDisabled:  common.Bool(false),
									IsManagementDisabled:  common.Bool(true),
									AreAllPluginsDisabled: common.Bool(true),
									PluginsConfig: []core.InstanceAgentPluginConfigDetails{
										{
											Name:         common.String("test-plugin"),
											DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
										},
									},
								},
								SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
							},
						},
					},
				})).
					Return(core.CreateInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
						},
					}, nil)

			},
		},
		{
			name:          "instance config update",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav1exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
				}
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
										NsgIds:       []string{"nsg-id"},
										SubnetId:     common.String("subnet-id"),
									},
									SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
									Metadata:      map[string]string{"user_data": "eGVzdA=="},
								},
							},
						},
					}, nil)
				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Eq(core.CreateInstanceConfigurationRequest{
					CreateInstanceConfiguration: core.CreateInstanceConfigurationDetails{
						DefinedTags:   definedTagsInterface,
						DisplayName:   common.String("test-20"),
						FreeformTags:  tags,
						CompartmentId: common.String("test-compartment"),
						InstanceDetails: core.ComputeInstanceDetails{
							LaunchDetails: &core.InstanceConfigurationLaunchInstanceDetails{
								DefinedTags:   definedTagsInterface,
								FreeformTags:  tags,
								DisplayName:   common.String("test"),
								CompartmentId: common.String("test-compartment"),
								CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
									DefinedTags:  definedTagsInterface,
									FreeformTags: tags,
									NsgIds:       []string{"nsg-id"},
									SubnetId:     common.String("subnet-id"),
								},
								Metadata: map[string]string{"user_data": "dGVzdA=="},
								Shape:    common.String("test-shape"),

								SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
							},
						},
					},
				})).
					Return(core.CreateInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
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
			tc.testSpecificSetup(ms)
			err := ms.ReconcileInstanceConfiguration(context.Background())
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

func TestInstancePoolCreate(t *testing.T) {
	var (
		ms                      *MachinePoolScope
		mockCtrl                *gomock.Controller
		computeManagementClient *mock_computemanagement.MockClient
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
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"value": []byte("test"),
			},
		}
		computeManagementClient = mock_computemanagement.NewMockClient(mockCtrl)
		ociCluster := &infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "cluster_uid",
			},
			Spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId:         "test-compartment",
				DefinedTags:           definedTags,
				OCIResourceIdentifier: "resource_uid",
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID: common.String("vcn-id"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.WorkerRole,
								ID:   common.String("subnet-id"),
								Type: infrastructurev1beta1.Private,
								Name: "worker-subnet",
							},
						},
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
							{
								Role: infrastructurev1beta1.WorkerRole,
								ID:   common.String("nsg-id"),
								Name: "worker-nsg",
							},
						},
					},
				},
			},
			Status: infrastructurev1beta1.OCIClusterStatus{
				AvailabilityDomains: map[string]infrastructurev1beta1.OCIAvailabilityDomain{
					"ad-1": {
						Name:         "ad-1",
						FaultDomains: []string{"fd-5", "fd-6"},
					},
				},
			},
		}
		size := int32(3)
		machinePool := &infrav1exp.OCIMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test",
				ResourceVersion: "20",
			},
			Spec: infrav1exp.OCIMachinePoolSpec{},
		}
		client := fake.NewClientBuilder().WithObjects(secret, machinePool).Build()
		ms, err = NewMachinePoolScope(MachinePoolScopeParams{
			ComputeManagementClient: computeManagementClient,
			OCIMachinePool:          machinePool,
			OCICluster:              ociCluster,
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{},
			},
			MachinePool: &expclusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: expclusterv1.MachinePoolSpec{
					Replicas: &size,
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Bootstrap: clusterv1.Bootstrap{
								DataSecretName: common.String("bootstrap"),
							},
						},
					},
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
		testSpecificSetup   func(ms *MachinePoolScope)
	}{
		{
			name:          "instance pool - config id is nil",
			errorExpected: true,
			testSpecificSetup: func(ms *MachinePoolScope) {
			},
		},
		{
			name:          "instance pool",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
				computeManagementClient.EXPECT().CreateInstancePool(gomock.Any(), gomock.Eq(core.CreateInstancePoolRequest{
					CreateInstancePoolDetails: core.CreateInstancePoolDetails{
						CompartmentId:           common.String("test-compartment"),
						InstanceConfigurationId: common.String("config_id"),
						Size:                    common.Int(3),
						DisplayName:             common.String("test"),
						PlacementConfigurations: []core.CreateInstancePoolPlacementConfigurationDetails{{
							AvailabilityDomain: common.String("ad-1"),
							PrimarySubnetId:    common.String("subnet-id"),
							FaultDomains:       []string{"fd-5", "fd-6"},
						}},
						FreeformTags: tags,
					},
				})).
					Return(core.CreateInstancePoolResponse{
						InstancePool: core.InstancePool{
							Id: common.String("id"),
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
			tc.testSpecificSetup(ms)
			_, err := ms.CreateInstancePool(context.Background())
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestInstancePoolUpdate(t *testing.T) {
	var (
		ms                      *MachinePoolScope
		mockCtrl                *gomock.Controller
		computeManagementClient *mock_computemanagement.MockClient
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
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"value": []byte("test"),
			},
		}
		computeManagementClient = mock_computemanagement.NewMockClient(mockCtrl)
		ociCluster := &infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "cluster_uid",
			},
			Spec: infrastructurev1beta1.OCIClusterSpec{
				CompartmentId:         "test-compartment",
				DefinedTags:           definedTags,
				OCIResourceIdentifier: "resource_uid",
				NetworkSpec: infrastructurev1beta1.NetworkSpec{
					Vcn: infrastructurev1beta1.VCN{
						ID: common.String("vcn-id"),
						Subnets: []*infrastructurev1beta1.Subnet{
							{
								Role: infrastructurev1beta1.WorkerRole,
								ID:   common.String("subnet-id"),
								Type: infrastructurev1beta1.Private,
								Name: "worker-subnet",
							},
						},
						NetworkSecurityGroups: []*infrastructurev1beta1.NSG{
							{
								Role: infrastructurev1beta1.WorkerRole,
								ID:   common.String("nsg-id"),
								Name: "worker-nsg",
							},
						},
					},
				},
			},
			Status: infrastructurev1beta1.OCIClusterStatus{
				AvailabilityDomains: map[string]infrastructurev1beta1.OCIAvailabilityDomain{
					"ad-1": {
						Name:         "ad-1",
						FaultDomains: []string{"fd-5", "fd-6"},
					},
				},
			},
		}
		size := int32(3)
		machinePool := &infrav1exp.OCIMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test",
				ResourceVersion: "20",
			},
			Spec: infrav1exp.OCIMachinePoolSpec{},
		}
		client := fake.NewClientBuilder().WithObjects(secret, machinePool).Build()
		ms, err = NewMachinePoolScope(MachinePoolScopeParams{
			ComputeManagementClient: computeManagementClient,
			OCIMachinePool:          machinePool,
			OCICluster:              ociCluster,
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{},
			},
			MachinePool: &expclusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: expclusterv1.MachinePoolSpec{
					Replicas: &size,
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Bootstrap: clusterv1.Bootstrap{
								DataSecretName: common.String("bootstrap"),
							},
						},
					},
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
		instancepool        *core.InstancePool
		testSpecificSetup   func(ms *MachinePoolScope)
	}{
		{
			name:          "instance pool no update",
			errorExpected: false,
			instancepool: &core.InstancePool{
				Size:                    common.Int(3),
				InstanceConfigurationId: common.String("config_id"),
			},
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
			},
		},
		{
			name:          "instance pool update",
			errorExpected: false,
			instancepool: &core.InstancePool{
				Size:                    common.Int(3),
				InstanceConfigurationId: common.String("config_id"),
			},
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id_new")
				computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(core.UpdateInstancePoolRequest{
					UpdateInstancePoolDetails: core.UpdateInstancePoolDetails{
						Size:                    common.Int(3),
						InstanceConfigurationId: common.String("config_id_new"),
					},
				})).
					Return(core.UpdateInstancePoolResponse{
						InstancePool: core.InstancePool{
							Id: common.String("id"),
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
			tc.testSpecificSetup(ms)
			_, err := ms.UpdatePool(context.Background(), tc.instancepool)
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}
