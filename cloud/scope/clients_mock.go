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
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"

	"github.com/oracle/cluster-api-provider-oci/cloud/config"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/identity"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	"k8s.io/klog/v2/klogr"
)

type MockOCIClients struct {
	VCNClient                 vcn.Client
	ComputeClient             compute.ComputeClient
	NetworkLoadBalancerClient *networkloadbalancer.NetworkLoadBalancerClient
	LoadBalancerClient        *loadbalancer.LoadBalancerClient
	IdentityClient            identity.Client
}

var (
	MockTestRegion = "us-lexington-1"
)

func MockNewClientProvider(mockClients MockOCIClients) (*ClientProvider, error) {

	clientsInject := map[string]OCIClients{MockTestRegion: {
		VCNClient:                 mockClients.VCNClient,
		NetworkLoadBalancerClient: mockClients.NetworkLoadBalancerClient,
		LoadBalancerClient:        mockClients.LoadBalancerClient,
		IdentityClient:            mockClients.IdentityClient,
		ComputeClient:             mockClients.ComputeClient,
	}}

	authConfig, err := MockAuthConfig()
	if err != nil {
		return nil, err
	}

	ociAuthConfigProvider, err := config.NewConfigurationProvider(&authConfig)
	if err != nil {
		fmt.Printf("expected ociAuthConfigProvider to be created %s \n", err)
		return nil, err
	}
	log := klogr.New()
	clientProvider := ClientProvider{
		Logger:                &log,
		ociClients:            clientsInject,
		ociClientsLock:        new(sync.RWMutex),
		ociAuthConfigProvider: ociAuthConfigProvider,
	}

	return &clientProvider, nil
}

func MockAuthConfig() (config.AuthConfig, error) {
	privateKey, err := generatePrivateKeyPEM()
	if err != nil {
		fmt.Println("error generating a private key")
		return config.AuthConfig{}, err
	}

	authConfig := config.AuthConfig{
		UseInstancePrincipals: false,
		Region:                MockTestRegion,
		Fingerprint:           "mock_computemanagement-finger-print",
		PrivateKey:            privateKey,
		UserID:                "ocid1.tenancy.oc1..<unique_ID>",
		TenancyID:             "ocid1.tenancy.oc1..<unique_ID>",
	}

	return authConfig, nil
}

func generatePrivateKeyPEM() (string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	var privateKeyBuf bytes.Buffer
	err = pem.Encode(&privateKeyBuf, privateKeyBlock)
	if err != nil {
		fmt.Printf("error when encode private pem: %s \n", err)
		return "", err
	}

	return privateKeyBuf.String(), err
}
