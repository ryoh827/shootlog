package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ryoh827/shootlog/internal/exif"
)

func main() {
	inputPath := flag.String("input", "", "path to the image file")
	flag.Parse()

	if *inputPath == "" {
		fmt.Fprintln(os.Stderr, "--input is required")
		os.Exit(2)
	}

	summary, err := exif.ExtractSummary(exif.NewOSFileReader(), *inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to extract exif: %v\n", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode summary: %v\n", err)
		os.Exit(1)
	}
}
