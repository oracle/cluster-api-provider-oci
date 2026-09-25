//go:build integration

/*
Copyright (c) 2026 Oracle and/or its affiliates.

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

package integration_test

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/cluster-api-provider-oci/cloud/scope"
	"github.com/oracle/oci-go-sdk/v65/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestOCIMachineLifecycle(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "ocimachine-integration-"},
	}
	g.Expect(testEnvironment.Create(testContext, namespace)).To(Succeed())

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-lifecycle",
			Namespace: namespace.Name,
		},
		Spec: clusterv1.ClusterSpec{
			Paused: common.Bool(true),
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     scope.OCIClusterKind,
				Name:     "machine-lifecycle",
			},
		},
	}
	g.Expect(testEnvironment.Create(testContext, cluster)).To(Succeed())

	ociCluster := &infrastructurev1beta2.OCICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: namespace.Name,
			Annotations: map[string]string{
				// Keep the infrastructure controller from reconciling networking;
				// this test is scoped to the machine controller lifecycle.
				clusterv1.ManagedByAnnotation: "integration-test",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterv1.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: infrastructurev1beta2.OCIClusterSpec{
			CompartmentId: "ocid1.compartment.oc1..integration",
			Region:        scope.MockTestRegion,
		},
	}
	g.Expect(testEnvironment.Create(testContext, ociCluster)).To(Succeed())

	storedOCICluster := &infrastructurev1beta2.OCICluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(
		testContext,
		client.ObjectKeyFromObject(ociCluster),
		storedOCICluster,
	)).To(Succeed())
	storedOCICluster.Status.FailureDomains = clusterv1beta1.FailureDomains{
		"1": {
			Attributes: map[string]string{
				scope.AvailabilityDomain: "integration-ad-1",
				scope.FaultDomain:        "FAULT-DOMAIN-1",
			},
		},
	}
	g.Expect(testEnvironment.Status().Update(testContext, storedOCICluster)).To(Succeed())

	bootstrapSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-lifecycle-bootstrap",
			Namespace: namespace.Name,
		},
		Data: map[string][]byte{"value": []byte("#!/bin/sh\necho integration")},
	}
	g.Expect(testEnvironment.Create(testContext, bootstrapSecret)).To(Succeed())

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-lifecycle",
			Namespace: namespace.Name,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: cluster.Name,
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: common.String(bootstrapSecret.Name),
			},
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrastructurev1beta2.GroupVersion.Group,
				Kind:     scope.OCIMachineKind,
				Name:     "machine-lifecycle",
			},
			FailureDomain: "1",
		},
	}
	g.Expect(testEnvironment.Create(testContext, machine)).To(Succeed())

	ociMachine := &infrastructurev1beta2.OCIMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machine.Name,
			Namespace: namespace.Name,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(machine, clusterv1.GroupVersion.WithKind("Machine")),
			},
		},
		Spec: infrastructurev1beta2.OCIMachineSpec{
			ImageId: "ocid1.image.oc1..integration",
			Shape:   "VM.Standard.E4.Flex",
			NetworkDetails: infrastructurev1beta2.NetworkDetails{
				SubnetId: common.String("ocid1.subnet.oc1..integration"),
			},
		},
	}
	g.Expect(testEnvironment.Create(testContext, ociMachine)).To(Succeed())
	t.Cleanup(func() {
		for _, object := range []client.Object{
			ociMachine,
			machine,
			bootstrapSecret,
			ociCluster,
			cluster,
			namespace,
		} {
			deleteAndIgnoreNotFound(t, object)
		}
	})

	g.Consistently(func() int {
		launches, _, _ := fakeOCI.compute.counts()
		return launches
	}).WithTimeout(time.Second).WithPolling(100 * time.Millisecond).Should(Equal(0))

	storedCluster := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(
		testContext,
		client.ObjectKeyFromObject(cluster),
		storedCluster,
	)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())

	ociMachineKey := client.ObjectKeyFromObject(ociMachine)
	g.Eventually(func(g Gomega) {
		stored := &infrastructurev1beta2.OCIMachine{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, ociMachineKey, stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrastructurev1beta2.MachineFinalizer)).To(BeTrue())
		g.Expect(stored.Spec.InstanceId).NotTo(BeNil())
		g.Expect(stored.Spec.ProviderID).NotTo(BeNil())
		g.Expect(*stored.Spec.ProviderID).To(Equal(fmt.Sprintf("oci://%s", *stored.Spec.InstanceId)))
		g.Expect(stored.Status.Ready).To(BeTrue())
		g.Expect(v1beta1conditions.IsTrue(stored, infrastructurev1beta2.InstanceReadyCondition)).To(BeTrue())
		g.Expect(stored.Status.Addresses).To(ContainElement(clusterv1beta1.MachineAddress{
			Type:    clusterv1beta1.MachineInternalIP,
			Address: "10.0.0.10",
		}))
		launches, _, _ := fakeOCI.compute.counts()
		g.Expect(launches).To(Equal(1))
		g.Expect(fakeOCI.vcn.getVNICCalls()).To(BeNumerically(">=", 1))
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	g.Consistently(func() int {
		launches, _, _ := fakeOCI.compute.counts()
		return launches
	}).WithTimeout(750 * time.Millisecond).WithPolling(100 * time.Millisecond).Should(Equal(1))

	_, _, inspectionsBeforeMachineUpdate := fakeOCI.compute.counts()
	g.Consistently(func() int {
		_, _, inspections := fakeOCI.compute.counts()
		return inspections
	}).WithTimeout(300 * time.Millisecond).WithPolling(50 * time.Millisecond).
		Should(Equal(inspectionsBeforeMachineUpdate))

	storedMachine := &clusterv1.Machine{}
	g.Expect(testEnvironment.GetAPIReader().Get(
		testContext,
		client.ObjectKeyFromObject(machine),
		storedMachine,
	)).To(Succeed())
	storedMachine.Annotations = map[string]string{"integration-test/reconcile": time.Now().UTC().Format(time.RFC3339Nano)}
	g.Expect(testEnvironment.Update(testContext, storedMachine)).To(Succeed())
	g.Eventually(func() int {
		_, _, inspections := fakeOCI.compute.counts()
		return inspections
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).
		Should(BeNumerically(">", inspectionsBeforeMachineUpdate))

	launches, _, _ := fakeOCI.compute.counts()
	g.Expect(launches).To(Equal(1))

	g.Expect(testEnvironment.Delete(testContext, ociMachine)).To(Succeed())
	g.Eventually(func(g Gomega) {
		_, terminations, _ := fakeOCI.compute.counts()
		g.Expect(terminations).To(Equal(1))
		err := testEnvironment.GetAPIReader().Get(
			testContext,
			ociMachineKey,
			&infrastructurev1beta2.OCIMachine{},
		)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected OCIMachine deletion, got %v", err)
	}).WithTimeout(15 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}
