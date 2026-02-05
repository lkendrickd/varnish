// Package registry manages the mapping of directories to project names.
//
// The registry lives at ~/.varnish/registry.yaml and maps absolute
// directory paths to project names. This allows varnish to know which
// project config to use based on the current working directory.
package registry

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/dk/varnish/internal/config"
	"gopkg.in/yaml.v3"
)

// Registry maps directory paths to project names.
type Registry struct {
	Version  int               `yaml:"version"`
	Projects map[string]string `yaml:"projects"` // dir path -> project name
}

// New creates an empty registry with version 1.
func New() *Registry {
	return &Registry{
		Version:  1,
		Projects: make(map[string]string),
	}
}

// Load loads the registry from ~/.varnish/registry.yaml.
// Returns an empty registry if the file doesn't exist.
func Load() (*Registry, error) {
	path := config.RegistryPath()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, err
	}

	if reg.Projects == nil {
		reg.Projects = make(map[string]string)
	}

	return &reg, nil
}

// Save writes the registry to ~/.varnish/registry.yaml.
func (r *Registry) Save() error {
	if err := config.EnsureVarnishDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}

	return config.AtomicWrite(config.RegistryPath(), data, config.PermConfig)
}

// Register associates a directory with a project name.
func (r *Registry) Register(dir, project string) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	r.Projects[absDir] = project
}

// Unregister removes a directory from the registry.
func (r *Registry) Unregister(dir string) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	delete(r.Projects, absDir)
}

// Lookup finds the project name for a directory.
// It checks the directory and all parent directories.
// Returns empty string if not found.
func (r *Registry) Lookup(dir string) string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	// Check exact match first
	if project, ok := r.Projects[absDir]; ok {
		return project
	}

	// Check parent directories
	for {
		parent := filepath.Dir(absDir)
		if parent == absDir {
			break // reached root
		}
		if project, ok := r.Projects[parent]; ok {
			return project
		}
		absDir = parent
	}

	return ""
}

// LookupCurrent finds the project for the current working directory.
func (r *Registry) LookupCurrent() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return r.Lookup(cwd)
}

// ProjectDirs returns all directories registered for a project.
func (r *Registry) ProjectDirs(project string) []string {
	var dirs []string
	for dir, p := range r.Projects {
		if p == project {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs
}

// AllProjects returns a sorted list of unique project names.
func (r *Registry) AllProjects() []string {
	seen := make(map[string]bool)
	for _, project := range r.Projects {
		seen[project] = true
	}

	projects := make([]string, 0, len(seen))
	for project := range seen {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	return projects
}
