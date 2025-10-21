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
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/config"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/labels/format"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	instanceMetadataRegionInfoURLV2 = "http://169.254.169.254/opc/v2/instance/regionInfo/regionIdentifier"
)

var (
	currentRegion *string
)

// GetClusterIdentityFromRef returns the OCIClusterIdentity referenced by the OCICluster.
// nolint:nilnil
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

// getOCIClientCertFromSecret returns the cert referenced by the OCICluster.
func getOCIClientCertFromSecret(ctx context.Context, c client.Client, ociClusterNamespace string, overrides *infrastructurev1beta2.ClientOverrides) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if overrides != nil && overrides.CertOverride != nil {
		certSecretRef := overrides.CertOverride
		namespace := certSecretRef.Namespace
		if namespace == "" {
			namespace = ociClusterNamespace
		}
		key := types.NamespacedName{Namespace: namespace, Name: certSecretRef.Name}
		if err := c.Get(ctx, key, secret); err != nil {
			return nil, err
		}
		return secret, nil
	}
	return nil, errors.New("OCI Client Cert not found")
}

func getOCIClientCertPool(ctx context.Context, c client.Client, namespace string, clientOverrides *infrastructurev1beta2.ClientOverrides) (*x509.CertPool, error) {
	var pool *x509.CertPool = nil
	if clientOverrides != nil && clientOverrides.CertOverride != nil {
		cert, err := getOCIClientCertFromSecret(ctx, c, namespace, clientOverrides)
		if err != nil {
			return nil, errors.Wrap(err, "Unable to fetch CertOverrideSecret")
		}
		pool = x509.NewCertPool()
		if cert == nil {
			return nil, errors.New("Cert Secret is nil")
		}
		if cert, ok := cert.Data["cert"]; ok {
			pool.AppendCertsFromPEM(cert)
		} else {
			return nil, errors.New("Cert Secret didn't contain 'cert' data")
		}
	}
	return pool, nil
}

// GetOrBuildClientFromIdentity creates ClientProvider from OCIClusterIdentity object
// nolint:nilaway
func GetOrBuildClientFromIdentity(ctx context.Context, c client.Client, identity *infrastructurev1beta2.OCIClusterIdentity, defaultRegion string, clientOverrides *infrastructurev1beta2.ClientOverrides, namespace string) (*scope.ClientProvider, error) {
	logger := log.FromContext(ctx)
	if !reflect.DeepEqual(identity.Spec, v1beta2.OCIClusterIdentitySpec{}) && identity.Spec.Type == infrastructurev1beta2.UserPrincipal {
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

		pool, err := getOCIClientCertPool(ctx, c, namespace, clientOverrides)
		if err != nil {
			return nil, err
		}

		clientProvider, err := scope.NewClientProvider(scope.ClientProviderParams{
			CertOverride:          pool,
			OciAuthConfigProvider: conf,
			ClientOverrides:       clientOverrides})

		if err != nil {
			return nil, err
		}
		return clientProvider, nil
	} else if identity.Spec.Type == infrastructurev1beta2.InstancePrincipal {
		provider, err := auth.InstancePrincipalConfigurationProvider()
		if err != nil {
			return nil, err
		}
		pool, err := getOCIClientCertPool(ctx, c, namespace, clientOverrides)
		if err != nil {
			return nil, err
		}
		clientProvider, err := scope.NewClientProvider(scope.ClientProviderParams{
			CertOverride:          pool,
			OciAuthConfigProvider: provider,
			ClientOverrides:       clientOverrides})

		if err != nil {
			return nil, err
		}
		return clientProvider, nil
	} else if identity.Spec.Type == infrastructurev1beta2.WorkloadPrincipal {
		_, containsVersion := os.LookupEnv(auth.ResourcePrincipalVersionEnvVar)
		if !containsVersion {
			os.Setenv(auth.ResourcePrincipalVersionEnvVar, auth.ResourcePrincipalVersion2_2)
		}
		_, containsRegion := os.LookupEnv(auth.ResourcePrincipalRegionEnvVar)
		if !containsRegion {
			// initialize the current region from region metadata
			if currentRegion == nil {
				regionByte, err := getRegionInfoFromInstanceMetadataServiceProd()
				if err != nil {
					return nil, err
				}
				currentRegion = common.String(string(regionByte))
			}
			logger.Info(fmt.Sprintf("Looked up region %s from instance metadata", *currentRegion))
			os.Setenv(auth.ResourcePrincipalRegionEnvVar, *currentRegion)
		}

		provider, err := auth.OkeWorkloadIdentityConfigurationProvider()
		if err != nil {
			return nil, err
		}
		pool, err := getOCIClientCertPool(ctx, c, namespace, clientOverrides)
		if err != nil {
			return nil, err
		}
		clientProvider, err := scope.NewClientProvider(scope.ClientProviderParams{
			CertOverride:          pool,
			OciAuthConfigProvider: provider,
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
		pool, err := getOCIClientCertPool(ctx, client, clusterAccessor.GetNameSpace(), clusterAccessor.GetClientOverrides())
		if err != nil {
			return nil, "", scope.OCIClients{}, err
		}
		// IdentityRef provider will be created with client host url overrides
		// but if no identityRef we will want to create a new client provider with the overrides
		clientProvider, err = scope.NewClientProvider(scope.ClientProviderParams{
			CertOverride:          pool,
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
// nolint:nilaway
func CreateClientProviderFromClusterIdentity(ctx context.Context, client client.Client, namespace string, defaultRegion string, clusterAccessor scope.OCIClusterAccessor, identityRef *corev1.ObjectReference) (*scope.ClientProvider, error) {
	identity, err := GetClusterIdentityFromRef(ctx, client, namespace, identityRef)
	if err != nil {
		return nil, err
	}

	if !IsClusterNamespaceAllowed(ctx, client, identity.Spec.AllowedNamespaces, namespace) {
		clusterAccessor.MarkConditionFalse(infrastructurev1beta2.ClusterReadyCondition, infrastructurev1beta2.NamespaceNotAllowedByIdentity, clusterv1.ConditionSeverityError, "")
		return nil, errors.Errorf("OCIClusterIdentity list of allowed namespaces doesn't include current cluster namespace %s", namespace)
	}
	clientProvider, err := GetOrBuildClientFromIdentity(ctx, client, identity, defaultRegion, clusterAccessor.GetClientOverrides(), namespace)
	if err != nil {
		return nil, err
	}
	return clientProvider, nil
}

// CreateMachinePoolMachinesIfNotExists creates the machine pool machines if not exists. This method lists the existing
// machines in the clusters and does a diff, and creates any missing machines based ont he spec provided.
func CreateMachinePoolMachinesIfNotExists(ctx context.Context, params MachineParams) error {

	machineList, err := getMachinepoolMachines(ctx, params.Client, params.MachinePool, params.Cluster, params.Namespace)
	if err != nil {
		return err
	}

	instanceNameToMachinePoolMachine := make(map[string]infrav2exp.OCIMachinePoolMachine)
	for _, machine := range machineList.Items {
		instanceNameToMachinePoolMachine[*machine.Spec.OCID] = machine
	}

	for _, specMachine := range params.SpecInfraMachines {
		if actualMachine, exists := instanceNameToMachinePoolMachine[*specMachine.Spec.OCID]; exists {
			if !reflect.DeepEqual(specMachine.Status.Ready, actualMachine.Status.Ready) {
				params.Logger.Info("Setting status of machine to active", "machine", actualMachine.Name)

				helper, err := patch.NewHelper(&actualMachine, params.Client)
				if err != nil {
					return err
				}
				actualMachine.Status.Ready = true
				err = helper.Patch(ctx, &actualMachine)
				if err != nil {
					return err
				}
			}
			continue
		}

		labels := map[string]string{
			clusterv1.ClusterNameLabel:     params.Cluster.Name,
			clusterv1.MachinePoolNameLabel: format.MustFormatValue(params.MachinePool.Name),
		}
		infraMachine := &infrav2exp.OCIMachinePoolMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    params.Namespace,
				GenerateName: params.MachinePool.Name,
				Labels:       labels,
				Annotations:  make(map[string]string),
				// set the parent to infra machinepool till the capi machine reconciler changes it to capi machinepool machine
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       params.InfraMachinePoolKind,
						Name:       params.InfraMachinePoolName,
						APIVersion: infrav2exp.GroupVersion.String(),
						UID:        params.InfraMachinePoolUID,
					},
				},
			},
			Spec: infrav2exp.OCIMachinePoolMachineSpec{
				OCID:         specMachine.Spec.OCID,
				ProviderID:   specMachine.Spec.ProviderID,
				InstanceName: specMachine.Spec.InstanceName,
				MachineType:  specMachine.Spec.MachineType,
			},
		}
		infraMachine.Status.Ready = specMachine.Status.Ready
		controllerutil.AddFinalizer(infraMachine, infrav2exp.MachinePoolMachineFinalizer)
		params.Logger.Info("Creating machinepool  machine", "machine", infraMachine.Name, "instanceName", specMachine.Name)

		if err := params.Client.Create(ctx, infraMachine); err != nil {
			return errors.Wrap(err, "failed to create machine")
		}
	}

	return nil
}

func getMachinepoolMachines(ctx context.Context, c client.Client, machinePool *expclusterv1.MachinePool, cluster *clusterv1.Cluster, namespace string) (*infrav2exp.OCIMachinePoolMachineList, error) {
	machineList := &infrav2exp.OCIMachinePoolMachineList{}
	labels := map[string]string{
		clusterv1.ClusterNameLabel:     cluster.Name,
		clusterv1.MachinePoolNameLabel: format.MustFormatValue(machinePool.Name),
	}
	if err := c.List(ctx, machineList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return machineList, nil
}

// DeleteOrphanedMachinePoolMachines deletes the machine pool machines which are not required. This method lists the
// existing machines in the clusters and does a diff with the spec and deletes any machines which are missing from the spec.
func DeleteOrphanedMachinePoolMachines(ctx context.Context, params MachineParams) error {
	machineList, err := getMachinepoolMachines(ctx, params.Client, params.MachinePool, params.Cluster, params.Namespace)
	if err != nil {
		return err
	}

	// create a set of instances in the spec, which will be used for lookup later
	instanceSpecSet := map[string]struct{}{}
	for _, specMachine := range params.SpecInfraMachines {
		instanceSpecSet[*specMachine.Spec.OCID] = struct{}{}
	}

	for i := range machineList.Items {
		machinePoolMachine := &machineList.Items[i]
		// lookup if the machinepool machine is not in the spec, if not delete the underlying machine
		if _, ok := instanceSpecSet[*machinePoolMachine.Spec.OCID]; !ok {
			machine, err := util.GetOwnerMachine(ctx, params.Client, machinePoolMachine.ObjectMeta)
			if err != nil {
				if apierrors.IsNotFound(err) {
					params.Logger.Info("Machinepool machine has not been created", "machine", machinePoolMachine.Name)
					continue
				}
				return errors.Wrapf(err, "failed to get owner Machine for machinepool machine %s/%s", machinePoolMachine.Namespace, machinePoolMachine.Name)
			}
			if machine == nil {
				return errors.Errorf("Machinepool %s/%s has no parent Machine, will reattempt deletion once parent Machine is present", machinePoolMachine.Namespace, machinePoolMachine.Name)
			}
			params.Logger.Info("Deleting machinepool machine", "machine", machine.Name)
			if err := params.Client.Delete(ctx, machine); err != nil {
				return errors.Wrapf(err, "failed to delete machinepool machine %s/%s", machine.Namespace, machine.Name)
			}
		} else {
			params.Logger.Info("Keeping machinepool, nothing to do", "machine", machinePoolMachine.Name, "namespace", machinePoolMachine.Namespace)
		}
	}

	return nil
}
func getRegionInfoFromInstanceMetadataServiceProd() ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, instanceMetadataRegionInfoURLV2, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", "Bearer Oracle")

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call instance metadata service")
	}

	statusCode := resp.StatusCode

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get region information from response body")
	}

	if statusCode != http.StatusOK {
		err = fmt.Errorf("HTTP Get failed: URL: %s, Status: %s, Message: %s",
			instanceMetadataRegionInfoURLV2, resp.Status, string(content))
		return nil, err
	}

	return content, nil
}

// MachineParams specifies the params required to create or delete machinepool machines.
// Infra machine pool specifed below refers to OCIManagedMachinePool/OCIMachinePool/OCIVirtualMachinePool
type MachineParams struct {
	Client               client.Client                      // the kubernetes client
	MachinePool          *expclusterv1.MachinePool          // the corresponding machinepool
	Cluster              *clusterv1.Cluster                 // the corresponding cluster
	InfraMachinePoolName string                             // the name of the infra machinepool corresponding(can be managed/self managed/virtual)
	InfraMachinePoolKind string                             // the kind of infra machinepool(can be managed/self managed/virtual)
	InfraMachinePoolUID  types.UID                          // the UID of the infra machinepool
	Namespace            string                             // the namespace in which machinepool machine has to be created
	SpecInfraMachines    []infrav2exp.OCIMachinePoolMachine // the spec of actual machines in the pool
	Logger               *logr.Logger                       // the logger which has to be used
}
