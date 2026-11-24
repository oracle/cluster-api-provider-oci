/*
 *
 * Copyright (c) 2022, Oracle and/or its affiliates.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 *
 */

package v1beta2

import (
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_validateSubnetCIDR(t *testing.T) {
	fldPath := field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("cidr")

	tests := []struct {
		name       string
		subnetCidr string
		vcnCidr    string
		wantErrs   int
		wantMsg    string
		notWantMsg string
	}{
		{
			name:       "valid subnet inside vcn",
			subnetCidr: "10.0.0.0/24",
			vcnCidr:    "10.0.0.0/16",
			wantErrs:   0,
		},
		{
			name:       "valid subnet outside vcn",
			subnetCidr: "192.168.0.0/24",
			vcnCidr:    "10.0.0.0/16",
			wantErrs:   1,
			wantMsg:    "not in VCN address space",
		},
		{
			name:       "malformed subnet cidr with valid vcn reports only the format error",
			subnetCidr: "not-a-cidr",
			vcnCidr:    "10.0.0.0/16",
			wantErrs:   1,
			wantMsg:    "invalid CIDR format",
			notWantMsg: "not in VCN address space",
		},
		{
			name:       "empty subnet cidr is allowed",
			subnetCidr: "",
			vcnCidr:    "10.0.0.0/16",
			wantErrs:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			errs := validateSubnetCIDR(tt.subnetCidr, tt.vcnCidr, fldPath)

			g.Expect(errs).To(gomega.HaveLen(tt.wantErrs))

			if tt.wantMsg != "" {
				joined := joinErrs(errs)
				g.Expect(joined).To(gomega.ContainSubstring(tt.wantMsg))
			}
			if tt.notWantMsg != "" {
				joined := joinErrs(errs)
				g.Expect(joined).NotTo(gomega.ContainSubstring(tt.notWantMsg))
			}
		})
	}
}

func joinErrs(errs field.ErrorList) string {
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "; ")
}
