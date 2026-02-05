// init.go implements the "varnish init" command.
//
// This file is used by:
//   - cli/root.go: dispatches "init" command here
//
// Registers the current directory with a project and creates/updates
// the project config in ~/.varnish/projects/<project>.yaml.
// Optionally imports defaults from a .env file into the store.
//
// Options:
//
//	--project        Project name for namespacing (default: current directory name)
//	--from           Path to .env file (auto-detects example.env or .env)
//	--no-import      Don't import default values into the store
//	--sync           Sync store with .env file (removes empty/missing vars)
//	--force          Overwrite existing project config
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dk/varnish/internal/config"
	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/registry"
	"github.com/dk/varnish/internal/store"
)

func runInit(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "project name for namespacing (default: current directory name)")
	fs.StringVar(projectFlag, "p", "", "project name (shorthand)")
	fromEnv := fs.String("from", "", "path to .env file (auto-detects example.env or .env if not specified)")
	fs.StringVar(fromEnv, "f", "", "path to .env file (shorthand)")
	noImport := fs.Bool("no-import", false, "don't import default values into the store")
	sync := fs.Bool("sync", false, "sync store with .env (removes vars that are empty/missing)")
	fs.BoolVar(sync, "s", false, "sync store (shorthand)")
	force := fs.Bool("force", false, "overwrite existing project config")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Determine project name
	projectName := *projectFlag
	if projectName == "" {
		projectName = filepath.Base(cwd)
	}

	// Load registry to check if already registered
	reg, err := registry.Load()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	// Check if this directory is already registered
	existingProject := reg.Lookup(cwd)
	if existingProject != "" && existingProject != projectName && !*force {
		return fmt.Errorf("directory already registered to project '%s' (use --force to change)", existingProject)
	}

	// Check if project config already exists
	if project.Exists(projectName) && !*force {
		return fmt.Errorf("project '%s' already exists (use --force to overwrite)", projectName)
	}

	var cfg *project.Config
	var vars []project.ExampleVar

	// Determine .env file path
	// Priority: explicit --from > .env > example.env
	envPath := *fromEnv
	if envPath == "" {
		if _, statErr := os.Stat(".env"); statErr == nil {
			envPath = ".env"
		} else if _, statErr := os.Stat("example.env"); statErr == nil {
			envPath = "example.env"
		}
	}

	if envPath == "" {
		// No .env file found - show helpful message
		fmt.Fprintln(stderr, "no .env or example.env found in current directory")
		fmt.Fprintln(stderr, "use: varnish init -f path/to/.env")
		return fmt.Errorf("no .env file found")
	}

	// Parse .env file and generate config
	vars, err = project.ParseExampleEnv(envPath)
	if err != nil {
		return fmt.Errorf("parse %s: %w", envPath, err)
	}

	if len(vars) == 0 {
		fmt.Fprintf(stderr, "warning: no variables found in %s\n", envPath)
		cfg = project.New()
	} else {
		cfg = project.GenerateConfig(vars)
		fmt.Fprintf(stdout, "parsed %d variables from %s\n", len(vars), envPath)
	}

	// Set project name
	cfg.Project = projectName

	// Save the project config to ~/.varnish/projects/<project>.yaml
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// Register this directory with the project
	reg.Register(cwd, projectName)
	if err := reg.Save(); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	configPath := config.ProjectConfigPathFor(projectName)
	fmt.Fprintf(stdout, "registered %s → project '%s'\n", cwd, projectName)
	fmt.Fprintf(stdout, "config: %s\n", configPath)

	// Import defaults into store if we have vars and not disabled
	if !*noImport && len(vars) > 0 {
		st, err := store.Load()
		if err != nil {
			return fmt.Errorf("load store: %w", err)
		}

		// Build set of keys that should exist (from .env file)
		shouldExist := make(map[string]bool)
		for _, v := range vars {
			storeKey := projectName + "." + v.Key
			shouldExist[storeKey] = true
		}

		added := 0
		removed := 0

		// Add/update variables with values
		for _, v := range vars {
			storeKey := projectName + "." + v.Key
			if v.HasValue {
				st.Set(storeKey, v.Default)
				added++
			} else if *sync {
				// --sync: remove variables that have no value in .env
				if st.Delete(storeKey) {
					removed++
					fmt.Fprintf(stdout, "removed %s (no value in .env)\n", v.Key)
				}
			}
		}

		// --sync: also remove variables NOT in .env file at all
		if *sync {
			prefix := projectName + "."
			for _, key := range st.Keys() {
				if strings.HasPrefix(key, prefix) && !shouldExist[key] {
					st.Delete(key)
					removed++
					// Show the key without project prefix
					shortKey := strings.TrimPrefix(key, prefix)
					fmt.Fprintf(stdout, "removed %s (not in .env)\n", shortKey)
				}
			}
		}

		if added > 0 || removed > 0 {
			if err := st.Save(); err != nil {
				return fmt.Errorf("save store: %w", err)
			}
			if added > 0 {
				fmt.Fprintf(stdout, "imported %d default values into store\n", added)
			}
			if removed > 0 {
				fmt.Fprintf(stdout, "removed %d stale variables\n", removed)
			}
		}
	}

	return nil
}
