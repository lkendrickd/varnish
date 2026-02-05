package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dk/varnish/internal/domain"
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
	reg, _ := domain.LoadRegistry()
	reg.Register(projectDir, "testproj")
	reg.Save()

	// Create project config
	cfg := domain.NewProjectConfig()
	cfg.Project = "testproj"
	cfg.Save()

	// Change to project directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(projectDir)

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

	reg, _ := domain.LoadRegistry()
	reg.Register(projectDir, "pathtest")
	reg.Save()

	cfg := domain.NewProjectConfig()
	cfg.Project = "pathtest"
	cfg.Save()

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(projectDir)

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
	defer os.Chdir(origWd)
	os.Chdir(projectDir)

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
	store := domain.NewStore()
	store.Set("alpha.db.host", "localhost")
	store.Set("alpha.db.port", "5432")
	store.Set("beta.api.key", "secret")
	store.Save()

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
	store := domain.NewStore()
	store.Set("deleteme.var1", "value1")
	store.Set("deleteme.var2", "value2")
	store.Set("keepme.var1", "value1")
	store.Save()

	var stdout, stderr bytes.Buffer
	err := runProject([]string{"delete", "deleteme"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject delete error: %v", err)
	}

	if !strings.Contains(stdout.String(), "deleted 2 variables") {
		t.Errorf("expected 'deleted 2 variables', got: %s", stdout.String())
	}

	// Verify deletion
	store, _ = domain.LoadStore()
	if _, exists := store.Get("deleteme.var1"); exists {
		t.Error("deleteme.var1 should have been deleted")
	}
	if _, exists := store.Get("keepme.var1"); !exists {
		t.Error("keepme.var1 should still exist")
	}
}

func TestRunProjectDeleteDryRun(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	store := domain.NewStore()
	store.Set("dryrun.var1", "value1")
	store.Set("dryrun.var2", "value2")
	store.Save()

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
	store, _ = domain.LoadStore()
	if _, exists := store.Get("dryrun.var1"); !exists {
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
	reg, _ := domain.LoadRegistry()
	reg.Register(projectDir, "registered")
	reg.Save()

	// Add to store
	store := domain.NewStore()
	store.Set("registered.key", "value")
	store.Save()

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
	reg, _ := domain.LoadRegistry()
	reg.Register(projectDir, "toclean")
	reg.Save()

	cfg := domain.NewProjectConfig()
	cfg.Project = "toclean"
	cfg.Save()

	// Add to store
	store := domain.NewStore()
	store.Set("toclean.key", "value")
	store.Save()

	var stdout, stderr bytes.Buffer
	err = runProject([]string{"delete", "toclean"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProject delete error: %v", err)
	}

	// Verify registry was cleaned
	reg, _ = domain.LoadRegistry()
	if reg.Lookup(projectDir) != "" {
		t.Error("directory should have been unregistered")
	}

	// Verify config was deleted
	if domain.ProjectConfigExists("toclean") {
		t.Error("project config should have been deleted")
	}
}

func setupProjectWithEnv(t *testing.T, projectName string) (string, func()) {
	t.Helper()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	envContent := "TEST_VAR=value\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Register project
	reg, _ := domain.LoadRegistry()
	reg.Register(projectDir, projectName)
	reg.Save()

	// Create config
	cfg := domain.NewProjectConfig()
	cfg.Project = projectName
	cfg.Include = []string{"test.*"}
	cfg.Save()

	return projectDir, func() {
		os.RemoveAll(projectDir)
	}
}
