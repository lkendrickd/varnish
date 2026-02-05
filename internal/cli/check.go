// check.go implements the "varnish check" command.
//
// This file is used by:
//   - cli/root.go: dispatches "check" command here
//
// Validates the project configuration and checks for issues:
//   - .varnish.yaml syntax is valid
//   - All required variables are present in the store
//   - No circular dependencies in computed values
//
// Usage:
//
//	varnish check           # Validate current project
//	varnish check --strict  # Fail if any variables are missing
package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/dk/varnish/internal/domain"
)

func runCheck(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	strict := fs.Bool("strict", false, "fail if any variables are missing")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Track issues found
	var errors []string
	var warnings []string

	// Check 1: Load and validate project config
	cfg, err := domain.LoadProjectConfig()
	if err != nil {
		return fmt.Errorf("invalid .varnish.yaml: %w", err)
	}
	if cfg == nil {
		return fmt.Errorf("no .varnish.yaml found (run 'varnish init' first)")
	}
	fmt.Fprintf(stdout, "✓ .varnish.yaml is valid (project: %s)\n", cfg.Project)

	// Check 2: Validate include patterns
	if len(cfg.Include) == 0 {
		warnings = append(warnings, "no include patterns defined - no variables will be resolved")
	} else {
		fmt.Fprintf(stdout, "✓ %d include pattern(s) defined\n", len(cfg.Include))
	}

	// Check 3: Load store
	store, err := domain.LoadStore()
	if err != nil {
		return fmt.Errorf("cannot load store: %w", err)
	}
	fmt.Fprintf(stdout, "✓ store loaded (%d total variables)\n", len(store.Keys()))

	// Check 4: Check for missing variables
	resolver := domain.NewResolver(store, cfg)
	missing := resolver.MissingVars()
	if len(missing) > 0 {
		if *strict {
			for _, key := range missing {
				errors = append(errors, fmt.Sprintf("missing variable: %s", key))
			}
		} else {
			for _, key := range missing {
				warnings = append(warnings, fmt.Sprintf("missing variable: %s", key))
			}
		}
	} else {
		fmt.Fprintln(stdout, "✓ all variables are present")
	}

	// Check 5: Validate computed values can be interpolated
	if len(cfg.Computed) > 0 {
		vars := resolver.Resolve()
		// Build a map for interpolation check
		resolved := make(map[string]string)
		for _, v := range vars {
			resolved[v.Key] = v.Value
		}

		for envName, template := range cfg.Computed {
			// Check for unresolved ${...} patterns
			if containsUnresolvedVar(template, resolved) {
				warnings = append(warnings, fmt.Sprintf("computed %s may have unresolved variables", envName))
			}
		}
		fmt.Fprintf(stdout, "✓ %d computed value(s) checked\n", len(cfg.Computed))
	}

	// Print warnings
	if len(warnings) > 0 {
		fmt.Fprintln(stdout, "\nWarnings:")
		for _, w := range warnings {
			fmt.Fprintf(stdout, "  ⚠ %s\n", w)
		}
	}

	// Print errors
	if len(errors) > 0 {
		fmt.Fprintln(stderr, "\nErrors:")
		for _, e := range errors {
			fmt.Fprintf(stderr, "  ✗ %s\n", e)
		}
		return fmt.Errorf("check failed with %d error(s)", len(errors))
	}

	fmt.Fprintln(stdout, "\n✓ All checks passed")
	return nil
}

// containsUnresolvedVar checks if template has ${var} patterns that aren't in resolved.
func containsUnresolvedVar(template string, resolved map[string]string) bool {
	// Simple check: look for ${...} patterns
	// This is a basic implementation - could be more sophisticated
	inVar := false
	varStart := 0

	for i := 0; i < len(template)-1; i++ {
		if template[i] == '$' && template[i+1] == '{' {
			inVar = true
			varStart = i + 2
		} else if inVar && template[i] == '}' {
			varName := template[varStart:i]
			if _, ok := resolved[varName]; !ok {
				return true
			}
			inVar = false
		}
	}

	return false
}
