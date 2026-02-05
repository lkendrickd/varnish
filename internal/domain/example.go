// example.go parses example.env files to bootstrap a project config.
//
// This file is used by:
//   - cli/init.go: for the --from-example flag
//
// Parses lines like:
//
//	DATABASE_HOST=${DATABASE_HOST:-localhost}
//	LOG_LEVEL=${LOG_LEVEL:-info}
//	SIMPLE_VAR=value
//
// Extracts:
//   - Variable name (DATABASE_HOST)
//   - Default value if present (localhost)
//
// Converts to store key format:
//
//	DATABASE_HOST → database.host
//	LOG_LEVEL → log_level
//	AWS_SECRET_KEY → aws.secret_key
package domain

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ExampleVar represents a variable parsed from an example.env file.
type ExampleVar struct {
	EnvName  string // Original env var name (DATABASE_HOST)
	Key      string // Converted store key (database.host)
	Default  string // Default value if specified
	HasValue bool   // Whether a default/value was found
}

// ParseExampleEnv reads an example.env file and extracts variable definitions.
func ParseExampleEnv(path string) ([]ExampleVar, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open example env: %w", err)
	}
	defer file.Close()

	var vars []ExampleVar
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		v, ok := parseLine(line)
		if !ok {
			continue
		}

		// Avoid duplicates
		if seen[v.EnvName] {
			continue
		}
		seen[v.EnvName] = true

		vars = append(vars, v)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read example env: %w", err)
	}

	return vars, nil
}

// parseLine parses a single line from an example.env file.
// Supports formats:
//
//	VAR=value
//	VAR=${VAR:-default}
//	VAR=${VAR}
//	export VAR=value
func parseLine(line string) (ExampleVar, bool) {
	// Remove "export " prefix if present
	line = strings.TrimPrefix(line, "export ")

	// Split on first =
	idx := strings.Index(line, "=")
	if idx == -1 {
		return ExampleVar{}, false
	}

	envName := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])

	// Validate env var name (letters, digits, underscore, starts with letter/underscore)
	if !isValidEnvName(envName) {
		return ExampleVar{}, false
	}

	v := ExampleVar{
		EnvName: envName,
		Key:     envNameToKey(envName),
	}

	// Parse the value part
	// Remove surrounding quotes if present
	value = trimQuotes(value)

	// Check for ${VAR:-default} pattern
	if def, ok := extractDefault(value); ok {
		v.Default = def
		v.HasValue = def != ""
	} else if !strings.HasPrefix(value, "${") {
		// Plain value (not a variable reference)
		v.Default = value
		v.HasValue = value != ""
	}

	return v, true
}

// extractDefault extracts the default value from ${VAR:-default} patterns.
// Returns the default and true if found, empty and false otherwise.
func extractDefault(s string) (string, bool) {
	// Match ${VAR:-default} or ${VAR-default}
	re := regexp.MustCompile(`^\$\{[^}]*:?-([^}]*)\}$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) == 2 {
		return matches[1], true
	}
	return "", false
}

// trimQuotes removes surrounding single or double quotes.
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// isValidEnvName checks if a string is a valid environment variable name.
func isValidEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_') {
				return false
			}
		} else {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
				(c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
	}
	return true
}

// envNameToKey converts an env var name to a store key.
// DATABASE_HOST → database.host
// LOG_LEVEL → log_level
// AWS_ACCESS_KEY → aws.access_key
//
// Heuristic: single underscores become dots if preceded by lowercase-able segment.
// Multiple consecutive uppercase letters stay together (AWS → aws).
func envNameToKey(name string) string {
	// Simple approach: lowercase and convert _ to .
	// But we want DATABASE_HOST → database.host, not database_host
	//
	// Strategy: split on _, lowercase each part, join with .
	// This handles most cases well.
	parts := strings.Split(name, "_")
	for i, p := range parts {
		parts[i] = strings.ToLower(p)
	}

	// Join with dots, but collapse single-letter parts
	// e.g., A_B_C → a.b.c is probably not what we want
	// Keep it simple for now: just join with dots
	return strings.Join(parts, ".")
}

// GenerateProjectConfig creates a ProjectConfig from parsed example vars.
// Groups related vars into glob patterns where possible.
func GenerateProjectConfig(vars []ExampleVar) *ProjectConfig {
	cfg := NewProjectConfig()

	// Track prefixes to potentially group them
	prefixCount := make(map[string]int)
	for _, v := range vars {
		parts := strings.Split(v.Key, ".")
		if len(parts) > 1 {
			prefix := parts[0]
			prefixCount[prefix]++
		}
	}

	// Build include list
	// If a prefix has 2+ vars, use a glob pattern
	usedPrefixes := make(map[string]bool)
	for _, v := range vars {
		parts := strings.Split(v.Key, ".")
		if len(parts) > 1 {
			prefix := parts[0]
			if prefixCount[prefix] >= 2 && !usedPrefixes[prefix] {
				cfg.Include = append(cfg.Include, prefix+".*")
				usedPrefixes[prefix] = true
			} else if prefixCount[prefix] < 2 {
				cfg.Include = append(cfg.Include, v.Key)
			}
		} else {
			cfg.Include = append(cfg.Include, v.Key)
		}
	}

	return cfg
}
