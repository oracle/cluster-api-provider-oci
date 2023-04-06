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
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute/mock_compute"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileVnicAttachment(t *testing.T) {
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
					CompartmentId: "testCompartment",
					VnicAttachments: []infrastructurev1beta2.VnicAttachment{
						{
							DisplayName: common.String("VnicTest"),
							NicIndex:    common.Int(0),
						},
					},
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
			name:          "Crete vnic attachment",
			errorExpected: false,
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = common.String("test")
				computeClient.EXPECT().ListVnicAttachments(gomock.Any(), gomock.Eq(core.ListVnicAttachmentsRequest{
					InstanceId:    common.String("test"),
					CompartmentId: common.String("testCompartment"),
				})).
					Return(core.ListVnicAttachmentsResponse{
						Items: []core.VnicAttachment{
							{
								InstanceId:  common.String("test"),
								DisplayName: common.String("vnicDisplayName"),
							},
							{
								InstanceId:  common.String("test"),
								DisplayName: common.String("vnicDisplayName"),
							},
						},
					}, nil)

				computeClient.EXPECT().AttachVnic(gomock.Any(), gomock.Eq(core.AttachVnicRequest{
					AttachVnicDetails: core.AttachVnicDetails{
						DisplayName: common.String("VnicTest"),
						NicIndex:    common.Int(0),
						InstanceId:  common.String("test"),
						CreateVnicDetails: &core.CreateVnicDetails{
							DisplayName:    common.String("VnicTest"),
							AssignPublicIp: common.Bool(false),
							DefinedTags:    map[string]map[string]interface{}{},
							FreeformTags: map[string]string{
								ociutil.CreatedBy:                 ociutil.OCIClusterAPIProvider,
								ociutil.ClusterResourceIdentifier: "resource_uid",
							},
							NsgIds: make([]string, 0),
						},
					}})).
					Return(core.AttachVnicResponse{
						VnicAttachment: core.VnicAttachment{Id: common.String("vnic.id")},
					}, nil)
			},
		},
		{
			name:          "Crete vnic attachment error",
			errorExpected: true,
			matchError:    fmt.Errorf("could not attach to nic 10"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.OCIMachine.Spec.InstanceId = common.String("test")
				computeClient.EXPECT().ListVnicAttachments(gomock.Any(), gomock.Eq(core.ListVnicAttachmentsRequest{
					InstanceId:    common.String("test"),
					CompartmentId: common.String("testCompartment"),
				})).
					Return(core.ListVnicAttachmentsResponse{
						Items: []core.VnicAttachment{
							{
								InstanceId:  common.String("test"),
								DisplayName: common.String("vnicDisplayName"),
								NicIndex:    common.Int(10),
							},
						},
					}, nil)

				computeClient.EXPECT().AttachVnic(gomock.Any(), gomock.Any()).
					Return(core.AttachVnicResponse{}, errors.New("could not attach to nic 10"))
			},
		},
		{
			name:          "Crete vnic attachment on control plane will fail",
			errorExpected: true,
			matchError:    fmt.Errorf("cannot attach multiple vnics to ControlPlane machines"),
			testSpecificSetup: func(machineScope *MachineScope, computeClient *mock_compute.MockComputeClient) {
				ms.Machine.ObjectMeta.Labels = make(map[string]string)
				ms.Machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel] = "Test"
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer teardown(t, g)
			setup(t, g)
			tc.testSpecificSetup(ms, computeClient)
			err := ms.ReconcileVnicAttachments(context.Background())
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
