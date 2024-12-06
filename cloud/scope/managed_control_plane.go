/*
Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	baseclient "github.com/oracle/cluster-api-provider-oci/cloud/services/base"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// OKEInitScript is the cloud init script of OKE slef managed node:
	// Reference : https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengcloudinitforselfmanagednodes.htm
	OKEInitScript = "#!/usr/bin/env bash\nbash /etc/oke/oke-install.sh \\\n  --apiserver-endpoint \"CLUSTER_ENDPOINT\" \\\n  --kubelet-ca-cert \"BASE_64_CA\""
)

// ManagedControlPlaneScopeParams defines the params need to create a new ManagedControlPlaneScope
type ManagedControlPlaneScopeParams struct {
	Logger                 *logr.Logger
	Client                 client.Client
	Cluster                *clusterv1.Cluster
	ContainerEngineClient  containerengine.Client
	BaseClient             baseclient.BaseClient
	ClientProvider         *ClientProvider
	OCIManagedControlPlane *infrastructurev1beta2.OCIManagedControlPlane
	OCIClusterAccessor     OCIClusterAccessor
	RegionIdentifier       string
}

type ManagedControlPlaneScope struct {
	*logr.Logger
	client                 client.Client
	Cluster                *clusterv1.Cluster
	ContainerEngineClient  containerengine.Client
	BaseClient             baseclient.BaseClient
	ClientProvider         *ClientProvider
	OCIManagedControlPlane *infrastructurev1beta2.OCIManagedControlPlane
	OCIClusterAccessor     OCIClusterAccessor
	RegionIdentifier       string
	patchHelper            *patch.Helper
}

// NewManagedControlPlaneScope creates a ManagedControlPlaneScope given the ManagedControlPlaneScopeParams
func NewManagedControlPlaneScope(params ManagedControlPlaneScopeParams) (*ManagedControlPlaneScope, error) {
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

	return &ManagedControlPlaneScope{
		Logger:                 params.Logger,
		client:                 params.Client,
		Cluster:                params.Cluster,
		ContainerEngineClient:  params.ContainerEngineClient,
		RegionIdentifier:       params.RegionIdentifier,
		ClientProvider:         params.ClientProvider,
		OCIClusterAccessor:     params.OCIClusterAccessor,
		OCIManagedControlPlane: params.OCIManagedControlPlane,
		BaseClient:             params.BaseClient,
	}, nil
}

// GetOrCreateControlPlane tries to lookup a control plane(OKE cluster) based on ID/Name and returns the
// cluster if it exists, else creates an OKE cluster with the provided parameters in spec.
func (s *ManagedControlPlaneScope) GetOrCreateControlPlane(ctx context.Context) (*oke.Cluster, error) {
	cluster, err := s.GetOKECluster(ctx)
	if err != nil {
		return nil, err
	}
	if cluster != nil {
		s.Logger.Info("Found an existing control plane")
		s.OCIManagedControlPlane.Spec.ID = cluster.Id
		return cluster, nil
	}
	endpointConfig := &oke.CreateClusterEndpointConfigDetails{
		SubnetId:          s.getControlPlaneEndpointSubnet(),
		NsgIds:            s.getControlPlaneEndpointNSGList(),
		IsPublicIpEnabled: common.Bool(s.IsControlPlaneEndpointSubnetPublic()),
	}
	controlPlaneSpec := s.OCIManagedControlPlane.Spec
	podNetworks := make([]oke.ClusterPodNetworkOptionDetails, 0)
	if len(controlPlaneSpec.ClusterPodNetworkOptions) > 0 {
		for _, cniOption := range controlPlaneSpec.ClusterPodNetworkOptions {
			if cniOption.CniType == infrastructurev1beta2.FlannelCNI {
				podNetworks = append(podNetworks, oke.FlannelOverlayClusterPodNetworkOptionDetails{})
			} else if cniOption.CniType == infrastructurev1beta2.VCNNativeCNI {
				podNetworks = append(podNetworks, oke.OciVcnIpNativeClusterPodNetworkOptionDetails{})
			}
		}
	}
	createOptions := oke.ClusterCreateOptions{
		ServiceLbSubnetIds: s.getServiceLbSubnets(),
		PersistentVolumeConfig: &oke.PersistentVolumeConfigDetails{
			FreeformTags: s.getFreeFormTags(),
			DefinedTags:  s.getDefinedTags(),
		},
		ServiceLbConfig: &oke.ServiceLbConfigDetails{
			FreeformTags: s.getFreeFormTags(),
			DefinedTags:  s.getDefinedTags(),
		},
	}
	if s.Cluster.Spec.ClusterNetwork != nil {
		networkConfig := oke.KubernetesNetworkConfig{}
		if s.Cluster.Spec.ClusterNetwork.Pods != nil {
			if len(s.Cluster.Spec.ClusterNetwork.Pods.CIDRBlocks) > 0 {
				networkConfig.PodsCidr = common.String(s.Cluster.Spec.ClusterNetwork.Pods.CIDRBlocks[0])
			}
		}
		if s.Cluster.Spec.ClusterNetwork.Services != nil {
			if len(s.Cluster.Spec.ClusterNetwork.Services.CIDRBlocks) > 0 {
				networkConfig.ServicesCidr = common.String(s.Cluster.Spec.ClusterNetwork.Services.CIDRBlocks[0])
			}
		}
		createOptions.KubernetesNetworkConfig = &networkConfig
	}

	if controlPlaneSpec.ClusterOption.OpenIdConnectDiscovery != nil {
		createOptions.OpenIdConnectDiscovery = &oke.OpenIdConnectDiscovery{
			IsOpenIdConnectDiscoveryEnabled: controlPlaneSpec.ClusterOption.OpenIdConnectDiscovery.IsOpenIdConnectDiscoveryEnabled,
		}
	}

	if controlPlaneSpec.ClusterOption.OpenIdConnectTokenAuthenticationConfig != nil {
		oidcConfig := controlPlaneSpec.ClusterOption.OpenIdConnectTokenAuthenticationConfig
		createOptions.OpenIdConnectTokenAuthenticationConfig = &oke.OpenIdConnectTokenAuthenticationConfig{
			IsOpenIdConnectAuthEnabled: &oidcConfig.IsOpenIdConnectAuthEnabled,
		}

		if oidcConfig.IssuerUrl != nil {
			createOptions.OpenIdConnectTokenAuthenticationConfig.IssuerUrl = oidcConfig.IssuerUrl
		}
		if oidcConfig.ClientId != nil {
			createOptions.OpenIdConnectTokenAuthenticationConfig.ClientId = oidcConfig.ClientId
		}
		if oidcConfig.UsernameClaim != nil {
			createOptions.OpenIdConnectTokenAuthenticationConfig.UsernameClaim = oidcConfig.UsernameClaim
		}
		if oidcConfig.UsernamePrefix != nil {
			createOptions.OpenIdConnectTokenAuthenticationConfig.UsernamePrefix = oidcConfig.UsernamePrefix
		}
		if oidcConfig.GroupsClaim != nil {
			createOptions.OpenIdConnectTokenAuthenticationConfig.GroupsClaim = oidcConfig.GroupsClaim
		}
		if oidcConfig.GroupsPrefix != nil {
			createOptions.OpenIdConnectTokenAuthenticationConfig.GroupsPrefix = oidcConfig.GroupsPrefix
		}
		if oidcConfig.RequiredClaims != nil {
			// Convert []infrastructurev1beta2.KeyValue to []containerengine.KeyValue
			requiredClaims := make([]oke.KeyValue, len(oidcConfig.RequiredClaims))
			for i, rc := range oidcConfig.RequiredClaims {
				requiredClaims[i] = oke.KeyValue(rc)
			}
			createOptions.OpenIdConnectTokenAuthenticationConfig.RequiredClaims = requiredClaims
		}
		if oidcConfig.CaCertificate != nil {
			createOptions.OpenIdConnectTokenAuthenticationConfig.CaCertificate = oidcConfig.CaCertificate
		}
		if oidcConfig.SigningAlgorithms != nil {
			createOptions.OpenIdConnectTokenAuthenticationConfig.SigningAlgorithms = oidcConfig.SigningAlgorithms
		}
	}

	if controlPlaneSpec.ClusterOption.AddOnOptions != nil {
		createOptions.AddOns = &oke.AddOnOptions{
			IsKubernetesDashboardEnabled: controlPlaneSpec.ClusterOption.AddOnOptions.IsKubernetesDashboardEnabled,
			IsTillerEnabled:              controlPlaneSpec.ClusterOption.AddOnOptions.IsTillerEnabled,
		}
	}

	if controlPlaneSpec.ClusterOption.AdmissionControllerOptions != nil {
		createOptions.AdmissionControllerOptions = &oke.AdmissionControllerOptions{
			IsPodSecurityPolicyEnabled: controlPlaneSpec.ClusterOption.AdmissionControllerOptions.IsPodSecurityPolicyEnabled,
		}
	}

	clusterType := getOKEClusterTypeFromSpecType(controlPlaneSpec)

	details := oke.CreateClusterDetails{
		Name:                     common.String(s.GetClusterName()),
		CompartmentId:            common.String(s.OCIClusterAccessor.GetCompartmentId()),
		VcnId:                    s.OCIClusterAccessor.GetNetworkSpec().Vcn.ID,
		KubernetesVersion:        controlPlaneSpec.Version,
		FreeformTags:             s.getFreeFormTags(),
		DefinedTags:              s.getDefinedTags(),
		Options:                  &createOptions,
		EndpointConfig:           endpointConfig,
		ClusterPodNetworkOptions: podNetworks,
		KmsKeyId:                 controlPlaneSpec.KmsKeyId,
		Type:                     clusterType,
	}

	if controlPlaneSpec.ImagePolicyConfig != nil {
		details.ImagePolicyConfig = &oke.CreateImagePolicyConfigDetails{
			IsPolicyEnabled: s.OCIManagedControlPlane.Spec.ImagePolicyConfig.IsPolicyEnabled,
			KeyDetails:      s.getKeyDetails(),
		}
	}

	createClusterRequest := oke.CreateClusterRequest{
		CreateClusterDetails: details,
		OpcRetryToken:        ociutil.GetOPCRetryToken("%s-%s", "create-oke", string(s.OCIClusterAccessor.GetOCIResourceIdentifier())),
	}
	response, err := s.ContainerEngineClient.CreateCluster(ctx, createClusterRequest)

	if err != nil {
		return nil, err
	}
	wrResponse, err := s.ContainerEngineClient.GetWorkRequest(ctx, oke.GetWorkRequestRequest{
		WorkRequestId: response.OpcWorkRequestId,
	})
	if err != nil {
		return nil, err
	}

	var clusterId *string
	for _, resource := range wrResponse.Resources {
		if *resource.EntityType == "cluster" {
			clusterId = resource.Identifier
		}
	}
	if clusterId == nil {
		return nil, errors.New(fmt.Sprintf("oke cluster ws not created with the request, please create a "+
			"support ticket with opc-request-id %s", *wrResponse.OpcRequestId))
	}
	s.OCIManagedControlPlane.Spec.ID = clusterId
	return s.getOKEClusterFromOCID(ctx, clusterId)
}

func getOKEClusterTypeFromSpecType(controlPlaneSpec infrastructurev1beta2.OCIManagedControlPlaneSpec) oke.ClusterTypeEnum {
	if controlPlaneSpec.ClusterType != "" {
		switch controlPlaneSpec.ClusterType {
		case infrastructurev1beta2.BasicClusterType:
			return oke.ClusterTypeBasicCluster
			break
		case infrastructurev1beta2.EnhancedClusterType:
			return oke.ClusterTypeEnhancedCluster
			break
		default:
			break
		}
	}
	return ""
}

// GetOKECluster tries to lookup a control plane(OKE cluster) based on ID/Name and returns the
// cluster if it exists,
func (s *ManagedControlPlaneScope) GetOKECluster(ctx context.Context) (*oke.Cluster, error) {
	okeClusterID := s.OCIManagedControlPlane.Spec.ID
	if okeClusterID != nil {
		return s.getOKEClusterFromOCID(ctx, okeClusterID)
	}
	instance, err := s.getOKEClusterByDisplayName(ctx, s.GetClusterName())
	if err != nil {
		return nil, err
	}
	return instance, err
}

func (s *ManagedControlPlaneScope) getOKEClusterFromOCID(ctx context.Context, clusterID *string) (*oke.Cluster, error) {
	req := oke.GetClusterRequest{ClusterId: clusterID}

	// Send the request using the service client
	resp, err := s.ContainerEngineClient.GetCluster(ctx, req)
	if err != nil {
		return nil, err
	}
	return &resp.Cluster, nil
}

func (s *ManagedControlPlaneScope) getOKEClusterByDisplayName(ctx context.Context, name string) (*oke.Cluster, error) {
	var page *string
	for {
		req := oke.ListClustersRequest{
			Name:          common.String(name),
			CompartmentId: common.String(s.OCIClusterAccessor.GetCompartmentId()),
			Page:          page,
		}
		resp, err := s.ContainerEngineClient.ListClusters(ctx, req)
		if err != nil {
			return nil, err
		}
		if len(resp.Items) == 0 {
			return nil, nil
		}
		for _, cluster := range resp.Items {
			if s.isResourceCreatedByClusterAPI(cluster.FreeformTags) {
				return s.getOKEClusterFromOCID(ctx, cluster.Id)
			}
		}
		if resp.OpcNextPage == nil {
			break
		} else {
			page = resp.OpcNextPage
		}
	}
	return nil, nil
}

func (s *ManagedControlPlaneScope) isResourceCreatedByClusterAPI(resourceFreeFormTags map[string]string) bool {
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(s.OCIClusterAccessor.GetOCIResourceIdentifier())
	for k, v := range tagsAddedByClusterAPI {
		if resourceFreeFormTags[k] != v {
			return false
		}
	}
	return true
}

// GetClusterName returns the name of the cluster
func (s *ManagedControlPlaneScope) GetClusterName() string {
	return s.OCIManagedControlPlane.Name
}

func (s *ManagedControlPlaneScope) getDefinedTags() map[string]map[string]interface{} {
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
func (s *ManagedControlPlaneScope) getFreeFormTags() map[string]string {
	tags := s.OCIClusterAccessor.GetFreeformTags()
	if tags == nil {
		tags = make(map[string]string)
	}
	tagsAddedByClusterAPI := ociutil.BuildClusterTags(s.OCIClusterAccessor.GetOCIResourceIdentifier())
	for k, v := range tagsAddedByClusterAPI {
		tags[k] = v
	}
	return tags
}

func (s *ManagedControlPlaneScope) getServiceLbSubnets() []string {
	subnets := make([]string, 0)
	for _, subnet := range s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets {
		if subnet.Role == infrastructurev1beta2.ServiceLoadBalancerRole {
			subnets = append(subnets, *subnet.ID)
		}
	}
	return subnets
}

func (s *ManagedControlPlaneScope) getControlPlaneEndpointSubnet() *string {
	for _, subnet := range s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets {
		if subnet.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			return subnet.ID
		}
	}
	return nil
}

func (s *ManagedControlPlaneScope) getControlPlaneEndpointNSGList() []string {
	nsgs := make([]string, 0)
	for _, nsg := range s.OCIClusterAccessor.GetNetworkSpec().Vcn.NetworkSecurityGroup.List {
		if nsg.Role == infrastructurev1beta2.ControlPlaneEndpointRole {
			nsgs = append(nsgs, *nsg.ID)
		}
	}
	return nsgs
}

// IsControlPlaneEndpointSubnetPublic returns true if the control plane endpoint subnet is public
func (s *ManagedControlPlaneScope) IsControlPlaneEndpointSubnetPublic() bool {
	for _, subnet := range s.OCIClusterAccessor.GetNetworkSpec().Vcn.Subnets {
		if subnet.Role == infrastructurev1beta2.ControlPlaneEndpointRole && subnet.Type == infrastructurev1beta2.Public {
			return true
		}
	}
	return false
}

// DeleteOKECluster deletes an OKE cluster
func (s *ManagedControlPlaneScope) DeleteOKECluster(ctx context.Context, cluster *oke.Cluster) error {
	req := oke.DeleteClusterRequest{ClusterId: cluster.Id}
	_, err := s.ContainerEngineClient.DeleteCluster(ctx, req)
	return err
}

func (s *ManagedControlPlaneScope) createCAPIKubeconfigSecret(ctx context.Context, okeCluster *oke.Cluster, clusterRef types.NamespacedName) error {
	controllerOwnerRef := *metav1.NewControllerRef(s.OCIManagedControlPlane, infrastructurev1beta2.GroupVersion.WithKind("OCIManagedControlPlane"))
	req := oke.CreateKubeconfigRequest{ClusterId: okeCluster.Id}
	response, err := s.ContainerEngineClient.CreateKubeconfig(ctx, req)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(response.Content)
	if err != nil {
		return err
	}
	config, err := clientcmd.NewClientConfigFromBytes(body)
	if err != nil {
		return err
	}
	rawConfig, err := config.RawConfig()
	if err != nil {
		return err
	}
	userName := getKubeConfigUserName(*okeCluster.Name, false)
	currentContext := rawConfig.Contexts[rawConfig.CurrentContext]
	currentCluster := rawConfig.Clusters[currentContext.Cluster]

	cfg, err := createBaseKubeConfig(userName, currentCluster, currentContext.Cluster, rawConfig.CurrentContext)
	if err != nil {
		return err
	}
	token, err := s.BaseClient.GenerateToken(ctx, *okeCluster.Id)
	if err != nil {
		return fmt.Errorf("generating presigned token: %w", err)
	}
	cfg.AuthInfos = map[string]*api.AuthInfo{
		userName: {
			Token: token,
		},
	}
	out, err := clientcmd.Write(*cfg)
	if err != nil {
		return errors.Wrap(err, "failed to serialize config to yaml")
	}
	kubeconfigSecret := kubeconfig.GenerateSecretWithOwner(clusterRef, out, controllerOwnerRef)
	if err := s.client.Create(ctx, kubeconfigSecret); err != nil {
		return errors.Wrap(err, "failed to create kubeconfig secret")
	}
	return err

}

func createBaseKubeConfig(userName string, kubeconfigCluster *api.Cluster, clusterName string, contextName string) (*api.Config, error) {

	cfg := &api.Config{
		APIVersion: api.SchemeGroupVersion.Version,
		Clusters: map[string]*api.Cluster{
			clusterName: {
				Server:                   kubeconfigCluster.Server,
				CertificateAuthorityData: kubeconfigCluster.CertificateAuthorityData,
			},
		},
		Contexts: map[string]*api.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		CurrentContext: contextName,
	}

	return cfg, nil
}

// ReconcileKubeconfig reconciles the kubeconfig secret which will be used by core CAPI components to talk to
// the OKE cluster
func (s *ManagedControlPlaneScope) ReconcileKubeconfig(ctx context.Context, okeCluster *oke.Cluster) error {
	clusterRef := types.NamespacedName{
		Name:      s.Cluster.Name,
		Namespace: s.Cluster.Namespace,
	}

	// Create the kubeconfig used by CAPI
	configSecret, err := secret.GetFromNamespacedName(ctx, s.client, clusterRef, secret.Kubeconfig)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to get kubeconfig secret")
		}

		if createErr := s.createCAPIKubeconfigSecret(
			ctx,
			okeCluster,
			clusterRef,
		); createErr != nil {
			return fmt.Errorf("creating kubeconfig secret: %w", createErr)
		}
	} else if updateErr := s.updateCAPIKubeconfigSecret(ctx, configSecret, okeCluster); updateErr != nil {
		return fmt.Errorf("updating kubeconfig secret: %w", updateErr)
	}

	// Set initialized to true to indicate the kubconfig has been created
	s.OCIManagedControlPlane.Status.Initialized = true

	return nil
}

// ReconcileBootstrapSecret reconciles the bootsrap secret which will be used by self managed nodes
func (s *ManagedControlPlaneScope) ReconcileBootstrapSecret(ctx context.Context, okeCluster *oke.Cluster) error {
	controllerOwnerRef := *metav1.NewControllerRef(s.OCIManagedControlPlane, infrastructurev1beta2.GroupVersion.WithKind("OCIManagedControlPlane"))
	secretName := fmt.Sprintf("%s-self-managed", s.Cluster.Name)
	secretKey := client.ObjectKey{
		Namespace: s.Cluster.Namespace,
		Name:      secretName,
	}
	err := s.client.Get(ctx, secretKey, &corev1.Secret{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to get kubeconfig secret")
		}

		req := oke.CreateKubeconfigRequest{ClusterId: okeCluster.Id}
		response, err := s.ContainerEngineClient.CreateKubeconfig(ctx, req)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(response.Content)
		if err != nil {
			return err
		}
		config, err := clientcmd.NewClientConfigFromBytes(body)

		rawConfig, err := config.RawConfig()
		if err != nil {
			return err
		}
		currentContext := rawConfig.Contexts[rawConfig.CurrentContext]
		currentCluster := rawConfig.Clusters[currentContext.Cluster]
		certData := base64.StdEncoding.EncodeToString(currentCluster.CertificateAuthorityData)

		endpoint := strings.Split(*okeCluster.Endpoints.PrivateEndpoint, ":")[0]
		initStr := strings.Replace(OKEInitScript, "BASE_64_CA", certData, 1)
		initStr = strings.Replace(initStr, "CLUSTER_ENDPOINT", endpoint, 1)

		cluster := s.Cluster
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: cluster.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: cluster.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					controllerOwnerRef,
				},
			},
			Data: map[string][]byte{
				secret.KubeconfigDataName: []byte(initStr),
			},
		}

		if err := s.client.Create(ctx, secret); err != nil {
			return errors.Wrap(err, "failed to create kubeconfig secret")
		}
	}

	return nil
}

func (s *ManagedControlPlaneScope) updateCAPIKubeconfigSecret(ctx context.Context, configSecret *corev1.Secret, okeCluster *oke.Cluster) error {
	data, ok := configSecret.Data[secret.KubeconfigDataName]
	if !ok {
		return errors.Errorf("missing key %q in secret data", secret.KubeconfigDataName)
	}

	config, err := clientcmd.Load(data)
	if err != nil {
		return errors.Wrap(err, "failed to convert kubeconfig Secret into a clientcmdapi.Config")
	}

	token, err := s.BaseClient.GenerateToken(ctx, *okeCluster.Id)
	if err != nil {
		return fmt.Errorf("generating presigned token: %w", err)
	}

	if err != nil {
		return fmt.Errorf("generating presigned token: %w", err)
	}

	userName := getKubeConfigUserName(*okeCluster.Name, false)
	config.AuthInfos[userName].Token = token

	out, err := clientcmd.Write(*config)
	if err != nil {
		return errors.Wrap(err, "failed to serialize config to yaml")
	}

	configSecret.Data[secret.KubeconfigDataName] = out

	err = s.client.Update(ctx, configSecret)
	if err != nil {
		return fmt.Errorf("updating kubeconfig secret: %w", err)
	}

	return nil
}

func (s *ManagedControlPlaneScope) getKeyDetails() []oke.KeyDetails {
	if len(s.OCIManagedControlPlane.Spec.ImagePolicyConfig.KeyDetails) > 0 {
		keys := make([]oke.KeyDetails, 0)
		for _, key := range s.OCIManagedControlPlane.Spec.ImagePolicyConfig.KeyDetails {
			keys = append(keys, oke.KeyDetails{
				KmsKeyId: key.KmsKeyId,
			})
		}
		return keys
	}
	return nil
}

// UpdateControlPlane updates the control plane(OKE cluster) if necessary based on spec changes
func (s *ManagedControlPlaneScope) UpdateControlPlane(ctx context.Context, okeCluster *oke.Cluster) (bool, error) {
	spec := s.OCIManagedControlPlane.Spec.DeepCopy()
	setControlPlaneSpecDefaults(spec)

	actual := s.getSpecFromActual(okeCluster)
	// Log the actual and desired specs
	if !s.compareSpecs(spec, actual) {
		controlPlaneSpec := s.OCIManagedControlPlane.Spec
		updateOptions := oke.UpdateClusterOptionsDetails{}
		if controlPlaneSpec.ClusterOption.AdmissionControllerOptions != nil {
			updateOptions.AdmissionControllerOptions = &oke.AdmissionControllerOptions{
				IsPodSecurityPolicyEnabled: controlPlaneSpec.ClusterOption.AdmissionControllerOptions.IsPodSecurityPolicyEnabled,
			}
		}
		if controlPlaneSpec.ClusterOption.OpenIdConnectDiscovery != nil {
			updateOptions.OpenIdConnectDiscovery = &oke.OpenIdConnectDiscovery{
				IsOpenIdConnectDiscoveryEnabled: controlPlaneSpec.ClusterOption.OpenIdConnectDiscovery.IsOpenIdConnectDiscoveryEnabled,
			}
		}
		if controlPlaneSpec.ClusterOption.OpenIdConnectTokenAuthenticationConfig != nil {
			s.Logger.Info("Updating OIDC Connect Token config")
			oidcConfig := controlPlaneSpec.ClusterOption.OpenIdConnectTokenAuthenticationConfig
			updateOptions.OpenIdConnectTokenAuthenticationConfig = &oke.OpenIdConnectTokenAuthenticationConfig{
				IsOpenIdConnectAuthEnabled: &oidcConfig.IsOpenIdConnectAuthEnabled,
			}

			if oidcConfig.IssuerUrl != nil {
				updateOptions.OpenIdConnectTokenAuthenticationConfig.IssuerUrl = oidcConfig.IssuerUrl
			}
			if oidcConfig.ClientId != nil {
				updateOptions.OpenIdConnectTokenAuthenticationConfig.ClientId = oidcConfig.ClientId
			}
			if oidcConfig.UsernameClaim != nil {
				updateOptions.OpenIdConnectTokenAuthenticationConfig.UsernameClaim = oidcConfig.UsernameClaim
			}
			if oidcConfig.UsernamePrefix != nil {
				updateOptions.OpenIdConnectTokenAuthenticationConfig.UsernamePrefix = oidcConfig.UsernamePrefix
			}
			if oidcConfig.GroupsClaim != nil {
				updateOptions.OpenIdConnectTokenAuthenticationConfig.GroupsClaim = oidcConfig.GroupsClaim
			}
			if oidcConfig.GroupsPrefix != nil {
				updateOptions.OpenIdConnectTokenAuthenticationConfig.GroupsPrefix = oidcConfig.GroupsPrefix
			}
			if oidcConfig.RequiredClaims != nil {
				// Convert []infrastructurev1beta2.KeyValue to []containerengine.KeyValue
				requiredClaims := make([]oke.KeyValue, len(oidcConfig.RequiredClaims))
				for i, rc := range oidcConfig.RequiredClaims {
					requiredClaims[i] = oke.KeyValue(rc)
				}
				updateOptions.OpenIdConnectTokenAuthenticationConfig.RequiredClaims = requiredClaims
			}
			if oidcConfig.CaCertificate != nil {
				updateOptions.OpenIdConnectTokenAuthenticationConfig.CaCertificate = oidcConfig.CaCertificate
			}
			if oidcConfig.SigningAlgorithms != nil {
				updateOptions.OpenIdConnectTokenAuthenticationConfig.SigningAlgorithms = oidcConfig.SigningAlgorithms
			}

			s.Logger.Info("Updated OIDC Connect Token config", "config", updateOptions.OpenIdConnectTokenAuthenticationConfig)
		}
		details := oke.UpdateClusterDetails{
			Name:              common.String(s.GetClusterName()),
			KubernetesVersion: controlPlaneSpec.Version,
			Options:           &updateOptions,
		}
		if controlPlaneSpec.ImagePolicyConfig != nil {
			details.ImagePolicyConfig = &oke.UpdateImagePolicyConfigDetails{
				IsPolicyEnabled: s.OCIManagedControlPlane.Spec.ImagePolicyConfig.IsPolicyEnabled,
				KeyDetails:      s.getKeyDetails(),
			}
		}
		clusterType := getOKEClusterTypeFromSpecType(controlPlaneSpec)
		details.Type = clusterType
		updateClusterRequest := oke.UpdateClusterRequest{
			ClusterId:            okeCluster.Id,
			UpdateClusterDetails: details,
		}
		_, err := s.ContainerEngineClient.UpdateCluster(ctx, updateClusterRequest)
		if err != nil {
			return false, errors.Wrapf(err, "failed to update cluster")
		}

		s.Info("Updated control plane")
		return true, nil
	} else {
		s.Info("No reconciliation needed for control plane")
	}
	return false, nil
}

// compareSpecs compares two OCIManagedControlPlaneSpec objects for equality
func (s *ManagedControlPlaneScope) compareSpecs(spec1, spec2 *infrastructurev1beta2.OCIManagedControlPlaneSpec) bool {
	if spec1 == nil || spec2 == nil {
		return spec1 == spec2
	}

	// Use go-cmp to compare the specs
	equal := cmp.Equal(spec1, spec2, cmpopts.EquateEmpty())
	if !equal {
		diff := cmp.Diff(spec1, spec2, cmpopts.EquateEmpty())
		s.Logger.Info("Specs are different", "diff", diff)
	}
	return equal
}

// setControlPlaneSpecDefaults sets the defaults in the spec as returned by OKE API. We need to set defaults here rather than webhook as well as
// there is a chance user will edit the cluster
func setControlPlaneSpecDefaults(spec *infrastructurev1beta2.OCIManagedControlPlaneSpec) {
	spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{}
	if spec.ClusterType == "" {
		spec.ClusterType = infrastructurev1beta2.BasicClusterType
	}
	spec.Addons = nil
	if spec.ImagePolicyConfig == nil {
		spec.ImagePolicyConfig = &infrastructurev1beta2.ImagePolicyConfig{
			IsPolicyEnabled: common.Bool(false),
			KeyDetails:      make([]infrastructurev1beta2.KeyDetails, 0),
		}
	}
	if spec.ClusterOption.AdmissionControllerOptions == nil {
		spec.ClusterOption.AdmissionControllerOptions = &infrastructurev1beta2.AdmissionControllerOptions{
			IsPodSecurityPolicyEnabled: common.Bool(false),
		}
	}
	if spec.ClusterOption.AddOnOptions == nil {
		spec.ClusterOption.AddOnOptions = &infrastructurev1beta2.AddOnOptions{
			IsTillerEnabled:              common.Bool(false),
			IsKubernetesDashboardEnabled: common.Bool(false),
		}
	}
}

func (s *ManagedControlPlaneScope) getSpecFromActual(cluster *oke.Cluster) *infrastructurev1beta2.OCIManagedControlPlaneSpec {
	spec := infrastructurev1beta2.OCIManagedControlPlaneSpec{
		Version:  cluster.KubernetesVersion,
		KmsKeyId: cluster.KmsKeyId,
		ID:       cluster.Id,
		Addons:   nil,
	}
	if cluster.ImagePolicyConfig != nil {
		keys := make([]infrastructurev1beta2.KeyDetails, 0)
		for _, key := range cluster.ImagePolicyConfig.KeyDetails {
			keys = append(keys, infrastructurev1beta2.KeyDetails{
				KmsKeyId: key.KmsKeyId,
			})
		}
		spec.ImagePolicyConfig = &infrastructurev1beta2.ImagePolicyConfig{
			IsPolicyEnabled: cluster.ImagePolicyConfig.IsPolicyEnabled,
			KeyDetails:      keys,
		}
	}
	if len(cluster.ClusterPodNetworkOptions) > 0 {
		podNetworks := make([]infrastructurev1beta2.ClusterPodNetworkOptions, 0)
		for _, cniOption := range cluster.ClusterPodNetworkOptions {
			_, ok := cniOption.(oke.OciVcnIpNativeClusterPodNetworkOptionDetails)
			if ok {
				podNetworks = append(podNetworks, infrastructurev1beta2.ClusterPodNetworkOptions{
					CniType: infrastructurev1beta2.VCNNativeCNI,
				})
			} else {
				podNetworks = append(podNetworks, infrastructurev1beta2.ClusterPodNetworkOptions{
					CniType: infrastructurev1beta2.FlannelCNI,
				})
			}
		}
		spec.ClusterPodNetworkOptions = podNetworks
	}
	if cluster.Options != nil {
		if cluster.Options.AdmissionControllerOptions != nil {
			spec.ClusterOption.AdmissionControllerOptions = &infrastructurev1beta2.AdmissionControllerOptions{
				IsPodSecurityPolicyEnabled: cluster.Options.AdmissionControllerOptions.IsPodSecurityPolicyEnabled,
			}
		}
		if cluster.Options.AddOns != nil {
			spec.ClusterOption.AddOnOptions = &infrastructurev1beta2.AddOnOptions{
				IsTillerEnabled:              cluster.Options.AddOns.IsTillerEnabled,
				IsKubernetesDashboardEnabled: cluster.Options.AddOns.IsKubernetesDashboardEnabled,
			}
		}
		if cluster.Options.OpenIdConnectDiscovery != nil {
			spec.ClusterOption.OpenIdConnectDiscovery = &infrastructurev1beta2.OpenIDConnectDiscovery{
				IsOpenIdConnectDiscoveryEnabled: cluster.Options.OpenIdConnectDiscovery.IsOpenIdConnectDiscoveryEnabled,
			}
		}
		if cluster.Options.OpenIdConnectTokenAuthenticationConfig != nil {
			oidcConfig := cluster.Options.OpenIdConnectTokenAuthenticationConfig
			requiredClaims := make([]infrastructurev1beta2.KeyValue, len(oidcConfig.RequiredClaims))
			for i, rc := range oidcConfig.RequiredClaims {
				requiredClaims[i] = infrastructurev1beta2.KeyValue(rc)
			}
			spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig = &infrastructurev1beta2.OpenIDConnectTokenAuthenticationConfig{
				IsOpenIdConnectAuthEnabled: *oidcConfig.IsOpenIdConnectAuthEnabled,
				IssuerUrl:                  oidcConfig.IssuerUrl,
				ClientId:                   oidcConfig.ClientId,
				UsernameClaim:              oidcConfig.UsernameClaim,
				UsernamePrefix:             oidcConfig.UsernamePrefix,
				GroupsClaim:                oidcConfig.GroupsClaim,
				GroupsPrefix:               oidcConfig.GroupsPrefix,
				RequiredClaims:             requiredClaims,
				CaCertificate:              oidcConfig.CaCertificate,
				SigningAlgorithms:          oidcConfig.SigningAlgorithms,
			}
		}
	}
	if cluster.Type != "" {
		switch cluster.Type {
		case oke.ClusterTypeBasicCluster:
			spec.ClusterType = infrastructurev1beta2.BasicClusterType
			break
		case oke.ClusterTypeEnhancedCluster:
			spec.ClusterType = infrastructurev1beta2.EnhancedClusterType
			break
		default:
			spec.ClusterType = infrastructurev1beta2.BasicClusterType
			break
		}
	}
	return &spec
}

// ReconcileAddons reconciles addons which have been specified in the spec on the OKE cluster
func (s *ManagedControlPlaneScope) ReconcileAddons(ctx context.Context, okeCluster *oke.Cluster) error {
	addonSpec := s.OCIManagedControlPlane.Spec.Addons
	// go through the list of addons present in the spec and reconcile them, reconcile can be 2 ways, either
	// install the addon if it has not been installed till now, or update the addon
	for _, addon := range addonSpec {
		resp, err := s.ContainerEngineClient.GetAddon(ctx, oke.GetAddonRequest{
			ClusterId: okeCluster.Id,
			AddonName: addon.Name,
		})
		if err != nil {
			// addon is not present, hence install it
			if ociutil.IsNotFound(err) {
				s.Info(fmt.Sprintf("Install addon %s", *addon.Name))
				_, err = s.ContainerEngineClient.InstallAddon(ctx, oke.InstallAddonRequest{
					ClusterId: okeCluster.Id,
					InstallAddonDetails: oke.InstallAddonDetails{
						Version:        addon.Version,
						Configurations: getAddonConfigurations(addon.Configurations),
						AddonName:      addon.Name,
					},
				})
				if err != nil {
					return err
				}
				// add it to status, details will be reconciled in next loop
				status := infrastructurev1beta2.AddonStatus{
					LifecycleState: common.String(string(oke.AddonLifecycleStateCreating)),
				}
				s.OCIManagedControlPlane.SetAddonStatus(*addon.Name, status)
			} else {
				return err
			}
		} else {
			s.OCIManagedControlPlane.SetAddonStatus(*addon.Name, s.getStatus(resp.Addon))
			// addon present, update it
			err = s.handleExistingAddon(ctx, okeCluster, resp.Addon, addon)
			if err != nil {
				return err
			}
		}
	}
	// for addons which are present in the status object but not in the spec, the possibility
	// is that user deleted it from the spec. Hence disable the addon
	for k, _ := range s.OCIManagedControlPlane.Status.AddonStatus {
		// present in status but not in spec
		if getAddon(addonSpec, k) == nil {
			err := s.handleDeletedAddon(ctx, okeCluster, k)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ManagedControlPlaneScope) getStatus(addon oke.Addon) infrastructurev1beta2.AddonStatus {
	// update status of the addon
	status := infrastructurev1beta2.AddonStatus{
		LifecycleState:            common.String(string(addon.LifecycleState)),
		CurrentlyInstalledVersion: addon.CurrentInstalledVersion,
	}
	if addon.AddonError != nil {
		status.AddonError = &infrastructurev1beta2.AddonError{
			Status:  addon.AddonError.Status,
			Code:    addon.AddonError.Code,
			Message: addon.AddonError.Message,
		}
	}
	return status
}

func (s *ManagedControlPlaneScope) handleExistingAddon(ctx context.Context, okeCluster *oke.Cluster, addon oke.Addon, addonInSpec infrastructurev1beta2.Addon) error {
	// if the addon can be updated do so
	// if the addon is already in updating state, or in failed state, do not update
	s.Info(fmt.Sprintf("Reconciling addon %s with lifecycle state %s", *addon.Name, string(addon.LifecycleState)))
	if !(addon.LifecycleState == oke.AddonLifecycleStateUpdating ||
		addon.LifecycleState == oke.AddonLifecycleStateFailed) {
		addonConfigurationsActual := getActualAddonConfigurations(addon.Configurations)
		// if the version changed or the configuration changed, update the addon
		// if the lifecycle state is needs attention, try to update
		if addon.LifecycleState == oke.AddonLifecycleStateNeedsAttention ||
			!reflect.DeepEqual(addonInSpec.Version, addon.Version) ||
			!reflect.DeepEqual(addonConfigurationsActual, addonInSpec.Configurations) {
			s.Info(fmt.Sprintf("Updating addon %s", *addon.Name))
			_, err := s.ContainerEngineClient.UpdateAddon(ctx, oke.UpdateAddonRequest{
				ClusterId: okeCluster.Id,
				AddonName: addon.Name,
				UpdateAddonDetails: oke.UpdateAddonDetails{
					Version:        addonInSpec.Version,
					Configurations: getAddonConfigurations(addonInSpec.Configurations),
				},
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ManagedControlPlaneScope) handleDeletedAddon(ctx context.Context, okeCluster *oke.Cluster, addonName string) error {
	resp, err := s.ContainerEngineClient.GetAddon(ctx, oke.GetAddonRequest{
		ClusterId: okeCluster.Id,
		AddonName: common.String(addonName),
	})
	if err != nil {
		if ociutil.IsNotFound(err) {
			s.OCIManagedControlPlane.RemoveAddonStatus(addonName)
			return nil
		} else {
			return err
		}
	}
	addonState := resp.LifecycleState
	switch addonState {
	// nothing to do if addon is in deleting state
	case oke.AddonLifecycleStateDeleting:
		s.Info(fmt.Sprintf("Addon %s is in deleting state", addonName))
		break
	case oke.AddonLifecycleStateDeleted:
		// delete addon from status if addon has been deleted
		s.Info(fmt.Sprintf("Addon %s is in deleted state", addonName))
		s.OCIManagedControlPlane.RemoveAddonStatus(addonName)
		break
	default:
		// else delete the addon
		// delete addon is called disable addon with remove flag turned on
		_, err := s.ContainerEngineClient.DisableAddon(ctx, oke.DisableAddonRequest{
			ClusterId:             okeCluster.Id,
			AddonName:             common.String(addonName),
			IsRemoveExistingAddOn: common.Bool(true),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func getAddonConfigurations(configurations []infrastructurev1beta2.AddonConfiguration) []oke.AddonConfiguration {
	if len(configurations) == 0 {
		return nil
	}
	config := make([]oke.AddonConfiguration, len(configurations))
	for i, c := range configurations {
		config[i] = oke.AddonConfiguration{
			Key:   c.Key,
			Value: c.Value,
		}
	}
	return config
}

func getActualAddonConfigurations(addonConfigurations []oke.AddonConfiguration) []infrastructurev1beta2.AddonConfiguration {
	if len(addonConfigurations) == 0 {
		return nil
	}
	config := make([]infrastructurev1beta2.AddonConfiguration, len(addonConfigurations))
	for i, c := range addonConfigurations {
		config[i] = infrastructurev1beta2.AddonConfiguration{
			Key:   c.Key,
			Value: c.Value,
		}
	}
	return config
}

func getAddon(addons []infrastructurev1beta2.Addon, name string) *infrastructurev1beta2.Addon {
	for i, addon := range addons {
		if *addon.Name == name {
			return &addons[i]
		}
	}
	return nil
}

func getKubeConfigUserName(clusterName string, isUser bool) string {
	if isUser {
		return fmt.Sprintf("%s-user", clusterName)
	}

	return fmt.Sprintf("%s-capi-admin", clusterName)
}
