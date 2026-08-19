package featuregate

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/openshift-online/rosa-hyperfleet-api/hack/api-codegen/pkg/registry"
)

func TestCRDVariantGenerator_shouldIncludeField(t *testing.T) {
	g := &CRDVariantGenerator{
		fieldRegistry: registry.TypedFieldRegistry{
			"Cluster": {
				"spec.tags": {
					FieldPath:   "spec.tags",
					WriteMode:   registry.Mutable,
					FeatureGate: "HyperFleetAutoScaling", // TechPreview gate
					OwnerType:   "Cluster",
				},
				"spec.displayName": {
					FieldPath: "spec.displayName",
					WriteMode: registry.Mutable,
					// No feature gate
					OwnerType: "Cluster",
				},
				"spec.creatorARN": {
					FieldPath: "spec.creatorARN",
					WriteMode: registry.ServiceSet,
					Hidden:    true,
					OwnerType: "Cluster",
				},
				"spec.hiddenGated": {
					FieldPath:   "spec.hiddenGated",
					WriteMode:   registry.ServiceSet,
					Hidden:      true,
					FeatureGate: "HyperFleetAutoScaling",
					OwnerType:   "Cluster",
				},
			},
		},
		resourceType: "Cluster",
	}

	tests := []struct {
		name       string
		fieldPath  string
		featureSet FeatureSet
		want       bool
	}{
		{
			name:       "gated field with Default - excluded",
			fieldPath:  "spec.tags",
			featureSet: Default,
			want:       false,
		},
		{
			name:       "gated field with TechPreview - included",
			fieldPath:  "spec.tags",
			featureSet: TechPreviewNoUpgrade,
			want:       true,
		},
		{
			name:       "non-gated field with Default - included",
			fieldPath:  "spec.displayName",
			featureSet: Default,
			want:       true,
		},
		{
			name:       "structural field (not in registry) - included",
			fieldPath:  "properties",
			featureSet: Default,
			want:       true,
		},
		{
			name:       "type field (not in registry) - included",
			fieldPath:  "type",
			featureSet: Default,
			want:       true,
		},
		{
			name:       "hidden field with Default - excluded",
			fieldPath:  "spec.creatorARN",
			featureSet: Default,
			want:       false,
		},
		{
			name:       "hidden field with TechPreview - excluded",
			fieldPath:  "spec.creatorARN",
			featureSet: TechPreviewNoUpgrade,
			want:       false,
		},
		{
			name:       "hidden and gated field with TechPreview - excluded",
			fieldPath:  "spec.hiddenGated",
			featureSet: TechPreviewNoUpgrade,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.shouldIncludeField(tt.fieldPath, tt.featureSet)
			if got != tt.want {
				t.Errorf("shouldIncludeField(%q, %v) = %v, want %v", tt.fieldPath, tt.featureSet, got, tt.want)
			}
		})
	}
}

func TestCRDVariantGenerator_GenerateVariant(t *testing.T) {
	// Create a minimal test CRD
	testCRD := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusters.hyperfleet.io
spec:
  group: hyperfleet.io
  names:
    kind: Cluster
    plural: clusters
  scope: Namespaced
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              displayName:
                type: string
                description: Display name for the cluster
              tags:
                type: object
                description: Customer tags (TechPreview feature)
                additionalProperties:
                  type: string
`

	// Write test CRD to temp file
	tmpFile, err := os.CreateTemp("", "test-crd-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(testCRD); err != nil {
		t.Fatalf("writing test CRD: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("closing temp file: %v", err)
	}

	// Create generator with test registry for Cluster
	g := &CRDVariantGenerator{
		fieldRegistry: registry.TypedFieldRegistry{
			"Cluster": {
				"spec.tags": {
					FieldPath:   "spec.tags",
					WriteMode:   registry.Mutable,
					FeatureGate: "HyperFleetAutoScaling", // TechPreview
					OwnerType:   "Cluster",
				},
				"spec.displayName": {
					FieldPath: "spec.displayName",
					WriteMode: registry.Mutable,
					OwnerType: "Cluster",
				},
			},
		},
		resourceType: "Cluster",
	}

	tests := []struct {
		name             string
		featureSet       FeatureSet
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:             "Default variant excludes gated fields",
			featureSet:       Default,
			shouldContain:    []string{"displayName"},
			shouldNotContain: []string{"tags:"},
		},
		{
			name:             "TechPreview variant includes gated fields",
			featureSet:       TechPreviewNoUpgrade,
			shouldContain:    []string{"displayName", "tags:"},
			shouldNotContain: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := g.WriteVariantToWriter(tmpFile.Name(), &buf, tt.featureSet); err != nil {
				t.Fatalf("GenerateVariant() error = %v", err)
			}

			output := buf.String()

			for _, want := range tt.shouldContain {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q but doesn't", want)
				}
			}

			for _, notWant := range tt.shouldNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("output should not contain %q but does", notWant)
				}
			}
		})
	}
}

func TestCRDVariantGenerator_GenerateAllVariants(t *testing.T) {
	// Create minimal test CRD
	testCRD := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test.hyperfleet.io
spec:
  group: hyperfleet.io
  names:
    kind: Test
    plural: tests
`

	// Write test CRD
	tmpFile, err := os.CreateTemp("", "test-crd-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(testCRD); err != nil {
		t.Fatalf("writing test CRD: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("closing temp file: %v", err)
	}

	// Create temp output directory
	tmpDir := t.TempDir()

	g := NewCRDVariantGenerator("Test")

	// Generate all variants
	if err := g.GenerateAllVariants(tmpFile.Name(), tmpDir, "test"); err != nil {
		t.Fatalf("GenerateAllVariants() error = %v", err)
	}

	// Check that all three variants were created
	expectedFiles := []string{
		"test_default.yaml",
		"test_techpreview.yaml",
		"test_devpreview.yaml",
	}

	for _, filename := range expectedFiles {
		path := tmpDir + "/" + filename
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s was not created", filename)
		}
	}
}

func TestNewCRDVariantGenerator(t *testing.T) {
	g := NewCRDVariantGenerator("Cluster")
	if g == nil {
		t.Fatal("NewCRDVariantGenerator() returned nil")
	}

	// Verify it uses the real registry
	if g.fieldRegistry == nil {
		t.Error("fieldRegistry is nil")
	}

	// Check that resourceType was set
	if g.resourceType != "Cluster" {
		t.Errorf("expected resourceType=Cluster, got %s", g.resourceType)
	}

	// Check that a known field exists in the Cluster registry
	clusterFields, exists := g.fieldRegistry["Cluster"]
	if !exists {
		t.Error("expected Cluster fields to exist in registry")
	}
	if len(clusterFields) == 0 {
		t.Error("expected Cluster fields to be non-empty")
	}
}
