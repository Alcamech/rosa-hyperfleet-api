package handlers

import "testing"

func TestRedact(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"a", "*"},
		{"ab", "*b"},
		{"abcd", "**cd"},
		{"123456789012", "******789012"},
		{"odd", "**d"},
		{"aé", "*é"},
	}

	for _, tt := range tests {
		if got := redact(tt.input); got != tt.want {
			t.Errorf("redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
