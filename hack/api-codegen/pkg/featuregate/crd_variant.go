package featuregate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/openshift-online/rosa-hyperfleet-api/hack/api-codegen/pkg/registry"
)

// CRDVariantGenerator generates feature-set-specific CRD variants
type CRDVariantGenerator struct {
	fieldRegistry registry.TypedFieldRegistry
	resourceType  string
}

// NewCRDVariantGenerator creates a new CRD variant generator for a specific resource type
func NewCRDVariantGenerator(resourceType string) *CRDVariantGenerator {
	return &CRDVariantGenerator{
		fieldRegistry: registry.FieldRegistry,
		resourceType:  resourceType,
	}
}

// GenerateVariant reads a base CRD and generates a filtered variant for a feature set
func (g *CRDVariantGenerator) GenerateVariant(inputPath string, outputPath string, featureSet FeatureSet) error {
	// Read input CRD
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading CRD: %w", err)
	}

	// Parse YAML
	var crd yaml.Node
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	// Filter the CRD based on feature set
	ctx := &filterContext{
		featureSet: featureSet,
		inSchema:   false,
		fieldPath:  "",
	}
	if err := g.filterCRDNode(&crd, ctx); err != nil {
		return fmt.Errorf("filtering CRD: %w", err)
	}

	// Write to a temp file and rename on success so a failed encode
	// doesn't destroy the existing valid output file.
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), ".crd-variant-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	encoder := yaml.NewEncoder(tmp)
	encoder.SetIndent(2)
	if err := encoder.Encode(&crd); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("closing YAML encoder: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, outputPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

type filterContext struct {
	featureSet FeatureSet
	inSchema   bool   // true when inside openAPIV3Schema.properties
	fieldPath  string // current field path (e.g., "spec.tags")
}

// filterCRDNode walks the CRD YAML tree and removes fields not available in the feature set
func (g *CRDVariantGenerator) filterCRDNode(node *yaml.Node, ctx *filterContext) error {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		// Process document content
		for _, child := range node.Content {
			if err := g.filterCRDNode(child, ctx); err != nil {
				return err
			}
		}

	case yaml.MappingNode:
		// Process key-value pairs
		// YAML mappings have alternating key/value nodes
		newContent := make([]*yaml.Node, 0, len(node.Content))

		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}

			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			fieldName := keyNode.Value

			// Track when we enter the schema's properties section
			enteringSchema := !ctx.inSchema && fieldName == "properties"

			// Save old context
			oldInSchema := ctx.inSchema
			oldFieldPath := ctx.fieldPath

			// Update context for this key
			isStructuralWrapper := fieldName == "items" || fieldName == "additionalProperties"
			if enteringSchema {
				ctx.inSchema = true
			} else if ctx.inSchema && fieldName != "properties" && !isStructuralWrapper {
				// We're inside schema properties, build field path
				if ctx.fieldPath == "" {
					ctx.fieldPath = fieldName
				} else {
					ctx.fieldPath = ctx.fieldPath + "." + fieldName
				}
			}

			// Check if we should include this field
			shouldInclude := true
			if ctx.inSchema && fieldName != "properties" && ctx.fieldPath != "" {
				shouldInclude = g.shouldIncludeField(ctx.fieldPath, ctx.featureSet)
			}

			if shouldInclude {
				// Recurse into value
				if err := g.filterCRDNode(valueNode, ctx); err != nil {
					return err
				}
				newContent = append(newContent, keyNode, valueNode)
			}

			// Restore context
			ctx.inSchema = oldInSchema
			ctx.fieldPath = oldFieldPath
		}

		node.Content = newContent

	case yaml.SequenceNode:
		// Process array elements
		for _, child := range node.Content {
			if err := g.filterCRDNode(child, ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// shouldIncludeField checks if a field should be included in the given feature set
func (g *CRDVariantGenerator) shouldIncludeField(fieldPath string, featureSet FeatureSet) bool {
	// Get fields for this resource type
	fieldsForType, exists := g.fieldRegistry[g.resourceType]
	if !exists {
		// Resource type not in registry - include all fields
		return true
	}

	// Check if field is in registry
	meta, exists := fieldsForType[fieldPath]
	if !exists {
		// Field not in registry - include it (it's a structural field like "properties", "type", etc.)
		return true
	}

	if meta.Hidden {
		return false
	}

	// If field has a feature gate, check if it's enabled
	if meta.FeatureGate != "" {
		return IsGateEnabled(meta.FeatureGate, featureSet)
	}

	// No feature gate - always include
	return true
}

// GenerateAllVariants generates CRD variants for all feature sets
func (g *CRDVariantGenerator) GenerateAllVariants(inputPath string, outputDir string, baseName string) error {
	featureSets := []struct {
		set    FeatureSet
		suffix string
	}{
		{Default, "default"},
		{TechPreviewNoUpgrade, "techpreview"},
		{DevPreviewNoUpgrade, "devpreview"},
	}

	for _, fs := range featureSets {
		outputPath := fmt.Sprintf("%s/%s_%s.yaml", outputDir, baseName, fs.suffix)
		if err := g.GenerateVariant(inputPath, outputPath, fs.set); err != nil {
			return fmt.Errorf("generating %s variant: %w", fs.suffix, err)
		}
	}

	return nil
}

// WriteVariantToWriter generates a variant and writes it to a writer (useful for testing)
func (g *CRDVariantGenerator) WriteVariantToWriter(inputPath string, w io.Writer, featureSet FeatureSet) error {
	// Read input CRD
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading CRD: %w", err)
	}

	// Parse YAML
	var crd yaml.Node
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	// Filter the CRD based on feature set
	ctx := &filterContext{
		featureSet: featureSet,
		inSchema:   false,
		fieldPath:  "",
	}
	if err := g.filterCRDNode(&crd, ctx); err != nil {
		return fmt.Errorf("filtering CRD: %w", err)
	}

	// Write to writer
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(&crd); err != nil {
		return fmt.Errorf("writing YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("closing YAML encoder: %w", err)
	}

	return nil
}
