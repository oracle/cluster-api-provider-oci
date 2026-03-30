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
	"encoding/base64"
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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
			MachinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: clusterv1.MachinePoolSpec{
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
		name                 string
		errorExpected        bool
		objects              []client.Object
		expectedEvent        string
		eventNotExpected     string
		matchError           error
		errorSubStringMatch  bool
		existingInstancePool *core.InstancePool
		testSpecificSetup    func(ms *MachinePoolScope, g *WithT)
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
						AssignIpv6Ip:           common.Bool(false),
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
						AssignIpv6Ip:           common.Bool(false),
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
			name:          "instance config recreated when bootstrap data differs",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
				}
				// Simulate a previous reconciliation: the bootstrap hash annotation
				// stores the hash of the OLD user_data ("eGVzdA=="), which differs
				// from the current bootstrap secret ("test" → base64 "dGVzdA==").
				oldBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "eGVzdA=="})
				ms.OCIMachinePool.Annotations = map[string]string{
					BootstrapDataHashAnnotation: oldBootstrapHash,
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

				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
					Return(core.CreateInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance config recreated when kubeadm bootstrap token rotates",
			errorExpected: false,
			existingInstancePool: &core.InstancePool{
				Id:                      common.String("pool-id"),
				InstanceConfigurationId: common.String("test"),
				Size:                    common.Int(3),
			},
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				currentBootstrapData := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: abcdef.0123456789abcdef
    kind: JoinConfiguration
`
				rotatedBootstrapData := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: zyxwvu.fedcba9876543210
    kind: JoinConfiguration
`
				currentUserData := base64.StdEncoding.EncodeToString([]byte(currentBootstrapData))
				rotatedUserData := base64.StdEncoding.EncodeToString([]byte(rotatedBootstrapData))
				// Guard this fixture: only the kubeadm discovery token changes.
				g.Expect(hash.ComputeUserDataHash(map[string]string{"user_data": currentUserData})).
					ToNot(Equal(hash.ComputeUserDataHash(map[string]string{"user_data": rotatedUserData})))
				g.Expect(hash.ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": currentUserData})).
					To(Equal(hash.ComputeUserDataHashIgnoringKubeadmToken(map[string]string{"user_data": rotatedUserData})))

				secret := &corev1.Secret{}
				err := ms.Client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "bootstrap"}, secret)
				g.Expect(err).To(BeNil())
				secret.Data["value"] = []byte(currentBootstrapData)
				err = ms.Client.Update(context.Background(), secret)
				g.Expect(err).To(BeNil())

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
									Metadata: map[string]string{
										"user_data": base64.StdEncoding.EncodeToString([]byte(rotatedBootstrapData)),
									},
								},
							},
						},
					}, nil)
				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)
				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
					Return(core.CreateInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance config recreated when kubeadm bootstrap token rotates and pool scales up",
			errorExpected: false,
			existingInstancePool: &core.InstancePool{
				Id:                      common.String("pool-id"),
				InstanceConfigurationId: common.String("test"),
				Size:                    common.Int(2),
			},
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				currentBootstrapData := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: abcdef.0123456789abcdef
    kind: JoinConfiguration
`
				rotatedBootstrapData := `#cloud-config
write_files:
- path: /run/kubeadm/kubeadm-join-config.yaml
  content: |
    ---
    apiVersion: kubeadm.k8s.io/v1beta4
    discovery:
      bootstrapToken:
        apiServerEndpoint: 10.0.0.1:6443
        token: zyxwvu.fedcba9876543210
    kind: JoinConfiguration
`

				secret := &corev1.Secret{}
				err := ms.Client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "bootstrap"}, secret)
				g.Expect(err).To(BeNil())
				secret.Data["value"] = []byte(currentBootstrapData)
				err = ms.Client.Update(context.Background(), secret)
				g.Expect(err).To(BeNil())

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
									Metadata: map[string]string{
										"user_data": base64.StdEncoding.EncodeToString([]byte(rotatedBootstrapData)),
									},
								},
							},
						},
					}, nil)
				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)
				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
					Return(core.CreateInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
						},
					}, nil)
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
			name:          "instance config unchanged when actual includes flex shape defaults",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("VM.Standard.E4.Flex"),
					InstanceConfigurationId: common.String("test"),
					ShapeConfig: &infrav2exp.ShapeConfig{
						Ocpus: common.String("1"),
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
									Shape:         common.String("VM.Standard.E4.Flex"),
									CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
										FreeformTags: tags,
										NsgIds:       []string{"nsg-id"},
										SubnetId:     common.String("subnet-id"),
									},
									ShapeConfig: &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{
										Ocpus:       common.Float32(1),
										MemoryInGBs: common.Float32(16),
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
			name:          "instance config unchanged when plugin config order differs",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
					AgentConfig: &infrastructurev1beta2.LaunchInstanceAgentConfig{
						PluginsConfig: []infrastructurev1beta2.InstanceAgentPluginConfig{
							{
								Name:         common.String("plugin-b"),
								DesiredState: infrastructurev1beta2.InstanceAgentPluginConfigDetailsDesiredStateDisabled,
							},
							{
								Name:         common.String("plugin-a"),
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
									Shape:         common.String("test-shape"),
									CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
										FreeformTags: tags,
										NsgIds:       []string{"nsg-id"},
										SubnetId:     common.String("subnet-id"),
									},
									AgentConfig: &core.InstanceConfigurationLaunchInstanceAgentConfigDetails{
										PluginsConfig: []core.InstanceAgentPluginConfigDetails{
											{
												Name:         common.String("plugin-a"),
												DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
											},
											{
												Name:         common.String("plugin-b"),
												DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateDisabled,
											},
										},
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
						AssignIpv6Ip:           common.Bool(false),
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
			name:          "instance config recreated when extended metadata changes but bootstrap unchanged",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
					ExtendedMetadata: map[string]apiextensionsv1.JSON{
						"workload": {Raw: []byte(`{"profile":"standard","features":{"hpc":true}}`)},
					},
				}
				currentBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "dGVzdA=="})
				ms.OCIMachinePool.Annotations = map[string]string{
					InstanceConfigurationHashAnnotation: "previous-config-hash",
					BootstrapDataHashAnnotation:         currentBootstrapHash,
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
									ExtendedMetadata: map[string]interface{}{
										"workload": map[string]interface{}{
											"profile": "legacy",
											"features": map[string]interface{}{
												"hpc": false,
											},
										},
									},
								},
							},
						},
					}, nil)

				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
					Return(core.CreateInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance config not recreated when extended metadata matches desired payload",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
					ExtendedMetadata: map[string]apiextensionsv1.JSON{
						"workload": {Raw: []byte(`{"profile":"standard","features":{"hpc":true}}`)},
					},
				}
				currentBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "dGVzdA=="})
				ms.OCIMachinePool.Annotations = map[string]string{
					InstanceConfigurationHashAnnotation: "previous-config-hash",
					BootstrapDataHashAnnotation:         currentBootstrapHash,
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
									ExtendedMetadata: map[string]interface{}{
										"workload": map[string]interface{}{
											"features": map[string]interface{}{
												"hpc": true,
											},
											"profile": "standard",
										},
									},
								},
							},
						},
					}, nil)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name:          "instance config recreated when desired removes top-level extended metadata key",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
					ExtendedMetadata: map[string]apiextensionsv1.JSON{
						"workload": {Raw: []byte(`{"profile":"standard","features":{"hpc":true}}`)},
					},
				}
				currentBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "dGVzdA=="})
				ms.OCIMachinePool.Annotations = map[string]string{
					InstanceConfigurationHashAnnotation: "previous-config-hash",
					BootstrapDataHashAnnotation:         currentBootstrapHash,
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
									ExtendedMetadata: map[string]interface{}{
										"workload": map[string]interface{}{
											"features": map[string]interface{}{
												"hpc": true,
											},
											"profile": "standard",
										},
										"legacy": "remove-me",
									},
								},
							},
						},
					}, nil)

				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
					Return(core.CreateInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance config recreated when config changes but bootstrap unchanged",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("new-shape"),
					InstanceConfigurationId: common.String("test"),
				}
				// Pre-populate both annotations to simulate a previously reconciled cluster.
				// Bootstrap hash matches current secret ("test" → "dGVzdA=="), so bootstrap is unchanged.
				currentBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "dGVzdA=="})
				ms.OCIMachinePool.Annotations = map[string]string{
					InstanceConfigurationHashAnnotation: "previous-config-hash",
					BootstrapDataHashAnnotation:         currentBootstrapHash,
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
									Shape:         common.String("old-shape"),
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

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
					Return(core.CreateInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance config recreated when both config and bootstrap change",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("new-shape"),
					InstanceConfigurationId: common.String("test"),
				}
				// Pre-populate both annotations with OLD values so both signals differ.
				oldBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "b2xkLWRhdGE="})
				ms.OCIMachinePool.Annotations = map[string]string{
					InstanceConfigurationHashAnnotation: "previous-config-hash",
					BootstrapDataHashAnnotation:         oldBootstrapHash,
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
									Shape:         common.String("old-shape"),
									CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
										FreeformTags: tags,
										NsgIds:       []string{"nsg-id"},
										SubnetId:     common.String("subnet-id"),
									},
									SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
									Metadata:      map[string]string{"user_data": "b2xkLWRhdGE="},
								},
							},
						},
					}, nil)

				computeManagementClient.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
					Return(core.ListInstanceConfigurationsResponse{}, nil)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
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
			err := ms.ReconcileInstanceConfiguration(context.Background(), tc.existingInstancePool)
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

// TestBackfillAnnotations_Upgrade verifies the backfill logic that runs on the
// first reconciliation after upgrading from a version without the bootstrap
// hash annotation.  This is the most fragile part of the change-signal logic:
//
//   - Both annotations missing → backfill from actual/desired, NO recreate.
//   - Only bootstrap annotation missing → backfill from desired, NO recreate.
//   - Only config annotation missing → backfill from actual, NO recreate.
//   - Both missing + real config drift → backfill bootstrap, detect config change.
//
// Getting any of these wrong either causes a spurious instance-pool churn on
// upgrade or silently swallows a real change.
func TestBackfillAnnotations_Upgrade(t *testing.T) {
	// Shared helpers ---------------------------------------------------
	tags := map[string]string{
		ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
		ociutil.ClusterResourceIdentifier: "resource_uid",
	}
	definedTags := map[string]map[string]string{
		"ns1": {"tag1": "foo"},
	}
	definedTagsInterface := make(map[string]map[string]interface{})
	for ns, m := range definedTags {
		vals := make(map[string]interface{})
		for k, v := range m {
			vals[k] = v
		}
		definedTagsInterface[ns] = vals
	}

	// matchingActualLaunch returns OCI launch details whose config hash
	// matches what the controller would build from the given shape.  The
	// user_data equals base64("test") = "dGVzdA==" which matches the
	// bootstrap secret created in buildScope.
	matchingActualLaunch := func() *core.InstanceConfigurationLaunchInstanceDetails {
		return &core.InstanceConfigurationLaunchInstanceDetails{
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
		}
	}

	// buildScope creates a fresh MachinePoolScope + mock for each sub-test.
	buildScope := func(t *testing.T) (*MachinePoolScope, *mock_computemanagement.MockClient) {
		t.Helper()
		mockCtrl := gomock.NewController(t)
		t.Cleanup(func() { mockCtrl.Finish() })

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "bootstrap", Namespace: "default"},
			Data:       map[string][]byte{"value": []byte("test")},
		}
		computeMgmt := mock_computemanagement.NewMockClient(mockCtrl)
		ociCluster := &infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{UID: "cluster_uid"},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId:         "test-compartment",
				DefinedTags:           definedTags,
				OCIResourceIdentifier: "resource_uid",
				NetworkSpec: infrastructurev1beta2.NetworkSpec{
					Vcn: infrastructurev1beta2.VCN{
						ID: common.String("vcn-id"),
						Subnets: []*infrastructurev1beta2.Subnet{{
							Role: infrastructurev1beta2.WorkerRole,
							ID:   common.String("subnet-id"),
							Type: infrastructurev1beta2.Private,
							Name: "worker-subnet",
						}},
						NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
							List: []*infrastructurev1beta2.NSG{{
								Role: infrastructurev1beta2.WorkerRole,
								ID:   common.String("nsg-id"),
								Name: "worker-nsg",
							}},
						},
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
		cl := fake.NewClientBuilder().WithStatusSubresource(machinePool).WithObjects(secret, machinePool).Build()
		ms, err := NewMachinePoolScope(MachinePoolScopeParams{
			ComputeManagementClient: computeMgmt,
			OCIMachinePool:          machinePool,
			OCIClusterAccessor:      OCISelfManagedCluster{OCICluster: ociCluster},
			Cluster:                 &clusterv1.Cluster{},
			MachinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: clusterv1.MachinePoolSpec{
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
			Client: cl,
		})
		if err != nil {
			t.Fatalf("NewMachinePoolScope: %v", err)
		}
		return ms, computeMgmt
	}

	// -----------------------------------------------------------------
	t.Run("both annotations missing, config matches — backfill only, no recreate", func(t *testing.T) {
		g := NewWithT(t)
		ms, computeMgmt := buildScope(t)

		ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
			Shape:                   common.String("test-shape"),
			InstanceConfigurationId: common.String("test"),
		}
		// No annotations at all — simulates upgrade.
		ms.OCIMachinePool.Annotations = nil

		computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Any()).
			Return(core.GetInstanceConfigurationResponse{
				InstanceConfiguration: core.InstanceConfiguration{
					Id:              common.String("test"),
					InstanceDetails: core.ComputeInstanceDetails{LaunchDetails: matchingActualLaunch()},
				},
			}, nil)
		// CreateInstanceConfiguration must NOT be called.
		computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

		err := ms.ReconcileInstanceConfiguration(context.Background(), nil)
		g.Expect(err).To(BeNil())

		// Both annotations should now be populated.
		g.Expect(ms.OCIMachinePool.Annotations[InstanceConfigurationHashAnnotation]).ToNot(BeEmpty())
		g.Expect(ms.OCIMachinePool.Annotations[BootstrapDataHashAnnotation]).ToNot(BeEmpty())
	})

	// -----------------------------------------------------------------
	t.Run("only bootstrap annotation missing, config matches — backfill bootstrap, no recreate", func(t *testing.T) {
		g := NewWithT(t)
		ms, computeMgmt := buildScope(t)

		ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
			Shape:                   common.String("test-shape"),
			InstanceConfigurationId: common.String("test"),
		}
		// Config annotation exists from a previous reconciliation (pre-upgrade
		// version that only tracked config hash).
		ms.OCIMachinePool.Annotations = map[string]string{
			InstanceConfigurationHashAnnotation: "some-existing-config-hash",
		}

		computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Any()).
			Return(core.GetInstanceConfigurationResponse{
				InstanceConfiguration: core.InstanceConfiguration{
					Id:              common.String("test"),
					InstanceDetails: core.ComputeInstanceDetails{LaunchDetails: matchingActualLaunch()},
				},
			}, nil)
		computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

		err := ms.ReconcileInstanceConfiguration(context.Background(), nil)
		g.Expect(err).To(BeNil())

		// Bootstrap annotation should now be backfilled.
		g.Expect(ms.OCIMachinePool.Annotations[BootstrapDataHashAnnotation]).ToNot(BeEmpty())
		// Config annotation should be updated to match the actual hash.
		g.Expect(ms.OCIMachinePool.Annotations[InstanceConfigurationHashAnnotation]).ToNot(Equal("some-existing-config-hash"))
	})

	// -----------------------------------------------------------------
	t.Run("only config annotation missing, bootstrap matches — backfill config, no recreate", func(t *testing.T) {
		g := NewWithT(t)
		ms, computeMgmt := buildScope(t)

		ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
			Shape:                   common.String("test-shape"),
			InstanceConfigurationId: common.String("test"),
		}
		// Bootstrap annotation exists, config annotation does not.
		currentBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "dGVzdA=="})
		ms.OCIMachinePool.Annotations = map[string]string{
			BootstrapDataHashAnnotation: currentBootstrapHash,
		}

		computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Any()).
			Return(core.GetInstanceConfigurationResponse{
				InstanceConfiguration: core.InstanceConfiguration{
					Id:              common.String("test"),
					InstanceDetails: core.ComputeInstanceDetails{LaunchDetails: matchingActualLaunch()},
				},
			}, nil)
		computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

		err := ms.ReconcileInstanceConfiguration(context.Background(), nil)
		g.Expect(err).To(BeNil())

		// Config annotation should now be backfilled.
		g.Expect(ms.OCIMachinePool.Annotations[InstanceConfigurationHashAnnotation]).ToNot(BeEmpty())
		// Bootstrap annotation should be unchanged.
		g.Expect(ms.OCIMachinePool.Annotations[BootstrapDataHashAnnotation]).To(Equal(currentBootstrapHash))
	})

	// -----------------------------------------------------------------
	t.Run("both annotations missing + real config drift — backfill bootstrap, detect config change", func(t *testing.T) {
		g := NewWithT(t)
		ms, computeMgmt := buildScope(t)

		// Desired shape is "new-shape", but OCI still has "old-shape".
		ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
			Shape:                   common.String("new-shape"),
			InstanceConfigurationId: common.String("test"),
		}
		ms.OCIMachinePool.Annotations = nil

		driftedActual := matchingActualLaunch()
		driftedActual.Shape = common.String("old-shape")

		computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Any()).
			Return(core.GetInstanceConfigurationResponse{
				InstanceConfiguration: core.InstanceConfiguration{
					Id:              common.String("test"),
					InstanceDetails: core.ComputeInstanceDetails{LaunchDetails: driftedActual},
				},
			}, nil)
		// Config drift should trigger a new IC.
		computeMgmt.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
			Return(core.ListInstanceConfigurationsResponse{}, nil)
		computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
			Return(core.CreateInstanceConfigurationResponse{
				InstanceConfiguration: core.InstanceConfiguration{Id: common.String("new-id")},
			}, nil)

		err := ms.ReconcileInstanceConfiguration(context.Background(), nil)
		g.Expect(err).To(BeNil())

		// Both annotations should be populated after reconciliation.
		g.Expect(ms.OCIMachinePool.Annotations[InstanceConfigurationHashAnnotation]).ToNot(BeEmpty())
		g.Expect(ms.OCIMachinePool.Annotations[BootstrapDataHashAnnotation]).ToNot(BeEmpty())
	})

	// -----------------------------------------------------------------
	t.Run("bootstrap annotation missing + OCI has stale bootstrap — backfill from actual, detect real change", func(t *testing.T) {
		g := NewWithT(t)
		ms, computeMgmt := buildScope(t)

		ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
			Shape:                   common.String("test-shape"),
			InstanceConfigurationId: common.String("test"),
		}
		// Simulate upgrade: bootstrap annotation missing. OCI IC has OLD
		// user_data ("b2xkLWRhdGE=") while the current bootstrap secret has
		// new data ("test" → "dGVzdA=="). The backfill seeds from OCI's
		// actual user_data. Then the change detection sees desired != stored
		// (the bootstrap secret genuinely changed) and creates a new IC.
		//
		// This is correct: the running IC has stale bootstrap data, so new
		// nodes would get the wrong join token. A new IC is needed.
		ms.OCIMachinePool.Annotations = map[string]string{
			InstanceConfigurationHashAnnotation: "will-be-overwritten",
		}

		actualWithOldBootstrap := matchingActualLaunch()
		actualWithOldBootstrap.Metadata = map[string]string{"user_data": "b2xkLWRhdGE="}

		computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Any()).
			Return(core.GetInstanceConfigurationResponse{
				InstanceConfiguration: core.InstanceConfiguration{
					Id:              common.String("test"),
					InstanceDetails: core.ComputeInstanceDetails{LaunchDetails: actualWithOldBootstrap},
				},
			}, nil)
		// Bootstrap data genuinely differs from OCI — new IC should be created.
		computeMgmt.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
			Return(core.ListInstanceConfigurationsResponse{}, nil)
		computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
			Return(core.CreateInstanceConfigurationResponse{
				InstanceConfiguration: core.InstanceConfiguration{Id: common.String("new-id")},
			}, nil)

		err := ms.ReconcileInstanceConfiguration(context.Background(), nil)
		g.Expect(err).To(BeNil())

		// Bootstrap annotation should reflect the NEW desired hash (after IC recreation).
		desiredBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "dGVzdA=="})
		g.Expect(ms.OCIMachinePool.Annotations[BootstrapDataHashAnnotation]).To(Equal(desiredBootstrapHash))
	})

	// -----------------------------------------------------------------
	t.Run("bootstrap annotation missing + OCI matches desired — backfill from actual, no recreate", func(t *testing.T) {
		g := NewWithT(t)
		ms, computeMgmt := buildScope(t)

		ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
			Shape:                   common.String("test-shape"),
			InstanceConfigurationId: common.String("test"),
		}
		// Simulate upgrade where bootstrap secret has NOT changed since
		// the IC was created. OCI's user_data matches the current secret.
		// Backfill from actual → stored == desired → no change.
		ms.OCIMachinePool.Annotations = map[string]string{
			InstanceConfigurationHashAnnotation: "will-be-overwritten",
		}

		computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Any()).
			Return(core.GetInstanceConfigurationResponse{
				InstanceConfiguration: core.InstanceConfiguration{
					Id:              common.String("test"),
					InstanceDetails: core.ComputeInstanceDetails{LaunchDetails: matchingActualLaunch()},
				},
			}, nil)
		// OCI user_data matches current secret — no IC recreation.
		computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

		err := ms.ReconcileInstanceConfiguration(context.Background(), nil)
		g.Expect(err).To(BeNil())

		// Bootstrap annotation backfilled from actual (which matches desired).
		actualBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "dGVzdA=="})
		g.Expect(ms.OCIMachinePool.Annotations[BootstrapDataHashAnnotation]).To(Equal(actualBootstrapHash))
	})
}

func TestGetLaunchInstanceDetailsCopiesMetadataAndPropagatesSupportedFields(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("test"),
		},
	}
	ociCluster := &infrastructurev1beta2.OCICluster{
		Spec: infrastructurev1beta2.OCIClusterSpec{
			CompartmentId: "test-compartment",
			NetworkSpec: infrastructurev1beta2.NetworkSpec{
				Vcn: infrastructurev1beta2.VCN{
					Subnets: []*infrastructurev1beta2.Subnet{
						{
							Role: infrastructurev1beta2.WorkerRole,
							ID:   common.String("worker-subnet-id"),
							Name: "worker-subnet",
						},
					},
					NetworkSecurityGroup: infrastructurev1beta2.NetworkSecurityGroup{
						List: []*infrastructurev1beta2.NSG{
							{
								Role: infrastructurev1beta2.WorkerRole,
								ID:   common.String("worker-nsg-id"),
								Name: "worker-nsg",
							},
						},
					},
				},
			},
		},
	}
	machinePool := &infrav2exp.OCIMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	client := fake.NewClientBuilder().WithStatusSubresource(machinePool).WithObjects(secret, machinePool).Build()
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: mock_computemanagement.NewMockClient(mockCtrl),
		OCIMachinePool:          machinePool,
		OCIClusterAccessor: OCISelfManagedCluster{
			OCICluster: ociCluster,
		},
		Cluster: &clusterv1.Cluster{},
		MachinePool: &clusterv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
			},
			Spec: clusterv1.MachinePoolSpec{
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

	metadata := map[string]string{"ssh_authorized_keys": "ssh-rsa test"}
	spec := infrav2exp.InstanceConfiguration{
		Shape:                          common.String("test-shape"),
		Metadata:                       metadata,
		ExtendedMetadata:               map[string]apiextensionsv1.JSON{"network": {Raw: []byte(`{"cni":{"type":"cilium","enabled":true},"zones":["ad-1","ad-2"],"retries":3}`)}},
		IsPvEncryptionInTransitEnabled: common.Bool(true),
		InstanceSourceViaImageDetails: &infrav2exp.InstanceSourceViaImageConfig{
			ImageId:             common.String("image-id"),
			KmsKeyId:            common.String("kms-id"),
			BootVolumeSizeInGBs: common.Int64(100),
			BootVolumeVpusPerGB: common.Int64(20),
		},
		AvailabilityConfig: &infrastructurev1beta2.LaunchInstanceAvailabilityConfig{
			IsLiveMigrationPreferred: common.Bool(true),
			RecoveryAction:           infrastructurev1beta2.LaunchInstanceAvailabilityConfigDetailsRecoveryActionRestoreInstance,
		},
		InstanceVnicConfiguration: &infrastructurev1beta2.NetworkDetails{
			SubnetId:     common.String("explicit-subnet-id"),
			NSGIds:       []string{"explicit-nsg-id"},
			AssignIpv6Ip: true,
		},
	}
	ms.OCIMachinePool.Spec.InstanceConfiguration = spec

	launchDetails, err := ms.getLaunchInstanceDetails(spec, map[string]string{"freeform": "tag"}, nil)
	g.Expect(err).To(BeNil())
	g.Expect(metadata).To(Equal(map[string]string{"ssh_authorized_keys": "ssh-rsa test"}))
	g.Expect(launchDetails.Metadata).To(HaveKey("user_data"))
	g.Expect(launchDetails.Metadata["ssh_authorized_keys"]).To(Equal("ssh-rsa test"))
	g.Expect(launchDetails.ExtendedMetadata).To(Equal(map[string]interface{}{
		"network": map[string]interface{}{
			"cni": map[string]interface{}{
				"type":    "cilium",
				"enabled": true,
			},
			"zones":   []interface{}{"ad-1", "ad-2"},
			"retries": float64(3),
		},
	}))
	g.Expect(*launchDetails.IsPvEncryptionInTransitEnabled).To(BeTrue())
	g.Expect(*launchDetails.CreateVnicDetails.SubnetId).To(Equal("explicit-subnet-id"))
	g.Expect(launchDetails.CreateVnicDetails.NsgIds).To(Equal([]string{"explicit-nsg-id"}))
	g.Expect(*launchDetails.CreateVnicDetails.AssignIpv6Ip).To(BeTrue())
	g.Expect(*launchDetails.CreateVnicDetails.AssignPublicIp).To(BeFalse())

	sourceDetails, ok := launchDetails.SourceDetails.(core.InstanceConfigurationInstanceSourceViaImageDetails)
	g.Expect(ok).To(BeTrue())
	g.Expect(*sourceDetails.KmsKeyId).To(Equal("kms-id"))
	g.Expect(*sourceDetails.BootVolumeSizeInGBs).To(Equal(int64(100)))
	g.Expect(*sourceDetails.BootVolumeVpusPerGB).To(Equal(int64(20)))
	g.Expect(*launchDetails.AvailabilityConfig.IsLiveMigrationPreferred).To(BeTrue())
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
			MachinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: clusterv1.MachinePoolSpec{
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
			MachinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: clusterv1.MachinePoolSpec{
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
		{
			name:          "no update due to change in replica size as annotation is set",
			errorExpected: false,
			instancepool: &core.InstancePool{
				Size:                    common.Int(3),
				InstanceConfigurationId: common.String("config_id"),
			},
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.MachinePool.Annotations = map[string]string{
					clusterv1.ReplicasManagedByAnnotation: "", // empty value counts as true (= externally managed)
				}
				newReplicas := int32(4)
				ms.MachinePool.Spec.Replicas = &newReplicas
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
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

func TestSyncReplicasFromInstancePool(t *testing.T) {
	var (
		ms       *MachinePoolScope
		mockCtrl *gomock.Controller
	)

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
		ociCluster := &infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "cluster_uid",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				CompartmentId:         "test-compartment",
				OCIResourceIdentifier: "resource_uid",
			},
		}
		replicas := int32(3)
		infraMachinePool := &infrav2exp.OCIMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
		}
		machinePool := &clusterv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: clusterv1.MachinePoolSpec{
				Replicas: &replicas,
				Template: clusterv1.MachineTemplateSpec{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: common.String("bootstrap"),
						},
					},
				},
			},
		}
		scheme := runtime.NewScheme()
		g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
		g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		g.Expect(infrav2exp.AddToScheme(scheme)).To(Succeed())

		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, infraMachinePool, machinePool).Build()
		ms, err = NewMachinePoolScope(MachinePoolScopeParams{
			ComputeManagementClient: mock_computemanagement.NewMockClient(mockCtrl),
			OCIMachinePool:          infraMachinePool,
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: ociCluster,
			},
			Cluster:     &clusterv1.Cluster{},
			MachinePool: machinePool,
			Client:      client,
		})
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name             string
		setup            func(ms *MachinePoolScope)
		instancePool     *core.InstancePool
		expectedReplicas int32
	}{
		{
			name: "does not patch replicas when annotation is not set",
			setup: func(ms *MachinePoolScope) {
				ms.MachinePool.Annotations = nil
			},
			instancePool:     &core.InstancePool{Size: common.Int(4)},
			expectedReplicas: 3,
		},
		{
			name: "patches replicas from observed size when annotation is set",
			setup: func(ms *MachinePoolScope) {
				ms.MachinePool.Annotations = map[string]string{
					clusterv1.ReplicasManagedByAnnotation: "", // empty value counts as true (= externally managed)
				}
			},
			instancePool:     &core.InstancePool{Size: common.Int(4)},
			expectedReplicas: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.setup(ms)

			err := ms.SyncReplicasFromInstancePool(context.Background(), tc.instancePool)
			g.Expect(err).To(BeNil())

			updatedMachinePool := &clusterv1.MachinePool{}
			err = ms.Client.Get(context.Background(), client.ObjectKey{Name: ms.MachinePool.Name, Namespace: ms.MachinePool.Namespace}, updatedMachinePool)
			g.Expect(err).To(BeNil())
			g.Expect(updatedMachinePool.Spec.Replicas).ToNot(BeNil())
			g.Expect(*updatedMachinePool.Spec.Replicas).To(Equal(tc.expectedReplicas))
		})
	}
}
