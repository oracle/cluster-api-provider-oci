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

package v1beta2

import (
	"sort"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/randfill"
)

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		v1beta2ConditionFuzzer,
		v1beta2ControlPlaneStatusFuzzer,
		v1beta2OCIMachineFuzzer,
		v1beta2OCIMachineTemplateFuzzer,
		v1beta3OCIClusterFuzzer,
		v1beta3OCIClusterIdentityFuzzer,
		v1beta3OCIManagedClusterFuzzer,
		v1beta3OCIMachineFuzzer,
		v1beta3OCIMachineTemplateFuzzer,
		v1beta3OCIManagedControlPlaneFuzzer,
	}
}

func v1beta2ConditionFuzzer(obj *Condition, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Severity = ""
	obj.Reason = sanitizeConditionReason(obj.Reason, obj.Type, obj.Status)
}

func v1beta2ControlPlaneStatusFuzzer(obj *OCIManagedControlPlaneStatus, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Ready = obj.Initialized
	if obj.Version != nil && *obj.Version == "" {
		obj.Version = nil
	}
}

func v1beta2OCIMachineFuzzer(obj *OCIMachine, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.ExtendedMetadata = nil
}

func v1beta2OCIMachineTemplateFuzzer(obj *OCIMachineTemplate, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.Template.Spec.ExtendedMetadata = nil
}

func normalizeHubFailureDomains(fds []clusterv1.FailureDomain) []clusterv1.FailureDomain {
	if len(fds) == 0 {
		return nil
	}
	seen := make(map[string]clusterv1.FailureDomain, len(fds))
	for _, fd := range fds {
		if fd.ControlPlane == nil {
			fd.ControlPlane = ptr.To(false)
		}
		if len(fd.Attributes) == 0 {
			fd.Attributes = nil
		}
		seen[fd.Name] = fd
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]clusterv1.FailureDomain, 0, len(names))
	for _, name := range names {
		out = append(out, seen[name])
	}
	return out
}

func normalizeHubConditions(conds []metav1.Condition) []metav1.Condition {
	if len(conds) == 0 {
		return conds
	}
	seen := make(map[string]bool, len(conds))
	out := make([]metav1.Condition, 0, len(conds))
	for _, c := range conds {
		if !seen[c.Type] {
			seen[c.Type] = true
			c.Reason = sanitizeConditionReason(c.Reason, ConditionType(c.Type), corev1.ConditionStatus(c.Status))
			out = append(out, c)
		}
	}
	return out
}

func v1beta3OCIClusterFuzzer(obj *v1beta3.OCICluster, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.FailureDomains = normalizeHubFailureDomains(obj.Status.FailureDomains)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func v1beta3OCIClusterIdentityFuzzer(obj *v1beta3.OCIClusterIdentity, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func v1beta3OCIManagedClusterFuzzer(obj *v1beta3.OCIManagedCluster, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.FailureDomains = normalizeHubFailureDomains(obj.Status.FailureDomains)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func v1beta3OCIMachineFuzzer(obj *v1beta3.OCIMachine, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.ExtendedMetadata = nil
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func v1beta3OCIMachineTemplateFuzzer(obj *v1beta3.OCIMachineTemplate, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.Template.Spec.ExtendedMetadata = nil
}

func v1beta3OCIManagedControlPlaneFuzzer(obj *v1beta3.OCIManagedControlPlane, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
	for i := range obj.Status.Versions {
		if obj.Status.Versions[i].Version == "" {
			obj.Status.Versions[i].Version = "v1.32.0"
		}
	}
}

func TestLegacyConditionReasonSanitization(t *testing.T) {
	g := NewWithT(t)

	in := Conditions{
		{Type: "ClusterReady", Status: corev1.ConditionTrue, Reason: ""},
		{Type: "InstanceReady", Status: corev1.ConditionFalse, Reason: "Instance failed! (timeout)"},
		{Type: "123InvalidType", Status: corev1.ConditionTrue, Reason: ""},
		{Type: "123InvalidTypeFalse", Status: corev1.ConditionFalse, Reason: "!!!"},
		{Type: "123InvalidTypeUnknown", Status: corev1.ConditionUnknown, Reason: ""},
		{Type: "Ready", Status: corev1.ConditionFalse, Reason: "ValidReason_1"},
	}

	out := ConvertConditionsToMeta(in)
	g.Expect(out).To(HaveLen(len(in)))
	g.Expect(out[0].Reason).To(Equal("ClusterReady"))
	g.Expect(out[1].Reason).To(Equal("Instancefailedtimeout"))
	g.Expect(out[2].Reason).To(Equal("InvalidType"))
	g.Expect(out[3].Reason).To(Equal("InvalidTypeFalse"))
	g.Expect(out[4].Reason).To(Equal("InvalidTypeUnknown"))
	g.Expect(out[5].Reason).To(Equal("ValidReason_1"))

	for _, c := range out {
		g.Expect(validConditionReasonRegex.MatchString(c.Reason)).To(BeTrue(), "reason %q must satisfy metav1.Condition OpenAPI pattern", c.Reason)
	}

	// Purely non-alphabetic type and empty reason fall back by status.
	fallback := ConvertConditionsToMeta(Conditions{
		{Type: "123", Status: corev1.ConditionTrue, Reason: ""},
		{Type: "456", Status: corev1.ConditionFalse, Reason: ""},
		{Type: "789", Status: corev1.ConditionUnknown, Reason: ""},
	})
	g.Expect(fallback[0].Reason).To(Equal("Succeeded"))
	g.Expect(fallback[1].Reason).To(Equal("Failed"))
	g.Expect(fallback[2].Reason).To(Equal("Unknown"))
}

func TestConvertToDoesNotClobberUpdatedStatusFromStaleAnnotation(t *testing.T) {
	g := NewWithT(t)

	// 1. OCICluster: ConvertFrom with Provisioned=true, then v1beta2 client sets Ready=false.
	hubCluster := &v1beta3.OCICluster{
		Status: v1beta3.OCIClusterStatus{
			Initialization: v1beta3.OCIClusterInitializationStatus{
				Provisioned: ptr.To(true),
			},
		},
	}
	spokeCluster := &OCICluster{}
	g.Expect(spokeCluster.ConvertFrom(hubCluster)).To(Succeed())
	g.Expect(spokeCluster.Status.Ready).To(BeTrue())

	spokeCluster.Status.Ready = false
	convertedCluster := &v1beta3.OCICluster{}
	g.Expect(spokeCluster.ConvertTo(convertedCluster)).To(Succeed())
	g.Expect(convertedCluster.Status.Initialization.Provisioned).To(BeNil())

	// 2. OCIManagedControlPlane: ConvertFrom with Versions=[v1.31.0], then v1beta2 client updates Version=v1.32.0.
	hubCP := &v1beta3.OCIManagedControlPlane{
		Status: v1beta3.OCIManagedControlPlaneStatus{
			Initialization: v1beta3.OCIManagedControlPlaneInitializationStatus{
				ControlPlaneInitialized: ptr.To(true),
			},
			Versions: []clusterv1.StatusVersion{{Version: "v1.31.0"}, {Version: "v1.31.1"}},
		},
	}
	spokeCP := &OCIManagedControlPlane{}
	g.Expect(spokeCP.ConvertFrom(hubCP)).To(Succeed())
	g.Expect(spokeCP.Status.Version).To(Equal(ptr.To("v1.31.0")))

	spokeCP.Status.Version = ptr.To("v1.32.0")
	spokeCP.Status.Initialized = false
	spokeCP.Status.Ready = false
	convertedCP := &v1beta3.OCIManagedControlPlane{}
	g.Expect(spokeCP.ConvertTo(convertedCP)).To(Succeed())
	g.Expect(convertedCP.Status.Versions).To(Equal([]clusterv1.StatusVersion{{Version: "v1.32.0"}}))
	g.Expect(convertedCP.Status.Initialization.ControlPlaneInitialized).To(BeNil())
}

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1beta3.AddToScheme(scheme)).To(Succeed())

	t.Run("for OCICluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCICluster{},
		Spoke:       &OCICluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIClusterTemplate{},
		Spoke:       &OCIClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIMachine{},
		Spoke:       &OCIMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIMachineTemplate{},
		Spoke:       &OCIMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIClusterIdentity", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIClusterIdentity{},
		Spoke:       &OCIClusterIdentity{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIManagedCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIManagedCluster{},
		Spoke:       &OCIManagedCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIManagedClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIManagedClusterTemplate{},
		Spoke:       &OCIManagedClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIManagedControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIManagedControlPlane{},
		Spoke:       &OCIManagedControlPlane{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIManagedControlPlaneTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIManagedControlPlaneTemplate{},
		Spoke:       &OCIManagedControlPlaneTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))
}
