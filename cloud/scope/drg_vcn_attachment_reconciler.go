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

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/pkg/errors"
)

// ReconcileDRGVCNAttachment tries to attach the DRG to the VCN
func (s *ClusterScope) ReconcileDRGVCNAttachment(ctx context.Context) error {
	if !s.isPeeringEnabled() {
		s.Logger.Info("VCN Peering is not enabled, ignoring reconciliation")
		return nil
	}

	attachment, err := s.GetDRGAttachment(ctx)
	if err != nil {
		return err
	}

	if attachment != nil {
		s.getDRG().VcnAttachmentId = attachment.Id
		s.Logger.Info("DRG already attached to VCN")
		if err != nil {
			return err
		}
		if !s.IsTagsEqual(attachment.FreeformTags, attachment.DefinedTags) {
			_, err := s.UpdateDRGAttachment(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}

	response, err := s.VCNClient.CreateDrgAttachment(ctx, core.CreateDrgAttachmentRequest{
		CreateDrgAttachmentDetails: core.CreateDrgAttachmentDetails{
			DisplayName:  common.String(s.OCICluster.Name),
			DrgId:        s.getDrgID(),
			VcnId:        s.OCICluster.Spec.NetworkSpec.Vcn.ID,
			FreeformTags: s.GetFreeFormTags(),
			DefinedTags:  s.GetDefinedTags(),
		},
	})
	if err != nil {
		return err
	}
	s.getDRG().VcnAttachmentId = response.Id
	s.Logger.Info("DRG has been attached", "attachmentId", response.Id)
	return nil
}

func (s *ClusterScope) GetDRGAttachment(ctx context.Context) (*core.DrgAttachment, error) {
	if s.getDRG().VcnAttachmentId != nil {
		response, err := s.VCNClient.GetDrgAttachment(ctx, core.GetDrgAttachmentRequest{
			DrgAttachmentId: s.getDRG().VcnAttachmentId,
		})
		if err != nil {
			return nil, err
		}
		if s.IsResourceCreatedByClusterAPI(response.FreeformTags) {
			return &response.DrgAttachment, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}

	attachments, err := s.VCNClient.ListDrgAttachments(ctx, core.ListDrgAttachmentsRequest{
		AttachmentType: core.ListDrgAttachmentsAttachmentTypeVcn,
		DisplayName:    common.String(s.OCICluster.Name),
		DrgId:          s.getDrgID(),
		NetworkId:      s.OCICluster.Spec.NetworkSpec.Vcn.ID,
		CompartmentId:  common.String(s.GetCompartmentId()),
	})

	if err != nil {
		return nil, err
	}

	if len(attachments.Items) == 0 {
		return nil, nil
	} else if len(attachments.Items) > 1 {
		return nil, errors.New("found more than one DRG VCN attachment to same VCN, please remove any " +
			"DRG VCN attachments which has been created outside Cluster API for Oracle for the VCN")
	} else {
		attachment := attachments.Items[0]
		if s.IsResourceCreatedByClusterAPI(attachment.FreeformTags) {
			return &attachment, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
}

func (s *ClusterScope) UpdateDRGAttachment(ctx context.Context) (*core.DrgAttachment, error) {
	response, err := s.VCNClient.UpdateDrgAttachment(ctx, core.UpdateDrgAttachmentRequest{
		DrgAttachmentId: s.getDRG().VcnAttachmentId,
		UpdateDrgAttachmentDetails: core.UpdateDrgAttachmentDetails{
			FreeformTags: s.GetFreeFormTags(),
			DefinedTags:  s.GetDefinedTags(),
		},
	})
	if err != nil {
		return nil, err
	}
	return &response.DrgAttachment, nil
}

func (s *ClusterScope) DeleteDRGVCNAttachment(ctx context.Context) error {
	if !s.isPeeringEnabled() {
		s.Logger.Info("VCN Peering is not enabled, ignoring reconciliation")
		return nil
	}
	attachment, err := s.GetDRGAttachment(ctx)
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if attachment == nil {
		s.Logger.Info("DRG VCN Attachment is already deleted")
		return nil
	}
	_, err = s.VCNClient.DeleteDrgAttachment(ctx, core.DeleteDrgAttachmentRequest{
		DrgAttachmentId: attachment.Id,
	})
	if err != nil {
		return err
	}
	return nil
}
