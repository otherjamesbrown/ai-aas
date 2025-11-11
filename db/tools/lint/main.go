package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var basePath string
	flag.StringVar(&basePath, "path", "db/migrations", "Base path containing migration directories")
	flag.Parse()

	issues, err := RunNamingLint(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint error: %v\n", err)
		os.Exit(1)
	}

	if len(issues) > 0 {
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "[lint] %s\n", issue)
		}
		os.Exit(1)
	}
}
