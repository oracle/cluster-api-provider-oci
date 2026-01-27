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
	"github.com/oracle/cluster-api-provider-oci/cloud/hash"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/computemanagement/mock_computemanagement"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
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
		ociCluster := &infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "cluster_uid",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId:         "test-compartment",
				DefinedTags:           definedTags,
				OCIResourceIdentifier: "resource_uid",
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
						},
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									Role: infrastructurev1beta2.WorkerRole,
									ID:   common.String("nsg-id"),
									Name: "worker-nsg",
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
		size := int32(3)
		machinePool := &infrav2exp.OCIMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test",
				ResourceVersion: "20",
			},
			Spec: infrav2exp.OCIMachinePoolSpec{},
		}
		client := fake.NewClientBuilder().WithStatusSubresource(machinePool).WithObjects(secret, machinePool).Build()
		ms, err = NewMachinePoolScope(MachinePoolScopeParams{
			ComputeManagementClient: computeManagementClient,
			OCIMachinePool:          machinePool,
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: ociCluster,
			},
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
		testSpecificSetup   func(ms *MachinePoolScope, g *WithT)
	}{
		{
			name:          "instance config exists",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
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
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape: common.String("test-shape"),
					ShapeConfig: &infrav2exp.ShapeConfig{
						Ocpus:                   common.String("2"),
						MemoryInGBs:             common.String("65"),
						BaselineOcpuUtilization: "BASELINE_1_1",
						Nvmes:                   common.Int(5),
					},
					InstanceVnicConfiguration: &infrastructurev1beta2.NetworkDetails{
						AssignPublicIp:         true,
						SubnetName:             "worker-subnet",
						SkipSourceDestCheck:    common.Bool(true),
						NsgNames:               []string{"worker-nsg"},
						HostnameLabel:          common.String("test"),
						DisplayName:            common.String("test-display"),
						AssignPrivateDnsRecord: common.Bool(true),
					},
					PlatformConfig: &infrastructurev1beta2.PlatformConfig{
						PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeAmdvm,
						AmdVmPlatformConfig: infrastructurev1beta2.AmdVmPlatformConfig{
							IsMeasuredBootEnabled:          common.Bool(false),
							IsTrustedPlatformModuleEnabled: common.Bool(true),
							IsSecureBootEnabled:            common.Bool(true),
							IsMemoryEncryptionEnabled:      common.Bool(true),
						},
					},
					AgentConfig: &infrastructurev1beta2.LaunchInstanceAgentConfig{
						IsMonitoringDisabled:  common.Bool(false),
						IsManagementDisabled:  common.Bool(true),
						AreAllPluginsDisabled: common.Bool(true),
						PluginsConfig: []infrastructurev1beta2.InstanceAgentPluginConfig{
							{
								Name:         common.String("test-plugin"),
								DesiredState: infrastructurev1beta2.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
							},
						},
					},
				}
				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil).Times(2)

				expectedLaunch := &core.InstanceConfigurationLaunchInstanceDetails{
					DefinedTags:   definedTagsInterface,
					FreeformTags:  tags,
					DisplayName:   common.String(ms.OCIMachinePool.GetName()),
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
						IsMemoryEncryptionEnabled:      common.Bool(true),
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
				}

				launchHash, err := hash.ComputeHash(expectedLaunch)
				g.Expect(err).To(BeNil())
				suffix := launchHash
				if len(suffix) > 10 {
					suffix = suffix[:10]
				}
				expectedDisplayName := fmt.Sprintf("%s-%s", ms.OCIMachinePool.GetName(), suffix)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Eq(core.CreateInstanceConfigurationRequest{
					CreateInstanceConfiguration: core.CreateInstanceConfigurationDetails{
						DefinedTags:   definedTagsInterface,
						DisplayName:   common.String(expectedDisplayName),
						FreeformTags:  tags,
						CompartmentId: common.String("test-compartment"),
						InstanceDetails: core.ComputeInstanceDetails{
							LaunchDetails: expectedLaunch,
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
			name:          "instance config create - LaunchInstanceAgentConfig contains nil",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape: common.String("test-shape"),
					ShapeConfig: &infrav2exp.ShapeConfig{
						Ocpus:                   common.String("2"),
						MemoryInGBs:             common.String("65"),
						BaselineOcpuUtilization: "BASELINE_1_1",
						Nvmes:                   common.Int(5),
					},
					InstanceVnicConfiguration: &infrastructurev1beta2.NetworkDetails{
						AssignPublicIp:         true,
						SubnetName:             "worker-subnet",
						SkipSourceDestCheck:    common.Bool(true),
						NsgNames:               []string{"worker-nsg"},
						HostnameLabel:          common.String("test"),
						DisplayName:            common.String("test-display"),
						AssignPrivateDnsRecord: common.Bool(true),
					},
					PlatformConfig: &infrastructurev1beta2.PlatformConfig{
						PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeAmdvm,
						AmdVmPlatformConfig: infrastructurev1beta2.AmdVmPlatformConfig{
							IsMeasuredBootEnabled:          common.Bool(false),
							IsTrustedPlatformModuleEnabled: common.Bool(true),
							IsSecureBootEnabled:            common.Bool(true),
							IsMemoryEncryptionEnabled:      common.Bool(true),
						},
					},
					AgentConfig: &infrastructurev1beta2.LaunchInstanceAgentConfig{
						IsMonitoringDisabled:  nil,
						IsManagementDisabled:  nil,
						AreAllPluginsDisabled: nil,
						PluginsConfig: []infrastructurev1beta2.InstanceAgentPluginConfig{
							{
								Name:         nil,
								DesiredState: infrastructurev1beta2.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
							},
						},
					},
				}
				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil).Times(2)

				expectedLaunch := &core.InstanceConfigurationLaunchInstanceDetails{
					DefinedTags:   definedTagsInterface,
					FreeformTags:  tags,
					DisplayName:   common.String(ms.OCIMachinePool.GetName()),
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
						IsMemoryEncryptionEnabled:      common.Bool(true),
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
						IsMonitoringDisabled:  nil,
						IsManagementDisabled:  nil,
						AreAllPluginsDisabled: nil,
						PluginsConfig: []core.InstanceAgentPluginConfigDetails{
							{
								Name:         nil,
								DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
							},
						},
					},
					SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
				}

				launchHash, err := hash.ComputeHash(expectedLaunch)
				g.Expect(err).To(BeNil())
				suffix := launchHash
				if len(suffix) > 10 {
					suffix = suffix[:10]
				}
				expectedDisplayName := fmt.Sprintf("%s-%s", ms.OCIMachinePool.GetName(), suffix)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Eq(core.CreateInstanceConfigurationRequest{
					CreateInstanceConfiguration: core.CreateInstanceConfigurationDetails{
						DefinedTags:   definedTagsInterface,
						DisplayName:   common.String(expectedDisplayName),
						FreeformTags:  tags,
						CompartmentId: common.String("test-compartment"),
						InstanceDetails: core.ComputeInstanceDetails{
							LaunchDetails: expectedLaunch,
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
			name:          "instance config unchanged when bootstrap data differs",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
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
				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)
			},
		},

		{
			name:          "instance config unchanged when nsg order differs",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				networkSpec := ms.OCIClusterAccesor.GetNetworkSpec()
				networkSpec.Vcn.NetworkSecurityGroup.List = []*infrastructurev1beta2.NSG{
					{
						Role: infrastructurev1beta2.WorkerRole,
						ID:   common.String("nsg-id"),
						Name: "worker-nsg",
					},
					{
						Role: infrastructurev1beta2.WorkerRole,
						ID:   common.String("nsg-id-2"),
						Name: "worker-nsg-2",
					},
				}
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
					InstanceVnicConfiguration: &infrastructurev1beta2.NetworkDetails{
						NsgNames: []string{"worker-nsg-2", "worker-nsg"},
					},
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
										FreeformTags:   tags,
										NsgIds:         []string{"nsg-id", "nsg-id-2"},
										AssignPublicIp: common.Bool(false),
										SubnetId:       common.String("subnet-id"),
									},
									SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
									Metadata:      map[string]string{"user_data": "dGVzdA=="},
								},
							},
						},
					}, nil)
				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name:          "instance config update when shape changes",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
					ShapeConfig: &infrav2exp.ShapeConfig{
						Ocpus:                   common.String("2"),
						MemoryInGBs:             common.String("65"),
						BaselineOcpuUtilization: "BASELINE_1_1",
						Nvmes:                   common.Int(5),
					},
					InstanceVnicConfiguration: &infrastructurev1beta2.NetworkDetails{
						AssignPublicIp:         true,
						SubnetName:             "worker-subnet",
						SkipSourceDestCheck:    common.Bool(true),
						NsgNames:               []string{"worker-nsg"},
						HostnameLabel:          common.String("test"),
						DisplayName:            common.String("test-display"),
						AssignPrivateDnsRecord: common.Bool(true),
					},
					PlatformConfig: &infrastructurev1beta2.PlatformConfig{
						PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeAmdvm,
						AmdVmPlatformConfig: infrastructurev1beta2.AmdVmPlatformConfig{
							IsMeasuredBootEnabled:          common.Bool(false),
							IsTrustedPlatformModuleEnabled: common.Bool(true),
							IsSecureBootEnabled:            common.Bool(true),
							IsMemoryEncryptionEnabled:      common.Bool(true),
						},
					},
					AgentConfig: &infrastructurev1beta2.LaunchInstanceAgentConfig{
						IsMonitoringDisabled:  common.Bool(false),
						IsManagementDisabled:  common.Bool(true),
						AreAllPluginsDisabled: common.Bool(true),
						PluginsConfig: []infrastructurev1beta2.InstanceAgentPluginConfig{
							{
								Name:         common.String("test-plugin"),
								DesiredState: infrastructurev1beta2.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
							},
						},
					},
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
									Shape:         common.String("old-test-shape"),
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

				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)

				expectedLaunch := &core.InstanceConfigurationLaunchInstanceDetails{
					DefinedTags:   definedTagsInterface,
					FreeformTags:  tags,
					DisplayName:   common.String(ms.OCIMachinePool.GetName()),
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
						IsMemoryEncryptionEnabled:      common.Bool(true),
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
				}

				launchHash, err := hash.ComputeHash(expectedLaunch)
				g.Expect(err).To(BeNil())
				suffix := launchHash
				if len(suffix) > 10 {
					suffix = suffix[:10]
				}
				expectedDisplayName := fmt.Sprintf("%s-%s", ms.OCIMachinePool.GetName(), suffix)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Eq(core.CreateInstanceConfigurationRequest{
					CreateInstanceConfiguration: core.CreateInstanceConfigurationDetails{
						DefinedTags:   definedTagsInterface,
						DisplayName:   common.String(expectedDisplayName),
						FreeformTags:  tags,
						CompartmentId: common.String("test-compartment"),
						InstanceDetails: core.ComputeInstanceDetails{
							LaunchDetails: expectedLaunch,
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
			tc.testSpecificSetup(ms, g)
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
		ociCluster := &infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "cluster_uid",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId:         "test-compartment",
				DefinedTags:           definedTags,
				OCIResourceIdentifier: "resource_uid",
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
						},
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									Role: infrastructurev1beta2.WorkerRole,
									ID:   common.String("nsg-id"),
									Name: "worker-nsg",
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
		size := int32(3)
		machinePool := &infrav2exp.OCIMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test",
				ResourceVersion: "20",
			},
			Spec: infrav2exp.OCIMachinePoolSpec{},
		}
		client := fake.NewClientBuilder().WithStatusSubresource(machinePool).WithObjects(secret, machinePool).Build()
		ms, err = NewMachinePoolScope(MachinePoolScopeParams{
			ComputeManagementClient: computeManagementClient,
			OCIMachinePool:          machinePool,
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: ociCluster,
			},
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
		ociCluster := &infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "cluster_uid",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId:         "test-compartment",
				DefinedTags:           definedTags,
				OCIResourceIdentifier: "resource_uid",
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
						},
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{
								{
									Role: infrastructurev1beta2.WorkerRole,
									ID:   common.String("nsg-id"),
									Name: "worker-nsg",
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
		size := int32(3)
		machinePool := &infrav2exp.OCIMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test",
				ResourceVersion: "20",
			},
			Spec: infrav2exp.OCIMachinePoolSpec{},
		}
		client := fake.NewClientBuilder().WithObjects(secret, machinePool).Build()
		ms, err = NewMachinePoolScope(MachinePoolScopeParams{
			ComputeManagementClient: computeManagementClient,
			OCIMachinePool:          machinePool,
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: ociCluster,
			},
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
