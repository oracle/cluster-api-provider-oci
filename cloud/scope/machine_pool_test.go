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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

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
										DefinedTags:  definedTagsInterface,
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
										DefinedTags:    definedTagsInterface,
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
										DefinedTags:  definedTagsInterface,
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
										DefinedTags:  definedTagsInterface,
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
			name:          "instance config recreated when machine pool freeform tags change",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
					FreeformTags: map[string]string{
						"workload": "batch",
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
										DefinedTags:  definedTagsInterface,
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

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req core.CreateInstanceConfigurationRequest) (core.CreateInstanceConfigurationResponse, error) {
						expectedTags := map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
							"workload":                        "batch",
						}
						createDetails := req.CreateInstanceConfiguration.(core.CreateInstanceConfigurationDetails)
						launchDetails := createDetails.InstanceDetails.(core.ComputeInstanceDetails).LaunchDetails
						g.Expect(createDetails.FreeformTags).To(Equal(expectedTags))
						g.Expect(launchDetails.FreeformTags).To(Equal(expectedTags))
						g.Expect(launchDetails.CreateVnicDetails.FreeformTags).To(Equal(expectedTags))
						return core.CreateInstanceConfigurationResponse{
							InstanceConfiguration: core.InstanceConfiguration{
								Id: common.String("id"),
							},
						}, nil
					})
			},
		},
		{
			name:          "instance config recreated when machine pool defined tags change",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("test-shape"),
					InstanceConfigurationId: common.String("test"),
					DefinedTags: map[string]map[string]string{
						"ns1": {
							"tag1": "pool-override",
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
										DefinedTags:  definedTagsInterface,
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

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req core.CreateInstanceConfigurationRequest) (core.CreateInstanceConfigurationResponse, error) {
						expectedDefinedTags := map[string]map[string]interface{}{
							"ns1": {
								"tag1": "pool-override",
								"tag2": "bar",
							},
							"ns2": {
								"tag1": "foo1",
								"tag2": "bar1",
							},
						}
						createDetails := req.CreateInstanceConfiguration.(core.CreateInstanceConfigurationDetails)
						launchDetails := createDetails.InstanceDetails.(core.ComputeInstanceDetails).LaunchDetails
						g.Expect(createDetails.DefinedTags).To(Equal(expectedDefinedTags))
						g.Expect(launchDetails.DefinedTags).To(Equal(expectedDefinedTags))
						g.Expect(launchDetails.CreateVnicDetails.DefinedTags).To(Equal(expectedDefinedTags))
						return core.CreateInstanceConfigurationResponse{
							InstanceConfiguration: core.InstanceConfiguration{
								Id: common.String("id"),
							},
						}, nil
					})
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
			name:          "instance config recreated once when optional fields including vcpus are removed",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope, g *WithT) {
				ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
					Shape:                   common.String("new-shape"),
					InstanceConfigurationId: common.String("test"),
					PlatformConfig: &infrastructurev1beta2.PlatformConfig{
						PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeAmdvm,
					},
				}
				oldLaunch, err := ms.getLaunchInstanceDetails(ms.OCIMachinePool.Spec.InstanceConfiguration, tags, definedTagsInterface)
				g.Expect(err).To(BeNil())
				oldLaunch.LaunchMode = core.InstanceConfigurationLaunchInstanceDetailsLaunchModeNative
				oldLaunch.PreferredMaintenanceAction = core.InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionReboot
				oldLaunch.LicensingConfigs = []core.LaunchInstanceLicensingConfig{
					core.LaunchInstanceWindowsLicensingConfig{LicenseType: core.LaunchInstanceLicensingConfigLicenseTypeBringYourOwnLicense},
				}
				oldLaunch.PlatformConfig = core.AmdVmPlatformConfig{IsSymmetricMultiThreadingEnabled: common.Bool(false)}
				oldLaunch.ShapeConfig = &core.InstanceConfigurationLaunchInstanceShapeConfigDetails{Vcpus: common.Int(4)}
				oldConfigHash, err := hash.ComputeHash(oldLaunch)
				g.Expect(err).To(BeNil())
				currentBootstrapHash := hash.ComputeUserDataHash(map[string]string{"user_data": "dGVzdA=="})
				ms.OCIMachinePool.Annotations = map[string]string{
					InstanceConfigurationHashAnnotation: oldConfigHash,
					BootstrapDataHashAnnotation:         currentBootstrapHash,
				}

				computeManagementClient.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
					InstanceConfigurationId: common.String("test"),
				})).
					Return(core.GetInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("test"),
							InstanceDetails: core.ComputeInstanceDetails{
								LaunchDetails: oldLaunch,
							},
						},
					}, nil)
				computeManagementClient.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
					InstanceConfigurationId: common.String("id"),
				})).
					Return(core.GetInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
							InstanceDetails: core.ComputeInstanceDetails{
								LaunchDetails: oldLaunch,
							},
						},
					}, nil)

				computeManagementClient.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
					Return(core.CreateInstanceConfigurationResponse{
						InstanceConfiguration: core.InstanceConfiguration{
							Id: common.String("id"),
						},
					}, nil)

				// The first reconciliation detects the desired-hash change and creates one
				// replacement. The table runner reconciles again against OCI readback that
				// still includes VCPUs; that second pass must converge without another create.
				err = ms.ReconcileInstanceConfiguration(context.Background(), nil)
				g.Expect(err).To(BeNil())
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
				DefinedTags:  definedTagsInterface,
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

func TestReconcileInstanceConfigurationDefersCleanupBeforePoolSwitch(t *testing.T) {
	g := NewWithT(t)
	ms, computeMgmt := newInstanceConfigurationOrderingScope(t, "test")

	ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
		Shape:                   common.String("new-shape"),
		InstanceConfigurationId: common.String("old-id"),
	}
	activePool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		InstanceConfigurationId: common.String("old-id"),
		Size:                    common.Int(3),
		LifecycleState:          core.InstancePoolLifecycleStateRunning,
	}

	computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
		InstanceConfigurationId: common.String("old-id"),
	})).
		Return(core.GetInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{
				Id: common.String("old-id"),
				InstanceDetails: core.ComputeInstanceDetails{
					LaunchDetails: orderingLaunchDetails("old-shape", "test"),
				},
			},
		}, nil)
	computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
		Return(core.CreateInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{Id: common.String("new-id")},
		}, nil)
	computeMgmt.EXPECT().DeleteInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

	err := ms.ReconcileInstanceConfiguration(context.Background(), activePool)
	g.Expect(err).To(BeNil())
	g.Expect(ms.GetInstanceConfigurationId()).To(Equal(common.String("new-id")))
}

func TestFailedInstancePoolUpdatePreservesActiveInstanceConfiguration(t *testing.T) {
	g := NewWithT(t)
	ms, computeMgmt := newInstanceConfigurationOrderingScope(t, "test")

	ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
		Shape:                   common.String("new-shape"),
		InstanceConfigurationId: common.String("old-id"),
	}
	activePool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		InstanceConfigurationId: common.String("old-id"),
		Size:                    common.Int(3),
		LifecycleState:          core.InstancePoolLifecycleStateRunning,
	}

	computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Any()).
		Return(core.GetInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{
				Id: common.String("old-id"),
				InstanceDetails: core.ComputeInstanceDetails{
					LaunchDetails: orderingLaunchDetails("old-shape", "test"),
				},
			},
		}, nil)
	computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
		Return(core.CreateInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{Id: common.String("new-id")},
		}, nil)
	computeMgmt.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Any()).
		Return(core.UpdateInstancePoolResponse{}, fmt.Errorf("update failed"))
	computeMgmt.EXPECT().DeleteInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

	err := ms.ReconcileInstanceConfiguration(context.Background(), activePool)
	g.Expect(err).To(BeNil())
	g.Expect(ms.GetInstanceConfigurationId()).To(Equal(common.String("new-id")))

	_, err = ms.UpdatePool(context.Background(), activePool)
	g.Expect(err).To(HaveOccurred())
	g.Expect(ms.GetInstanceConfigurationId()).To(Equal(common.String("new-id")))
}

func TestCleanupInstanceConfigurationDefersWhenInstancePoolSwitchStalls(t *testing.T) {
	g := NewWithT(t)
	ms, computeMgmt := newInstanceConfigurationOrderingScope(t, "test")

	ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
		InstanceConfigurationId: common.String("new-id"),
	}
	stalledPool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		InstanceConfigurationId: common.String("old-id"),
	}

	computeMgmt.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).Times(0)
	computeMgmt.EXPECT().DeleteInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

	err := ms.CleanupInstanceConfiguration(context.Background(), stalledPool)
	g.Expect(err).To(BeNil())
}

func TestCleanupInstanceConfigurationDeletesOnlyAfterSuccessfulSwitch(t *testing.T) {
	g := NewWithT(t)
	ms, computeMgmt := newInstanceConfigurationOrderingScope(t, "test")

	ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
		InstanceConfigurationId: common.String("new-id"),
	}
	switchedPool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		InstanceConfigurationId: common.String("new-id"),
	}

	computeMgmt.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
		Return(core.ListInstanceConfigurationsResponse{
			Items: []core.InstanceConfigurationSummary{
				{
					Id:           common.String("new-id"),
					DisplayName:  common.String("test-new"),
					FreeformTags: ms.GetFreeFormTags(),
				},
				{
					Id:           common.String("old-id"),
					DisplayName:  common.String("test-old"),
					FreeformTags: ms.GetFreeFormTags(),
				},
			},
		}, nil)
	computeMgmt.EXPECT().DeleteInstanceConfiguration(gomock.Any(), gomock.Eq(core.DeleteInstanceConfigurationRequest{
		InstanceConfigurationId: common.String("old-id"),
	})).
		Return(core.DeleteInstanceConfigurationResponse{}, nil)

	err := ms.CleanupInstanceConfiguration(context.Background(), switchedPool)
	g.Expect(err).To(BeNil())
}

func TestCleanupInstanceConfigurationDefersWhenFormatterRemovalStalls(t *testing.T) {
	g := NewWithT(t)
	ms, computeMgmt := newInstanceConfigurationOrderingScope(t, "test")

	ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
		InstanceConfigurationId: common.String("new-id"),
	}
	stalledPool := &core.InstancePool{
		Id:                           common.String("pool-id"),
		InstanceConfigurationId:      common.String("new-id"),
		InstanceDisplayNameFormatter: common.String("old-${launchCount}"),
		InstanceHostnameFormatter:    common.String("old-host-${launchCount}"),
	}

	computeMgmt.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).Times(0)
	computeMgmt.EXPECT().DeleteInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

	g.Expect(ms.InstancePoolUsesDesiredInstanceConfiguration(stalledPool)).To(BeFalse())
	err := ms.CleanupInstanceConfiguration(context.Background(), stalledPool)
	g.Expect(err).To(BeNil())

	stalledPool.InstanceDisplayNameFormatter = common.String("")
	stalledPool.InstanceHostnameFormatter = common.String("")
	g.Expect(ms.InstancePoolUsesDesiredInstanceConfiguration(stalledPool)).To(BeTrue())
}

func TestReconcileInstanceConfigurationCoalescesMultipleApprovedFieldChanges(t *testing.T) {
	g := NewWithT(t)
	ms, computeMgmt := newInstanceConfigurationOrderingScope(t, "test")

	ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
		Shape:                      common.String("new-shape"),
		InstanceConfigurationId:    common.String("old-id"),
		LaunchMode:                 infrav2exp.LaunchModeEnum(core.InstanceConfigurationLaunchInstanceDetailsLaunchModeNative),
		PreferredMaintenanceAction: infrav2exp.PreferredMaintenanceActionEnum(core.InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionReboot),
	}
	stalledPool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		InstanceConfigurationId: common.String("old-id"),
		Size:                    common.Int(3),
	}
	desiredLaunch := orderingLaunchDetails("new-shape", "test")
	desiredLaunch.LaunchMode = core.InstanceConfigurationLaunchInstanceDetailsLaunchModeNative
	desiredLaunch.PreferredMaintenanceAction = core.InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionReboot
	desiredLaunch.FreeformTags = map[string]string{
		ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
		ociutil.ClusterResourceIdentifier: "resource_uid",
	}
	desiredLaunch.CreateVnicDetails.FreeformTags = desiredLaunch.FreeformTags

	computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
		InstanceConfigurationId: common.String("old-id"),
	})).
		Return(core.GetInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{
				Id: common.String("old-id"),
				InstanceDetails: core.ComputeInstanceDetails{
					LaunchDetails: orderingLaunchDetails("old-shape", "test"),
				},
			},
		}, nil)
	computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
		Return(core.CreateInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{Id: common.String("new-id")},
		}, nil)

	err := ms.ReconcileInstanceConfiguration(context.Background(), stalledPool)
	g.Expect(err).To(BeNil())
	g.Expect(ms.GetInstanceConfigurationId()).To(Equal(common.String("new-id")))

	computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
		InstanceConfigurationId: common.String("new-id"),
	})).
		Return(core.GetInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{
				Id: common.String("new-id"),
				InstanceDetails: core.ComputeInstanceDetails{
					LaunchDetails: desiredLaunch,
				},
			},
		}, nil)
	computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

	err = ms.ReconcileInstanceConfiguration(context.Background(), stalledPool)
	g.Expect(err).To(BeNil())
	g.Expect(ms.GetInstanceConfigurationId()).To(Equal(common.String("new-id")))
}

func TestBootstrapTriggeredRotationDefersCleanupBeforePoolSwitch(t *testing.T) {
	g := NewWithT(t)
	ms, computeMgmt := newInstanceConfigurationOrderingScope(t, "new-bootstrap")

	ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
		Shape:                   common.String("test-shape"),
		InstanceConfigurationId: common.String("old-id"),
	}
	activePool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		InstanceConfigurationId: common.String("old-id"),
		Size:                    common.Int(3),
	}

	computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Any()).
		Return(core.GetInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{
				Id: common.String("old-id"),
				InstanceDetails: core.ComputeInstanceDetails{
					LaunchDetails: orderingLaunchDetails("test-shape", "old-bootstrap"),
				},
			},
		}, nil)
	computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
		Return(core.CreateInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{Id: common.String("new-id")},
		}, nil)
	computeMgmt.EXPECT().DeleteInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

	err := ms.ReconcileInstanceConfiguration(context.Background(), activePool)
	g.Expect(err).To(BeNil())
	g.Expect(ms.GetInstanceConfigurationId()).To(Equal(common.String("new-id")))
}

func TestBootstrapTriggeredRotationUpdatesPoolBeforeCleanupEligibility(t *testing.T) {
	g := NewWithT(t)
	ms, computeMgmt := newInstanceConfigurationOrderingScope(t, "new-bootstrap")

	ms.OCIMachinePool.Spec.InstanceConfiguration = infrav2exp.InstanceConfiguration{
		Shape:                   common.String("test-shape"),
		InstanceConfigurationId: common.String("old-id"),
	}
	activePool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		InstanceConfigurationId: common.String("old-id"),
		Size:                    common.Int(3),
	}

	computeMgmt.EXPECT().GetInstanceConfiguration(gomock.Any(), gomock.Eq(core.GetInstanceConfigurationRequest{
		InstanceConfigurationId: common.String("old-id"),
	})).
		Return(core.GetInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{
				Id: common.String("old-id"),
				InstanceDetails: core.ComputeInstanceDetails{
					LaunchDetails: orderingLaunchDetails("test-shape", "old-bootstrap"),
				},
			},
		}, nil)
	computeMgmt.EXPECT().CreateInstanceConfiguration(gomock.Any(), gomock.Any()).
		Return(core.CreateInstanceConfigurationResponse{
			InstanceConfiguration: core.InstanceConfiguration{Id: common.String("new-id")},
		}, nil)

	err := ms.ReconcileInstanceConfiguration(context.Background(), activePool)
	g.Expect(err).To(BeNil())
	g.Expect(ms.GetInstanceConfigurationId()).To(Equal(common.String("new-id")))

	computeMgmt.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(expectedUpdateInstancePoolRequest(ms, activePool, core.UpdateInstancePoolDetails{
		Size:                    common.Int(3),
		InstanceConfigurationId: common.String("new-id"),
		FreeformTags: map[string]string{
			ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
			ociutil.ClusterResourceIdentifier: "resource_uid",
		},
	}))).
		Return(core.UpdateInstancePoolResponse{
			InstancePool: core.InstancePool{
				Id:                      common.String("pool-id"),
				InstanceConfigurationId: common.String("old-id"),
				Size:                    common.Int(3),
			},
		}, nil)
	computeMgmt.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).Times(0)
	computeMgmt.EXPECT().DeleteInstanceConfiguration(gomock.Any(), gomock.Any()).Times(0)

	updateOutcome, err := ms.UpdatePool(context.Background(), activePool)
	g.Expect(err).To(BeNil())
	g.Expect(updateOutcome).To(Equal(InstancePoolUpdateSubmitted))
	g.Expect(ms.InstancePoolUsesDesiredInstanceConfiguration(activePool)).To(BeFalse())
	err = ms.CleanupInstanceConfiguration(context.Background(), activePool)
	g.Expect(err).To(BeNil())

	switchedPool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		InstanceConfigurationId: common.String("new-id"),
		Size:                    common.Int(3),
		FreeformTags:            ms.GetFreeFormTags(),
		LifecycleState:          core.InstancePoolLifecycleStateRunning,
	}
	updateOutcome, err = ms.UpdatePool(context.Background(), switchedPool)
	g.Expect(err).To(BeNil())
	g.Expect(updateOutcome).To(Equal(InstancePoolUpdateConverged))
	g.Expect(ms.HasPendingInstancePoolUpdate()).To(BeFalse())
	computeMgmt.EXPECT().ListInstanceConfigurations(gomock.Any(), gomock.Any()).
		Return(core.ListInstanceConfigurationsResponse{
			Items: []core.InstanceConfigurationSummary{
				{
					Id:           common.String("new-id"),
					DisplayName:  common.String("test-new"),
					FreeformTags: ms.GetFreeFormTags(),
				},
				{
					Id:           common.String("old-id"),
					DisplayName:  common.String("test-old"),
					FreeformTags: ms.GetFreeFormTags(),
				},
			},
		}, nil)
	computeMgmt.EXPECT().DeleteInstanceConfiguration(gomock.Any(), gomock.Eq(core.DeleteInstanceConfigurationRequest{
		InstanceConfigurationId: common.String("old-id"),
	})).
		Return(core.DeleteInstanceConfigurationResponse{}, nil)

	err = ms.CleanupInstanceConfiguration(context.Background(), switchedPool)
	g.Expect(err).To(BeNil())
}

func newInstanceConfigurationOrderingScope(t *testing.T, bootstrapData string) (*MachinePoolScope, *mock_computemanagement.MockClient) {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte(bootstrapData),
		},
	}
	computeMgmt := mock_computemanagement.NewMockClient(mockCtrl)
	ociMachinePool := &infrav2exp.OCIMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
	client := fake.NewClientBuilder().WithStatusSubresource(ociMachinePool).WithObjects(secret, ociMachinePool).Build()
	replicas := int32(3)
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: computeMgmt,
		OCIMachinePool:          ociMachinePool,
		OCIClusterAccessor: OCISelfManagedCluster{
			OCICluster: &infrastructurev1beta2.OCICluster{
				ObjectMeta: metav1.ObjectMeta{UID: "cluster_uid"},
				Spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId:         "test-compartment",
					OCIResourceIdentifier: "resource_uid",
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							Subnets: []*infrastructurev1beta2.Subnet{
								{
									Role: infrastructurev1beta2.WorkerRole,
									ID:   common.String("subnet-id"),
									Type: infrastructurev1beta2.Private,
									Name: "worker-subnet",
								},
							},
						},
					},
				},
			},
		},
		Cluster: &clusterv1.Cluster{},
		MachinePool: &clusterv1.MachinePool{
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
		},
		Client: client,
	})
	if err != nil {
		t.Fatalf("NewMachinePoolScope: %v", err)
	}
	return ms, computeMgmt
}

func orderingLaunchDetails(shape, bootstrapData string) *core.InstanceConfigurationLaunchInstanceDetails {
	return &core.InstanceConfigurationLaunchInstanceDetails{
		CompartmentId: common.String("test-compartment"),
		Shape:         common.String(shape),
		CreateVnicDetails: &core.InstanceConfigurationCreateVnicDetails{
			SubnetId: common.String("subnet-id"),
		},
		SourceDetails: core.InstanceConfigurationInstanceSourceViaImageDetails{},
		Metadata: map[string]string{
			"user_data": base64.StdEncoding.EncodeToString([]byte(bootstrapData)),
		},
	}
}

func expectedUpdateInstancePoolRequest(ms *MachinePoolScope, instancePool *core.InstancePool, details core.UpdateInstancePoolDetails) core.UpdateInstancePoolRequest {
	if instancePool.LifecycleState == "" {
		instancePool.LifecycleState = core.InstancePoolLifecycleStateRunning
	}
	attempt, err := newInstancePoolUpdateAttempt(details, ms.InstancePoolETag)
	if err != nil {
		panic(err)
	}
	attempt.RetryToken = "test-update-instance-pool-" + attempt.Fingerprint
	data, err := json.Marshal(attempt)
	if err != nil {
		panic(err)
	}
	expected := ""
	if ms.OCIMachinePool.Annotations != nil {
		expected = ms.OCIMachinePool.Annotations[InstancePoolUpdateAttemptAnnotation]
	}
	if err := ms.transitionInstancePoolUpdateAttempt(context.Background(), expected, string(data)); err != nil {
		panic(err)
	}
	request := core.UpdateInstancePoolRequest{
		InstancePoolId:            instancePool.Id,
		UpdateInstancePoolDetails: attempt.Target.updateDetails(),
		OpcRetryToken:             common.String(attempt.RetryToken),
	}
	if attempt.IfMatch != "" {
		request.IfMatch = common.String(attempt.IfMatch)
	}
	return request
}

func setTestInstancePoolUpdateAttempt(t *testing.T, machinePool *infrav2exp.OCIMachinePool, attempt *instancePoolUpdateAttempt) {
	t.Helper()
	data, err := json.Marshal(attempt)
	if err != nil {
		t.Fatalf("marshal instance pool update attempt: %v", err)
	}
	if machinePool.Annotations == nil {
		machinePool.Annotations = map[string]string{}
	}
	machinePool.Annotations[InstancePoolUpdateAttemptAnnotation] = string(data)
}

type testServiceError struct {
	status int
}

func (e testServiceError) Error() string          { return fmt.Sprintf("OCI service error %d", e.status) }
func (e testServiceError) GetHTTPStatusCode() int { return e.status }
func (e testServiceError) GetMessage() string     { return e.Error() }
func (e testServiceError) GetCode() string        { return "NoEtagMatch" }
func (e testServiceError) GetOpcRequestID() string {
	return "test-request-id"
}

func TestInstancePoolUpdateTargetFingerprintIsPlacementOrderIndependent(t *testing.T) {
	g := NewWithT(t)
	details := core.UpdateInstancePoolDetails{
		PlacementConfigurations: []core.UpdateInstancePoolPlacementConfigurationDetails{
			{AvailabilityDomain: common.String("ad-2"), FaultDomains: []string{"fd-2", "fd-1"}},
			{AvailabilityDomain: common.String("ad-1"), FaultDomains: []string{"fd-1"}},
		},
	}
	reordered := details
	reordered.PlacementConfigurations = []core.UpdateInstancePoolPlacementConfigurationDetails{
		{AvailabilityDomain: common.String("ad-1"), FaultDomains: []string{"fd-1"}},
		{AvailabilityDomain: common.String("ad-2"), FaultDomains: []string{"fd-1", "fd-2"}},
	}

	first, err := newInstancePoolUpdateTarget(details).fingerprint()
	g.Expect(err).To(BeNil())
	second, err := newInstancePoolUpdateTarget(reordered).fingerprint()
	g.Expect(err).To(BeNil())
	g.Expect(second).To(Equal(first))
}

func TestInstancePoolUpdateAttemptRejectsUnsupportedSchemaVersion(t *testing.T) {
	g := NewWithT(t)
	attempt, err := newInstancePoolUpdateAttempt(core.UpdateInstancePoolDetails{Size: common.Int(2)}, nil)
	g.Expect(err).To(BeNil())
	attempt.Version++
	machinePool := &infrav2exp.OCIMachinePool{}
	setTestInstancePoolUpdateAttempt(t, machinePool, attempt)

	_, err = (&MachinePoolScope{OCIMachinePool: machinePool}).getInstancePoolUpdateAttempt()
	g.Expect(err).To(MatchError(ContainSubstring("unsupported instance pool update attempt schema version")))
	g.Expect(err).To(MatchError(ContainSubstring("verify the OCI operation is terminal")))
}

func TestInstancePoolFormatterClearingWireSerialization(t *testing.T) {
	g := NewWithT(t)
	requestBody := func(details core.UpdateInstancePoolDetails) map[string]interface{} {
		req, err := (core.UpdateInstancePoolRequest{
			InstancePoolId:            common.String("pool-id"),
			UpdateInstancePoolDetails: details,
		}).HTTPRequest(http.MethodPut, "/20160918/instancePools/pool-id", nil, nil)
		g.Expect(err).To(BeNil())
		defer req.Body.Close()

		body, err := io.ReadAll(req.Body)
		g.Expect(err).To(BeNil())
		payload := map[string]interface{}{}
		g.Expect(json.Unmarshal(body, &payload)).To(Succeed())
		return payload
	}

	omitted := requestBody(core.UpdateInstancePoolDetails{})
	g.Expect(omitted).ToNot(HaveKey("instanceDisplayNameFormatter"))
	g.Expect(omitted).ToNot(HaveKey("instanceHostnameFormatter"))

	cleared := requestBody(core.UpdateInstancePoolDetails{
		InstanceDisplayNameFormatter: common.String(""),
		InstanceHostnameFormatter:    common.String(""),
	})
	g.Expect(cleared).To(HaveKeyWithValue("instanceDisplayNameFormatter", ""))
	g.Expect(cleared).To(HaveKeyWithValue("instanceHostnameFormatter", ""))
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
	percentage := 50
	vcpus := 4
	spec := infrav2exp.InstanceConfiguration{
		Shape:                          common.String("test-shape"),
		Metadata:                       metadata,
		IsPvEncryptionInTransitEnabled: common.Bool(true),
		ClusterPlacementGroupId:        common.String("cluster-placement-group-id"),
		IpxeScript:                     common.String("#!ipxe"),
		LaunchMode:                     infrav2exp.LaunchModeEnum(core.InstanceConfigurationLaunchInstanceDetailsLaunchModeNative),
		LicensingConfigs: []infrav2exp.LaunchInstanceLicensingConfig{{
			Type:        infrav2exp.LaunchInstanceLicensingConfigTypeEnum(core.LaunchInstanceLicensingConfigTypeWindows),
			LicenseType: infrav2exp.LaunchInstanceLicensingConfigLicenseTypeEnum(core.LaunchInstanceLicensingConfigLicenseTypeBringYourOwnLicense),
		}},
		PreferredMaintenanceAction: infrav2exp.PreferredMaintenanceActionEnum(core.InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionReboot),
		ShapeConfig: &infrav2exp.ShapeConfig{
			Vcpus: &vcpus,
		},
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
		PlatformConfig: &infrastructurev1beta2.PlatformConfig{
			PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeIntelSkylakeBm,
			IntelSkylakeBmPlatformConfig: infrastructurev1beta2.IntelSkylakeBmPlatformConfig{
				IsSymmetricMultiThreadingEnabled:         common.Bool(false),
				IsInputOutputMemoryManagementUnitEnabled: common.Bool(true),
				PercentageOfCoresEnabled:                 &percentage,
				NumaNodesPerSocket:                       infrastructurev1beta2.IntelSkylakeBmPlatformConfigNumaNodesPerSocketNps2,
			},
		},
	}
	ms.OCIMachinePool.Spec.InstanceConfiguration = spec

	launchDetails, err := ms.getLaunchInstanceDetails(spec, map[string]string{"freeform": "tag"}, nil)
	g.Expect(err).To(BeNil())
	g.Expect(metadata).To(Equal(map[string]string{"ssh_authorized_keys": "ssh-rsa test"}))
	g.Expect(launchDetails.Metadata).To(HaveKey("user_data"))
	g.Expect(launchDetails.Metadata["ssh_authorized_keys"]).To(Equal("ssh-rsa test"))
	g.Expect(*launchDetails.IsPvEncryptionInTransitEnabled).To(BeTrue())
	g.Expect(*launchDetails.CreateVnicDetails.SubnetId).To(Equal("explicit-subnet-id"))
	g.Expect(launchDetails.CreateVnicDetails.NsgIds).To(Equal([]string{"explicit-nsg-id"}))
	g.Expect(*launchDetails.CreateVnicDetails.AssignIpv6Ip).To(BeTrue())
	g.Expect(*launchDetails.CreateVnicDetails.AssignPublicIp).To(BeFalse())
	g.Expect(*launchDetails.ClusterPlacementGroupId).To(Equal("cluster-placement-group-id"))
	g.Expect(*launchDetails.IpxeScript).To(Equal("#!ipxe"))
	g.Expect(launchDetails.LaunchMode).To(Equal(core.InstanceConfigurationLaunchInstanceDetailsLaunchModeNative))
	g.Expect(launchDetails.PreferredMaintenanceAction).To(Equal(core.InstanceConfigurationLaunchInstanceDetailsPreferredMaintenanceActionReboot))
	g.Expect(launchDetails.LicensingConfigs).To(Equal([]core.LaunchInstanceLicensingConfig{core.LaunchInstanceWindowsLicensingConfig{
		LicenseType: core.LaunchInstanceLicensingConfigLicenseTypeBringYourOwnLicense,
	}}))
	g.Expect(*launchDetails.ShapeConfig.Vcpus).To(Equal(4))
	platformConfig, ok := launchDetails.PlatformConfig.(core.IntelSkylakeBmPlatformConfig)
	g.Expect(ok).To(BeTrue())
	g.Expect(*platformConfig.IsSymmetricMultiThreadingEnabled).To(BeFalse())
	g.Expect(*platformConfig.IsInputOutputMemoryManagementUnitEnabled).To(BeTrue())
	g.Expect(*platformConfig.PercentageOfCoresEnabled).To(Equal(50))
	g.Expect(platformConfig.NumaNodesPerSocket).To(Equal(core.IntelSkylakeBmPlatformConfigNumaNodesPerSocketNps2))

	sourceDetails, ok := launchDetails.SourceDetails.(core.InstanceConfigurationInstanceSourceViaImageDetails)
	g.Expect(ok).To(BeTrue())
	g.Expect(*sourceDetails.KmsKeyId).To(Equal("kms-id"))
	g.Expect(*sourceDetails.BootVolumeSizeInGBs).To(Equal(int64(100)))
	g.Expect(*sourceDetails.BootVolumeVpusPerGB).To(Equal(int64(20)))
	g.Expect(*launchDetails.AvailabilityConfig.IsLiveMigrationPreferred).To(BeTrue())
}

func TestMachinePoolEffectiveInstanceTags(t *testing.T) {
	g := NewWithT(t)

	ociCluster := &infrastructurev1beta2.OCICluster{
		Spec: infrastructurev1beta2.OCIClusterSpec{
			OCIResourceIdentifier: "resource_uid",
			FreeformTags: map[string]string{
				"cluster-only":                    "cluster",
				"overlap":                         "cluster",
				ociutil.CreatedBy:                 "user-created-by",
				ociutil.ClusterResourceIdentifier: "user-resource-id",
			},
			DefinedTags: map[string]map[string]string{
				"Operations": {
					"CostCenter": "cluster-42",
					"Owner":      "cluster-platform",
				},
				"Security": {
					"Profile": "restricted",
				},
			},
		},
	}
	machinePool := &infrav2exp.OCIMachinePool{
		Spec: infrav2exp.OCIMachinePoolSpec{
			InstanceConfiguration: infrav2exp.InstanceConfiguration{
				FreeformTags: map[string]string{
					"pool-only":       "pool",
					"overlap":         "pool",
					ociutil.CreatedBy: "pool-created-by",
				},
				DefinedTags: map[string]map[string]string{
					"Operations": {
						"CostCenter": "pool-43",
					},
					"Billing": {
						"Project": "capoci",
					},
				},
			},
		},
	}
	ms := &MachinePoolScope{
		OCIClusterAccesor: OCISelfManagedCluster{OCICluster: ociCluster},
		OCIMachinePool:    machinePool,
	}

	g.Expect(ms.GetFreeFormTags()).To(Equal(map[string]string{
		"cluster-only":                    "cluster",
		"pool-only":                       "pool",
		"overlap":                         "pool",
		ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
		ociutil.ClusterResourceIdentifier: "resource_uid",
	}))

	effectiveDefinedTags := ms.getDefinedTags()
	g.Expect(effectiveDefinedTags).To(Equal(map[string]map[string]interface{}{
		"Operations": {
			"CostCenter": "pool-43",
			"Owner":      "cluster-platform",
		},
		"Security": {
			"Profile": "restricted",
		},
		"Billing": {
			"Project": "capoci",
		},
	}))

	ociCluster.Spec.DefinedTags["Operations"]["Owner"] = "mutated"
	machinePool.Spec.InstanceConfiguration.DefinedTags["Operations"]["CostCenter"] = "mutated"
	g.Expect(effectiveDefinedTags["Operations"]["Owner"]).To(Equal("cluster-platform"))
	g.Expect(effectiveDefinedTags["Operations"]["CostCenter"]).To(Equal("pool-43"))
}

func TestGetLaunchInstanceDetailsAppliesEffectiveInstanceTagsToLaunchAndVnic(t *testing.T) {
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
			CompartmentId:         "test-compartment",
			OCIResourceIdentifier: "resource_uid",
			FreeformTags: map[string]string{
				"cluster-only": "cluster",
				"overlap":      "cluster",
			},
			DefinedTags: map[string]map[string]string{
				"Operations": {
					"CostCenter": "cluster-42",
					"Owner":      "cluster-platform",
				},
			},
			NetworkSpec: infrastructurev1beta2.NetworkSpec{
				Vcn: infrastructurev1beta2.VCN{
					Subnets: []*infrastructurev1beta2.Subnet{
						{
							Role: infrastructurev1beta2.WorkerRole,
							ID:   common.String("worker-subnet-id"),
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
		Spec: infrav2exp.OCIMachinePoolSpec{
			InstanceConfiguration: infrav2exp.InstanceConfiguration{
				Shape: common.String("test-shape"),
				FreeformTags: map[string]string{
					"pool-only": "pool",
					"overlap":   "pool",
				},
				DefinedTags: map[string]map[string]string{
					"Operations": {
						"CostCenter": "pool-43",
					},
				},
			},
		},
	}
	client := fake.NewClientBuilder().WithStatusSubresource(machinePool).WithObjects(secret, machinePool).Build()
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: mock_computemanagement.NewMockClient(mockCtrl),
		OCIMachinePool:          machinePool,
		OCIClusterAccessor:      OCISelfManagedCluster{OCICluster: ociCluster},
		Cluster:                 &clusterv1.Cluster{},
		MachinePool: &clusterv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
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

	freeformTags := ms.GetFreeFormTags()
	definedTags := ms.getDefinedTags()
	launchDetails, err := ms.getLaunchInstanceDetails(machinePool.Spec.InstanceConfiguration, freeformTags, definedTags)
	g.Expect(err).To(BeNil())

	expectedFreeformTags := map[string]string{
		"cluster-only":                    "cluster",
		"pool-only":                       "pool",
		"overlap":                         "pool",
		ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
		ociutil.ClusterResourceIdentifier: "resource_uid",
	}
	expectedDefinedTags := map[string]map[string]interface{}{
		"Operations": {
			"CostCenter": "pool-43",
			"Owner":      "cluster-platform",
		},
	}
	g.Expect(launchDetails.FreeformTags).To(Equal(expectedFreeformTags))
	g.Expect(launchDetails.DefinedTags).To(Equal(expectedDefinedTags))
	g.Expect(launchDetails.CreateVnicDetails.FreeformTags).To(Equal(expectedFreeformTags))
	g.Expect(launchDetails.CreateVnicDetails.DefinedTags).To(Equal(expectedDefinedTags))
}

func TestGetLaunchInstanceDetailsMapsDeprecatedNSGIdFallback(t *testing.T) {
	tests := []struct {
		name     string
		config   infrastructurev1beta2.NetworkDetails
		expected []string
	}{
		{
			name: "uses deprecated nsgId when nsgIds is empty",
			config: infrastructurev1beta2.NetworkDetails{
				NSGId: common.String("legacy-nsg-id"),
			},
			expected: []string{"legacy-nsg-id"},
		},
		{
			name: "prefers nsgIds over deprecated nsgId",
			config: infrastructurev1beta2.NetworkDetails{
				NSGId:  common.String("legacy-nsg-id"),
				NSGIds: []string{"preferred-nsg-id"},
			},
			expected: []string{"preferred-nsg-id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ms, _ := newInstanceConfigurationOrderingScope(t, "test")
			spec := infrav2exp.InstanceConfiguration{
				Shape:                     common.String("test-shape"),
				InstanceVnicConfiguration: &tt.config,
			}
			ms.OCIMachinePool.Spec.InstanceConfiguration = spec

			launchDetails, err := ms.getLaunchInstanceDetails(spec, nil, nil)
			g.Expect(err).To(BeNil())
			g.Expect(launchDetails.CreateVnicDetails).ToNot(BeNil())
			g.Expect(launchDetails.CreateVnicDetails.NsgIds).To(Equal(tt.expected))
		})
	}
}

func TestGetLaunchInstanceDetailsRejectsUnknownLaunchMode(t *testing.T) {
	g := NewWithT(t)
	ms, _ := newInstanceConfigurationOrderingScope(t, "test")
	spec := infrav2exp.InstanceConfiguration{
		Shape:      common.String("test-shape"),
		LaunchMode: infrav2exp.LaunchModeEnum("UNKNOWN_MODE"),
	}
	ms.OCIMachinePool.Spec.InstanceConfiguration = spec

	launchDetails, err := ms.getLaunchInstanceDetails(spec, nil, nil)
	g.Expect(err).To(MatchError(ContainSubstring(`unsupported launch mode "UNKNOWN_MODE"`)))
	g.Expect(launchDetails).To(BeNil())
}

func TestGetLaunchInstanceDetailsRejectsUnknownPreferredMaintenanceAction(t *testing.T) {
	g := NewWithT(t)
	ms, _ := newInstanceConfigurationOrderingScope(t, "test")
	spec := infrav2exp.InstanceConfiguration{
		Shape:                      common.String("test-shape"),
		PreferredMaintenanceAction: infrav2exp.PreferredMaintenanceActionEnum("POWER_CYCLE"),
	}
	ms.OCIMachinePool.Spec.InstanceConfiguration = spec

	launchDetails, err := ms.getLaunchInstanceDetails(spec, nil, nil)
	g.Expect(err).To(MatchError(ContainSubstring(`unsupported preferred maintenance action "POWER_CYCLE"`)))
	g.Expect(launchDetails).To(BeNil())
}

func TestGetLaunchInstanceDetailsRejectsUnknownLicensingConfigType(t *testing.T) {
	g := NewWithT(t)
	ms, _ := newInstanceConfigurationOrderingScope(t, "test")
	spec := infrav2exp.InstanceConfiguration{
		Shape: common.String("test-shape"),
		LicensingConfigs: []infrav2exp.LaunchInstanceLicensingConfig{{
			Type:        infrav2exp.LaunchInstanceLicensingConfigTypeEnum("LINUX"),
			LicenseType: infrav2exp.LaunchInstanceLicensingConfigLicenseTypeBringYourOwnLicense,
		}},
	}
	ms.OCIMachinePool.Spec.InstanceConfiguration = spec

	launchDetails, err := ms.getLaunchInstanceDetails(spec, nil, nil)
	g.Expect(err).To(MatchError(ContainSubstring(`unsupported licensing config type "LINUX"`)))
	g.Expect(launchDetails).To(BeNil())
}

func TestGetLaunchInstanceDetailsRejectsUnknownLicensingConfigLicenseType(t *testing.T) {
	g := NewWithT(t)
	ms, _ := newInstanceConfigurationOrderingScope(t, "test")
	spec := infrav2exp.InstanceConfiguration{
		Shape: common.String("test-shape"),
		LicensingConfigs: []infrav2exp.LaunchInstanceLicensingConfig{{
			Type:        infrav2exp.LaunchInstanceLicensingConfigTypeWindows,
			LicenseType: infrav2exp.LaunchInstanceLicensingConfigLicenseTypeEnum("RENTED"),
		}},
	}
	ms.OCIMachinePool.Spec.InstanceConfiguration = spec

	launchDetails, err := ms.getLaunchInstanceDetails(spec, nil, nil)
	g.Expect(err).To(MatchError(ContainSubstring(`unsupported licensing config license type "RENTED"`)))
	g.Expect(launchDetails).To(BeNil())
}

func TestGetPlatformConfigPropagatesApprovedPlatformFields(t *testing.T) {
	falseValue := false
	trueValue := true
	percentage := 50

	tests := []struct {
		name           string
		platformConfig *infrastructurev1beta2.PlatformConfig
		assert         func(g *WithT, platformConfig core.PlatformConfig)
	}{
		{
			name: "AMD VM SMT",
			platformConfig: &infrastructurev1beta2.PlatformConfig{
				PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeAmdvm,
				AmdVmPlatformConfig: infrastructurev1beta2.AmdVmPlatformConfig{
					IsSymmetricMultiThreadingEnabled: &falseValue,
				},
			},
			assert: func(g *WithT, platformConfig core.PlatformConfig) {
				actual, ok := platformConfig.(core.AmdVmPlatformConfig)
				g.Expect(ok).To(BeTrue())
				g.Expect(actual.IsSymmetricMultiThreadingEnabled).To(Equal(&falseValue))
			},
		},
		{
			name: "Intel Skylake BM approved knobs",
			platformConfig: &infrastructurev1beta2.PlatformConfig{
				PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeIntelSkylakeBm,
				IntelSkylakeBmPlatformConfig: infrastructurev1beta2.IntelSkylakeBmPlatformConfig{
					IsSymmetricMultiThreadingEnabled:         &falseValue,
					IsInputOutputMemoryManagementUnitEnabled: &trueValue,
					PercentageOfCoresEnabled:                 &percentage,
					NumaNodesPerSocket:                       infrastructurev1beta2.IntelSkylakeBmPlatformConfigNumaNodesPerSocketNps2,
				},
			},
			assert: func(g *WithT, platformConfig core.PlatformConfig) {
				actual, ok := platformConfig.(core.IntelSkylakeBmPlatformConfig)
				g.Expect(ok).To(BeTrue())
				g.Expect(actual.IsSymmetricMultiThreadingEnabled).To(Equal(&falseValue))
				g.Expect(actual.IsInputOutputMemoryManagementUnitEnabled).To(Equal(&trueValue))
				g.Expect(actual.PercentageOfCoresEnabled).To(Equal(&percentage))
				g.Expect(actual.NumaNodesPerSocket).To(Equal(core.IntelSkylakeBmPlatformConfigNumaNodesPerSocketNps2))
			},
		},
		{
			name: "Intel VM SMT",
			platformConfig: &infrastructurev1beta2.PlatformConfig{
				PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeIntelVm,
				IntelVmPlatformConfig: infrastructurev1beta2.IntelVmPlatformConfig{
					IsSymmetricMultiThreadingEnabled: &trueValue,
				},
			},
			assert: func(g *WithT, platformConfig core.PlatformConfig) {
				actual, ok := platformConfig.(core.IntelVmPlatformConfig)
				g.Expect(ok).To(BeTrue())
				g.Expect(actual.IsSymmetricMultiThreadingEnabled).To(Equal(&trueValue))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ms := &MachinePoolScope{
				OCIMachinePool: &infrav2exp.OCIMachinePool{
					Spec: infrav2exp.OCIMachinePoolSpec{
						InstanceConfiguration: infrav2exp.InstanceConfiguration{
							PlatformConfig: tt.platformConfig,
						},
					},
				},
			}

			tt.assert(g, ms.getPlatformConfig())
		})
	}
}

func TestValidatePlatformConfigRejectsUnknownSkylakeNumaNodes(t *testing.T) {
	err := validatePlatformConfig(&infrastructurev1beta2.PlatformConfig{
		PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeIntelSkylakeBm,
		IntelSkylakeBmPlatformConfig: infrastructurev1beta2.IntelSkylakeBmPlatformConfig{
			NumaNodesPerSocket: infrastructurev1beta2.IntelSkylakeBmPlatformConfigNumaNodesPerSocketEnum("NPS3"),
		},
	})
	NewWithT(t).Expect(err).To(MatchError(ContainSubstring("unsupported Intel Skylake NUMA nodes per socket")))
}

func TestGetLaunchInstanceDetailsExtendedMetadata(t *testing.T) {
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
			OCICluster: &infrastructurev1beta2.OCICluster{
				Spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId: "test-compartment",
				},
			},
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

	spec := infrav2exp.InstanceConfiguration{
		Shape: common.String("test-shape"),
		ExtendedMetadata: map[string]apiextensionsv1.JSON{
			"cilium-primary-vnic": {
				Raw: []byte(`{"ip-count":32,"cidr-blocks":["10.0.0.0/24"]}`),
			},
		},
	}
	ms.OCIMachinePool.Spec.InstanceConfiguration = spec

	launchDetails, err := ms.getLaunchInstanceDetails(spec, map[string]string{"freeform": "tag"}, nil)
	g.Expect(err).To(BeNil())
	g.Expect(launchDetails.ExtendedMetadata).ToNot(BeNil())
	g.Expect(launchDetails.ExtendedMetadata).To(HaveKey("cilium-primary-vnic"))
	ciliumConfig, ok := launchDetails.ExtendedMetadata["cilium-primary-vnic"].(map[string]interface{})
	g.Expect(ok).To(BeTrue())
	g.Expect(ciliumConfig["ip-count"]).To(Equal(float64(32)))
	g.Expect(ciliumConfig["cidr-blocks"]).To(Equal([]interface{}{"10.0.0.0/24"}))
}

func TestGetLaunchInstanceDetailsNilExtendedMetadata(t *testing.T) {
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
			OCICluster: &infrastructurev1beta2.OCICluster{
				Spec: infrastructurev1beta2.OCIClusterSpec{
					CompartmentId: "test-compartment",
				},
			},
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

	spec := infrav2exp.InstanceConfiguration{
		Shape: common.String("test-shape"),
	}
	ms.OCIMachinePool.Spec.InstanceConfiguration = spec

	launchDetails, err := ms.getLaunchInstanceDetails(spec, map[string]string{"freeform": "tag"}, nil)
	g.Expect(err).To(BeNil())
	g.Expect(launchDetails.ExtendedMetadata).To(BeNil())
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
				ms.OCIMachinePool.Spec.InstanceDisplayNameFormatter = common.String("worker-${launchCount}")
				ms.OCIMachinePool.Spec.InstanceHostnameFormatter = common.String("worker-${launchCount}")
				ms.OCIMachinePool.Spec.PlacementDetails = []infrav2exp.PlacementDetails{{
					AvailabilityDomain: 1,
					PrimaryVnicSubnets: &infrav2exp.InstancePoolPlacementPrimarySubnet{
						SubnetId:       common.String("primary-subnet-id"),
						IsAssignIpv6Ip: common.Bool(true),
					},
				}}
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
				computeManagementClient.EXPECT().CreateInstancePool(gomock.Any(), gomock.Eq(core.CreateInstancePoolRequest{
					CreateInstancePoolDetails: core.CreateInstancePoolDetails{
						CompartmentId:           common.String("test-compartment"),
						InstanceConfigurationId: common.String("config_id"),
						Size:                    common.Int(3),
						DisplayName:             common.String("test"),
						PlacementConfigurations: []core.CreateInstancePoolPlacementConfigurationDetails{{
							AvailabilityDomain: common.String("ad-1"),
							FaultDomains:       []string{"fd-5", "fd-6"},
							PrimaryVnicSubnets: &core.InstancePoolPlacementPrimarySubnet{
								SubnetId:       common.String("primary-subnet-id"),
								IsAssignIpv6Ip: common.Bool(true),
							},
						}},
						FreeformTags:                 tags,
						InstanceDisplayNameFormatter: common.String("worker-${launchCount}"),
						InstanceHostnameFormatter:    common.String("worker-${launchCount}"),
					},
				})).
					Return(core.CreateInstancePoolResponse{
						InstancePool: core.InstancePool{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance pool primary VNIC subnet defaults to worker subnet",
			errorExpected: false,
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.PlacementDetails = []infrav2exp.PlacementDetails{{
					AvailabilityDomain: 1,
					PrimaryVnicSubnets: &infrav2exp.InstancePoolPlacementPrimarySubnet{
						IsAssignIpv6Ip: common.Bool(false),
					},
				}}
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
				computeManagementClient.EXPECT().CreateInstancePool(gomock.Any(), gomock.Eq(core.CreateInstancePoolRequest{
					CreateInstancePoolDetails: core.CreateInstancePoolDetails{
						CompartmentId:           common.String("test-compartment"),
						InstanceConfigurationId: common.String("config_id"),
						Size:                    common.Int(3),
						DisplayName:             common.String("test"),
						PlacementConfigurations: []core.CreateInstancePoolPlacementConfigurationDetails{{
							AvailabilityDomain: common.String("ad-1"),
							FaultDomains:       []string{"fd-5", "fd-6"},
							PrimaryVnicSubnets: &core.InstancePoolPlacementPrimarySubnet{
								SubnetId:       common.String("subnet-id"),
								IsAssignIpv6Ip: common.Bool(false),
							},
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

func TestBuildInstancePoolPlacement(t *testing.T) {
	tests := []struct {
		name             string
		placementDetails []infrav2exp.PlacementDetails
		expected         []core.CreateInstancePoolPlacementConfigurationDetails
		errorExpected    bool
	}{
		{
			name: "uses requested fault domains for selected availability domain",
			placementDetails: []infrav2exp.PlacementDetails{
				{
					AvailabilityDomain: 1,
					FaultDomains:       []string{"fd-2"},
				},
			},
			expected: []core.CreateInstancePoolPlacementConfigurationDetails{
				{
					AvailabilityDomain: common.String("test-ad-1"),
					PrimarySubnetId:    common.String("subnet-id"),
					FaultDomains:       []string{"fd-2"},
				},
			},
		},
		{
			name: "uses cluster fault domains when selected availability domain omits fault domains",
			placementDetails: []infrav2exp.PlacementDetails{
				{
					AvailabilityDomain: 2,
				},
			},
			expected: []core.CreateInstancePoolPlacementConfigurationDetails{
				{
					AvailabilityDomain: common.String("test-ad-2"),
					PrimarySubnetId:    common.String("subnet-id"),
					FaultDomains:       []string{"fd-3", "fd-4"},
				},
			},
		},
		{
			name: "uses all availability domains and cluster fault domains when placement details are omitted",
			expected: []core.CreateInstancePoolPlacementConfigurationDetails{
				{
					AvailabilityDomain: common.String("test-ad-1"),
					PrimarySubnetId:    common.String("subnet-id"),
					FaultDomains:       []string{"fd-1", "fd-2"},
				},
				{
					AvailabilityDomain: common.String("test-ad-2"),
					PrimarySubnetId:    common.String("subnet-id"),
					FaultDomains:       []string{"fd-3", "fd-4"},
				},
			},
		},
		{
			name: "errors when more placement details are requested than cluster availability domains",
			placementDetails: []infrav2exp.PlacementDetails{
				{AvailabilityDomain: 1},
				{AvailabilityDomain: 2},
				{AvailabilityDomain: 3},
			},
			errorExpected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			infraMachinePool := &infrav2exp.OCIMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: infrav2exp.OCIMachinePoolSpec{
					PlacementDetails: tc.placementDetails,
				},
			}
			scheme := runtime.NewScheme()
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
			g.Expect(infrav2exp.AddToScheme(scheme)).To(Succeed())

			ms, err := NewMachinePoolScope(MachinePoolScopeParams{
				ComputeManagementClient: mock_computemanagement.NewMockClient(mockCtrl),
				OCIMachinePool:          infraMachinePool,
				OCIClusterAccessor: OCISelfManagedCluster{
					OCICluster: &infrastructurev1beta2.OCICluster{
						Spec: infrastructurev1beta2.OCIClusterSpec{
							NetworkSpec: infrastructurev1beta2.NetworkSpec{
								Vcn: infrastructurev1beta2.VCN{
									Subnets: []*infrastructurev1beta2.Subnet{
										{
											Role: infrastructurev1beta2.WorkerRole,
											ID:   common.String("subnet-id"),
										},
									},
								},
							},
							AvailabilityDomains: map[string]infrastructurev1beta2.OCIAvailabilityDomain{
								"test-ad-1": {
									Name:         "test-ad-1",
									FaultDomains: []string{"fd-1", "fd-2"},
								},
								"test-ad-2": {
									Name:         "test-ad-2",
									FaultDomains: []string{"fd-3", "fd-4"},
								},
							},
						},
					},
				},
				Cluster:     &clusterv1.Cluster{},
				MachinePool: &clusterv1.MachinePool{},
				Client:      fake.NewClientBuilder().WithScheme(scheme).WithObjects(infraMachinePool).Build(),
			})
			g.Expect(err).To(BeNil())

			placements, err := ms.BuildInstancePoolPlacement()
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
				return
			}
			g.Expect(err).To(BeNil())
			g.Expect(placements).To(Equal(tc.expected))
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
					"ad-2": {
						Name:         "ad-2",
						FaultDomains: []string{"fd-7", "fd-8"},
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

	matchingPlacementConfigurations := []core.InstancePoolPlacementConfiguration{
		{
			AvailabilityDomain: common.String("ad-1"),
			PrimarySubnetId:    common.String("subnet-id"),
			FaultDomains:       []string{"fd-5", "fd-6"},
		},
		{
			AvailabilityDomain: common.String("ad-2"),
			PrimarySubnetId:    common.String("subnet-id"),
			FaultDomains:       []string{"fd-7", "fd-8"},
		},
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
				PlacementConfigurations: matchingPlacementConfigurations,
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
				PlacementConfigurations: matchingPlacementConfigurations,
			},
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id_new")
				computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(expectedUpdateInstancePoolRequest(ms, &core.InstancePool{
					Size:                    common.Int(3),
					InstanceConfigurationId: common.String("config_id"),
					PlacementConfigurations: matchingPlacementConfigurations,
				}, core.UpdateInstancePoolDetails{
					Size:                    common.Int(3),
					InstanceConfigurationId: common.String("config_id_new"),
					FreeformTags:            tags,
				}))).
					Return(core.UpdateInstancePoolResponse{
						InstancePool: core.InstancePool{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance pool update due to availability domain and fault domain change",
			errorExpected: false,
			instancepool: &core.InstancePool{
				Size:                    common.Int(3),
				InstanceConfigurationId: common.String("config_id"),
				PlacementConfigurations: []core.InstancePoolPlacementConfiguration{
					{
						AvailabilityDomain: common.String("ad-1"),
						PrimarySubnetId:    common.String("subnet-id"),
						FaultDomains:       []string{"fd-5", "fd-6"},
					},
				},
			},
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
				ms.OCIMachinePool.Spec.PlacementDetails = []infrav2exp.PlacementDetails{
					{
						AvailabilityDomain: 2,
						FaultDomains:       []string{"fd-7"},
					},
				}
				computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(expectedUpdateInstancePoolRequest(ms, &core.InstancePool{
					Size:                    common.Int(3),
					InstanceConfigurationId: common.String("config_id"),
					PlacementConfigurations: []core.InstancePoolPlacementConfiguration{
						{
							AvailabilityDomain: common.String("ad-1"),
							PrimarySubnetId:    common.String("subnet-id"),
							FaultDomains:       []string{"fd-5", "fd-6"},
						},
					},
				}, core.UpdateInstancePoolDetails{
					Size:                    common.Int(3),
					InstanceConfigurationId: common.String("config_id"),
					FreeformTags:            tags,
					PlacementConfigurations: []core.UpdateInstancePoolPlacementConfigurationDetails{
						{
							AvailabilityDomain: common.String("ad-2"),
							PrimarySubnetId:    common.String("subnet-id"),
							FaultDomains:       []string{"fd-7"},
						},
					},
				}))).
					Return(core.UpdateInstancePoolResponse{
						InstancePool: core.InstancePool{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance pool placement update omits size when replicas are externally managed",
			errorExpected: false,
			instancepool: &core.InstancePool{
				Size:                    common.Int(5),
				InstanceConfigurationId: common.String("config_id"),
				PlacementConfigurations: []core.InstancePoolPlacementConfiguration{
					{
						AvailabilityDomain: common.String("ad-1"),
						PrimarySubnetId:    common.String("subnet-id"),
						FaultDomains:       []string{"fd-5", "fd-6"},
					},
				},
			},
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.MachinePool.Annotations = map[string]string{
					clusterv1.ReplicasManagedByAnnotation: "",
				}
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
				ms.OCIMachinePool.Spec.PlacementDetails = []infrav2exp.PlacementDetails{
					{
						AvailabilityDomain: 2,
						FaultDomains:       []string{"fd-7"},
					},
				}
				computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(expectedUpdateInstancePoolRequest(ms, &core.InstancePool{
					Size:                    common.Int(5),
					InstanceConfigurationId: common.String("config_id"),
					PlacementConfigurations: []core.InstancePoolPlacementConfiguration{
						{
							AvailabilityDomain: common.String("ad-1"),
							PrimarySubnetId:    common.String("subnet-id"),
							FaultDomains:       []string{"fd-5", "fd-6"},
						},
					},
				}, core.UpdateInstancePoolDetails{
					InstanceConfigurationId: common.String("config_id"),
					FreeformTags:            tags,
					PlacementConfigurations: []core.UpdateInstancePoolPlacementConfigurationDetails{
						{
							AvailabilityDomain: common.String("ad-2"),
							PrimarySubnetId:    common.String("subnet-id"),
							FaultDomains:       []string{"fd-7"},
						},
					},
				}))).
					Return(core.UpdateInstancePoolResponse{
						InstancePool: core.InstancePool{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance pool primary VNIC subnet placement update",
			errorExpected: false,
			instancepool: &core.InstancePool{
				Size:                    common.Int(3),
				InstanceConfigurationId: common.String("config_id"),
				PlacementConfigurations: []core.InstancePoolPlacementConfiguration{
					{
						AvailabilityDomain: common.String("ad-1"),
						PrimarySubnetId:    common.String("subnet-id"),
						FaultDomains:       []string{"fd-5", "fd-6"},
					},
				},
			},
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
				ms.OCIMachinePool.Spec.PlacementDetails = []infrav2exp.PlacementDetails{
					{
						AvailabilityDomain: 1,
						FaultDomains:       []string{"fd-5", "fd-6"},
						PrimaryVnicSubnets: &infrav2exp.InstancePoolPlacementPrimarySubnet{
							SubnetId:       common.String("primary-subnet-id"),
							IsAssignIpv6Ip: common.Bool(true),
						},
					},
				}
				computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(expectedUpdateInstancePoolRequest(ms, &core.InstancePool{
					Size:                    common.Int(3),
					InstanceConfigurationId: common.String("config_id"),
					PlacementConfigurations: []core.InstancePoolPlacementConfiguration{
						{
							AvailabilityDomain: common.String("ad-1"),
							PrimarySubnetId:    common.String("subnet-id"),
							FaultDomains:       []string{"fd-5", "fd-6"},
						},
					},
				}, core.UpdateInstancePoolDetails{
					Size:                    common.Int(3),
					InstanceConfigurationId: common.String("config_id"),
					FreeformTags:            tags,
					PlacementConfigurations: []core.UpdateInstancePoolPlacementConfigurationDetails{
						{
							AvailabilityDomain: common.String("ad-1"),
							FaultDomains:       []string{"fd-5", "fd-6"},
							PrimaryVnicSubnets: &core.InstancePoolPlacementPrimarySubnet{
								SubnetId:       common.String("primary-subnet-id"),
								IsAssignIpv6Ip: common.Bool(true),
							},
						},
					},
				}))).
					Return(core.UpdateInstancePoolResponse{
						InstancePool: core.InstancePool{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance pool formatter removal update",
			errorExpected: false,
			instancepool: &core.InstancePool{
				Size:                         common.Int(3),
				InstanceConfigurationId:      common.String("config_id"),
				InstanceDisplayNameFormatter: common.String("old-display-${launchCount}"),
				InstanceHostnameFormatter:    common.String("old-host-${launchCount}"),
			},
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
				computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(expectedUpdateInstancePoolRequest(ms, &core.InstancePool{
					Size:                         common.Int(3),
					InstanceConfigurationId:      common.String("config_id"),
					InstanceDisplayNameFormatter: common.String("old-display-${launchCount}"),
					InstanceHostnameFormatter:    common.String("old-host-${launchCount}"),
				}, core.UpdateInstancePoolDetails{
					Size:                         common.Int(3),
					InstanceConfigurationId:      common.String("config_id"),
					InstanceDisplayNameFormatter: common.String(""),
					InstanceHostnameFormatter:    common.String(""),
					FreeformTags: map[string]string{
						ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
						ociutil.ClusterResourceIdentifier: "resource_uid",
					},
				}))).
					Return(core.UpdateInstancePoolResponse{
						InstancePool: core.InstancePool{
							Id: common.String("id"),
						},
					}, nil)
			},
		},
		{
			name:          "instance pool formatter update omits size when replicas are externally managed",
			errorExpected: false,
			instancepool: &core.InstancePool{
				Size:                         common.Int(5),
				InstanceConfigurationId:      common.String("config_id"),
				InstanceDisplayNameFormatter: common.String("old-display-${launchCount}"),
				InstanceHostnameFormatter:    common.String("old-host-${launchCount}"),
			},
			testSpecificSetup: func(ms *MachinePoolScope) {
				ms.MachinePool.Annotations = map[string]string{
					clusterv1.ReplicasManagedByAnnotation: "",
				}
				ms.OCIMachinePool.Spec.InstanceConfiguration.InstanceConfigurationId = common.String("config_id")
				ms.OCIMachinePool.Spec.InstanceDisplayNameFormatter = common.String("new-display-${launchCount}")
				ms.OCIMachinePool.Spec.InstanceHostnameFormatter = common.String("new-host-${launchCount}")
				computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(expectedUpdateInstancePoolRequest(ms, &core.InstancePool{
					Size:                         common.Int(5),
					InstanceConfigurationId:      common.String("config_id"),
					InstanceDisplayNameFormatter: common.String("old-display-${launchCount}"),
					InstanceHostnameFormatter:    common.String("old-host-${launchCount}"),
				}, core.UpdateInstancePoolDetails{
					InstanceConfigurationId: common.String("config_id"),
					FreeformTags: map[string]string{
						ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
						ociutil.ClusterResourceIdentifier: "resource_uid",
					},
					InstanceDisplayNameFormatter: common.String("new-display-${launchCount}"),
					InstanceHostnameFormatter:    common.String("new-host-${launchCount}"),
				}))).
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
				PlacementConfigurations: matchingPlacementConfigurations,
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
			if tc.instancepool.LifecycleState == "" {
				tc.instancepool.LifecycleState = core.InstancePoolLifecycleStateRunning
			}
			_, err := ms.UpdatePool(context.Background(), tc.instancepool)
			if tc.errorExpected {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestInstancePoolUpdateSkipsDefaultPlacementDrift(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	replicas := int32(3)
	infraMachinePool := &infrav2exp.OCIMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: infrav2exp.OCIMachinePoolSpec{
			InstanceConfiguration: infrav2exp.InstanceConfiguration{
				InstanceConfigurationId: common.String("config_id"),
			},
		},
	}
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: mock_computemanagement.NewMockClient(mockCtrl),
		OCIMachinePool:          infraMachinePool,
		OCIClusterAccessor: OCISelfManagedCluster{
			OCICluster: &infrastructurev1beta2.OCICluster{},
		},
		Cluster: &clusterv1.Cluster{},
		MachinePool: &clusterv1.MachinePool{
			Spec: clusterv1.MachinePoolSpec{
				Replicas: &replicas,
			},
		},
		Client: fake.NewClientBuilder().WithStatusSubresource(infraMachinePool).WithObjects(infraMachinePool).Build(),
	})
	g.Expect(err).To(BeNil())

	instancePool := &core.InstancePool{
		Size:                    common.Int(3),
		InstanceConfigurationId: common.String("config_id"),
	}

	updateOutcome, err := ms.UpdatePool(context.Background(), instancePool)
	g.Expect(err).To(BeNil())
	g.Expect(updateOutcome).To(Equal(InstancePoolUpdateNoChange))
}

func TestInstancePoolUpdateClearsStalePrimaryVnicSubnetPlacement(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	replicas := int32(3)
	computeManagementClient := mock_computemanagement.NewMockClient(mockCtrl)
	infraMachinePool := &infrav2exp.OCIMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: infrav2exp.OCIMachinePoolSpec{
			InstanceConfiguration: infrav2exp.InstanceConfiguration{
				InstanceConfigurationId: common.String("config_id"),
			},
		},
	}
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: computeManagementClient,
		OCIMachinePool:          infraMachinePool,
		OCIClusterAccessor: OCISelfManagedCluster{
			OCICluster: &infrastructurev1beta2.OCICluster{
				Spec: infrastructurev1beta2.OCIClusterSpec{
					NetworkSpec: infrastructurev1beta2.NetworkSpec{
						Vcn: infrastructurev1beta2.VCN{
							Subnets: []*infrastructurev1beta2.Subnet{
								{
									Role: infrastructurev1beta2.WorkerRole,
									ID:   common.String("new-subnet-id"),
								},
							},
						},
					},
					AvailabilityDomains: map[string]infrastructurev1beta2.OCIAvailabilityDomain{
						"ad-1": {
							Name:         "ad-1",
							FaultDomains: []string{"fd-5"},
						},
					},
				},
			},
		},
		Cluster: &clusterv1.Cluster{},
		MachinePool: &clusterv1.MachinePool{
			Spec: clusterv1.MachinePoolSpec{
				Replicas: &replicas,
			},
		},
		Client: fake.NewClientBuilder().WithStatusSubresource(infraMachinePool).WithObjects(infraMachinePool).Build(),
	})
	g.Expect(err).To(BeNil())

	instancePool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		Size:                    common.Int(3),
		InstanceConfigurationId: common.String("config_id"),
		PlacementConfigurations: []core.InstancePoolPlacementConfiguration{
			{
				AvailabilityDomain: common.String("ad-1"),
				FaultDomains:       []string{"fd-5"},
				PrimaryVnicSubnets: &core.InstancePoolPlacementPrimarySubnet{
					SubnetId:       common.String("old-subnet-id"),
					IsAssignIpv6Ip: common.Bool(true),
				},
			},
		},
	}
	desiredUpdate := core.UpdateInstancePoolDetails{
		Size:                    common.Int(3),
		InstanceConfigurationId: common.String("config_id"),
		FreeformTags: map[string]string{
			ociutil.ClusterResourceIdentifier: "",
			ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
		},
		PlacementConfigurations: []core.UpdateInstancePoolPlacementConfigurationDetails{
			{
				AvailabilityDomain: common.String("ad-1"),
				PrimarySubnetId:    common.String("new-subnet-id"),
				FaultDomains:       []string{"fd-5"},
			},
		},
	}
	computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(expectedUpdateInstancePoolRequest(ms, instancePool, desiredUpdate))).Return(core.UpdateInstancePoolResponse{
		InstancePool: core.InstancePool{
			Id: common.String("pool-id"),
		},
	}, nil)

	_, err = ms.UpdatePool(context.Background(), instancePool)
	g.Expect(err).To(BeNil())

	instancePool.PlacementConfigurations = []core.InstancePoolPlacementConfiguration{
		{
			AvailabilityDomain: common.String("ad-1"),
			PrimarySubnetId:    common.String("old-subnet-id"),
			FaultDomains:       []string{"old-fd"},
		},
	}
	computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(expectedUpdateInstancePoolRequest(ms, instancePool, desiredUpdate))).Return(core.UpdateInstancePoolResponse{}, nil)
	_, err = ms.UpdatePool(context.Background(), instancePool)
	g.Expect(err).To(BeNil())
}

func TestInstancePoolUpdateAttemptRetriesSameRequestAfterRestart(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tags := map[string]string{
		ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
		ociutil.ClusterResourceIdentifier: "resource_uid",
	}
	details := core.UpdateInstancePoolDetails{
		Size:                    common.Int(2),
		InstanceConfigurationId: common.String("config-id"),
		FreeformTags:            tags,
	}
	attempt, err := newInstancePoolUpdateAttempt(details, common.String("etag-before-update"))
	g.Expect(err).To(BeNil())
	attempt.RetryToken = "stable-retry-token"

	infraMachinePool := &infrav2exp.OCIMachinePool{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}
	setTestInstancePoolUpdateAttempt(t, infraMachinePool, attempt)
	client := fake.NewClientBuilder().WithStatusSubresource(infraMachinePool).WithObjects(infraMachinePool).Build()
	computeManagementClient := mock_computemanagement.NewMockClient(mockCtrl)
	replicas := int32(2)
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: computeManagementClient,
		OCIMachinePool:          infraMachinePool,
		OCIClusterAccessor: OCISelfManagedCluster{OCICluster: &infrastructurev1beta2.OCICluster{
			Spec: infrastructurev1beta2.OCIClusterSpec{OCIResourceIdentifier: "resource_uid"},
		}},
		Cluster:     &clusterv1.Cluster{},
		MachinePool: &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: &replicas}},
		Client:      client,
	})
	g.Expect(err).To(BeNil())

	instancePool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		Size:                    common.Int(3),
		InstanceConfigurationId: common.String("config-id"),
		FreeformTags:            tags,
		LifecycleState:          core.InstancePoolLifecycleStateRunning,
	}
	computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Eq(core.UpdateInstancePoolRequest{
		InstancePoolId:            common.String("pool-id"),
		UpdateInstancePoolDetails: attempt.Target.updateDetails(),
		OpcRetryToken:             common.String("stable-retry-token"),
		IfMatch:                   common.String("etag-before-update"),
	})).Return(core.UpdateInstancePoolResponse{Etag: common.String("etag-after-update")}, nil)

	outcome, err := ms.UpdatePool(context.Background(), instancePool)
	g.Expect(err).To(BeNil())
	g.Expect(outcome).To(Equal(InstancePoolUpdateSubmitted))

	reloaded := &infrav2exp.OCIMachinePool{}
	g.Expect(client.Get(context.Background(), types.NamespacedName{Name: "test", Namespace: "default"}, reloaded)).To(Succeed())
	persisted := &instancePoolUpdateAttempt{}
	g.Expect(json.Unmarshal([]byte(reloaded.Annotations[InstancePoolUpdateAttemptAnnotation]), persisted)).To(Succeed())
	g.Expect(persisted.Version).To(Equal(instancePoolUpdateAttemptSchemaVersion))
	g.Expect(persisted.RetryToken).To(Equal("stable-retry-token"))
	g.Expect(persisted.Phase).To(Equal(instancePoolUpdatePhaseSubmitted))
}

func TestInstancePoolUpdateAttemptRecoversFromStaleETag(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tags := map[string]string{
		ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
		ociutil.ClusterResourceIdentifier: "resource_uid",
	}
	infraMachinePool := &infrav2exp.OCIMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: infrav2exp.OCIMachinePoolSpec{
			InstanceConfiguration: infrav2exp.InstanceConfiguration{InstanceConfigurationId: common.String("config-id")},
		},
	}
	client := fake.NewClientBuilder().WithStatusSubresource(infraMachinePool).WithObjects(infraMachinePool).Build()
	computeManagementClient := mock_computemanagement.NewMockClient(mockCtrl)
	replicas := int32(2)
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: computeManagementClient,
		InstancePoolETag:        common.String("etag-1"),
		OCIMachinePool:          infraMachinePool,
		OCIClusterAccessor: OCISelfManagedCluster{OCICluster: &infrastructurev1beta2.OCICluster{
			Spec: infrastructurev1beta2.OCIClusterSpec{OCIResourceIdentifier: "resource_uid"},
		}},
		Cluster:     &clusterv1.Cluster{},
		MachinePool: &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: &replicas}},
		Client:      client,
	})
	g.Expect(err).To(BeNil())

	instancePool := &core.InstancePool{
		Id:                      common.String("pool-id"),
		Size:                    common.Int(3),
		InstanceConfigurationId: common.String("config-id"),
		FreeformTags:            tags,
		LifecycleState:          core.InstancePoolLifecycleStateRunning,
	}
	firstToken := ""
	computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
			g.Expect(request.IfMatch).To(Equal(common.String("etag-1")))
			g.Expect(request.OpcRetryToken).ToNot(BeNil())
			firstToken = *request.OpcRetryToken
			return core.UpdateInstancePoolResponse{}, testServiceError{status: http.StatusPreconditionFailed}
		},
	)

	outcome, err := ms.UpdatePool(context.Background(), instancePool)
	g.Expect(err).To(BeNil())
	g.Expect(outcome).To(Equal(InstancePoolUpdateRetryRequired))
	g.Expect(ms.HasPendingInstancePoolUpdate()).To(BeFalse())
	reloadedAfterRejection := &infrav2exp.OCIMachinePool{}
	g.Expect(client.Get(context.Background(), types.NamespacedName{Name: "test", Namespace: "default"}, reloadedAfterRejection)).To(Succeed())
	g.Expect(reloadedAfterRejection.Annotations).ToNot(HaveKey(InstancePoolUpdateAttemptAnnotation))

	ms.InstancePoolETag = common.String("etag-2")
	computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
			g.Expect(request.IfMatch).To(Equal(common.String("etag-2")))
			g.Expect(request.OpcRetryToken).ToNot(BeNil())
			g.Expect(*request.OpcRetryToken).ToNot(Equal(firstToken))
			return core.UpdateInstancePoolResponse{}, nil
		},
	)

	outcome, err = ms.UpdatePool(context.Background(), instancePool)
	g.Expect(err).To(BeNil())
	g.Expect(outcome).To(Equal(InstancePoolUpdateSubmitted))
	g.Expect(ms.HasPendingInstancePoolUpdate()).To(BeTrue())
}

func TestInstancePoolUpdateAttemptClearDoesNotDeleteNewerAttempt(t *testing.T) {
	g := NewWithT(t)

	firstAttempt, err := newInstancePoolUpdateAttempt(core.UpdateInstancePoolDetails{Size: common.Int(2)}, nil)
	g.Expect(err).To(BeNil())
	firstAttempt.RetryToken = "first-token"
	infraMachinePool := &infrav2exp.OCIMachinePool{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}
	setTestInstancePoolUpdateAttempt(t, infraMachinePool, firstAttempt)
	client := fake.NewClientBuilder().WithStatusSubresource(infraMachinePool).WithObjects(infraMachinePool).Build()

	newScope := func(machinePool *infrav2exp.OCIMachinePool) *MachinePoolScope {
		scope, scopeErr := NewMachinePoolScope(MachinePoolScopeParams{
			ComputeManagementClient: mock_computemanagement.NewMockClient(gomock.NewController(t)),
			OCIMachinePool:          machinePool,
			OCIClusterAccessor:      OCISelfManagedCluster{OCICluster: &infrastructurev1beta2.OCICluster{}},
			Cluster:                 &clusterv1.Cluster{},
			MachinePool:             &clusterv1.MachinePool{},
			Client:                  client,
		})
		g.Expect(scopeErr).To(BeNil())
		return scope
	}

	winner := newScope(infraMachinePool.DeepCopy())
	loser := newScope(infraMachinePool.DeepCopy())
	g.Expect(winner.clearInstancePoolUpdateAttempt(context.Background())).To(Succeed())

	newerAttempt, err := newInstancePoolUpdateAttempt(core.UpdateInstancePoolDetails{Size: common.Int(3)}, nil)
	g.Expect(err).To(BeNil())
	newerAttempt.RetryToken = "newer-token"
	g.Expect(winner.setInstancePoolUpdateAttempt(context.Background(), newerAttempt)).To(Succeed())
	g.Expect(loser.clearInstancePoolUpdateAttempt(context.Background())).To(MatchError(ContainSubstring("changed concurrently")))

	reloaded := &infrav2exp.OCIMachinePool{}
	g.Expect(client.Get(context.Background(), types.NamespacedName{Name: "test", Namespace: "default"}, reloaded)).To(Succeed())
	persisted := &instancePoolUpdateAttempt{}
	g.Expect(json.Unmarshal([]byte(reloaded.Annotations[InstancePoolUpdateAttemptAnnotation]), persisted)).To(Succeed())
	g.Expect(persisted.RetryToken).To(Equal("newer-token"))
}

func TestInstancePoolUpdateAttemptSerializesChangedDesiredState(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tags := map[string]string{
		ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
		ociutil.ClusterResourceIdentifier: "resource_uid",
	}
	oldDetails := core.UpdateInstancePoolDetails{
		Size:                    common.Int(2),
		InstanceConfigurationId: common.String("config-id"),
		FreeformTags:            tags,
	}
	attempt, err := newInstancePoolUpdateAttempt(oldDetails, common.String("etag-before-old-update"))
	g.Expect(err).To(BeNil())
	attempt.RetryToken = "old-retry-token"
	attempt.Phase = instancePoolUpdatePhaseSubmitted

	infraMachinePool := &infrav2exp.OCIMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: infrav2exp.OCIMachinePoolSpec{
			InstanceConfiguration: infrav2exp.InstanceConfiguration{InstanceConfigurationId: common.String("config-id")},
		},
	}
	setTestInstancePoolUpdateAttempt(t, infraMachinePool, attempt)
	client := fake.NewClientBuilder().WithStatusSubresource(infraMachinePool).WithObjects(infraMachinePool).Build()
	computeManagementClient := mock_computemanagement.NewMockClient(mockCtrl)
	replicas := int32(3)
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: computeManagementClient,
		OCIMachinePool:          infraMachinePool,
		OCIClusterAccessor: OCISelfManagedCluster{OCICluster: &infrastructurev1beta2.OCICluster{
			Spec: infrastructurev1beta2.OCIClusterSpec{OCIResourceIdentifier: "resource_uid"},
		}},
		Cluster:     &clusterv1.Cluster{},
		MachinePool: &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: &replicas}},
		Client:      client,
	})
	g.Expect(err).To(BeNil())

	staleReadback := &core.InstancePool{
		Id:                      common.String("pool-id"),
		Size:                    common.Int(3),
		InstanceConfigurationId: common.String("config-id"),
		FreeformTags:            tags,
		LifecycleState:          core.InstancePoolLifecycleStateRunning,
	}
	outcome, err := ms.UpdatePool(context.Background(), staleReadback)
	g.Expect(err).To(BeNil())
	g.Expect(outcome).To(Equal(InstancePoolUpdateWaiting))
	g.Expect(ms.HasPendingInstancePoolUpdate()).To(BeTrue())
	g.Expect(ms.CleanupInstanceConfiguration(context.Background(), staleReadback)).To(Succeed())

	oldTargetReadback := staleReadback
	oldTargetReadback.Size = common.Int(2)
	ms.InstancePoolETag = common.String("etag-after-old-update")
	computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
			g.Expect(request.UpdateInstancePoolDetails.Size).To(Equal(common.Int(3)))
			g.Expect(request.OpcRetryToken).ToNot(BeNil())
			g.Expect(*request.OpcRetryToken).ToNot(Equal("old-retry-token"))
			g.Expect(request.IfMatch).To(Equal(common.String("etag-after-old-update")))
			return core.UpdateInstancePoolResponse{}, nil
		},
	)

	outcome, err = ms.UpdatePool(context.Background(), oldTargetReadback)
	g.Expect(err).To(BeNil())
	g.Expect(outcome).To(Equal(InstancePoolUpdateSubmitted))
	g.Expect(ms.HasPendingInstancePoolUpdate()).To(BeTrue())
}

func TestInstancePoolUpdateAttemptTimesOutWithoutRotatingToken(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	details := core.UpdateInstancePoolDetails{
		Size:                    common.Int(2),
		InstanceConfigurationId: common.String("config-id"),
		FreeformTags:            map[string]string{},
	}
	attempt, err := newInstancePoolUpdateAttempt(details, nil)
	g.Expect(err).To(BeNil())
	attempt.RetryToken = "expired-but-not-rotated"
	attempt.Phase = instancePoolUpdatePhaseSubmitted
	attempt.StartedAt = time.Now().Add(-instancePoolUpdateAttemptTimeout - time.Minute)
	infraMachinePool := &infrav2exp.OCIMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: infrav2exp.OCIMachinePoolSpec{
			InstanceConfiguration: infrav2exp.InstanceConfiguration{InstanceConfigurationId: common.String("config-id")},
		},
	}
	setTestInstancePoolUpdateAttempt(t, infraMachinePool, attempt)
	client := fake.NewClientBuilder().WithStatusSubresource(infraMachinePool).WithObjects(infraMachinePool).Build()
	replicas := int32(3)
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: mock_computemanagement.NewMockClient(mockCtrl),
		OCIMachinePool:          infraMachinePool,
		OCIClusterAccessor:      OCISelfManagedCluster{OCICluster: &infrastructurev1beta2.OCICluster{}},
		Cluster:                 &clusterv1.Cluster{},
		MachinePool:             &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: &replicas}},
		Client:                  client,
	})
	g.Expect(err).To(BeNil())

	outcome, err := ms.UpdatePool(context.Background(), &core.InstancePool{
		Id:                      common.String("pool-id"),
		Size:                    common.Int(3),
		InstanceConfigurationId: common.String("config-id"),
		FreeformTags:            map[string]string{},
		LifecycleState:          core.InstancePoolLifecycleStateScaling,
	})
	g.Expect(outcome).To(Equal(InstancePoolUpdateWaiting))
	g.Expect(err).To(MatchError(ContainSubstring("has not converged")))
	persisted, err := ms.getInstancePoolUpdateAttempt()
	g.Expect(err).To(BeNil())
	g.Expect(persisted.RetryToken).To(Equal("expired-but-not-rotated"))
}

func TestInstancePoolUpdateAttemptTimeoutStartsWhenSubmitted(t *testing.T) {
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	details := core.UpdateInstancePoolDetails{
		Size:                    common.Int(2),
		InstanceConfigurationId: common.String("config-id"),
		FreeformTags:            map[string]string{},
	}
	attempt, err := newInstancePoolUpdateAttempt(details, nil)
	g.Expect(err).To(BeNil())
	attempt.StartedAt = time.Now().Add(-instancePoolUpdateAttemptTimeout - time.Minute)
	infraMachinePool := &infrav2exp.OCIMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: infrav2exp.OCIMachinePoolSpec{
			InstanceConfiguration: infrav2exp.InstanceConfiguration{InstanceConfigurationId: common.String("config-id")},
		},
	}
	setTestInstancePoolUpdateAttempt(t, infraMachinePool, attempt)
	client := fake.NewClientBuilder().WithStatusSubresource(infraMachinePool).WithObjects(infraMachinePool).Build()
	computeManagementClient := mock_computemanagement.NewMockClient(mockCtrl)
	computeManagementClient.EXPECT().UpdateInstancePool(gomock.Any(), gomock.Any()).Return(core.UpdateInstancePoolResponse{}, nil)
	replicas := int32(3)
	ms, err := NewMachinePoolScope(MachinePoolScopeParams{
		ComputeManagementClient: computeManagementClient,
		OCIMachinePool:          infraMachinePool,
		OCIClusterAccessor:      OCISelfManagedCluster{OCICluster: &infrastructurev1beta2.OCICluster{}},
		Cluster:                 &clusterv1.Cluster{},
		MachinePool:             &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: &replicas}},
		Client:                  client,
	})
	g.Expect(err).To(BeNil())

	beforeSubmission := time.Now().UTC()
	outcome, err := ms.UpdatePool(context.Background(), &core.InstancePool{
		Id:                      common.String("pool-id"),
		Size:                    common.Int(3),
		InstanceConfigurationId: common.String("config-id"),
		FreeformTags:            map[string]string{},
		LifecycleState:          core.InstancePoolLifecycleStateRunning,
	})
	g.Expect(err).To(BeNil())
	g.Expect(outcome).To(Equal(InstancePoolUpdateSubmitted))
	persisted, err := ms.getInstancePoolUpdateAttempt()
	g.Expect(err).To(BeNil())
	g.Expect(persisted.StartedAt).To(BeTemporally(">=", beforeSubmission))
}

func TestStringMapsEqualDistinguishesMissingEmptyValues(t *testing.T) {
	g := NewWithT(t)
	g.Expect(stringMapsEqual(map[string]string{"left": ""}, map[string]string{"right": ""})).To(BeFalse())
	g.Expect(stringMapsEqual(map[string]string{"same": ""}, map[string]string{"same": ""})).To(BeTrue())
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
