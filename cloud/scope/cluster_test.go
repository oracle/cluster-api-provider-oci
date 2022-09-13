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
	"testing"

	"github.com/golang/mock/gomock"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/identity/mock_identity"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestClusterScope_ReconcileFailureDomains(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	identityClient := mock_identity.NewMockClient(mockCtrl)
	identityClient.EXPECT().ListAvailabilityDomains(gomock.Any(), gomock.Eq(identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String("3ad"),
	})).Return(identity.ListAvailabilityDomainsResponse{Items: []identity.AvailabilityDomain{
		{
			Name: common.String("ad1"),
		},
		{
			Name: common.String("ad2"),
		},
		{
			Name: common.String("ad3"),
		},
	}}, nil)

	identityClient.EXPECT().ListAvailabilityDomains(gomock.Any(), gomock.Eq(identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String("list-ad-error"),
	})).Return(identity.ListAvailabilityDomainsResponse{}, errors.New("some error"))

	identityClient.EXPECT().ListAvailabilityDomains(gomock.Any(), gomock.Eq(identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String("1ad"),
	})).Return(identity.ListAvailabilityDomainsResponse{Items: []identity.AvailabilityDomain{
		{
			Name: common.String("ad1"),
		},
	}}, nil)

	identityClient.EXPECT().ListAvailabilityDomains(gomock.Any(), gomock.Eq(identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String("list-fd-error"),
	})).Return(identity.ListAvailabilityDomainsResponse{Items: []identity.AvailabilityDomain{
		{
			Name: common.String("ad1"),
		},
	}}, nil)

	identityClient.EXPECT().ListAvailabilityDomains(gomock.Any(), gomock.Eq(identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String("2ad"),
	})).Return(identity.ListAvailabilityDomainsResponse{Items: []identity.AvailabilityDomain{
		{
			Name: common.String("ad1"),
		},
		{
			Name: common.String("ad2"),
		},
	}}, nil)

	identityClient.EXPECT().ListFaultDomains(gomock.Any(), gomock.Eq(identity.ListFaultDomainsRequest{
		CompartmentId:      common.String("1ad"),
		AvailabilityDomain: common.String("ad1"),
	})).Return(identity.ListFaultDomainsResponse{Items: []identity.FaultDomain{
		{
			Name:               common.String("fd1"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fd2"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fd3"),
			AvailabilityDomain: common.String("ad1"),
		},
	}}, nil)

	identityClient.EXPECT().ListFaultDomains(gomock.Any(), gomock.Eq(identity.ListFaultDomainsRequest{
		CompartmentId:      common.String("3ad"),
		AvailabilityDomain: common.String("ad1"),
	})).Return(identity.ListFaultDomainsResponse{Items: []identity.FaultDomain{
		{
			Name:               common.String("fault-domain-1"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fault-domain-2"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fault-domain-3"),
			AvailabilityDomain: common.String("ad1"),
		},
	}}, nil)

	identityClient.EXPECT().ListFaultDomains(gomock.Any(), gomock.Eq(identity.ListFaultDomainsRequest{
		CompartmentId:      common.String("3ad"),
		AvailabilityDomain: common.String("ad2"),
	})).Return(identity.ListFaultDomainsResponse{Items: []identity.FaultDomain{
		{
			Name:               common.String("fault-domain-1"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fault-domain-2"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fault-domain-3"),
			AvailabilityDomain: common.String("ad1"),
		},
	}}, nil)

	identityClient.EXPECT().ListFaultDomains(gomock.Any(), gomock.Eq(identity.ListFaultDomainsRequest{
		CompartmentId:      common.String("3ad"),
		AvailabilityDomain: common.String("ad3"),
	})).Return(identity.ListFaultDomainsResponse{Items: []identity.FaultDomain{
		{
			Name:               common.String("fault-domain-1"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fault-domain-2"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fault-domain-3"),
			AvailabilityDomain: common.String("ad1"),
		},
	}}, nil)

	identityClient.EXPECT().ListFaultDomains(gomock.Any(), gomock.Eq(identity.ListFaultDomainsRequest{
		CompartmentId:      common.String("2ad"),
		AvailabilityDomain: common.String("ad1"),
	})).Return(identity.ListFaultDomainsResponse{Items: []identity.FaultDomain{
		{
			Name:               common.String("fd1"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fd2"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fd3"),
			AvailabilityDomain: common.String("ad1"),
		},
	}}, nil)

	identityClient.EXPECT().ListFaultDomains(gomock.Any(), gomock.Eq(identity.ListFaultDomainsRequest{
		CompartmentId:      common.String("2ad"),
		AvailabilityDomain: common.String("ad2"),
	})).Return(identity.ListFaultDomainsResponse{Items: []identity.FaultDomain{
		{
			Name:               common.String("fd1"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fd2"),
			AvailabilityDomain: common.String("ad1"),
		},
		{
			Name:               common.String("fd3"),
			AvailabilityDomain: common.String("ad1"),
		},
	}}, nil)

	identityClient.EXPECT().ListFaultDomains(gomock.Any(), gomock.Eq(identity.ListFaultDomainsRequest{
		CompartmentId:      common.String("list-fd-error"),
		AvailabilityDomain: common.String("ad1"),
	})).Return(identity.ListFaultDomainsResponse{}, errors.New("some error"))

	tests := []struct {
		name          string
		spec          infrastructurev1beta1.OCIClusterSpec
		wantErr       bool
		expectedError string
	}{
		{
			name: "3ad region",
			spec: infrastructurev1beta1.OCIClusterSpec{CompartmentId: "3ad"},
		},
		{
			name: "1ad region",
			spec: infrastructurev1beta1.OCIClusterSpec{CompartmentId: "1ad"},
		},
		{
			name:          "2ad region",
			spec:          infrastructurev1beta1.OCIClusterSpec{CompartmentId: "2ad"},
			wantErr:       true,
			expectedError: "invalid number of Availability Domains, should be either 1 or 3, but got 2",
		},
		{
			name:          "list ad error",
			spec:          infrastructurev1beta1.OCIClusterSpec{CompartmentId: "list-ad-error"},
			wantErr:       true,
			expectedError: "some error",
		},
		{
			name:          "list fd error",
			spec:          infrastructurev1beta1.OCIClusterSpec{CompartmentId: "list-fd-error"},
			wantErr:       true,
			expectedError: "some error",
		},
	}
	l := log.FromContext(context.Background())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ociClusterAccessor := OCISelfManagedCluster{
				OCICluster: &infrastructurev1beta1.OCICluster{
					Spec: tt.spec,
					ObjectMeta: metav1.ObjectMeta{
						UID: "a",
					},
				},
			}
			s := &ClusterScope{
				IdentityClient:     identityClient,
				OCIClusterAccessor: &ociClusterAccessor,
				Logger:             &l,
			}
			err := s.ReconcileFailureDomains(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("ReconcileFailureDomains() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("ReconcileFailureDomains() expected error = %s, actual error %s", tt.expectedError, err.Error())
				}

			}
		})
	}

}

func Eq(customMatcher func(arg interface{}) error) gomock.Matcher {
	return &matcherCustomizer{matcherFunction: customMatcher}
}

type matcherCustomizer struct {
	matcherFunction func(arg interface{}) error
	err             error
}

func (o matcherCustomizer) Matches(x interface{}) bool {
	// in case of this matcher has been used in a failed test before
	o.err = nil
	if err := o.matcherFunction(x); err != nil {
		o.err = err
	}
	return o.err == nil
}

func (o *matcherCustomizer) String() string {
	if o.err == nil {
		return "is as expected"
	}
	return o.err.Error()
}
