package ptr

import (
	"testing"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
)

func TestToString(t *testing.T) {
	tests := []struct {
		name  string
		input *string
		want  string
	}{
		{
			name:  "nil pointer",
			input: nil,
			want:  "",
		},
		{
			name:  "non-nil pointer",
			input: func() *string { s := "test"; return &s }(),
			want:  "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToString(tt.input); got != tt.want {
				t.Errorf("ToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringEquals(t *testing.T) {
	tests := []struct {
		name  string
		input *string
		value string
		want  bool
	}{
		{
			name:  "nil pointer",
			input: nil,
			value: "test",
			want:  false,
		},
		{
			name:  "non-nil pointer equals value",
			input: func() *string { s := "test"; return &s }(),
			value: "test",
			want:  true,
		},
		{
			name:  "non-nil pointer does not equal value",
			input: func() *string { s := "test"; return &s }(),
			value: "different",
			want:  false,
		},
		{
			name:  "non-nil pointer with empty string equals empty value",
			input: func() *string { s := ""; return &s }(),
			value: "",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringEquals(tt.input, tt.value); got != tt.want {
				t.Errorf("StringEquals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		name  string
		input *bool
		want  bool
	}{
		{
			name:  "nil pointer",
			input: nil,
			want:  false,
		},
		{
			name:  "pointer to true",
			input: func() *bool { b := true; return &b }(),
			want:  true,
		},
		{
			name:  "pointer to false",
			input: func() *bool { b := false; return &b }(),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToBool(tt.input); got != tt.want {
				t.Errorf("ToBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		input []*string
		want  []string
	}{
		{
			name:  "nil slice",
			input: nil,
			want:  []string{},
		},
		{
			name:  "empty slice",
			input: []*string{},
			want:  []string{},
		},
		{
			name:  "slice with nil elements",
			input: []*string{nil, nil},
			want:  []string{"", ""},
		},
		{
			name:  "slice with non-nil elements",
			input: []*string{func() *string { s := "test1"; return &s }(), func() *string { s := "test2"; return &s }()},
			want:  []string{"test1", "test2"},
		},
		{
			name:  "slice with mixed nil and non-nil elements",
			input: []*string{nil, func() *string { s := "test"; return &s }(), nil},
			want:  []string{"", "test", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToStringSlice(tt.input); len(got) != len(tt.want) || (len(got) > 0 && got[0] != tt.want[0]) {
				t.Errorf("ToStringSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToNSGSlice(t *testing.T) {
	tests := []struct {
		name  string
		input []*infrastructurev1beta2.NSG
		want  []infrastructurev1beta2.NSG
	}{
		{
			name:  "nil slice",
			input: nil,
			want:  []infrastructurev1beta2.NSG{},
		},
		{
			name:  "empty slice",
			input: []*infrastructurev1beta2.NSG{},
			want:  []infrastructurev1beta2.NSG{},
		},
		{
			name:  "slice with nil elements",
			input: []*infrastructurev1beta2.NSG{nil, nil},
			want:  []infrastructurev1beta2.NSG{{}, {}},
		},
		{
			name:  "slice with non-nil elements",
			input: []*infrastructurev1beta2.NSG{&infrastructurev1beta2.NSG{}, &infrastructurev1beta2.NSG{}},
			want:  []infrastructurev1beta2.NSG{{}, {}},
		},
		{
			name:  "slice with mixed nil and non-nil elements",
			input: []*infrastructurev1beta2.NSG{nil, &infrastructurev1beta2.NSG{}, nil},
			want:  []infrastructurev1beta2.NSG{{}, {}, {}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToNSGSlice(tt.input); len(got) != len(tt.want) {
				t.Errorf("ToNSGSlice() length = %v, want %v", len(got), len(tt.want))
			}
		})
	}
}

func TestToSubnetSlice(t *testing.T) {
	tests := []struct {
		name  string
		input []*infrastructurev1beta2.Subnet
		want  []infrastructurev1beta2.Subnet
	}{
		{
			name:  "nil slice",
			input: nil,
			want:  []infrastructurev1beta2.Subnet{},
		},
		{
			name:  "empty slice",
			input: []*infrastructurev1beta2.Subnet{},
			want:  []infrastructurev1beta2.Subnet{},
		},
		{
			name:  "slice with nil elements",
			input: []*infrastructurev1beta2.Subnet{nil, nil},
			want:  []infrastructurev1beta2.Subnet{{}, {}},
		},
		{
			name:  "slice with non-nil elements",
			input: []*infrastructurev1beta2.Subnet{&infrastructurev1beta2.Subnet{}, &infrastructurev1beta2.Subnet{}},
			want:  []infrastructurev1beta2.Subnet{{}, {}},
		},
		{
			name:  "slice with mixed nil and non-nil elements",
			input: []*infrastructurev1beta2.Subnet{nil, &infrastructurev1beta2.Subnet{}, nil},
			want:  []infrastructurev1beta2.Subnet{{}, {}, {}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToSubnetSlice(tt.input); len(got) != len(tt.want) {
				t.Errorf("ToSubnetlice() length = %v, want %v", len(got), len(tt.want))
			}
		})
	}
}
