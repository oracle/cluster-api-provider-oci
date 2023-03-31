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

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ReconcileApiServerLB tries to move the Load Balancer to the desired OCICluster Spec
func (s *ClusterScope) ReconcileApiServerLbsLB(ctx context.Context) error {
	desiredApiServerLb := s.LbsLBSpec()

	lb, err := s.GetLbsLoadBalancer(ctx)
	if err != nil {
		return err
	}
	if lb != nil {
		lbIP, err := s.getLbsLoadbalancerIp(*lb)
		if err != nil {
			return err
		}
		networkSpec := s.OCIClusterAccessor.GetNetworkSpec()
		networkSpec.APIServerLB.LoadBalancerId = lb.Id
		s.OCIClusterAccessor.SetControlPlaneEndpoint(clusterv1.APIEndpoint{
			Host: *lbIP,
			Port: s.APIServerPort(),
		})
		if s.IsLbsLBEqual(lb, desiredApiServerLb) {
			s.Logger.Info("No Reconciliation Required for ApiServerLB", "lb", lb.Id)
			return nil
		}
		s.Logger.Info("Reconciliation Required for ApiServerLB", "lb", lb.Id)
		return s.UpdateLbsLB(ctx, desiredApiServerLb)
	}
	lbID, lbIP, err := s.CreateLbsLB(ctx, desiredApiServerLb)
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

// DeleteApiServerLbsLB retrieves and attempts to delete the Load Balancer if found.
// It will await the Work Request completion before returning
func (s *ClusterScope) DeleteApiServerLbsLB(ctx context.Context) error {
	lb, err := s.GetLbsLoadBalancer(ctx)
	if err != nil && !ociutil.IsNotFound(err) {
		return err
	}
	if lb == nil {
		s.Logger.Info("loadbalancer is already deleted")
		return nil
	}
	lbResponse, err := s.LoadBalancerServiceClient.DeleteLoadBalancer(ctx, loadbalancer.DeleteLoadBalancerRequest{
		LoadBalancerId: lb.Id,
	})
	if err != nil {
		s.Logger.Error(err, "failed to delete apiserver lbs lb")
		return errors.Wrap(err, "failed to delete apiserver lbs lb")
	}
	_, err = ociutil.AwaitLbsLBWorkRequest(ctx, s.LoadBalancerServiceClient, lbResponse.OpcWorkRequestId)
	if err != nil {
		return errors.Wrap(err, "work request to delete lb failed")
	}
	s.Logger.Info("Successfully deleted apiserver lb", "lb", lb.Id)
	return nil
}

// LBSpec builds the LoadBalancer from the ClusterScope and returns it
func (s *ClusterScope) LbsLBSpec() infrastructurev1beta2.LoadBalancer {
	lbSpec := infrastructurev1beta2.LoadBalancer{
		Name: s.GetControlPlaneLoadBalancerName(),
	}
	return lbSpec
}

// GetControlPlaneLoadBalancerName returns the user defined APIServerLB name from the spec or
// assigns the name based on the OCICluster's name
func (s *ClusterScope) GetControlPlaneLbsLoadBalancerName() string {
	if s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.Name != "" {
		return s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.Name
	}
	return fmt.Sprintf("%s-%s", s.OCIClusterAccessor.GetName(), "apiserver")
}

// UpdateLB updates existing Load Balancer's DisplayName, FreeformTags and DefinedTags
func (s *ClusterScope) UpdateLbsLB(ctx context.Context, lb infrastructurev1beta2.LoadBalancer) error {
	lbId := s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId
	updateLBDetails := loadbalancer.UpdateLoadBalancerDetails{
		DisplayName:  common.String(lb.Name),
		FreeformTags: s.GetFreeFormTags(),
		DefinedTags:  s.GetDefinedTags(),
	}
	lbResponse, err := s.LoadBalancerServiceClient.UpdateLoadBalancer(ctx, loadbalancer.UpdateLoadBalancerRequest{
		UpdateLoadBalancerDetails: updateLBDetails,
		LoadBalancerId:            lbId,
	})
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the apiserver LB, failed to generate update lb workrequest")
		return errors.Wrap(err, "failed to reconcile the apiserver LB, failed to generate update lb workrequest")
	}
	_, err = ociutil.AwaitLbsLBWorkRequest(ctx, s.LoadBalancerServiceClient, lbResponse.OpcWorkRequestId)
	if err != nil {
		s.Logger.Error(err, "failed to reconcile the apiserver LB, failed to update lb")
		return errors.Wrap(err, "failed to reconcile the apiserver LB, failed to update lb")
	}
	return nil
}

// CreateLbsLB configures and creates the Load Balancer for the cluster based on the ClusterScope.
// This configures the LB Listeners and Backend Sets in order to create the Load Balancer.
// It will await the Work Request completion before returning
//
// See https://docs.oracle.com/en-us/iaas/Content/LoadBalancer/overview.htm for more details on the Network
// Load Balancer
func (s *ClusterScope) CreateLbsLB(ctx context.Context, lb infrastructurev1beta2.LoadBalancer) (*string, *string, error) {
	listenerDetails := make(map[string]loadbalancer.ListenerDetails)
	listenerDetails[APIServerLBListener] = loadbalancer.ListenerDetails{
		Protocol:              common.String("TCP"),
		Port:                  common.Int(int(s.APIServerPort())),
		DefaultBackendSetName: common.String(APIServerLBBackendSetName),
	}

	backendSetDetails := make(map[string]loadbalancer.BackendSetDetails)
	backendSetDetails[APIServerLBBackendSetName] = loadbalancer.BackendSetDetails{
		Policy: common.String("ROUND_ROBIN"),
		/*
			HealthChecker: &loadbalancer.HealthCheckerDetails{
				Port:       common.Int(int(s.APIServerPort())),
				Protocol:   common.String("HTTP"),
				UrlPath:    common.String("/healthz"),
				ReturnCode: common.Int(200),
			},
		*/
		HealthChecker: &loadbalancer.HealthCheckerDetails{
			// Work-around not knowing certs for the k8s endpoint.
			// Network Load Balancer doesn't need to know this information.
			Port:     common.Int(int(s.APIServerPort())),
			Protocol: common.String("TCP"),
		},
		Backends: []loadbalancer.BackendDetails{},
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
	lbDetails := loadbalancer.CreateLoadBalancerDetails{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(lb.Name),
		ShapeName:     common.String("flexible"),
		ShapeDetails: &loadbalancer.ShapeDetails{MinimumBandwidthInMbps: common.Int(10),
			MaximumBandwidthInMbps: common.Int(100)},
		SubnetIds:    controlPlaneEndpointSubnets,
		IsPrivate:    common.Bool(s.isControlPlaneEndpointSubnetPrivate()),
		Listeners:    listenerDetails,
		BackendSets:  backendSetDetails,
		FreeformTags: s.GetFreeFormTags(),
		DefinedTags:  s.GetDefinedTags(),
	}

	for _, nsg := range s.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List {
		if nsg.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			if nsg.ID != nil {
				lbDetails.NetworkSecurityGroupIds = []string{*nsg.ID}
			}
		}
	}

	s.Logger.Info("Creating lbs load balancer...")
	lbResponse, err := s.LoadBalancerServiceClient.CreateLoadBalancer(ctx, loadbalancer.CreateLoadBalancerRequest{
		CreateLoadBalancerDetails: lbDetails,
		OpcRetryToken:             ociutil.GetOPCRetryToken("%s-%s", "create-lb", s.OCIClusterAccessor.GetOCIResourceIdentifier()),
	})
	if err != nil {
		s.Logger.Error(err, "failed to create apiserver lb, failed to create work request")
		return nil, nil, errors.Wrap(err, "failed to create apiserver lb, failed to create work request")
	}

	wr, err := ociutil.AwaitLbsLBWorkRequest(ctx, s.LoadBalancerServiceClient, lbResponse.OpcWorkRequestId)
	if err != nil {
		return nil, nil, errors.Wrap(err, "awaiting load balancer")
	}

	// // Not handling pagination, but think we ought to not really need to..
	// listResp, err := s.LoadBalancerServiceClient.ListLoadBalancers(ctx, loadbalancer.ListLoadBalancersRequest{
	// 	CompartmentId: common.String(s.GetCompartmentId()),
	// 	DisplayName:   common.String(lb.Name),
	// 	SortBy:        loadbalancer.ListLoadBalancersSortByTimecreated,
	// 	SortOrder:     loadbalancer.ListLoadBalancersSortOrderDesc,
	// })
	// if err != nil {
	// 	s.Logger.Error(err, "failed to list lbs after creation")
	// 	return nil, nil, errors.Wrap(err, "failed to list lbs after creation")
	// }

	// if len(listResp.Items) < 1 {
	// 	s.Logger.Error(err, "failed to find lbs apiserver lb after creation")
	// 	return nil, nil, errors.Wrap(err, "failed to find lbs apiserver lb after creation")
	// }

	lbs, err := s.LoadBalancerServiceClient.GetLoadBalancer(ctx, loadbalancer.GetLoadBalancerRequest{
		LoadBalancerId: wr.LoadBalancerId,
	})
	if err != nil {
		s.Logger.Error(err, "failed to get apiserver lb after creation")
		return nil, nil, errors.Wrap(err, "failed to get apiserver lb after creation")
	}

	lbIp, err := s.getLbsLoadbalancerIp(lbs.LoadBalancer)
	if err != nil {
		return nil, nil, err
	}

	s.Logger.Info("Successfully created apiserver lb", "lb", lbs.Id, "ip", lbIp)
	return lbs.Id, lbIp, nil
}

func (s *ClusterScope) getLbsLoadbalancerIp(lb loadbalancer.LoadBalancer) (*string, error) {
	var lbIp *string
	if len(lb.IpAddresses) < 1 {
		return nil, errors.New("lb does not have valid ip addresses")
	}
	if *lb.IsPrivate {
		lbIp = lb.IpAddresses[0].IpAddress
	} else {
		for _, ip := range lb.IpAddresses {
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

// IsLBEqual determines if the actual loadbalancer.LoadBalancer is equal to the desired.
// Equality is determined by DisplayName, FreeformTags and DefinedTags matching.
func (s *ClusterScope) IsLbsLBEqual(actual *loadbalancer.LoadBalancer, desired infrastructurev1beta2.LoadBalancer) bool {
	if desired.Name != *actual.DisplayName {
		return false
	}
	return true
}

// GetLbsLoadBalancer retrieves the Cluster's loadbalancer.LoadBalancer using the one of the following methods
//
// 1. the OCICluster's spec LoadBalancerId
//
// 2. Listing the LoadBalancers for the Compartment (by ID) and DisplayName then filtering by tag
func (s *ClusterScope) GetLbsLoadBalancer(ctx context.Context) (*loadbalancer.LoadBalancer, error) {
	lbOcid := s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId
	if lbOcid != nil {
		resp, err := s.LoadBalancerServiceClient.GetLoadBalancer(ctx, loadbalancer.GetLoadBalancerRequest{
			LoadBalancerId: lbOcid,
		})
		if err != nil {
			return nil, err
		}
		nlb := resp.LoadBalancer
		if s.IsResourceCreatedByClusterAPI(nlb.FreeformTags) {
			return &nlb, nil
		} else {
			return nil, errors.New("cluster api tags have been modified out of context")
		}
	}
	lbs, err := s.LoadBalancerServiceClient.ListLoadBalancers(ctx, loadbalancer.ListLoadBalancersRequest{
		CompartmentId: common.String(s.GetCompartmentId()),
		DisplayName:   common.String(s.GetControlPlaneLoadBalancerName()),
	})
	if err != nil {
		s.Logger.Error(err, "Failed to list lbs lb by name")
		return nil, errors.Wrap(err, "failed to list lbs lb by name")
	}

	for _, lb := range lbs.Items {
		if s.IsResourceCreatedByClusterAPI(lb.FreeformTags) {
			resp, err := s.LoadBalancerServiceClient.GetLoadBalancer(ctx, loadbalancer.GetLoadBalancerRequest{
				LoadBalancerId: lb.Id,
			})
			if err != nil {
				return nil, err
			}
			return &resp.LoadBalancer, nil
		}
	}
	return nil, nil
}
