/*
Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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

package v1beta1

import (
	"github.com/oracle/cluster-api-provider-oci/exp/api/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts the v1beta1 OCIManagedMachinePoolMachine receiver to a v1beta2 OCIManagedMachinePoolMachine.
func (src *OCIManagedMachinePoolMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.OCIManagedMachinePoolMachine)

	if err := Convert_v1beta1_OCIManagedMachinePoolMachine_To_v1beta2_OCIManagedMachinePoolMachine(src, dst, nil); err != nil {
		return err
	}

	restored := &v1beta2.OCIManagedMachinePoolMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	return nil
}

// ConvertFrom converts receiver to a v1beta2 OCIManagedMachinePoolMachine.
func (r *OCIManagedMachinePoolMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.OCIManagedMachinePoolMachine)

	if err := Convert_v1beta2_OCIManagedMachinePoolMachine_To_v1beta1_OCIManagedMachinePoolMachine(src, r, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, r); err != nil {
		return err
	}

	return nil
}
