package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/registry"
	"github.com/dk/varnish/internal/store"
)

func TestRunProjectHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := runProject([]string{"help"}, &stdout, &stderr)
	if err != nil {
		t.Errorf("runProject help error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Usage:") {
		t.Error("expected usage info in help output")
	}
	if !strings.Contains(output, "list") {
		t.Error("expected 'list' subcommand in help")
	}
	if !strings.Contains(output, "delete") {
		t.Error("expected 'delete' subcommand in help")
	}
}

func TestRunProjectName(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create and register a project
	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	// Register the project
	reg, _ := registry.Load()
	reg.Register(projectDir, "testproj")
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Create project config
	cfg := project.New()
	cfg.Project = "testproj"
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Change to project directory
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runProject([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject error: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "testproj" {
		t.Errorf("project name = %q, want 'testproj'", strings.TrimSpace(stdout.String()))
	}
}

func TestRunProjectNameWithPath(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	reg, _ := registry.Load()
	reg.Register(projectDir, "pathtest")
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	cfg := project.New()
	cfg.Project = "pathtest"
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runProject([]string{"--path"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject --path error: %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.Contains(output, "pathtest.yaml") {
		t.Errorf("expected path to contain 'pathtest.yaml', got: %s", output)
	}
}

func TestRunProjectNotRegistered(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runProject([]string{}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error for unregistered directory")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("expected 'not registered' error, got: %v", err)
	}
}

func TestRunProjectList(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create store with multiple projects
	st := store.New()
	st.Set("alpha.db.host", "localhost")
	st.Set("alpha.db.port", "5432")
	st.Set("beta.api.key", "secret")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runProject([]string{"list"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject list error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "alpha") {
		t.Errorf("expected 'alpha' in list output, got: %s", output)
	}
	if !strings.Contains(output, "beta") {
		t.Errorf("expected 'beta' in list output, got: %s", output)
	}
	if !strings.Contains(output, "2 variables") {
		t.Errorf("expected '2 variables' for alpha, got: %s", output)
	}
}

func TestRunProjectListEmpty(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	var stdout, stderr bytes.Buffer
	err := runProject([]string{"list"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject list error: %v", err)
	}

	if !strings.Contains(stderr.String(), "no projects found") {
		t.Errorf("expected 'no projects found' message, got stdout: %s, stderr: %s", stdout.String(), stderr.String())
	}
}

func TestRunProjectDelete(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create store with a project
	st := store.New()
	st.Set("deleteme.var1", "value1")
	st.Set("deleteme.var2", "value2")
	st.Set("keepme.var1", "value1")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runProject([]string{"delete", "deleteme"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject delete error: %v", err)
	}

	if !strings.Contains(stdout.String(), "deleted 2 variables") {
		t.Errorf("expected 'deleted 2 variables', got: %s", stdout.String())
	}

	// Verify deletion
	st, _ = store.Load()
	if _, exists := st.Get("deleteme.var1"); exists {
		t.Error("deleteme.var1 should have been deleted")
	}
	if _, exists := st.Get("keepme.var1"); !exists {
		t.Error("keepme.var1 should still exist")
	}
}

func TestRunProjectDeleteDryRun(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	st := store.New()
	st.Set("dryrun.var1", "value1")
	st.Set("dryrun.var2", "value2")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runProject([]string{"delete", "--dry-run", "dryrun"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject delete --dry-run error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "would delete") {
		t.Errorf("expected 'would delete' in dry-run output, got: %s", output)
	}

	// Verify NOT deleted
	st, _ = store.Load()
	if _, exists := st.Get("dryrun.var1"); !exists {
		t.Error("dryrun.var1 should still exist after dry-run")
	}
}

func TestRunProjectDeleteNotFound(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	var stdout, stderr bytes.Buffer
	err := runProject([]string{"delete", "nonexistent"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when deleting non-existent project")
	}
	if !strings.Contains(err.Error(), "no variables found") {
		t.Errorf("expected 'no variables found' error, got: %v", err)
	}
}

func TestRunProjectDeleteNoName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runProject([]string{"delete"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when project name not provided")
	}
}

func TestRunProjectUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runProject([]string{"unknown"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected 'unknown' in error, got: %v", err)
	}
}

func TestRunProjectListWithRegistry(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a project directory
	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	// Register it
	reg, _ := registry.Load()
	reg.Register(projectDir, "registered")
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Add to store
	st := store.New()
	st.Set("registered.key", "value")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runProject([]string{"list"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject list error: %v", err)
	}

	output := stdout.String()
	// Should show the directory path
	if !strings.Contains(output, "registered") {
		t.Errorf("expected 'registered' in output, got: %s", output)
	}
	// Should show the arrow indicating directory mapping
	if !strings.Contains(output, "→") {
		t.Errorf("expected '→' showing directory mapping, got: %s", output)
	}
}

func TestRunProjectDeleteCleansRegistry(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	// Register and create config
	reg, _ := registry.Load()
	reg.Register(projectDir, "toclean")
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	cfg := project.New()
	cfg.Project = "toclean"
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Add to store
	st := store.New()
	st.Set("toclean.key", "value")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runProject([]string{"delete", "toclean"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject delete error: %v", err)
	}

	// Verify registry was cleaned
	reg, _ = registry.Load()
	if reg.Lookup(projectDir) != "" {
		t.Error("directory should have been unregistered")
	}

	// Verify config was deleted
	if project.Exists("toclean") {
		t.Error("project config should have been deleted")
	}
}

func TestResolveProjectRef(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Test with name (not a number)
	name, err := resolveProjectRef("myproject")
	if err != nil {
		t.Fatalf("resolveProjectRef(name) error: %v", err)
	}
	if name != "myproject" {
		t.Errorf("resolveProjectRef(name) = %q, want 'myproject'", name)
	}

	// Test with empty store (no projects)
	_, err = resolveProjectRef("1")
	if err == nil {
		t.Error("expected error when no projects exist")
	}
	if !strings.Contains(err.Error(), "no projects found") {
		t.Errorf("expected 'no projects found' error, got: %v", err)
	}

	// Create some projects in store
	st := store.New()
	st.Set("alpha.key", "value")
	st.Set("beta.key", "value")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save store: %v", err)
	}

	// Test with valid ID
	name, err = resolveProjectRef("1")
	if err != nil {
		t.Fatalf("resolveProjectRef(1) error: %v", err)
	}
	if name != "alpha" {
		t.Errorf("resolveProjectRef(1) = %q, want 'alpha'", name)
	}

	name, err = resolveProjectRef("2")
	if err != nil {
		t.Fatalf("resolveProjectRef(2) error: %v", err)
	}
	if name != "beta" {
		t.Errorf("resolveProjectRef(2) = %q, want 'beta'", name)
	}

	// Test with invalid ID (out of range)
	_, err = resolveProjectRef("0")
	if err == nil {
		t.Error("expected error for ID 0")
	}

	_, err = resolveProjectRef("3")
	if err == nil {
		t.Error("expected error for ID out of range")
	}
}

func TestResolveProjectFlag(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Test with global flag (empty project name)
	name, err := resolveProjectFlag("", true)
	if err != nil {
		t.Fatalf("resolveProjectFlag(global) error: %v", err)
	}
	if name != "" {
		t.Errorf("resolveProjectFlag(global) = %q, want empty", name)
	}

	// Test with explicit project name
	name, err = resolveProjectFlag("myapp", false)
	if err != nil {
		t.Fatalf("resolveProjectFlag(name) error: %v", err)
	}
	if name != "myapp" {
		t.Errorf("resolveProjectFlag(name) = %q, want 'myapp'", name)
	}

	// Test with numeric project ref
	st := store.New()
	st.Set("testproj.key", "value")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save store: %v", err)
	}

	name, err = resolveProjectFlag("1", false)
	if err != nil {
		t.Fatalf("resolveProjectFlag(1) error: %v", err)
	}
	if name != "testproj" {
		t.Errorf("resolveProjectFlag(1) = %q, want 'testproj'", name)
	}
}
