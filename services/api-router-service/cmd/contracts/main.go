// Command contracts provides a CLI tool for contract generation and validation.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/pkg/contracts"
)

func main() {
	var (
		validateFlag = flag.Bool("validate", false, "Validate OpenAPI specification")
		generateFlag  = flag.Bool("generate", false, "Generate Go types from OpenAPI specification")
		specPath      = flag.String("spec", "", "Path to OpenAPI specification (default: auto-detect)")
		outputPath    = flag.String("output", "", "Path to output file (default: pkg/contracts/generated.go)")
		packageName   = flag.String("package", "contracts", "Package name for generated code")
	)
	flag.Parse()

	if !*validateFlag && !*generateFlag {
		fmt.Fprintf(os.Stderr, "Usage: %s [-validate] [-generate] [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *validateFlag {
		spec := *specPath
		if spec == "" {
			spec = contracts.GetOpenAPISpecPath()
		}

		fmt.Printf("Validating OpenAPI specification: %s\n", spec)
		if err := contracts.ValidateOpenAPI(spec); err != nil {
			fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ OpenAPI specification is valid")
	}

	if *generateFlag {
		opts := contracts.GenerateOptions{
			OpenAPISpecPath: *specPath,
			OutputPath:      *outputPath,
			PackageName:     *packageName,
			GenerateTypes:   true,
		}

		if opts.OpenAPISpecPath == "" {
			opts.OpenAPISpecPath = contracts.GetOpenAPISpecPath()
		}

		fmt.Printf("Generating Go types from: %s\n", opts.OpenAPISpecPath)
		if err := contracts.GenerateGoTypes(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Generation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Go types generated successfully\n")
	}
}

