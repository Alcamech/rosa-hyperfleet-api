package validation

import "testing"

func TestHasJSONField(t *testing.T) {
	object := map[string]any{
		"spec": map[string]any{
			"empty": "",
			"null":  nil,
			"zero":  float64(0),
			"false": false,
			"nested": map[string]any{
				"value": "present",
			},
		},
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "deeply nested value", path: "spec.nested.value", want: true},
		{name: "empty string", path: "spec.empty", want: true},
		{name: "null", path: "spec.null", want: true},
		{name: "zero", path: "spec.zero", want: true},
		{name: "false", path: "spec.false", want: true},
		{name: "omitted", path: "spec.omitted", want: false},
		{name: "below null", path: "spec.null.value", want: false},
		{name: "empty path", path: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasJSONField(object, tt.path); got != tt.want {
				t.Errorf("HasJSONField(%q) = %t, want %t", tt.path, got, tt.want)
			}
		})
	}
}

func TestHasJSONFieldMatchesCaseInsensitiveNames(t *testing.T) {
	object := map[string]any{
		"SPEC": map[string]any{
			"EC2InstanceMetadataHttpTokens": nil,
		},
	}

	if !HasJSONField(object, "spec.ec2InstanceMetadataHttpTokens") {
		t.Fatal("HasJSONField should match field names case-insensitively")
	}
}
