// store.go implements the "varnish store" subcommands.
//
// This file is used by:
//   - cli/root.go: dispatches "store" command here
//
// Subcommands:
//
//	varnish store set <key> <value>   Add/update a variable
//	varnish store set <key> --stdin   Read value from stdin (for secrets)
//	varnish store get <key>           Retrieve a variable
//	varnish store list [--pattern]    List variables (optional glob filter)
//	varnish store delete <key>        Remove a variable
//	varnish store import <file>       Import from .env file
//
// Project auto-detection:
//
//	When in a directory with .varnish.yaml, store commands automatically
//	use that project's namespace. Use --global to bypass this.
package cli

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dk/varnish/internal/crypto"
	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/registry"
	"github.com/dk/varnish/internal/store"
)

// detectProject returns the project name for the current directory.
// Uses the registry to find which project this directory belongs to.
// Returns empty string if not in a registered project directory.
func detectProject() string {
	reg, err := registry.Load()
	if err != nil {
		return ""
	}
	return reg.LookupCurrent()
}

// normalizeKey converts shell-style variable names to dot notation.
// DATABASE_HOST → database.host
// API_KEY → api.key
// PORT → port (single words are lowercased)
// If the key is already in dot notation or mixed case, it's returned unchanged.
func normalizeKey(key string) string {
	// Check if this looks like a shell-style variable (UPPER_SNAKE_CASE)
	// Must contain only uppercase letters, digits, and underscores
	isShellStyle := true
	for _, r := range key {
		if r == '_' {
			// underscore - OK
		} else if r >= 'A' && r <= 'Z' {
			// uppercase letter - OK
		} else if r >= '0' && r <= '9' {
			// digit - OK
		} else {
			// lowercase letter, dot, or other character - not shell style
			isShellStyle = false
			break
		}
	}

	// Only convert if it's shell-style (all uppercase)
	if !isShellStyle {
		return key
	}

	// Convert: lowercase and replace underscores with dots
	return strings.ToLower(strings.ReplaceAll(key, "_", "."))
}

func runStore(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printStoreUsage(stdout)
		return nil
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "set":
		return runStoreSet(subArgs, stdout, stderr)
	case "get":
		return runStoreGet(subArgs, stdout, stderr)
	case "list", "ls":
		return runStoreList(subArgs, stdout, stderr)
	case "delete", "rm":
		return runStoreDelete(subArgs, stdout, stderr)
	case "import":
		return runStoreImport(subArgs, stdout, stderr)
	case "encrypt":
		return runStoreEncrypt(subArgs, stdout, stderr)
	case "help", "-h", "--help":
		printStoreUsage(stdout)
		return nil
	default:
		fmt.Fprintf(stderr, "unknown store subcommand: %s\n\n", subcmd)
		printStoreUsage(stderr)
		return fmt.Errorf("unknown store subcommand: %s", subcmd)
	}
}

func printStoreUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: varnish store <subcommand> [flags]

Subcommands:
  set <key> <value>   Add or update a variable in the store
  set <key>=<value>   Alternative syntax with equals sign
  set <key> --stdin   Read value from stdin (for secrets)
  get <key>           Retrieve a variable's value
  list, ls            List all variables (optional glob filter)
  delete, rm <key>    Remove a variable from the store
  import <file>       Import variables from a .env file
  encrypt             Enable encryption on the store

Keys can use either dot notation (db.host) or shell-style (DATABASE_HOST).
Shell-style keys are automatically converted: DATABASE_HOST → database.host

Flags:
  -p, --project <ref>   Namespace under project (name or ID from 'varnish project list')
  -g, --global          Bypass project auto-detection, use global namespace

When in a directory with .varnish.yaml, the project is auto-detected.
Use --global to set/get variables without a project prefix.

Examples:
  varnish store set db.host localhost      # dot notation
  varnish store set DATABASE_HOST localhost # shell-style (same as above)
  varnish store set -p 1 db.host localhost # by project ID
  varnish store list -p 2                  # list project #2's vars
  varnish store list --global              # shows all vars`)
}

// resolveProjectFlag resolves the project flag value.
// If projectFlag is set, it resolves it (could be name or numeric ID).
// If projectFlag is empty and not global, it auto-detects from current directory.
// Returns the resolved project name (empty string if global/no project).
func resolveProjectFlag(projectFlag string, global bool) (string, error) {
	if global {
		return "", nil
	}
	if projectFlag == "" {
		return detectProject(), nil
	}
	// Resolve the project reference (could be name or ID)
	return resolveProjectRef(projectFlag)
}

// runStoreSet handles: varnish store set <key> <value> [--stdin] [--project]
// Also supports: varnish store set <key>=<value>
func runStoreSet(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("store set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fromStdin := fs.Bool("stdin", false, "read value from stdin")
	projectFlag := fs.String("project", "", "namespace under project name")
	fs.StringVar(projectFlag, "p", "", "namespace under project name (shorthand)")
	global := fs.Bool("global", false, "bypass project auto-detection")
	fs.BoolVar(global, "g", false, "bypass project auto-detection (shorthand)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()

	// Need at least key
	if len(remaining) < 1 {
		fmt.Fprintln(stderr, "usage: varnish store set <key> <value>")
		fmt.Fprintln(stderr, "       varnish store set <key>=<value>")
		fmt.Fprintln(stderr, "       varnish store set <key> --stdin")
		return fmt.Errorf("missing key")
	}

	// Resolve project (auto-detect or resolve ID/name)
	resolvedProject, err := resolveProjectFlag(*projectFlag, *global)
	if err != nil {
		return err
	}

	var key, value string

	// Check if first arg contains = (key=value syntax)
	if idx := strings.Index(remaining[0], "="); idx > 0 {
		key = normalizeKey(remaining[0][:idx])
		value = remaining[0][idx+1:]
	} else {
		key = normalizeKey(remaining[0])

		if *fromStdin {
			// Read value from stdin (trim trailing newline)
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return fmt.Errorf("read stdin: %w", err)
			}
			value = strings.TrimSuffix(line, "\n")
			value = strings.TrimSuffix(value, "\r") // Handle Windows line endings
		} else {
			// Value from argument
			if len(remaining) < 2 {
				fmt.Fprintln(stderr, "usage: varnish store set <key> <value>")
				fmt.Fprintln(stderr, "       varnish store set <key>=<value>")
				return fmt.Errorf("missing value")
			}
			value = remaining[1]
		}
	}

	// Apply project prefix
	storeKey := key
	if resolvedProject != "" {
		storeKey = resolvedProject + "." + key
	}

	// Load, modify, save
	st, err := store.Load()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	st.Set(storeKey, value)

	if err := st.Save(); err != nil {
		return fmt.Errorf("save store: %w", err)
	}

	fmt.Fprintf(stdout, "set %s\n", storeKey)

	// If we have a project, ensure the key pattern is in the project's include list
	if resolvedProject != "" {
		if err := ensureIncludePattern(resolvedProject, key, stdout); err != nil {
			// Non-fatal - warn but don't fail
			fmt.Fprintf(stderr, "warning: could not update project config: %v\n", err)
		}
	}

	return nil
}

// runStoreGet handles: varnish store get <key> [--project]
func runStoreGet(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("store get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "namespace under project name")
	fs.StringVar(projectFlag, "p", "", "namespace under project name (shorthand)")
	global := fs.Bool("global", false, "bypass project auto-detection")
	fs.BoolVar(global, "g", false, "bypass project auto-detection (shorthand)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: varnish store get <key>")
		return fmt.Errorf("expected exactly one key")
	}

	key := normalizeKey(fs.Arg(0))

	// Resolve project (auto-detect or resolve ID/name)
	resolvedProject, err := resolveProjectFlag(*projectFlag, *global)
	if err != nil {
		return err
	}

	// Apply project prefix
	storeKey := key
	if resolvedProject != "" {
		storeKey = resolvedProject + "." + key
	}

	st, err := store.Load()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	value, ok := st.Get(storeKey)
	if !ok {
		return fmt.Errorf("key not found: %s", storeKey)
	}

	fmt.Fprintln(stdout, value)
	return nil
}

// runStoreList handles: varnish store list [--pattern <glob>] [--project]
func runStoreList(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("store list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pattern := fs.String("pattern", "", "glob pattern to filter keys")
	projectFlag := fs.String("project", "", "filter to project namespace")
	fs.StringVar(projectFlag, "p", "", "filter to project namespace (shorthand)")
	global := fs.Bool("global", false, "show all variables (bypass project auto-detection)")
	fs.BoolVar(global, "g", false, "show all variables (shorthand)")
	jsonOutput := fs.Bool("json", false, "output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve project (auto-detect or resolve ID/name)
	resolvedProject, err := resolveProjectFlag(*projectFlag, *global)
	if err != nil {
		return err
	}

	st, err := store.Load()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	keys := st.Keys()
	if len(keys) == 0 {
		if *jsonOutput {
			return json.NewEncoder(stdout).Encode(map[string]interface{}{
				"variables": []interface{}{},
			})
		}
		fmt.Fprintln(stderr, "store is empty")
		return nil
	}

	// Build effective pattern
	effectivePattern := *pattern
	if resolvedProject != "" && effectivePattern == "" {
		effectivePattern = resolvedProject + ".*"
	} else if resolvedProject != "" {
		effectivePattern = resolvedProject + "." + effectivePattern
	}

	// Collect matching variables
	variables := make(map[string]string)
	for _, key := range keys {
		// Filter by pattern if specified
		if effectivePattern != "" && !matchGlob(effectivePattern, key) {
			continue
		}
		value, _ := st.Get(key)
		variables[key] = value
	}

	if *jsonOutput {
		return json.NewEncoder(stdout).Encode(map[string]interface{}{
			"variables": variables,
		})
	}

	for _, key := range keys {
		if value, ok := variables[key]; ok {
			fmt.Fprintf(stdout, "%s=%s\n", key, value)
		}
	}

	return nil
}

// runStoreDelete handles: varnish store delete <key> [--project]
func runStoreDelete(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("store delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "namespace under project name")
	fs.StringVar(projectFlag, "p", "", "namespace under project name (shorthand)")
	global := fs.Bool("global", false, "bypass project auto-detection")
	fs.BoolVar(global, "g", false, "bypass project auto-detection (shorthand)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: varnish store delete <key>")
		return fmt.Errorf("expected exactly one key")
	}

	key := normalizeKey(fs.Arg(0))

	// Resolve project (auto-detect or resolve ID/name)
	resolvedProject, err := resolveProjectFlag(*projectFlag, *global)
	if err != nil {
		return err
	}

	// Apply project prefix
	storeKey := key
	if resolvedProject != "" {
		storeKey = resolvedProject + "." + key
	}

	st, err := store.Load()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	if !st.Delete(storeKey) {
		return fmt.Errorf("key not found: %s", storeKey)
	}

	if err := st.Save(); err != nil {
		return fmt.Errorf("save store: %w", err)
	}

	fmt.Fprintf(stdout, "deleted %s\n", storeKey)
	return nil
}

// runStoreImport handles: varnish store import <file> [--project]
func runStoreImport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("store import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectFlag := fs.String("project", "", "namespace under project name")
	fs.StringVar(projectFlag, "p", "", "namespace under project name (shorthand)")
	global := fs.Bool("global", false, "bypass project auto-detection")
	fs.BoolVar(global, "g", false, "bypass project auto-detection (shorthand)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve project (auto-detect or resolve ID/name)
	resolvedProject, err := resolveProjectFlag(*projectFlag, *global)
	if err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: varnish store import <file> [--project name-or-id]")
		return fmt.Errorf("expected exactly one file")
	}

	filePath := fs.Arg(0)

	// Parse the .env file using our example parser
	vars, err := project.ParseExampleEnv(filePath)
	if err != nil {
		return fmt.Errorf("parse file: %w", err)
	}

	if len(vars) == 0 {
		fmt.Fprintln(stderr, "no variables found in file")
		return nil
	}

	// Load store
	st, err := store.Load()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	// Import each variable
	count := 0
	for _, v := range vars {
		if v.HasValue {
			// Apply project prefix
			storeKey := v.Key
			if resolvedProject != "" {
				storeKey = resolvedProject + "." + v.Key
			}
			st.Set(storeKey, v.Default)
			count++
			fmt.Fprintf(stdout, "imported %s → %s\n", v.EnvName, storeKey)
		}
	}

	if count == 0 {
		fmt.Fprintln(stderr, "no variables with values to import")
		return nil
	}

	if err := st.Save(); err != nil {
		return fmt.Errorf("save store: %w", err)
	}

	fmt.Fprintf(stdout, "imported %d variables\n", count)
	return nil
}

func runStoreEncrypt(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("store encrypt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	password := fs.String("password", "", "encryption password (or set VARNISH_PASSWORD)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// If --password provided, set the env var for this session
	if *password != "" {
		os.Setenv(crypto.PasswordEnvVar, *password)
	}

	// Validate password is available
	if _, err := crypto.GetPassword(); err != nil {
		return fmt.Errorf("encryption requires --password or VARNISH_PASSWORD env var")
	}

	// Load store
	st, err := store.Load()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	// Check if already encrypted
	if st.IsEncrypted() {
		fmt.Fprintln(stdout, "store is already encrypted")
		return nil
	}

	// Enable encryption
	if err := st.EnableEncryption(); err != nil {
		return fmt.Errorf("enable encryption: %w", err)
	}

	// Save encrypted store
	if err := st.Save(); err != nil {
		return fmt.Errorf("save store: %w", err)
	}

	fmt.Fprintf(stdout, "store encrypted (%d variables)\n", st.Len())
	return nil
}

// matchGlob is a simple glob matcher for store list --pattern.
// Supports * as wildcard.
func matchGlob(pattern, s string) bool {
	// Simple implementation: convert * to .* and use contains logic
	// For full glob, we'd use filepath.Match, but that has path separator issues
	if pattern == "*" {
		return true
	}

	// Handle prefix match (e.g., "database.*")
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(s, prefix+".")
	}

	// Handle suffix match (e.g., "*.host")
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(s, suffix)
	}

	// Exact match
	return pattern == s
}

// ensureIncludePattern adds a pattern to the project config if the key isn't already covered.
// For example, if key is "db.user", it will add "db.*" if not already included.
func ensureIncludePattern(projectName, key string, stdout io.Writer) error {
	cfg, err := project.LoadByName(projectName)
	if err != nil {
		return err
	}

	// Check if key is already matched by existing includes
	for _, pat := range cfg.Include {
		if matchGlob(pat, key) {
			return nil // Already covered
		}
	}

	// Generate pattern for this key
	// For "db.user" -> "db.*", for "simple" -> "simple"
	pat := key
	if idx := strings.Index(key, "."); idx > 0 {
		pat = key[:idx] + ".*"
	}

	// Check if this pattern already exists
	for _, p := range cfg.Include {
		if p == pat {
			return nil // Pattern already exists
		}
	}

	// Add the new pattern
	cfg.Include = append(cfg.Include, pat)

	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "added '%s' to project includes\n", pat)
	return nil
}
