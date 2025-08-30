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
