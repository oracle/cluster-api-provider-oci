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

	fuzz "github.com/google/gofuzz"
	. "github.com/onsi/gomega"
	infrav1beta1 "github.com/oracle/cluster-api-provider-oci/api/v1beta1"
	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		OCIMachinePoolFuzzer,
		OCIClusterFuzzer,
	}
}

func OCIMachinePoolFuzzer(obj *OCIMachinePool, c fuzz.Continue) {
	c.FuzzNoCustom(obj)
	// nil fields which have been removed so that tests dont fail
	if obj.Spec.InstanceConfiguration.InstanceVnicConfiguration != nil {
		obj.Spec.InstanceConfiguration.InstanceVnicConfiguration.NSGId = nil
		obj.Spec.InstanceConfiguration.InstanceVnicConfiguration.SubnetId = nil
	}
}

func OCIClusterFuzzer(obj *OCIManagedCluster, c fuzz.Continue) {
	c.FuzzNoCustom(obj)
	// nil fields which have been removed so that tests dont fail
	for _, nsg := range obj.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
		if nsg != nil {
			ingressRules := make([]infrav1beta1.IngressSecurityRuleForNSG, len(nsg.IngressRules))
			for _, rule := range nsg.IngressRules {
				rule.ID = nil
				ingressRules = append(ingressRules, rule)
			}
			nsg.IngressRules = ingressRules

			egressRules := make([]infrav1beta1.EgressSecurityRuleForNSG, len(nsg.EgressRules))
			for _, rule := range nsg.EgressRules {
				(&rule).ID = nil
				egressRules = append(egressRules, rule)
			}
			nsg.EgressRules = egressRules
		}
	}
}

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1beta2.AddToScheme(scheme)).To(Succeed())

	t.Run("for OCIManagedCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIManagedCluster{},
		Spoke:       &OCIManagedCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIMachinePool", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIMachinePool{},
		Spoke:       &OCIMachinePool{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIManagedMachinePool", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIManagedMachinePool{},
		Spoke:       &OCIManagedMachinePool{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIManagedControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIManagedControlPlane{},
		Spoke:       &OCIManagedControlPlane{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

}
