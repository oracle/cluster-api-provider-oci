package ptr

import infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"

// ToString returns string value dereferenced if the passed
// in pointer was not nil. Returns a string zero value if the
// pointer was nil.
func ToString(p *string) (v string) {
	if p == nil {
		return v
	}

	return *p
}

// StringEquals safely checks if a string pointer is non-nil and equals the given value.
// Returns false if the pointer is nil.
func StringEquals(p *string, value string) bool {
	if p == nil {
		return false
	}
	return *p == value
}

// ToBool returns bool value dereferenced if the passed
// in pointer was not nil. Returns a bool zero value (false) if the
// pointer was nil.
func ToBool(p *bool) (v bool) {
	if p == nil {
		return v
	}

	return *p
}

// ToStringSlice returns a slice of string values, that are
// dereferenced if the passed in pointer was not nil. Returns a string
// zero value if the pointer was nil.
func ToStringSlice(vs []*string) []string {
	ps := make([]string, len(vs))
	for i, v := range vs {
		ps[i] = ToString(v)
	}

	return ps
}

// ToNSGSlice returns a slice of NSG values, that are
// dereferenced if the passed in pointer was not nil. Returns an empty slice
// zero value if the pointer was nil.
func ToNSGSlice(vs []*infrastructurev1beta2.NSG) []infrastructurev1beta2.NSG {
	ps := make([]infrastructurev1beta2.NSG, len(vs))
	for i, v := range vs {
		if v != nil {
			ps[i] = *v
		}
	}

	return ps
}

// ToSubnetSlice returns a slice of Subnet values, that are
// dereferenced if the passed in pointer was not nil. Returns an empty slice
// zero value if the pointer was nil.
func ToSubnetSlice(vs []*infrastructurev1beta2.Subnet) []infrastructurev1beta2.Subnet {
	ps := make([]infrastructurev1beta2.Subnet, len(vs))
	for i, v := range vs {
		if v != nil {
			ps[i] = *v
		}
	}

	return ps
}
