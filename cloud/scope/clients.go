/*
Copyright (c) 2022, Oracle and/or its affiliates.

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
	"sync"

	"github.com/go-logr/logr"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute"
	identityClient "github.com/oracle/cluster-api-provider-oci/cloud/services/identity"
	nlb "github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"
	"github.com/oracle/oci-go-sdk/v63/common"
	"github.com/oracle/oci-go-sdk/v63/core"
	"github.com/oracle/oci-go-sdk/v63/identity"
	"github.com/oracle/oci-go-sdk/v63/networkloadbalancer"
	"github.com/pkg/errors"
	"k8s.io/klog/v2/klogr"
)

// OCIClients is the struct of all the needed OCI clients
type OCIClients struct {
	ComputeClient      compute.Client
	VCNClient          vcn.Client
	LoadBalancerClient nlb.Client
	IdentityClient     identityClient.Client
}

// ClientProvider defines the regional clients
type ClientProvider struct {
	*logr.Logger
	ociClients            map[string]OCIClients
	ociClientsLock        *sync.RWMutex
	ociAuthConfigProvider common.ConfigurationProvider
}

// NewClientProvider builds the ClientProvider with a client for the given region
func NewClientProvider(ociAuthConfigProvider common.ConfigurationProvider) (*ClientProvider, error) {
	log := klogr.New()

	if ociAuthConfigProvider == nil {
		return nil, errors.New("ConfigurationProvider can not be nil")
	}

	provider := ClientProvider{
		Logger:                &log,
		ociAuthConfigProvider: ociAuthConfigProvider,
		ociClients:            map[string]OCIClients{},
		ociClientsLock:        new(sync.RWMutex),
	}

	return &provider, nil
}

// GetOrBuildClient if the OCIClients exist for the region they are returned, if not clients will build them
func (c *ClientProvider) GetOrBuildClient(region string) (OCIClients, error) {
	if len(region) <= 0 {
		return OCIClients{}, errors.New("ClientProvider.GetOrBuildClient region can not be empty")
	}

	c.ociClientsLock.RLock()
	clients, regionalClientsExists := c.ociClients[region]
	c.ociClientsLock.RUnlock()

	if regionalClientsExists {
		return clients, nil
	}

	c.ociClientsLock.Lock()
	defer c.ociClientsLock.Unlock()
	regionalClient, err := createClients(region, c.ociAuthConfigProvider, c.Logger)
	if err != nil {
		return regionalClient, err
	}
	c.ociClients[region] = regionalClient

	return regionalClient, nil
}

func createClients(region string, oCIAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (OCIClients, error) {
	vcnClient, err := createVncClient(region, oCIAuthConfigProvider, logger)
	lbClient, err := createLbClient(region, oCIAuthConfigProvider, logger)
	identityClient, err := createIdentityClient(region, oCIAuthConfigProvider, logger)
	computeClient, err := createComputeClient(region, oCIAuthConfigProvider, logger)

	if err != nil {
		return OCIClients{}, err
	}

	return OCIClients{
		VCNClient:          vcnClient,
		LoadBalancerClient: lbClient,
		IdentityClient:     identityClient,
		ComputeClient:      computeClient,
	}, err
}

func createVncClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*core.VirtualNetworkClient, error) {
	vcnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI VCN Client")
		return nil, err
	}
	vcnClient.SetRegion(region)

	return &vcnClient, nil
}

func createLbClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*networkloadbalancer.NetworkLoadBalancerClient, error) {
	lbClient, err := networkloadbalancer.NewNetworkLoadBalancerClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI LB Client")
		return nil, err
	}
	lbClient.SetRegion(region)

	return &lbClient, nil
}

func createIdentityClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*identity.IdentityClient, error) {
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Identity Client")
		return nil, err
	}
	identityClient.SetRegion(region)

	return &identityClient, nil
}

func createComputeClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*core.ComputeClient, error) {
	computeClient, err := core.NewComputeClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Compute Client")
		return nil, err
	}
	computeClient.SetRegion(region)

	return &computeClient, nil
}
