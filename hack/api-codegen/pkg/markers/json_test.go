package markers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTypedRegistryFromJSON_NonexistentFile(t *testing.T) {
	_, err := LoadTypedRegistryFromJSON("/nonexistent/file.json")
	if err == nil {
		t.Error("LoadTypedRegistryFromJSON() with nonexistent file should return error")
	}
}

func TestLoadTypedRegistryFromJSON_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(invalidFile, []byte("not valid json {"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err = LoadTypedRegistryFromJSON(invalidFile)
	if err == nil {
		t.Error("LoadTypedRegistryFromJSON() with invalid JSON should return error")
	}
}

func TestLoadTypedRegistryFromJSON_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "valid.json")

	// Write valid JSON (typed format with owners)
	jsonContent := `[
		{
			"fieldPath": "spec.name",
			"writeMode": "immutable",
			"hidden": false,
			"ownerType": "Cluster",
			"ownerGVK": "hyperfleet.io/v1alpha1.Cluster"
		},
		{
			"fieldPath": "spec.accountId",
			"writeMode": "service-set",
			"hidden": true,
			"ownerType": "Cluster",
			"ownerGVK": "hyperfleet.io/v1alpha1.Cluster"
		},
		{
			"fieldPath": "spec.accountId",
			"writeMode": "mutable",
			"hidden": false,
			"ownerType": "NodePool",
			"ownerGVK": "hyperfleet.io/v1alpha1.NodePool"
		}
	]`

	err := os.WriteFile(validFile, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loaded, err := LoadTypedRegistryFromJSON(validFile)
	if err != nil {
		t.Fatalf("LoadTypedRegistryFromJSON() error = %v", err)
	}

	// Check Cluster owner has 2 fields
	if clusterFields, exists := loaded["Cluster"]; !exists {
		t.Error("Cluster owner not found in loaded registry")
	} else if len(clusterFields) != 2 {
		t.Errorf("Cluster has %d fields, want 2", len(clusterFields))
	}

	// Check NodePool owner has 1 field
	if nodePoolFields, exists := loaded["NodePool"]; !exists {
		t.Error("NodePool owner not found in loaded registry")
	} else if len(nodePoolFields) != 1 {
		t.Errorf("NodePool has %d fields, want 1", len(nodePoolFields))
	}

	// Check spec.accountId differs between owners (no collision)
	clusterAccountId := loaded["Cluster"]["spec.accountId"]
	nodePoolAccountId := loaded["NodePool"]["spec.accountId"]
	if clusterAccountId.WriteMode != ServiceSet {
		t.Errorf("Cluster.spec.accountId WriteMode = %s, want service-set", clusterAccountId.WriteMode)
	}
	if nodePoolAccountId.WriteMode != Mutable {
		t.Errorf("NodePool.spec.accountId WriteMode = %s, want mutable", nodePoolAccountId.WriteMode)
	}
}

func TestLoadTypedRegistryFromJSON_EmptyArray(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.json")

	// Write empty JSON array
	err := os.WriteFile(emptyFile, []byte("[]"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loaded, err := LoadTypedRegistryFromJSON(emptyFile)
	if err != nil {
		t.Fatalf("LoadTypedRegistryFromJSON() error = %v", err)
	}

	if len(loaded) != 0 {
		t.Errorf("Expected empty registry, got %d owners", len(loaded))
	}
}

func TestLoadTypedRegistryFromJSON_WithFeatureGates(t *testing.T) {
	tmpDir := t.TempDir()
	gatedFile := filepath.Join(tmpDir, "gated.json")

	// Write JSON with feature-gated field
	jsonContent := `[
		{
			"fieldPath": "spec.etcd",
			"writeMode": "mutable",
			"featureGate": "HyperFleetEtcdConfig",
			"hidden": false,
			"ownerType": "Cluster",
			"ownerGVK": "hyperfleet.io/v1alpha1.Cluster"
		}
	]`

	err := os.WriteFile(gatedFile, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loaded, err := LoadTypedRegistryFromJSON(gatedFile)
	if err != nil {
		t.Fatalf("LoadTypedRegistryFromJSON() error = %v", err)
	}

	clusterFields, exists := loaded["Cluster"]
	if !exists {
		t.Fatal("Cluster owner not found")
	}

	if meta, exists := clusterFields["spec.etcd"]; exists {
		if meta.FeatureGate != "HyperFleetEtcdConfig" {
			t.Errorf("spec.etcd FeatureGate = %s, want HyperFleetEtcdConfig", meta.FeatureGate)
		}
	} else {
		t.Error("spec.etcd not found in Cluster fields")
	}
}
