package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/openshift-online/rosa-hyperfleet-api/hack/api-codegen/pkg/openapi"
)

func main() {
	var (
		inputDirs    string
		inputPackage string
		outputFile   string
		title        string
		version      string
	)

	flag.StringVar(&inputDirs, "input-dirs", "", "Comma-separated list of directories to scan for Go types")
	flag.StringVar(&inputPackage, "input-package", "", "Go import path to load (e.g., ./api/v1alpha1)")
	flag.StringVar(&outputFile, "output-file", "", "Output file for OpenAPI schema (required)")
	flag.StringVar(&title, "title", "HyperFleet API", "API title")
	flag.StringVar(&version, "version", "v1alpha1", "API version")
	flag.Parse()

	if outputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	gen := openapi.NewGenerator(nil, outputFile)
	gen.Title = title
	gen.Version = version

	if inputPackage != "" {
		gen.InputPackages = []string{inputPackage}
	}
	if inputDirs != "" {
		for _, dir := range strings.Split(inputDirs, ",") {
			gen.InputDirs = append(gen.InputDirs, strings.TrimSpace(dir))
		}
	}

	log.Printf("Generating OpenAPI v3 schema: %s %s", title, version)

	if err := gen.Generate(); err != nil {
		log.Fatalf("Failed to generate OpenAPI schema: %v", err)
	}

	fmt.Printf("Successfully generated OpenAPI schema at %s\n", outputFile)
}
