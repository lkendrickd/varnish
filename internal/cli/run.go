// run.go implements the "varnish run" command.
//
// This file is used by:
//   - cli/root.go: dispatches "run" command here
//
// Executes a command with resolved environment variables injected.
// Usage:
//
//	varnish run -- ./myserver
//	varnish run --clean -- printenv
//
// Options:
//
//	--clean   Start with empty environment (only varnish vars)
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/dk/varnish/internal/domain"
)

func runRun(args []string, _ /* stdout */, stderr io.Writer) error {
	// Find the -- separator
	dashIdx := -1
	for i, arg := range args {
		if arg == "--" {
			dashIdx = i
			break
		}
	}

	var flagArgs, cmdArgs []string
	if dashIdx >= 0 {
		flagArgs = args[:dashIdx]
		cmdArgs = args[dashIdx+1:]
	} else {
		// No --, treat all args after flags as command
		// We need to parse flags first to know where they end
		flagArgs = args
		cmdArgs = nil
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	clean := fs.Bool("clean", false, "start with empty environment")

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	// If no -- was found, remaining args after flags are the command
	if dashIdx < 0 {
		cmdArgs = fs.Args()
	}

	if len(cmdArgs) == 0 {
		fmt.Fprintln(stderr, "usage: varnish run [--clean] -- <command> [args...]")
		return fmt.Errorf("no command specified")
	}

	// Load project config
	cfg, err := domain.LoadProjectConfig()
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if cfg == nil {
		return fmt.Errorf("no .varnish.yaml found (run 'varnish init' first)")
	}

	// Load store
	store, err := domain.LoadStore()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	// Resolve variables
	resolver := domain.NewResolver(store, cfg)
	vars := resolver.Resolve()

	// Build environment
	var env []string
	if *clean {
		// Start fresh - only include PATH and HOME for basic functionality
		if path := os.Getenv("PATH"); path != "" {
			env = append(env, "PATH="+path)
		}
		if home := os.Getenv("HOME"); home != "" {
			env = append(env, "HOME="+home)
		}
	} else {
		// Start with current environment
		env = os.Environ()
	}

	// Add resolved variables (these override existing)
	for _, v := range vars {
		env = append(env, v.EnvName+"="+v.Value)
	}

	// Find the executable
	executable, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		return fmt.Errorf("command not found: %s", cmdArgs[0])
	}

	// Replace current process with the command (exec)
	// This is the Unix way - varnish disappears and becomes the target process
	err = syscall.Exec(executable, cmdArgs, env)
	if err != nil {
		return fmt.Errorf("exec %s: %w", cmdArgs[0], err)
	}

	// This line is never reached - syscall.Exec replaces the process
	return nil
}
