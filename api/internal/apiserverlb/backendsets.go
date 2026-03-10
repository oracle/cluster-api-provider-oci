package apiserverlb

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oracle/cluster-api-provider-oci/cloud/ociutil"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const BackendSetNameRegex = "^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$"

type BackendSet struct {
	Name         string
	ListenerPort *int32
}

func CanonicalBackendSets[T any](configured []T, synthesize func() T) []T {
	if len(configured) > 0 {
		return append([]T(nil), configured...)
	}
	return []T{synthesize()}
}

func ValidateBackendSets(backendSets []BackendSet, legacyConfigured bool, apiServerPort *int32, supportedSecondaryListenerPort int32, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(backendSets) > 0 && legacyConfigured {
		allErrs = append(allErrs, field.Forbidden(
			fldPath.Child("backendSetDetails"),
			"cannot be set when backendSets is configured; move legacy backendSetDetails into backendSets",
		))
	}

	nameRegex := regexp.MustCompile(BackendSetNameRegex)
	seen := map[string]int{}
	for i, backendSet := range backendSets {
		namePath := fldPath.Child("backendSets").Index(i).Child("name")
		name := strings.TrimSpace(backendSet.Name)
		if name == "" {
			allErrs = append(allErrs, field.Required(namePath, "must not be empty"))
			continue
		}
		if !nameRegex.MatchString(name) {
			allErrs = append(allErrs, field.Invalid(
				namePath,
				backendSet.Name,
				"must match ^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$ (1-32 chars; alphanumeric, '-', '_')",
			))
		}
		if firstIdx, ok := seen[name]; ok {
			allErrs = append(allErrs, field.Invalid(
				namePath,
				backendSet.Name,
				fmt.Sprintf("duplicate backend set name, already used at backendSets[%d].name", firstIdx),
			))
			continue
		}
		seen[name] = i
	}

	usedPorts := map[int32]int{}
	omittedListenerPortIndex := -1
	for i, backendSet := range backendSets {
		portPath := fldPath.Child("backendSets").Index(i).Child("listenerPort")
		if backendSet.ListenerPort != nil {
			port := *backendSet.ListenerPort
			if port < 1 || port > 65535 {
				allErrs = append(allErrs, field.Invalid(portPath, port, "must be between 1 and 65535"))
				continue
			}
			if !IsSupportedAPIServerListenerPort(port, apiServerPort, supportedSecondaryListenerPort) {
				allErrs = append(allErrs, field.Invalid(
					portPath,
					port,
					fmt.Sprintf("must be one of %s to avoid exposing arbitrary control-plane ports", SupportedAPIServerListenerPortsString(apiServerPort, supportedSecondaryListenerPort)),
				))
				continue
			}
		}

		port, known := EffectiveAPIServerListenerPort(backendSet.ListenerPort, apiServerPort)
		if !known {
			if omittedListenerPortIndex >= 0 {
				allErrs = append(allErrs, field.Invalid(
					portPath,
					backendSet.ListenerPort,
					fmt.Sprintf("multiple backend sets omit listenerPort, already omitted at backendSets[%d].listenerPort", omittedListenerPortIndex),
				))
				continue
			}
			omittedListenerPortIndex = i
			continue
		}

		if firstIdx, ok := usedPorts[port]; ok {
			allErrs = append(allErrs, field.Invalid(
				portPath,
				backendSet.ListenerPort,
				fmt.Sprintf("duplicate effective listenerPort %d, already used at backendSets[%d].listenerPort", port, firstIdx),
			))
			continue
		}
		usedPorts[port] = i
	}

	return allErrs
}

func EffectiveAPIServerListenerPort(listenerPort *int32, apiServerPort *int32) (int32, bool) {
	if listenerPort != nil {
		return *listenerPort, true
	}
	if apiServerPort == nil {
		return 0, false
	}
	return *apiServerPort, true
}

func IsSupportedAPIServerListenerPort(port int32, apiServerPort *int32, supportedSecondaryListenerPort int32) bool {
	for _, allowedPort := range SupportedAPIServerListenerPorts(apiServerPort, supportedSecondaryListenerPort) {
		if port == allowedPort {
			return true
		}
	}
	return false
}

func SupportedAPIServerListenerPorts(apiServerPort *int32, supportedSecondaryListenerPort int32) []int32 {
	ports := []int32{ociutil.DefaultAPIServerPort}
	if apiServerPort != nil && *apiServerPort != ociutil.DefaultAPIServerPort {
		ports = append(ports, *apiServerPort)
	}
	if supportedSecondaryListenerPort != ociutil.DefaultAPIServerPort {
		ports = append(ports, supportedSecondaryListenerPort)
	}
	return ports
}

func SupportedAPIServerListenerPortsString(apiServerPort *int32, supportedSecondaryListenerPort int32) string {
	values := SupportedAPIServerListenerPorts(apiServerPort, supportedSecondaryListenerPort)
	parts := make([]string, 0, len(values))
	seen := map[int32]struct{}{}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		parts = append(parts, fmt.Sprintf("%d", value))
	}
	return strings.Join(parts, ", ")
}
