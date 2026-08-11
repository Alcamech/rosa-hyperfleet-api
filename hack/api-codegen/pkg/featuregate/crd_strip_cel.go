package featuregate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// StripCELFromSubtrees reads a CRD YAML file, removes all x-kubernetes-validations
// keys from within each named dot-separated field path, and writes the result back
// in-place. Paths are relative to openAPIV3Schema (e.g. "spec.hostedCluster").
func StripCELFromSubtrees(crdPath string, fieldPaths []string) error {
	data, err := os.ReadFile(crdPath)
	if err != nil {
		return fmt.Errorf("reading CRD: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	for _, path := range fieldPaths {
		segments := strings.Split(path, ".")
		stripCELAtPath(&doc, segments)
	}

	tmp, err := os.CreateTemp(filepath.Dir(crdPath), ".strip-cel-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	enc := yaml.NewEncoder(tmp)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("closing encoder: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, crdPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// stripCELAtPath navigates to the subtree at the given field path segments
// (relative to openAPIV3Schema) and recursively removes x-kubernetes-validations.
// It handles both single-version and multi-version CRDs by traversing the
// spec.versions[*].schema.openAPIV3Schema prefix automatically.
func stripCELAtPath(doc *yaml.Node, segments []string) {
	// Walk every version's schema — the generator produces one version but
	// the function handles multiple to be safe.
	versions := findSchemaNodes(doc)
	for _, schema := range versions {
		target := navigateTo(schema, segments)
		if target != nil {
			stripCELRecursive(target)
		}
	}
}

// findSchemaNodes returns the openAPIV3Schema mapping node for each version.
func findSchemaNodes(doc *yaml.Node) []*yaml.Node {
	// spec.versions[*].schema.openAPIV3Schema
	spec := mappingChild(doc, "spec")
	if spec == nil {
		return nil
	}
	versionsSeq := mappingChild(spec, "versions")
	if versionsSeq == nil || versionsSeq.Kind != yaml.SequenceNode {
		return nil
	}
	var schemas []*yaml.Node
	for _, ver := range versionsSeq.Content {
		schema := mappingChild(ver, "schema")
		if schema == nil {
			continue
		}
		openAPI := mappingChild(schema, "openAPIV3Schema")
		if openAPI != nil {
			schemas = append(schemas, openAPI)
		}
	}
	return schemas
}

// navigateTo descends through "properties" wrappers following the segment path.
// Each segment steps into the "properties" map of the current node.
func navigateTo(node *yaml.Node, segments []string) *yaml.Node {
	cur := node
	for _, seg := range segments {
		props := mappingChild(cur, "properties")
		if props == nil {
			return nil
		}
		cur = mappingChild(props, seg)
		if cur == nil {
			return nil
		}
	}
	return cur
}

// stripCELRecursive removes x-kubernetes-validations from node and all descendants.
func stripCELRecursive(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		newContent := make([]*yaml.Node, 0, len(node.Content))
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}
			key := node.Content[i]
			val := node.Content[i+1]
			if key.Value == "x-kubernetes-validations" {
				continue
			}
			stripCELRecursive(val)
			newContent = append(newContent, key, val)
		}
		node.Content = newContent
		return
	}
	for _, child := range node.Content {
		stripCELRecursive(child)
	}
}

// mappingChild returns the value node for key in a YAML mapping node, or nil.
func mappingChild(node *yaml.Node, key string) *yaml.Node {
	if node == nil {
		return nil
	}
	// Unwrap document node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return mappingChild(node.Content[0], key)
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
