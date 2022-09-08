/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

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

	"github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer/mock_nlb"
	"github.com/oracle/oci-go-sdk/v63/networkloadbalancer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute/mock_compute"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInstanceReconciliation(t *testing.T) {
	var (
		ms            *MachineScope
		mockCtrl      *gomock.Controller
		computeClient *mock_compute.MockComputeClient
		ociCluster    infrastructurev1beta1.OCICluster
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"value": []byte("test"),
			},
		}

		mockCtrl = gomock.NewController(t)
		computeClient = mock_compute.NewMockComputeClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects(secret).Build()
		ociCluster = infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
			Spec: infrastructurev1beta1.OCIClusterSpec{
				OCIResourceIdentifier: "resource_uid",
			},
		}
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ms, err = NewMachineScope(MachineScopeParams{
			ComputeClient: computeClient,
			OCIMachine: &infrastructurev1beta1.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: infrastructurev1beta1.OCIMachineSpec{
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
			Cluster:    &clusterv1.Cluster{},
			OCICluster: &ociCluster,
			Client:     client,
		})
		ms.Machine.Namespace = "default"
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
		testSpecificSetup   func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient)
	}{
		{
			name:          "machine exists",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = common.String("test")
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{
						Instance: core.Instance{
							Id: common.String("test"),
						},
					}, nil)
			},
		},
		{
			name:          "machine get call errors out",
			errorExpected: true,
			matchError:    fmt.Errorf("internal server error"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = common.String("test")
				error := fmt.Errorf("internal server error")
				computeClient.EXPECT().GetInstance(gomock.Any(), gomock.Eq(core.GetInstanceRequest{
					InstanceId: common.String("test"),
				})).
					Return(core.GetInstanceResponse{}, error)
			},
		},
		{
			name:          "use list instances",
			errorExpected: false,
			matchError:    fmt.Errorf("internal server error"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("test"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{
					Items: []core.Instance{
						{
							Id: common.String("test"),
							FreeformTags: map[string]string{
								ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
								ociutil.ClusterResourceIdentifier: "resource_uid",
							},
						},
					},
				}, nil)
			},
		},
		{
			name:          "no bootstrap data",
			errorExpected: true,
			matchError:    errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = nil
				ms.OCIMachine.Name = "test"
				ms.Machine.Spec.Bootstrap.DataSecretName = nil
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("test"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
			},
		},
		{
			name:          "invalid ocpu",
			errorExpected: true,
			matchError:    errors.New(fmt.Sprintf("ocpus provided %s is not a valid floating point", "invalid")),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.ShapeConfig.Ocpus = "invalid"
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("test"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
			},
		},
		{
			name:          "invalid MemoryInGBs",
			errorExpected: true,
			matchError:    errors.New(fmt.Sprintf("memoryInGBs provided %s is not a valid floating point", "invalid")),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.ShapeConfig.MemoryInGBs = "invalid"
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("test"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
			},
		},
		{
			name:          "invalid BaselineOcpuUtilization",
			errorExpected: true,
			matchError:    errors.New("invalid baseline cpu optimization parameter"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.ShapeConfig.BaselineOcpuUtilization = "invalid"
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("test"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
			},
		},
		{
			name:          "invalid BootVolumeSizeInGBs",
			errorExpected: true,
			matchError: errors.New(fmt.Sprintf("bootVolumeSizeInGBs provided %s is not a valid floating point",
				"invalid")),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.BootVolumeSizeInGBs = "invalid"
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("test"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
			},
		},
		{
			name:          "invalid BootVolumeSizeInGBs",
			errorExpected: true,
			matchError: errors.New(fmt.Sprintf("bootVolumeSizeInGBs provided %s is not a valid floating point",
				"invalid")),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = nil
				ms.OCIMachine.Name = "test"
				ms.OCIMachine.Spec.BootVolumeSizeInGBs = "invalid"
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("test"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
			},
		},
		{
			name:                "invalid Failure Domain - 1",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("invalid failure domain parameter, must be a valid integer"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.Machine.Spec.FailureDomain = common.String("invalid")
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("test"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
			},
		},
		{
			name:          "invalid Failure Domain - 2",
			errorExpected: true,
			matchError:    errors.New("failure domain should be a value between 1 and 3"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.Machine.Spec.FailureDomain = common.String("4")
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("test"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
			},
		},
		{
			name:          "check displayname",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return instanceDisplayNameMatcher(request, "name")
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "check compartment at cluster",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCICluster.Spec.CompartmentId = "clustercompartment"
				ms.OCIMachine.Spec.CompartmentId = ""

				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("clustercompartment"),
				})).Return(core.ListInstancesResponse{}, nil)

				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return instanceCompartmentIDMatcher(request, "clustercompartment")
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "check compartment at cluster and machine",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCICluster.Spec.CompartmentId = "clustercompartment"

				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return instanceCompartmentIDMatcher(request, "test")
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "check all params together",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("nodesubnet"),
						AssignPublicIp: common.Bool(false),
					},
					Metadata: map[string]string{
						"user_data": base64.StdEncoding.EncodeToString([]byte("test")),
					},
					Shape: common.String("shape"),
					ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
						Ocpus:                   common.Float32(2),
						MemoryInGBs:             common.Float32(100),
						BaselineOcpuUtilization: core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization8,
					},
					AvailabilityDomain:             common.String("ad2"),
					CompartmentId:                  common.String("test"),
					IsPvEncryptionInTransitEnabled: common.Bool(true),
					DefinedTags:                    map[string]map[string]interface{}{},
					FreeformTags: map[string]string{
						ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
						ociutil.ClusterResourceIdentifier: "resource_uid",
					},
				}
				computeClient.EXPECT().LaunchInstance(gomock.Any(), gomock.Eq(core.LaunchInstanceRequest{
					LaunchInstanceDetails: launchDetails,
					OpcRetryToken:         ociutil.GetOPCRetryToken("machineuid")})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "shape config is empty",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.ShapeConfig = infrastructurev1beta1.ShapeConfig{}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("nodesubnet"),
						AssignPublicIp: common.Bool(false),
					},
					Metadata: map[string]string{
						"user_data": base64.StdEncoding.EncodeToString([]byte("test")),
					},
					Shape:                          common.String("shape"),
					AvailabilityDomain:             common.String("ad2"),
					CompartmentId:                  common.String("test"),
					IsPvEncryptionInTransitEnabled: common.Bool(true),
					DefinedTags:                    map[string]map[string]interface{}{},
					FreeformTags: map[string]string{
						ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
						ociutil.ClusterResourceIdentifier: "resource_uid",
					},
				}
				computeClient.EXPECT().LaunchInstance(gomock.Any(), gomock.Eq(core.LaunchInstanceRequest{
					LaunchInstanceDetails: launchDetails,
					OpcRetryToken:         ociutil.GetOPCRetryToken("machineuid")})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "multiple subnets - use default",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCICluster.Spec.NetworkSpec.Vcn.Subnets = append(ms.OCICluster.Spec.NetworkSpec.Vcn.Subnets, &infrastructurev1beta1.Subnet{
					Role: infrastructurev1beta1.WorkerRole,
					ID:   common.String("test-subnet-1"),
				})
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("nodesubnet"),
						AssignPublicIp: common.Bool(false),
					},
					Metadata: map[string]string{
						"user_data": base64.StdEncoding.EncodeToString([]byte("test")),
					},
					Shape: common.String("shape"),
					ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
						Ocpus:                   common.Float32(2),
						MemoryInGBs:             common.Float32(100),
						BaselineOcpuUtilization: core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization8,
					},
					AvailabilityDomain:             common.String("ad2"),
					CompartmentId:                  common.String("test"),
					IsPvEncryptionInTransitEnabled: common.Bool(true),
					DefinedTags:                    map[string]map[string]interface{}{},
					FreeformTags: map[string]string{
						ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
						ociutil.ClusterResourceIdentifier: "resource_uid",
					},
				}
				computeClient.EXPECT().LaunchInstance(gomock.Any(), gomock.Eq(core.LaunchInstanceRequest{
					LaunchInstanceDetails: launchDetails,
					OpcRetryToken:         ociutil.GetOPCRetryToken("machineuid")})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "multiple subnets - use provided",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCICluster.Spec.NetworkSpec.Vcn.Subnets = append(ms.OCICluster.Spec.NetworkSpec.Vcn.Subnets, &infrastructurev1beta1.Subnet{
					Role: infrastructurev1beta1.WorkerRole,
					Name: "test-subnet-name",
					ID:   common.String("test-subnet-1"),
				})
				ms.OCIMachine.Spec.SubnetName = "test-subnet-name"
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("test-subnet-1"),
						AssignPublicIp: common.Bool(false),
					},
					Metadata: map[string]string{
						"user_data": base64.StdEncoding.EncodeToString([]byte("test")),
					},
					Shape: common.String("shape"),
					ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
						Ocpus:                   common.Float32(2),
						MemoryInGBs:             common.Float32(100),
						BaselineOcpuUtilization: core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization8,
					},
					AvailabilityDomain:             common.String("ad2"),
					CompartmentId:                  common.String("test"),
					IsPvEncryptionInTransitEnabled: common.Bool(true),
					DefinedTags:                    map[string]map[string]interface{}{},
					FreeformTags: map[string]string{
						ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
						ociutil.ClusterResourceIdentifier: "resource_uid",
					},
				}
				computeClient.EXPECT().LaunchInstance(gomock.Any(), gomock.Eq(core.LaunchInstanceRequest{
					LaunchInstanceDetails: launchDetails,
					OpcRetryToken:         ociutil.GetOPCRetryToken("machineuid")})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "multiple NSG - use default",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups = append(ms.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups, &infrastructurev1beta1.NSG{
					Role: infrastructurev1beta1.WorkerRole,
					ID:   common.String("test-nsg-1"),
					Name: "test-nsg",
				}, &infrastructurev1beta1.NSG{
					Role: infrastructurev1beta1.WorkerRole,
					ID:   common.String("test-nsg-2"),
					Name: "test-nsg-2",
				})
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("nodesubnet"),
						AssignPublicIp: common.Bool(false),
						NsgIds:         []string{"test-nsg-1"},
					},
					Metadata: map[string]string{
						"user_data": base64.StdEncoding.EncodeToString([]byte("test")),
					},
					Shape: common.String("shape"),
					ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
						Ocpus:                   common.Float32(2),
						MemoryInGBs:             common.Float32(100),
						BaselineOcpuUtilization: core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization8,
					},
					AvailabilityDomain:             common.String("ad2"),
					CompartmentId:                  common.String("test"),
					IsPvEncryptionInTransitEnabled: common.Bool(true),
					DefinedTags:                    map[string]map[string]interface{}{},
					FreeformTags: map[string]string{
						ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
						ociutil.ClusterResourceIdentifier: "resource_uid",
					},
				}
				computeClient.EXPECT().LaunchInstance(gomock.Any(), gomock.Eq(core.LaunchInstanceRequest{
					LaunchInstanceDetails: launchDetails,
					OpcRetryToken:         ociutil.GetOPCRetryToken("machineuid")})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "multiple NSG - use provided",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups = append(ms.OCICluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups, &infrastructurev1beta1.NSG{
					Role: infrastructurev1beta1.WorkerRole,
					ID:   common.String("test-nsg-1"),
					Name: "test-nsg",
				}, &infrastructurev1beta1.NSG{
					Role: infrastructurev1beta1.WorkerRole,
					ID:   common.String("test-nsg-2"),
					Name: "test-nsg-name-2",
				})
				ms.OCIMachine.Spec.NSGName = "test-nsg-name-2"
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("nodesubnet"),
						AssignPublicIp: common.Bool(false),
						NsgIds:         []string{"test-nsg-2"},
					},
					Metadata: map[string]string{
						"user_data": base64.StdEncoding.EncodeToString([]byte("test")),
					},
					Shape: common.String("shape"),
					ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
						Ocpus:                   common.Float32(2),
						MemoryInGBs:             common.Float32(100),
						BaselineOcpuUtilization: core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization8,
					},
					AvailabilityDomain:             common.String("ad2"),
					CompartmentId:                  common.String("test"),
					IsPvEncryptionInTransitEnabled: common.Bool(true),
					DefinedTags:                    map[string]map[string]interface{}{},
					FreeformTags: map[string]string{
						ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
						ociutil.ClusterResourceIdentifier: "resource_uid",
					},
				}
				computeClient.EXPECT().LaunchInstance(gomock.Any(), gomock.Eq(core.LaunchInstanceRequest{
					LaunchInstanceDetails: launchDetails,
					OpcRetryToken:         ociutil.GetOPCRetryToken("machineuid")})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, computeClient)
			_, err := ms.GetOrCreateMachine(context.Background())
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

func instanceDisplayNameMatcher(request interface{}, matchStr string) error {
	r, ok := request.(core.LaunchInstanceRequest)
	if !ok {
		return errors.New("expecting LaunchInstanceRequest type")
	}
	if *r.LaunchInstanceDetails.DisplayName != matchStr {
		return errors.New(fmt.Sprintf("expecting DisplayName as %s", matchStr))
	}
	return nil
}

func instanceCompartmentIDMatcher(request interface{}, matchStr string) error {
	r, ok := request.(core.LaunchInstanceRequest)
	if !ok {
		return errors.New("expecting LaunchInstanceRequest type")
	}
	if *r.LaunchInstanceDetails.CompartmentId != matchStr {
		return errors.New(fmt.Sprintf("expecting DisplayName as %s", matchStr))
	}
	return nil
}

func TestLBReconciliationCreation(t *testing.T) {
	var (
		ms         *MachineScope
		mockCtrl   *gomock.Controller
		nlbClient  *mock_nlb.MockNetworkLoadBalancerClient
		ociCluster infrastructurev1beta1.OCICluster
	)
	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects().Build()
		ociCluster = infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
		}
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlbid")
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ms, err = NewMachineScope(MachineScopeParams{
			NetworkLoadBalancerClient: nlbClient,
			OCIMachine: &infrastructurev1beta1.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "uid",
				},
				Spec: infrastructurev1beta1.OCIMachineSpec{
					CompartmentId: "test",
				},
			},
			Machine:    &clusterv1.Machine{},
			Cluster:    &clusterv1.Cluster{},
			OCICluster: &ociCluster,
			Client:     client,
		})
		ms.Machine.Namespace = "default"
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
		testSpecificSetup   func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient)
	}{
		{
			name:          "ip doesnt exist",
			errorExpected: true,
			matchError:    errors.New("could not find machine IP Address in status object"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
			},
		},
		{
			name:          "ip exists",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{},
							},
						},
					},
				}, nil)

				nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					networkloadbalancer.CreateBackendRequest{
						NetworkLoadBalancerId: common.String("nlbid"),
						BackendSetName:        common.String(APIServerLBBackendSetName),
						CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
							Name:      common.String("test"),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(networkloadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					}}, nil)
			},
		},
		{
			name:          "work request exists",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{},
							},
						},
					},
				}, nil)
				machineScope.OCIMachine.Status.CreateBackendWorkRequestId = "wrid"
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					}}, nil)
			},
		},
		{
			name:          "work request exists error",
			errorExpected: true,
			matchError:    errors.Errorf("WorkRequest %s failed", "wrid"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				machineScope.OCIMachine.Status.CreateBackendWorkRequestId = "wrid"
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{},
							},
						},
					},
				}, nil)
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusFailed,
					}}, nil)
			},
		},
		{
			name:          "backend exists",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{
									{
										Name: common.String("test"),
									},
								},
							},
						},
					},
				}, nil)
			},
		},
		{
			name:          "create backend error",
			errorExpected: true,
			matchError:    errors.New("could not create backend"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{},
							},
						},
					},
				}, nil)

				nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					networkloadbalancer.CreateBackendRequest{
						NetworkLoadBalancerId: common.String("nlbid"),
						BackendSetName:        common.String(APIServerLBBackendSetName),
						CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
							Name:      common.String("test"),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(networkloadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, errors.New("could not create backend"))
			},
		},
		{
			name:          "get nlb error",
			errorExpected: true,
			matchError:    errors.New("could not get nlb"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{}}, errors.New("could not get nlb"))
			},
		},
		{
			name:          "work request failed",
			errorExpected: true,
			matchError:    errors.Errorf("WorkRequest %s failed", "wrid"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{},
							},
						},
					},
				}, nil)

				nlbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					networkloadbalancer.CreateBackendRequest{
						NetworkLoadBalancerId: common.String("nlbid"),
						BackendSetName:        common.String(APIServerLBBackendSetName),
						CreateBackendDetails: networkloadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
							Name:      common.String("test"),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(networkloadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusFailed,
					}}, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, nlbClient)
			err := ms.ReconcileCreateInstanceOnLB(context.Background())
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

func TestLBReconciliationDeletion(t *testing.T) {
	var (
		ms         *MachineScope
		mockCtrl   *gomock.Controller
		nlbClient  *mock_nlb.MockNetworkLoadBalancerClient
		ociCluster infrastructurev1beta1.OCICluster
	)
	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects().Build()
		ociCluster = infrastructurev1beta1.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
		}
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlbid")
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ms, err = NewMachineScope(MachineScopeParams{
			NetworkLoadBalancerClient: nlbClient,
			OCIMachine: &infrastructurev1beta1.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "uid",
				},
				Spec: infrastructurev1beta1.OCIMachineSpec{
					CompartmentId: "test",
				},
			},
			Machine:    &clusterv1.Machine{},
			Cluster:    &clusterv1.Cluster{},
			OCICluster: &ociCluster,
			Client:     client,
		})
		ms.Machine.Namespace = "default"
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
		testSpecificSetup   func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient)
	}{
		{
			name:          "get nlb error",
			errorExpected: true,
			matchError:    errors.New("could not get nlb"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{}}, errors.New("could not get nlb"))
			},
		},
		{
			name:          "backend exists",
			errorExpected: false,
			matchError:    errors.New("could not get nlb"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{
									{
										Name: common.String("test"),
									},
								},
							},
						},
					},
				}, nil)
				nlbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Eq(networkloadbalancer.DeleteBackendRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
					BackendSetName:        common.String(APIServerLBBackendSetName),
					BackendName:           common.String("test"),
				})).Return(networkloadbalancer.DeleteBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					}}, nil)
			},
		},
		{
			name:          "backend does not exist",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{},
							},
						},
					},
				}, nil)
			},
		},
		{
			name:          "work request exists",
			errorExpected: false,
			matchError:    errors.New("could not get nlb"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				machineScope.OCIMachine.Status.DeleteBackendWorkRequestId = "wrid"
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{
									{
										Name: common.String("test"),
									},
								},
							},
						},
					},
				}, nil)
				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					}}, nil)
			},
		},
		{
			name:          "work request failed",
			errorExpected: true,
			matchError:    errors.Errorf("WorkRequest %s failed", "wrid"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{
									{
										Name: common.String("test"),
									},
								},
							},
						},
					},
				}, nil)
				nlbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Eq(networkloadbalancer.DeleteBackendRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
					BackendSetName:        common.String(APIServerLBBackendSetName),
					BackendName:           common.String("test"),
				})).Return(networkloadbalancer.DeleteBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusFailed,
					}}, nil)
			},
		},
		{
			name:          "delete backend fails",
			errorExpected: true,
			matchError:    errors.New("backend request failed"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient) {
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{
						BackendSets: map[string]networkloadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []networkloadbalancer.Backend{
									{
										Name: common.String("test"),
									},
								},
							},
						},
					},
				}, nil)
				nlbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Eq(networkloadbalancer.DeleteBackendRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
					BackendSetName:        common.String(APIServerLBBackendSetName),
					BackendName:           common.String("test"),
				})).Return(networkloadbalancer.DeleteBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, errors.New("backend request failed"))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, nlbClient)
			err := ms.ReconcileDeleteInstanceOnLB(context.Background())
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
func TestInstanceDeletion(t *testing.T) {
	var (
		ms            *MachineScope
		mockCtrl      *gomock.Controller
		computeClient *mock_compute.MockComputeClient
		ociCluster    infrastructurev1beta1.OCICluster
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		computeClient = mock_compute.NewMockComputeClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociCluster = infrastructurev1beta1.OCICluster{}
		ms, err = NewMachineScope(MachineScopeParams{
			ComputeClient: computeClient,
			OCIMachine: &infrastructurev1beta1.OCIMachine{
				Spec: infrastructurev1beta1.OCIMachineSpec{
					CompartmentId: "test",
					InstanceId:    common.String("test"),
				},
			},
			Machine:    &clusterv1.Machine{},
			Cluster:    &clusterv1.Cluster{},
			OCICluster: &ociCluster,
			Client:     client,
		})
		ms.Machine.Namespace = "default"
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
		testSpecificSetup   func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient)
	}{
		{
			name:          "delete instance",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = common.String("test")
				computeClient.EXPECT().TerminateInstance(gomock.Any(), gomock.Eq(core.TerminateInstanceRequest{
					InstanceId:         common.String("test"),
					PreserveBootVolume: common.Bool(false),
				})).Return(core.TerminateInstanceResponse{}, nil)
			},
		},
		{
			name:          "delete instance error",
			errorExpected: true,
			matchError:    errors.New("could not terminate instance"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = common.String("test")
				computeClient.EXPECT().TerminateInstance(gomock.Any(), gomock.Eq(core.TerminateInstanceRequest{
					InstanceId:         common.String("test"),
					PreserveBootVolume: common.Bool(false),
				})).Return(core.TerminateInstanceResponse{}, errors.New("could not terminate instance"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, computeClient)
			err := ms.DeleteMachine(context.Background())
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

func setupAllParams(ms *MachineScope) {
	ms.OCIMachine.Spec.BootVolumeSizeInGBs = "120"
	ms.OCIMachine.Spec.ImageId = "image"
	ms.OCIMachine.Spec.Shape = "shape"
	ms.OCIMachine.Name = "name"
	ms.OCIMachine.Spec.ShapeConfig.Ocpus = "2"
	ms.OCIMachine.Spec.ShapeConfig.MemoryInGBs = "100"
	ms.OCIMachine.Spec.ShapeConfig.BaselineOcpuUtilization = "BASELINE_1_8"
	ms.OCIMachine.Spec.IsPvEncryptionInTransitEnabled = true
	ms.OCICluster.Status.FailureDomains = map[string]clusterv1.FailureDomainSpec{
		"1": {
			Attributes: map[string]string{
				"AvailabilityDomain": "ad1",
			},
		},
		"2": {
			Attributes: map[string]string{
				"AvailabilityDomain": "ad2",
			},
		},
		"3": {
			Attributes: map[string]string{
				"AvailabilityDomain": "ad3",
			},
		},
	}
	ms.Machine.Spec.FailureDomain = common.String("2")
	ms.OCICluster.Spec.NetworkSpec.Vcn.Subnets = []*infrastructurev1beta1.Subnet{
		{
			Role: infrastructurev1beta1.WorkerRole,
			ID:   common.String("nodesubnet"),
		},
	}
	ms.OCICluster.UID = "uid"
	ms.OCICluster.Spec.OCIResourceIdentifier = "resource_uid"
	ms.OCIMachine.UID = "machineuid"
}
