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
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	identityClent "github.com/oracle/cluster-api-provider-oci/cloud/services/identity"
	nlb "github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/identity"
	"github.com/pkg/errors"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AvailabilityDomain = "AvailabilityDomain"
	FaultDomain        = "FaultDomain"
	OCIClusterKind     = "OCICluster"
)

// ClusterScopeParams defines the params need to create a new ClusterScope
type ClusterScopeParams struct {
	Client                client.Client
	Logger                *logr.Logger
	Cluster               *clusterv1.Cluster
	OCICluster            *infrastructurev1beta1.OCICluster
	VCNClient             vcn.Client
	LoadBalancerClient    nlb.Client
	IdentityClient        identityClent.Client
	Region                string
	OCIAuthConfigProvider common.ConfigurationProvider
}

type ClusterScope struct {
	*logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster            *clusterv1.Cluster
	OCICluster         *infrastructurev1beta1.OCICluster
	VCNClient          vcn.Client
	LoadBalancerClient nlb.Client
	IdentityClient     identityClent.Client
	Region             string
}

// NewClusterScope creates a ClusterScope given the ClusterScopeParams
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	// TODO add conditions everywhere properly and events as well
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.OCICluster == nil {
		return nil, errors.New("failed to generate new scope from nil OCICluster")
	}

	if params.Logger == nil {
		log := klogr.New()
		params.Logger = &log
	}

	helper, err := patch.NewHelper(params.OCICluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ClusterScope{
		Logger:             params.Logger,
		client:             params.Client,
		Cluster:            params.Cluster,
		OCICluster:         params.OCICluster,
		patchHelper:        helper,
		VCNClient:          params.VCNClient,
		LoadBalancerClient: params.LoadBalancerClient,
		IdentityClient:     params.IdentityClient,
		Region:             params.Region,
	}, nil
}

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.OCICluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

func (s *ClusterScope) ReconcileFailureDomains(ctx context.Context) error {
	if s.OCICluster.Status.FailureDomains == nil {
		return s.setFailureDomains(ctx)
	}
	return nil
}

func (s *ClusterScope) IsResourceCreatedByClusterAPI(resourceFreeFormTags map[string]string) bool {
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(string(s.OCICluster.UID))
	for k, v := range tagsAddedByClusterAPI {
		if resourceFreeFormTags[k] != v {
			return false
		}
	}
	return true
}

// setFailureDomains sets the failure domains of the environment based on whether it is single AD or multi AD regions
// in case of single AD regions, the failure domain will be fault domain, in case of multi Ad regions, it will
// be AD
func (s *ClusterScope) setFailureDomains(ctx context.Context) error {
	req := identity.ListAvailabilityDomainsRequest{CompartmentId: common.String(s.GetCompartmentId())}

	resp, err := s.IdentityClient.ListAvailabilityDomains(ctx, req)
	if err != nil {
		s.Logger.Error(err, "failed to list identity domains")
		return err
	}

	numOfAds := len(resp.Items)
	if numOfAds != 1 && numOfAds != 3 {
		err := errors.New(fmt.Sprintf("invalid number of Availability Domains, should be either 1 or 3, but got %d", numOfAds))
		s.Logger.Error(err, "invalid number of Availability Domains")
		return err
	}

	if numOfAds == 3 {
		for i, ad := range resp.Items {
			s.SetFailureDomain(strconv.Itoa(i+1), clusterv1.FailureDomainSpec{
				ControlPlane: true,
				Attributes:   map[string]string{AvailabilityDomain: *ad.Name},
			})
		}
	} else {
		req := identity.ListFaultDomainsRequest{
			CompartmentId:      common.String(s.GetCompartmentId()),
			AvailabilityDomain: resp.Items[0].Name,
		}
		resp, err := s.IdentityClient.ListFaultDomains(ctx, req)
		if err != nil {
			s.Logger.Error(err, "failed to list fault domains")
			return err
		}
		for i, fd := range resp.Items {
			s.SetFailureDomain(strconv.Itoa(i+1), clusterv1.FailureDomainSpec{
				ControlPlane: true,
				Attributes: map[string]string{
					AvailabilityDomain: *fd.AvailabilityDomain,
					FaultDomain:        *fd.Name,
				},
			})
		}
	}

	return nil
}

// SetFailureDomain sets the cluster's failure domain in the status
func (s *ClusterScope) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	if s.OCICluster.Status.FailureDomains == nil {
		s.OCICluster.Status.FailureDomains = make(clusterv1.FailureDomains)
	}
	s.OCICluster.Status.FailureDomains[id] = spec
}

func (s *ClusterScope) IsTagsEqual(freeFromTags map[string]string, definedTags map[string]map[string]interface{}) bool {
	if reflect.DeepEqual(freeFromTags, s.GetFreeFormTags()) && reflect.DeepEqual(definedTags, s.GetDefinedTags()) {
		return true
	}
	return false
}

// GetRegionCodeFromRegion pulls all OCI regions available and returns the passed in region's code if contained in
// the list.
//
// example: "ca-toronto-1" -> "YYZ"
func (s *ClusterScope) GetRegionCodeFromRegion(ctx context.Context, region string) (string, error) {
	regionCodes, err := s.IdentityClient.ListRegions(ctx)
	if err != nil {
		s.Logger.Error(err, "failed to list oci regions")
		return "", errors.Wrap(err, "failed to list oci regions")
	}
	for _, regionCode := range regionCodes.Items {
		if *regionCode.Name == region {
			return *regionCode.Key, nil
		}
	}
	return "", errors.Errorf("unable to get region code from region name")
}

// GetDefinedTags returns a map of DefinedTags defined in the OCICluster's spec
func (s *ClusterScope) GetDefinedTags() map[string]map[string]interface{} {
	if s.OCICluster.Spec.DefinedTags == nil {
		return make(map[string]map[string]interface{})
	}
	definedTags := make(map[string]map[string]interface{})
	for ns, mapNs := range s.OCICluster.Spec.DefinedTags {
		mapValues := make(map[string]interface{})
		for k, v := range mapNs {
			mapValues[k] = v
		}
		definedTags[ns] = mapValues
	}
	return definedTags
}

// GetCompartmentId returns the CompartmentId defined in OCICluster's spec
func (s *ClusterScope) GetCompartmentId() string {
	return s.OCICluster.Spec.CompartmentId
}

// APIServerPort returns the APIServerPort to use when creating the load balancer.
func (s *ClusterScope) APIServerPort() int32 {
	if s.Cluster.Spec.ClusterNetwork != nil && s.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return *s.Cluster.Spec.ClusterNetwork.APIServerPort
	}
	return ApiServerPort
}

// GetFreeFormTags returns a map of FreeformTags defined in the OCICluster's spec
func (s *ClusterScope) GetFreeFormTags() map[string]string {
	tags := s.OCICluster.Spec.FreeformTags
	if tags == nil {
		tags = make(map[string]string)
	}
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(string(s.OCICluster.UID))
	for k, v := range tagsAddedByClusterAPI {
		tags[k] = v
	}
	return tags
}

func (s *ClusterScope) GetOCICluster() *infrastructurev1beta1.OCICluster {
	return s.OCICluster
}
