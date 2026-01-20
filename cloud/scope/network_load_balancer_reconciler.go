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
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil/ptr"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// ReconcileApiServerNLB tries to move the Network Load Balancer to the desired OCICluster Spec
func (s *ClusterScope) ReconcileApiServerNLB(ctx context.Context) error {
	desiredApiServerNLB := s.NLBSpec()

	nlb, err := s.GetNetworkLoadBalancers(ctx)
	if err != nil {
		return err
	}
	if nlb != nil {
		if nlb.LifecycleState != networkloadbalancer.LifecycleStateActive {
			return errors.New(fmt.Sprintf("network load balancer is in %s state. Waiting for ACTIVE state.", nlb.LifecycleState))
		}
		lbIP, err := s.getNetworkLoadbalancerIp(*nlb)
		if err != nil {
			return err
		}
		networkSpec := s.OCIClusterAccessor.GetNetworkSpec()
		networkSpec.APIServerLB.LoadBalancerId = nlb.Id
		s.OCIClusterAccessor.SetControlPlaneEndpoint(clusterv1.APIEndpoint{
			Host: *lbIP,
			Port: s.APIServerPort(),
		})
		if s.IsNLBEqual(nlb, desiredApiServerNLB) {
			s.Logger.Info("No Reconciliation Required for ApiServerLB", "nlb", nlb.Id)
			return nil
		}
		s.Logger.Info("Reconciliation Required for ApiServerLB", "nlb", nlb.Id)
		return s.UpdateNLB(ctx, desiredApiServerNLB)
	}
	nlbID, nlbIP, err := s.CreateNLB(ctx, desiredApiServerNLB)
	if err != nil {
		return err
	}
	networkSpec := s.OCIClusterAccessor.GetNetworkSpec()
	networkSpec.APIServerLB.LoadBalancerId = nlbID
	s.OCIClusterAccessor.SetControlPlaneEndpoint(clusterv1.APIEndpoint{
		Host: *nlbIP,
		Port: s.APIServerPort(),
	})
	return err
}

// DeleteApiServerNLB retrieves and attempts to delete the Network Load Balancer if found.
// It will await the Work Request completion before returning
func (s *ClusterScope) DeleteApiServerNLB(ctx context.Context) error {
	nlb, err := s.GetNetworkLoadBalancers(ctx)
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if nlb == nil {
		s.Logger.Info("network loadbalancer is already deleted")
		return nil
	}
	lbResponse, err := s.NetworkLoadBalancerClient.DeleteNetworkLoadBalancer(ctx, networkloadbalancer.DeleteNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: nlb.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to delete apiserver nlb")
		return errors.Wrap(err, "failed to delete apiserver nlb")
	}
	_, err = ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, lbResponse.OpcWorkRequestId)
	if err != nil {
		return errors.Wrap(err, "work request to delete nlb failed")
	}
	s.Logger.Info("Successfully deleted apiserver nlb", "nlb", nlb.Id)
	return nil
}

// NLBSpec builds the Network LoadBalancer from the ClusterScope and returns it
func (s *ClusterScope) NLBSpec() infrastructurev1beta2.LoadBalancer {
	nlbSpec := infrastructurev1beta2.LoadBalancer{
		Name:    s.GetControlPlaneLoadBalancerName(),
		NLBSpec: s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.NLBSpec,
	}
	return nlbSpec
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
func (s *ClusterScope) UpdateNLB(ctx context.Context, nlb infrastructurev1beta2.LoadBalancer) error {
	nlbId := s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId
	updateLBDetails := networkloadbalancer.UpdateNetworkLoadBalancerDetails{
		DisplayName: common.String(nlb.Name),
	}
	nlbResponse, err := s.NetworkLoadBalancerClient.UpdateNetworkLoadBalancer(ctx, networkloadbalancer.UpdateNetworkLoadBalancerRequest{
		UpdateNetworkLoadBalancerDetails: updateLBDetails,
		NetworkLoadBalancerId:            nlbId,
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the apiserver NLB, failed to generate update nlb workrequest")
		return errors.Wrap(err, "failed to reconcile the apiserver NLB, failed to generate update nlb workrequest")
	}
	_, err = ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, nlbResponse.OpcWorkRequestId)
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the apiserver NLB, failed to update nlb")
		return errors.Wrap(err, "failed to reconcile the apiserver NLB, failed to update nlb")
	}
	return nil
}

// CreateNLB configures and creates the Network Load Balancer for the cluster based on the ClusterScope.
// This configures the LB Listeners and Backend Sets in order to create the Network Load Balancer.
// It will await the Work Request completion before returning
//
// See https://docs.oracle.com/en-us/iaas/Content/NetworkLoadBalancer/overview.htm for more details on the Network
// Load Balancer
func (s *ClusterScope) CreateNLB(ctx context.Context, lb infrastructurev1beta2.LoadBalancer) (*string, *string, error) {
	isPreserverSourceIp := lb.NLBSpec.BackendSetDetails.IsPreserveSource
	if isPreserverSourceIp == nil {
		isPreserverSourceIp = common.Bool(false)
	}
	listenerDetails := make(map[string]networkloadbalancer.ListenerDetails)
	listenerDetails[APIServerLBListener] = networkloadbalancer.ListenerDetails{
		Protocol:              networkloadbalancer.ListenerProtocolsTcp,
		Port:                  common.Int(int(s.APIServerPort())),
		DefaultBackendSetName: common.String(APIServerLBBackendSetName),
		Name:                  common.String(APIServerLBListener),
	}

	backendSetDetails := make(map[string]networkloadbalancer.BackendSetDetails)
	healthCheckUrl := lb.NLBSpec.BackendSetDetails.HealthChecker.UrlPath
	if healthCheckUrl == nil {
		healthCheckUrl = common.String("/healthz")
	}
	backendSetDetails[APIServerLBBackendSetName] = networkloadbalancer.BackendSetDetails{
		Policy:                   LoadBalancerPolicy,
		IsPreserveSource:         isPreserverSourceIp,
		IsFailOpen:               lb.NLBSpec.BackendSetDetails.IsFailOpen,
		IsInstantFailoverEnabled: lb.NLBSpec.BackendSetDetails.IsInstantFailoverEnabled,
		HealthChecker: &networkloadbalancer.HealthChecker{
			Port:       common.Int(int(s.APIServerPort())),
			Protocol:   networkloadbalancer.HealthCheckProtocolsHttps,
			UrlPath:    healthCheckUrl,
			ReturnCode: common.Int(200),
		},
		Backends: []networkloadbalancer.Backend{},
	}

	var controlPlaneEndpointSubnets []string
	for _, subnet := range ptr.ToSubnetSlice(s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets) {
		if subnet.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			if subnet.ID != nil {
				controlPlaneEndpointSubnets = append(controlPlaneEndpointSubnets, *subnet.ID)
			}
		}
	}
	var reservedIps []networkloadbalancer.ReservedIp
	if len(lb.NLBSpec.ReservedIpIds) > 0 {
		// since max is one we only take the first ip id supplied
		reservedIps = append(reservedIps, networkloadbalancer.ReservedIp{Id: common.String(lb.NLBSpec.ReservedIpIds[0])})
	}

	if len(controlPlaneEndpointSubnets) < 1 {
		return nil, nil, errors.New("control plane endpoint subnet not provided")
	}

	if len(controlPlaneEndpointSubnets) > 1 {
		return nil, nil, errors.New("cannot have more than 1 control plane endpoint subnet")
	}
	nlbDetails := networkloadbalancer.CreateNetworkLoadBalancerDetails{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(lb.Name),
		SubnetId:      common.String(controlPlaneEndpointSubnets[0]),
		IsPrivate:     common.Bool(s.isControlPlaneEndpointSubnetPrivate()),
		Listeners:     listenerDetails,
		BackendSets:   backendSetDetails,
		FreeformTags:  s.GetFreeFormTags(),
		DefinedTags:   s.GetDefinedTags(),
		ReservedIps:   reservedIps,
	}
	nsgs := make([]string, 0)
	for _, nsg := range ptr.ToNSGSlice(s.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List) {
		if nsg.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			if nsg.ID != nil {
				nsgs = append(nsgs, *nsg.ID)
			}
		}
	}
	nlbDetails.NetworkSecurityGroupIds = nsgs

	s.Logger.Info("Creating network load balancer")
	nlbResponse, err := s.NetworkLoadBalancerClient.CreateNetworkLoadBalancer(ctx, networkloadbalancer.CreateNetworkLoadBalancerRequest{
		CreateNetworkLoadBalancerDetails: nlbDetails,
		OpcRetryToken:                    ociutil.GetOPCRetryToken("%s-%s", "create-nlb", s.OCIClusterAccessor.GetOCIResourceIdentifier()),
	})
	if err != nil {
		s.Logger.Error(err, "failed to create apiserver nlb, failed to create work request")
		return nil, nil, errors.Wrap(err, "failed to create apiserver nlb, failed to create work request")
	}
	_, err = ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, nlbResponse.OpcWorkRequestId)
	if err != nil {
		return nil, nil, errors.Wrap(err, "awaiting network load balancer")
	}

	nlb, err := s.NetworkLoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: nlbResponse.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to get apiserver lb after creation")
		return nil, nil, errors.Wrap(err, "failed to get apiserver lb after creation")
	}

	nlbIp, err := s.getNetworkLoadbalancerIp(nlb.NetworkLoadBalancer)
	if err != nil {
		return nil, nil, err
	}

	s.Logger.Info("Successfully created apiserver lb", "lb", nlb.Id, "ip", nlbIp)
	return nlb.Id, nlbIp, nil
}

func (s *ClusterScope) getNetworkLoadbalancerIp(nlb networkloadbalancer.NetworkLoadBalancer) (*string, error) {
	var nlbIp *string
	if len(nlb.IpAddresses) < 1 {
		return nil, errors.New("nlb does not have valid ip addresses")
	}
	if ptr.ToBool(nlb.IsPrivate) {
		nlbIp = nlb.IpAddresses[0].IpAddress
	} else {
		for _, ip := range nlb.IpAddresses {
			if *ip.IsPublic {
				nlbIp = ip.IpAddress
			}
		}
	}
	if nlbIp == nil {
		return nil, errors.New("nlb does not have valid public ip address")
	}
	return nlbIp, nil
}

// IsNLBEqual determines if the actual networkloadbalancer.NetworkLoadBalancer is equal to the desired.
// Equality is determined by DisplayName
func (s *ClusterScope) IsNLBEqual(actual *networkloadbalancer.NetworkLoadBalancer, desired infrastructurev1beta2.LoadBalancer) bool {
	if desired.Name != *actual.DisplayName {
		return false
	}
	return true
}

// GetNetworkLoadBalancers retrieves the Cluster's networkloadbalancer.NetworkLoadBalancer using the one of the following methods
//
// 1. the OCICluster's spec LoadBalancerId
//
// 2. Listing the NetworkLoadBalancers for the Compartment (by ID) and DisplayName then filtering by tag
// nolint:nilnil
func (s *ClusterScope) GetNetworkLoadBalancers(ctx context.Context) (*networkloadbalancer.NetworkLoadBalancer, error) {
	nlbOcid := s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId
	if nlbOcid != nil {
		resp, err := s.NetworkLoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: nlbOcid,
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
	nlbs, err := s.NetworkLoadBalancerClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(s.GetControlPlaneLoadBalancerName()),
	})
	if err != nil {
		s.Logger.Error(err, "Failed to list nlb by name")
		return nil, errors.Wrap(err, "failed to list nlb by name")
	}

	for _, nlb := range nlbs.Items {
		if s.IsResourceCreatedByClusterAPI(nlb.FreeformTags) {
			resp, err := s.NetworkLoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
				NetworkLoadBalancerId: nlb.Id,
			})
			if err != nil {
				return nil, err
			}
			return &resp.NetworkLoadBalancer, nil
		}
	}
	return nil, nil
}
