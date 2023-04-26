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

package util

import (
	"context"
	"fmt"
	"reflect"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/config"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetClusterIdentityFromRef returns the OCIClusterIdentity referenced by the OCICluster.
func GetClusterIdentityFromRef(ctx context.Context, c client.Client, ociClusterNamespace string, ref *corev1.ObjectReference) (*infrastructurev1beta2.OCIClusterIdentity, error) {
	identity := &infrastructurev1beta2.OCIClusterIdentity{}
	if ref != nil {
		namespace := ref.Namespace
		if namespace == "" {
			namespace = ociClusterNamespace
		}
		key := client.ObjectKey{Name: ref.Name, Namespace: namespace}
		if err := c.Get(ctx, key, identity); err != nil {
			return nil, err
		}
		return identity, nil
	}
	return nil, nil
}

// GetOrBuildClientFromIdentity creates ClientProvider from OCIClusterIdentity object
func GetOrBuildClientFromIdentity(ctx context.Context, c client.Client, identity *infrastructurev1beta2.OCIClusterIdentity, defaultRegion string, clientOverrides *infrastructurev1beta2.ClientOverrides) (*scope.ClientProvider, error) {
	if identity.Spec.Type == infrastructurev1beta2.UserPrincipal {
		secretRef := identity.Spec.PrincipalSecret
		key := types.NamespacedName{
			Namespace: secretRef.Namespace,
			Name:      secretRef.Name,
		}
		secret := &corev1.Secret{}

		if err := c.Get(ctx, key, secret); err != nil {
			return nil, errors.Wrap(err, "Unable to fetch ClientSecret")
		}

		tenancyId := string(secret.Data[config.Tenancy])
		userId := string(secret.Data[config.User])
		fingerPrint := string(secret.Data[config.Fingerprint])
		passphrase := string(secret.Data[config.Passphrase])
		privatekey := string(secret.Data[config.Key])
		region := string(secret.Data[config.Region])
		// set the default region if not provided in the secret
		if region == "" {
			region = defaultRegion
		}
		conf := common.NewRawConfigurationProvider(
			tenancyId,
			userId,
			region,
			fingerPrint,
			privatekey,
			common.String(passphrase))

		clientProvider, err := scope.NewClientProvider(scope.ClientProviderParams{
			OciAuthConfigProvider: conf,
			ClientOverrides:       clientOverrides})
		if err != nil {
			return nil, err
		}
		return clientProvider, nil
	}
	return nil, errors.New(fmt.Sprintf("invalid oci principal format type: %s", identity.Spec.Type))
}

// IsClusterNamespaceAllowed indicates if the cluster namespace is allowed.
func IsClusterNamespaceAllowed(ctx context.Context, k8sClient client.Client, allowedNamespaces *infrastructurev1beta2.AllowedNamespaces, namespace string) bool {
	if allowedNamespaces == nil {
		return false
	}

	// empty value matches with all namespaces
	if reflect.DeepEqual(*allowedNamespaces, infrastructurev1beta2.AllowedNamespaces{}) {
		return true
	}

	for _, v := range allowedNamespaces.NamespaceList {
		if v == namespace {
			return true
		}
	}

	// Check if clusterNamespace is in the namespaces selected by the identity's allowedNamespaces selector.
	namespaces := &corev1.NamespaceList{}
	selector, err := metav1.LabelSelectorAsSelector(allowedNamespaces.Selector)
	if err != nil {
		return false
	}

	// If a Selector has a nil or empty selector, it should match nothing.
	if selector.Empty() {
		return false
	}

	if err := k8sClient.List(ctx, namespaces, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return false
	}

	for _, n := range namespaces.Items {
		if n.Name == namespace {
			return true
		}
	}

	return false
}

// InitClientsAndRegion initializes the OCI Clients and Region based on various parameters
func InitClientsAndRegion(ctx context.Context, client client.Client, defaultRegion string, clusterAccessor scope.OCIClusterAccessor, defaultClientProvider *scope.ClientProvider) (*scope.ClientProvider, string, scope.OCIClients, error) {
	var clientProvider *scope.ClientProvider
	var err error
	// Region is calculated as follows
	// 1) If region is set in the cluster spec, that takes highest priority
	// 2) If region is set in the cluster identity, that takes the next priority
	// 3) Last priority is for region set at the Pod initialization time OCI identity
	clusterRegion := defaultRegion

	identityRef := clusterAccessor.GetIdentityRef()
	// If Cluster identity is set, OCI Clients should be created using the identity
	if identityRef != nil {
		clientProvider, err = CreateClientProviderFromClusterIdentity(ctx, client, clusterAccessor.GetNameSpace(), defaultRegion, clusterAccessor, identityRef)
		if err != nil {
			return nil, "", scope.OCIClients{}, err
		}
		region, err := clientProvider.GetRegion()
		if err != nil {
			return nil, "", scope.OCIClients{}, err
		}
		clusterRegion = region
	} else if clusterAccessor.GetClientOverrides() != nil {
		// IdentityRef provider will be created with client host url overrides
		// but if no identityRef we will want to create a new client provider with the overrides
		clientProvider, err = scope.NewClientProvider(scope.ClientProviderParams{
			OciAuthConfigProvider: defaultClientProvider.GetAuthProvider(),
			ClientOverrides:       clusterAccessor.GetClientOverrides()})
		if err != nil {
			return nil, "", scope.OCIClients{}, err
		}
	} else {
		clientProvider = defaultClientProvider
	}

	if clientProvider == nil {
		return nil, "", scope.OCIClients{}, errors.New("OCI authentication credentials could not be retrieved from pod or cluster level," +
			"please install Cluster API Provider for OCI with OCI authentication credentials or set Cluster Identity in the OCICluster")
	}

	// Region set at cluster takes highest precedence
	if len(clusterAccessor.GetRegion()) > 0 {
		clusterRegion = clusterAccessor.GetRegion()
	}
	if len(clusterRegion) <= 0 {
		return nil, "", scope.OCIClients{}, errors.New("OCI Region could not be identified for the cluster")
	}
	clients, err := clientProvider.GetOrBuildClient(clusterRegion)
	if err != nil {
		return nil, "", scope.OCIClients{}, err
	}
	return clientProvider, clusterRegion, clients, nil
}

// CreateClientProviderFromClusterIdentity creates scope.ClientProvider from Cluster Identity
func CreateClientProviderFromClusterIdentity(ctx context.Context, client client.Client, namespace string, defaultRegion string, clusterAccessor scope.OCIClusterAccessor, identityRef *corev1.ObjectReference) (*scope.ClientProvider, error) {
	identity, err := GetClusterIdentityFromRef(ctx, client, namespace, identityRef)
	if err != nil {
		return nil, err
	}
	if !IsClusterNamespaceAllowed(ctx, client, identity.Spec.AllowedNamespaces, namespace) {
		clusterAccessor.MarkConditionFalse(infrastructurev1beta2.ClusterReadyCondition, infrastructurev1beta2.NamespaceNotAllowedByIdentity, clusterv1.ConditionSeverityError, "")
		return nil, errors.Errorf("OCIClusterIdentity list of allowed namespaces doesn't include current cluster namespace %s", namespace)
	}

	clientProvider, err := GetOrBuildClientFromIdentity(ctx, client, identity, defaultRegion, clusterAccessor.GetClientOverrides())
	if err != nil {
		return nil, err
	}
	return clientProvider, nil
}
