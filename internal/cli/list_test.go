package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/registry"
	"github.com/dk/varnish/internal/store"
)

func TestRunListBasic(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForList(t, "listtest")
	defer cleanupProject()

	// Add variables to store
	store, _ := store.Load()
	store.Set("listtest.test.var", "value123")
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runList([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runList error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "resolved variables") {
		t.Errorf("expected 'resolved variables' header, got: %s", output)
	}
	if !strings.Contains(output, "TEST_VAR") {
		t.Errorf("expected 'TEST_VAR' in output, got: %s", output)
	}
	if !strings.Contains(output, "value123") {
		t.Errorf("expected 'value123' in output, got: %s", output)
	}
}

func TestRunListJSON(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForList(t, "listjson")
	defer cleanupProject()

	store, _ := store.Load()
	store.Set("listjson.test.var", "jsonvalue")
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runList([]string{"--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runList --json error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "\"variables\"") {
		t.Errorf("expected JSON with 'variables' field, got: %s", output)
	}
	if !strings.Contains(output, "TEST_VAR") {
		t.Errorf("expected 'TEST_VAR' in JSON, got: %s", output)
	}
	if !strings.Contains(output, "jsonvalue") {
		t.Errorf("expected 'jsonvalue' in JSON, got: %s", output)
	}
}

func TestRunListMissing(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForList(t, "listmissing")
	defer cleanupProject()

	// Don't add any variables to store - they'll be missing

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runList([]string{"--missing"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runList --missing error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "missing variables") {
		t.Errorf("expected 'missing variables' in output, got: %s", output)
	}
}

func TestRunListMissingNone(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForList(t, "nomissing")
	defer cleanupProject()

	// Add all required variables
	store, _ := store.Load()
	store.Set("nomissing.test.var", "present")
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runList([]string{"--missing"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runList --missing error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "no missing variables") {
		t.Errorf("expected 'no missing variables', got: %s", output)
	}
}

func TestRunListMissingJSON(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForList(t, "missingjson")
	defer cleanupProject()

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runList([]string{"--missing", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runList --missing --json error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "\"missing\"") {
		t.Errorf("expected JSON with 'missing' field, got: %s", output)
	}
}

func TestRunListEmpty(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	// Create project with no include patterns
	reg, _ := registry.Load()
	reg.Register(projectDir, "emptylist")
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	cfg := project.New()
	cfg.Project = "emptylist"
	cfg.Include = []string{} // No patterns
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runList([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runList error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "no variables configured") {
		t.Errorf("expected 'no variables configured', got: %s", output)
	}
}

func TestRunListNoConfig(t *testing.T) {
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
	err = runList([]string{}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when no config exists")
	}
	if !strings.Contains(err.Error(), "varnish init") {
		t.Errorf("expected helpful error about 'varnish init', got: %v", err)
	}
}

func TestRunListShowsSource(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	reg, _ := registry.Load()
	reg.Register(projectDir, "sourcetest")
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	cfg := project.New()
	cfg.Project = "sourcetest"
	cfg.Include = []string{"db.*"}
	cfg.Overrides = map[string]string{"db.name": "override_db"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	store, _ := store.Load()
	store.Set("sourcetest.db.host", "localhost")
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runList([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runList error: %v", err)
	}

	output := stdout.String()
	// Should show source information
	if !strings.Contains(output, "store:") {
		t.Errorf("expected 'store:' source indicator, got: %s", output)
	}
	if !strings.Contains(output, "override:") {
		t.Errorf("expected 'override:' source indicator, got: %s", output)
	}
}

func TestFormatSource(t *testing.T) {
	tests := []struct {
		source   string
		key      string
		expected string
	}{
		{"store", "db.host", "store: db.host"},
		{"override", "db.name", "override: db.name"},
		{"computed", "", "computed"},
		{"unknown", "key", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := formatSource(tt.source, tt.key)
			if result != tt.expected {
				t.Errorf("formatSource(%q, %q) = %q, want %q", tt.source, tt.key, result, tt.expected)
			}
		})
	}
}

// setupProjectForList creates a project with include patterns for testing list command
func setupProjectForList(t *testing.T, projectName string) (string, func()) {
	t.Helper()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	envContent := "TEST_VAR=default\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	reg, _ := registry.Load()
	reg.Register(projectDir, projectName)
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	cfg := project.New()
	cfg.Project = projectName
	cfg.Include = []string{"test.*"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	return projectDir, func() {
		os.RemoveAll(projectDir)
	}
}
