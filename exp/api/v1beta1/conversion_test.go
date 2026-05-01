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
	"os"
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/randfill"
)

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		OCIMachinePoolFuzzer,
		OCIMachinePoolHubFuzzer,
	}
}

func sampleExtendedMetadata() map[string]apiextensionsv1.JSON {
	return map[string]apiextensionsv1.JSON{
		"cilium-primary-vnic": {Raw: []byte(`{"ip-count":32,"cidr-blocks":["10.0.0.0/24"]}`)},
		"simple-key":          {Raw: []byte(`"string-value"`)},
	}
}

func sampleSecurityAttributes() map[string]map[string]apiextensionsv1.JSON {
	return map[string]map[string]apiextensionsv1.JSON{
		"Oracle-DataSecurity-ZPR": {
			"MaxEgressCount": {Raw: []byte(`{"value":"42","mode":"audit"}`)},
		},
	}
}

func OCIMachinePoolFuzzer(obj *OCIMachinePool, c randfill.Continue) {
	c.FillNoCustom(obj)
	// nil fields which have been removed so that tests dont fail
	if obj.Spec.InstanceConfiguration.InstanceVnicConfiguration != nil {
		obj.Spec.InstanceConfiguration.InstanceVnicConfiguration.NSGId = nil
		obj.Spec.InstanceConfiguration.InstanceVnicConfiguration.SubnetId = nil
		obj.Spec.InstanceConfiguration.InstanceVnicConfiguration.SecurityAttributes = sampleSecurityAttributes()
	}
	// Replace fuzzed bytes with valid JSON so the roundtrip works
	obj.Spec.InstanceConfiguration.ExtendedMetadata = sampleExtendedMetadata()
	obj.Spec.InstanceConfiguration.SecurityAttributes = sampleSecurityAttributes()
}

func OCIMachinePoolHubFuzzer(obj *v1beta2.OCIMachinePool, c randfill.Continue) {
	c.FillNoCustom(obj)
	// Replace fuzzed bytes with valid JSON so the roundtrip works
	obj.Spec.InstanceConfiguration.ExtendedMetadata = sampleExtendedMetadata()
	obj.Spec.InstanceConfiguration.SecurityAttributes = sampleSecurityAttributes()
	if obj.Spec.InstanceConfiguration.InstanceVnicConfiguration != nil {
		obj.Spec.InstanceConfiguration.InstanceVnicConfiguration.SecurityAttributes = sampleSecurityAttributes()
	}
}

func TestOCIMachinePoolDeferredAndOutOfScopeFieldsRemainAbsent(t *testing.T) {
	g := NewWithT(t)

	assertNoJSONField := func(obj interface{}, field string) {
		t.Helper()
		typ := reflect.TypeOf(obj)
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		for i := 0; i < typ.NumField(); i++ {
			jsonTag := typ.Field(i).Tag.Get("json")
			if strings.Split(jsonTag, ",")[0] == field {
				t.Fatalf("%s unexpectedly exposes json field %q", typ.Name(), field)
			}
		}
	}

	apiChecks := []struct {
		obj    interface{}
		fields []string
	}{
		{OCIMachinePoolSpec{}, []string{"definedTags", "displayName", "freeformTags", "loadBalancers", "placementConfigurations"}},
		{InstanceConfiguration{}, []string{"availabilityDomain", "blockVolumes", "faultDomain", "instanceSourceImageFilterDetails", "secondaryVnics"}},
		{MachinePoolNetworkDetails{}, []string{"privateIp"}},
		{PlacementDetails{}, []string{"secondaryVnicSubnets"}},
		{v1beta2.OCIMachinePoolSpec{}, []string{"definedTags", "displayName", "freeformTags", "loadBalancers", "placementConfigurations"}},
		{v1beta2.InstanceConfiguration{}, []string{"availabilityDomain", "blockVolumes", "faultDomain", "instanceSourceImageFilterDetails", "secondaryVnics"}},
		{v1beta2.MachinePoolNetworkDetails{}, []string{"privateIp"}},
		{v1beta2.PlacementDetails{}, []string{"secondaryVnicSubnets"}},
	}
	for _, check := range apiChecks {
		for _, field := range check.fields {
			assertNoJSONField(check.obj, field)
		}
	}

	crdBytes, err := os.ReadFile("../../../config/crd/bases/infrastructure.cluster.x-k8s.io_ocimachinepools.yaml")
	g.Expect(err).To(BeNil())
	crd := string(crdBytes)
	for _, field := range []string{
		"blockVolumes:",
		"definedTags:",
		"faultDomain:",
		"freeformTags:",
		"instanceSourceImageFilterDetails:",
		"loadBalancers:",
		"placementConfigurations:",
		"privateIp:",
		"secondaryVnicSubnets:",
		"secondaryVnics:",
	} {
		g.Expect(crd).ToNot(ContainSubstring(field), "CRD unexpectedly exposes %s", field)
	}
}

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1beta2.AddToScheme(scheme)).To(Succeed())

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

}
