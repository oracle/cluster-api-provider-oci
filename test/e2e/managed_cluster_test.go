//go:build e2e
// +build e2e

/*
 Copyright (c) 2023 Oracle and/or its affiliates.

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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kind/pkg/errors"
)

const (
	retryableOperationInterval = 3 * time.Second
	// retryableOperationTimeout requires a higher value especially for self-hosted upgrades.
	// Short unavailability of the Kube APIServer due to joining etcd members paired with unreachable conversion webhooks due to
	// failed leader election and thus controller restarts lead to longer taking retries.
	// The timeout occurs when listing machines in `GetControlPlaneMachinesByCluster`.
	retryableOperationTimeout = 3 * time.Minute
)

var _ = Describe("Managed Workload cluster creation", func() {
	var (
		ctx               = context.TODO()
		specName          = "create-managed-workload-cluster"
		namespace         *corev1.Namespace
		cancelWatches     context.CancelFunc
		result            *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName       string
		clusterNamePrefix string
		additionalCleanup func()
	)

	BeforeEach(func() {

		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersion))

		// CLUSTER_NAME and CLUSTER_NAMESPACE allows for testing existing clusters
		// if CLUSTER_NAMESPACE is set don't generate a new prefix otherwise
		// the correct namespace won't be found and a new cluster will be created
		clusterNameSpace := os.Getenv("CLUSTER_NAMESPACE")
		if clusterNameSpace == "" {
			clusterNamePrefix = fmt.Sprintf("capoci-e2e-%s", util.RandomString(3))
		} else {
			clusterNamePrefix = clusterNameSpace
		}

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		var err error
		namespace, cancelWatches, err = setupSpecNamespace(ctx, clusterNamePrefix, bootstrapClusterProxy, artifactFolder)
		Expect(err).NotTo(HaveOccurred())

		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)
		additionalCleanup = nil

	})

	AfterEach(func() {
		if result.Cluster == nil {
			// this means the cluster failed to come up. We make an attempt to find the cluster to be able to fetch logs for the failed bootstrapping.
			_ = bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{Name: clusterName, Namespace: namespace.Name}, result.Cluster)
		}

		cleanInput := cleanupInput{
			SpecName:          specName,
			Cluster:           result.Cluster,
			ClusterProxy:      bootstrapClusterProxy,
			Namespace:         namespace,
			ClusterctlConfigPath: clusterctlConfigPath,
			CancelWatches:     cancelWatches,
			IntervalsGetter:   e2eConfig.GetIntervals,
			SkipCleanup:       skipCleanup,
			AdditionalCleanup: additionalCleanup,
			ArtifactFolder:    artifactFolder,
		}
		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})

	It("Managed Cluster - Simple [PRBlocking]", func() {
		clusterName = getClusterName(clusterNamePrefix, "managed")
		input := clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "managed",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}
		input.WaitForControlPlaneInitialized = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoveryAndWaitForControlPlaneInitialized")
			lister := input.ClusterProxy.GetClient()
			Expect(lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoveryAndWaitForControlPlaneInitialized")
			var controlPlane *infrastructurev1beta2.OCIManagedControlPlane
			Eventually(func(g Gomega) {
				controlPlane = GetOCIManagedControlPlaneByCluster(ctx, lister, result.Cluster.Name, result.Cluster.Namespace)
				if controlPlane != nil {
					Log(fmt.Sprintf("Control plane is not nil, status is %t", controlPlane.Status.Ready))
				}
				g.Expect(controlPlane).ToNot(BeNil())
				g.Expect(controlPlane.Status.Ready).To(BeTrue())
			}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "Couldn't get the control plane ready status for the cluster %s", klog.KObj(result.Cluster))
		}
		input.WaitForControlPlaneMachinesReady = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			// Not applicable
		}

		clusterctl.ApplyClusterTemplateAndWait(ctx, input, result)

		validateMachinePoolMachines(ctx, result.Cluster, bootstrapClusterProxy, result.MachinePools)

		By("Scaling the machine pool up")
		framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
			ClusterProxy:              bootstrapClusterProxy,
			Cluster:                   result.Cluster,
			Replicas:                  2,
			MachinePools:              result.MachinePools,
			WaitForMachinePoolToScale: e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
		})
		validateMachinePoolMachines(ctx, result.Cluster, bootstrapClusterProxy, result.MachinePools)

		By("Scaling the machine pool down")
		framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
			ClusterProxy:              bootstrapClusterProxy,
			Cluster:                   result.Cluster,
			Replicas:                  1,
			MachinePools:              result.MachinePools,
			WaitForMachinePoolToScale: e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
		})
		validateMachinePoolMachines(ctx, result.Cluster, bootstrapClusterProxy, result.MachinePools)
		upgradeControlPlaneVersionSpec(ctx, bootstrapClusterProxy.GetClient(), clusterName, namespace.Name,
			e2eConfig.GetIntervals(specName, "wait-control-plane"))
	})

	It("Managed Cluster - Cluster Identity", func() {
		clusterName = getClusterName(clusterNamePrefix, "cls-iden")
		input := clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "managed-cluster-identity",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}
		input.WaitForControlPlaneInitialized = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoveryAndWaitForControlPlaneInitialized")
			lister := input.ClusterProxy.GetClient()
			Expect(lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoveryAndWaitForControlPlaneInitialized")
			var controlPlane *infrastructurev1beta2.OCIManagedControlPlane
			Eventually(func(g Gomega) {
				controlPlane = GetOCIManagedControlPlaneByCluster(ctx, lister, result.Cluster.Name, result.Cluster.Namespace)
				if controlPlane != nil {
					Log(fmt.Sprintf("Control plane is not nil, status is %t", controlPlane.Status.Ready))
				}
				g.Expect(controlPlane).ToNot(BeNil())
				g.Expect(controlPlane.Status.Ready).To(BeTrue())
			}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "Couldn't get the control plane ready status for the cluster %s", klog.KObj(result.Cluster))
		}
		input.WaitForControlPlaneMachinesReady = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			// Not applicable
		}

		clusterctl.ApplyClusterTemplateAndWait(ctx, input, result)
	})

	It("Managed Cluster - Node Recycling", func() {
		clusterName = getClusterName(clusterNamePrefix, "cls-iden")
		input := clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "managed-node-recycling",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}
		input.WaitForControlPlaneInitialized = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoveryAndWaitForControlPlaneInitialized")
			lister := input.ClusterProxy.GetClient()
			Expect(lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoveryAndWaitForControlPlaneInitialized")
			var controlPlane *infrastructurev1beta2.OCIManagedControlPlane
			Eventually(func(g Gomega) {
				controlPlane = GetOCIManagedControlPlaneByCluster(ctx, lister, result.Cluster.Name, result.Cluster.Namespace)
				if controlPlane != nil {
					Log(fmt.Sprintf("Control plane is not nil, status is %t", controlPlane.Status.Ready))
				}
				g.Expect(controlPlane).ToNot(BeNil())
				g.Expect(controlPlane.Status.Ready).To(BeTrue())
			}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "Couldn't get the control plane ready status for the cluster %s", klog.KObj(result.Cluster))
		}
		input.WaitForControlPlaneMachinesReady = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			// Not applicable
		}

		clusterctl.ApplyClusterTemplateAndWait(ctx, input, result)

		updateMachinePoolVersion(ctx, result.Cluster, bootstrapClusterProxy, result.MachinePools,
			e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"))
	})

	It("Managed Cluster - Virtual Node Pool [PRBlocking]", func() {
		clusterName = getClusterName(clusterNamePrefix, "virtual")
		input := clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "managed-virtual",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}
		input.WaitForControlPlaneInitialized = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoveryAndWaitForControlPlaneInitialized")
			lister := input.ClusterProxy.GetClient()
			Expect(lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoveryAndWaitForControlPlaneInitialized")
			var controlPlane *infrastructurev1beta2.OCIManagedControlPlane
			Eventually(func(g Gomega) {
				controlPlane = GetOCIManagedControlPlaneByCluster(ctx, lister, result.Cluster.Name, result.Cluster.Namespace)
				if controlPlane != nil {
					Log(fmt.Sprintf("Control plane is not nil, status is %t", controlPlane.Status.Ready))
				}
				g.Expect(controlPlane).ToNot(BeNil())
				g.Expect(controlPlane.Status.Ready).To(BeTrue())
			}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "Couldn't get the control plane ready status for the cluster %s", klog.KObj(result.Cluster))
		}
		input.WaitForControlPlaneMachinesReady = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			// Not applicable
		}

		clusterctl.ApplyClusterTemplateAndWait(ctx, input, result)

		validateMachinePoolMachines(ctx, result.Cluster, bootstrapClusterProxy, result.MachinePools)
		controlPlane := GetOCIManagedControlPlaneByCluster(ctx, bootstrapClusterProxy.GetClient(), clusterName, namespace.Name)
		Expect(controlPlane).To(Not(BeNil()))
		clusterOcid := controlPlane.Spec.ID
		Eventually(func() error {
			_, err := okeClient.GetAddon(ctx, oke.GetAddonRequest{
				ClusterId: clusterOcid,
				AddonName: common.String("KubernetesDashboard"),
			})
			return err
		}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to install Addon")
	})

	It("Managed Cluster - Self managed nodes", func() {
		clusterName = getClusterName(clusterNamePrefix, "self")
		input := clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "managed-self-managed-nodes",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
		}
		input.WaitForControlPlaneInitialized = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoveryAndWaitForControlPlaneInitialized")
			lister := input.ClusterProxy.GetClient()
			Expect(lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoveryAndWaitForControlPlaneInitialized")
			var controlPlane *infrastructurev1beta2.OCIManagedControlPlane
			Eventually(func(g Gomega) {
				controlPlane = GetOCIManagedControlPlaneByCluster(ctx, lister, result.Cluster.Name, result.Cluster.Namespace)
				if controlPlane != nil {
					Log(fmt.Sprintf("Control plane is not nil, status is %t", controlPlane.Status.Ready))
				}
				g.Expect(controlPlane).ToNot(BeNil())
				g.Expect(controlPlane.Status.Ready).To(BeTrue())
			}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "Couldn't get the control plane ready status for the cluster %s", klog.KObj(result.Cluster))
		}
		input.WaitForControlPlaneMachinesReady = func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			// Not applicable
		}

		clusterctl.ApplyClusterTemplateAndWait(ctx, input, result)
		validateMachinePoolMachines(ctx, result.Cluster, bootstrapClusterProxy, result.MachinePools)
	})
})

// GetKubeadmControlPlaneByCluster returns the KubeadmControlPlane objects for a cluster.
// Important! this method relies on labels that are created by the CAPI controllers during the first reconciliation, so
// it is necessary to ensure this is already happened before calling it.
func GetOCIManagedControlPlaneByCluster(ctx context.Context, lister client.Client, clusterName string, namespaceName string) *infrastructurev1beta2.OCIManagedControlPlane {
	controlPlaneList := &infrastructurev1beta2.OCIManagedControlPlaneList{}
	Eventually(func() error {
		return lister.List(ctx, controlPlaneList, byClusterOptions(clusterName, namespaceName)...)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to list OCIManagedControlPlane object for Cluster %s", klog.KRef(namespaceName, clusterName))
	Expect(len(controlPlaneList.Items)).ToNot(BeNumerically(">", 1), "Cluster %s should not have more than 1 OCIManagedControlPlane object", klog.KRef(namespaceName, clusterName))
	if len(controlPlaneList.Items) == 1 {
		return &controlPlaneList.Items[0]
	}
	return nil
}

// byClusterOptions returns a set of ListOptions that allows to identify all the objects belonging to a Cluster.
func byClusterOptions(name, namespace string) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel: name,
		},
	}
}

func upgradeControlPlaneVersionSpec(ctx context.Context, lister client.Client, clusterName string, namespaceName string, WaitForControlPlaneIntervals []interface{}) {
	controlPlane := GetOCIManagedControlPlaneByCluster(ctx, lister, clusterName, namespaceName)
	Expect(controlPlane).NotTo(BeNil())

	patchHelper, err := patch.NewHelper(controlPlane, lister)
	Expect(err).ToNot(HaveOccurred())
	Expect(e2eConfig.Variables).To(HaveKey(ManagedKubernetesUpgradeVersion), "Missing %s variable in the config", ManagedKubernetesUpgradeVersion)
	managedKubernetesUpgradeVersion := e2eConfig.MustGetVariable(ManagedKubernetesUpgradeVersion)
	Log(fmt.Sprintf("Upgrade test is starting, upgrade version is %s", managedKubernetesUpgradeVersion))
	controlPlane.Spec.Version = &managedKubernetesUpgradeVersion
	Expect(patchHelper.Patch(ctx, controlPlane)).To(Succeed())
	Log("Upgrade test is starting")

	Eventually(func() (bool, error) {
		controlPlane := GetOCIManagedControlPlaneByCluster(ctx, lister, clusterName, namespaceName)
		Expect(controlPlane).NotTo(BeNil())
		if reflect.DeepEqual(controlPlane.Status.Version, &managedKubernetesUpgradeVersion) {
			return true, nil
		}
		return false, nil
	}, WaitForControlPlaneIntervals...).Should(BeTrue())
	Log("Upgrade test has completed")
}

func updateMachinePoolVersion(ctx context.Context, cluster *clusterv1.Cluster, clusterProxy framework.ClusterProxy, machinePools []*expv1.MachinePool, waitInterval []interface{}) {
	var machinePool *expv1.MachinePool
	for _, pool := range machinePools {
		if strings.HasSuffix(pool.Name, "-1") {
			machinePool = pool
			break
		}
	}
	lister := clusterProxy.GetClient()
	Expect(machinePool).NotTo(BeNil())
	managedKubernetesUpgradeVersion := e2eConfig.MustGetVariable(ManagedKubernetesUpgradeVersion)

	patchHelper, err := patch.NewHelper(machinePool, lister)
	Expect(err).ToNot(HaveOccurred())
	Expect(e2eConfig.Variables).To(HaveKey(ManagedKubernetesUpgradeVersion), "Missing %s variable in the config", ManagedKubernetesUpgradeVersion)
	Log(fmt.Sprintf("Upgrade test is starting, upgrade version is %s", managedKubernetesUpgradeVersion))
	machinePool.Spec.Template.Spec.Version = &managedKubernetesUpgradeVersion
	Expect(patchHelper.Patch(ctx, machinePool)).To(Succeed())

	ociMachinePool := &infrav2exp.OCIManagedMachinePool{}
	err = lister.Get(ctx, client.ObjectKey{Name: machinePool.Name, Namespace: cluster.Namespace}, ociMachinePool)
	Expect(err).To(BeNil())
	patchHelper, err = patch.NewHelper(ociMachinePool, lister)
	// to update a node pool, set the version and set the current image to nil so that CAPOCI will
	// automatically lookup a new version
	ociMachinePool.Spec.Version = &managedKubernetesUpgradeVersion
	ociMachinePool.Spec.NodeSourceViaImage.ImageId = nil
	Expect(err).ToNot(HaveOccurred())
	Expect(patchHelper.Patch(ctx, ociMachinePool)).To(Succeed())

	Log("Upgrade test is starting")

	Eventually(func() (int, error) {
		mpKey := client.ObjectKey{
			Namespace: machinePool.Namespace,
			Name:      machinePool.Name,
		}
		if err := lister.Get(ctx, mpKey, machinePool); err != nil {
			return 0, err
		}
		versions := getMachinePoolInstanceVersions(ctx, clusterProxy, cluster, machinePool)
		matches := 0
		for _, version := range versions {
			if version == managedKubernetesUpgradeVersion {
				matches++
			}
		}

		if matches != len(versions) {
			return 0, errors.Errorf("old version instances remain. Expected %d instances at version %v. Got version list: %v", len(versions), managedKubernetesUpgradeVersion, versions)
		}

		return matches, nil
	}, waitInterval...).Should(Equal(1), "Timed out waiting for all MachinePool %s instances to be upgraded to Kubernetes version %s", klog.KObj(machinePool), managedKubernetesUpgradeVersion)
}

func validateMachinePoolMachines(ctx context.Context, cluster *clusterv1.Cluster, clusterProxy framework.ClusterProxy, machinePools []*expv1.MachinePool) {
	Eventually(func() error {
		lister := clusterProxy.GetClient()
		for _, pool := range machinePools {
			machineList := &infrav2exp.OCIMachinePoolMachineList{}
			labels := map[string]string{
				clusterv1.ClusterNameLabel:     cluster.Name,
				clusterv1.MachinePoolNameLabel: pool.Name,
			}
			if err := lister.List(ctx, machineList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
				return err
			}

			if len(machineList.Items) != int(*pool.Spec.Replicas) {
				return errors.New(fmt.Sprintf("Infra machines does not equal machine pool replicas for machinepool %s", pool.Name))
			}
			for _, managedMachine := range machineList.Items {
				_, err := util.GetOwnerMachine(ctx, lister, managedMachine.ObjectMeta)
				if err != nil {
					return err
				}
			}
			Logf("Machinepool machines created successfully for machinepool %s", pool.Name)
		}
		return nil
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Machinepool machines were not created properly")
}

// getMachinePoolInstanceVersions returns the Kubernetes versions of the machine pool instances.
// This method was forked because we need to lookup the kubeconfig with each call
// as the tokens are refreshed in case of OKE
func getMachinePoolInstanceVersions(ctx context.Context, clusterProxy framework.ClusterProxy, cluster *clusterv1.Cluster, machinePool *expv1.MachinePool) []string {
	Expect(ctx).NotTo(BeNil(), "ctx is required for getMachinePoolInstanceVersions")

	instances := machinePool.Status.NodeRefs
	versions := make([]string, len(instances))
	for i, instance := range instances {
		node := &corev1.Node{}
		var nodeGetError error
		err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
			nodeGetError = clusterProxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name).
				GetClient().Get(ctx, client.ObjectKey{Name: instance.Name}, node)
			if nodeGetError != nil {
				return false, nil //nolint:nilerr
			}
			return true, nil
		})
		if err != nil {
			versions[i] = "unknown"
			if nodeGetError != nil {
				// Dump the instance name and error here so that we can log it as part of the version array later on.
				versions[i] = fmt.Sprintf("%s error: %s", instance.Name, errors.Wrap(err, nodeGetError.Error()))
			}
		} else {
			versions[i] = node.Status.NodeInfo.KubeletVersion
		}
	}

	return versions
}
