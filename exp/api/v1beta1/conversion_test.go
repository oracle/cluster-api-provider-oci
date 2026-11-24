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
	"testing"

	. "github.com/onsi/gomega"
	infrastructurev1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta3"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/randfill"
)

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		conditionFuzzer,
		OCIMachinePoolFuzzer,
		OCIMachinePoolHubFuzzer,
		OCIManagedMachinePoolHubFuzzer,
		OCIVirtualMachinePoolHubFuzzer,
		OCIMachinePoolMachineHubFuzzer,
	}
}

func conditionFuzzer(obj *infrastructurev1beta1.Condition, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Severity = ""
	converted := infrastructurev1beta1.ConvertConditionsToMeta(infrastructurev1beta1.Conditions{*obj})
	obj.Reason = converted[0].Reason
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
			c.Reason = infrastructurev1beta1.ConvertConditionsToMeta(infrastructurev1beta1.ConvertConditionsFromMeta([]metav1.Condition{c}))[0].Reason
			out = append(out, c)
		}
	}
	return out
}

func sampleExtendedMetadata() map[string]apiextensionsv1.JSON {
	return map[string]apiextensionsv1.JSON{
		"cilium-primary-vnic": {Raw: []byte(`{"ip-count":32,"cidr-blocks":["10.0.0.0/24"]}`)},
		"simple-key":          {Raw: []byte(`"string-value"`)},
	}
}

func OCIMachinePoolFuzzer(obj *OCIMachinePool, c randfill.Continue) {
	c.FillNoCustom(obj)
	if obj.Spec.InstanceConfiguration.InstanceVnicConfiguration != nil {
		obj.Spec.InstanceConfiguration.InstanceVnicConfiguration.NSGId = nil
		obj.Spec.InstanceConfiguration.InstanceVnicConfiguration.SubnetId = nil
	}
	obj.Spec.InstanceConfiguration.ExtendedMetadata = sampleExtendedMetadata()
}

func OCIMachinePoolHubFuzzer(obj *v1beta3.OCIMachinePool, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.InstanceConfiguration.ExtendedMetadata = sampleExtendedMetadata()
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func OCIManagedMachinePoolHubFuzzer(obj *v1beta3.OCIManagedMachinePool, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func OCIVirtualMachinePoolHubFuzzer(obj *v1beta3.OCIVirtualMachinePool, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func OCIMachinePoolMachineHubFuzzer(obj *v1beta3.OCIMachinePoolMachine, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Status.Conditions = normalizeHubConditions(obj.Status.Conditions)
}

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1beta3.AddToScheme(scheme)).To(Succeed())

	t.Run("for OCIMachinePool", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIMachinePool{},
		Spoke:       &OCIMachinePool{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIManagedMachinePool", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIManagedMachinePool{},
		Spoke:       &OCIManagedMachinePool{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIManagedMachinePoolTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIManagedMachinePoolTemplate{},
		Spoke:       &OCIManagedMachinePoolTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIVirtualMachinePool", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIVirtualMachinePool{},
		Spoke:       &OCIVirtualMachinePool{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIMachinePoolMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta3.OCIMachinePoolMachine{},
		Spoke:       &OCIMachinePoolMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))
}
