/*
Copyright (c) 2022 Oracle and/or its affiliates.

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

package feature

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// MachinePool is used to enable instance pool support
	MachinePool featuregate.Feature = "MachinePool"

	// OKE is used to enable manged cluster(OKE)
	OKE featuregate.Feature = "OKE"
)

func init() {
	runtime.Must(MutableGates.Add(defaultCAPOCIFeatureGates))
}

// defaultCAPOCIFeatureGates consists of all known capa-specific feature keys.
// To add a new feature, define a key for it above and add it here.
var defaultCAPOCIFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	// Every feature should be initiated here:
	MachinePool: {Default: false, PreRelease: featuregate.Alpha},
	// Every feature should be initiated here:
	OKE: {Default: false, PreRelease: featuregate.Alpha},
}
