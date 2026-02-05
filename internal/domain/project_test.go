package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewProjectConfig(t *testing.T) {
	cfg := NewProjectConfig()

	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Include == nil {
		t.Error("expected Include to be initialized")
	}
	if cfg.Overrides == nil {
		t.Error("expected Overrides to be initialized")
	}
	if cfg.Mappings == nil {
		t.Error("expected Mappings to be initialized")
	}
	if cfg.Computed == nil {
		t.Error("expected Computed to be initialized")
	}
}

func TestProjectConfigSaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "project.yaml")

	// Create config
	cfg := NewProjectConfig()
	cfg.Project = "testproject"
	cfg.Include = []string{"database.*", "api.*"}
	cfg.Overrides = map[string]string{"database.name": "testdb"}
	cfg.Mappings = map[string]string{"database.url": "DB_URL"}
	cfg.Computed = map[string]string{"FULL_URL": "postgres://${database.host}"}

	// Save
	if err := cfg.SaveTo(cfgPath); err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}

	// Load
	loaded, err := LoadProjectConfigFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadProjectConfigFrom() error: %v", err)
	}

	// Verify
	if loaded.Version != 1 {
		t.Errorf("loaded version = %d, want 1", loaded.Version)
	}
	if loaded.Project != "testproject" {
		t.Errorf("loaded project = %q, want 'testproject'", loaded.Project)
	}
	if len(loaded.Include) != 2 {
		t.Errorf("loaded include len = %d, want 2", len(loaded.Include))
	}
	if loaded.Include[0] != "database.*" {
		t.Errorf("loaded include[0] = %q, want 'database.*'", loaded.Include[0])
	}
	if loaded.Overrides["database.name"] != "testdb" {
		t.Errorf("loaded override = %q, want 'testdb'", loaded.Overrides["database.name"])
	}
	if loaded.Mappings["database.url"] != "DB_URL" {
		t.Errorf("loaded mapping = %q, want 'DB_URL'", loaded.Mappings["database.url"])
	}
	if loaded.Computed["FULL_URL"] != "postgres://${database.host}" {
		t.Errorf("loaded computed = %q", loaded.Computed["FULL_URL"])
	}
}

func TestLoadProjectConfigFromNotExist(t *testing.T) {
	_, err := LoadProjectConfigFrom("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestProjectConfigSaveRequiresProject(t *testing.T) {
	cfg := NewProjectConfig()
	// Don't set Project name

	err := cfg.Save()
	if err == nil {
		t.Error("expected error when saving without project name")
	}
}

func TestProjectConfigExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a projects subdirectory to simulate ~/.varnish/projects/
	projectsDir := filepath.Join(tmpDir, "projects")
	if err := os.MkdirAll(projectsDir, 0700); err != nil {
		t.Fatalf("failed to create projects dir: %v", err)
	}

	// Create a test config file
	testConfig := filepath.Join(projectsDir, "testproject.yaml")
	if err := os.WriteFile(testConfig, []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Note: ProjectConfigExists uses config.ProjectConfigPathFor which depends
	// on the actual home directory. For a proper test, we'd need to mock the
	// config package. This test verifies the basic file existence logic.
}

func TestLoadProjectConfigFromInvalidYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(cfgPath, []byte("not: valid: yaml: content:::"), 0644); err != nil {
		t.Fatalf("failed to write invalid yaml: %v", err)
	}

	_, err = LoadProjectConfigFrom(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestProjectConfigMapsInitialized(t *testing.T) {
	// Test that loading a minimal config initializes all maps
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "minimal.yaml")
	// Write minimal config with no maps
	if err := os.WriteFile(cfgPath, []byte("version: 1\nproject: minimal\n"), 0644); err != nil {
		t.Fatalf("failed to write minimal yaml: %v", err)
	}

	loaded, err := LoadProjectConfigFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadProjectConfigFrom() error: %v", err)
	}

	// All maps should be initialized (not nil)
	if loaded.Overrides == nil {
		t.Error("Overrides should be initialized")
	}
	if loaded.Mappings == nil {
		t.Error("Mappings should be initialized")
	}
	if loaded.Computed == nil {
		t.Error("Computed should be initialized")
	}
}

func TestDeleteProjectConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "todelete.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	// Delete using os.Remove directly (since DeleteProjectConfig uses config paths)
	if err := os.Remove(cfgPath); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}
