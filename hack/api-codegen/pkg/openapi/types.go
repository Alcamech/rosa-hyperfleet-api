package openapi

// Generator generates OpenAPI v3 schemas from Go types using controller-tools.
type Generator struct {
	// InputPackages are Go import paths to scan (e.g., "./api/v1alpha1").
	InputPackages []string

	// InputDirs are filesystem directories to scan (legacy, converted to packages).
	InputDirs []string

	// OutputFile is where to write the OpenAPI schema JSON.
	OutputFile string

	// Title is the API title.
	Title string

	// Version is the API version.
	Version string
}

// NewGenerator creates a new OpenAPI generator.
func NewGenerator(inputDirs []string, outputFile string) *Generator {
	return &Generator{
		InputDirs:  inputDirs,
		OutputFile: outputFile,
		Title:      "HyperFleet API",
		Version:    "v1alpha1",
	}
}
