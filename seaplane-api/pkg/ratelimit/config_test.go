package ratelimit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limits.yaml")
	data := `
enabled: true
exemptAccounts:
  - "111111111111"
  - "222222222222"
default:
  rate: 100
  burst: 200
routes:
  - path: "/api/v0/clusters"
    method: POST
    rate: 5
    burst: 10
  - path: "/api/v0/clusters/*"
    method: GET
    rate: 50
    burst: 100
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}
	if cfg.Default.Rate != 100 {
		t.Errorf("expected default rate=100, got %d", cfg.Default.Rate)
	}
	if cfg.Default.Burst != 200 {
		t.Errorf("expected default burst=200, got %d", cfg.Default.Burst)
	}
	if len(cfg.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(cfg.Routes))
	}
	if cfg.Routes[0].Path != "/api/v0/clusters" {
		t.Errorf("expected route 0 path=/api/v0/clusters, got %s", cfg.Routes[0].Path)
	}
	if cfg.Routes[0].Method != "POST" {
		t.Errorf("expected route 0 method=POST, got %s", cfg.Routes[0].Method)
	}
	if cfg.Routes[0].Rate != 5 {
		t.Errorf("expected route 0 rate=5, got %d", cfg.Routes[0].Rate)
	}
	if len(cfg.ExemptAccounts) != 2 {
		t.Errorf("expected 2 exempt accounts, got %d", len(cfg.ExemptAccounts))
	}
	if len(cfg.exemptSet) != 2 {
		t.Errorf("expected exemptSet size=2, got %d", len(cfg.exemptSet))
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limits.yaml")
	data := `enabled: true`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Default.Rate != 100 {
		t.Errorf("expected default rate=100, got %d", cfg.Default.Rate)
	}
	if cfg.Default.Burst != 200 {
		t.Errorf("expected default burst=200, got %d", cfg.Default.Burst)
	}
	if cfg.Default.Window != 1 {
		t.Errorf("expected default window=1, got %d", cfg.Default.Window)
	}
	if len(cfg.Routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(cfg.Routes))
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadConfig_RouteOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limits.yaml")
	data := `
enabled: true
default:
  rate: 100
  burst: 200
routes:
  - path: "/api/v0/clusters"
    method: POST
    rate: 5
    burst: 10
  - path: "/api/v0/clusters/*"
    method: PATCH
    rate: 10
    burst: 20
  - path: "/api/v0/clusters/*/nodepools"
    method: POST
    rate: 20
    burst: 30
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(cfg.Routes))
	}

	expected := []struct {
		path   string
		method string
		rate   int
		burst  int
	}{
		{"/api/v0/clusters", "POST", 5, 10},
		{"/api/v0/clusters/*", "PATCH", 10, 20},
		{"/api/v0/clusters/*/nodepools", "POST", 20, 30},
	}

	for i, exp := range expected {
		r := cfg.Routes[i]
		if r.Path != exp.path || r.Method != exp.method || r.Rate != exp.rate || r.Burst != exp.burst {
			t.Errorf("route %d: expected %+v, got path=%s method=%s rate=%d burst=%d",
				i, exp, r.Path, r.Method, r.Rate, r.Burst)
		}
	}
}

func TestLoadConfig_RouteWithZeroRate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limits.yaml")
	data := `
enabled: true
routes:
  - path: "/api/v0/clusters"
    method: POST
    rate: 0
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for route with rate=0")
	}
}

func TestLoadConfig_DefaultBurstLessThanRate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limits.yaml")
	data := `
enabled: true
default:
  rate: 100
  burst: 50
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Default.Burst != 50 {
		t.Errorf("expected burst=50 preserved, got %d", cfg.Default.Burst)
	}
}

func TestLoadConfig_RouteBurstLessThanRate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limits.yaml")
	data := `
enabled: true
routes:
  - path: "/api/v0/clusters"
    method: POST
    rate: 10
    burst: 5
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Routes[0].Burst != 5 {
		t.Errorf("expected burst=5 preserved, got %d", cfg.Routes[0].Burst)
	}
}

func TestLoadConfig_WindowInheritsFromDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limits.yaml")
	data := `
enabled: true
default:
  rate: 100
  burst: 200
  window: 120
routes:
  - path: "/api/v0/clusters"
    method: POST
    rate: 5
    burst: 10
  - path: "/api/v0/clusters"
    method: GET
    rate: 50
    burst: 100
    window: 30
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Default.Window != 120 {
		t.Errorf("expected default window=120, got %d", cfg.Default.Window)
	}
	if cfg.Routes[0].Window != 120 {
		t.Errorf("expected POST route to inherit default window=120, got %d", cfg.Routes[0].Window)
	}
	if cfg.Routes[1].Window != 30 {
		t.Errorf("expected GET route to keep explicit window=30, got %d", cfg.Routes[1].Window)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig(10, 5, 30)

	if cfg.Default.Rate != 10 {
		t.Errorf("expected rate=10, got %d", cfg.Default.Rate)
	}
	if cfg.Default.Burst != 5 {
		t.Errorf("expected burst=5, got %d", cfg.Default.Burst)
	}
	if cfg.Default.Window != 30 {
		t.Errorf("expected window=30, got %d", cfg.Default.Window)
	}
	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}
	if len(cfg.Routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(cfg.Routes))
	}
}

func TestNewDefaultConfig_FallsBackOnZeroValues(t *testing.T) {
	cfg := NewDefaultConfig(0, 0, 0)

	if cfg.Default.Rate != 100 {
		t.Errorf("expected fallback rate=100, got %d", cfg.Default.Rate)
	}
	if cfg.Default.Burst != 200 {
		t.Errorf("expected fallback burst=200, got %d", cfg.Default.Burst)
	}
	if cfg.Default.Window != 1 {
		t.Errorf("expected fallback window=1, got %d", cfg.Default.Window)
	}
}

func TestConfig_IsExempt(t *testing.T) {
	tests := []struct {
		name      string
		exempt    []string
		accountID string
		expected  bool
	}{
		{
			name:      "account in exempt list",
			exempt:    []string{"111111111111", "222222222222"},
			accountID: "111111111111",
			expected:  true,
		},
		{
			name:      "account not in exempt list",
			exempt:    []string{"111111111111", "222222222222"},
			accountID: "333333333333",
			expected:  false,
		},
		{
			name:      "empty account ID",
			exempt:    []string{"111111111111"},
			accountID: "",
			expected:  false,
		},
		{
			name:      "empty exempt list",
			exempt:    []string{},
			accountID: "111111111111",
			expected:  false,
		},
		{
			name:      "nil exempt list",
			exempt:    nil,
			accountID: "111111111111",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ExemptAccounts: tt.exempt,
				exemptSet:      make(map[string]struct{}, len(tt.exempt)),
			}
			for _, acc := range tt.exempt {
				cfg.exemptSet[acc] = struct{}{}
			}

			got := cfg.isExempt(tt.accountID)
			if got != tt.expected {
				t.Errorf("isExempt(%q) = %v, want %v", tt.accountID, got, tt.expected)
			}
		})
	}
}
