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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/conversion"

	fuzz "github.com/google/gofuzz"
	. "github.com/onsi/gomega"
	"github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		OCIMachineFuzzer,
		OCIMachineHubFuzzer,
		OCIMachineTemplateFuzzer,
		OCIMachineTemplateHubFuzzer,
		OCIClusterFuzzer,
		OCIClusterTemplateFuzzer,
		OCIManagedClusterFuzzer,
	}
}

func OCIMachineFuzzer(obj *OCIMachine, c fuzz.Continue) {
	c.FuzzNoCustom(obj)
	// nil fields which have been removed so that tests dont fail
	obj.Spec.NSGName = ""
	obj.Spec.ExtendedMetadata = nil
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

func OCIClusterTemplateFuzzer(obj *OCIClusterTemplate, c fuzz.Continue) {
	c.FuzzNoCustom(obj)
	// nil fields which have been removed so that tests dont fail
	for _, nsg := range obj.Spec.Template.Spec.NetworkSpec.Vcn.NetworkSecurityGroups {
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
	obj.Spec.Template.Spec.NSGName = ""
	obj.Spec.Template.Spec.ExtendedMetadata = nil
}

func OCIMachineHubFuzzer(obj *v1beta2.OCIMachine, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.ExtendedMetadata = nil
}

func OCIMachineTemplateHubFuzzer(obj *v1beta2.OCIMachineTemplate, c randfill.Continue) {
	c.FillNoCustom(obj)
	obj.Spec.Template.Spec.ExtendedMetadata = nil
}

func OCIManagedClusterFuzzer(obj *OCIManagedCluster, c fuzz.Continue) {
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

	t.Run("for OCIClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIClusterTemplate{},
		Spoke:       &OCIClusterTemplate{},
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

	t.Run("for OCIManagedControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIManagedControlPlane{},
		Spoke:       &OCIManagedControlPlane{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for OCIManagedCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1beta2.OCIManagedCluster{},
		Spoke:       &OCIManagedCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

}

func TestConvert_v1beta1_VCN_To_v1beta2_VCN(t *testing.T) {
	type args struct {
		in  *VCN
		out *v1beta2.VCN
		s   conversion.Scope
	}

	// helper variables for pointers
	id := "vcn-1"

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal conversion",
			args: args{
				in: &VCN{
					ID:   &id,
					CIDR: "10.0.0.0/16",
					Name: "test-vcn",
				},
				out: &v1beta2.VCN{},
				s:   nil, // assuming Scope is not used in this test
			},
			wantErr: false,
		},
		{
			name: "partial input",
			args: args{
				in: &VCN{
					ID: &id,
				},
				out: &v1beta2.VCN{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta1_VCN_To_v1beta2_VCN(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta1_VCN_To_v1beta2_VCN() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta2_VCN_To_v1beta1_VCN(t *testing.T) {
	type args struct {
		in  *v1beta2.VCN
		out *VCN
		s   conversion.Scope
	}

	// helper variables for pointers
	id := "vcn-1"

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal conversion",
			args: args{
				in: &v1beta2.VCN{
					ID:   &id,           // keep pointer for ID
					CIDR: "10.0.0.0/16", // plain string
					Name: "test-vcn",    // plain string
				},
				out: &VCN{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "partial input",
			args: args{
				in: &v1beta2.VCN{
					ID: &id, // pointer
				},
				out: &VCN{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta2_VCN_To_v1beta1_VCN(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta2_VCN_To_v1beta1_VCN() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta1_OCIClusterStatus_To_v1beta2_OCIClusterStatus(t *testing.T) {
	type args struct {
		in  *OCIClusterStatus
		out *v1beta2.OCIClusterStatus
		s   conversion.Scope
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal conversion",
			args: args{
				in: &OCIClusterStatus{
					Ready: true,
					AvailabilityDomains: map[string]OCIAvailabilityDomain{
						"AD-1": {
							Name:         "Uocm:PHX-AD-1",
							FaultDomains: []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
						},
					},
				},
				out: &v1beta2.OCIClusterStatus{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "partial input",
			args: args{
				in: &OCIClusterStatus{
					Ready: true,
				},
				out: &v1beta2.OCIClusterStatus{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "empty input",
			args: args{
				in:  &OCIClusterStatus{},
				out: &v1beta2.OCIClusterStatus{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta1_OCIClusterStatus_To_v1beta2_OCIClusterStatus(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta1_OCIClusterStatus_To_v1beta2_OCIClusterStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta2_OCIClusterSpec_To_v1beta1_OCIClusterSpec(t *testing.T) {
	type args struct {
		in  *v1beta2.OCIClusterSpec
		out *OCIClusterSpec
		s   conversion.Scope
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal conversion",
			args: args{
				in: &v1beta2.OCIClusterSpec{
					Region: "us-phoenix-1",
					AvailabilityDomains: map[string]v1beta2.OCIAvailabilityDomain{
						"AD-1": {
							Name:         "Uocm:PHX-AD-1",
							FaultDomains: []string{"FAULT-DOMAIN-1", "FAULT-DOMAIN-2", "FAULT-DOMAIN-3"},
						},
					},
				},
				out: &OCIClusterSpec{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "partial input",
			args: args{
				in: &v1beta2.OCIClusterSpec{
					Region: "us-phoenix-1",
				},
				out: &OCIClusterSpec{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "empty input",
			args: args{
				in:  &v1beta2.OCIClusterSpec{},
				out: &OCIClusterSpec{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta2_OCIClusterSpec_To_v1beta1_OCIClusterSpec(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta2_OCIClusterSpec_To_v1beta1_OCIClusterSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta1_EgressSecurityRuleForNSG_To_v1beta2_EgressSecurityRuleForNSG(t *testing.T) {
	type args struct {
		in  *EgressSecurityRuleForNSG
		out *v1beta2.EgressSecurityRuleForNSG
		s   conversion.Scope
	}

	// Example inline variables for pointer fields
	destination := "10.0.0.0/16"
	description := "test rule"
	protocol := "6" // TCP
	stateless := true

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal conversion",
			args: args{
				in: &EgressSecurityRuleForNSG{
					EgressSecurityRule: EgressSecurityRule{
						Destination:     &destination,
						Description:     &description,
						Protocol:        &protocol,
						DestinationType: "CIDR_BLOCK",
						IsStateless:     &stateless,
					},
				},
				out: &v1beta2.EgressSecurityRuleForNSG{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "partial input",
			args: args{
				in: &EgressSecurityRuleForNSG{
					EgressSecurityRule: EgressSecurityRule{
						Destination: &destination,
					},
				},
				out: &v1beta2.EgressSecurityRuleForNSG{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "empty input",
			args: args{
				in:  &EgressSecurityRuleForNSG{},
				out: &v1beta2.EgressSecurityRuleForNSG{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta1_EgressSecurityRuleForNSG_To_v1beta2_EgressSecurityRuleForNSG(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta1_EgressSecurityRuleForNSG_To_v1beta2_EgressSecurityRuleForNSG() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta1_IngressSecurityRuleForNSG_To_v1beta2_IngressSecurityRuleForNSG(t *testing.T) {
	type args struct {
		in  *IngressSecurityRuleForNSG
		out *v1beta2.IngressSecurityRuleForNSG
		s   conversion.Scope
	}

	source := "192.168.0.0/24"
	protocol := "6" // TCP
	description := "allow http"
	stateless := false

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "full rule",
			args: args{
				in: &IngressSecurityRuleForNSG{
					IngressSecurityRule: IngressSecurityRule{
						Source:      &source,
						Description: &description,
						Protocol:    &protocol,
						SourceType:  "CIDR_BLOCK",
						IsStateless: &stateless,
					},
				},
				out: &v1beta2.IngressSecurityRuleForNSG{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "partial rule",
			args: args{
				in: &IngressSecurityRuleForNSG{
					IngressSecurityRule: IngressSecurityRule{
						Source: &source,
					},
				},
				out: &v1beta2.IngressSecurityRuleForNSG{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "empty rule",
			args: args{
				in:  &IngressSecurityRuleForNSG{},
				out: &v1beta2.IngressSecurityRuleForNSG{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta1_IngressSecurityRuleForNSG_To_v1beta2_IngressSecurityRuleForNSG(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta1_IngressSecurityRuleForNSG_To_v1beta2_IngressSecurityRuleForNSG() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta1_NetworkDetails_To_v1beta2_NetworkDetails(t *testing.T) {
	type args struct {
		in  *NetworkDetails
		out *v1beta2.NetworkDetails
		s   conversion.Scope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "full network details",
			args: args{
				in: &NetworkDetails{
					SubnetId:               func() *string { s := "subnet-123"; return &s }(),
					AssignIpv6Ip:           true,
					AssignPublicIp:         false,
					SubnetName:             "my-subnet",
					NSGIds:                 []string{"nsg-1", "nsg-2"},
					SkipSourceDestCheck:    func() *bool { b := true; return &b }(),
					NsgNames:               []string{"nsg-a", "nsg-b"},
					HostnameLabel:          func() *string { s := "host1"; return &s }(),
					DisplayName:            func() *string { s := "display"; return &s }(),
					AssignPrivateDnsRecord: func() *bool { b := false; return &b }(),
				},
				out: &v1beta2.NetworkDetails{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "partial network details",
			args: args{
				in: &NetworkDetails{
					SubnetName: "default-subnet",
				},
				out: &v1beta2.NetworkDetails{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "empty network details",
			args: args{
				in:  &NetworkDetails{},
				out: &v1beta2.NetworkDetails{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta1_NetworkDetails_To_v1beta2_NetworkDetails(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta1_NetworkDetails_To_v1beta2_NetworkDetails() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta1_OCIMachineSpec_To_v1beta2_OCIMachineSpec(t *testing.T) {
	type args struct {
		in  *OCIMachineSpec
		out *v1beta2.OCIMachineSpec
		s   conversion.Scope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "all fields populated",
			args: args{
				in: &OCIMachineSpec{
					Shape: "VM.Standard.E4.Flex",
					ShapeConfig: ShapeConfig{
						Ocpus:       "2",
						MemoryInGBs: "16",
					},
					BootVolumeSizeInGBs: "100",
					NetworkDetails: NetworkDetails{
						SubnetName:     "subnet-a",
						AssignPublicIp: true,
					},
				},
				out: &v1beta2.OCIMachineSpec{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "minimal spec",
			args: args{
				in: &OCIMachineSpec{
					Shape: "VM.Standard2.1",
				},
				out: &v1beta2.OCIMachineSpec{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "empty struct",
			args: args{
				in:  &OCIMachineSpec{},
				out: &v1beta2.OCIMachineSpec{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta1_OCIMachineSpec_To_v1beta2_OCIMachineSpec(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta1_OCIMachineSpec_To_v1beta2_OCIMachineSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta2_LoadBalancer_To_v1beta1_LoadBalancer(t *testing.T) {
	type args struct {
		in  *v1beta2.LoadBalancer
		out *LoadBalancer
		s   conversion.Scope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "all fields populated",
			args: args{
				in: &v1beta2.LoadBalancer{
					Name:             "lb-test",
					LoadBalancerId:   func() *string { s := "lb-ocid"; return &s }(),
					LoadBalancerType: "NLB",
				},
				out: &LoadBalancer{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "minimal load balancer",
			args: args{
				in: &v1beta2.LoadBalancer{
					Name: "lb-test",
				},
				out: &LoadBalancer{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "empty struct",
			args: args{
				in:  &v1beta2.LoadBalancer{},
				out: &LoadBalancer{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta2_LoadBalancer_To_v1beta1_LoadBalancer(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta2_LoadBalancer_To_v1beta1_LoadBalancer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta2_OCIManagedControlPlaneStatus_To_v1beta1_OCIManagedControlPlaneStatus(t *testing.T) {
	type args struct {
		in  *v1beta2.OCIManagedControlPlaneStatus
		out *OCIManagedControlPlaneStatus
		s   conversion.Scope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "all fields populated",
			args: args{
				in: &v1beta2.OCIManagedControlPlaneStatus{
					Ready:       true,
					Version:     func() *string { s := "v1.29.0"; return &s }(),
					Initialized: true,
					Conditions: v1beta1.Conditions{
						{
							Type:   "Available",
							Status: v1.ConditionTrue,
						},
					},
				},
				out: &OCIManagedControlPlaneStatus{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "minimal status",
			args: args{
				in: &v1beta2.OCIManagedControlPlaneStatus{
					Ready: true,
				},
				out: &OCIManagedControlPlaneStatus{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "empty status",
			args: args{
				in:  &v1beta2.OCIManagedControlPlaneStatus{},
				out: &OCIManagedControlPlaneStatus{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta2_OCIManagedControlPlaneStatus_To_v1beta1_OCIManagedControlPlaneStatus(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta2_OCIManagedControlPlaneStatus_To_v1beta1_OCIManagedControlPlaneStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvert_v1beta2_OCIManagedControlPlaneSpec_To_v1beta1_OCIManagedControlPlaneSpec(t *testing.T) {
	type args struct {
		in  *v1beta2.OCIManagedControlPlaneSpec
		out *OCIManagedControlPlaneSpec
		s   conversion.Scope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "almost all fields populated",
			args: args{
				in: &v1beta2.OCIManagedControlPlaneSpec{
					Version: func() *string { s := "v1.29.0"; return &s }(),
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "1.2.3.4",
						Port: 6443,
					},
				},
				out: &OCIManagedControlPlaneSpec{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "minimal spec",
			args: args{
				in: &v1beta2.OCIManagedControlPlaneSpec{
					Version: func() *string { s := "v1.29.0"; return &s }(),
				},
				out: &OCIManagedControlPlaneSpec{},
				s:   nil,
			},
			wantErr: false,
		},
		{
			name: "empty spec",
			args: args{
				in:  &v1beta2.OCIManagedControlPlaneSpec{},
				out: &OCIManagedControlPlaneSpec{},
				s:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Convert_v1beta2_OCIManagedControlPlaneSpec_To_v1beta1_OCIManagedControlPlaneSpec(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
				t.Errorf("Convert_v1beta2_OCIManagedControlPlaneSpec_To_v1beta1_OCIManagedControlPlaneSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// func TestConvert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus(t *testing.T) {
// 	type args struct {
// 		in  *OCIManagedClusterStatus
// 		out *v1beta2.OCIManagedClusterStatus
// 		s   conversion.Scope
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if err := Convert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
// 				t.Errorf("Convert_v1beta1_OCIManagedClusterStatus_To_v1beta2_OCIManagedClusterStatus() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestConvert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(t *testing.T) {
// 	type args struct {
// 		in  *v1beta2.OCIManagedClusterSpec
// 		out *OCIManagedClusterSpec
// 		s   conversion.Scope
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if err := Convert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
// 				t.Errorf("Convert_v1beta2_OCIManagedClusterSpec_To_v1beta1_OCIManagedClusterSpec() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestConvert_v1beta2_ClusterOptions_To_v1beta1_ClusterOptions(t *testing.T) {
// 	type args struct {
// 		in  *v1beta2.ClusterOptions
// 		out *ClusterOptions
// 		s   conversion.Scope
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if err := Convert_v1beta2_ClusterOptions_To_v1beta1_ClusterOptions(tt.args.in, tt.args.out, tt.args.s); (err != nil) != tt.wantErr {
// 				t.Errorf("Convert_v1beta2_ClusterOptions_To_v1beta1_ClusterOptions() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }
