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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Workload cluster creation", func() {
	var (
		ctx               = context.TODO()
		specName          = "create-workload-cluster"
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
			ClusterctlConfigPath: clusterctlConfigPath,
			CancelWatches:     cancelWatches,
			IntervalsGetter:   e2eConfig.GetIntervals,
			SkipCleanup:       skipCleanup,
			AdditionalCleanup: additionalCleanup,
			ArtifactFolder:    artifactFolder,
		}
		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})

	It("Default CNI - with 1 control-plane nodes and 1 worker nodes [PRBlocking]", func() {
		clusterName = getClusterName(clusterNamePrefix, "simple")
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
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			CNIManifestPath:              e2eConfig.MustGetVariable(capi_e2e.CNIPath),
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	})

	It("Default CNI - With 3 control plane nodes spread across failure domains [PRBlocking]", func() {
		clusterName = getClusterName(clusterNamePrefix, "3nodecontrolplane")
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
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(3),
				WorkerMachineCount:       pointer.Int64(0),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
		validateFailureDomainSpread(namespace.Name, clusterName)
	})

	It("Antrea as CNI - With 1 control-plane nodes and 1 worker nodes", func() {
		clusterName = getClusterName(clusterNamePrefix, "antrea")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "antrea",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	})

	It("Oracle Linux - With 1 control-plane nodes and 1 worker nodes", func() {
		clusterName = getClusterName(clusterNamePrefix, "oracle-linux")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "oracle-linux",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
		validateOLImage(namespace.Name, clusterName)
	})

	It("Machine with IPv6 - With 1 control-plane nodes and 1 worker nodes [PRBlocking]", func() {
		clusterName = getClusterName(clusterNamePrefix, "machine-with-ipv6")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "machine-with-ipv6",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64Ptr(1),
				WorkerMachineCount:       pointer.Int64Ptr(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	})

	It("Machine with pravirtualized - With 1 control-plane nodes and 1 worker nodes [PRBlocking]", func() {
		clusterName = getClusterName(clusterNamePrefix, "with-paravirt-bv")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "with-paravirt-bv",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64Ptr(1),
				WorkerMachineCount:       pointer.Int64Ptr(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	})

	It("Windows - With 1 Linux control-plane nodes and with 1 Windows worker nodes using Calico CNI", func() {
		clusterName = getClusterName(clusterNamePrefix, "windows-calico")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "windows-calico",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64Ptr(1),
				WorkerMachineCount:       pointer.Int64Ptr(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-windows-worker-nodes"),
		}, result)
		validateWindowsImage(namespace.Name, clusterName)
	})

	It("Cloud Provider OCI testing [PRBlocking]", func() {
		clusterName = getClusterName(clusterNamePrefix, "ccm-testing")
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
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
		k8sClient := bootstrapClusterProxy.GetClient()
		ociCluster := &infrastructurev1beta1.OCICluster{}
		ociClusterName := client.ObjectKey{
			Namespace: namespace.Name,
			Name:      clusterName,
		}
		err := k8sClient.Get(ctx, ociClusterName, ociCluster)
		Expect(err).NotTo(HaveOccurred())

		vcn := ociCluster.Spec.NetworkSpec.Vcn.ID
		compartment := ociCluster.Spec.CompartmentId
		subnetId := ""
		for _, subnet := range ociCluster.Spec.NetworkSpec.Vcn.Subnets {
			if subnet.Role == infrastructurev1beta1.ControlPlaneEndpointRole {
				subnetId = *subnet.ID
			}
		}
		Expect(subnetId).To(Not(Equal("")))
		ccmPath := e2eConfig.MustGetVariable("CCM_PATH")
		b, err := os.ReadFile(ccmPath)
		Expect(err).NotTo(HaveOccurred())
		ccmCrs := string(b)
		ccmCrs = strings.ReplaceAll(ccmCrs, "OCI_COMPARTMENT_ID", compartment)
		ccmCrs = strings.ReplaceAll(ccmCrs, "OCI_COMPARTMENT_ID", compartment)
		ccmCrs = strings.ReplaceAll(ccmCrs, "VCN_ID", *vcn)
		ccmCrs = strings.ReplaceAll(ccmCrs, "SUBNET_ID", subnetId)

		workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)

		err = workloadClusterProxy.CreateOrUpdate(ctx, []byte(ccmCrs))
		Expect(err).NotTo(HaveOccurred())

		Log("Creating the LB service")
		clusterClient := workloadClusterProxy.GetClient()
		lbServiceName := "test-svc-" + util.RandomString(6)
		createLBService(metav1.NamespaceDefault, lbServiceName, clusterClient)

		nginxStatefulsetInfo := statefulSetInfo{
			name:                      "nginx-statefulset",
			namespace:                 metav1.NamespaceDefault,
			replicas:                  int32(1),
			selector:                  map[string]string{"app": "nginx"},
			storageClassName:          "oci-bv-encrypted",
			volumeName:                "nginx-volumes",
			svcName:                   "nginx-svc",
			svcPort:                   int32(80),
			svcPortName:               "nginx-web",
			containerName:             "nginx",
			containerImage:            "k8s.gcr.io/nginx-slim:0.8",
			containerPort:             int32(80),
			podTerminationGracePeriod: int64(30),
			volMountPath:              "/usr/share/nginx/html",
		}

		By("Deploying StatefulSet on infra")

		createStatefulSet(nginxStatefulsetInfo, clusterClient)

		By("Deleting LB service")
		deleteLBService(metav1.NamespaceDefault, lbServiceName, clusterClient)
		By("Deleting retained dynamically provisioned volumes")
		deleteStatefulSet(nginxStatefulsetInfo, clusterClient)
		deletePVC(nginxStatefulsetInfo, clusterClient)
	})

	It("Custom networking NSG [PRBlocking]", func() {
		clusterName = getClusterName(clusterNamePrefix, "custom-nsg")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "custom-networking-nsg",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
		k8sClient := bootstrapClusterProxy.GetClient()
		ociCluster := &infrastructurev1beta1.OCICluster{}
		ociClusterName := client.ObjectKey{
			Namespace: namespace.Name,
			Name:      clusterName,
		}
		err := k8sClient.Get(ctx, ociClusterName, ociCluster)
		Expect(err).NotTo(HaveOccurred())
		err = validateCustonNSGNetworking(ctx, *ociCluster, clusterName, namespace.Name, result.MachineDeployments[0].Name)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Custom networking Seclist", func() {
		clusterName = getClusterName(clusterNamePrefix, "custom-seclist")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "custom-networking-seclist",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
		k8sClient := bootstrapClusterProxy.GetClient()
		ociCluster := &infrastructurev1beta1.OCICluster{}
		ociClusterName := client.ObjectKey{
			Namespace: namespace.Name,
			Name:      clusterName,
		}
		err := k8sClient.Get(ctx, ociClusterName, ociCluster)
		Expect(err).NotTo(HaveOccurred())
		err = validateCustomNetworkingSeclist(ctx, *ociCluster, clusterName)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Multiple nsg and subnet", func() {
		clusterName = getClusterName(clusterNamePrefix, "multi-subnet-nsg")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "multiple-node-nsg",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)

		verifyMultipleNsgSubnet(ctx, namespace.Name, clusterName, result.MachineDeployments)
	})

	When("Bare Metal workload cluster creation", func() {

		It("Bare Metal - With 1 control-plane nodes and 1 worker nodes", func() {
			clusterName = getClusterName(clusterNamePrefix, "bare-metal")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "bare-metal",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64(1),
					WorkerMachineCount:       pointer.Int64(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster-bare-metal"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane-bare-metal"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes-bare-metal"),
			}, result)
		})
	})

	When("Multi-RegionIdentifier workload cluster creation", func() {

		It("Alternative region - With 1 control-plane nodes and 1 worker nodes", func() {
			clusterName = getClusterName(clusterNamePrefix, "alternative-region")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "alternative-region",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64(1),
					WorkerMachineCount:       pointer.Int64(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
		})
	})

	It("With 1 control-plane nodes and 1 worker nodes - LocalVCNPeering", func() {
		clusterName = getClusterName(clusterNamePrefix, "local-vcn-peering")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "local-vcn-peering",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			CNIManifestPath:              e2eConfig.MustGetVariable(capi_e2e.CNIPath),
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	})

	It("With 1 control-plane nodes and 1 worker nodes - RemoteVCNPeering", func() {
		clusterName = getClusterName(clusterNamePrefix, "remote-vcn-peering")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "remote-vcn-peering",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			CNIManifestPath:              e2eConfig.MustGetVariable(capi_e2e.CNIPath),
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	})

	It("ClusterClass - with 1 control-plane nodes and 1 worker nodes [PRBlocking]", func() {
		clusterName = getClusterName(clusterNamePrefix, "clusterclass")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "cluster-class",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			CNIManifestPath:              e2eConfig.MustGetVariable(capi_e2e.CNIPath),
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	})

	It("Externally managed VCN", func() {
		clusterName = getClusterName(clusterNamePrefix, "externally-managed-vcn")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "externally-managed-vcn",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			CNIManifestPath:              e2eConfig.MustGetVariable(capi_e2e.CNIPath),
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	})

	It("Self manage NSG", func() {
		clusterName = getClusterName(clusterNamePrefix, "self-manage-nsg")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "self-manage-nsg",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(0),
			},
			CNIManifestPath:              e2eConfig.MustGetVariable(capi_e2e.CNIPath),
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane-nsg"),
		}, result)
	})

	It("Machine Pool - Simple", func() {
		clusterName = getClusterName(clusterNamePrefix, "machine-pool")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "machine-pool",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			CNIManifestPath:              e2eConfig.MustGetVariable(capi_e2e.CNIPath),
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)

		By("Scaling the machine pool up")
		framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
			ClusterProxy:              bootstrapClusterProxy,
			Cluster:                   result.Cluster,
			Replicas:                  2,
			MachinePools:              result.MachinePools,
			WaitForMachinePoolToScale: e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
		})

		By("Scaling the machine pool down")
		framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
			ClusterProxy:              bootstrapClusterProxy,
			Cluster:                   result.Cluster,
			Replicas:                  1,
			MachinePools:              result.MachinePools,
			WaitForMachinePoolToScale: e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
		})
	})

	It("Cluster Identity - with 1 control-plane nodes and 1 worker nodes", func() {
		clusterName = getClusterName(clusterNamePrefix, "cluster-identity")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   "cluster-identity",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64(1),
				WorkerMachineCount:       pointer.Int64(1),
			},
			CNIManifestPath:              e2eConfig.MustGetVariable(capi_e2e.CNIPath),
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)
	})
})

func verifyMultipleNsgSubnet(ctx context.Context, namespace string, clusterName string, mcDeployments []*clusterv1.MachineDeployment) {
	ociCluster := &infrastructurev1beta1.OCICluster{}
	ociClusterName := client.ObjectKey{
		Namespace: namespace,
		Name:      clusterName,
	}
	err := bootstrapClusterProxy.GetClient().Get(ctx, ociClusterName, ociCluster)
	Expect(err).NotTo(HaveOccurred())
	arrSubnets := [2]string{}
	i := 0
	for _, subnet := range ociCluster.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == infrastructurev1beta1.WorkerRole {
			arrSubnets[i] = *subnet.ID
			i++
		}
	}
	i = 0
	arrNsgs := [2]string{}
	for _, nsg := range ociCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
		if nsg.Role == infrastructurev1beta1.WorkerRole {
			arrNsgs[i] = *nsg.ID
			i++
		}
	}

	for _, mcDeployment := range mcDeployments {
		lister := bootstrapClusterProxy.GetClient()
		inClustersNamespaceListOption := client.InNamespace(namespace)
		matchClusterListOption := client.MatchingLabels{
			clusterv1.ClusterNameLabel: clusterName,
		}
		matchClusterListOption[clusterv1.MachineDeploymentNameLabel] = mcDeployment.Name
		machineList := &clusterv1.MachineList{}
		Expect(lister.List(context.Background(), machineList, inClustersNamespaceListOption, matchClusterListOption)).
			To(Succeed(), "Couldn't list machines for the cluster %q", clusterName)

		requiredIndex := 0
		if mcDeployment.Name == fmt.Sprintf("%s-md-1", clusterName) {
			requiredIndex = 1
		}
		for _, machine := range machineList.Items {
			instanceOcid := strings.Split(*machine.Spec.ProviderID, "//")[1]
			Log(fmt.Sprintf("Instance OCID is %s", instanceOcid))

			exists := false
			resp, err := computeClient.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
				InstanceId:    common.String(instanceOcid),
				CompartmentId: common.String(os.Getenv("OCI_COMPARTMENT_ID")),
			})
			Expect(err).NotTo(HaveOccurred())

			for _, attachment := range resp.Items {
				if attachment.LifecycleState != core.VnicAttachmentLifecycleStateAttached {
					continue
				}

				if attachment.VnicId == nil {
					continue
				}
				vnic, err := vcnClient.GetVnic(ctx, core.GetVnicRequest{
					VnicId: attachment.VnicId,
				})

				Expect(err).NotTo(HaveOccurred())

				if vnic.IsPrimary != nil && *vnic.IsPrimary {
					exists = true
					Expect(vnic.NsgIds[0]).To(Equal(arrNsgs[requiredIndex]))
					Expect(*(vnic.SubnetId)).To(Equal(arrSubnets[requiredIndex]))
				}
			}
			Expect(exists).To(Equal(true))
		}
	}
}

func validateCustonNSGNetworking(ctx context.Context, ociCluster infrastructurev1beta1.OCICluster, clusterName string, nameSpace string, machineDeployment string) error {
	vcnId := ociCluster.Spec.NetworkSpec.Vcn.ID
	resp, err := vcnClient.GetVcn(ctx, core.GetVcnRequest{
		VcnId: vcnId,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.CidrBlocks[0]).To(Equal("15.0.0.0/16"))
	Expect(*resp.DisplayName).To(Equal(fmt.Sprintf("%s-test", clusterName)))

	listResponse, err := vcnClient.ListSubnets(ctx, core.ListSubnetsRequest{
		VcnId:         vcnId,
		CompartmentId: common.String(os.Getenv("OCI_COMPARTMENT_ID")),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(listResponse.Items)).To(Equal(4))

	for _, subnet := range ociCluster.Spec.NetworkSpec.Vcn.Subnets {
		subnetId := subnet.ID
		resp, err := vcnClient.GetSubnet(ctx, core.GetSubnetRequest{
			SubnetId: subnetId,
		})
		Expect(err).NotTo(HaveOccurred())
		switch subnet.Role {
		case infrastructurev1beta1.ControlPlaneEndpointRole:
			Expect(*resp.CidrBlock).To(Equal("15.0.0.0/28"))
			Expect(*resp.DisplayName).To(Equal("ep-subnet"))
		case infrastructurev1beta1.ControlPlaneRole:
			Expect(*resp.CidrBlock).To(Equal("15.0.5.0/28"))
			Expect(*resp.DisplayName).To(Equal("cp-mc-subnet"))
		case infrastructurev1beta1.WorkerRole:
			Expect(*resp.CidrBlock).To(Equal("15.0.10.0/24"))
			Expect(*resp.DisplayName).To(Equal("worker-subnet"))
		case infrastructurev1beta1.ServiceLoadBalancerRole:
			Expect(*resp.CidrBlock).To(Equal("15.0.20.0/24"))
			Expect(*resp.DisplayName).To(Equal("svc-lb-subnet"))
		default:
			return errors.New("invalid subnet role")
		}
	}

	// to make sure that the original spec was not changed, we should compare with the
	// original unchanged spec present in the file system
	reader, _ := os.Open(e2eConfig.MustGetVariable("NSG_CLUSTER_PATH"))
	ociClusterOriginal := &infrastructurev1beta1.OCICluster{}
	err = yaml.NewYAMLOrJSONDecoder(reader, 4096).Decode(ociClusterOriginal)
	Expect(err).NotTo(HaveOccurred())

	for _, nsg := range ociCluster.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
		nsgId := nsg.ID
		resp, err := vcnClient.GetNetworkSecurityGroup(ctx, core.GetNetworkSecurityGroupRequest{
			NetworkSecurityGroupId: nsgId,
		})
		Expect(err).NotTo(HaveOccurred())
		switch nsg.Role {
		case infrastructurev1beta1.ControlPlaneEndpointRole:
			verifyNsg(ctx, resp, ociClusterOriginal, &ociCluster, infrastructurev1beta1.ControlPlaneEndpointRole)
			lbId := ociCluster.Spec.NetworkSpec.APIServerLB.LoadBalancerId
			lb, err := lbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
				NetworkLoadBalancerId: lbId,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(lb.NetworkSecurityGroupIds[0]).To(Equal(*nsgId))
		case infrastructurev1beta1.ControlPlaneRole:
			verifyNsg(ctx, resp, ociClusterOriginal, &ociCluster, infrastructurev1beta1.ControlPlaneRole)
			Log("Validating control plane machine vnic NSG")
			validateVnicNSG(ctx, clusterName, nameSpace, nsgId, "")
		case infrastructurev1beta1.WorkerRole:
			verifyNsg(ctx, resp, ociClusterOriginal, &ociCluster, infrastructurev1beta1.WorkerRole)
			Log("Validating node machine vnic NSG")
			validateVnicNSG(ctx, clusterName, nameSpace, nsgId, machineDeployment)
		case infrastructurev1beta1.ServiceLoadBalancerRole:
			verifyNsg(ctx, resp, ociClusterOriginal, &ociCluster, infrastructurev1beta1.ServiceLoadBalancerRole)
		default:
			return errors.New("invalid nsg role")
		}
	}

	return nil
}

func validateVnicNSG(ctx context.Context, clusterName string, nameSpace string, nsgId *string, machineDeployment string) {
	lister := bootstrapClusterProxy.GetClient()
	inClustersNamespaceListOption := client.InNamespace(nameSpace)
	matchClusterListOption := client.MatchingLabels{
		clusterv1.ClusterNameLabel: clusterName,
	}
	// its either a machine deployment or control plane
	if machineDeployment != "" {
		matchClusterListOption[clusterv1.MachineDeploymentNameLabel] = machineDeployment
	} else {
		matchClusterListOption[clusterv1.MachineControlPlaneLabel] = ""
	}

	machineList := &clusterv1.MachineList{}
	Expect(lister.List(context.Background(), machineList, inClustersNamespaceListOption, matchClusterListOption)).
		To(Succeed(), "Couldn't list machines for the cluster %q", clusterName)
	Log(fmt.Sprintf("NSG id is %s", *nsgId))
	exists := false
	for _, machine := range machineList.Items {
		instanceOcid := strings.Split(*machine.Spec.ProviderID, "//")[1]
		Log(fmt.Sprintf("Instance OCID is %s", instanceOcid))

		resp, err := computeClient.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
			InstanceId:    common.String(instanceOcid),
			CompartmentId: common.String(os.Getenv("OCI_COMPARTMENT_ID")),
		})
		Expect(err).NotTo(HaveOccurred())

		for _, attachment := range resp.Items {
			if attachment.LifecycleState != core.VnicAttachmentLifecycleStateAttached {
				continue
			}

			if attachment.VnicId == nil {
				continue
			}
			vnic, err := vcnClient.GetVnic(ctx, core.GetVnicRequest{
				VnicId: attachment.VnicId,
			})

			Expect(err).NotTo(HaveOccurred())

			if vnic.IsPrimary != nil && *vnic.IsPrimary {
				exists = true
				Expect(vnic.NsgIds[0]).To(Equal(*nsgId))
			}
		}
		Expect(exists).To(Equal(true))
	}
}

func validateCustomNetworkingSeclist(ctx context.Context, ociCluster infrastructurev1beta1.OCICluster, clusterName string) error {
	vcnId := ociCluster.Spec.NetworkSpec.Vcn.ID
	resp, err := vcnClient.GetVcn(ctx, core.GetVcnRequest{
		VcnId: vcnId,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.CidrBlocks[0]).To(Equal("10.0.0.0/16"))
	Expect(*resp.DisplayName).To(Equal(fmt.Sprintf("%s-test", clusterName)))

	listResponse, err := vcnClient.ListSubnets(ctx, core.ListSubnetsRequest{
		VcnId:         vcnId,
		CompartmentId: common.String(os.Getenv("OCI_COMPARTMENT_ID")),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(listResponse.Items)).To(Equal(4))

	// to make sure that the original spec was not changed, we should compare with the
	// original unchanged spec present in the file system
	reader, _ := os.Open(e2eConfig.MustGetVariable("SECLIST_CLUSTER_PATH"))
	ociClusterOriginal := &infrastructurev1beta1.OCICluster{}
	err = yaml.NewYAMLOrJSONDecoder(reader, 4096).Decode(ociClusterOriginal)
	Expect(err).NotTo(HaveOccurred())

	for _, subnet := range ociCluster.Spec.NetworkSpec.Vcn.Subnets {
		subnetId := subnet.ID
		resp, err := vcnClient.GetSubnet(ctx, core.GetSubnetRequest{
			SubnetId: subnetId,
		})
		Expect(err).NotTo(HaveOccurred())
		switch subnet.Role {
		case infrastructurev1beta1.ControlPlaneEndpointRole:
			verifySeclistSubnet(ctx, resp, ociClusterOriginal, subnet, "ep-subnet", scope.ControlPlaneEndpointSubnetDefaultCIDR)
		case infrastructurev1beta1.ControlPlaneRole:
			verifySeclistSubnet(ctx, resp, ociClusterOriginal, subnet, "cp-mc-subnet", scope.ControlPlaneMachineSubnetDefaultCIDR)
		case infrastructurev1beta1.WorkerRole:
			verifySeclistSubnet(ctx, resp, ociClusterOriginal, subnet, "worker-subnet", scope.WorkerSubnetDefaultCIDR)
		case infrastructurev1beta1.ServiceLoadBalancerRole:
			verifySeclistSubnet(ctx, resp, ociClusterOriginal, subnet, "svc-lb-subnet", scope.ServiceLoadBalancerDefaultCIDR)
		default:
			return errors.New("invalid subnet role")
		}
	}
	return nil
}

func verifySeclistSubnet(ctx context.Context, resp core.GetSubnetResponse, ociClusterOriginal *infrastructurev1beta1.OCICluster, subnet *infrastructurev1beta1.Subnet, subnetName string, cidrBlock string) {
	Expect(*resp.CidrBlock).To(Equal(cidrBlock))
	Expect(*resp.DisplayName).To(Equal(subnetName))
	secList := resp.SecurityListIds[0]
	r, err := vcnClient.GetSecurityList(ctx, core.GetSecurityListRequest{
		SecurityListId: common.String(secList),
	})
	Expect(err).NotTo(HaveOccurred())
	matches := 0
	for _, n := range ociClusterOriginal.Spec.NetworkSpec.Vcn.Subnets {
		if n.Role == subnet.Role {
			matches++
			var ingressRules = make([]core.IngressSecurityRule, 0)
			for _, irule := range n.SecurityList.IngressRules {
				ingressRules = append(ingressRules, convertSecurityListIngressRule(irule))
			}
			var egressRules = make([]core.EgressSecurityRule, 0)
			for _, irule := range n.SecurityList.EgressRules {
				egressRules = append(egressRules, convertSecurityListEgressRule(irule))
			}

			Expect(r.EgressSecurityRules).To(Equal(egressRules))
			Expect(r.IngressSecurityRules).To(Equal(ingressRules))
		}
	}
	Expect(matches).To(Equal(1))
}

func verifyNsg(ctx context.Context, resp core.GetNetworkSecurityGroupResponse, ociClusterOriginal *infrastructurev1beta1.OCICluster, ociClusterActual *infrastructurev1beta1.OCICluster, role infrastructurev1beta1.Role) {
	listResponse, err := vcnClient.ListNetworkSecurityGroupSecurityRules(ctx, core.ListNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: resp.Id,
	})
	Expect(err).NotTo(HaveOccurred())
	ingressRules, egressRules := generateSpecFromSecurityRules(ociClusterActual, listResponse.Items)
	matches := 0
	for _, n := range ociClusterOriginal.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
		if n.Role == role {
			matches++
			Expect(ingressRules).To(Equal(n.IngressRules))
			Expect(egressRules).To(Equal(n.EgressRules))
		}
	}
	Expect(matches).To(Equal(1))
}

func generateSpecFromSecurityRules(ociClusterOriginal *infrastructurev1beta1.OCICluster, rules []core.SecurityRule) ([]infrastructurev1beta1.IngressSecurityRuleForNSG, []infrastructurev1beta1.EgressSecurityRuleForNSG) {
	var ingressRules []infrastructurev1beta1.IngressSecurityRuleForNSG
	var egressRules []infrastructurev1beta1.EgressSecurityRuleForNSG
	for _, rule := range rules {
		// while comparing values, the boolean value has to be always set
		stateless := rule.IsStateless
		if stateless == nil {
			stateless = common.Bool(false)
		}
		icmpOptions, tcpOptions, udpOptions := getProtocolOptionsForSpec(rule.IcmpOptions, rule.TcpOptions, rule.UdpOptions)
		switch rule.Direction {
		case core.SecurityRuleDirectionIngress:
			ingressRule := infrastructurev1beta1.IngressSecurityRuleForNSG{
				IngressSecurityRule: infrastructurev1beta1.IngressSecurityRule{
					Protocol:    rule.Protocol,
					Source:      rule.Source,
					IcmpOptions: icmpOptions,
					IsStateless: stateless,
					SourceType:  infrastructurev1beta1.IngressSecurityRuleSourceTypeEnum(rule.SourceType),
					TcpOptions:  tcpOptions,
					UdpOptions:  udpOptions,
					Description: rule.Description,
				},
			}
			ingressRules = append(ingressRules, ingressRule)
		case core.SecurityRuleDirectionEgress:
			egressRule := infrastructurev1beta1.EgressSecurityRuleForNSG{
				EgressSecurityRule: infrastructurev1beta1.EgressSecurityRule{
					Destination:     rule.Destination,
					Protocol:        rule.Protocol,
					DestinationType: infrastructurev1beta1.EgressSecurityRuleDestinationTypeEnum(rule.DestinationType),
					IcmpOptions:     icmpOptions,
					IsStateless:     stateless,
					TcpOptions:      tcpOptions,
					UdpOptions:      udpOptions,
					Description:     rule.Description,
				},
			}
			if rule.DestinationType == core.SecurityRuleDestinationTypeNetworkSecurityGroup {
				for _, nsg := range ociClusterOriginal.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
					if reflect.DeepEqual(nsg.ID, rule.Destination) {
						egressRule.Destination = common.String(nsg.Name)
					}
				}
			}
			egressRules = append(egressRules, egressRule)
		}
	}
	return ingressRules, egressRules
}

func validateFailureDomainSpread(nameSpace string, clusterName string) {
	lister := bootstrapClusterProxy.GetClient()
	inClustersNamespaceListOption := client.InNamespace(nameSpace)
	matchClusterListOption := client.MatchingLabels{
		clusterv1.ClusterNameLabel:         clusterName,
		clusterv1.MachineControlPlaneLabel: "",
	}

	machineList := &clusterv1.MachineList{}
	Expect(lister.List(context.Background(), machineList, inClustersNamespaceListOption, matchClusterListOption)).
		To(Succeed(), "Couldn't list machines for the cluster %q", clusterName)

	failureDomainCounts := map[string]int{}
	ociFailureDomain := map[string]int{}
	// Count all control plane machine failure domains.
	for _, machine := range machineList.Items {
		if machine.Spec.FailureDomain == nil {
			continue
		}
		failureDomainCounts[*machine.Spec.FailureDomain]++
		instanceOcid := strings.Split(*machine.Spec.ProviderID, "//")[1]
		Log(fmt.Sprintf("Instance OCID is %s", instanceOcid))

		resp, err := computeClient.GetInstance(context.Background(), core.GetInstanceRequest{
			InstanceId: common.String(instanceOcid),
		})
		Expect(err).NotTo(HaveOccurred())
		if adCount > 1 {
			ociFailureDomain[*resp.AvailabilityDomain]++
		} else {
			ociFailureDomain[*resp.FaultDomain]++
		}
	}
	Expect(len(failureDomainCounts)).To(Equal(3))
	Expect(len(ociFailureDomain)).To(Equal(3))
}

func validateOLImage(nameSpace string, clusterName string) {
	lister := bootstrapClusterProxy.GetClient()
	inClustersNamespaceListOption := client.InNamespace(nameSpace)
	matchClusterListOption := client.MatchingLabels{
		clusterv1.ClusterNameLabel: clusterName,
	}

	machineList := &clusterv1.MachineList{}
	Expect(lister.List(context.Background(), machineList, inClustersNamespaceListOption, matchClusterListOption)).
		To(Succeed(), "Couldn't list machines for the cluster %q", clusterName)

	Expect(len(machineList.Items)).To(Equal(2))
	for _, machine := range machineList.Items {
		instanceOcid := strings.Split(*machine.Spec.ProviderID, "//")[1]
		Log(fmt.Sprintf("Instance OCID is %s", instanceOcid))
		resp, err := computeClient.GetInstance(context.Background(), core.GetInstanceRequest{
			InstanceId: common.String(instanceOcid),
		})
		Expect(err).NotTo(HaveOccurred())
		instanceSourceDetails, ok := resp.SourceDetails.(core.InstanceSourceViaImageDetails)
		Expect(ok).To(BeTrue())
		Expect(*instanceSourceDetails.ImageId).To(Equal(os.Getenv("OCI_ORACLE_LINUX_IMAGE_ID")))
	}
}

func validateWindowsImage(nameSpace string, clusterName string) {
	lister := bootstrapClusterProxy.GetClient()
	inClustersNamespaceListOption := client.InNamespace(nameSpace)
	matchClusterListOption := client.MatchingLabels{
		clusterv1.ClusterNameLabel: clusterName,
	}

	machineList := &clusterv1.MachineList{}
	Expect(lister.List(context.Background(), machineList, inClustersNamespaceListOption, matchClusterListOption)).
		To(Succeed(), "Couldn't list machines for the cluster %q", clusterName)

	Expect(len(machineList.Items)).To(Equal(2))
	for _, machine := range machineList.Items {
		if machine.Labels["os"] == "windows" {
			instanceOcid := strings.Split(*machine.Spec.ProviderID, "//")[1]
			Log(fmt.Sprintf("Instance OCID is %s", instanceOcid))
			resp, err := computeClient.GetInstance(context.Background(), core.GetInstanceRequest{
				InstanceId: common.String(instanceOcid),
			})
			Expect(err).NotTo(HaveOccurred())
			instanceSourceDetails, ok := resp.SourceDetails.(core.InstanceSourceViaImageDetails)
			Expect(ok).To(BeTrue())
			Expect(*instanceSourceDetails.ImageId).To(Equal(os.Getenv("OCI_WINDOWS_IMAGE_ID")))
		}
	}
}

func getClusterName(prefix, specName string) string {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = fmt.Sprintf("%s-%s", prefix, specName)
	}
	fmt.Fprintf(GinkgoWriter, "INFO: Cluster name is %s\n", clusterName)
	return clusterName
}

func getProtocolOptionsForSpec(icmp *core.IcmpOptions, tcp *core.TcpOptions, udp *core.UdpOptions) (*infrastructurev1beta1.IcmpOptions, *infrastructurev1beta1.TcpOptions,
	*infrastructurev1beta1.UdpOptions) {
	var icmpOptions *infrastructurev1beta1.IcmpOptions
	var tcpOptions *infrastructurev1beta1.TcpOptions
	var udpOptions *infrastructurev1beta1.UdpOptions
	if icmp != nil {
		icmpOptions = &infrastructurev1beta1.IcmpOptions{
			Type: icmp.Type,
			Code: icmp.Type,
		}
	}
	if tcp != nil {
		tcpOptions = &infrastructurev1beta1.TcpOptions{}
		if tcp.DestinationPortRange != nil {
			tcpOptions.DestinationPortRange = &infrastructurev1beta1.PortRange{}
			tcpOptions.DestinationPortRange.Max = tcp.DestinationPortRange.Max
			tcpOptions.DestinationPortRange.Min = tcp.DestinationPortRange.Min
		}
		if tcp.SourcePortRange != nil {
			tcpOptions.SourcePortRange = &infrastructurev1beta1.PortRange{}
			tcpOptions.SourcePortRange.Max = tcp.SourcePortRange.Max
			tcpOptions.SourcePortRange.Min = tcp.SourcePortRange.Min
		}
	}
	if udp != nil {
		udpOptions = &infrastructurev1beta1.UdpOptions{}
		if udp.DestinationPortRange != nil {
			udpOptions.DestinationPortRange = &infrastructurev1beta1.PortRange{}
			udpOptions.DestinationPortRange.Max = udp.DestinationPortRange.Max
			udpOptions.DestinationPortRange.Min = udp.DestinationPortRange.Min
		}
		if udp.SourcePortRange != nil {
			udpOptions.SourcePortRange = &infrastructurev1beta1.PortRange{}
			udpOptions.SourcePortRange.Max = udp.SourcePortRange.Max
			udpOptions.SourcePortRange.Min = udp.SourcePortRange.Min
		}
	}
	return icmpOptions, tcpOptions, udpOptions
}

func convertSecurityListIngressRule(rule infrastructurev1beta1.IngressSecurityRule) core.IngressSecurityRule {
	var icmpOptions *core.IcmpOptions
	var tcpOptions *core.TcpOptions
	var udpOptions *core.UdpOptions
	icmpOptions, tcpOptions, udpOptions = getProtocolOptions(rule.IcmpOptions, rule.TcpOptions, rule.UdpOptions)

	// while comparing values, the boolean value has to be always set
	stateless := rule.IsStateless
	if stateless == nil {
		stateless = common.Bool(false)
	}
	return core.IngressSecurityRule{
		Protocol:    rule.Protocol,
		Source:      rule.Source,
		IcmpOptions: icmpOptions,
		IsStateless: stateless,
		SourceType:  core.IngressSecurityRuleSourceTypeEnum(rule.SourceType),
		TcpOptions:  tcpOptions,
		UdpOptions:  udpOptions,
		Description: rule.Description,
	}
}

func convertSecurityListEgressRule(rule infrastructurev1beta1.EgressSecurityRule) core.EgressSecurityRule {
	var icmpOptions *core.IcmpOptions
	var tcpOptions *core.TcpOptions
	var udpOptions *core.UdpOptions

	// while comparing values, the boolean value has to be always set
	stateless := rule.IsStateless
	if stateless == nil {
		stateless = common.Bool(false)
	}
	icmpOptions, tcpOptions, udpOptions = getProtocolOptions(rule.IcmpOptions, rule.TcpOptions, rule.UdpOptions)
	return core.EgressSecurityRule{
		Protocol:        rule.Protocol,
		Destination:     rule.Destination,
		IcmpOptions:     icmpOptions,
		IsStateless:     stateless,
		DestinationType: core.EgressSecurityRuleDestinationTypeEnum(rule.DestinationType),
		TcpOptions:      tcpOptions,
		UdpOptions:      udpOptions,
		Description:     rule.Description,
	}
}

func getProtocolOptions(icmp *infrastructurev1beta1.IcmpOptions, tcp *infrastructurev1beta1.TcpOptions,
	udp *infrastructurev1beta1.UdpOptions) (*core.IcmpOptions, *core.TcpOptions, *core.UdpOptions) {
	var icmpOptions *core.IcmpOptions
	var tcpOptions *core.TcpOptions
	var udpOptions *core.UdpOptions
	if icmp != nil {
		icmpOptions = &core.IcmpOptions{
			Type: icmp.Type,
			Code: icmp.Type,
		}
	}
	if tcp != nil {
		tcpOptions = &core.TcpOptions{}
		if tcp.DestinationPortRange != nil {
			tcpOptions.DestinationPortRange = &core.PortRange{}
			tcpOptions.DestinationPortRange.Max = tcp.DestinationPortRange.Max
			tcpOptions.DestinationPortRange.Min = tcp.DestinationPortRange.Min
		}
		if tcp.SourcePortRange != nil {
			tcpOptions.SourcePortRange = &core.PortRange{}
			tcpOptions.SourcePortRange.Max = tcp.SourcePortRange.Max
			tcpOptions.SourcePortRange.Min = tcp.SourcePortRange.Min
		}
	}
	if udp != nil {
		udpOptions = &core.UdpOptions{}
		if udp.DestinationPortRange != nil {
			udpOptions.DestinationPortRange = &core.PortRange{}
			udpOptions.DestinationPortRange.Max = udp.DestinationPortRange.Max
			udpOptions.DestinationPortRange.Min = udp.DestinationPortRange.Min
		}
		if udp.SourcePortRange != nil {
			udpOptions.SourcePortRange = &core.PortRange{}
			udpOptions.SourcePortRange.Max = udp.SourcePortRange.Max
			udpOptions.SourcePortRange.Min = udp.SourcePortRange.Min
		}
	}
	return icmpOptions, tcpOptions, udpOptions
}
