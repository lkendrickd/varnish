// Package resolver combines a Store and project Config to produce environment variables.
//
// Resolution order (later wins):
//  1. Store variables matching Include patterns
//  2. Overrides from project config
//  3. Computed values (with interpolation)
//
// Key transformation:
//   - Store keys like "database.host" become "DATABASE_HOST"
//   - Mappings can override this: mappings: { database.url: DB_URL }
//
// Interpolation in computed values:
//   - ${database.host} is replaced with the value of database.host
//   - Supports nested references to other computed values
package resolver

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/store"
)

// ResolvedVar represents a single resolved environment variable.
type ResolvedVar struct {
	EnvName string // The environment variable name (e.g., DATABASE_HOST)
	Value   string // The resolved value
	Source  string // Where it came from: "store", "override", or "computed"
	Key     string // Original store key (e.g., database.host)
}

// Resolver combines store and project config to produce env vars.
type Resolver struct {
	store   *store.Store
	project *project.Config
}

// New creates a resolver with the given store and project config.
func New(s *store.Store, p *project.Config) *Resolver {
	return &Resolver{
		store:   s,
		project: p,
	}
}

// Resolve produces the final set of environment variables.
// Returns them sorted by EnvName for consistent output.
func (r *Resolver) Resolve() []ResolvedVar {
	// Internal map: logical key (without project prefix) → value and source
	type intermediate struct {
		value  string
		source string
	}
	resolved := make(map[string]intermediate)

	// Step 1: Match store variables against Include patterns
	// If project is set, we look for "project.pattern" in store
	prefix := ""
	if r.project.Project != "" {
		prefix = r.project.Project + "."
	}

	for _, pattern := range r.project.Include {
		// The actual pattern to match in store
		storePattern := prefix + pattern

		for storeKey, value := range r.store.Variables {
			if matchPattern(storePattern, storeKey) {
				// Strip prefix from key for the logical name
				logicalKey := storeKey
				if prefix != "" && strings.HasPrefix(storeKey, prefix) {
					logicalKey = strings.TrimPrefix(storeKey, prefix)
				}
				resolved[logicalKey] = intermediate{value: value, source: "store"}
			}
		}
	}

	// Step 2: Apply overrides (these win over store values)
	for key, value := range r.project.Overrides {
		resolved[key] = intermediate{value: value, source: "override"}
	}

	// Step 3: Build the final env var list
	// First, convert store keys to env vars
	vars := make(map[string]ResolvedVar)

	for key, inter := range resolved {
		envName := r.keyToEnvName(key)
		vars[envName] = ResolvedVar{
			EnvName: envName,
			Value:   inter.value,
			Source:  inter.source,
			Key:     key,
		}
	}

	// Step 4: Process computed values (with interpolation)
	// Computed values can reference store keys or other computed values
	// Build a simple key→value map for interpolation
	valueMap := make(map[string]string)
	for key, inter := range resolved {
		valueMap[key] = inter.value
	}

	for envName, template := range r.project.Computed {
		value := r.interpolate(template, valueMap)
		vars[envName] = ResolvedVar{
			EnvName: envName,
			Value:   value,
			Source:  "computed",
			Key:     "", // Computed values don't have a store key
		}
	}

	// Convert to sorted slice
	result := make([]ResolvedVar, 0, len(vars))
	for _, v := range vars {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].EnvName < result[j].EnvName
	})

	return result
}

// MissingVars returns store keys referenced in Include patterns that don't exist.
// Returns logical keys (without project prefix) for display.
func (r *Resolver) MissingVars() []string {
	var missing []string
	seen := make(map[string]bool)

	prefix := ""
	if r.project.Project != "" {
		prefix = r.project.Project + "."
	}

	for _, pattern := range r.project.Include {
		// Check if pattern contains wildcards
		if strings.ContainsAny(pattern, "*?[") {
			// For glob patterns, we can't know what's "missing"
			continue
		}

		// Literal key - check if it exists (with prefix in store)
		storeKey := prefix + pattern
		if _, ok := r.store.Variables[storeKey]; !ok {
			if !seen[pattern] {
				missing = append(missing, pattern)
				seen[pattern] = true
			}
		}
	}

	sort.Strings(missing)
	return missing
}

// keyToEnvName converts a store key to an environment variable name.
// "database.host" → "DATABASE_HOST"
// Can be overridden by Mappings in project config.
func (r *Resolver) keyToEnvName(key string) string {
	// Check if there's an explicit mapping
	if envName, ok := r.project.Mappings[key]; ok {
		return envName
	}

	// Default: replace dots with underscores, uppercase
	name := strings.ReplaceAll(key, ".", "_")
	return strings.ToUpper(name)
}

// interpolate replaces ${key} references in a template with values.
// Looks up keys in the values map first, then falls back to the store.
func (r *Resolver) interpolate(template string, values map[string]string) string {
	// Match ${...} patterns
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	prefix := ""
	if r.project.Project != "" {
		prefix = r.project.Project + "."
	}

	return re.ReplaceAllStringFunc(template, func(match string) string {
		// Extract key name from ${key}
		key := match[2 : len(match)-1]

		// Look up in resolved values first (these are already logical keys)
		if value, ok := values[key]; ok {
			return value
		}

		// Fall back to store (for keys not in Include)
		// Try with project prefix first, then without
		storeKey := prefix + key
		if value, ok := r.store.Variables[storeKey]; ok {
			return value
		}
		if value, ok := r.store.Variables[key]; ok {
			return value
		}

		// Not found - leave as-is so user can see what's missing
		return match
	})
}

// matchPattern checks if a key matches a glob-like pattern.
// Supports * for any characters.
// "database.*" matches "database.host", "database.password", etc.
func matchPattern(pattern, key string) bool {
	// Use filepath.Match which supports glob patterns
	// But first, we need to handle "." as literal (not path separator)
	// filepath.Match treats "/" specially, so we substitute
	p := strings.ReplaceAll(pattern, ".", "\x00")
	k := strings.ReplaceAll(key, ".", "\x00")

	matched, err := filepath.Match(p, k)
	if err != nil {
		// Invalid pattern, treat as literal comparison
		return pattern == key
	}
	return matched
}
