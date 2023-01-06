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
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"

	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/pkg/errors"
)

func (m *MachineScope) ReconcileVnicAttachments(ctx context.Context) error {
	if m.IsControlPlane() {
		return errors.New("cannot attach multiple vnics to ControlPlane machines")
	}

	for index, vnicAttachment := range m.OCIMachine.Spec.VnicAttachments {
		if m.vnicAttachmentExists(ctx, vnicAttachment) {
			m.Logger.Info("vnicAttachment", ociutil.DerefString(vnicAttachment.DisplayName), " already exists and is immutable")
			continue
		}

		vnicId, err := m.createVnicAttachment(ctx, vnicAttachment)
		if err != nil {
			msg := fmt.Sprintf("Error creating VnicAttachment %s for cluster %s",
				*vnicAttachment.DisplayName, m.Cluster.Name)
			m.Logger.Error(err, msg)
			return err
		}

		m.OCIMachine.Spec.VnicAttachments[index].VnicAttachmentId = vnicId
	}

	return nil
}

func (m *MachineScope) createVnicAttachment(ctx context.Context, spec infrastructurev1beta1.VnicAttachment) (*string, error) {
	vnicName := spec.DisplayName

	// Default to machine subnet if spec doesn't supply one
	subnetId := m.getWorkerMachineSubnet()
	if spec.SubnetName != "" {
		var err error
		subnetId, err = m.getMachineSubnet(spec.SubnetName)
		if err != nil {
			return nil, err
		}
	}

	tags := m.getFreeFormTags(*m.OCICluster)

	definedTags := ConvertMachineDefinedTags(m.OCIMachine.Spec.DefinedTags)

	if spec.NicIndex == nil {
		spec.NicIndex = common.Int(0)
	}

	secondVnic := core.AttachVnicDetails{
		DisplayName: vnicName,
		NicIndex:    spec.NicIndex,
		InstanceId:  m.OCIMachine.Spec.InstanceId,
		CreateVnicDetails: &core.CreateVnicDetails{
			SubnetId:               subnetId,
			AssignPublicIp:         common.Bool(spec.AssignPublicIp),
			FreeformTags:           tags,
			DefinedTags:            definedTags,
			HostnameLabel:          m.OCIMachine.Spec.NetworkDetails.HostnameLabel,
			NsgIds:                 m.getWorkerMachineNSGs(),
			SkipSourceDestCheck:    m.OCIMachine.Spec.NetworkDetails.SkipSourceDestCheck,
			AssignPrivateDnsRecord: m.OCIMachine.Spec.NetworkDetails.AssignPrivateDnsRecord,
			DisplayName:            vnicName,
		},
	}

	req := core.AttachVnicRequest{AttachVnicDetails: secondVnic}
	resp, err := m.ComputeClient.AttachVnic(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Id, nil
}

func (m *MachineScope) vnicAttachmentExists(ctx context.Context, vnic infrastructurev1beta1.VnicAttachment) bool {

	found := false
	var page *string
	for {
		resp, err := m.ComputeClient.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
			InstanceId:    m.GetInstanceId(),
			CompartmentId: common.String(m.getCompartmentId()),
			Page:          page,
		})
		if err != nil {
			return false
		}
		for _, attachment := range resp.Items {
			if ociutil.DerefString(attachment.DisplayName) == ociutil.DerefString(vnic.DisplayName) {
				m.Logger.Info("Vnic is already attached ", attachment)
				return true
			}
		}

		if resp.OpcNextPage == nil {
			break
		} else {
			page = resp.OpcNextPage
		}
	}
	return found
}
