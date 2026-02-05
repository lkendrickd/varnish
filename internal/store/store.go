// Package store manages the central variable store at ~/.varnish/store.yaml.
//
// The store uses a flat namespace with dot-separated keys:
//
//	database.host: localhost
//	database.password: secret123
//	aws.region: us-east-1
//
// Writes are atomic: we write to a temp file then rename, so a crash
// mid-write won't corrupt the store.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/dk/varnish/internal/config"
	"gopkg.in/yaml.v3"
)

// Store holds all variables in the central store.
// The YAML file looks like:
//
//	version: 1
//	variables:
//	  database.host: localhost
//	  database.password: secret123
type Store struct {
	Version   int               `yaml:"version"`
	Variables map[string]string `yaml:"variables"`
}

// New creates an empty store with version 1.
func New() *Store {
	return &Store{
		Version:   1,
		Variables: make(map[string]string),
	}
}

// Load reads the store from ~/.varnish/store.yaml.
// If the file doesn't exist, returns an empty store (not an error).
func Load() (*Store, error) {
	path, err := config.StorePath()
	if err != nil {
		return nil, fmt.Errorf("get store path: %w", err)
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// No store yet, return empty one
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read store: %w", err)
	}

	var s Store
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse store: %w", err)
	}

	// Ensure map is initialized even if YAML had empty variables
	if s.Variables == nil {
		s.Variables = make(map[string]string)
	}

	return &s, nil
}

// Save writes the store to ~/.varnish/store.yaml atomically.
// Atomic write: write to temp file, then rename. This prevents corruption
// if the process is killed mid-write.
func (s *Store) Save() error {
	// Ensure the directory exists
	if err := config.EnsureVarnishDir(); err != nil {
		return fmt.Errorf("create varnish dir: %w", err)
	}

	path, err := config.StorePath()
	if err != nil {
		return fmt.Errorf("get store path: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal store: %w", err)
	}

	// Write to temp file in same directory (same filesystem for atomic rename)
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "store-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up temp file on any error
	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	// Write data
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	// Sync to disk before rename
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Set secure permissions before rename
	if err := os.Chmod(tmpPath, config.PermSecure); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	// Clear tmpPath so defer doesn't try to remove it
	tmpPath = ""

	return nil
}

// SaveTo writes the store to a specific path (for testing).
func (s *Store) SaveTo(path string) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal store: %w", err)
	}
	return config.AtomicWrite(path, data, config.PermSecure)
}

// LoadFrom reads a store from a specific path (for testing).
func LoadFrom(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read store: %w", err)
	}

	var s Store
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse store: %w", err)
	}

	if s.Variables == nil {
		s.Variables = make(map[string]string)
	}

	return &s, nil
}

// Set adds or updates a variable in the store.
// Does not persist - call Save() after making changes.
func (s *Store) Set(key, value string) {
	s.Variables[key] = value
}

// Get retrieves a variable from the store.
// Returns the value and true if found, empty string and false if not.
func (s *Store) Get(key string) (string, bool) {
	val, ok := s.Variables[key]
	return val, ok
}

// Delete removes a variable from the store.
// Returns true if the key existed, false if it didn't.
// Does not persist - call Save() after making changes.
func (s *Store) Delete(key string) bool {
	if _, ok := s.Variables[key]; !ok {
		return false
	}
	delete(s.Variables, key)
	return true
}

// Keys returns all variable keys in sorted order.
func (s *Store) Keys() []string {
	keys := make([]string, 0, len(s.Variables))
	for k := range s.Variables {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Len returns the number of variables in the store.
func (s *Store) Len() int {
	return len(s.Variables)
}
