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
	"net/http"
	"sync"

	"github.com/go-logr/logr"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/base"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/computemanagement"
	containerEngineClient "github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine"
	identityClient "github.com/oracle/cluster-api-provider-oci/cloud/services/identity"
	lb "github.com/oracle/cluster-api-provider-oci/cloud/services/loadbalancer"
	nlb "github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"
	"github.com/oracle/cluster-api-provider-oci/version"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	"github.com/pkg/errors"
	"k8s.io/klog/v2/klogr"
)

// OCIClients is the struct of all the needed OCI clients
type OCIClients struct {
	ComputeClient             compute.ComputeClient
	ComputeManagementClient   computemanagement.Client
	VCNClient                 vcn.Client
	NetworkLoadBalancerClient nlb.NetworkLoadBalancerClient
	LoadBalancerClient        lb.LoadBalancerClient
	IdentityClient            identityClient.Client
	ContainerEngineClient     containerEngineClient.Client
	BaseClient                base.BaseClient
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

// GetRegion returns the region from the authentication config provider
func (c *ClientProvider) GetRegion() (string, error) {
	return c.ociAuthConfigProvider.Region()
}

func createClients(region string, oCIAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (OCIClients, error) {
	vcnClient, err := createVncClient(region, oCIAuthConfigProvider, logger)
	if err != nil {
		return OCIClients{}, err
	}
	nlbClient, err := createNLbClient(region, oCIAuthConfigProvider, logger)
	if err != nil {
		return OCIClients{}, err
	}
	lbClient, err := createLBClient(region, oCIAuthConfigProvider, logger)
	if err != nil {
		return OCIClients{}, err
	}
	identityClient, err := createIdentityClient(region, oCIAuthConfigProvider, logger)
	if err != nil {
		return OCIClients{}, err
	}
	computeClient, err := createComputeClient(region, oCIAuthConfigProvider, logger)
	if err != nil {
		return OCIClients{}, err
	}
	computeManagementClient, err := createComputeManagementClient(region, oCIAuthConfigProvider, logger)
	if err != nil {
		return OCIClients{}, err
	}
	containerEngineClient, err := createContainerEngineClient(region, oCIAuthConfigProvider, logger)
	if err != nil {
		return OCIClients{}, err
	}
	baseClient, err := createBaseClient(region, oCIAuthConfigProvider, logger)
	if err != nil {
		return OCIClients{}, err
	}

	if err != nil {
		return OCIClients{}, err
	}

	return OCIClients{
		VCNClient:                 vcnClient,
		NetworkLoadBalancerClient: nlbClient,
		LoadBalancerClient:        lbClient,
		IdentityClient:            identityClient,
		ComputeClient:             computeClient,
		ComputeManagementClient:   computeManagementClient,
		ContainerEngineClient:     containerEngineClient,
		BaseClient:                baseClient,
	}, err
}

func createVncClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*core.VirtualNetworkClient, error) {
	vcnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI VCN Client")
		return nil, err
	}
	vcnClient.SetRegion(region)
	vcnClient.Interceptor = setVersionHeader()

	return &vcnClient, nil
}

func createNLbClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*networkloadbalancer.NetworkLoadBalancerClient, error) {
	nlbClient, err := networkloadbalancer.NewNetworkLoadBalancerClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI LB Client")
		return nil, err
	}
	nlbClient.SetRegion(region)
	nlbClient.Interceptor = setVersionHeader()

	return &nlbClient, nil
}

func createLBClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*loadbalancer.LoadBalancerClient, error) {
	lbClient, err := loadbalancer.NewLoadBalancerClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI LBaaS Client")
		return nil, err
	}
	lbClient.SetRegion(region)
	lbClient.Interceptor = setVersionHeader()

	return &lbClient, nil
}

func createIdentityClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*identity.IdentityClient, error) {
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Identity Client")
		return nil, err
	}
	identityClient.SetRegion(region)
	identityClient.Interceptor = setVersionHeader()

	return &identityClient, nil
}

func createComputeClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*core.ComputeClient, error) {
	computeClient, err := core.NewComputeClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Compute Client")
		return nil, err
	}
	computeClient.SetRegion(region)
	computeClient.Interceptor = setVersionHeader()

	return &computeClient, nil
}

func createComputeManagementClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*core.ComputeManagementClient, error) {
	computeManagementClient, err := core.NewComputeManagementClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Compute Management Client")
		return nil, err
	}
	computeManagementClient.SetRegion(region)
	computeManagementClient.Interceptor = setVersionHeader()

	return &computeManagementClient, nil
}

func createContainerEngineClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*containerengine.ContainerEngineClient, error) {
	containerEngineClient, err := containerengine.NewContainerEngineClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Container Engine Client")
		return nil, err
	}
	containerEngineClient.SetRegion(region)
	containerEngineClient.Interceptor = setVersionHeader()

	return &containerEngineClient, nil
}

func createBaseClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (base.BaseClient, error) {
	baseClient, err := base.NewBaseClient(ociAuthConfigProvider, logger)
	if err != nil {
		logger.Error(err, "unable to create OCI Base Client")
		return nil, err
	}
	return baseClient, nil
}

func setVersionHeader() func(request *http.Request) error {
	return func(request *http.Request) error {
		request.Header.Set("X-CAPOCI-VERSION", version.GitVersion)
		return nil
	}
}
