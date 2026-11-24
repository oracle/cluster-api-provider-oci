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

package v1beta1

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
		conditionFuzzer,
		controlPlaneStatusFuzzer,
		OCIMachineFuzzer,
		OCIMachineHubFuzzer,
		OCIMachineTemplateFuzzer,
		OCIMachineTemplateHubFuzzer,
		OCIClusterFuzzer,
		OCIClusterHubFuzzer,
		OCIClusterTemplateFuzzer,
		OCIManagedClusterFuzzer,
		OCIManagedClusterHubFuzzer,
		OCIManagedClusterTemplateFuzzer,
		OCIManagedControlPlaneHubFuzzer,
		OCIClusterIdentityHubFuzzer,
	}
}

func conditionFuzzer(obj *Condition, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Severity = ""
	obj.Reason = sanitizeConditionReason(obj.Reason, obj.Type, obj.Status)
}

func controlPlaneStatusFuzzer(obj *OCIManagedControlPlaneStatus, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Ready = obj.Initialized
	if obj.Version != nil && *obj.Version == "" {
		obj.Version = nil
	}
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

func clearRemovedNSGRuleIDs(nsgs []*NSG) {
	for _, nsg := range nsgs {
		if nsg != nil {
			for i := range nsg.IngressRules {
				nsg.IngressRules[i].ID = nil
			}
			for i := range nsg.EgressRules {
				nsg.EgressRules[i].ID = nil
			}
		}
	}
}

func OCIMachineFuzzer(obj *OCIMachine, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.NSGName = ""
	obj.Spec.ExtendedMetadata = nil
}

func OCIClusterFuzzer(obj *OCICluster, c randfill.Continue) {
	c.FillNoCustom(obj)
	clearRemovedNSGRuleIDs(obj.Spec.NetworkSpec.Vcn.NetworkSecurityGroups)
}

func OCIClusterHubFuzzer(obj *v1beta3.OCICluster, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.FailureDomains = normalizeHubFailureDomains(obj.Status.FailureDomains)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func OCIClusterTemplateFuzzer(obj *OCIClusterTemplate, c randfill.Continue) {
	c.FillNoCustom(obj)
	clearRemovedNSGRuleIDs(obj.Spec.Template.Spec.NetworkSpec.Vcn.NetworkSecurityGroups)
}

func OCIMachineTemplateFuzzer(obj *OCIMachineTemplate, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.Template.Spec.NSGName = ""
	obj.Spec.Template.Spec.ExtendedMetadata = nil
}

func OCIMachineHubFuzzer(obj *v1beta3.OCIMachine, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.ExtendedMetadata = nil
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func OCIMachineTemplateHubFuzzer(obj *v1beta3.OCIMachineTemplate, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.Template.Spec.ExtendedMetadata = nil
}

func OCIManagedClusterFuzzer(obj *OCIManagedCluster, c randfill.Continue) {
	c.FillNoCustom(obj)
	clearRemovedNSGRuleIDs(obj.Spec.NetworkSpec.Vcn.NetworkSecurityGroups)
}

func OCIManagedClusterTemplateFuzzer(obj *OCIManagedClusterTemplate, c randfill.Continue) {
	c.FillNoCustom(obj)
	clearRemovedNSGRuleIDs(obj.Spec.Template.Spec.NetworkSpec.Vcn.NetworkSecurityGroups)
}

func OCIManagedClusterHubFuzzer(obj *v1beta3.OCIManagedCluster, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.FailureDomains = normalizeHubFailureDomains(obj.Status.FailureDomains)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func OCIManagedControlPlaneHubFuzzer(obj *v1beta3.OCIManagedControlPlane, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
	for i := range obj.Status.Versions {
		if obj.Status.Versions[i].Version == "" {
			obj.Status.Versions[i].Version = "v1.32.0"
		}
	}
}

func OCIClusterIdentityHubFuzzer(obj *v1beta3.OCIClusterIdentity, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
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

	t.Run("for OCIManagedControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIManagedControlPlane{},
		Spoke:       &OCIManagedControlPlane{},
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

	t.Run("for OCIManagedControlPlaneTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIManagedControlPlaneTemplate{},
		Spoke:       &OCIManagedControlPlaneTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))
}
