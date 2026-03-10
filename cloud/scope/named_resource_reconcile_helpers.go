package scope

import infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"

func desiredAPIServerBackendSets(lb infrastructurev1beta2.LoadBalancer) []infrastructurev1beta2.NLBBackendSet {
	canonicalBackendSets := lb.NLBSpec.CanonicalBackendSets()
	if len(canonicalBackendSets) > 0 {
		return canonicalBackendSets
	}

	return []infrastructurev1beta2.NLBBackendSet{{Name: APIServerLBBackendSetName}}
}

func reconcileNamedResources[Actual any, Desired any](
	actual map[string]Actual,
	desired map[string]Desired,
	create func(name string, desired Desired) error,
	needsUpdate func(actual Actual, desired Desired) bool,
	update func(name string, desired Desired) error,
) error {
	for name, desiredResource := range desired {
		actualResource, exists := actual[name]
		if !exists {
			if err := create(name, desiredResource); err != nil {
				return err
			}
			continue
		}
		if !needsUpdate(actualResource, desiredResource) {
			continue
		}
		if err := update(name, desiredResource); err != nil {
			return err
		}
	}
	return nil
}

func deleteStaleNamedResources[Actual any, Desired any](
	actual map[string]Actual,
	desired map[string]Desired,
	shouldDelete func(actual Actual) bool,
	deleteFn func(name string, actual Actual) error,
) (bool, error) {
	deleted := false
	for name, actualResource := range actual {
		if _, keep := desired[name]; keep {
			continue
		}
		if shouldDelete != nil && !shouldDelete(actualResource) {
			continue
		}
		if err := deleteFn(name, actualResource); err != nil {
			return deleted, err
		}
		deleted = true
	}
	return deleted, nil
}

func desiredNameSet[T any](resources map[string]T) map[string]struct{} {
	names := make(map[string]struct{}, len(resources))
	for name := range resources {
		names[name] = struct{}{}
	}
	return names
}
