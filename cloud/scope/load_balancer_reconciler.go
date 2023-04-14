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

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ReconcileApiServerLB tries to move the Load Balancer to the desired OCICluster Spec
func (s *ClusterScope) ReconcileApiServerLB(ctx context.Context) error {
	desiredApiServerLb := s.LBSpec()

	lb, err := s.GetLoadBalancer(ctx)
	if err != nil {
		return err
	}
	if lb != nil {
		lbIP, err := s.getLoadbalancerIp(*lb)
		if err != nil {
			return err
		}
		networkSpec := s.OCIClusterAccessor.GetNetworkSpec()
		networkSpec.APIServerLB.LoadBalancerId = lb.Id
		s.OCIClusterAccessor.SetControlPlaneEndpoint(clusterv1.APIEndpoint{
			Host: *lbIP,
			Port: s.APIServerPort(),
		})
		if s.IsLBEqual(lb, desiredApiServerLb) {
			s.Logger.Info("No Reconciliation Required for ApiServerLB", "lb", lb.Id)
			return nil
		}
		s.Logger.Info("Reconciliation Required for ApiServerLB", "lb", lb.Id)
		return s.UpdateLB(ctx, desiredApiServerLb)
	}
	lbID, lbIP, err := s.CreateLB(ctx, desiredApiServerLb)
	if err != nil {
		return err
	}
	networkSpec := s.OCIClusterAccessor.GetNetworkSpec()
	networkSpec.APIServerLB.LoadBalancerId = lbID
	s.OCIClusterAccessor.SetControlPlaneEndpoint(clusterv1.APIEndpoint{
		Host: *lbIP,
		Port: s.APIServerPort(),
	})
	return err
}

// DeleteApiServerLB retrieves and attempts to delete the Network Load Balancer if found.
// It will await the Work Request completion before returning
func (s *ClusterScope) DeleteApiServerLB(ctx context.Context) error {
	lb, err := s.GetLoadBalancer(ctx)
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if lb == nil {
		s.Logger.Info("loadbalancer is already deleted")
		return nil
	}
	lbResponse, err := s.LoadBalancerClient.DeleteNetworkLoadBalancer(ctx, networkloadbalancer.DeleteNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: lb.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to delete apiserver lb")
		return errors.Wrap(err, "failed to delete apiserver lb")
	}
	_, err = ociutil.AwaitLBWorkRequest(ctx, s.LoadBalancerClient, lbResponse.OpcWorkRequestId)
	if err != nil {
		return errors.Wrap(err, "work request to delete lb failed")
	}
	s.Logger.Info("Successfully deleted apiserver lb", "lb", lb.Id)
	return nil
}

// LBSpec builds the LoadBalancer from the ClusterScope and returns it
func (s *ClusterScope) LBSpec() infrastructurev1beta2.LoadBalancer {
	lbSpec := infrastructurev1beta2.LoadBalancer{
		Name: s.GetControlPlaneLoadBalancerName(),
	}
	return lbSpec
}

// GetControlPlaneLoadBalancerName returns the user defined APIServerLB name from the spec or
// assigns the name based on the OCICluster's name
func (s *ClusterScope) GetControlPlaneLoadBalancerName() string {
	if s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.Name != "" {
		return s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.Name
	}
	return fmt.Sprintf("%s-%s", s.OCIClusterAccessor.GetName(), "apiserver")
}

// UpdateLB updates existing Load Balancer's DisplayName, FreeformTags and DefinedTags
func (s *ClusterScope) UpdateLB(ctx context.Context, lb infrastructurev1beta2.LoadBalancer) error {
	lbId := s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId
	updateLBDetails := networkloadbalancer.UpdateNetworkLoadBalancerDetails{
		DisplayName: common.String(lb.Name),
	}
	lbResponse, err := s.LoadBalancerClient.UpdateNetworkLoadBalancer(ctx, networkloadbalancer.UpdateNetworkLoadBalancerRequest{
		UpdateNetworkLoadBalancerDetails: updateLBDetails,
		NetworkLoadBalancerId:            lbId,
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the apiserver LB, failed to generate update lb workrequest")
		return errors.Wrap(err, "failed to reconcile the apiserver LB, failed to generate update lb workrequest")
	}
	_, err = ociutil.AwaitLBWorkRequest(ctx, s.LoadBalancerClient, lbResponse.OpcWorkRequestId)
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the apiserver LB, failed to update lb")
		return errors.Wrap(err, "failed to reconcile the apiserver LB, failed to update lb")
	}
	return nil
}

// CreateLB configures and creates the Network Load Balancer for the cluster based on the ClusterScope.
// This configures the LB Listeners and Backend Sets in order to create the Network Load Balancer.
// It will await the Work Request completion before returning
//
// See https://docs.oracle.com/en-us/iaas/Content/NetworkLoadBalancer/overview.htm for more details on the Network
// Load Balancer
func (s *ClusterScope) CreateLB(ctx context.Context, lb infrastructurev1beta2.LoadBalancer) (*string, *string, error) {
	listenerDetails := make(map[string]networkloadbalancer.ListenerDetails)
	listenerDetails[APIServerLBListener] = networkloadbalancer.ListenerDetails{
		Protocol:              networkloadbalancer.ListenerProtocolsTcp,
		Port:                  common.Int(int(s.APIServerPort())),
		DefaultBackendSetName: common.String(APIServerLBBackendSetName),
		Name:                  common.String(APIServerLBListener),
	}

	backendSetDetails := make(map[string]networkloadbalancer.BackendSetDetails)
	backendSetDetails[APIServerLBBackendSetName] = networkloadbalancer.BackendSetDetails{
		Policy:           LoadBalancerPolicy,
		IsPreserveSource: common.Bool(false),
		HealthChecker: &networkloadbalancer.HealthChecker{
			Port:       common.Int(int(s.APIServerPort())),
			Protocol:   networkloadbalancer.HealthCheckProtocolsHttps,
			UrlPath:    common.String("/healthz"),
			ReturnCode: common.Int(200),
		},
		Backends: []networkloadbalancer.Backend{},
	}

	var controlPlaneEndpointSubnets []string
	for _, subnet := range s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets {
		if subnet.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			controlPlaneEndpointSubnets = append(controlPlaneEndpointSubnets, *subnet.ID)
		}
	}
	if len(controlPlaneEndpointSubnets) < 1 {
		return nil, nil, errors.New("control plane endpoint subnet not provided")
	}

	if len(controlPlaneEndpointSubnets) > 1 {
		return nil, nil, errors.New("cannot have more than 1 control plane endpoint subnet")
	}
	lbDetails := networkloadbalancer.CreateNetworkLoadBalancerDetails{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(lb.Name),
		SubnetId:      common.String(controlPlaneEndpointSubnets[0]),
		IsPrivate:     common.Bool(s.isControlPlaneEndpointSubnetPrivate()),
		Listeners:     listenerDetails,
		BackendSets:   backendSetDetails,
		FreeformTags:  s.GetFreeFormTags(),
		DefinedTags:   s.GetDefinedTags(),
	}

	for _, nsg := range s.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List {
		if nsg.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			if nsg.ID != nil {
				lbDetails.NetworkSecurityGroupIds = []string{*nsg.ID}
			}
		}
	}

	s.Logger.Info("Creating network load balancer")
	lbResponse, err := s.LoadBalancerClient.CreateNetworkLoadBalancer(ctx, networkloadbalancer.CreateNetworkLoadBalancerRequest{
		CreateNetworkLoadBalancerDetails: lbDetails,
		OpcRetryToken:                    ociutil.GetOPCRetryToken("%s-%s", "create-lb", s.OCIClusterAccessor.GetOCIResourceIdentifier()),
	})
	if err != nil {
		s.Logger.Error(err, "failed to create apiserver lb, failed to create work request")
		return nil, nil, errors.Wrap(err, "failed to create apiserver lb, failed to create work request")
	}
	_, err = ociutil.AwaitLBWorkRequest(ctx, s.LoadBalancerClient, lbResponse.OpcWorkRequestId)
	if err != nil {
		return nil, nil, errors.Wrap(err, "awaiting load balancer")
	}

	nlb, err := s.LoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: lbResponse.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to get apiserver lb after creation")
		return nil, nil, errors.Wrap(err, "failed to get apiserver lb after creation")
	}

	lbIp, err := s.getLoadbalancerIp(nlb.NetworkLoadBalancer)
	if err != nil {
		return nil, nil, err
	}

	s.Logger.Info("Successfully created apiserver lb", "lb", nlb.Id, "ip", lbIp)
	return nlb.Id, lbIp, nil
}

func (s *ClusterScope) getLoadbalancerIp(nlb networkloadbalancer.NetworkLoadBalancer) (*string, error) {
	var lbIp *string
	if len(nlb.IpAddresses) < 1 {
		return nil, errors.New("lb does not have valid ip addresses")
	}
	if *nlb.IsPrivate {
		lbIp = nlb.IpAddresses[0].IpAddress
	} else {
		for _, ip := range nlb.IpAddresses {
			if *ip.IsPublic {
				lbIp = ip.IpAddress
			}
		}
	}
	if lbIp == nil {
		return nil, errors.New("lb does not have valid public ip address")
	}
	return lbIp, nil
}

// IsLBEqual determines if the actual networkloadbalancer.NetworkLoadBalancer is equal to the desired.
// Equality is determined by DisplayName
func (s *ClusterScope) IsLBEqual(actual *networkloadbalancer.NetworkLoadBalancer, desired infrastructurev1beta2.LoadBalancer) bool {
	if desired.Name != *actual.DisplayName {
		return false
	}
	return true
}

// GetLoadBalancer retrieves the Cluster's networkloadbalancer.NetworkLoadBalancer using the one of the following methods
//
// 1. the OCICluster's spec LoadBalancerId
//
// 2. Listing the NetworkLoadBalancers for the Compartment (by ID) and DisplayName then filtering by tag
func (s *ClusterScope) GetLoadBalancer(ctx context.Context) (*networkloadbalancer.NetworkLoadBalancer, error) {
	lbOcid := s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId
	if lbOcid != nil {
		resp, err := s.LoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: lbOcid,
		})
		if err != nil {
			return nil, err
		}
		nlb := resp.NetworkLoadBalancer
		if s.IsResourceCreatedByClusterAPI(nlb.FreeformTags) {
			return &nlb, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	var page *string;
	for  {
		lbs, err := s.LoadBalancerClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
			CompartmentId: common.String(s.GetCompartmentId()),
			DisplayName:   common.String(s.GetControlPlaneLoadBalancerName()),
			Page: page,
		})
		if err != nil {
			s.Logger.Error(err, "Failed to list lb by name")
			return nil, errors.Wrap(err, "failed to list lb by name")
		}

		for _, lb := range lbs.Items {
			if s.IsResourceCreatedByClusterAPI(lb.FreeformTags) {
				resp, err := s.LoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: lb.Id,
				})
				if err != nil {
					return nil, err
				}
				return &resp.NetworkLoadBalancer, nil
			}
		}

		if lbs.OpcNextPage == nil {
			break
		} else {
			page = lbs.OpcNextPage
		}
	}
	return nil, nil
}
