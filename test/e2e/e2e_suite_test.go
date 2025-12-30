//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	oci_config "github.com/oracle/cluster-api-provider-oci/cloud/config"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/compute"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/containerengine"
	nlb "github.com/oracle/cluster-api-provider-oci/cloud/services/networkloadbalancer"
	"github.com/oracle/cluster-api-provider-oci/cloud/services/vcn"
	infrav1exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta1"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1exp "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	AntreaCNIPath                   = "ANTREA_CNI"
	ManagedKubernetesUpgradeVersion = "OCI_MANAGED_KUBERNETES_VERSION_UPGRADE"
)

// Test suite flags
var (
	// configPath is the path to the e2e config file.
	configPath string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool
)

// Test suite global vars
var (
	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// bootstrapClusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	bootstrapClusterProvider bootstrap.ClusterProvider

	// bootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	bootstrapClusterProxy framework.ClusterProxy

	// kubetestConfigFilePath is the path to the kubetest configuration file
	kubetestConfigFilePath string

	// kubetestRepoListPath
	kubetestRepoListPath string

	// useCIArtifacts specifies whether or not to use the latest build from the main branch of the Kubernetes repository
	useCIArtifacts bool

	// usePRArtifacts specifies whether or not to use the build from a PR of the Kubernetes repository
	usePRArtifacts bool

	computeClient compute.ComputeClient

	vcnClient vcn.Client

	lbClient nlb.NetworkLoadBalancerClient

	okeClient containerengine.Client

	adCount int
)

type (
	OCIClusterProxy struct {
		framework.ClusterProxy
	}
)

func NewOCIClusterProxy(name string, kubeconfigPath string, scheme *runtime.Scheme, options ...framework.Option) *OCIClusterProxy {
	proxy, ok := framework.NewClusterProxy(name, kubeconfigPath, scheme, options...).(framework.ClusterProxy)
	Expect(ok).To(BeTrue(), "framework.NewClusterProxy must implement capi_e2e.ClusterProxy")
	return &OCIClusterProxy{
		ClusterProxy: proxy,
	}
}

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder where e2e test artifact should be stored")
	flag.BoolVar(&useCIArtifacts, "kubetest.use-ci-artifacts", false, "use the latest build from the main branch of the Kubernetes repository. Set KUBERNETES_VERSION environment variable to latest-1.xx to use the build from 1.xx release branch.")
	flag.BoolVar(&usePRArtifacts, "kubetest.use-pr-artifacts", false, "use the build from a PR of the Kubernetes repository")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
	flag.StringVar(&kubetestConfigFilePath, "kubetest.config-file", "", "path to the kubetest configuration file")
	flag.StringVar(&kubetestRepoListPath, "kubetest.repo-list-path", "", "path to the kubetest repo-list path")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	// Before all ParallelNodes.

	ctrl.SetLogger(klog.Background())

	RunSpecs(t, "capoci-e2e")
}

// Using a SynchronizedBeforeSuite for controlling how to create resources shared across ParallelNodes (~ginkgo threads).
// The local clusterctl repository & the bootstrap cluster are created once and shared across all the tests.
var _ = SynchronizedBeforeSuite(func() []byte {
	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder)

	By("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()

	By(fmt.Sprintf("Loading the e2e test configuration from %q", configPath))
	e2eConfig = loadE2EConfig(configPath)

	By(fmt.Sprintf("Creating a clusterctl local repository into %q", artifactFolder))
	clusterctlConfigPath = createClusterctlLocalRepository(e2eConfig, filepath.Join(artifactFolder, "repository"))

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy = setupBootstrapCluster(e2eConfig, scheme, useExistingCluster)

	By("Initializing the bootstrap cluster")
	initBootstrapCluster(bootstrapClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)

	return []byte(
		strings.Join([]string{
			artifactFolder,
			configPath,
			clusterctlConfigPath,
			bootstrapClusterProxy.GetKubeconfigPath(),
		}, ","),
	)
}, func(data []byte) {
	// Before each ParallelNode.

	parts := strings.Split(string(data), ",")
	Expect(parts).To(HaveLen(4))

	artifactFolder = parts[0]
	configPath = parts[1]
	clusterctlConfigPath = parts[2]
	kubeconfigPath := parts[3]

	e2eConfig = loadE2EConfig(configPath)
	bootstrapClusterProxy = NewOCIClusterProxy("bootstrap", kubeconfigPath, initScheme())

	s, useInstancePrincipalFlagSet := os.LookupEnv("USE_INSTANCE_PRINCIPAL_B64")
	useInstanePrincipal := false
	if useInstancePrincipalFlagSet {
		useInstanePrincipalStr, err := base64.StdEncoding.DecodeString(s)
		Expect(err).NotTo(HaveOccurred())
		useInstanePrincipal, err = strconv.ParseBool(string(useInstanePrincipalStr))
		Expect(err).NotTo(HaveOccurred())
	}
	var ociAuthConfigProvider common.ConfigurationProvider
	var err error
	if useInstanePrincipal {
		ociAuthConfigProvider, err = oci_config.NewConfigurationProvider(&oci_config.AuthConfig{
			UseInstancePrincipals: true,
		})
		Expect(err).NotTo(HaveOccurred())
		By("Using instance principal as auth provider")
	} else {
		tenancyId, err := base64.StdEncoding.DecodeString(os.Getenv("OCI_TENANCY_ID_B64"))
		Expect(err).NotTo(HaveOccurred())
		userId, err := base64.StdEncoding.DecodeString(os.Getenv("OCI_USER_ID_B64"))
		Expect(err).NotTo(HaveOccurred())
		passphrase, err := base64.StdEncoding.DecodeString(os.Getenv("OCI_CREDENTIALS_PASSPHRASE_B64"))
		Expect(err).NotTo(HaveOccurred())
		key, err := base64.StdEncoding.DecodeString(os.Getenv("OCI_CREDENTIALS_KEY_B64"))
		Expect(err).NotTo(HaveOccurred())
		fingerprint, err := base64.StdEncoding.DecodeString(os.Getenv("OCI_CREDENTIALS_FINGERPRINT_B64"))
		Expect(err).NotTo(HaveOccurred())
		region, err := base64.StdEncoding.DecodeString(os.Getenv("OCI_REGION_B64"))
		Expect(err).NotTo(HaveOccurred())

		ociAuthConfigProvider, err = oci_config.NewConfigurationProvider(&oci_config.AuthConfig{
			Region:                string(region),
			TenancyID:             string(tenancyId),
			UserID:                string(userId),
			PrivateKey:            string(key),
			Fingerprint:           string(fingerprint),
			Passphrase:            string(passphrase),
			UseInstancePrincipals: false,
		})
		Expect(err).NotTo(HaveOccurred())
		By("Using user principal as auth provider")
	}

	clientProvider, err := scope.NewClientProvider(scope.ClientProviderParams{OciAuthConfigProvider: ociAuthConfigProvider})
	Expect(err).NotTo(HaveOccurred())

	region, err := ociAuthConfigProvider.Region()
	Expect(err).NotTo(HaveOccurred())
	ociClients, err := clientProvider.GetOrBuildClient(region)
	Expect(err).NotTo(HaveOccurred())

	computeClient = ociClients.ComputeClient
	identityClient := ociClients.IdentityClient
	vcnClient = ociClients.VCNClient
	lbClient = ociClients.NetworkLoadBalancerClient
	okeClient = ociClients.ContainerEngineClient
	Expect(identityClient).NotTo(BeNil())
	Expect(lbClient).NotTo(BeNil())

	req := identity.ListAvailabilityDomainsRequest{CompartmentId: common.String(os.Getenv("OCI_COMPARTMENT_ID"))}
	resp, err := identityClient.ListAvailabilityDomains(context.Background(), req)
	adCount = len(resp.Items)
})

// Using a SynchronizedAfterSuite for controlling how to delete resources shared across ParallelNodes (~ginkgo threads).
// The bootstrap cluster is shared across all the tests, so it should be deleted only after all ParallelNodes completes.
// The local clusterctl repository is preserved like everything else created into the artifact folder.
var _ = SynchronizedAfterSuite(func() {
	// After each ParallelNode.
}, func() {
	// After all ParallelNodes.

	By("Tearing down the management cluster")
	if !skipCleanup {
		tearDown(bootstrapClusterProvider, bootstrapClusterProxy)
	}
})

func initScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	framework.TryAddDefaultSchemes(scheme)
	Expect(infrastructurev1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(infrastructurev1beta2.AddToScheme(scheme)).To(Succeed())
	Expect(infrav1exp.AddToScheme(scheme)).To(Succeed())
	Expect(infrav2exp.AddToScheme(scheme)).To(Succeed())
	Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
	Expect(clusterv1exp.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func loadE2EConfig(configPath string) *clusterctl.E2EConfig {
	config := clusterctl.LoadE2EConfig(context.TODO(), clusterctl.LoadE2EConfigInput{ConfigPath: configPath})
	Expect(config).ToNot(BeNil(), "Failed to load E2E config from %s", configPath)

	return config
}

func createClusterctlLocalRepository(config *clusterctl.E2EConfig, repositoryFolder string) string {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}

	// Ensuring a CNI file is defined in the config and register a FileTransformation to inject the referenced file as in place of the CNI_RESOURCES envSubst variable.
	Expect(config.Variables).To(HaveKey(capi_e2e.CNIPath), "Missing %s variable in the config", capi_e2e.CNIPath)
	cniPath := config.MustGetVariable(capi_e2e.CNIPath)
	Expect(cniPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", capi_e2e.CNIPath)
	createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(cniPath, capi_e2e.CNIResources)

	Expect(config.Variables).To(HaveKey(AntreaCNIPath), "Missing %s variable in the config", AntreaCNIPath)
	antreaCniPath := config.MustGetVariable(AntreaCNIPath)
	Expect(cniPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", AntreaCNIPath)
	createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(antreaCniPath, "ANTREA_RESOURCES")

	clusterctlConfig := clusterctl.CreateRepository(context.TODO(), createRepositoryInput)
	Expect(clusterctlConfig).To(BeAnExistingFile(), "The clusterctl config file does not exists in the local repository %s", repositoryFolder)

	return clusterctlConfig
}

func setupBootstrapCluster(config *clusterctl.E2EConfig, scheme *runtime.Scheme, useExistingCluster bool) (bootstrap.ClusterProvider, framework.ClusterProxy) {
	var clusterProvider bootstrap.ClusterProvider
	kubeconfigPath := ""
	if !useExistingCluster {
		clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(context.TODO(), bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               config.ManagementClusterName,
			RequiresDockerSock: config.HasDockerProvider(),
			Images:             config.Images,
		})
		Expect(clusterProvider).ToNot(BeNil(), "Failed to create a bootstrap cluster")

		kubeconfigPath = clusterProvider.GetKubeconfigPath()
		Expect(kubeconfigPath).To(BeAnExistingFile(), "Failed to get the kubeconfig file for the bootstrap cluster")
	}

	clusterProxy := NewOCIClusterProxy("bootstrap", kubeconfigPath, scheme)
	Expect(clusterProxy).ToNot(BeNil(), "Failed to get a bootstrap cluster proxy")

	return clusterProvider, clusterProxy
}

func initBootstrapCluster(bootstrapClusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig, clusterctlConfig, artifactFolder string) {
	clusterctl.InitManagementClusterAndWatchControllerLogs(context.TODO(), clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:            bootstrapClusterProxy,
		ClusterctlConfigPath:    clusterctlConfig,
		InfrastructureProviders: config.InfrastructureProviders(),
		LogFolder:               filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
	}, config.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
}

func tearDown(bootstrapClusterProvider bootstrap.ClusterProvider, bootstrapClusterProxy framework.ClusterProxy) {
	if bootstrapClusterProxy != nil {
		bootstrapClusterProxy.Dispose(context.TODO())
	}
	if bootstrapClusterProvider != nil {
		bootstrapClusterProvider.Dispose(context.TODO())
	}
}
