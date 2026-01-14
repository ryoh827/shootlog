package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"shootlog/internal/exif"
)

func main() {
	var inputPath string
	flag.StringVar(&inputPath, "input", "", "path to the image file")
	flag.Parse()

	if inputPath == "" {
		fmt.Fprintln(os.Stderr, "--input is required")
		os.Exit(1)
	}

	metadata, err := exif.Extract(inputPath, exif.OSImageSource{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to extract exif: %v\n", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode json: %v\n", err)
		os.Exit(1)
	}
}
