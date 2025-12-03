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
	"reflect"
	"testing"

	"github.com/oracle/cluster-api-provider-oci/cloud/services/loadbalancer/mock_lb"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer/mock_nlb"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/workrequests/mock_workrequests"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute/mock_compute"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
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
		ociCluster    infrastructurev1beta2.OCICluster
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
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
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
				ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
				ociCluster.Spec.CompartmentId = "clustercompartment"
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
				ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
				ociCluster.Spec.CompartmentId = "clustercompartment"

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
			name:          "retries alternate fault domains on capacity errors",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
				ociCluster.Status.FailureDomains = map[string]clusterv1.FailureDomainSpec{
					"1": {
						Attributes: map[string]string{
							"AvailabilityDomain": "ad1",
							"FaultDomain":        "FAULT-DOMAIN-1",
						},
					},
					"2": {
						Attributes: map[string]string{
							"AvailabilityDomain": "ad1",
							"FaultDomain":        "FAULT-DOMAIN-2",
						},
					},
				}
				ms.Machine.Spec.FailureDomain = common.String("1")
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				first := computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return faultDomainRequestMatcher(request, "FAULT-DOMAIN-1")
				})).Return(core.LaunchInstanceResponse{}, newOutOfCapacityServiceError("out of host capacity"))

				second := computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return faultDomainRequestMatcher(request, "FAULT-DOMAIN-2")
				})).Return(core.LaunchInstanceResponse{
					Instance: core.Instance{
						Id: common.String("instance-id"),
					},
				}, nil)

				gomock.InOrder(first, second)
			},
		},
		{
			name:                "returns error after exhausting all fault domains",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("out of host capacity"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
				ociCluster.Status.FailureDomains = map[string]clusterv1.FailureDomainSpec{
					"1": {
						Attributes: map[string]string{
							"AvailabilityDomain": "ad1",
							"FaultDomain":        "FAULT-DOMAIN-1",
						},
					},
					"2": {
						Attributes: map[string]string{
							"AvailabilityDomain": "ad1",
							"FaultDomain":        "FAULT-DOMAIN-2",
						},
					},
				}
				ms.Machine.Spec.FailureDomain = common.String("1")
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				first := computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return faultDomainRequestMatcher(request, "FAULT-DOMAIN-1")
				})).Return(core.LaunchInstanceResponse{}, newOutOfCapacityServiceError("out of host capacity"))

				second := computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return faultDomainRequestMatcher(request, "FAULT-DOMAIN-2")
				})).Return(core.LaunchInstanceResponse{}, newOutOfCapacityServiceError("out of host capacity in fd2"))

				gomock.InOrder(first, second)
			},
		},
		{
			name:                "does not retry when fault domain is derived from availability domain cache",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.New("out of host capacity"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
				ociCluster.Status.FailureDomains = map[string]clusterv1.FailureDomainSpec{
					"1": {
						Attributes: map[string]string{
							"AvailabilityDomain": "ad1",
						},
					},
				}
				ociCluster.Spec.AvailabilityDomains = map[string]infrastructurev1beta2.OCIAvailabilityDomain{
					"ad1": {
						Name:         "ad1",
						FaultDomains: []string{"FAULT-DOMAIN-1"},
					},
				}
				ms.Machine.Spec.FailureDomain = common.String("1")
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return faultDomainRequestMatcher(request, "FAULT-DOMAIN-1")
				})).Return(core.LaunchInstanceResponse{}, newOutOfCapacityServiceError("out of host capacity"))
			},
		},
		{
			name:          "check all params together",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.CapacityReservationId = common.String("cap-id")
				ms.OCIMachine.Spec.ComputeClusterId = common.String("cluster-id")
				ms.OCIMachine.Spec.DedicatedVmHostId = common.String("dedicated-host-id")
				ms.OCIMachine.Spec.NetworkDetails.HostnameLabel = common.String("hostname-label")
				ms.OCIMachine.Spec.NetworkDetails.SkipSourceDestCheck = common.Bool(true)
				ms.OCIMachine.Spec.NetworkDetails.AssignPrivateDnsRecord = common.Bool(true)
				ms.OCIMachine.Spec.NetworkDetails.DisplayName = common.String("display-name")
				ms.OCIMachine.Spec.NetworkDetails.AssignIpv6Ip = true
				ms.OCIMachine.Spec.InstanceSourceViaImageDetails = &infrastructurev1beta2.InstanceSourceViaImageConfig{
					KmsKeyId:            common.String("kms-key-id"),
					BootVolumeVpusPerGB: common.Int64(32),
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					CapacityReservationId: common.String("cap-id"),
					DedicatedVmHostId:     common.String("dedicated-host-id"),
					ComputeClusterId:      common.String("cluster-id"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
						KmsKeyId:            common.String("kms-key-id"),
						BootVolumeVpusPerGB: common.Int64(32),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("nodesubnet"),
						AssignPublicIp: common.Bool(false),
						DefinedTags:    map[string]map[string]interface{}{},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
						NsgIds:                 make([]string, 0),
						HostnameLabel:          common.String("hostname-label"),
						SkipSourceDestCheck:    common.Bool(true),
						AssignPrivateDnsRecord: common.Bool(true),
						AssignIpv6Ip:           common.Bool(true),
						DisplayName:            common.String("display-name"),
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
			name:          "check all params together, with subnet id set",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.CapacityReservationId = common.String("cap-id")
				ms.OCIMachine.Spec.DedicatedVmHostId = common.String("dedicated-host-id")
				ms.OCIMachine.Spec.NetworkDetails.HostnameLabel = common.String("hostname-label")
				ms.OCIMachine.Spec.NetworkDetails.SubnetId = common.String("subnet-machine-id")
				ms.OCIMachine.Spec.NetworkDetails.NSGId = common.String("nsg-machine-id")
				ms.OCIMachine.Spec.NetworkDetails.SkipSourceDestCheck = common.Bool(true)
				ms.OCIMachine.Spec.NetworkDetails.AssignPrivateDnsRecord = common.Bool(true)
				ms.OCIMachine.Spec.NetworkDetails.AssignIpv6Ip = true
				ms.OCIMachine.Spec.NetworkDetails.DisplayName = common.String("display-name")
				ms.OCIMachine.Spec.InstanceSourceViaImageDetails = &infrastructurev1beta2.InstanceSourceViaImageConfig{
					KmsKeyId:            common.String("kms-key-id"),
					BootVolumeVpusPerGB: common.Int64(32),
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					CapacityReservationId: common.String("cap-id"),
					DedicatedVmHostId:     common.String("dedicated-host-id"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
						KmsKeyId:            common.String("kms-key-id"),
						BootVolumeVpusPerGB: common.Int64(32),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("subnet-machine-id"),
						AssignPublicIp: common.Bool(false),
						DefinedTags:    map[string]map[string]interface{}{},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
						NsgIds:                 []string{"nsg-machine-id"},
						HostnameLabel:          common.String("hostname-label"),
						SkipSourceDestCheck:    common.Bool(true),
						AssignPrivateDnsRecord: common.Bool(true),
						AssignIpv6Ip:           common.Bool(true),
						DisplayName:            common.String("display-name"),
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
			name:          "check all params together, with subnet id set, nsg id list",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.CapacityReservationId = common.String("cap-id")
				ms.OCIMachine.Spec.DedicatedVmHostId = common.String("dedicated-host-id")
				ms.OCIMachine.Spec.NetworkDetails.HostnameLabel = common.String("hostname-label")
				ms.OCIMachine.Spec.NetworkDetails.SubnetId = common.String("subnet-machine-id")
				ms.OCIMachine.Spec.NetworkDetails.NSGIds = []string{"nsg-machine-id-1", "nsg-machine-id-2"}
				// above array should take precedence
				ms.OCIMachine.Spec.NetworkDetails.NSGId = common.String("nsg-machine-id")
				ms.OCIMachine.Spec.NetworkDetails.SkipSourceDestCheck = common.Bool(true)
				ms.OCIMachine.Spec.NetworkDetails.AssignPrivateDnsRecord = common.Bool(true)
				ms.OCIMachine.Spec.NetworkDetails.AssignIpv6Ip = true
				ms.OCIMachine.Spec.NetworkDetails.DisplayName = common.String("display-name")
				ms.OCIMachine.Spec.LaunchVolumeAttachment = []infrastructurev1beta2.LaunchVolumeAttachment{
					{
						Type: infrastructurev1beta2.IscsiType,
						IscsiAttachment: infrastructurev1beta2.LaunchIscsiVolumeAttachment{
							Device:      common.String("/dev/oci"),
							IsShareable: common.Bool(true),
							LaunchCreateVolumeFromAttributes: infrastructurev1beta2.LaunchCreateVolumeFromAttributes{
								DisplayName: common.String("test-volume"),
								SizeInGBs:   common.Int64(75),
								VpusPerGB:   common.Int64(20),
							},
						},
					},
				}
				ms.OCIMachine.Spec.InstanceSourceViaImageDetails = &infrastructurev1beta2.InstanceSourceViaImageConfig{
					KmsKeyId:            common.String("kms-key-id"),
					BootVolumeVpusPerGB: common.Int64(32),
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					CapacityReservationId: common.String("cap-id"),
					DedicatedVmHostId:     common.String("dedicated-host-id"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
						KmsKeyId:            common.String("kms-key-id"),
						BootVolumeVpusPerGB: common.Int64(32),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("subnet-machine-id"),
						AssignPublicIp: common.Bool(false),
						DefinedTags:    map[string]map[string]interface{}{},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
						NsgIds:                 []string{"nsg-machine-id-1", "nsg-machine-id-2"},
						HostnameLabel:          common.String("hostname-label"),
						SkipSourceDestCheck:    common.Bool(true),
						AssignPrivateDnsRecord: common.Bool(true),
						AssignIpv6Ip:           common.Bool(true),
						DisplayName:            common.String("display-name"),
					},
					LaunchVolumeAttachments: []core.LaunchAttachVolumeDetails{
						core.LaunchAttachIScsiVolumeDetails{
							Device:      common.String("/dev/oci"),
							IsShareable: common.Bool(true),
							LaunchCreateVolumeDetails: core.LaunchCreateVolumeFromAttributes{
								DisplayName: common.String("test-volume"),
								SizeInGBs:   common.Int64(75),
								VpusPerGB:   common.Int64(20),
							},
						},
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
			name:          "check all params together, with paravirtualized volume support",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.CapacityReservationId = common.String("cap-id")
				ms.OCIMachine.Spec.DedicatedVmHostId = common.String("dedicated-host-id")
				ms.OCIMachine.Spec.NetworkDetails.HostnameLabel = common.String("hostname-label")
				ms.OCIMachine.Spec.NetworkDetails.SubnetId = common.String("subnet-machine-id")
				ms.OCIMachine.Spec.NetworkDetails.NSGIds = []string{"nsg-machine-id-1", "nsg-machine-id-2"}
				// above array should take precedence
				ms.OCIMachine.Spec.NetworkDetails.NSGId = common.String("nsg-machine-id")
				ms.OCIMachine.Spec.NetworkDetails.SkipSourceDestCheck = common.Bool(true)
				ms.OCIMachine.Spec.NetworkDetails.AssignPrivateDnsRecord = common.Bool(true)
				ms.OCIMachine.Spec.NetworkDetails.AssignIpv6Ip = true
				ms.OCIMachine.Spec.NetworkDetails.DisplayName = common.String("display-name")
				ms.OCIMachine.Spec.LaunchVolumeAttachment = []infrastructurev1beta2.LaunchVolumeAttachment{
					{
						Type: infrastructurev1beta2.ParavirtualizedType,
						ParavirtualizedAttachment: infrastructurev1beta2.LaunchParavirtualizedVolumeAttachment{
							Device:                         common.String("/dev/oci"),
							IsShareable:                    common.Bool(true),
							IsPvEncryptionInTransitEnabled: common.Bool(false),
							LaunchCreateVolumeFromAttributes: infrastructurev1beta2.LaunchCreateVolumeFromAttributes{
								DisplayName: common.String("test-volume"),
								SizeInGBs:   common.Int64(75),
								VpusPerGB:   common.Int64(20),
							},
						},
					},
				}
				ms.OCIMachine.Spec.InstanceSourceViaImageDetails = &infrastructurev1beta2.InstanceSourceViaImageConfig{
					KmsKeyId:            common.String("kms-key-id"),
					BootVolumeVpusPerGB: common.Int64(32),
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					CapacityReservationId: common.String("cap-id"),
					DedicatedVmHostId:     common.String("dedicated-host-id"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
						KmsKeyId:            common.String("kms-key-id"),
						BootVolumeVpusPerGB: common.Int64(32),
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("subnet-machine-id"),
						AssignPublicIp: common.Bool(false),
						DefinedTags:    map[string]map[string]interface{}{},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
						NsgIds:                 []string{"nsg-machine-id-1", "nsg-machine-id-2"},
						HostnameLabel:          common.String("hostname-label"),
						SkipSourceDestCheck:    common.Bool(true),
						AssignPrivateDnsRecord: common.Bool(true),
						AssignIpv6Ip:           common.Bool(true),
						DisplayName:            common.String("display-name"),
					},
					LaunchVolumeAttachments: []core.LaunchAttachVolumeDetails{
						core.LaunchAttachParavirtualizedVolumeDetails{
							Device:                         common.String("/dev/oci"),
							IsShareable:                    common.Bool(true),
							IsPvEncryptionInTransitEnabled: common.Bool(false),
							LaunchCreateVolumeDetails: core.LaunchCreateVolumeFromAttributes{
								DisplayName: common.String("test-volume"),
								SizeInGBs:   common.Int64(75),
								VpusPerGB:   common.Int64(20),
							},
						},
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
				ms.OCIMachine.Spec.ShapeConfig = infrastructurev1beta2.ShapeConfig{}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)

				launchDetails := core.LaunchInstanceDetails{DisplayName: common.String("name"),
					SourceDetails: core.InstanceSourceViaImageDetails{
						ImageId:             common.String("image"),
						BootVolumeSizeInGBs: common.Int64(120),
						KmsKeyId:            nil,
					},
					CreateVnicDetails: &core.CreateVnicDetails{
						SubnetId:       common.String("nodesubnet"),
						AssignPublicIp: common.Bool(false),
						AssignIpv6Ip:   common.Bool(false),
						DefinedTags:    map[string]map[string]interface{}{},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
						NsgIds: make([]string, 0),
						// Explicitly calling out nil checks here
						HostnameLabel:          nil,
						SkipSourceDestCheck:    nil,
						DisplayName:            nil,
						AssignPrivateDnsRecord: nil,
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
				ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
				ociCluster.Spec.NetworkSpec.Vcn.Subnets = append(ociCluster.Spec.NetworkSpec.Vcn.Subnets, &infrastructurev1beta2.Subnet{
					Role: infrastructurev1beta2.WorkerRole,
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
						AssignIpv6Ip:   common.Bool(false),
						DefinedTags:    map[string]map[string]interface{}{},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
						NsgIds: make([]string, 0),
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
				ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
				ociCluster.Spec.NetworkSpec.Vcn.Subnets = append(ociCluster.Spec.NetworkSpec.Vcn.Subnets, &infrastructurev1beta2.Subnet{
					Role: infrastructurev1beta2.WorkerRole,
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
						AssignIpv6Ip:   common.Bool(false),
						DefinedTags:    map[string]map[string]interface{}{},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
						NsgIds: make([]string, 0),
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
				ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
				ociCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List = append(ociCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List, &infrastructurev1beta2.NSG{
					Role: infrastructurev1beta2.WorkerRole,
					ID:   common.String("test-nsg-1"),
					Name: "test-nsg",
				}, &infrastructurev1beta2.NSG{
					Role: infrastructurev1beta2.WorkerRole,
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
						AssignIpv6Ip:   common.Bool(false),
						DefinedTags:    map[string]map[string]interface{}{},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
						NsgIds: []string{"test-nsg-1", "test-nsg-2"},
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
				ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
				ociCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List = append(ociCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List, &infrastructurev1beta2.NSG{
					Role: infrastructurev1beta2.WorkerRole,
					ID:   common.String("test-nsg-1"),
					Name: "test-nsg",
				}, &infrastructurev1beta2.NSG{
					Role: infrastructurev1beta2.WorkerRole,
					ID:   common.String("test-nsg-2"),
					Name: "test-nsg-name-2",
				})
				ms.OCIMachine.Spec.NetworkDetails.NsgNames = []string{"test-nsg-name-2"}
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
						AssignIpv6Ip:   common.Bool(false),
						DefinedTags:    map[string]map[string]interface{}{},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
						NsgIds: []string{"test-nsg-2"},
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
			name:          "check platform config amd vm",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PlatformConfig = &infrastructurev1beta2.PlatformConfig{
					PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeAmdvm,
					AmdVmPlatformConfig: infrastructurev1beta2.AmdVmPlatformConfig{
						IsMeasuredBootEnabled:          common.Bool(false),
						IsTrustedPlatformModuleEnabled: common.Bool(true),
						IsSecureBootEnabled:            common.Bool(true),
						IsMemoryEncryptionEnabled:      common.Bool(true),
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return platformConfigMatcher(request, core.AmdVmPlatformConfig{
						IsMeasuredBootEnabled:          common.Bool(false),
						IsTrustedPlatformModuleEnabled: common.Bool(true),
						IsSecureBootEnabled:            common.Bool(true),
						IsMemoryEncryptionEnabled:      common.Bool(true),
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "check platform config intel vm",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PlatformConfig = &infrastructurev1beta2.PlatformConfig{
					PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeIntelVm,
					IntelVmPlatformConfig: infrastructurev1beta2.IntelVmPlatformConfig{
						IsMeasuredBootEnabled:          common.Bool(false),
						IsTrustedPlatformModuleEnabled: common.Bool(true),
						IsSecureBootEnabled:            common.Bool(true),
						IsMemoryEncryptionEnabled:      common.Bool(false),
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return platformConfigMatcher(request, core.IntelVmPlatformConfig{
						IsMeasuredBootEnabled:          common.Bool(false),
						IsTrustedPlatformModuleEnabled: common.Bool(true),
						IsSecureBootEnabled:            common.Bool(true),
						IsMemoryEncryptionEnabled:      common.Bool(false),
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "check platform config amd rome bm",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PlatformConfig = &infrastructurev1beta2.PlatformConfig{
					PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeAmdRomeBm,
					AmdRomeBmPlatformConfig: infrastructurev1beta2.AmdRomeBmPlatformConfig{
						IsMeasuredBootEnabled:                    common.Bool(false),
						IsTrustedPlatformModuleEnabled:           common.Bool(true),
						IsSecureBootEnabled:                      common.Bool(true),
						IsMemoryEncryptionEnabled:                common.Bool(true),
						IsSymmetricMultiThreadingEnabled:         common.Bool(false),
						IsAccessControlServiceEnabled:            common.Bool(true),
						AreVirtualInstructionsEnabled:            common.Bool(false),
						IsInputOutputMemoryManagementUnitEnabled: common.Bool(false),
						PercentageOfCoresEnabled:                 common.Int(50),
						NumaNodesPerSocket:                       infrastructurev1beta2.AmdRomeBmPlatformConfigNumaNodesPerSocketNps4,
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return platformConfigMatcher(request, core.AmdRomeBmPlatformConfig{
						IsMeasuredBootEnabled:                    common.Bool(false),
						IsTrustedPlatformModuleEnabled:           common.Bool(true),
						IsSecureBootEnabled:                      common.Bool(true),
						IsMemoryEncryptionEnabled:                common.Bool(true),
						IsSymmetricMultiThreadingEnabled:         common.Bool(false),
						IsAccessControlServiceEnabled:            common.Bool(true),
						AreVirtualInstructionsEnabled:            common.Bool(false),
						IsInputOutputMemoryManagementUnitEnabled: common.Bool(false),
						PercentageOfCoresEnabled:                 common.Int(50),
						NumaNodesPerSocket:                       core.AmdRomeBmPlatformConfigNumaNodesPerSocketNps4,
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "check platform config amd rome gpu bm",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PlatformConfig = &infrastructurev1beta2.PlatformConfig{
					PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeAmdRomeBmGpu,
					AmdRomeBmGpuPlatformConfig: infrastructurev1beta2.AmdRomeBmGpuPlatformConfig{
						IsMeasuredBootEnabled:                    common.Bool(false),
						IsTrustedPlatformModuleEnabled:           common.Bool(true),
						IsSecureBootEnabled:                      common.Bool(true),
						IsMemoryEncryptionEnabled:                common.Bool(false),
						IsSymmetricMultiThreadingEnabled:         common.Bool(false),
						IsAccessControlServiceEnabled:            common.Bool(true),
						AreVirtualInstructionsEnabled:            common.Bool(false),
						IsInputOutputMemoryManagementUnitEnabled: common.Bool(false),
						NumaNodesPerSocket:                       infrastructurev1beta2.AmdRomeBmGpuPlatformConfigNumaNodesPerSocketNps2,
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return platformConfigMatcher(request, core.AmdRomeBmGpuPlatformConfig{
						IsMeasuredBootEnabled:                    common.Bool(false),
						IsTrustedPlatformModuleEnabled:           common.Bool(true),
						IsSecureBootEnabled:                      common.Bool(true),
						IsMemoryEncryptionEnabled:                common.Bool(false),
						IsSymmetricMultiThreadingEnabled:         common.Bool(false),
						IsAccessControlServiceEnabled:            common.Bool(true),
						AreVirtualInstructionsEnabled:            common.Bool(false),
						IsInputOutputMemoryManagementUnitEnabled: common.Bool(false),
						NumaNodesPerSocket:                       core.AmdRomeBmGpuPlatformConfigNumaNodesPerSocketNps2,
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "check platform config intel icelake bm",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PlatformConfig = &infrastructurev1beta2.PlatformConfig{
					PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeIntelIcelakeBm,
					IntelIcelakeBmPlatformConfig: infrastructurev1beta2.IntelIcelakeBmPlatformConfig{
						IsMeasuredBootEnabled:                    common.Bool(false),
						IsTrustedPlatformModuleEnabled:           common.Bool(true),
						IsSecureBootEnabled:                      common.Bool(true),
						IsMemoryEncryptionEnabled:                common.Bool(true),
						IsSymmetricMultiThreadingEnabled:         common.Bool(false),
						IsInputOutputMemoryManagementUnitEnabled: common.Bool(false),
						PercentageOfCoresEnabled:                 common.Int(56),
						NumaNodesPerSocket:                       infrastructurev1beta2.IntelIcelakeBmPlatformConfigNumaNodesPerSocketNps1,
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return platformConfigMatcher(request, core.IntelIcelakeBmPlatformConfig{
						IsMeasuredBootEnabled:                    common.Bool(false),
						IsTrustedPlatformModuleEnabled:           common.Bool(true),
						IsSecureBootEnabled:                      common.Bool(true),
						IsMemoryEncryptionEnabled:                common.Bool(true),
						IsSymmetricMultiThreadingEnabled:         common.Bool(false),
						IsInputOutputMemoryManagementUnitEnabled: common.Bool(false),
						PercentageOfCoresEnabled:                 common.Int(56),
						NumaNodesPerSocket:                       core.IntelIcelakeBmPlatformConfigNumaNodesPerSocketNps1,
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "check platform config intel skylake bm",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PlatformConfig = &infrastructurev1beta2.PlatformConfig{
					PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeIntelSkylakeBm,
					IntelSkylakeBmPlatformConfig: infrastructurev1beta2.IntelSkylakeBmPlatformConfig{
						IsMeasuredBootEnabled:          common.Bool(false),
						IsTrustedPlatformModuleEnabled: common.Bool(true),
						IsSecureBootEnabled:            common.Bool(true),
						IsMemoryEncryptionEnabled:      common.Bool(false),
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return platformConfigMatcher(request, core.IntelSkylakeBmPlatformConfig{
						IsMeasuredBootEnabled:          common.Bool(false),
						IsTrustedPlatformModuleEnabled: common.Bool(true),
						IsSecureBootEnabled:            common.Bool(true),
						IsMemoryEncryptionEnabled:      common.Bool(false),
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "check platform config amd milan bm",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PlatformConfig = &infrastructurev1beta2.PlatformConfig{
					PlatformConfigType: infrastructurev1beta2.PlatformConfigTypeAmdMilanBm,
					AmdMilanBmPlatformConfig: infrastructurev1beta2.AmdMilanBmPlatformConfig{
						IsMeasuredBootEnabled:                    common.Bool(false),
						IsTrustedPlatformModuleEnabled:           common.Bool(true),
						IsSecureBootEnabled:                      common.Bool(true),
						IsAccessControlServiceEnabled:            common.Bool(true),
						IsMemoryEncryptionEnabled:                common.Bool(true),
						IsSymmetricMultiThreadingEnabled:         common.Bool(false),
						IsInputOutputMemoryManagementUnitEnabled: common.Bool(false),
						AreVirtualInstructionsEnabled:            common.Bool(true),
						PercentageOfCoresEnabled:                 common.Int(56),
						NumaNodesPerSocket:                       infrastructurev1beta2.AmdMilanBmPlatformConfigNumaNodesPerSocketNps1,
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return platformConfigMatcher(request, core.AmdMilanBmPlatformConfig{
						IsMeasuredBootEnabled:                    common.Bool(false),
						IsTrustedPlatformModuleEnabled:           common.Bool(true),
						IsSecureBootEnabled:                      common.Bool(true),
						IsMemoryEncryptionEnabled:                common.Bool(true),
						IsAccessControlServiceEnabled:            common.Bool(true),
						IsSymmetricMultiThreadingEnabled:         common.Bool(false),
						IsInputOutputMemoryManagementUnitEnabled: common.Bool(false),
						AreVirtualInstructionsEnabled:            common.Bool(true),
						PercentageOfCoresEnabled:                 common.Int(56),
						NumaNodesPerSocket:                       core.AmdMilanBmPlatformConfigNumaNodesPerSocketNps1,
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "agent config",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.AgentConfig = &infrastructurev1beta2.LaunchInstanceAgentConfig{
					IsMonitoringDisabled:  common.Bool(false),
					IsManagementDisabled:  common.Bool(true),
					AreAllPluginsDisabled: common.Bool(true),
					PluginsConfig: []infrastructurev1beta2.InstanceAgentPluginConfig{
						{
							Name:         common.String("test-plugin"),
							DesiredState: infrastructurev1beta2.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
						},
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return agentConfigMatcher(request, &core.LaunchInstanceAgentConfigDetails{
						IsMonitoringDisabled:  common.Bool(false),
						IsManagementDisabled:  common.Bool(true),
						AreAllPluginsDisabled: common.Bool(true),
						PluginsConfig: []core.InstanceAgentPluginConfigDetails{
							{
								Name:         common.String("test-plugin"),
								DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
							},
						},
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "agent config - LaunchInstanceAgentConfigDetails has nil properties",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.AgentConfig = &infrastructurev1beta2.LaunchInstanceAgentConfig{
					IsMonitoringDisabled:  nil,
					IsManagementDisabled:  nil,
					AreAllPluginsDisabled: nil,
					PluginsConfig: []infrastructurev1beta2.InstanceAgentPluginConfig{
						{
							Name:         nil,
							DesiredState: infrastructurev1beta2.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
						},
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return agentConfigMatcher(request, &core.LaunchInstanceAgentConfigDetails{
						IsMonitoringDisabled:  nil,
						IsManagementDisabled:  nil,
						AreAllPluginsDisabled: nil,
						PluginsConfig: []core.InstanceAgentPluginConfigDetails{
							{
								Name:         nil,
								DesiredState: core.InstanceAgentPluginConfigDetailsDesiredStateEnabled,
							},
						},
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "launch options",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.LaunchOptions = &infrastructurev1beta2.LaunchOptions{
					BootVolumeType:                  infrastructurev1beta2.LaunchOptionsBootVolumeTypeIde,
					Firmware:                        infrastructurev1beta2.LaunchOptionsFirmwareUefi64,
					NetworkType:                     infrastructurev1beta2.LaunchOptionsNetworkTypeVfio,
					RemoteDataVolumeType:            infrastructurev1beta2.LaunchOptionsRemoteDataVolumeTypeIde,
					IsConsistentVolumeNamingEnabled: common.Bool(true),
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return launchOptionsMatcher(request, &core.LaunchOptions{
						BootVolumeType:                  core.LaunchOptionsBootVolumeTypeIde,
						Firmware:                        core.LaunchOptionsFirmwareUefi64,
						NetworkType:                     core.LaunchOptionsNetworkTypeVfio,
						RemoteDataVolumeType:            core.LaunchOptionsRemoteDataVolumeTypeIde,
						IsConsistentVolumeNamingEnabled: common.Bool(true),
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "launch options - without IsConsistentVolumeNamingEnabled",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.LaunchOptions = &infrastructurev1beta2.LaunchOptions{
					BootVolumeType:       infrastructurev1beta2.LaunchOptionsBootVolumeTypeIde,
					Firmware:             infrastructurev1beta2.LaunchOptionsFirmwareUefi64,
					NetworkType:          infrastructurev1beta2.LaunchOptionsNetworkTypeVfio,
					RemoteDataVolumeType: infrastructurev1beta2.LaunchOptionsRemoteDataVolumeTypeIde,
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return launchOptionsMatcher(request, &core.LaunchOptions{
						BootVolumeType:                  core.LaunchOptionsBootVolumeTypeIde,
						Firmware:                        core.LaunchOptionsFirmwareUefi64,
						NetworkType:                     core.LaunchOptionsNetworkTypeVfio,
						RemoteDataVolumeType:            core.LaunchOptionsRemoteDataVolumeTypeIde,
						IsConsistentVolumeNamingEnabled: nil,
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "instance options",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.InstanceOptions = &infrastructurev1beta2.InstanceOptions{
					AreLegacyImdsEndpointsDisabled: common.Bool(true),
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return instanceOptionsMatcher(request, &core.InstanceOptions{
						AreLegacyImdsEndpointsDisabled: common.Bool(true),
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "instance options - AreLegacyImdsEndpointsDisabled is nil",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.InstanceOptions = &infrastructurev1beta2.InstanceOptions{
					AreLegacyImdsEndpointsDisabled: nil,
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return instanceOptionsMatcher(request, &core.InstanceOptions{
						AreLegacyImdsEndpointsDisabled: nil,
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "availability config",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.AvailabilityConfig = &infrastructurev1beta2.LaunchInstanceAvailabilityConfig{
					IsLiveMigrationPreferred: common.Bool(true),
					RecoveryAction:           infrastructurev1beta2.LaunchInstanceAvailabilityConfigDetailsRecoveryActionRestoreInstance,
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return avalabilityConfigMatcher(request, &core.LaunchInstanceAvailabilityConfigDetails{
						IsLiveMigrationPreferred: common.Bool(true),
						RecoveryAction:           core.LaunchInstanceAvailabilityConfigDetailsRecoveryActionRestoreInstance,
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "availability config - IsLiveMigrationPreferred nil",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.AvailabilityConfig = &infrastructurev1beta2.LaunchInstanceAvailabilityConfig{
					IsLiveMigrationPreferred: nil,
					RecoveryAction:           infrastructurev1beta2.LaunchInstanceAvailabilityConfigDetailsRecoveryActionRestoreInstance,
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return avalabilityConfigMatcher(request, &core.LaunchInstanceAvailabilityConfigDetails{
						IsLiveMigrationPreferred: nil,
						RecoveryAction:           core.LaunchInstanceAvailabilityConfigDetailsRecoveryActionRestoreInstance,
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "preemtible config",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PreemptibleInstanceConfig = &infrastructurev1beta2.PreemptibleInstanceConfig{
					TerminatePreemptionAction: &infrastructurev1beta2.TerminatePreemptionAction{
						PreserveBootVolume: common.Bool(true),
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return preemtibleConfigMatcher(request, &core.PreemptibleInstanceConfigDetails{
						PreemptionAction: core.TerminatePreemptionAction{
							PreserveBootVolume: common.Bool(true),
						},
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "preemtible config - TerminatePreemptionAction nil",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PreemptibleInstanceConfig = &infrastructurev1beta2.PreemptibleInstanceConfig{
					TerminatePreemptionAction: nil,
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return preemtibleConfigMatcher(request, &core.PreemptibleInstanceConfigDetails{
						PreemptionAction: nil,
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
			},
		},
		{
			name:          "preemtible config - TerminatePreemptionAction.PreserveBootVolume nil",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				setupAllParams(ms)
				ms.OCIMachine.Spec.PreemptibleInstanceConfig = &infrastructurev1beta2.PreemptibleInstanceConfig{
					TerminatePreemptionAction: &infrastructurev1beta2.TerminatePreemptionAction{
						PreserveBootVolume: nil,
					},
				}
				computeClient.EXPECT().ListInstances(gomock.Any(), gomock.Eq(core.ListInstancesRequest{
					DisplayName:   common.String("name"),
					CompartmentId: common.String("test"),
				})).Return(core.ListInstancesResponse{}, nil)
				computeClient.EXPECT().LaunchInstance(gomock.Any(), Eq(func(request interface{}) error {
					return preemtibleConfigMatcher(request, &core.PreemptibleInstanceConfigDetails{
						PreemptionAction: core.TerminatePreemptionAction{
							PreserveBootVolume: nil,
						},
					})
				})).Return(core.LaunchInstanceResponse{}, nil)
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

func faultDomainRequestMatcher(request interface{}, matchStr string) error {
	r, ok := request.(core.LaunchInstanceRequest)
	if !ok {
		return errors.New("expecting LaunchInstanceRequest type")
	}
	if matchStr == "" {
		if r.LaunchInstanceDetails.FaultDomain != nil {
			return errors.New("expected fault domain to be nil")
		}
		return nil
	}
	if r.LaunchInstanceDetails.FaultDomain == nil {
		return errors.New("fault domain was nil")
	}
	if *r.LaunchInstanceDetails.FaultDomain != matchStr {
		return errors.New(fmt.Sprintf("expecting fault domain %s but got %s", matchStr, *r.LaunchInstanceDetails.FaultDomain))
	}
	return nil
}

func platformConfigMatcher(actual interface{}, expected core.PlatformConfig) error {
	r, ok := actual.(core.LaunchInstanceRequest)
	if !ok {
		return errors.New("expecting LaunchInstanceRequest type")
	}
	if !reflect.DeepEqual(r.PlatformConfig, expected) {
		return errors.New(fmt.Sprintf("expecting %v, actual %v", expected, r.PlatformConfig))
	}
	return nil
}
func agentConfigMatcher(actual interface{}, expected *core.LaunchInstanceAgentConfigDetails) error {
	r, ok := actual.(core.LaunchInstanceRequest)
	if !ok {
		return errors.New("expecting LaunchInstanceRequest type")
	}
	if !reflect.DeepEqual(r.AgentConfig, expected) {
		return errors.New(fmt.Sprintf("expecting %v, actual %v", expected, r.AgentConfig))
	}
	return nil
}

func launchOptionsMatcher(actual interface{}, expected *core.LaunchOptions) error {
	r, ok := actual.(core.LaunchInstanceRequest)
	if !ok {
		return errors.New("expecting LaunchInstanceRequest type")
	}
	if !reflect.DeepEqual(r.LaunchOptions, expected) {
		return errors.New(fmt.Sprintf("expecting %v, actual %v", expected, r.LaunchOptions))
	}
	return nil
}

func instanceOptionsMatcher(actual interface{}, expected *core.InstanceOptions) error {
	r, ok := actual.(core.LaunchInstanceRequest)
	if !ok {
		return errors.New("expecting LaunchInstanceRequest type")
	}
	if !reflect.DeepEqual(r.InstanceOptions, expected) {
		return errors.New(fmt.Sprintf("expecting %v, actual %v", expected, r.InstanceOptions))
	}
	return nil
}

func avalabilityConfigMatcher(actual interface{}, expected *core.LaunchInstanceAvailabilityConfigDetails) error {
	r, ok := actual.(core.LaunchInstanceRequest)
	if !ok {
		return errors.New("expecting LaunchInstanceRequest type")
	}
	if !reflect.DeepEqual(r.AvailabilityConfig, expected) {
		return errors.New(fmt.Sprintf("expecting %v, actual %v", expected, r.AvailabilityConfig))
	}
	return nil
}

func preemtibleConfigMatcher(actual interface{}, expected *core.PreemptibleInstanceConfigDetails) error {
	r, ok := actual.(core.LaunchInstanceRequest)
	if !ok {
		return errors.New("expecting LaunchInstanceRequest type")
	}
	if !reflect.DeepEqual(r.PreemptibleInstanceConfig, expected) {
		return errors.New(fmt.Sprintf("expecting %v, actual %v", expected, r.PreemptibleInstanceConfig))
	}
	return nil
}

func TestNLBReconciliationCreation(t *testing.T) {
	var (
		ms         *MachineScope
		mockCtrl   *gomock.Controller
		nlbClient  *mock_nlb.MockNetworkLoadBalancerClient
		wrClient   *mock_workrequests.MockClient
		ociCluster infrastructurev1beta2.OCICluster
	)
	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
		wrClient = mock_workrequests.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects().Build()
		ociCluster = infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
		}
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlbid")
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ms, err = NewMachineScope(MachineScopeParams{
			NetworkLoadBalancerClient: nlbClient,
			WorkRequestsClient:        wrClient,
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "uid",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test",
				},
			},
			Machine: &clusterv1.Machine{},
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
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
		testSpecificSetup   func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient)
	}{
		{
			name:          "ip doesnt exist",
			errorExpected: true,
			matchError:    errors.New("could not find machine IP Address in status object"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
			},
		},
		{
			name:          "ip exists",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
			name:          "work request exists, will retry",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
					OpcWorkRequestId: common.String("wrid-1"),
				}, nil)

				nlbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					networkloadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid-1"),
					})).Return(networkloadbalancer.GetWorkRequestResponse{
					WorkRequest: networkloadbalancer.WorkRequest{
						Status: networkloadbalancer.OperationStatusSucceeded,
					}}, nil)
			},
		},
		{
			name:          "backend exists",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
			name:                "work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.Errorf("WorkRequest %s failed", "wrid"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
						CompartmentId: common.String("compartment-id"),
						Status:        networkloadbalancer.OperationStatusFailed,
					}}, nil)
				nlbClient.EXPECT().ListWorkRequestErrors(gomock.Any(), gomock.Eq(networkloadbalancer.ListWorkRequestErrorsRequest{
					WorkRequestId: common.String("wrid"),
					CompartmentId: common.String("compartment-id"),
				})).Return(networkloadbalancer.ListWorkRequestErrorsResponse{
					WorkRequestErrorCollection: networkloadbalancer.WorkRequestErrorCollection{
						Items: []networkloadbalancer.WorkRequestError{
							{
								Code:    common.String("OKE-001"),
								Message: common.String("No more Ip available in CIDR 1.1.1.1/1"),
							},
						},
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
			tc.testSpecificSetup(ms, nlbClient, wrClient)
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

func TestNLBReconciliationDeletion(t *testing.T) {
	var (
		ms         *MachineScope
		mockCtrl   *gomock.Controller
		nlbClient  *mock_nlb.MockNetworkLoadBalancerClient
		wrClient   *mock_workrequests.MockClient
		ociCluster infrastructurev1beta2.OCICluster
	)
	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		nlbClient = mock_nlb.NewMockNetworkLoadBalancerClient(mockCtrl)
		wrClient = mock_workrequests.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects().Build()
		ociCluster = infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
		}
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("nlbid")
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ms, err = NewMachineScope(MachineScopeParams{
			NetworkLoadBalancerClient: nlbClient,
			WorkRequestsClient:        wrClient,
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "uid",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test",
				},
			},
			Machine: &clusterv1.Machine{},
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
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
		testSpecificSetup   func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient)
	}{
		{
			name:          "get nlb error",
			errorExpected: true,
			matchError:    errors.New("could not get nlb"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{}}, errors.New("could not get nlb"))
			},
		},
		{
			name:          "get nlb error, not found",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				nlbClient.EXPECT().GetNetworkLoadBalancer(gomock.Any(), gomock.Eq(networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: common.String("nlbid"),
				})).Return(networkloadbalancer.GetNetworkLoadBalancerResponse{
					NetworkLoadBalancer: networkloadbalancer.NetworkLoadBalancer{}}, ociutil.ErrNotFound)
			},
		},
		{
			name:          "backend exists",
			errorExpected: false,
			matchError:    errors.New("could not get nlb"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
			name:          "work request exists, still delete should be called",
			errorExpected: false,
			matchError:    errors.New("could not get nlb"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
			name:                "work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.Errorf("WorkRequest %s failed", "wrid"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
						CompartmentId: common.String("compartment-id"),
						Status:        networkloadbalancer.OperationStatusFailed,
					}}, nil)

				nlbClient.EXPECT().ListWorkRequestErrors(gomock.Any(), gomock.Eq(networkloadbalancer.ListWorkRequestErrorsRequest{
					WorkRequestId: common.String("wrid"),
					CompartmentId: common.String("compartment-id"),
				})).Return(networkloadbalancer.ListWorkRequestErrorsResponse{
					WorkRequestErrorCollection: networkloadbalancer.WorkRequestErrorCollection{
						Items: []networkloadbalancer.WorkRequestError{
							{
								Code:    common.String("OKE-001"),
								Message: common.String("Failed due to unknown error"),
							},
						},
					},
				}, nil)
			},
		},
		{
			name:          "delete backend fails",
			errorExpected: true,
			matchError:    errors.New("backend request failed"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_nlb.MockNetworkLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
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
			tc.testSpecificSetup(ms, nlbClient, wrClient)
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

func TestLBReconciliationCreation(t *testing.T) {
	var (
		ms         *MachineScope
		mockCtrl   *gomock.Controller
		lbClient   *mock_lb.MockLoadBalancerClient
		wrClient   *mock_workrequests.MockClient
		ociCluster infrastructurev1beta2.OCICluster
	)
	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		lbClient = mock_lb.NewMockLoadBalancerClient(mockCtrl)
		wrClient = mock_workrequests.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects().Build()
		ociCluster = infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
		}
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("lbid")
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerType = infrastructurev1beta2.LoadBalancerTypeLB
		ms, err = NewMachineScope(MachineScopeParams{
			LoadBalancerClient: lbClient,
			WorkRequestsClient: wrClient,
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "uid",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test",
				},
			},
			Machine: &clusterv1.Machine{},
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
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
		testSpecificSetup   func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient)
	}{
		{
			name:          "ip doesnt exist",
			errorExpected: true,
			matchError:    errors.New("could not find machine IP Address in status object"),
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
			},
		},
		{
			name:          "ip exists",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{},
							},
						},
					},
				}, nil)

				lbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					loadbalancer.CreateBackendRequest{
						LoadBalancerId: common.String("lbid"),
						BackendSetName: common.String(APIServerLBBackendSetName),
						CreateBackendDetails: loadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(loadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					loadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					}}, nil)
			},
		},
		{
			name:          "work request exists, will retry",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{},
							},
						},
					},
				}, nil)
				machineScope.OCIMachine.Status.CreateBackendWorkRequestId = "wrid"
				lbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					loadbalancer.CreateBackendRequest{
						LoadBalancerId: common.String("lbid"),
						BackendSetName: common.String(APIServerLBBackendSetName),
						CreateBackendDetails: loadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(loadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid-1"),
				}, nil)

				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					loadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid-1"),
					})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					}}, nil)
			},
		},
		{
			name:          "backend exists",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{
									{
										Name: common.String("1.1.1.1:6443"),
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
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{},
							},
						},
					},
				}, nil)

				lbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					loadbalancer.CreateBackendRequest{
						LoadBalancerId: common.String("lbid"),
						BackendSetName: common.String(APIServerLBBackendSetName),
						CreateBackendDetails: loadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(loadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, errors.New("could not create backend"))
			},
		},
		{
			name:          "get lb error",
			errorExpected: true,
			matchError:    errors.New("could not get lb"),
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{}}, errors.New("could not get lb"))
			},
		},
		{
			name:                "work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.Errorf("WorkRequest %s failed", "wrid"),
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{},
							},
						},
					},
				}, nil)

				lbClient.EXPECT().CreateBackend(gomock.Any(), gomock.Eq(
					loadbalancer.CreateBackendRequest{
						LoadBalancerId: common.String("lbid"),
						BackendSetName: common.String(APIServerLBBackendSetName),
						CreateBackendDetails: loadbalancer.CreateBackendDetails{
							IpAddress: common.String("1.1.1.1"),
							Port:      common.Int(6443),
						},
						OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-backend", "uid"),
					})).Return(loadbalancer.CreateBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					loadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateFailed,
						ErrorDetails: []loadbalancer.WorkRequestError{
							{
								Message: common.String("Internal server error to create lb"),
							},
						},
					}}, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, lbClient, wrClient)
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
		lbClient   *mock_lb.MockLoadBalancerClient
		wrClient   *mock_workrequests.MockClient
		ociCluster infrastructurev1beta2.OCICluster
	)
	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		lbClient = mock_lb.NewMockLoadBalancerClient(mockCtrl)
		wrClient = mock_workrequests.NewMockClient(mockCtrl)
		client := fake.NewClientBuilder().WithObjects().Build()
		ociCluster = infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
		}
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId = common.String("lbid")
		ociCluster.Spec.ControlPlaneEndpoint.Port = 6443
		ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerType = infrastructurev1beta2.LoadBalancerTypeLB
		ms, err = NewMachineScope(MachineScopeParams{
			LoadBalancerClient: lbClient,
			WorkRequestsClient: wrClient,
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "uid",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test",
				},
			},
			Machine: &clusterv1.Machine{},
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
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
		testSpecificSetup   func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient)
	}{
		{
			name:          "get lb error",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				nlbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{}}, ociutil.ErrNotFound)
			},
		},
		{
			name:          "get lb error",
			errorExpected: true,
			matchError:    errors.New("could not get lb"),
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				nlbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{}}, errors.New("could not get lb"))
			},
		},
		{
			name:          "no ip",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, nlbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				nlbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{}}, nil)
			},
		},
		{
			name:          "backend exists",
			errorExpected: false,
			matchError:    errors.New("could not get lb"),
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{
									{
										Name: common.String("1.1.1.1:6443"),
									},
								},
							},
						},
					},
				}, nil)
				lbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Eq(loadbalancer.DeleteBackendRequest{
					LoadBalancerId: common.String("lbid"),
					BackendSetName: common.String(APIServerLBBackendSetName),
					BackendName:    common.String("1.1.1.1%3A6443"),
				})).Return(loadbalancer.DeleteBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					loadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					}}, nil)
			},
		},
		{
			name:          "backend does not exist",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name:     common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{},
							},
						},
					},
				}, nil)
			},
		},
		{
			name:          "work request exists, still delete should be called",
			errorExpected: false,
			matchError:    errors.New("could not get lb"),
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				machineScope.OCIMachine.Status.DeleteBackendWorkRequestId = "wrid"
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{
									{
										Name: common.String("1.1.1.1:6443"),
									},
								},
							},
						},
					},
				}, nil)
				lbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Eq(loadbalancer.DeleteBackendRequest{
					LoadBalancerId: common.String("lbid"),
					BackendSetName: common.String(APIServerLBBackendSetName),
					BackendName:    common.String("1.1.1.1%3A6443"),
				})).Return(loadbalancer.DeleteBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					loadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateSucceeded,
					}}, nil)
			},
		},
		{
			name:                "work request failed",
			errorExpected:       true,
			errorSubStringMatch: true,
			matchError:          errors.Errorf("WorkRequest %s failed", "wrid"),
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{
									{
										Name: common.String("1.1.1.1:6443"),
									},
								},
							},
						},
					},
				}, nil)
				lbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Eq(loadbalancer.DeleteBackendRequest{
					LoadBalancerId: common.String("lbid"),
					BackendSetName: common.String(APIServerLBBackendSetName),
					BackendName:    common.String("1.1.1.1%3A6443"),
				})).Return(loadbalancer.DeleteBackendResponse{
					OpcWorkRequestId: common.String("wrid"),
				}, nil)

				lbClient.EXPECT().GetWorkRequest(gomock.Any(), gomock.Eq(
					loadbalancer.GetWorkRequestRequest{
						WorkRequestId: common.String("wrid"),
					})).Return(loadbalancer.GetWorkRequestResponse{
					WorkRequest: loadbalancer.WorkRequest{
						LifecycleState: loadbalancer.WorkRequestLifecycleStateFailed,
						ErrorDetails: []loadbalancer.WorkRequestError{
							{
								Message: common.String("Internal Server error to delete lb"),
							},
						},
					}}, nil)
			},
		},
		{
			name:          "delete backend fails",
			errorExpected: true,
			matchError:    errors.New("backend request failed"),
			testSpecificSetup: func(machineScope *MachineScope, lbClient *mock_lb.MockLoadBalancerClient, wrClient *mock_workrequests.MockClient) {
				machineScope.OCIMachine.Status.Addresses = []clusterv1.MachineAddress{
					{
						Type:    clusterv1.MachineInternalIP,
						Address: "1.1.1.1",
					},
				}
				lbClient.EXPECT().GetLoadBalancer(gomock.Any(), gomock.Eq(loadbalancer.GetLoadBalancerRequest{
					LoadBalancerId: common.String("lbid"),
				})).Return(loadbalancer.GetLoadBalancerResponse{
					LoadBalancer: loadbalancer.LoadBalancer{
						BackendSets: map[string]loadbalancer.BackendSet{
							APIServerLBBackendSetName: {
								Name: common.String(APIServerLBBackendSetName),
								Backends: []loadbalancer.Backend{
									{
										Name: common.String("1.1.1.1:6443"),
									},
								},
							},
						},
					},
				}, nil)
				lbClient.EXPECT().DeleteBackend(gomock.Any(), gomock.Eq(loadbalancer.DeleteBackendRequest{
					LoadBalancerId: common.String("lbid"),
					BackendSetName: common.String(APIServerLBBackendSetName),
					BackendName:    common.String("1.1.1.1%3A6443"),
				})).Return(loadbalancer.DeleteBackendResponse{
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
			tc.testSpecificSetup(ms, lbClient, wrClient)
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
		ociCluster    infrastructurev1beta2.OCICluster
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		computeClient = mock_compute.NewMockComputeClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociCluster = infrastructurev1beta2.OCICluster{}
		ms, err = NewMachineScope(MachineScopeParams{
			ComputeClient: computeClient,
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test",
					InstanceId:    common.String("test"),
				},
			},
			Machine: &clusterv1.Machine{},
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
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
		instance            *core.Instance
		errorSubStringMatch bool
		testSpecificSetup   func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient)
	}{
		{
			name:          "delete instance",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = common.String("test")
				computeClient.EXPECT().TerminateInstance(gomock.Any(), gomock.Eq(core.TerminateInstanceRequest{
					InstanceId:                         common.String("test"),
					PreserveBootVolume:                 common.Bool(false),
					PreserveDataVolumesCreatedAtLaunch: common.Bool(false),
				})).Return(core.TerminateInstanceResponse{}, nil)
			},
			instance: &core.Instance{
				Id: common.String("test"),
			},
		},
		{
			name:          "delete instance error",
			errorExpected: true,
			matchError:    errors.New("could not terminate instance"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = common.String("test")
				computeClient.EXPECT().TerminateInstance(gomock.Any(), gomock.Eq(core.TerminateInstanceRequest{
					InstanceId:                         common.String("test"),
					PreserveBootVolume:                 common.Bool(false),
					PreserveDataVolumesCreatedAtLaunch: common.Bool(false),
				})).Return(core.TerminateInstanceResponse{}, errors.New("could not terminate instance"))
			},
			instance: &core.Instance{
				Id: common.String("test"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, computeClient)
			err := ms.DeleteMachine(context.Background(), tc.instance)
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
	ociCluster := ms.OCIClusterAccessor.(OCISelfManagedCluster).OCICluster
	ociCluster.Status.FailureDomains = map[string]clusterv1.FailureDomainSpec{
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
	ociCluster.Spec.AvailabilityDomains = map[string]infrastructurev1beta2.OCIAvailabilityDomain{
		"ad1": {
			Name:         "ad1",
			FaultDomains: []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
		},
		"ad2": {
			Name:         "ad2",
			FaultDomains: []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
		},
		"ad3": {
			Name:         "ad3",
			FaultDomains: []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
		},
	}
	ms.Machine.Spec.FailureDomain = common.String("2")
	ociCluster.Spec.NetworkSpec.Vcn.Subnets = []*infrastructurev1beta2.Subnet{
		{
			Role: infrastructurev1beta2.WorkerRole,
			ID:   common.String("nodesubnet"),
		},
	}
	ociCluster.UID = "uid"
	ociCluster.Spec.OCIResourceIdentifier = "resource_uid"
	ms.OCIMachine.UID = "machineuid"
}

func newOutOfCapacityServiceError(message string) error {
	return fmt.Errorf("%s: %s", ociutil.OutOfHostCapacityErr, message)
}
