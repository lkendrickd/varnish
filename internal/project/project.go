// Package project manages the per-project config files stored in ~/.varnish/projects/.
//
// Project configs are stored centrally in ~/.varnish/projects/<project>.yaml
// instead of in the project directory. The registry maps directories to
// project names so varnish knows which config to use.
//
// A project config specifies:
//   - include: glob patterns for which store variables to pull in
//   - overrides: project-specific values that override the store
//   - mappings: rename store keys to different env var names
//   - computed: variables built from other variables (interpolation)
package project

import (
	"fmt"
	"os"

	"github.com/dk/varnish/internal/config"
	"github.com/dk/varnish/internal/registry"
	"gopkg.in/yaml.v3"
)

// Config holds the per-project configuration.
type Config struct {
	Version   int               `yaml:"version"`
	Project   string            `yaml:"project,omitempty"`
	Include   []string          `yaml:"include,omitempty"`
	Overrides map[string]string `yaml:"overrides,omitempty"`
	Mappings  map[string]string `yaml:"mappings,omitempty"`
	Computed  map[string]string `yaml:"computed,omitempty"`
}

// New creates an empty project config with version 1.
func New() *Config {
	return &Config{
		Version:   1,
		Include:   []string{},
		Overrides: make(map[string]string),
		Mappings:  make(map[string]string),
		Computed:  make(map[string]string),
	}
}

// Load loads the config for the current directory's project.
// It uses the registry to find which project the current directory belongs to,
// then loads that project's config from ~/.varnish/projects/<project>.yaml.
// Returns nil (not an error) if no project is registered for this directory.
func Load() (*Config, error) {
	reg, err := registry.Load()
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	proj := reg.LookupCurrent()
	if proj == "" {
		return nil, nil
	}

	return LoadByName(proj)
}

// LoadByName loads a project config by project name.
// Looks for ~/.varnish/projects/<project>.yaml
func LoadByName(name string) (*Config, error) {
	path := config.ProjectConfigPathFor(name)
	return LoadFrom(path)
}

// LoadFrom reads a project config from a specific path.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project config not found: %s", path)
		}
		return nil, fmt.Errorf("read project config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}

	// Ensure maps are initialized
	if cfg.Overrides == nil {
		cfg.Overrides = make(map[string]string)
	}
	if cfg.Mappings == nil {
		cfg.Mappings = make(map[string]string)
	}
	if cfg.Computed == nil {
		cfg.Computed = make(map[string]string)
	}

	return &cfg, nil
}

// Save writes the project config to ~/.varnish/projects/<project>.yaml.
// The project name must be set in the config.
func (c *Config) Save() error {
	if c.Project == "" {
		return fmt.Errorf("project name is required")
	}

	// Ensure projects directory exists
	if err := config.EnsureProjectsDir(); err != nil {
		return fmt.Errorf("create projects directory: %w", err)
	}

	path := config.ProjectConfigPathFor(c.Project)
	return c.SaveTo(path)
}

// SaveTo writes the project config to a specific path.
func (c *Config) SaveTo(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}

	// Use atomic write for safety
	if err := config.AtomicWrite(path, data, config.PermConfig); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	return nil
}

// Exists checks if a project config exists for the given name.
func Exists(name string) bool {
	path := config.ProjectConfigPathFor(name)
	_, err := os.Stat(path)
	return err == nil
}

// Delete removes a project's config file.
func Delete(name string) error {
	path := config.ProjectConfigPathFor(name)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
