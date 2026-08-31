package ratelimit

import "testing"

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"exact match", "/api/v0/clusters", "/api/v0/clusters", true},
		{"single wildcard", "/api/v0/clusters/*", "/api/v0/clusters/abc123", true},
		{"multi segment wildcard", "/api/v0/clusters/*/nodepools/*", "/api/v0/clusters/abc/nodepools/np1", true},
		{"wildcard mismatch length", "/api/v0/clusters/*", "/api/v0/clusters/abc/extra", false},
		{"shorter path", "/api/v0/clusters/*", "/api/v0/clusters", false},
		{"literal mismatch", "/api/v0/clusters", "/api/v0/nodepools", false},
		{"trailing slash pattern", "/api/v0/clusters/", "/api/v0/clusters", true},
		{"trailing slash path", "/api/v0/clusters", "/api/v0/clusters/", true},
		{"both trailing slashes", "/api/v0/clusters/", "/api/v0/clusters/", true},
		{"root path", "/", "/", true},
		{"wildcard does not match empty segment", "/api/v0/*/run", "/api/v0//run", false},
		{"middle wildcard", "/api/v0/clusters/*/status", "/api/v0/clusters/abc123/status", true},
		{"middle wildcard mismatch suffix", "/api/v0/clusters/*/status", "/api/v0/clusters/abc123/delete", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPath(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
