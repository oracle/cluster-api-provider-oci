/*
Copyright 2022 The Kubernetes Authors.

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
	"github.com/oracle/cluster-api-provider-oci/cloud/config"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn/mock_vcn"
)

func TestClients_NewClientProvider(t *testing.T) {
	authConfig, err := MockAuthConfig()
	if err != nil {
		t.Errorf("Expected error:%v to not equal nil", err)
	}

	ociAuthConfigProvider, err := config.NewConfigurationProvider(&authConfig)
	if err != nil {
		t.Errorf("Expected error:%v to not equal nil", err)

	}

	clientProvider, err := NewClientProvider(ociAuthConfigProvider)
	if err != nil {
		t.Errorf("Expected %v to equal nil", err)
	}

	if reflect.DeepEqual(clientProvider, ClientProvider{}) {
		t.Errorf("clientProvider can not be an empty struct")
	}
}

func TestClients_NewClientProviderWithBadAuthConfig(t *testing.T) {

	clientProvider, err := NewClientProvider(nil)
	if err == nil {
		t.Errorf("Expected error:%v to not equal nil", err)
	}

	if clientProvider != nil {
		t.Errorf("Expected clientProvider:%v to equal nil", clientProvider)
	}
}

func TestClients_BuildNewClients(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	clientProvider, err := MockNewClientProvider(MockOCIClients{
		VCNClient: vcnClient,
	})
	if err != nil {
		t.Errorf("Expected %v to equal nil", err)
	}

	clients, err := clientProvider.GetOrBuildClient(MockTestRegion)
	if err != nil {
		t.Errorf("Expected %v to equal nil", err)
	}
	vcn := clients.VCNClient

	if vcn != vcnClient {
		t.Errorf("Expected %v to equal %v", vcnClient, vcn)
	}

	// build clients for a region not in our provider list yet
	clients, err = clientProvider.GetOrBuildClient("us-austin-1")
	if err != nil {
		t.Errorf("Expected %v to equal nil", err)
	}
	vcn = clients.VCNClient
	if vcn == vcnClient {
		t.Errorf("Expected %v to NOT equal %v", vcnClient, vcn)
	}
}

func TestClients_ReuseClients(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	vcnClient := mock_vcn.NewMockClient(mockCtrl)

	clientProvider, err := MockNewClientProvider(MockOCIClients{
		VCNClient: vcnClient,
	})
	if err != nil {
		t.Errorf("Expected %v to equal nil", err)
	}

	firstClients, err := clientProvider.GetOrBuildClient(MockTestRegion)
	if err != nil {
		t.Errorf("Expected %v to equal nil", err)
	}

	secondClients, err := clientProvider.GetOrBuildClient(MockTestRegion)
	if err != nil {
		t.Errorf("Expected %v to equal nil", err)
	}

	if &secondClients.VCNClient == &firstClients.VCNClient {
		t.Errorf("Expected %v to equal %v", secondClients.VCNClient, firstClients.VCNClient)
	}
}
