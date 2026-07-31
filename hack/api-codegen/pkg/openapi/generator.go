package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crdgen "sigs.k8s.io/controller-tools/pkg/crd"
	crdmarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"

	"github.com/openshift-online/rosa-hyperfleet-api/hack/api-codegen/pkg/registry"
)

// schemaOutput is the top-level JSON structure written to the output file.
type schemaOutput struct {
	OpenAPI     string                                     `json:"openapi"`
	Info        schemaInfo                                 `json:"info"`
	Definitions map[string]apiextensionsv1.JSONSchemaProps `json:"definitions"`
}

type schemaInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// Generate creates an OpenAPI v3 schema from Go types using controller-tools.
func (g *Generator) Generate() error {
	if len(g.InputDirs) == 0 && len(g.InputPackages) == 0 {
		return g.generateMinimal()
	}

	roots, err := g.loadPackages()
	if err != nil {
		return fmt.Errorf("loading packages: %w", err)
	}

	definitions, err := g.generateDefinitions(roots)
	if err != nil {
		return fmt.Errorf("generating definitions: %w", err)
	}

	// Filter hidden fields using the registry
	filterHiddenFields(definitions)

	output := schemaOutput{
		OpenAPI: "3.0.0",
		Info: schemaInfo{
			Title:   g.Title,
			Version: g.Version,
			Description: fmt.Sprintf(
				"OpenAPI schema for %s generated from Go types with controller-tools\n\n"+
					"Fields marked with +k8s:openapi-gen=false are excluded from this schema.",
				g.Title,
			),
		},
		Definitions: definitions,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling schema: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(g.OutputFile), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	return os.WriteFile(g.OutputFile, data, 0644)
}

// generateMinimal creates a minimal schema with no definitions.
func (g *Generator) generateMinimal() error {
	output := schemaOutput{
		OpenAPI: "3.0.0",
		Info: schemaInfo{
			Title:       g.Title,
			Version:     g.Version,
			Description: "OpenAPI schema for " + g.Title + " (minimal)",
		},
		Definitions: make(map[string]apiextensionsv1.JSONSchemaProps),
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling schema: %w", err)
	}

	return os.WriteFile(g.OutputFile, data, 0644)
}

// loadPackages loads Go packages from input directories or import paths.
func (g *Generator) loadPackages() ([]*loader.Package, error) {
	var loadPaths []string
	loadPaths = append(loadPaths, g.InputDirs...)
	loadPaths = append(loadPaths, g.InputPackages...)

	return loader.LoadRoots(loadPaths...)
}

// generateDefinitions uses controller-tools to generate OpenAPI schemas for
// all exported struct types in the loaded packages.
func (g *Generator) generateDefinitions(roots []*loader.Package) (map[string]apiextensionsv1.JSONSchemaProps, error) {
	reg := &markers.Registry{}
	if err := crdmarkers.Register(reg); err != nil {
		return nil, fmt.Errorf("registering markers: %w", err)
	}

	collector := &markers.Collector{Registry: reg}
	checker := &loader.TypeChecker{}

	parser := &crdgen.Parser{
		Collector: collector,
		Checker:   checker,
	}
	crdgen.AddKnownTypes(parser)

	for _, root := range roots {
		parser.NeedPackage(root)
	}

	// Collect all type identifiers, then generate flattened schemas
	var typeIdents []crdgen.TypeIdent
	for ident := range parser.Types {
		typeIdents = append(typeIdents, ident)
	}
	sort.Slice(typeIdents, func(i, j int) bool {
		return typeIdents[i].Name < typeIdents[j].Name
	})

	for _, ident := range typeIdents {
		parser.NeedFlattenedSchemaFor(ident)
	}

	definitions := make(map[string]apiextensionsv1.JSONSchemaProps)
	for ident, schema := range parser.FlattenedSchemata {
		definitions[ident.Name] = schema
	}

	return definitions, nil
}

// filterHiddenFields removes fields marked as hidden in the registry from
// all schema definitions. A field is hidden when its FieldRegistry entry has
// Hidden == true (i.e., +k8s:openapi-gen=false).
func filterHiddenFields(definitions map[string]apiextensionsv1.JSONSchemaProps) {
	hiddenPaths := make(map[string]bool)
	for path, meta := range registry.FieldRegistry {
		if meta.Hidden {
			hiddenPaths[path] = true
		}
	}

	if len(hiddenPaths) == 0 {
		return
	}

	for typeName, schema := range definitions {
		pruned := pruneHiddenProperties(&schema, typeName, hiddenPaths)
		definitions[typeName] = *pruned
	}
}

// pruneHiddenProperties recursively removes properties whose registry path
// is marked hidden. pathPrefix is the dotted field path so far (e.g., "spec").
func pruneHiddenProperties(schema *apiextensionsv1.JSONSchemaProps, pathPrefix string, hidden map[string]bool) *apiextensionsv1.JSONSchemaProps {
	if schema == nil || schema.Properties == nil {
		return schema
	}

	for propName, propSchema := range schema.Properties {
		fieldPath := pathPrefix + "." + propName
		if hidden[fieldPath] {
			delete(schema.Properties, propName)
			// Also remove from required list
			removeRequired(schema, propName)
			continue
		}
		pruned := pruneHiddenProperties(&propSchema, fieldPath, hidden)
		schema.Properties[propName] = *pruned
	}

	return schema
}

// removeRequired removes a field name from a schema's Required slice.
func removeRequired(schema *apiextensionsv1.JSONSchemaProps, field string) {
	for i, r := range schema.Required {
		if r == field {
			schema.Required = append(schema.Required[:i], schema.Required[i+1:]...)
			return
		}
	}
}
