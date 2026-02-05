// project.go implements the "varnish project" command.
//
// This file is used by:
//   - cli/root.go: dispatches "project" command here
//
// Shows the current project name from the registry.
// Useful to confirm which project context you're in.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/dk/varnish/internal/config"
	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/registry"
	"github.com/dk/varnish/internal/store"
)

func runProject(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		// Default: show current project name
		return runProjectName(args, stdout, stderr)
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "name":
		return runProjectName(subArgs, stdout, stderr)
	case "list":
		return runProjectList(subArgs, stdout, stderr)
	case "delete":
		return runProjectDelete(subArgs, stdout, stderr)
	case "help", "-h", "--help":
		printProjectUsage(stdout)
		return nil
	default:
		// If not a subcommand, treat as flags for "name"
		if strings.HasPrefix(subcmd, "-") {
			return runProjectName(args, stdout, stderr)
		}
		fmt.Fprintf(stderr, "unknown project subcommand: %s\n\n", subcmd)
		printProjectUsage(stderr)
		return fmt.Errorf("unknown project subcommand: %s", subcmd)
	}
}

func printProjectUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: varnish project [subcommand]

Subcommands:
  name            Show current project name (default)
  list            List all projects in the store (with numeric IDs)
  delete <ref>    Delete all variables for a project (by name or ID)

Flags:
  --path      Show path to project config (with 'name')
  --dry-run   Preview deletions without making changes (with 'delete')

Projects can be referenced by name or numeric ID from 'varnish project list'.

Examples:
  varnish project                   # show current project name
  varnish project --path            # show path to project config
  varnish project list              # list all projects with IDs
  varnish project delete myapp      # delete by name
  varnish project delete 1          # delete by ID
  varnish project delete 2 --dry-run  # preview deletion by ID`)
}

// getOrderedProjects returns project names sorted alphabetically with their variable counts.
// The order is stable and used for numeric ID assignment.
func getOrderedProjects() ([]string, map[string]int, error) {
	st, err := store.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load store: %w", err)
	}

	// Extract unique project prefixes from store keys
	projects := make(map[string]int) // project -> variable count
	for _, key := range st.Keys() {
		idx := strings.Index(key, ".")
		if idx > 0 {
			proj := key[:idx]
			projects[proj]++
		}
	}

	// Sort project names for stable ordering
	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)

	return names, projects, nil
}

// resolveProjectRef converts a project reference (name or numeric ID) to a project name.
// If ref is a number like "1", "2", etc., it looks up the project by index.
// Otherwise, it returns the ref as-is (assumed to be a project name).
func resolveProjectRef(ref string) (string, error) {
	// Try to parse as a number
	num, err := strconv.Atoi(ref)
	if err != nil {
		// Not a number, return as-is (it's a project name)
		return ref, nil
	}

	// It's a number, look up by index
	names, _, err := getOrderedProjects()
	if err != nil {
		return "", err
	}

	if len(names) == 0 {
		return "", fmt.Errorf("no projects found in store")
	}

	// IDs are 1-based
	if num < 1 || num > len(names) {
		return "", fmt.Errorf("invalid project ID: %d (valid range: 1-%d)", num, len(names))
	}

	return names[num-1], nil
}

// runProjectName shows the current project name
func runProjectName(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("project name", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showPath := fs.Bool("path", false, "show path to project config")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Load registry
	reg, err := registry.Load()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	// Look up current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	proj := reg.Lookup(cwd)
	if proj == "" {
		return fmt.Errorf("directory not registered (run 'varnish init' first)")
	}

	if *showPath {
		// Show path to project config
		configPath := config.ProjectConfigPathFor(proj)
		fmt.Fprintln(stdout, configPath)
	} else {
		// Show project name
		fmt.Fprintln(stdout, proj)
	}

	return nil
}

// runProjectList lists all projects found in the store
func runProjectList(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("project list", flag.ContinueOnError)
	fs.SetOutput(stderr)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	names, projects, err := getOrderedProjects()
	if err != nil {
		return err
	}

	if len(names) == 0 {
		fmt.Fprintln(stderr, "no projects found in store")
		return nil
	}

	// Load registry to show registered directories
	reg, regErr := registry.Load()

	// Print projects with IDs, variable counts, and registered directories
	for i, name := range names {
		id := i + 1 // 1-based IDs
		if regErr != nil {
			// No registry, just show without directory info
			fmt.Fprintf(stdout, "%d  %s (%d variables)\n", id, name, projects[name])
		} else {
			dirs := reg.ProjectDirs(name)
			if len(dirs) > 0 {
				fmt.Fprintf(stdout, "%d  %s (%d variables) → %s\n", id, name, projects[name], dirs[0])
			} else {
				fmt.Fprintf(stdout, "%d  %s (%d variables)\n", id, name, projects[name])
			}
		}
	}

	return nil
}

// runProjectDelete deletes all variables for a project
func runProjectDelete(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("project delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "preview deletions without making changes")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: varnish project delete <name-or-id> [--dry-run]")
		return fmt.Errorf("expected project name or ID")
	}

	// Resolve project reference (could be name or numeric ID)
	projectName, err := resolveProjectRef(fs.Arg(0))
	if err != nil {
		return err
	}
	prefix := projectName + "."

	st, err := store.Load()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	// Find all keys for this project
	var toDelete []string
	for _, key := range st.Keys() {
		if strings.HasPrefix(key, prefix) {
			toDelete = append(toDelete, key)
		}
	}

	if len(toDelete) == 0 {
		return fmt.Errorf("no variables found for project: %s", projectName)
	}

	if *dryRun {
		fmt.Fprintf(stdout, "would delete %d variables for project '%s':\n", len(toDelete), projectName)
		for _, key := range toDelete {
			fmt.Fprintf(stdout, "  %s\n", key)
		}
		return nil
	}

	// Delete all keys
	for _, key := range toDelete {
		st.Delete(key)
	}

	if saveErr := st.Save(); saveErr != nil {
		return fmt.Errorf("save store: %w", saveErr)
	}

	// Also remove from registry and delete config
	reg, regErr := registry.Load()
	if regErr == nil {
		// Remove all directory registrations for this project
		for dir, p := range reg.Projects {
			if p == projectName {
				delete(reg.Projects, dir)
			}
		}
		_ = reg.Save() // Best effort
	}

	// Delete project config file (best effort)
	_ = project.Delete(projectName)

	fmt.Fprintf(stdout, "deleted %d variables for project '%s'\n", len(toDelete), projectName)
	return nil
}
