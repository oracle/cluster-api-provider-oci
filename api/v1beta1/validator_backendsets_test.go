package v1beta1

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
			nil,
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
			nil,
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
			nil,
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for invalid name format")
		}
		if !strings.Contains(errs[0].Error(), "must match ^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$") {
			t.Fatalf("expected regex guidance in error, got %q", errs[0].Error())
		}
	})

	t.Run("accepts canonical backend sets with distinct effective ports", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{{Name: "apiserver_a"}, {Name: "apiserver-b", ListenerPort: int32Ptr(9345)}},
					},
				},
			},
			NetworkSpec{},
			nil,
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
			nil,
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for duplicate listener ports")
		}
		if !strings.Contains(errs[0].Error(), "duplicate effective listenerPort 9345") {
			t.Fatalf("expected duplicate effective listener port guidance in error, got %q", errs[0].Error())
		}
	})

	t.Run("rejects unsupported listener ports", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{
							{Name: "set-a", ListenerPort: int32Ptr(10250)},
						},
					},
				},
			},
			NetworkSpec{},
			nil,
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for unsupported listener port")
		}
		if !strings.Contains(errs[0].Error(), "must be one of 6443, 9345") {
			t.Fatalf("expected supported-port guidance in error, got %q", errs[0].Error())
		}
	})

	t.Run("rejects multiple omitted listener ports even when cluster API server port is unknown", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{
							{Name: "set-a"},
							{Name: "set-b"},
						},
					},
				},
			},
			NetworkSpec{},
			nil,
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for multiple omitted listener ports")
		}
		if !strings.Contains(errs[0].Error(), "multiple backend sets omit listenerPort") {
			t.Fatalf("expected omitted listener port guidance in error, got %q", errs[0].Error())
		}
	})

	t.Run("accepts omitted and explicit listener ports when cluster API server port is unknown", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{
							{Name: "set-a"},
							{Name: "set-b", ListenerPort: int32Ptr(6443)},
						},
					},
				},
			},
			NetworkSpec{},
			nil,
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) != 0 {
			t.Fatalf("expected no validation errors, got %v", errs.ToAggregate())
		}
	})

	t.Run("rejects omitted and explicit listener ports when cluster API server port is provided", func(t *testing.T) {
		apiServerPort := int32(7443)
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{
							{Name: "set-a"},
							{Name: "set-b", ListenerPort: int32Ptr(7443)},
						},
					},
				},
			},
			NetworkSpec{},
			&apiServerPort,
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for duplicate effective listener ports")
		}
		if !strings.Contains(errs[0].Error(), "duplicate effective listenerPort 7443") {
			t.Fatalf("expected duplicate effective listener port guidance in error, got %q", errs[0].Error())
		}
	})

	t.Run("accepts explicit listener port that differs from provided cluster API server port", func(t *testing.T) {
		apiServerPort := int32(7443)
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{
							{Name: "set-a"},
							{Name: "set-b", ListenerPort: int32Ptr(6443)},
						},
					},
				},
			},
			NetworkSpec{},
			&apiServerPort,
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) != 0 {
			t.Fatalf("expected no validation errors, got %v", errs.ToAggregate())
		}
	})

	t.Run("rejects unsupported listener port when cluster API server port is provided", func(t *testing.T) {
		apiServerPort := int32(7443)
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{
							{Name: "set-a"},
							{Name: "set-b", ListenerPort: int32Ptr(10250)},
						},
					},
				},
			},
			NetworkSpec{},
			&apiServerPort,
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) == 0 {
			t.Fatalf("expected validation error for unsupported listener port")
		}
		if !strings.Contains(errs[0].Error(), "must be one of 6443, 7443, 9345") {
			t.Fatalf("expected supported-port guidance in error, got %q", errs[0].Error())
		}
	})

	t.Run("accepts single omitted listener port", func(t *testing.T) {
		errs := ValidateNetworkSpec(
			OCIClusterSubnetRoles,
			NetworkSpec{
				APIServerLB: LoadBalancer{
					NLBSpec: NLBSpec{
						BackendSets: []NLBBackendSet{{Name: "set-a"}},
					},
				},
			},
			NetworkSpec{},
			nil,
			field.NewPath("spec").Child("networkSpec"),
		)

		if len(errs) != 0 {
			t.Fatalf("expected no validation errors, got %v", errs.ToAggregate())
		}
	})
}

func int32Ptr(v int32) *int32 {
	return &v
}
