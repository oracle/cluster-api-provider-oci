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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
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
		s.OCIClusterAccessor.SetControlPlaneEndpoint(clusterv1beta1.APIEndpoint{
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
	s.OCIClusterAccessor.SetControlPlaneEndpoint(clusterv1beta1.APIEndpoint{
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
	if err := s.reconcileNLBResources(ctx, nlb); err != nil {
		return err
	}
	return nil
}

func (s *ClusterScope) reconcileNLBResources(ctx context.Context, nlb infrastructurev1beta2.LoadBalancer) error {
	nlbID := s.OCIClusterAccessor.GetNetworkSpec().APIServerLB.LoadBalancerId
	actualResp, err := s.NetworkLoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: nlbID,
	})
	if err != nil {
		return errors.Wrap(err, "failed to get nlb for resource reconciliation")
	}
	desiredListeners, desiredBackendSets := s.buildDesiredNLBListenersAndBackendSets(nlb)

	for name, desired := range desiredListeners {
		actual, exists := actualResp.NetworkLoadBalancer.Listeners[name]
		if !exists {
			resp, err := s.NetworkLoadBalancerClient.CreateListener(ctx, networkloadbalancer.CreateListenerRequest{
				NetworkLoadBalancerId: nlbID,
				CreateListenerDetails: networkloadbalancer.CreateListenerDetails{
					Name:                  common.String(name),
					DefaultBackendSetName: desired.DefaultBackendSetName,
					Port:                  desired.Port,
					Protocol:              desired.Protocol,
				},
			})
			if err != nil {
				if isAlreadyExistsOCIError(err) {
					s.Logger.Info("NLB listener already exists during reconcile; continuing", "listener", name)
					continue
				}
				return errors.Wrapf(err, "failed to create nlb listener %q", name)
			}
			if _, err := ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, resp.OpcWorkRequestId); err != nil {
				return errors.Wrapf(err, "failed awaiting create listener %q", name)
			}
			continue
		}
		if !intPtrEqual(actual.Port, desired.Port) ||
			!ptr.StringEquals(actual.DefaultBackendSetName, ptr.ToString(desired.DefaultBackendSetName)) ||
			actual.Protocol != desired.Protocol {
			resp, err := s.NetworkLoadBalancerClient.UpdateListener(ctx, networkloadbalancer.UpdateListenerRequest{
				NetworkLoadBalancerId: nlbID,
				ListenerName:          common.String(name),
				UpdateListenerDetails: networkloadbalancer.UpdateListenerDetails{
					DefaultBackendSetName: desired.DefaultBackendSetName,
					Port:                  desired.Port,
					Protocol:              desired.Protocol,
				},
			})
			if err != nil {
				return errors.Wrapf(err, "failed to update nlb listener %q", name)
			}
			if _, err := ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, resp.OpcWorkRequestId); err != nil {
				return errors.Wrapf(err, "failed awaiting update listener %q", name)
			}
		}
	}
	staleListenerDeleted := false
	for name := range actualResp.NetworkLoadBalancer.Listeners {
		if _, keep := desiredListeners[name]; keep {
			continue
		}
		resp, err := s.NetworkLoadBalancerClient.DeleteListener(ctx, networkloadbalancer.DeleteListenerRequest{
			NetworkLoadBalancerId: nlbID,
			ListenerName:          common.String(name),
		})
		if err != nil {
			return errors.Wrapf(err, "failed to delete stale nlb listener %q", name)
		}
		if _, err := ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, resp.OpcWorkRequestId); err != nil {
			return errors.Wrapf(err, "failed awaiting delete listener %q", name)
		}
		staleListenerDeleted = true
	}
	if staleListenerDeleted {
		actualResp, err = s.NetworkLoadBalancerClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: nlbID,
		})
		if err != nil {
			return errors.Wrap(err, "failed to refresh nlb after stale listener reconciliation")
		}
	}

	for name, desired := range desiredBackendSets {
		actual, exists := actualResp.NetworkLoadBalancer.BackendSets[name]
		if !exists {
			resp, err := s.NetworkLoadBalancerClient.CreateBackendSet(ctx, networkloadbalancer.CreateBackendSetRequest{
				NetworkLoadBalancerId: nlbID,
				CreateBackendSetDetails: networkloadbalancer.CreateBackendSetDetails{
					Name:                     common.String(name),
					Policy:                   desired.Policy,
					HealthChecker:            nlbHealthCheckerToDetails(desired.HealthChecker),
					IsPreserveSource:         desired.IsPreserveSource,
					IsFailOpen:               desired.IsFailOpen,
					IsInstantFailoverEnabled: desired.IsInstantFailoverEnabled,
				},
			})
			if err != nil {
				if isAlreadyExistsOCIError(err) {
					s.Logger.Info("NLB backend set already exists during reconcile; continuing", "backendSet", name)
					continue
				}
				return errors.Wrapf(err, "failed to create nlb backend set %q", name)
			}
			if _, err := ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, resp.OpcWorkRequestId); err != nil {
				return errors.Wrapf(err, "failed awaiting create backend set %q", name)
			}
			continue
		}
		if actual.HealthChecker == nil || desired.HealthChecker == nil {
			// Reconcile nullable health checker by updating if either side is nil.
			resp, err := s.NetworkLoadBalancerClient.UpdateBackendSet(ctx, networkloadbalancer.UpdateBackendSetRequest{
				NetworkLoadBalancerId: nlbID,
				BackendSetName:        common.String(name),
				UpdateBackendSetDetails: networkloadbalancer.UpdateBackendSetDetails{
					Policy:                   common.String(string(desired.Policy)),
					HealthChecker:            nlbHealthCheckerToDetails(desired.HealthChecker),
					IsPreserveSource:         desired.IsPreserveSource,
					IsFailOpen:               desired.IsFailOpen,
					IsInstantFailoverEnabled: desired.IsInstantFailoverEnabled,
				},
			})
			if err != nil {
				return errors.Wrapf(err, "failed to update nlb backend set %q", name)
			}
			if _, err := ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, resp.OpcWorkRequestId); err != nil {
				return errors.Wrapf(err, "failed awaiting update backend set %q", name)
			}
			continue
		}
		if actual.Policy != desired.Policy ||
			!boolPtrEqual(actual.IsPreserveSource, desired.IsPreserveSource) ||
			!boolPtrEqual(actual.IsFailOpen, desired.IsFailOpen) ||
			!boolPtrEqual(actual.IsInstantFailoverEnabled, desired.IsInstantFailoverEnabled) ||
			!intPtrEqual(actual.HealthChecker.Port, desired.HealthChecker.Port) ||
			actual.HealthChecker.Protocol != desired.HealthChecker.Protocol ||
			!ptr.StringEquals(actual.HealthChecker.UrlPath, ptr.ToString(desired.HealthChecker.UrlPath)) {
			resp, err := s.NetworkLoadBalancerClient.UpdateBackendSet(ctx, networkloadbalancer.UpdateBackendSetRequest{
				NetworkLoadBalancerId: nlbID,
				BackendSetName:        common.String(name),
				UpdateBackendSetDetails: networkloadbalancer.UpdateBackendSetDetails{
					Policy:                   common.String(string(desired.Policy)),
					HealthChecker:            nlbHealthCheckerToDetails(desired.HealthChecker),
					IsPreserveSource:         desired.IsPreserveSource,
					IsFailOpen:               desired.IsFailOpen,
					IsInstantFailoverEnabled: desired.IsInstantFailoverEnabled,
				},
			})
			if err != nil {
				return errors.Wrapf(err, "failed to update nlb backend set %q", name)
			}
			if _, err := ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, resp.OpcWorkRequestId); err != nil {
				return errors.Wrapf(err, "failed awaiting update backend set %q", name)
			}
		}
	}
	for name, actual := range actualResp.NetworkLoadBalancer.BackendSets {
		if _, keep := desiredBackendSets[name]; keep {
			continue
		}
		if len(actual.Backends) > 0 {
			continue
		}
		resp, err := s.NetworkLoadBalancerClient.DeleteBackendSet(ctx, networkloadbalancer.DeleteBackendSetRequest{
			NetworkLoadBalancerId: nlbID,
			BackendSetName:        common.String(name),
		})
		if err != nil {
			return errors.Wrapf(err, "failed to delete stale nlb backend set %q", name)
		}
		if _, err := ociutil.AwaitNLBWorkRequest(ctx, s.NetworkLoadBalancerClient, resp.OpcWorkRequestId); err != nil {
			return errors.Wrapf(err, "failed awaiting delete backend set %q", name)
		}
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
	listenerDetails, backendSetDetails := s.buildDesiredNLBListenersAndBackendSets(lb)

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

func (s *ClusterScope) buildDesiredNLBListenersAndBackendSets(lb infrastructurev1beta2.LoadBalancer) (map[string]networkloadbalancer.ListenerDetails, map[string]networkloadbalancer.BackendSetDetails) {
	canonicalBackendSets := lb.NLBSpec.CanonicalBackendSets()
	if len(canonicalBackendSets) == 0 {
		canonicalBackendSets = []infrastructurev1beta2.NLBBackendSet{{Name: APIServerLBBackendSetName}}
	}

	backendSetDetails := make(map[string]networkloadbalancer.BackendSetDetails, len(canonicalBackendSets))
	for _, backendSet := range canonicalBackendSets {
		isPreserveSourceIp := backendSet.BackendSetDetails.IsPreserveSource
		if isPreserveSourceIp == nil {
			isPreserveSourceIp = common.Bool(false)
		}
		healthCheckURL := backendSet.BackendSetDetails.HealthChecker.UrlPath
		if healthCheckURL == nil {
			healthCheckURL = common.String("/healthz")
		}
		backendSetDetails[backendSet.Name] = networkloadbalancer.BackendSetDetails{
			Policy:                   LoadBalancerPolicy,
			IsPreserveSource:         isPreserveSourceIp,
			IsFailOpen:               backendSet.BackendSetDetails.IsFailOpen,
			IsInstantFailoverEnabled: backendSet.BackendSetDetails.IsInstantFailoverEnabled,
			HealthChecker: &networkloadbalancer.HealthChecker{
				Port:       common.Int(int(s.APIServerPort())),
				Protocol:   networkloadbalancer.HealthCheckProtocolsHttps,
				UrlPath:    healthCheckURL,
				ReturnCode: common.Int(200),
			},
			Backends: []networkloadbalancer.Backend{},
		}
	}

	listenerDetails := make(map[string]networkloadbalancer.ListenerDetails, len(canonicalBackendSets))
	for i, backendSet := range canonicalBackendSets {
		listenerName := desiredAPIServerListenerName(i, len(canonicalBackendSets), backendSet.Name)
		port := desiredAPIServerListenerPort(s.APIServerPort(), backendSet)
		listenerDetails[listenerName] = networkloadbalancer.ListenerDetails{
			Protocol:              networkloadbalancer.ListenerProtocolsTcp,
			Port:                  common.Int(int(port)),
			DefaultBackendSetName: common.String(backendSet.Name),
			Name:                  common.String(listenerName),
		}
	}
	return listenerDetails, backendSetDetails
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func nlbHealthCheckerToDetails(in *networkloadbalancer.HealthChecker) *networkloadbalancer.HealthCheckerDetails {
	if in == nil {
		return nil
	}
	return &networkloadbalancer.HealthCheckerDetails{
		Protocol:          in.Protocol,
		Port:              in.Port,
		Retries:           in.Retries,
		TimeoutInMillis:   in.TimeoutInMillis,
		IntervalInMillis:  in.IntervalInMillis,
		UrlPath:           in.UrlPath,
		ResponseBodyRegex: in.ResponseBodyRegex,
		ReturnCode:        in.ReturnCode,
		RequestData:       in.RequestData,
		ResponseData:      in.ResponseData,
	}
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
// Equality is determined by the NLB identity plus desired listener/backend-set resources matching.
func (s *ClusterScope) IsNLBEqual(actual *networkloadbalancer.NetworkLoadBalancer, desired infrastructurev1beta2.LoadBalancer) bool {
	if desired.Name != *actual.DisplayName {
		return false
	}

	desiredListeners, desiredBackendSets := s.buildDesiredNLBListenersAndBackendSets(desired)
	if len(actual.Listeners) != len(desiredListeners) {
		return false
	}
	for name, desiredListener := range desiredListeners {
		actualListener, exists := actual.Listeners[name]
		if !exists {
			return false
		}
		if !intPtrEqual(actualListener.Port, desiredListener.Port) ||
			!ptr.StringEquals(actualListener.DefaultBackendSetName, ptr.ToString(desiredListener.DefaultBackendSetName)) ||
			actualListener.Protocol != desiredListener.Protocol {
			return false
		}
	}

	if len(actual.BackendSets) != len(desiredBackendSets) {
		return false
	}
	for name, desiredBackendSet := range desiredBackendSets {
		actualBackendSet, exists := actual.BackendSets[name]
		if !exists {
			return false
		}
		if actualBackendSet.HealthChecker == nil || desiredBackendSet.HealthChecker == nil {
			return false
		}
		if actualBackendSet.Policy != desiredBackendSet.Policy ||
			!boolPtrEqual(actualBackendSet.IsPreserveSource, desiredBackendSet.IsPreserveSource) ||
			!boolPtrEqual(actualBackendSet.IsFailOpen, desiredBackendSet.IsFailOpen) ||
			!boolPtrEqual(actualBackendSet.IsInstantFailoverEnabled, desiredBackendSet.IsInstantFailoverEnabled) ||
			!intPtrEqual(actualBackendSet.HealthChecker.Port, desiredBackendSet.HealthChecker.Port) ||
			actualBackendSet.HealthChecker.Protocol != desiredBackendSet.HealthChecker.Protocol ||
			!ptr.StringEquals(actualBackendSet.HealthChecker.UrlPath, ptr.ToString(desiredBackendSet.HealthChecker.UrlPath)) {
			return false
		}
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
