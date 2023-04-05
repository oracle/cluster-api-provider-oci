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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/go-logr/logr"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	baseclient "github.com/oracle/cluster-api-provider-oci/cloud/services/base"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
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

// ManagedControlPlaneScopeParams defines the params need to create a new ManagedControlPlaneScope
type ManagedControlPlaneScopeParams struct {
	Logger                 *logr.Logger
	Client                 client.Client
	Cluster                *clusterv1.Cluster
	ContainerEngineClient  containerengine.Client
	BaseClient             baseclient.BaseClient
	ClientProvider         *ClientProvider
	OCIManagedControlPlane *infrav2exp.OCIManagedControlPlane
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
	OCIManagedControlPlane *infrav2exp.OCIManagedControlPlane
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
			if cniOption.CniType == infrav2exp.FlannelCNI {
				podNetworks = append(podNetworks, oke.FlannelOverlayClusterPodNetworkOptionDetails{})
			} else if cniOption.CniType == infrav2exp.VCNNativeCNI {
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
	var clusterType oke.ClusterTypeEnum
	if controlPlaneSpec.ClusterType != "" {
		switch controlPlaneSpec.ClusterType {
		case infrav2exp.BasicClusterType:
			clusterType = oke.ClusterTypeBasicCluster
			break
		case infrav2exp.EnhancedClusterType:
			clusterType = oke.ClusterTypeEnhancedCluster
			break
		default:
			break
		}
	}

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
		Type:                     oke.ClusterTypeEnhancedCluster,
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
	body, err := ioutil.ReadAll(response.Content)
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
	if !reflect.DeepEqual(spec, actual) {
		// printing json specs will help debug problems when there are spurious/unwanted updates
		jsonSpec, err := json.Marshal(*spec)
		if err != nil {
			return false, err
		}
		jsonActual, err := json.Marshal(*actual)
		if err != nil {
			return false, err
		}
		s.Logger.Info("Control plane", "spec", jsonSpec, "actual", jsonActual)
		controlPlaneSpec := s.OCIManagedControlPlane.Spec
		updateOptions := oke.UpdateClusterOptionsDetails{}
		if controlPlaneSpec.ClusterOption.AdmissionControllerOptions != nil {
			updateOptions.AdmissionControllerOptions = &oke.AdmissionControllerOptions{
				IsPodSecurityPolicyEnabled: controlPlaneSpec.ClusterOption.AdmissionControllerOptions.IsPodSecurityPolicyEnabled,
			}
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
		var clusterType oke.ClusterTypeEnum
		if controlPlaneSpec.ClusterType != "" {
			switch controlPlaneSpec.ClusterType {
			case infrav2exp.BasicClusterType:
				clusterType = oke.ClusterTypeBasicCluster
				break
			case infrav2exp.EnhancedClusterType:
				clusterType = oke.ClusterTypeEnhancedCluster
				break
			default:
				break
			}
		}
		details.Type = clusterType
		updateClusterRequest := oke.UpdateClusterRequest{
			ClusterId:            okeCluster.Id,
			UpdateClusterDetails: details,
		}
		_, err = s.ContainerEngineClient.UpdateCluster(ctx, updateClusterRequest)
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

// setControlPlaneSpecDefaults sets the defaults in the spec as returned by OKE API. We need to set defaults here rather than webhook as well as
// there is a chance user will edit the cluster
func setControlPlaneSpecDefaults(spec *infrav2exp.OCIManagedControlPlaneSpec) {
	spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{}
	if spec.ImagePolicyConfig == nil {
		spec.ImagePolicyConfig = &infrav2exp.ImagePolicyConfig{
			IsPolicyEnabled: common.Bool(false),
			KeyDetails:      make([]infrav2exp.KeyDetails, 0),
		}
	}
	if spec.ClusterOption.AdmissionControllerOptions == nil {
		spec.ClusterOption.AdmissionControllerOptions = &infrav2exp.AdmissionControllerOptions{
			IsPodSecurityPolicyEnabled: common.Bool(false),
		}
	}
	if spec.ClusterOption.AddOnOptions == nil {
		spec.ClusterOption.AddOnOptions = &infrav2exp.AddOnOptions{
			IsTillerEnabled:              common.Bool(false),
			IsKubernetesDashboardEnabled: common.Bool(false),
		}
	}
}

func (s *ManagedControlPlaneScope) getSpecFromActual(cluster *oke.Cluster) *infrav2exp.OCIManagedControlPlaneSpec {
	spec := infrav2exp.OCIManagedControlPlaneSpec{
		Version:  cluster.KubernetesVersion,
		KmsKeyId: cluster.KmsKeyId,
		ID:       cluster.Id,
	}
	if cluster.ImagePolicyConfig != nil {
		keys := make([]infrav2exp.KeyDetails, 0)
		for _, key := range cluster.ImagePolicyConfig.KeyDetails {
			keys = append(keys, infrav2exp.KeyDetails{
				KmsKeyId: key.KmsKeyId,
			})
		}
		spec.ImagePolicyConfig = &infrav2exp.ImagePolicyConfig{
			IsPolicyEnabled: cluster.ImagePolicyConfig.IsPolicyEnabled,
			KeyDetails:      keys,
		}
	}
	if len(cluster.ClusterPodNetworkOptions) > 0 {
		podNetworks := make([]infrav2exp.ClusterPodNetworkOptions, 0)
		for _, cniOption := range cluster.ClusterPodNetworkOptions {
			_, ok := cniOption.(oke.OciVcnIpNativeClusterPodNetworkOptionDetails)
			if ok {
				podNetworks = append(podNetworks, infrav2exp.ClusterPodNetworkOptions{
					CniType: infrav2exp.VCNNativeCNI,
				})
			} else {
				podNetworks = append(podNetworks, infrav2exp.ClusterPodNetworkOptions{
					CniType: infrav2exp.FlannelCNI,
				})
			}
		}
		spec.ClusterPodNetworkOptions = podNetworks
	}
	if cluster.Options != nil {
		if cluster.Options.AdmissionControllerOptions != nil {
			spec.ClusterOption.AdmissionControllerOptions = &infrav2exp.AdmissionControllerOptions{
				IsPodSecurityPolicyEnabled: cluster.Options.AdmissionControllerOptions.IsPodSecurityPolicyEnabled,
			}
		}
		if cluster.Options.AddOns != nil {
			spec.ClusterOption.AddOnOptions = &infrav2exp.AddOnOptions{
				IsTillerEnabled:              cluster.Options.AddOns.IsTillerEnabled,
				IsKubernetesDashboardEnabled: cluster.Options.AddOns.IsKubernetesDashboardEnabled,
			}
		}
	}
	return &spec
}

func getKubeConfigUserName(clusterName string, isUser bool) string {
	if isUser {
		return fmt.Sprintf("%s-user", clusterName)
	}

	return fmt.Sprintf("%s-capi-admin", clusterName)
}
