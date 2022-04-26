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

	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/pkg/errors"
)

func (s *ClusterScope) ReconcileVCN(ctx context.Context) error {
	desiredVCN := s.VCNSpec()

	var err error
	vcn, err := s.GetVCN(ctx)
	if err != nil {
		return err
	}
	if vcn != nil {
		s.OCICluster.Spec.NetworkSpec.Vcn.ID = vcn.Id
		if s.IsVcnEquals(vcn, desiredVCN) {
			s.Logger.Info("No Reconciliation Required for VCN", "vcn", s.getVcnId())
			return nil
		}
		return s.UpdateVCN(ctx, desiredVCN)
	}
	vcnId, err := s.CreateVCN(ctx, desiredVCN)
	s.OCICluster.Spec.NetworkSpec.Vcn.ID = vcnId
	return err
}

func (s *ClusterScope) IsVcnEquals(actual *core.Vcn, desired infrastructurev1beta1.VCN) bool {
	if *actual.DisplayName == desired.Name && s.IsTagsEqual(actual.FreeformTags, actual.DefinedTags) {
		return true
	}
	return false
}

func (s *ClusterScope) GetVcnName() string {
	if s.OCICluster.Spec.NetworkSpec.Vcn.Name != "" {
		return s.OCICluster.Spec.NetworkSpec.Vcn.Name
	}
	return fmt.Sprintf("%s", s.OCICluster.Name)
}

func (s *ClusterScope) GetVcnCidr() string {
	if s.OCICluster.Spec.NetworkSpec.Vcn.CIDR != "" {
		return s.OCICluster.Spec.NetworkSpec.Vcn.CIDR
	}
	return VcnDefaultCidr
}

func (s *ClusterScope) VCNSpec() infrastructurev1beta1.VCN {
	vcnSpec := infrastructurev1beta1.VCN{
		Name: s.GetVcnName(),
		CIDR: s.GetVcnCidr(),
	}
	return vcnSpec
}

func (s *ClusterScope) GetVCN(ctx context.Context) (*core.Vcn, error) {
	vcnId := s.getVcnId()
	if vcnId != nil {
		resp, err := s.VCNClient.GetVcn(ctx, core.GetVcnRequest{
			VcnId: vcnId,
		})
		if err != nil {
			return nil, err
		}
		vcn := resp.Vcn
		if s.IsResourceCreatedByClusterAPI(vcn.FreeformTags) {
			return &vcn, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	vcns, err := s.VCNClient.ListVcns(ctx, core.ListVcnsRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(s.GetVcnName()),
	})
	if err != nil {
		s.Logger.Error(err, "failed to list vcn by name")
		return nil, errors.Wrap(err, "failed to list vcn by name")
	}

	for _, vcn := range vcns.Items {
		if s.IsResourceCreatedByClusterAPI(vcn.FreeformTags) {
			return &vcn, nil
		}
	}
	return nil, nil
}

func (s *ClusterScope) UpdateVCN(ctx context.Context, vcn infrastructurev1beta1.VCN) error {
	updateVCNDetails := core.UpdateVcnDetails{
		DisplayName:  common.String(vcn.Name),
		DefinedTags:  s.GetDefinedTags(),
		FreeformTags: s.GetFreeFormTags(),
	}
	vcnResponse, err := s.VCNClient.UpdateVcn(ctx, core.UpdateVcnRequest{
		UpdateVcnDetails: updateVCNDetails,
		VcnId:            s.getVcnId(),
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the vcn, failed to update")
		return errors.Wrap(err, "failed to reconcile the vcn, failed to update")
	}
	s.Logger.Info("successfully updated the vcn", "vcn", *vcnResponse.Id)
	return nil
}

func (s *ClusterScope) CreateVCN(ctx context.Context, spec infrastructurev1beta1.VCN) (*string, error) {
	vcnDetails := core.CreateVcnDetails{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(spec.Name),
		CidrBlocks:    []string{spec.CIDR},
		FreeformTags:  s.GetFreeFormTags(),
		DefinedTags:   s.GetDefinedTags(),
	}
	vcnResponse, err := s.VCNClient.CreateVcn(ctx, core.CreateVcnRequest{
		CreateVcnDetails: vcnDetails,
		OpcRetryToken:    ociutil.GetOPCRetryToken("%s-%s", "create-vcn", string(s.OCICluster.GetOCIResourceIdentifier())),
	})
	if err != nil {
		s.Logger.Error(err, "failed create vcn")
		return nil, errors.Wrap(err, "failed create vcn")
	}
	s.Logger.Info("successfully created the vcn", "vcn", *vcnResponse.Vcn.Id)
	return vcnResponse.Vcn.Id, nil
}

func (s *ClusterScope) DeleteVCN(ctx context.Context) error {
	vcn, err := s.GetVCN(ctx)

	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if vcn == nil {
		s.Logger.Info("VCN is already deleted")
		return nil
	}
	_, err = s.VCNClient.DeleteVcn(ctx, core.DeleteVcnRequest{
		VcnId: vcn.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to delete vcn")
		return errors.Wrap(err, "failed to delete vcn")
	}
	s.Logger.Info("Successfully deleted VCN")
	return nil
}

func (s *ClusterScope) getVcnId() *string {
	return s.OCICluster.Spec.NetworkSpec.Vcn.ID
}
