package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	cfg := New()

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

func TestSaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "project.yaml")

	// Create config
	cfg := New()
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
	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
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

func TestLoadFromNotExist(t *testing.T) {
	_, err := LoadFrom("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestSaveRequiresProject(t *testing.T) {
	cfg := New()
	// Don't set Project name

	err := cfg.Save()
	if err == nil {
		t.Error("expected error when saving without project name")
	}
}

func TestLoadFromInvalidYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(cfgPath, []byte("not: valid: yaml: content:::"), 0644); err != nil {
		t.Fatalf("failed to write invalid yaml: %v", err)
	}

	_, err = LoadFrom(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestMapsInitialized(t *testing.T) {
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

	loaded, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
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

func TestSaveWithRealPath(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	cfg := New()
	cfg.Project = "testproj"
	cfg.Include = []string{"db.*"}
	cfg.Overrides["db.host"] = "localhost"

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file was created
	projectsDir := filepath.Join(tmpHome, ".varnish", "projects")
	expectedPath := filepath.Join(projectsDir, "testproj.yaml")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected file at %s: %v", expectedPath, err)
	}
}

func TestLoadByName(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	// Create and save a project
	cfg := New()
	cfg.Project = "myproject"
	cfg.Include = []string{"api.*"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Load by name
	loaded, err := LoadByName("myproject")
	if err != nil {
		t.Fatalf("LoadByName() error: %v", err)
	}

	if loaded.Project != "myproject" {
		t.Errorf("loaded.Project = %q, want 'myproject'", loaded.Project)
	}
	if len(loaded.Include) != 1 || loaded.Include[0] != "api.*" {
		t.Errorf("loaded.Include = %v, want [api.*]", loaded.Include)
	}
}

func TestLoadByNameNotFound(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	_, err = LoadByName("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent project")
	}
}

func TestExists(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	// Before creating
	if Exists("testexists") {
		t.Error("Exists() should return false for non-existent project")
	}

	// Create the project
	cfg := New()
	cfg.Project = "testexists"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// After creating
	if !Exists("testexists") {
		t.Error("Exists() should return true after creating project")
	}
}

func TestDelete(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	// Create a project
	cfg := New()
	cfg.Project = "todelete"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if !Exists("todelete") {
		t.Fatal("project should exist after Save()")
	}

	// Delete
	if err := Delete("todelete"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if Exists("todelete") {
		t.Error("project should not exist after Delete()")
	}

	// Delete non-existent (should not error)
	if err := Delete("nonexistent"); err != nil {
		t.Errorf("Delete() non-existent should not error: %v", err)
	}
}

func TestLoad(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Create a project directory
	projectDir := filepath.Join(tmpHome, "myproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	t.Setenv("HOME", tmpHome)

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	// Test 1: No project registered - should return nil, nil
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg != nil {
		t.Error("Load() should return nil when no project registered")
	}

	// Test 2: Register project and create config
	// First, create the registry
	regDir := filepath.Join(tmpHome, ".varnish")
	if err := os.MkdirAll(regDir, 0700); err != nil {
		t.Fatalf("failed to create varnish dir: %v", err)
	}
	regContent := "version: 1\nprojects:\n    " + projectDir + ": loadtest\n"
	if err := os.WriteFile(filepath.Join(regDir, "registry.yaml"), []byte(regContent), 0644); err != nil {
		t.Fatalf("failed to write registry: %v", err)
	}

	// Create project config
	projCfg := New()
	projCfg.Project = "loadtest"
	projCfg.Include = []string{"test.*"}
	if err := projCfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Now Load() should find the project
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() should return config when project registered")
	}
	if loaded.Project != "loadtest" {
		t.Errorf("loaded.Project = %q, want 'loadtest'", loaded.Project)
	}
}
