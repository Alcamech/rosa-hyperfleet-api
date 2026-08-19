package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/openshift-online/rosa-hyperfleet-api/hack/api-codegen/pkg/markers"
)

func main() {
	var (
		inputDirs  string
		outputFile string
		validate   bool
		verbose    bool
	)

	flag.StringVar(&inputDirs, "input-dirs", "", "Comma-separated list of directories to scan (required)")
	flag.StringVar(&outputFile, "output-file", "", "Output file for generated registry (required)")
	flag.BoolVar(&validate, "validate", true, "Validate that all visible fields have write-mode markers")
	flag.BoolVar(&verbose, "verbose", false, "Show detailed table of fields and their markers")
	flag.Parse()

	if inputDirs == "" || outputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	dirs := strings.Split(inputDirs, ",")
	for i := range dirs {
		dirs[i] = strings.TrimSpace(dirs[i])
	}

	// Create scanner and scan directories
	scanner, err := markers.NewScanner(dirs, verbose)
	if err != nil {
		panic(fmt.Sprintf("Error creating scanner: %v", err))
	}

	log.Printf("Scanning directories: %v", dirs)
	if err := scanner.Scan(); err != nil {
		log.Fatalf("Error scanning: %v", err)
	}

	// Count total fields across all owner types
	totalFields := 0
	for _, fields := range scanner.TypedRegistry {
		totalFields += len(fields)
	}
	log.Printf("Found %d fields with markers across %d CRD types", totalFields, len(scanner.TypedRegistry))

	// Show scanned fields if verbose
	if verbose {
		fmt.Println()
		fmt.Println("=== Scanned Fields ===")
		fmt.Println()
		printTypedRegistryTable(scanner.TypedRegistry)
		printTypedRegistryStats(scanner.TypedRegistry)
		fmt.Println()
	}

	// Validate if requested
	if validate {
		if err := scanner.TypedRegistry.Validate(); err != nil {
			log.Fatalf("Validation failed: %v", err)
		}
		log.Println("Validation passed")
	}

	// Generate registry file
	log.Printf("Generating registry: %s", outputFile)
	if err := scanner.Generate(outputFile); err != nil {
		log.Fatalf("Error generating registry: %v", err)
	}

	fmt.Printf("Successfully generated field registry at %s\n", outputFile)

	// Also generate JSON file for use by other tools
	jsonFile := strings.TrimSuffix(outputFile, ".go") + ".json"
	log.Printf("Generating JSON registry: %s", jsonFile)
	if err := scanner.GenerateJSON(jsonFile); err != nil {
		log.Fatalf("Error generating JSON registry: %v", err)
	}

	fmt.Printf("Successfully generated JSON registry at %s\n", jsonFile)

	// Show what was generated if verbose
	if verbose {
		fmt.Println()
		fmt.Println("=== Generated Registry Contents ===")
		fmt.Printf("File: %s\n", outputFile)
		fmt.Printf("Package: registry\n")
		fmt.Printf("Exported: FieldRegistry TypedFieldRegistry (map[string]map[string]FieldMeta)\n")
		fmt.Println()
		fmt.Println("The generated file contains:")
		printTypedRegistryTable(scanner.TypedRegistry)
		printTypedRegistryStats(scanner.TypedRegistry)
	}
}

// printTypedRegistryTable displays the typed field registry as a formatted table
func printTypedRegistryTable(registry markers.TypedFieldRegistry) {
	// Sort owner types
	var owners []string
	for owner := range registry {
		owners = append(owners, owner)
	}
	sort.Strings(owners)

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "OWNER TYPE\tFIELD PATH\tWRITE MODE\tFEATURE GATE\tHIDDEN")
	_, _ = fmt.Fprintln(w, "-----------\t-----------\t-----------\t-----------\t------")

	// Print each owner type and its fields
	for _, owner := range owners {
		fields := registry[owner]

		// Sort field paths
		var paths []string
		for path := range fields {
			paths = append(paths, path)
		}
		sort.Strings(paths)

		// Print each field
		for _, path := range paths {
			meta := fields[path]

			writeMode := string(meta.WriteMode)
			if writeMode == "" {
				writeMode = "-"
			}

			featureGate := meta.FeatureGate
			if featureGate == "" {
				featureGate = "-"
			}

			hidden := "no"
			if meta.Hidden {
				hidden = "yes"
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", owner, path, writeMode, featureGate, hidden)
		}
	}

	_ = w.Flush()
}

// printTypedRegistryStats displays summary statistics about the typed registry
func printTypedRegistryStats(registry markers.TypedFieldRegistry) {
	var (
		totalFields int
		mutable     int
		immutable   int
		serviceSet  int
		hidden      int
		gated       int
	)

	for _, fields := range registry {
		for _, meta := range fields {
			totalFields++
			switch meta.WriteMode {
			case markers.Mutable:
				mutable++
			case markers.Immutable:
				immutable++
			case markers.ServiceSet:
				serviceSet++
			}
			if meta.Hidden {
				hidden++
			}
			if meta.FeatureGate != "" {
				gated++
			}
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d total fields across %d CRD types\n", totalFields, len(registry))
	fmt.Printf("  Write Modes: %d mutable, %d immutable, %d service-set\n", mutable, immutable, serviceSet)
	fmt.Printf("  Visibility:  %d visible, %d hidden\n", totalFields-hidden, hidden)
	fmt.Printf("  Gating:      %d gated, %d ungated\n", gated, totalFields-gated)
}
