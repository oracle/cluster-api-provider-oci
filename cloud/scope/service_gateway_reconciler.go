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
	"strings"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/pkg/errors"
)

func (s *ClusterScope) ReconcileServiceGateway(ctx context.Context) error {
	if s.IsAllSubnetsPublic() {
		s.Logger.Info("All subnets are public, we don't need service gateway")
		return nil
	}
	var err error
	sgw, err := s.GetServiceGateway(ctx)
	if err != nil {
		return err
	}
	if sgw != nil {
		s.OCICluster.Spec.NetworkSpec.Vcn.ServiceGatewayId = sgw.Id
		if !s.IsTagsEqual(sgw.FreeformTags, sgw.DefinedTags) {
			return s.UpdateServiceGateway(ctx)
		}
		s.Logger.Info("No Reconciliation Required for Service Gateway", "service_gateway", sgw.Id)
		return nil
	}
	serviceGateway, err := s.CreateServiceGateway(ctx)
	s.OCICluster.Spec.NetworkSpec.Vcn.ServiceGatewayId = serviceGateway
	return err
}

func (s *ClusterScope) UpdateServiceGateway(ctx context.Context) error {
	updateSGWDetails := core.UpdateServiceGatewayDetails{
		FreeformTags: s.GetFreeFormTags(),
		DefinedTags:  s.GetDefinedTags(),
	}
	sgwResponse, err := s.VCNClient.UpdateServiceGateway(ctx, core.UpdateServiceGatewayRequest{
		ServiceGatewayId:            s.OCICluster.Spec.NetworkSpec.Vcn.ServiceGatewayId,
		UpdateServiceGatewayDetails: updateSGWDetails,
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the service gateway, failed to update")
		return errors.Wrap(err, "failed to reconcile the service gateway, failed to update")
	}
	s.Logger.Info("successfully updated the service gateway", "service_gateway", *sgwResponse.Id)
	return nil
}

func (s *ClusterScope) CreateServiceGateway(ctx context.Context) (*string, error) {
	var serviceOcid string
	var isServiceFound bool
	listServicesResponse, err := s.VCNClient.ListServices(ctx, core.ListServicesRequest{})
	if err != nil {
		s.Logger.Error(err, "failed to get the list of services")
		return nil, errors.Wrap(err, "failed to get the list of services")
	}
	for _, service := range listServicesResponse.Items {
		if strings.HasSuffix(*service.CidrBlock, SGWServiceSuffix) {
			serviceOcid = *service.Id
			isServiceFound = true
			break
		}
	}
	if !isServiceFound {
		s.Logger.Error(err, "failed to get the services ocid")
		return nil, errors.Wrap(err, "failed to get the services ocid")
	}

	sgwDetails := core.CreateServiceGatewayDetails{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(ServiceGatewayName),
		VcnId:         s.getVcnId(),
		Services:      []core.ServiceIdRequestDetails{{ServiceId: common.String(serviceOcid)}},
		FreeformTags:  s.GetFreeFormTags(),
		DefinedTags:   s.GetDefinedTags(),
	}
	sgwResponse, err := s.VCNClient.CreateServiceGateway(ctx, core.CreateServiceGatewayRequest{
		CreateServiceGatewayDetails: sgwDetails,
	})
	if err != nil {
		s.Logger.Error(err, "failed create service gateway")
		return nil, errors.Wrap(err, "failed create service gateway")
	}
	s.Logger.Info("successfully created the service gateway", "ngw", *sgwResponse.Id)
	return sgwResponse.Id, nil
}

func (s *ClusterScope) DeleteServiceGateway(ctx context.Context) error {
	sgw, err := s.GetServiceGateway(ctx)
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if sgw == nil {
		s.Logger.Info("Service Gateway is already deleted")
		return nil
	}
	_, err = s.VCNClient.DeleteServiceGateway(ctx, core.DeleteServiceGatewayRequest{
		ServiceGatewayId: sgw.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to delete ServiceGateways")
		return errors.Wrap(err, "failed to delete ServiceGateways")
	}
	s.Logger.Info("successfully deleted ServiceGateway", "subnet", *sgw.Id)
	return nil
}

func (s *ClusterScope) GetServiceGateway(ctx context.Context) (*core.ServiceGateway, error) {
	sgwId := s.OCICluster.Spec.NetworkSpec.Vcn.ServiceGatewayId
	if sgwId != nil {
		resp, err := s.VCNClient.GetServiceGateway(ctx, core.GetServiceGatewayRequest{
			ServiceGatewayId: sgwId,
		})
		if err != nil {
			return nil, err
		}
		sgw := resp.ServiceGateway
		if s.IsResourceCreatedByClusterAPI(sgw.FreeformTags) {
			return &sgw, err
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	sgws, err := s.VCNClient.ListServiceGateways(ctx, core.ListServiceGatewaysRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		VcnId:         s.getVcnId(),
	})
	if err != nil {
		s.Logger.Error(err, "failed to list Service gateways")
		return nil, errors.Wrap(err, "failed to list Service gateways")
	}
	for _, sgw := range sgws.Items {
		if *sgw.DisplayName == ServiceGatewayName {
			if s.IsResourceCreatedByClusterAPI(sgw.FreeformTags) {
				return &sgw, nil
			}
		}
	}
	return nil, nil
}
