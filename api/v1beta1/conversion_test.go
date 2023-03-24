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
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"testing"

	fuzz "github.com/google/gofuzz"
	. "github.com/onsi/gomega"
	"github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		OCIMachineFuzzer,
		OCIMachineTemplateFuzzer,
		OCIClusterFuzzer,
	}
}

func OCIMachineFuzzer(obj *OCIMachine, c fuzz.Continue) {
	c.FuzzNoCustom(obj)
	// nil fields which have been removed so that tests dont fail
	obj.Spec.NetworkDetails.NSGId = nil
	obj.Spec.NetworkDetails.SubnetId = nil
	obj.Spec.NSGName = ""
}

func OCIClusterFuzzer(obj *OCICluster, c fuzz.Continue) {
	c.FuzzNoCustom(obj)
	// nil fields which have been removed so that tests dont fail
	for _, nsg := range obj.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
		if nsg != nil {
			ingressRules := make([]IngressSecurityRuleForNSG, len(nsg.IngressRules))
			for _, rule := range nsg.IngressRules {
				rule.ID = nil
				ingressRules = append(ingressRules, rule)
			}
			nsg.IngressRules = ingressRules

			egressRules := make([]EgressSecurityRuleForNSG, len(nsg.EgressRules))
			for _, rule := range nsg.EgressRules {
				(&rule).ID = nil
				egressRules = append(egressRules, rule)
			}
			nsg.EgressRules = egressRules
		}
	}
}

func OCIMachineTemplateFuzzer(obj *OCIMachineTemplate, c fuzz.Continue) {
	c.FuzzNoCustom(obj)
	// nil fields which ave been removed so that tests dont fail
	obj.Spec.Template.Spec.NetworkDetails.NSGId = nil
	obj.Spec.Template.Spec.NetworkDetails.SubnetId = nil
	obj.Spec.Template.Spec.NSGName = ""
}

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1beta2.AddToScheme(scheme)).To(Succeed())

	t.Run("for OCICluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCICluster{},
		Spoke:       &OCICluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIMachine{},
		Spoke:       &OCIMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIMachineTemplate{},
		Spoke:       &OCIMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIClusterIdentity", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIClusterIdentity{},
		Spoke:       &OCIClusterIdentity{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

}
