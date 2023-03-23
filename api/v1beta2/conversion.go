/*
 Copyright (c) 2021, 2022 Oracle and/or its affiliates.

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

package v1beta2

// Hub marks OCICluster as a conversion hub.
func (*OCICluster) Hub() {}

// Hub marks OCICluster as a conversion hub.
func (*OCIClusterList) Hub() {}

// Hub marks OCICluster as a conversion hub.
func (*OCIClusterTemplate) Hub() {}

// Hub marks OCICluster as a conversion hub.
func (*OCIClusterTemplateList) Hub() {}

// Hub marks OCICluster as a conversion hub.
func (*OCIMachine) Hub() {}

// Hub marks OCICluster as a conversion hub.
func (*OCIMachineList) Hub() {}

// Hub marks OCICluster as a conversion hub.
func (*OCIMachineTemplate) Hub() {}

// Hub marks OCICluster as a conversion hub.
func (*OCIMachineTemplateList) Hub() {}

// Hub marks OCICluster as a conversion hub.
func (*OCIClusterIdentity) Hub() {}

// Hub marks OCICluster as a conversion hub.
func (*OCIClusterIdentityList) Hub() {}
