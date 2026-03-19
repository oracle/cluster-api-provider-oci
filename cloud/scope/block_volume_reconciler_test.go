/*
 Copyright (c) 2025 Oracle and/or its affiliates.

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
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute/mock_compute"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/volume/mock_volume"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBlockVolumeReconciliation(t *testing.T) {
	var (
		ms                *MachineScope
		mockCtrl          *gomock.Controller
		blockVolumeClient *mock_volume.MockBlockVolumeClient
		ociCluster        infrastructurev1beta2.OCICluster
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		blockVolumeClient = mock_volume.NewMockBlockVolumeClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociCluster = infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				OCIResourceIdentifier: "resource_uid",
				CompartmentId:         "test-compartment",
			},
		}
		ms, err = NewMachineScope(MachineScopeParams{
			BlockVolumeClient: blockVolumeClient,
			ComputeClient:     mock_compute.NewMockComputeClient(mockCtrl),
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-machine",
					UID:  "machine-uid",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test-compartment",
				},
			},
			Machine: &clusterv1.Machine{},
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
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
		matchError          error
		errorSubStringMatch bool
		testSpecificSetup   func(machineScope *MachineScope, blockVolumeClient *mock_volume.MockBlockVolumeClient)
	}{
		{
			name:          "list volumes returns error",
			errorExpected: true,
			matchError:    errors.New("list volumes failed"),
			testSpecificSetup: func(machineScope *MachineScope, blockVolumeClient *mock_volume.MockBlockVolumeClient) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					DisplayName:        common.String("test-volume"),
					SizeInGBs:          common.Int64(50),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
				blockVolumeClient.EXPECT().ListVolumes(gomock.Any(), gomock.Any()).
					Return(core.ListVolumesResponse{}, errors.New("list volumes failed"))
			},
		},
		{
			name:          "create volume returns error",
			errorExpected: true,
			matchError:    errors.New("create volume failed"),
			testSpecificSetup: func(machineScope *MachineScope, blockVolumeClient *mock_volume.MockBlockVolumeClient) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					DisplayName:        common.String("new-volume"),
					SizeInGBs:          common.Int64(100),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
				blockVolumeClient.EXPECT().ListVolumes(gomock.Any(), gomock.Any()).
					Return(core.ListVolumesResponse{Items: []core.Volume{}}, nil)
				blockVolumeClient.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).
					Return(core.CreateVolumeResponse{}, errors.New("create volume failed"))
			},
		},
		{
			name:          "volume created with autotune policies",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, blockVolumeClient *mock_volume.MockBlockVolumeClient) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					DisplayName:        common.String("autotune-volume"),
					SizeInGBs:          common.Int64(50),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "DETACHED_VOLUME"},
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
				blockVolumeClient.EXPECT().ListVolumes(gomock.Any(), gomock.Any()).
					Return(core.ListVolumesResponse{Items: []core.Volume{}}, nil)
				blockVolumeClient.EXPECT().CreateVolume(gomock.Any(), gomock.Eq(core.CreateVolumeRequest{
					CreateVolumeDetails: core.CreateVolumeDetails{
						AvailabilityDomain: common.String("ad1"),
						CompartmentId:      common.String("test-compartment"),
						DisplayName:        common.String("autotune-volume"),
						SizeInGBs:          common.Int64(50),
						AutotunePolicies: []core.AutotunePolicy{
							core.DetachedVolumeAutotunePolicy{},
							core.PerformanceBasedAutotunePolicy{MaxVpusPerGB: common.Int64(20)},
						},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-bv", "machine-uid"),
				})).Return(core.CreateVolumeResponse{}, nil)
			},
		},
		{
			name:          "volume created with custom compartment id",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, blockVolumeClient *mock_volume.MockBlockVolumeClient) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					DisplayName:        common.String("custom-compartment-volume"),
					SizeInGBs:          common.Int64(50),
					CompartmentId:      common.String("custom-compartment"),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
				blockVolumeClient.EXPECT().ListVolumes(gomock.Any(), gomock.Eq(core.ListVolumesRequest{
					CompartmentId: common.String("custom-compartment"),
					DisplayName:   common.String("custom-compartment-volume"),
				})).Return(core.ListVolumesResponse{Items: []core.Volume{}}, nil)
				blockVolumeClient.EXPECT().CreateVolume(gomock.Any(), gomock.Eq(core.CreateVolumeRequest{
					CreateVolumeDetails: core.CreateVolumeDetails{
						AvailabilityDomain: common.String("ad1"),
						CompartmentId:      common.String("custom-compartment"),
						DisplayName:        common.String("custom-compartment-volume"),
						SizeInGBs:          common.Int64(50),
						AutotunePolicies: []core.AutotunePolicy{
							core.PerformanceBasedAutotunePolicy{MaxVpusPerGB: common.Int64(20)},
						},
						FreeformTags: map[string]string{
							ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
							ociutil.ClusterResourceIdentifier: "resource_uid",
						},
					},
					OpcRetryToken: ociutil.GetOPCRetryToken("%s-%s", "create-bv", "machine-uid"),
				})).Return(core.CreateVolumeResponse{}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, blockVolumeClient)
			err := ms.ReconcileBlockVolume(context.Background())
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

func TestDeleteBlockVolume(t *testing.T) {
	var (
		ms                *MachineScope
		mockCtrl          *gomock.Controller
		blockVolumeClient *mock_volume.MockBlockVolumeClient
		ociCluster        infrastructurev1beta2.OCICluster
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		blockVolumeClient = mock_volume.NewMockBlockVolumeClient(mockCtrl)
		client := fake.NewClientBuilder().Build()
		ociCluster = infrastructurev1beta2.OCICluster{
			ObjectMeta: metav1.ObjectMeta{
				UID: "uid",
			},
			Spec: infrastructurev1beta2.OCIClusterSpec{
				OCIResourceIdentifier: "resource_uid",
				CompartmentId:         "test-compartment",
			},
		}
		ms, err = NewMachineScope(MachineScopeParams{
			BlockVolumeClient: blockVolumeClient,
			ComputeClient:     mock_compute.NewMockComputeClient(mockCtrl),
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-machine",
					UID:  "machine-uid",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{
					CompartmentId: "test-compartment",
				},
			},
			Machine: &clusterv1.Machine{},
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
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
		matchError          error
		errorSubStringMatch bool
		testSpecificSetup   func(machineScope *MachineScope, blockVolumeClient *mock_volume.MockBlockVolumeClient)
	}{
		{
			name:          "volume found and deleted successfully",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, blockVolumeClient *mock_volume.MockBlockVolumeClient) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					DisplayName:        common.String("test-volume"),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
				blockVolumeClient.EXPECT().ListVolumes(gomock.Any(), gomock.Eq(core.ListVolumesRequest{
					CompartmentId: common.String("test-compartment"),
					DisplayName:   common.String("test-volume"),
				})).Return(core.ListVolumesResponse{
					Items: []core.Volume{
						{
							Id:          common.String("vol-id"),
							DisplayName: common.String("test-volume"),
							FreeformTags: map[string]string{
								ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
								ociutil.ClusterResourceIdentifier: "resource_uid",
							},
						},
					},
				}, nil)
				blockVolumeClient.EXPECT().DeleteVolume(gomock.Any(), gomock.Eq(core.DeleteVolumeRequest{
					VolumeId: common.String("vol-id"),
				})).Return(core.DeleteVolumeResponse{}, nil)
			},
		},
		{
			name:          "delete volume error",
			errorExpected: true,
			matchError:    errors.New("delete volume failed"),
			testSpecificSetup: func(machineScope *MachineScope, blockVolumeClient *mock_volume.MockBlockVolumeClient) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					DisplayName:        common.String("test-volume"),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
				blockVolumeClient.EXPECT().ListVolumes(gomock.Any(), gomock.Any()).
					Return(core.ListVolumesResponse{
						Items: []core.Volume{
							{
								Id:          common.String("vol-id"),
								DisplayName: common.String("test-volume"),
								FreeformTags: map[string]string{
									ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
									ociutil.ClusterResourceIdentifier: "resource_uid",
								},
							},
						},
					}, nil)
				blockVolumeClient.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).
					Return(core.DeleteVolumeResponse{}, errors.New("delete volume failed"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, blockVolumeClient)
			err := ms.DeleteBlockVolume(context.Background())
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

func TestGetBlockVolumeDesiredName(t *testing.T) {
	var (
		ms       *MachineScope
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().Build()
		ociCluster := infrastructurev1beta2.OCICluster{}
		ms, err = NewMachineScope(MachineScopeParams{
			ComputeClient:     mock_compute.NewMockComputeClient(mockCtrl),
			BlockVolumeClient: mock_volume.NewMockBlockVolumeClient(mockCtrl),
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-machine",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{},
			},
			Machine: &clusterv1.Machine{},
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
		})
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name              string
		expectedName      string
		testSpecificSetup func(machineScope *MachineScope)
	}{
		{
			name:         "returns empty when DisplayName is nil",
			expectedName: "",
			testSpecificSetup: func(machineScope *MachineScope) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{}
			},
		},
		{
			name:         "returns DisplayName when set",
			expectedName: "my-volume",
			testSpecificSetup: func(machineScope *MachineScope) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					DisplayName:        common.String("my-volume"),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
			},
		},
		{
			name:         "returns DisplayName even when AvailabilityDomain is nil",
			expectedName: "my-volume",
			testSpecificSetup: func(machineScope *MachineScope) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					DisplayName: common.String("my-volume"),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
			},
		},
		{
			name:         "returns empty when DisplayName is empty string",
			expectedName: "",
			testSpecificSetup: func(machineScope *MachineScope) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					DisplayName:        common.String(""),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
			},
		},
		{
			name:         "returns empty when DisplayName is nil",
			expectedName: "",
			testSpecificSetup: func(machineScope *MachineScope) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					DisplayName:        nil,
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
					VolumeType: "PARAVIRTUALIZED",
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms)
			name := ms.GetBlockVolumeDesiredName()
			g.Expect(name).To(Equal(tc.expectedName))
		})
	}
}

func TestToOCIAutotunePolicy(t *testing.T) {
	var (
		ms       *MachineScope
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T, g *WithT) {
		var err error
		mockCtrl = gomock.NewController(t)
		client := fake.NewClientBuilder().Build()
		ociCluster := infrastructurev1beta2.OCICluster{}
		ms, err = NewMachineScope(MachineScopeParams{
			ComputeClient:     mock_compute.NewMockComputeClient(mockCtrl),
			BlockVolumeClient: mock_volume.NewMockBlockVolumeClient(mockCtrl),
			OCIMachine: &infrastructurev1beta2.OCIMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-machine",
				},
				Spec: infrastructurev1beta2.OCIMachineSpec{},
			},
			Machine: &clusterv1.Machine{},
			Cluster: &clusterv1.Cluster{},
			OCIClusterAccessor: OCISelfManagedCluster{
				OCICluster: &ociCluster,
			},
			Client: client,
		})
		g.Expect(err).To(BeNil())
	}
	teardown := func(t *testing.T, g *WithT) {
		mockCtrl.Finish()
	}

	tests := []struct {
		name              string
		testSpecificSetup func(machineScope *MachineScope)
		expectedLen       int
		expectedPolicies  []core.AutotunePolicy
	}{
		{
			name: "detached volume policy",
			testSpecificSetup: func(machineScope *MachineScope) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "DETACHED_VOLUME"},
					},
				}
			},
			expectedLen: 1,
			expectedPolicies: []core.AutotunePolicy{
				core.DetachedVolumeAutotunePolicy{},
			},
		},
		{
			name: "performance based policy",
			testSpecificSetup: func(machineScope *MachineScope) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(20)},
					},
				}
			},
			expectedLen: 1,
			expectedPolicies: []core.AutotunePolicy{
				core.PerformanceBasedAutotunePolicy{MaxVpusPerGB: common.Int64(20)},
			},
		},
		{
			name: "multiple policies combined",
			testSpecificSetup: func(machineScope *MachineScope) {
				machineScope.OCIMachine.Spec.BlockVolumeSpec = infrastructurev1beta2.BlockVolumeSpec{
					AvailabilityDomain: common.String("ad1"),
					AutotunePolicies: []infrastructurev1beta2.AutotunePolicy{
						{AutotuneType: "DETACHED_VOLUME"},
						{AutotuneType: "PERFORMANCE_BASED", MaxVPUsPerGB: common.Int64(30)},
					},
				}
			},
			expectedLen: 2,
			expectedPolicies: []core.AutotunePolicy{
				core.DetachedVolumeAutotunePolicy{},
				core.PerformanceBasedAutotunePolicy{MaxVpusPerGB: common.Int64(30)},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms)
			policies := ms.ToOCIAutotunePolicy()
			if tc.expectedLen == 0 {
				g.Expect(policies).To(BeNil())
			} else {
				g.Expect(policies).To(HaveLen(tc.expectedLen))
				g.Expect(policies).To(Equal(tc.expectedPolicies))
			}
		})
	}
}
