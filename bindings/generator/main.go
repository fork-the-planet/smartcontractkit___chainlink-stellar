// Package main provides a CLI tool to generate Go bindings from Stellar contract Rust bindings.
//
// Usage:
//
//	stellar contract bindings rust --wasm <contract.wasm> | go run ./generator -name OnRamp -pkg onramp -out ./onramp
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	name := flag.String("name", "", "Contract name (e.g., OnRamp)")
	pkg := flag.String("pkg", "", "Go package name for generated code")
	out := flag.String("out", "", "Output directory for generated files")
	events := flag.String("events", "", "Optional path to Rust events source file (e.g., contracts/onramp/src/events.rs)")
	flag.Parse()

	if *name == "" || *pkg == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "Usage: stellar contract bindings rust --wasm <contract.wasm> | go run ./generator -name <Name> -pkg <package> -out <dir> [-events <events.rs>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Read Rust bindings from stdin
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read input: %v\n", err)
		os.Exit(1)
	}

	// Parse the Rust bindings
	contract, err := ParseRustBindings(string(input))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse Rust bindings: %v\n", err)
		os.Exit(1)
	}
	contract.Name = *name

	// Optionally parse events from Rust source file
	if *events != "" {
		eventsSource, err := os.ReadFile(*events)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read events file: %v\n", err)
			os.Exit(1)
		}
		parsedEvents := parseEvents(string(eventsSource))
		contract.Events = append(contract.Events, parsedEvents...)
		fmt.Printf("Parsed %d events from %s\n", len(parsedEvents), *events)
	}

	// Create output directory
	if err := os.MkdirAll(*out, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Generate types file
	typesCode := GenerateTypes(*pkg, contract)
	typesPath := filepath.Join(*out, "types.go")
	if err := os.WriteFile(typesPath, []byte(typesCode), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write types.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated %s\n", typesPath)

	// Generate client file
	clientCode := GenerateClient(*pkg, contract)
	clientPath := filepath.Join(*out, "client.go")
	if err := os.WriteFile(clientPath, []byte(clientCode), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write client.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated %s\n", clientPath)

	fmt.Printf("Successfully generated Go bindings for %s\n", *name)
}
