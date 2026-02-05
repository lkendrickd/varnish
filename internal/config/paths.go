// Package config handles filesystem paths and permissions for varnish.
//
// This package is used by:
//   - domain/store.go: to know where to read/write the central variable store
//   - domain/registry.go: to locate the directory-to-project registry
//   - domain/project.go: to locate project configs
//   - cli/*: to locate project configs and the central store
//
// Varnish stores all data in ~/.varnish/:
//   - store.yaml: all variables (0600 permissions - contains secrets)
//   - registry.yaml: maps directories to project names (0644)
//   - projects/: directory containing per-project configs
//   - <project>.yaml: project-specific config (0644)
package config

import (
	"os"
	"path/filepath"
)

const (
	// DirName is the hidden directory name in the user's home.
	DirName = ".varnish"

	// StoreFileName is the central variable store.
	StoreFileName = "store.yaml"

	// ConfigFileName is the global config file.
	ConfigFileName = "config.yaml"

	// RegistryFileName maps directories to project names.
	RegistryFileName = "registry.yaml"

	// ProjectsDirName is the subdirectory for project configs.
	ProjectsDirName = "projects"

	// ProjectConfigName is the legacy per-project config file name.
	// Kept for migration purposes.
	ProjectConfigName = ".varnish.yaml"

	// PermSecure is for files containing secrets (owner read/write only).
	PermSecure os.FileMode = 0600

	// PermDir is for the .varnish directory (owner full access only).
	PermDir os.FileMode = 0700

	// PermConfig is for non-secret config files.
	PermConfig os.FileMode = 0644
)

// VarnishDir returns the path to ~/.varnish.
// It expands ~ to the actual home directory.
func VarnishDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DirName), nil
}

// StorePath returns the path to ~/.varnish/store.yaml.
func StorePath() (string, error) {
	dir, err := VarnishDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, StoreFileName), nil
}

// ConfigPath returns the path to ~/.varnish/config.yaml.
func ConfigPath() (string, error) {
	dir, err := VarnishDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// EnsureVarnishDir creates ~/.varnish if it doesn't exist.
// Sets permissions to 0700 (owner only) since it will contain secrets.
func EnsureVarnishDir() error {
	dir, err := VarnishDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, PermDir)
}

// RegistryPath returns the path to ~/.varnish/registry.yaml.
func RegistryPath() string {
	dir, err := VarnishDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, RegistryFileName)
}

// ProjectsDir returns the path to ~/.varnish/projects/.
func ProjectsDir() string {
	dir, err := VarnishDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, ProjectsDirName)
}

// EnsureProjectsDir creates ~/.varnish/projects/ if it doesn't exist.
func EnsureProjectsDir() error {
	if err := EnsureVarnishDir(); err != nil {
		return err
	}
	return os.MkdirAll(ProjectsDir(), PermDir)
}

// ProjectConfigPathFor returns the path for a specific project's config.
// e.g., ~/.varnish/projects/myapp.yaml
func ProjectConfigPathFor(project string) string {
	return filepath.Join(ProjectsDir(), project+".yaml")
}

// AtomicWrite writes data to a file atomically by writing to a temp file
// first, syncing, then renaming. This prevents partial writes.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Create temp file in same directory (for atomic rename)
	tmp, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// Clean up on any error
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	// Write data
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}

	// Sync to disk
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}

	// Close before rename
	if err := tmp.Close(); err != nil {
		return err
	}

	// Set permissions
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}

	tmpName = "" // Prevent cleanup since rename succeeded
	return nil
}

// FindProjectConfig searches for .varnish.yaml starting from the current
// directory and walking up to the filesystem root.
// Returns the path if found, empty string if not found.
func FindProjectConfig() (string, error) {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree
	for {
		candidate := filepath.Join(dir, ProjectConfigName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, not found
			return "", nil
		}
		dir = parent
	}
}

// ProjectConfigPath returns the path where .varnish.yaml should be created
// in the current working directory.
func ProjectConfigPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ProjectConfigName), nil
}
