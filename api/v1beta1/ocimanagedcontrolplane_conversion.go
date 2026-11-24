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
	"github.com/oracle/cluster-api-provider-oci/api/v1beta3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *OCIManagedControlPlane) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedControlPlane)
	if err := Convert_v1beta1_OCIManagedControlPlane_To_v1beta3_OCIManagedControlPlane(src, dst, nil); err != nil {
		return err
	}

	restored := &v1beta3.OCIManagedControlPlane{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.ClusterType = restored.Spec.ClusterType
	dst.Spec.Addons = restored.Spec.Addons
	dst.Status.AddonStatus = restored.Status.AddonStatus

	if restored.Spec.ClusterOption.OpenIdConnectDiscovery != nil {
		if dst.Spec.ClusterOption.OpenIdConnectDiscovery == nil {
			dst.Spec.ClusterOption.OpenIdConnectDiscovery = &v1beta3.OpenIDConnectDiscovery{}
		}
		dst.Spec.ClusterOption.OpenIdConnectDiscovery.IsOpenIdConnectDiscoveryEnabled =
			restored.Spec.ClusterOption.OpenIdConnectDiscovery.IsOpenIdConnectDiscoveryEnabled
	}

	if restored.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig != nil {
		if dst.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig == nil {
			dst.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig = &v1beta3.OpenIDConnectTokenAuthenticationConfig{}
		}
		*dst.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig =
			*restored.Spec.ClusterOption.OpenIdConnectTokenAuthenticationConfig
	}

	if restored.Status.Initialization.ControlPlaneInitialized != nil && !*restored.Status.Initialization.ControlPlaneInitialized && !src.Status.Initialized && !src.Status.Ready {
		dst.Status.Initialization.ControlPlaneInitialized = restored.Status.Initialization.ControlPlaneInitialized
	}
	if len(restored.Status.Versions) > 0 && src.Status.Version != nil && *src.Status.Version != "" && restored.Status.Versions[0].Version == *src.Status.Version {
		dst.Status.Versions = restored.Status.Versions
	}
	restoreMetaConditions(dst.Status.Conditions, restored.Status.Conditions)

	return nil
}

func (r *OCIManagedControlPlane) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedControlPlane)

	if err := Convert_v1beta3_OCIManagedControlPlane_To_v1beta1_OCIManagedControlPlane(src, r, nil); err != nil {
		return err
	}

	return utilconversion.MarshalDataUnsafeNoCopy(src, r)
}

func (src *OCIManagedControlPlaneList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta3.OCIManagedControlPlaneList)
	return Convert_v1beta1_OCIManagedControlPlaneList_To_v1beta3_OCIManagedControlPlaneList(src, dst, nil)
}

func (r *OCIManagedControlPlaneList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta3.OCIManagedControlPlaneList)
	return Convert_v1beta3_OCIManagedControlPlaneList_To_v1beta1_OCIManagedControlPlaneList(src, r, nil)
}
