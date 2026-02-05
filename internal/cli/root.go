// Package cli implements the command-line interface for varnish.
//
// root.go is the entry point - it dispatches to the appropriate command
// based on os.Args. Uses standard library flag package, no frameworks.
//
// This file is used by:
//   - cmd/varnish/main.go: calls cli.Run(os.Args[1:])
//
// Command structure:
//
//	varnish init [flags]
//	varnish store <subcommand> [flags]
//	varnish env [flags]
//	varnish run [flags] -- <command>
//	varnish list [flags]
//	varnish version
//	varnish help
package cli

import (
	"fmt"
	"io"
	"os"
)

// Run is the main entry point for the CLI.
// args should be os.Args[1:] (command and flags, not program name).
func Run(args []string) error {
	return run(args, os.Stdout, os.Stderr)
}

// run is the internal implementation, accepting writers for testing.
func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "init":
		return runInit(cmdArgs, stdout, stderr)
	case "store":
		return runStore(cmdArgs, stdout, stderr)
	case "env":
		return runEnv(cmdArgs, stdout, stderr)
	case "export":
		return runExport(cmdArgs, stdout, stderr)
	case "run":
		return runRun(cmdArgs, stdout, stderr)
	case "list":
		return runList(cmdArgs, stdout, stderr)
	case "project":
		return runProject(cmdArgs, stdout, stderr)
	case "completion":
		return runCompletion(cmdArgs, stdout, stderr)
	case "check":
		return runCheck(cmdArgs, stdout, stderr)
	case "version":
		return runVersion(stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", cmd)
		printUsage(stderr)
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `varnish - environment variable manager

Usage:
  varnish <command> [flags]

Commands:
  init        Initialize project (.varnish.yaml)
  store       Manage central store (set/get/list/delete/import)
  env         Generate .env file from store + project config
  export      Output shell export statements (use with eval)
  run         Execute command with injected env vars
  list        Show project's resolved variables
  project     Show current project name
  check       Validate config and check for missing variables
  completion  Generate shell completion scripts
  version     Show version
  help        Show this help

Examples:
  varnish store set database.host localhost --project myapp
  varnish env --force
  eval $(varnish export)

Run 'varnish <command> -h' for help on a specific command.`)
}

func runVersion(w io.Writer) error {
	// Version is set at build time via ldflags, defaults to "dev"
	fmt.Fprintln(w, Version)
	return nil
}

// Version is set at build time via:
//
//	go build -ldflags="-X github.com/lkendrickd/varnish/internal/cli.Version=1.0.0"
var Version = "dev"
