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
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	identityClient "github.com/oracle/cluster-api-provider-oci/cloud/services/identity"
	nlb "github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/pkg/errors"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AvailabilityDomain                = "AvailabilityDomain"
	FaultDomain                       = "FaultDomain"
	OCIClusterKind                    = "OCICluster"
	OCIManagedClusterKind             = "OCIManagedCluster"
	OCIManagedClusterControlPlaneKind = "OCIManagedClusterControlPlane"
)

// ClusterScopeParams defines the params need to create a new ClusterScope
type ClusterScopeParams struct {
	Client             client.Client
	Logger             *logr.Logger
	Cluster            *clusterv1.Cluster
	VCNClient          vcn.Client
	LoadBalancerClient nlb.NetworkLoadBalancerClient
	IdentityClient     identityClient.Client
	// RegionIdentifier Identifier as specified here https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm
	RegionIdentifier      string
	OCIAuthConfigProvider common.ConfigurationProvider
	ClientProvider        *ClientProvider
	OCIClusterAccessor    OCIClusterAccessor
	// RegionIdentifier Key as specified here https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm
	RegionKey string
}

type ClusterScope struct {
	*logr.Logger
	client             client.Client
	patchHelper        *patch.Helper
	Cluster            *clusterv1.Cluster
	VCNClient          vcn.Client
	LoadBalancerClient nlb.NetworkLoadBalancerClient
	IdentityClient     identityClient.Client
	// RegionIdentifier Identifier as specified here https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm
	RegionIdentifier   string
	ClientProvider     *ClientProvider
	OCIClusterAccessor OCIClusterAccessor
	// RegionIdentifier Key as specified here https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm
	RegionKey string
}

// NewClusterScope creates a ClusterScope given the ClusterScopeParams
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	// TODO add conditions everywhere properly and events as well
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.OCIClusterAccessor == nil {
		return nil, errors.New("failed to generate new scope from nil OCIClusterAccessor")
	}

	if params.Logger == nil {
		log := klogr.New()
		params.Logger = &log
	}

	return &ClusterScope{
		Logger:             params.Logger,
		client:             params.Client,
		Cluster:            params.Cluster,
		VCNClient:          params.VCNClient,
		LoadBalancerClient: params.LoadBalancerClient,
		IdentityClient:     params.IdentityClient,
		RegionIdentifier:   params.RegionIdentifier,
		ClientProvider:     params.ClientProvider,
		OCIClusterAccessor: params.OCIClusterAccessor,
		RegionKey:          params.RegionKey,
	}, nil
}

func (s *ClusterScope) IsResourceCreatedByClusterAPI(resourceFreeFormTags map[string]string) bool {
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(s.OCIClusterAccessor.GetOCIResourceIdentifier())
	for k, v := range tagsAddedByClusterAPI {
		if resourceFreeFormTags[k] != v {
			return false
		}
	}
	return true
}

func (s *ClusterScope) ReconcileFailureDomains(ctx context.Context) error {
	if s.OCIClusterAccessor.GetFailureDomains() == nil {
		return s.setFailureDomains(ctx)
	}
	return nil
}

// setFailureDomains sets the failure domains of the environment based on whether it is single AD or multi AD regions
// in case of single AD regions, the failure domain will be fault domain, in case of multi Ad regions, it will
// be AD
func (s *ClusterScope) setFailureDomains(ctx context.Context) error {
	adMap := s.OCIClusterAccessor.GetAvailabilityDomains()
	if adMap == nil {
		reqAd := identity.ListAvailabilityDomainsRequest{CompartmentId: common.String(s.GetCompartmentId())}

		respAd, err := s.IdentityClient.ListAvailabilityDomains(ctx, reqAd)
		if err != nil {
			s.Logger.Error(err, "failed to list identity domains")
			return err
		}

		// build the AD list for cluster
		adMap, err = s.setAvailabiltyDomainSpec(ctx, respAd.Items)
		if err != nil {
			return err
		}
	}

	numOfAds := len(adMap)
	if numOfAds != 1 && numOfAds != 3 {
		err := errors.New(fmt.Sprintf("invalid number of Availability Domains, should be either 1 or 3, but got %d", numOfAds))
		s.Logger.Error(err, "invalid number of Availability Domains")
		return err
	}

	if numOfAds == 3 {
		for k := range adMap {
			adIndex := strings.LastIndexAny(k, "-")
			if adIndex < 0 {
				return errors.New(fmt.Sprintf("could not infer ad number from availability domain %s", k))
			}
			adNumber := k[adIndex+1:]
			_, err := strconv.Atoi(adNumber)
			if err != nil {
				return errors.New(fmt.Sprintf("availability domain is not a valid integer: availability domain %s", k))
			}
			s.SetFailureDomain(adNumber, clusterv1.FailureDomainSpec{
				ControlPlane: true,
				Attributes:   map[string]string{AvailabilityDomain: k},
			})
		}
	} else {
		// only first element is used, hence break at the end
		for k := range adMap {
			for i, fd := range adMap[k].FaultDomains {
				s.SetFailureDomain(strconv.Itoa(i+1), clusterv1.FailureDomainSpec{
					ControlPlane: true,
					Attributes: map[string]string{
						AvailabilityDomain: k,
						FaultDomain:        fd,
					},
				})
			}
			break
		}

	}

	return nil
}

// SetFailureDomain sets the cluster's failure domain in the status
func (s *ClusterScope) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	s.OCIClusterAccessor.SetFailureDomain(id, spec)
}

// setAvailabiltyDomainSpec builds the OCIAvailabilityDomain list and sets the OCICluster's spec with this list
// so that other parts of the provider have access to ADs and FDs without having to make multiple calls to identity.
func (s *ClusterScope) setAvailabiltyDomainSpec(ctx context.Context, ads []identity.AvailabilityDomain) (map[string]infrastructurev1beta2.OCIAvailabilityDomain, error) {
	clusterAds := make(map[string]infrastructurev1beta2.OCIAvailabilityDomain)
	for _, ad := range ads {
		reqFd := identity.ListFaultDomainsRequest{
			CompartmentId:      common.String(s.GetCompartmentId()),
			AvailabilityDomain: ad.Name,
		}
		respFd, err := s.IdentityClient.ListFaultDomains(ctx, reqFd)
		if err != nil {
			s.Logger.Error(err, "failed to list fault domains")
			return nil, err
		}

		var faultDomains []string
		for _, fd := range respFd.Items {
			faultDomains = append(faultDomains, *fd.Name)
		}

		adName := *ad.Name
		clusterAds[adName] = infrastructurev1beta2.OCIAvailabilityDomain{
			Name:         adName,
			FaultDomains: faultDomains,
		}
	}
	s.OCIClusterAccessor.SetAvailabilityDomains(clusterAds)

	return clusterAds, nil
}

// GetDefinedTags returns a map of DefinedTags defined in the OCICluster's spec
func (s *ClusterScope) GetDefinedTags() map[string]map[string]interface{} {
	tags := s.OCIClusterAccessor.GetDefinedTags()
	if tags == nil {
		return make(map[string]map[string]interface{})
	}
	definedTags := make(map[string]map[string]interface{})
	for ns, mapNs := range tags {
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
	return s.OCIClusterAccessor.GetCompartmentId()
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
	tags := s.OCIClusterAccessor.GetFreeformTags()
	completeTags := make(map[string]string)
	for k, v := range tags {
		completeTags[k] = v
	}
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(s.OCIClusterAccessor.GetOCIResourceIdentifier())
	for k, v := range tagsAddedByClusterAPI {
		completeTags[k] = v
	}
	return completeTags
}

func (s *ClusterScope) GetOCIClusterAccessor() OCIClusterAccessor {
	return s.OCIClusterAccessor
}

func (s *ClusterScope) getDRG() *infrastructurev1beta2.DRG {
	return s.OCIClusterAccessor.GetNetworkSpec().VCNPeering.DRG
}

func (s *ClusterScope) getDrgID() *string {
	return s.getDRG().ID
}

func (s *ClusterScope) isPeeringEnabled() bool {
	return s.OCIClusterAccessor.GetNetworkSpec().VCNPeering != nil
}

// SetRegionKey sets the region key in the scope
func (s *ClusterScope) SetRegionKey(ctx context.Context) error {
	regionCode, err := GetRegionCodeFromRegion(ctx, s.IdentityClient, s.RegionIdentifier)
	if err != nil {
		s.Logger.Error(err, "failed to get shortId for the region")
		return err
	}
	s.RegionKey = regionCode
	return nil
}

// GetRegionCodeFromRegion pulls all OCI regions available and returns the passed in region's code if contained in
// the list.
//
// example: "ca-toronto-1" -> "YYZ"
func GetRegionCodeFromRegion(ctx context.Context, identityClient identityClient.Client, region string) (string, error) {
	regionCodes, err := identityClient.ListRegions(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to list oci regions")
	}
	for _, regionCode := range regionCodes.Items {
		if *regionCode.Name == region {
			return *regionCode.Key, nil
		}
	}
	return "", errors.Errorf("unable to get region code from region name")
}
