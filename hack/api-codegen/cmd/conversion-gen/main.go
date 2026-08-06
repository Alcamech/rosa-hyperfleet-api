package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift-online/rosa-hyperfleet-api/hack/api-codegen/pkg/conversion"
)

func main() {
	var (
		apiVersion    string
		crdPackage    string
		inputDirs     string
		outputDir     string
		outputPackage string
		restOutputDir string
		restPackage   string
	)

	flag.StringVar(&apiVersion, "api-version", "v1alpha1", "API version to generate for")
	flag.StringVar(&crdPackage, "crd-package", "", "Import path to CRD types (required)")
	flag.StringVar(&inputDirs, "input-dirs", "", "Comma-separated list of directories containing CRD source files (required)")
	flag.StringVar(&outputDir, "output-dir", "", "Output directory for generated code (required)")
	flag.StringVar(&outputPackage, "output-package", "", "Import path for the parent package of the output directory (overrides default)")
	flag.StringVar(&restOutputDir, "rest-output-dir", "", "Output directory for REST types (defaults to <output-dir>/rest)")
	flag.StringVar(&restPackage, "rest-package", "", "Import path for the REST types package (defaults to <output-package>/<base>/rest)")
	flag.Parse()

	if crdPackage == "" || inputDirs == "" || outputDir == "" {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\nError: Missing required flags\n\n")
		fmt.Fprintf(os.Stderr, "Example usage:\n")
		fmt.Fprintf(os.Stderr, "  %s \\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "    --api-version=v1alpha1 \\\n")
		fmt.Fprintf(os.Stderr, "    --crd-package=github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1 \\\n")
		fmt.Fprintf(os.Stderr, "    --input-dirs=./api/v1alpha1 \\\n")
		fmt.Fprintf(os.Stderr, "    --output-dir=./platform-api/pkg/conversion/v1alpha1 \\\n")
		fmt.Fprintf(os.Stderr, "    --output-package=github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/conversion\n")
		os.Exit(1)
	}

	// Split input directories
	dirs := strings.Split(inputDirs, ",")
	for i, dir := range dirs {
		// Convert to absolute path
		absDir, err := filepath.Abs(dir)
		if err != nil {
			log.Fatalf("Failed to resolve directory %s: %v", dir, err)
		}
		dirs[i] = absDir
	}

	// Create generator
	gen := conversion.NewGenerator(apiVersion, crdPackage, dirs, outputDir)
	if outputPackage != "" {
		gen.OutputPackage = outputPackage
	}
	if restOutputDir != "" {
		gen.RESTOutputDir = restOutputDir
	}
	if restPackage != "" {
		gen.RESTImportPath = restPackage
	}

	log.Printf("Conversion code generator")
	log.Printf("  API Version: %s", apiVersion)
	log.Printf("  CRD Package: %s", crdPackage)
	log.Printf("  Input Dirs: %s", strings.Join(dirs, ", "))
	log.Printf("  Output Dir: %s", outputDir)
	if outputPackage != "" {
		log.Printf("  Output Pkg: %s", outputPackage)
	}
	if restOutputDir != "" {
		log.Printf("  REST Dir:   %s", restOutputDir)
	}
	if restPackage != "" {
		log.Printf("  REST Pkg:   %s", restPackage)
	}
	log.Println()

	// Generate
	if err := gen.Generate(); err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	restTarget := gen.RESTOutputDir
	if restTarget == "" {
		restTarget = outputDir + "/rest"
	}
	log.Println("✓ Successfully generated:")
	log.Printf("  - REST types (%s)", restTarget)
	log.Println("  - ServiceSetFields (../types.go)")
	log.Println("  - Conversion functions (cluster.go, nodepool.go)")
}
