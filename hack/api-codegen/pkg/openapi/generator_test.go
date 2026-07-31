package openapi

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGenerateMinimal(t *testing.T) {
	tmpFile := "/tmp/openapi-test.json"
	defer func() { _ = os.Remove(tmpFile) }()

	gen := NewGenerator(nil, tmpFile)
	gen.Title = "Test API"
	gen.Version = "v1"

	if err := gen.Generate(); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if _, err := os.Stat(tmpFile); err != nil {
		t.Fatalf("Output file not created: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	var output schemaOutput
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if output.OpenAPI != "3.0.0" {
		t.Errorf("Expected OpenAPI 3.0.0, got %s", output.OpenAPI)
	}
	if output.Info.Title != "Test API" {
		t.Errorf("Expected title 'Test API', got %s", output.Info.Title)
	}
	if output.Info.Version != "v1" {
		t.Errorf("Expected version 'v1', got %s", output.Info.Version)
	}
	if len(output.Definitions) != 0 {
		t.Errorf("Expected 0 definitions in minimal mode, got %d", len(output.Definitions))
	}
}
