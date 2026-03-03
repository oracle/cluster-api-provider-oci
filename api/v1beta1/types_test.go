package v1beta1

import (
	"encoding/json"
	"testing"
)

func TestNLBSpecCanonicalBackendSets_Precedence(t *testing.T) {
	t.Run("uses backendSets when present", func(t *testing.T) {
		spec := NLBSpec{
			BackendSets: []NLBBackendSet{
				{Name: "new-set"},
			},
			BackendSetDetails: BackendSetDetails{
				IsFailOpen: boolPtr(true),
			},
		}

		got := spec.CanonicalBackendSets()
		if len(got) != 1 || got[0].Name != "new-set" {
			t.Fatalf("expected canonical backendSets to be used, got %#v", got)
		}
	})

	t.Run("falls back to legacy backendSetDetails", func(t *testing.T) {
		spec := NLBSpec{
			BackendSetDetails: BackendSetDetails{
				IsFailOpen: boolPtr(true),
			},
		}

		got := spec.CanonicalBackendSets()
		if len(got) != 1 {
			t.Fatalf("expected one synthesized backend set, got %#v", got)
		}
		if got[0].Name != APIServerLBBackendSetName {
			t.Fatalf("expected synthesized backend set name %q, got %q", APIServerLBBackendSetName, got[0].Name)
		}
		if got[0].BackendSetDetails.IsFailOpen == nil || *got[0].BackendSetDetails.IsFailOpen != true {
			t.Fatalf("expected legacy backend set details to be preserved, got %#v", got[0].BackendSetDetails)
		}
	})
}

func TestNLBSpecLegacyDecode_ToCanonical(t *testing.T) {
	raw := []byte(`{"backendSetDetails":{"isFailOpen":true}}`)
	var spec NLBSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	got := spec.CanonicalBackendSets()
	if len(got) != 1 || got[0].Name != APIServerLBBackendSetName {
		t.Fatalf("expected legacy decode to map to one canonical backend set, got %#v", got)
	}
}

func TestNLBSpecDeepCopy_NoSharedPointers(t *testing.T) {
	spec := &NLBSpec{
		BackendSets: []NLBBackendSet{
			{
				Name: "set-a",
				BackendSetDetails: BackendSetDetails{
					IsFailOpen: boolPtr(true),
				},
			},
		},
	}

	cp := spec.DeepCopy()
	*cp.BackendSets[0].BackendSetDetails.IsFailOpen = false

	if spec.BackendSets[0].BackendSetDetails.IsFailOpen == nil || *spec.BackendSets[0].BackendSetDetails.IsFailOpen != true {
		t.Fatalf("expected deepcopy to isolate nested pointers, original=%#v", spec.BackendSets[0].BackendSetDetails)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
