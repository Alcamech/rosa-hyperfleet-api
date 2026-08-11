package featuregate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// PassthroughTarget describes a CRD file and the schema paths within it that
// contain embedded HyperShift passthrough types and should have their
// x-kubernetes-validations stripped.
type PassthroughTarget struct {
	// CRDFile is the absolute path to the CRD YAML file.
	CRDFile string
	// Paths are dot-separated schema paths relative to openAPIV3Schema
	// (e.g. "spec.hostedCluster").
	Paths []string
}

// DetectPassthroughTargets scans the Go source files in apiDir for root CRD
// types (marked +kubebuilder:object:root=true) whose Spec structs contain
// fields typed with a name ending in "Passthrough". For each such field it
// locates the corresponding CRD file in crdDir and records the schema path.
//
// CRD files are matched by the lowercase-plural of the root type name
// (e.g. Cluster → clusters → *_clusters.yaml).
func DetectPassthroughTargets(apiDir, crdDir string) ([]PassthroughTarget, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, apiDir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", apiDir, err)
	}

	// Collect all struct types by name and their position in the file.
	type structEntry struct {
		st  *ast.StructType
		pos token.Pos
	}
	structs := make(map[string]structEntry)

	// rootTypes: names of types annotated with +kubebuilder:object:root=true.
	// Detected by finding comment groups containing the marker, then mapping
	// them to the nearest following type declaration in the same file.
	rootTypes := make(map[string]bool)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			// Build a sorted list of (commentGroupEndPos, markerPresent) for this file.
			// We only care about comment groups that contain the root marker.
			var markerGroupEnds []token.Pos
			for _, cg := range file.Comments {
				if hasMarker(cg, "+kubebuilder:object:root=true") {
					markerGroupEnds = append(markerGroupEnds, cg.End())
				}
			}

			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					structs[ts.Name.Name] = structEntry{st: st, pos: gd.Pos()}

					// A type is a root type if any marker comment group in the
					// same file ends before this declaration starts. We use a
					// generous window: any marker group that ends within 50 lines
					// before the type declaration is considered associated with it.
					declLine := fset.Position(gd.Pos()).Line
					for _, end := range markerGroupEnds {
						endLine := fset.Position(end).Line
						if endLine < declLine && declLine-endLine <= 50 {
							rootTypes[ts.Name.Name] = true
							break
						}
					}
				}
			}
		}
	}

	// For each root type, inspect its Spec struct for Passthrough fields.
	type hit struct {
		plural  string // CRD plural name (e.g. "clusters")
		jsonTag string // JSON field name (e.g. "hostedCluster")
	}
	var hits []hit

	for typeName := range rootTypes {
		specName := typeName + "Spec"
		entry, ok := structs[specName]
		if !ok {
			continue
		}
		plural := strings.ToLower(typeName) + "s"
		for _, field := range entry.st.Fields.List {
			typStr := typeString(field.Type)
			if !strings.HasSuffix(typStr, "Passthrough") {
				continue
			}
			tag := jsonTag(field)
			if tag == "" || tag == "-" {
				continue
			}
			hits = append(hits, hit{plural: plural, jsonTag: tag})
		}
	}

	if len(hits) == 0 {
		return nil, nil
	}

	// Resolve each hit to an actual CRD file in crdDir.
	entries, err := os.ReadDir(crdDir)
	if err != nil {
		return nil, fmt.Errorf("reading CRD dir %s: %w", crdDir, err)
	}

	// Index CRD files by their plural suffix: "clusters" → full path.
	crdByPlural := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		// File names look like hyperfleet.io_clusters.yaml
		parts := strings.SplitN(strings.TrimSuffix(e.Name(), ".yaml"), "_", 2)
		if len(parts) == 2 {
			crdByPlural[parts[1]] = filepath.Join(crdDir, e.Name())
		}
	}

	// Group hits by CRD file.
	byFile := make(map[string][]string)
	for _, h := range hits {
		crdFile, ok := crdByPlural[h.plural]
		if !ok {
			return nil, fmt.Errorf("no CRD file found for plural %q in %s", h.plural, crdDir)
		}
		byFile[crdFile] = append(byFile[crdFile], "spec."+h.jsonTag)
	}

	var targets []PassthroughTarget
	for file, paths := range byFile {
		targets = append(targets, PassthroughTarget{CRDFile: file, Paths: paths})
	}
	return targets, nil
}

// hasMarker reports whether the comment group contains the given marker text.
func hasMarker(cg *ast.CommentGroup, marker string) bool {
	if cg == nil {
		return false
	}
	for _, c := range cg.List {
		if strings.Contains(c.Text, marker) {
			return true
		}
	}
	return false
}

// typeString returns the base type name from a field type expression,
// stripping any pointer or selector qualifier.
func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return typeString(t.X)
	case *ast.SelectorExpr:
		return typeString(t.Sel)
	case *ast.ArrayType:
		return typeString(t.Elt)
	}
	return ""
}

// jsonTag extracts the first comma-separated segment of the "json" struct tag.
func jsonTag(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	raw := strings.Trim(field.Tag.Value, "`")
	tag := reflect.StructTag(raw).Get("json")
	if tag == "" {
		return ""
	}
	return strings.SplitN(tag, ",", 2)[0]
}
