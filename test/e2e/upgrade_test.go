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
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/oci-go-sdk/v65/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
)

var _ = Describe("Cluster Upgrade Tests", func() {
	var (
		ctx               = context.TODO()
		specName          = "upgrade-workload-cluster"
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
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersionUpgradeFrom))
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersionUpgradeTo))

		// CLUSTER_NAME and CLUSTER_NAMESPACE allows for testing existing clusters
		// if CLUSTER_NAMESPACE is set don't generate a new prefix otherwise
		// the correct namespace won't be found and a new cluster will be created
		clusterNameSpace := os.Getenv("CLUSTER_NAMESPACE")
		if clusterNameSpace == "" {
			clusterNamePrefix = fmt.Sprintf("capoci-e2e-%s", util.RandomString(6))
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
			CancelWatches:     cancelWatches,
			IntervalsGetter:   e2eConfig.GetIntervals,
			SkipCleanup:       skipCleanup,
			AdditionalCleanup: additionalCleanup,
			ArtifactFolder:    artifactFolder,
		}
		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})

	It("Upgrade of control-plane nodes and worker nodes", func() {
		clusterName = getClusterName(clusterNamePrefix, "upgrade")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   clusterctl.DefaultFlavor,
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersionUpgradeFrom),
				ControlPlaneMachineCount: pointer.Int64Ptr(1),
				WorkerMachineCount:       pointer.Int64Ptr(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)

		newCPMachineTemplateName := fmt.Sprintf("%s-control-plane-upgraded", clusterName)
		updatedControlPlaneTemplate := makeOCIMachineTemplate(namespace.Name, newCPMachineTemplateName)
		err := bootstrapClusterProxy.GetClient().Create(ctx, updatedControlPlaneTemplate)
		Expect(err).NotTo(HaveOccurred())

		controlPlane := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
			Lister:      bootstrapClusterProxy.GetClient(),
			Namespace:   namespace.Name,
			ClusterName: clusterName,
		})
		patchHelper, err := patch.NewHelper(controlPlane, bootstrapClusterProxy.GetClient())

		upgradeVersion := e2eConfig.GetVariable(capi_e2e.KubernetesVersionUpgradeTo)
		controlPlane.Spec.MachineTemplate.InfrastructureRef.Name = newCPMachineTemplateName
		controlPlane.Spec.Version = upgradeVersion

		Expect(err).ToNot(HaveOccurred())
		Expect(patchHelper.Patch(ctx, controlPlane)).To(Succeed())

		Log("Waiting for control-plane machines to have the upgraded kubernetes version")
		framework.WaitForControlPlaneMachinesToBeUpgraded(ctx, framework.WaitForControlPlaneMachinesToBeUpgradedInput{
			Lister:                   bootstrapClusterProxy.GetClient(),
			Cluster:                  result.Cluster,
			MachineCount:             1,
			KubernetesUpgradeVersion: upgradeVersion,
		}, e2eConfig.GetIntervals(specName, "wait-machine-upgrade")...)

		Expect(err).NotTo(HaveOccurred())
		for _, deployment := range result.MachineDeployments {
			Log(fmt.Sprintf("Patching the new kubernetes version to Machine Deployment %s/%s", deployment.Namespace, deployment.Name))
			patchHelper, err := patch.NewHelper(deployment, bootstrapClusterProxy.GetClient())
			Expect(err).ToNot(HaveOccurred())

			newWorkerMachineTemplateName := fmt.Sprintf("%s-upgraded", deployment.Name)
			updatedWorkerTemplate := makeOCIMachineTemplate(namespace.Name, newWorkerMachineTemplateName)
			err = bootstrapClusterProxy.GetClient().Create(ctx, updatedWorkerTemplate)
			Expect(err).NotTo(HaveOccurred())

			oldVersion := deployment.Spec.Template.Spec.Version
			deployment.Spec.Template.Spec.Version = common.String(upgradeVersion)
			deployment.Spec.Template.Spec.InfrastructureRef.Name = newWorkerMachineTemplateName

			Expect(patchHelper.Patch(ctx, deployment)).To(Succeed())

			Log(fmt.Sprintf("Waiting for Kubernetes versions of machines in MachineDeployment %s/%s to be upgraded from %s to %s",
				deployment.Namespace, deployment.Name, *oldVersion, upgradeVersion))
			framework.WaitForMachineDeploymentMachinesToBeUpgraded(ctx, framework.WaitForMachineDeploymentMachinesToBeUpgradedInput{
				Lister:                   bootstrapClusterProxy.GetClient(),
				Cluster:                  result.Cluster,
				MachineCount:             1,
				KubernetesUpgradeVersion: upgradeVersion,
				MachineDeployment:        *deployment,
			}, e2eConfig.GetIntervals(specName, "wait-machine-upgrade")...)
		}
	})
})

func makeOCIMachineTemplate(namespace, name string) *infrastructurev1beta1.OCIMachineTemplate {
	ociMachine := &infrastructurev1beta1.OCIMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrastructurev1beta1.OCIMachineTemplateSpec{
			Template: infrastructurev1beta1.OCIMachineTemplateResource{
				Spec: infrastructurev1beta1.OCIMachineSpec{
					ImageId: os.Getenv("OCI_UPGRADE_IMAGE_ID"),
					Shape:   os.Getenv("OCI_NODE_MACHINE_TYPE"),
					ShapeConfig: infrastructurev1beta1.ShapeConfig{
						Ocpus: "1",
					},
					Metadata: map[string]string{
						"ssh_authorized_keys": os.Getenv("OCI_SSH_KEY"),
					},
				},
			},
		},
	}
	return ociMachine
}
