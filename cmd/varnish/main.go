// Package main is the entry point for the varnish CLI.
//
// This file:
//   - Calls cli.Run with command-line arguments
//   - Exits with code 1 on error
//
// Build with version:
//
//	go build -ldflags="-X github.com/dk/varnish/internal/cli.Version=0.0.1" ./cmd/varnish
package main

import (
	"fmt"
	"os"

	"github.com/dk/varnish/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
