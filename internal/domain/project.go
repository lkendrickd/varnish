// project.go manages the per-project config files stored in ~/.varnish/projects/.
//
// This file is used by:
//   - cli/init.go: to create a new project config
//   - cli/env.go: to know which variables the project needs
//   - cli/list.go: to show the project's variable requirements
//   - domain/resolver.go: to resolve variables from store → env vars
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
//
// Example ~/.varnish/projects/myapp.yaml:
//
//	version: 1
//	project: myapp
//	include:
//	  - database.*
//	  - log_level
//	overrides:
//	  database.name: myproject_dev
//	mappings:
//	  database.url: DATABASE_URL
//	computed:
//	  DATABASE_URL: "postgres://${database.user}@${database.host}/${database.name}"
//
// The project field namespaces variables in the store. When this project
// requests "database.host", the resolver looks up "myapp.database.host".
package domain

import (
	"fmt"
	"os"

	"github.com/dk/varnish/internal/config"
	"gopkg.in/yaml.v3"
)

// ProjectConfig holds the per-project configuration.
type ProjectConfig struct {
	Version   int               `yaml:"version"`
	Project   string            `yaml:"project,omitempty"`
	Include   []string          `yaml:"include,omitempty"`
	Overrides map[string]string `yaml:"overrides,omitempty"`
	Mappings  map[string]string `yaml:"mappings,omitempty"`
	Computed  map[string]string `yaml:"computed,omitempty"`
}

// NewProjectConfig creates an empty project config with version 1.
func NewProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Version:   1,
		Include:   []string{},
		Overrides: make(map[string]string),
		Mappings:  make(map[string]string),
		Computed:  make(map[string]string),
	}
}

// LoadProjectConfig loads the config for the current directory's project.
// It uses the registry to find which project the current directory belongs to,
// then loads that project's config from ~/.varnish/projects/<project>.yaml.
// Returns nil (not an error) if no project is registered for this directory.
func LoadProjectConfig() (*ProjectConfig, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	project := reg.LookupCurrent()
	if project == "" {
		return nil, nil
	}

	return LoadProjectConfigByName(project)
}

// LoadProjectConfigByName loads a project config by project name.
// Looks for ~/.varnish/projects/<project>.yaml
func LoadProjectConfigByName(project string) (*ProjectConfig, error) {
	path := config.ProjectConfigPathFor(project)
	return LoadProjectConfigFrom(path)
}

// LoadProjectConfigFrom reads a project config from a specific path.
func LoadProjectConfigFrom(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project config not found: %s", path)
		}
		return nil, fmt.Errorf("read project config: %w", err)
	}

	var cfg ProjectConfig
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
func (p *ProjectConfig) Save() error {
	if p.Project == "" {
		return fmt.Errorf("project name is required")
	}

	// Ensure projects directory exists
	if err := config.EnsureProjectsDir(); err != nil {
		return fmt.Errorf("create projects directory: %w", err)
	}

	path := config.ProjectConfigPathFor(p.Project)
	return p.SaveTo(path)
}

// SaveTo writes the project config to a specific path.
func (p *ProjectConfig) SaveTo(path string) error {
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}

	// Use atomic write for safety
	if err := config.AtomicWrite(path, data, config.PermConfig); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	return nil
}

// ProjectConfigExists checks if a project config exists for the given name.
func ProjectConfigExists(project string) bool {
	path := config.ProjectConfigPathFor(project)
	_, err := os.Stat(path)
	return err == nil
}

// DeleteProjectConfig removes a project's config file.
func DeleteProjectConfig(project string) error {
	path := config.ProjectConfigPathFor(project)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
