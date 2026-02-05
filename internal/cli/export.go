// export.go implements the "varnish export" command.
//
// This file is used by:
//   - cli/root.go: dispatches "export" command here
//
// Outputs shell export statements for use with eval:
//
//	eval $(varnish export)
//	source <(varnish export)
//
// This loads the project's environment variables directly into the current shell.
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/resolver"
	"github.com/dk/varnish/internal/store"
)

func runExport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: varnish export

Output shell export statements for loading environment variables
into the current shell session.

Usage:
  eval $(varnish export)        # bash/zsh - load into current shell
  source <(varnish export)      # bash/zsh - alternative syntax
  varnish export > .env.sh      # save to file for later sourcing

The output format is:
  export DATABASE_HOST=localhost
  export DATABASE_PORT=5432
  ...

This reads .varnish.yaml in the current directory and resolves
variables from the store with the project prefix.`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
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

	// Resolve variables
	res := resolver.New(st, cfg)
	vars := res.Resolve()

	// Check for missing variables
	missing := res.MissingVars()
	if len(missing) > 0 {
		fmt.Fprintf(stderr, "# warning: missing variables in store: %s\n", strings.Join(missing, ", "))
	}

	// Output export statements
	for _, v := range vars {
		// Quote values for shell safety
		value := shellQuote(v.Value)
		fmt.Fprintf(stdout, "export %s=%s\n", v.EnvName, value)
	}

	return nil
}

// shellQuote quotes a value for safe use in shell.
// Uses single quotes and escapes internal single quotes.
func shellQuote(s string) string {
	// If the value is simple (alphanumeric, underscores, dots, dashes), no quotes needed
	simple := true
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.' || c == '/' || c == ':') {
			simple = false
			break
		}
	}
	if simple && s != "" {
		return s
	}

	// Use single quotes, escape internal single quotes as '\''
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}
