package v1beta2

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateNetworkSpec_BackendSets(t *testing.T) {
	t.Run("rejects mixed legacy and canonical fields", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{{Name: "new-set"}},
						BackendSetDetails: BackendSetDetails{
							IsFailOpen: boolPtr(true),
						},
					},
				},
			},
			NetworkSpec{},
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for mixed fields")
		}
		if !strings.Contains(errs[0].Error(), "backendSetDetails") {
			t.Fatalf("expected backendSetDetails path in error, got %q", errs[0].Error())
		}
	})

	t.Run("rejects duplicate backend set names", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{{Name: "dup"}, {Name: "dup"}},
					},
				},
			},
			NetworkSpec{},
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for duplicate names")
		}
		if !strings.Contains(errs[0].Error(), "duplicate backend set name") {
			t.Fatalf("expected duplicate-name guidance in error, got %q", errs[0].Error())
		}
	})

	t.Run("rejects invalid backend set name format", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{{Name: "bad name"}},
					},
				},
			},
			NetworkSpec{},
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for invalid name format")
		}
		if !strings.Contains(errs[0].Error(), "must match ^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$") {
			t.Fatalf("expected regex guidance in error, got %q", errs[0].Error())
		}
	})

	t.Run("accepts canonical backend sets", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{{Name: "apiserver_a"}, {Name: "apiserver-b"}},
					},
				},
			},
			NetworkSpec{},
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) != 0 {
			t.Fatalf("expected no validation errors, got %v", errs.ToAggregate())
		}
	})

	t.Run("rejects duplicate listener ports", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{
							{Name: "set-a", ListenerPort: int32Ptr(9345)},
							{Name: "set-b", ListenerPort: int32Ptr(9345)},
						},
					},
				},
			},
			NetworkSpec{},
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for duplicate listener ports")
		}
		if !strings.Contains(errs[0].Error(), "duplicate listenerPort") {
			t.Fatalf("expected duplicate listener port guidance in error, got %q", errs[0].Error())
		}
	})
}

func int32Ptr(v int32) *int32 {
	return &v
}
