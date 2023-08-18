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
	"crypto/x509"
	"net/http"
	"sync"

	"github.com/go-logr/logr"
	"github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/metrics"
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
	ociClientOverrides    *v1beta2.ClientOverrides
	certOverride          *x509.CertPool
}

// ClientProviderParams is the params struct for NewClientProvider
type ClientProviderParams struct {
	// OciAuthConfigProvider wraps information about the account owner
	OciAuthConfigProvider common.ConfigurationProvider

	// ClientOverrides contains information about client host url overrides.
	ClientOverrides *v1beta2.ClientOverrides

	// CertOverride a x509 CertPool to use as an override for client TLSClientConfig
	CertOverride *x509.CertPool
}

// NewClientProvider builds the ClientProvider with a client for the given region
func NewClientProvider(params ClientProviderParams) (*ClientProvider, error) {
	log := klogr.New()

	if params.OciAuthConfigProvider == nil {
		return nil, errors.New("ConfigurationProvider can not be nil")
	}

	provider := ClientProvider{
		Logger:                &log,
		certOverride:          params.CertOverride,
		ociAuthConfigProvider: params.OciAuthConfigProvider,
		ociClients:            map[string]OCIClients{},
		ociClientsLock:        new(sync.RWMutex),
		ociClientOverrides:    params.ClientOverrides,
	}

	return &provider, nil
}

// GetAuthProvider returns the client provider auth config
func (c *ClientProvider) GetAuthProvider() common.ConfigurationProvider {
	return c.ociAuthConfigProvider
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
	regionalClient, err := c.createClients(region)
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

func (c *ClientProvider) createClients(region string) (OCIClients, error) {
	vcnClient, err := c.createVcnClient(region, c.ociAuthConfigProvider, c.Logger)
	if err != nil {
		return OCIClients{}, err
	}
	nlbClient, err := c.createNLbClient(region, c.ociAuthConfigProvider, c.Logger)
	if err != nil {
		return OCIClients{}, err
	}
	lbClient, err := c.createLBClient(region, c.ociAuthConfigProvider, c.Logger)
	if err != nil {
		return OCIClients{}, err
	}
	identityClt, err := c.createIdentityClient(region, c.ociAuthConfigProvider, c.Logger)
	if err != nil {
		return OCIClients{}, err
	}
	computeClient, err := c.createComputeClient(region, c.ociAuthConfigProvider, c.Logger)
	if err != nil {
		return OCIClients{}, err
	}
	computeManagementClient, err := c.createComputeManagementClient(region, c.ociAuthConfigProvider, c.Logger)
	if err != nil {
		return OCIClients{}, err
	}
	containerEngineClt, err := c.createContainerEngineClient(region, c.ociAuthConfigProvider, c.Logger)
	if err != nil {
		return OCIClients{}, err
	}
	baseClient, err := c.createBaseClient(region, c.ociAuthConfigProvider, c.Logger)
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
		IdentityClient:            identityClt,
		ComputeClient:             computeClient,
		ComputeManagementClient:   computeManagementClient,
		ContainerEngineClient:     containerEngineClt,
		BaseClient:                baseClient,
	}, err
}

func (c *ClientProvider) createVcnClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*core.VirtualNetworkClient, error) {
	vcnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI VCN Client")
		return nil, err
	}
	vcnClient.SetRegion(region)
	dispatcher := vcnClient.HTTPClient
	vcnClient.HTTPClient = metrics.NewHttpRequestDispatcherWrapper(dispatcher, region)

	if c.certOverride != nil {
		if client, ok := dispatcher.(*http.Client); ok {
			err = c.setCerts(client)
			if err != nil {
				logger.Error(err, "unable to create OCI VCN Client")
				return nil, err
			}
		} else {
			return nil, errors.New("The VCN Client dispatcher is not of http.Client type. Can not patch the tls config.")
		}

	}

	if c.ociClientOverrides != nil && c.ociClientOverrides.VCNClientUrl != nil {
		vcnClient.Host = *c.ociClientOverrides.VCNClientUrl
	}
	vcnClient.Interceptor = setVersionHeader()

	return &vcnClient, nil
}

func (c *ClientProvider) createNLbClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*networkloadbalancer.NetworkLoadBalancerClient, error) {
	nlbClient, err := networkloadbalancer.NewNetworkLoadBalancerClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI LB Client")
		return nil, err
	}
	nlbClient.SetRegion(region)
	dispatcher := nlbClient.HTTPClient
	nlbClient.HTTPClient = metrics.NewHttpRequestDispatcherWrapper(dispatcher, region)

	if c.certOverride != nil {
		if client, ok := dispatcher.(*http.Client); ok {
			err = c.setCerts(client)
			if err != nil {
				logger.Error(err, "unable to create OCI NetworkLoadBalancer Client")
				return nil, err
			}
		} else {
			return nil, errors.New("The Network Loadbalancer Client dispatcher is not of http.Client type. Can not patch the tls config.")
		}
	}

	if c.ociClientOverrides != nil && c.ociClientOverrides.NetworkLoadBalancerClientUrl != nil {
		nlbClient.Host = *c.ociClientOverrides.NetworkLoadBalancerClientUrl
	}
	nlbClient.Interceptor = setVersionHeader()

	return &nlbClient, nil
}

func (c *ClientProvider) createLBClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*loadbalancer.LoadBalancerClient, error) {
	lbClient, err := loadbalancer.NewLoadBalancerClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI LBaaS Client")
		return nil, err
	}
	lbClient.SetRegion(region)
	dispatcher := lbClient.HTTPClient
	lbClient.HTTPClient = metrics.NewHttpRequestDispatcherWrapper(dispatcher, region)

	if c.certOverride != nil {
		if client, ok := dispatcher.(*http.Client); ok {
			err = c.setCerts(client)
			if err != nil {
				logger.Error(err, "unable to create OCI Loadbalancer Client")
				return nil, err
			}
		} else {
			return nil, errors.New("The Loadbalancer Client dispatcher is not of http.Client type. Can not patch the tls config.")
		}
	}

	if c.ociClientOverrides != nil && c.ociClientOverrides.LoadBalancerClientUrl != nil {
		lbClient.Host = *c.ociClientOverrides.LoadBalancerClientUrl
	}
	lbClient.Interceptor = setVersionHeader()

	return &lbClient, nil
}

func (c *ClientProvider) createIdentityClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*identity.IdentityClient, error) {
	identityClt, err := identity.NewIdentityClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Identity Client")
		return nil, err
	}
	identityClt.SetRegion(region)
	dispatcher := identityClt.HTTPClient
	identityClt.HTTPClient = metrics.NewHttpRequestDispatcherWrapper(dispatcher, region)

	if c.certOverride != nil {
		if client, ok := dispatcher.(*http.Client); ok {
			err = c.setCerts(client)
			if err != nil {
				logger.Error(err, "unable to create OCI Identity Client")
				return nil, err
			}
		} else {
			return nil, errors.New("The Identity Client dispatcher is not of http.Client type. Can not patch the tls config.")
		}
	}

	if c.ociClientOverrides != nil && c.ociClientOverrides.IdentityClientUrl != nil {
		identityClt.Host = *c.ociClientOverrides.IdentityClientUrl
	}
	identityClt.Interceptor = setVersionHeader()

	return &identityClt, nil
}

func (c *ClientProvider) createComputeClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (compute.ComputeClient, error) {
	computeClient, err := core.NewComputeClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Compute Client")
		return nil, err
	}
	computeClient.SetRegion(region)
	dispatcher := computeClient.HTTPClient
	computeClient.HTTPClient = metrics.NewHttpRequestDispatcherWrapper(dispatcher, region)

	if c.certOverride != nil {
		if client, ok := dispatcher.(*http.Client); ok {
			err = c.setCerts(client)
			if err != nil {
				logger.Error(err, "unable to create OCI Compute Client")
				return nil, err
			}
		} else {
			return nil, errors.New("The Compute Client dispatcher is not of http.Client type. Can not patch the tls config.")
		}
	}

	if c.ociClientOverrides != nil && c.ociClientOverrides.ComputeClientUrl != nil {
		computeClient.Host = *c.ociClientOverrides.ComputeClientUrl
	}
	computeClient.Interceptor = setVersionHeader()

	return &computeClient, nil
}

func (c *ClientProvider) createComputeManagementClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*core.ComputeManagementClient, error) {
	computeManagementClient, err := core.NewComputeManagementClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Compute Management Client")
		return nil, err
	}
	computeManagementClient.SetRegion(region)
	dispatcher := computeManagementClient.HTTPClient
	computeManagementClient.HTTPClient = metrics.NewHttpRequestDispatcherWrapper(dispatcher, region)

	if c.certOverride != nil {
		if client, ok := dispatcher.(*http.Client); ok {
			err = c.setCerts(client)
			if err != nil {
				logger.Error(err, "unable to create OCI Compute Management Client")
				return nil, err
			}
		} else {
			return nil, errors.New("The Compute Management Client dispatcher is not of http.Client type. Can not patch the tls config.")
		}
	}

	if c.ociClientOverrides != nil && c.ociClientOverrides.ComputeManagementClientUrl != nil {
		computeManagementClient.Host = *c.ociClientOverrides.ComputeManagementClientUrl
	}
	computeManagementClient.Interceptor = setVersionHeader()

	return &computeManagementClient, nil
}

func (c *ClientProvider) createContainerEngineClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (*containerengine.ContainerEngineClient, error) {
	containerEngineClt, err := containerengine.NewContainerEngineClientWithConfigurationProvider(ociAuthConfigProvider)
	if err != nil {
		logger.Error(err, "unable to create OCI Container Engine Client")
		return nil, err
	}
	containerEngineClt.SetRegion(region)
	dispatcher := containerEngineClt.HTTPClient
	containerEngineClt.HTTPClient = metrics.NewHttpRequestDispatcherWrapper(dispatcher, region)

	if c.certOverride != nil {
		if client, ok := dispatcher.(*http.Client); ok {
			err = c.setCerts(client)
			if err != nil {
				logger.Error(err, "unable to create OCI Container Engine Client")
				return nil, err
			}
		} else {
			return nil, errors.New("The Container Engine Client dispatcher is not of http.Client type. Can not patch the tls config.")
		}
	}

	if c.ociClientOverrides != nil && c.ociClientOverrides.ContainerEngineClientUrl != nil {
		containerEngineClt.Host = *c.ociClientOverrides.ContainerEngineClientUrl
	}
	containerEngineClt.Interceptor = setVersionHeader()

	return &containerEngineClt, nil
}

func (c *ClientProvider) createBaseClient(region string, ociAuthConfigProvider common.ConfigurationProvider, logger *logr.Logger) (base.BaseClient, error) {
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

// setCerts updates the client TLSClientConfig with the ClientProvider certOverride
func (c *ClientProvider) setCerts(client *http.Client) error {
	tr := client.Transport.(*http.Transport).Clone()
	if tr.TLSClientConfig != nil {
		tr.TLSClientConfig.RootCAs = c.certOverride
	} else {
		// If TLS client config is not set, we should error out. else the default values will
		// be used which will be insecure
		return errors.New("TLSClientConfig is not set on the client")
	}
	client.Transport = tr

	return nil
}
