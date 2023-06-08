/*
Copyright (c) 2023 Oracle and/or its affiliates.

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

// Hub marks OCIManagedControlPlane as a conversion hub.
func (*OCIManagedControlPlane) Hub() {}

// Hub marks OCIManagedCluster as a conversion hub.
func (*OCIManagedCluster) Hub() {}

// Hub marks OCIManagedClusterTemplate as a conversion hub.
func (*OCIManagedClusterTemplate) Hub() {}

// Hub marks OCIManagedClusterTemplateList as a conversion hub.
func (*OCIManagedClusterTemplateList) Hub() {}

// Hub marks OCIManagedControlPlaneList as a conversion hub.
func (*OCIManagedControlPlaneList) Hub() {}

// Hub marks OCIManagedMachinePool as a conversion hub.
func (*OCIManagedMachinePool) Hub() {}

// Hub marks OCIManagedMachinePoolList as a conversion hub.
func (*OCIManagedMachinePoolList) Hub() {}

// Hub marks OCIMachinePool as a conversion hub.
func (*OCIMachinePool) Hub() {}

// Hub marks OCIMachinePoolList as a conversion hub.
func (*OCIMachinePoolList) Hub() {}

// Hub marks OCIVirtualMachinePool as a conversion hub.
func (*OCIVirtualMachinePool) Hub() {}

// Hub marks OCIVirtualMachinePool as a conversion hub.
func (*OCIVirtualMachinePoolList) Hub() {}
