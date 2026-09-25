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
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func triggerObjectReconcile(t *testing.T, object client.Object) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() error {
		stored := object.DeepCopyObject().(client.Object)
		if err := testEnvironment.GetAPIReader().Get(testContext, client.ObjectKeyFromObject(object), stored); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		annotations := stored.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations["integration-test/reconcile"] = time.Now().UTC().Format(time.RFC3339Nano)
		stored.SetAnnotations(annotations)
		return testEnvironment.Update(testContext, stored)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
}

func waitForComputeAttempt(t *testing.T, operation fakeComputeOperation, previous int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() int {
		return fakeOCI.compute.operationAttemptCount(operation)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeNumerically(">", previous))
}

func waitForVCNAttempt(t *testing.T, operation fakeVCNOperation, previous int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() int {
		return fakeOCI.vcn.operationAttemptCount(operation)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeNumerically(">", previous))
}

func waitForOKEAttempt(t *testing.T, operation fakeOKEOperation, previous int) {
	t.Helper()
	g := NewWithT(t)
	g.Eventually(func() int {
		return fakeOCI.oke.operationAttemptCount(operation)
	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(BeNumerically(">", previous))
}
