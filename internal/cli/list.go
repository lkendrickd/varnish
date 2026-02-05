// list.go implements the "varnish list" command.
//
// This file is used by:
//   - cli/root.go: dispatches "list" command here
//
// Shows the project's resolved variables.
// Options:
//
//	--resolved   Show final resolved values (default behavior)
//	--missing    Only show variables that are missing from the store
//	--json       Output as JSON
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/resolver"
	"github.com/dk/varnish/internal/store"
)

func runList(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	resolved := fs.Bool("resolved", false, "show resolved values (default)")
	missing := fs.Bool("missing", false, "only show missing variables")
	jsonOutput := fs.Bool("json", false, "output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load project config
	cfg, err := project.Load()
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if cfg == nil {
		return fmt.Errorf("no .varnish.yaml found (run 'varnish init' first)")
	}

	// Load store
	st, err := store.Load()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	// Create resolver
	res := resolver.New(st, cfg)

	if *missing {
		// Show only missing variables
		missingVars := res.MissingVars()

		if *jsonOutput {
			return json.NewEncoder(stdout).Encode(map[string]interface{}{
				"missing": missingVars,
			})
		}

		if len(missingVars) == 0 {
			fmt.Fprintln(stdout, "no missing variables")
			return nil
		}

		fmt.Fprintln(stdout, "missing variables:")
		for _, key := range missingVars {
			fmt.Fprintf(stdout, "  %s\n", key)
		}
		return nil
	}

	// Default: show resolved variables
	_ = resolved // Flag exists for explicitness, but is default behavior
	vars := res.Resolve()
	missingVars := res.MissingVars()

	if *jsonOutput {
		// Build JSON-friendly structure
		varList := make([]map[string]string, 0, len(vars))
		for _, v := range vars {
			varList = append(varList, map[string]string{
				"name":   v.EnvName,
				"value":  v.Value,
				"source": v.Source,
				"key":    v.Key,
			})
		}
		return json.NewEncoder(stdout).Encode(map[string]interface{}{
			"variables": varList,
			"missing":   missingVars,
		})
	}

	if len(vars) == 0 {
		fmt.Fprintln(stdout, "no variables configured")
		return nil
	}

	// Print with source information
	fmt.Fprintln(stdout, "resolved variables:")
	for _, v := range vars {
		source := formatSource(v.Source, v.Key)
		fmt.Fprintf(stdout, "  %s=%s  (%s)\n", v.EnvName, v.Value, source)
	}

	// Also show any missing
	if len(missingVars) > 0 {
		fmt.Fprintln(stdout, "\nmissing from store:")
		for _, key := range missingVars {
			fmt.Fprintf(stdout, "  %s\n", key)
		}
	}

	return nil
}

// formatSource creates a human-readable source description.
func formatSource(source, key string) string {
	switch source {
	case "store":
		return fmt.Sprintf("store: %s", key)
	case "override":
		return fmt.Sprintf("override: %s", key)
	case "computed":
		return "computed"
	default:
		return source
	}
}
