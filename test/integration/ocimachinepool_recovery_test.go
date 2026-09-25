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
	"errors"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	infrav2exp "github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestOCIMachinePoolRecoversFromCloudFailures(t *testing.T) {
	g := NewWithT(t)
	fakeOCI.reset()
	fakeOCI.computeManagement.setFailure(createInstancePoolOperation, errors.New("injected instance-pool create failure"))

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "instance-pool-recovery-"}}
	g.Expect(testEnvironment.Create(testContext, namespace)).To(Succeed())

	cluster := instancePoolCAPICluster(namespace.Name)
	g.Expect(testEnvironment.Create(testContext, cluster)).To(Succeed())
	storedCluster := &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Status.Initialization.InfrastructureProvisioned = common.Bool(true)
	g.Expect(testEnvironment.Status().Update(testContext, storedCluster)).To(Succeed())

	ociCluster := instancePoolOCICluster(cluster)
	g.Expect(testEnvironment.Create(testContext, ociCluster)).To(Succeed())
	storedOCICluster := &infrastructurev1beta2.OCICluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(ociCluster), storedOCICluster)).To(Succeed())
	storedOCICluster.Status.Ready = true
	g.Expect(testEnvironment.Status().Update(testContext, storedOCICluster)).To(Succeed())

	bootstrapSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-pool-recovery-bootstrap", Namespace: namespace.Name},
		Data:       map[string][]byte{"value": []byte("#!/bin/sh\necho instance-pool-recovery")},
	}
	g.Expect(testEnvironment.Create(testContext, bootstrapSecret)).To(Succeed())

	machinePool := instanceCAPIMachinePool(cluster, bootstrapSecret.Name, 2)
	g.Expect(testEnvironment.Create(testContext, machinePool)).To(Succeed())
	ociMachinePool := instanceInfrastructureMachinePool(machinePool, cluster.Name)
	g.Expect(testEnvironment.Create(testContext, ociMachinePool)).To(Succeed())

	t.Cleanup(func() {
		for _, operation := range []fakeComputeManagementOperation{
			createInstancePoolOperation,
			updateInstancePoolOperation,
			terminateInstancePoolOperation,
		} {
			fakeOCI.computeManagement.clearFailure(operation)
		}
		machines := &infrav2exp.OCIMachinePoolMachineList{}
		if err := testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(namespace.Name)); err == nil {
			for i := range machines.Items {
				deleteAndIgnoreNotFound(t, &machines.Items[i])
			}
		}
		for _, object := range []client.Object{ociMachinePool, machinePool, bootstrapSecret, ociCluster, cluster, namespace} {
			deleteAndIgnoreNotFound(t, object)
		}
	})

	storedCluster = &clusterv1.Cluster{}
	g.Expect(testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(cluster), storedCluster)).To(Succeed())
	storedCluster.Spec.Paused = common.Bool(false)
	g.Expect(testEnvironment.Update(testContext, storedCluster)).To(Succeed())

	ociMachinePoolKey := client.ObjectKeyFromObject(ociMachinePool)
	waitForComputeManagementAttempt(t, createInstancePoolOperation, 0)
	g.Eventually(func(g Gomega) {
		configurations, pools, configCreates, _, poolCreates, _, _, _ := fakeOCI.computeManagement.counts()
		g.Expect(configurations).To(Equal(1))
		g.Expect(pools).To(BeZero())
		g.Expect(configCreates).To(Equal(1))
		g.Expect(poolCreates).To(BeZero())

		stored := &infrav2exp.OCIMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, ociMachinePoolKey, stored)).To(Succeed())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrav2exp.MachinePoolFinalizer)).To(BeTrue())
		condition := v1beta1conditions.Get(stored, infrav2exp.InstancePoolReadyCondition)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(condition.Reason).To(Equal(infrav2exp.InstancePoolProvisionFailedReason))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForWarningEvent(t, namespace.Name, ociMachinePool.Name, "ReconcileError", "injected instance-pool create failure")

	fakeOCI.computeManagement.clearFailure(createInstancePoolOperation)
	triggerInstanceMachinePoolReconcile(t, ociMachinePool)
	waitForInstanceMachinePool(t, ociMachinePoolKey, 2, 2, 2, 1, 1, 0)
	g.Expect(fakeOCI.computeManagement.operationAttemptCount(createInstancePoolOperation)).To(BeNumerically(">=", 2))

	_, originalConfigurationID := fakeOCI.computeManagement.instancePoolState()
	updateAttempts := fakeOCI.computeManagement.operationAttemptCount(updateInstancePoolOperation)
	fakeOCI.computeManagement.setFailure(updateInstancePoolOperation, errors.New("injected instance-pool update failure"))
	g.Eventually(func() error {
		stored := &infrav2exp.OCIMachinePool{}
		if err := testEnvironment.GetAPIReader().Get(testContext, ociMachinePoolKey, stored); err != nil {
			return err
		}
		stored.Spec.InstanceConfiguration.ShapeConfig.MemoryInGBs = common.String("32")
		stored.Spec.InstanceConfiguration.InstanceSourceViaImageDetails.ImageId = common.String("ocid1.image.oc1..instance-pool-recovery-v2")
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForComputeManagementAttempt(t, updateInstancePoolOperation, updateAttempts)
	g.Eventually(func(g Gomega) {
		configurations, pools, configCreates, configDeletes, poolCreates, poolUpdates, _, _ := fakeOCI.computeManagement.counts()
		g.Expect(configurations).To(Equal(2))
		g.Expect(pools).To(Equal(1))
		g.Expect(configCreates).To(Equal(2))
		g.Expect(configDeletes).To(BeZero())
		g.Expect(poolCreates).To(Equal(1))
		g.Expect(poolUpdates).To(BeZero())

		poolSize, activeConfigurationID := fakeOCI.computeManagement.instancePoolState()
		g.Expect(poolSize).To(Equal(2))
		g.Expect(activeConfigurationID).To(Equal(originalConfigurationID))

		stored := &infrav2exp.OCIMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, ociMachinePoolKey, stored)).To(Succeed())
		g.Expect(stored.Spec.InstanceConfiguration.InstanceConfigurationId).NotTo(BeNil())
		g.Expect(*stored.Spec.InstanceConfiguration.InstanceConfigurationId).NotTo(Equal(originalConfigurationID))
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	waitForWarningEvent(t, namespace.Name, ociMachinePool.Name, "FailedUpdate", "injected instance-pool update failure")

	fakeOCI.computeManagement.clearFailure(updateInstancePoolOperation)
	triggerInstanceMachinePoolReconcile(t, ociMachinePool)
	waitForInstanceMachinePool(t, ociMachinePoolKey, 2, 2, 2, 1, 2, 1)
	_, replacementConfigurationID := fakeOCI.computeManagement.instancePoolState()
	g.Expect(replacementConfigurationID).NotTo(Equal(originalConfigurationID))
	_, _, _, configDeletes, _, _, _, _ := fakeOCI.computeManagement.counts()
	g.Expect(configDeletes).To(Equal(1))

	terminateAttempts := fakeOCI.computeManagement.operationAttemptCount(terminateInstancePoolOperation)
	fakeOCI.computeManagement.setFailure(terminateInstancePoolOperation, errors.New("injected instance-pool terminate failure"))
	g.Expect(testEnvironment.Delete(testContext, ociMachinePool)).To(Succeed())
	waitForComputeManagementAttempt(t, terminateInstancePoolOperation, terminateAttempts)
	waitForWarningEvent(t, namespace.Name, ociMachinePool.Name, "FailedDelete", "injected instance-pool terminate failure")
	g.Eventually(func(g Gomega) {
		stored := &infrav2exp.OCIMachinePool{}
		g.Expect(testEnvironment.GetAPIReader().Get(testContext, ociMachinePoolKey, stored)).To(Succeed())
		g.Expect(stored.DeletionTimestamp.IsZero()).To(BeFalse())
		g.Expect(controllerutil.ContainsFinalizer(stored, infrav2exp.MachinePoolFinalizer)).To(BeTrue())

		configurations, pools, _, deletes, _, _, terminates, _ := fakeOCI.computeManagement.counts()
		g.Expect(configurations).To(Equal(1))
		g.Expect(pools).To(Equal(1))
		g.Expect(deletes).To(Equal(1))
		g.Expect(terminates).To(BeZero())
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	fakeOCI.computeManagement.clearFailure(terminateInstancePoolOperation)
	triggerInstanceMachinePoolReconcile(t, ociMachinePool)
	waitForInstancePoolTerminateCount(t, 1)
	triggerInstanceMachinePoolReconcile(t, ociMachinePool)
	g.Eventually(func(g Gomega) {
		configurations, pools, _, deletes, _, _, terminates, _ := fakeOCI.computeManagement.counts()
		g.Expect(configurations).To(BeZero())
		g.Expect(pools).To(BeZero())
		g.Expect(deletes).To(Equal(2))
		g.Expect(terminates).To(Equal(1))
		err := testEnvironment.GetAPIReader().Get(testContext, ociMachinePoolKey, &infrav2exp.OCIMachinePool{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected OCI machine-pool deletion, got %v", err)
	}).WithTimeout(20 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	machines := &infrav2exp.OCIMachinePoolMachineList{}
	g.Expect(testEnvironment.GetAPIReader().List(testContext, machines, client.InNamespace(namespace.Name))).To(Succeed())
	g.Expect(machines.Items).NotTo(BeEmpty())
	machine := machines.Items[0].DeepCopy()
	g.Expect(controllerutil.ContainsFinalizer(machine, infrav2exp.MachinePoolMachineFinalizer)).To(BeTrue())
	g.Expect(testEnvironment.Delete(testContext, machine)).To(Succeed())
	g.Eventually(func() bool {
		err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(machine), &infrav2exp.OCIMachinePoolMachine{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
}

func waitForComputeManagementAttempt(t *testing.T, operation fakeComputeManagementOperation, previous int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() int {
		return fakeOCI.computeManagement.operationAttemptCount(operation)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeNumerically(">", previous))
}

func waitForWarningEvent(t *testing.T, namespace, objectName, reason, message string) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func(g Gomega) bool {
		events := &corev1.EventList{}
		g.Expect(testEnvironment.GetAPIReader().List(testContext, events, client.InNamespace(namespace))).To(Succeed())
		for i := range events.Items {
			event := &events.Items[i]
			if event.InvolvedObject.Name == objectName && event.Reason == reason && strings.Contains(event.Message, message) {
				return true
			}
		}
		return false
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
}
