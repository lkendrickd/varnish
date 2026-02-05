package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dk/varnish/internal/domain"
)

func TestRunExportBasic(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForExport(t, "exporttest")
	defer cleanupProject()

	store, _ := domain.LoadStore()
	store.Set("exporttest.db.host", "localhost")
	store.Set("exporttest.db.port", "5432")
	store.Save()

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(projectDir)

	var stdout, stderr bytes.Buffer
	err := runExport([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runExport error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "export DB_HOST=localhost") {
		t.Errorf("expected 'export DB_HOST=localhost', got: %s", output)
	}
	if !strings.Contains(output, "export DB_PORT=5432") {
		t.Errorf("expected 'export DB_PORT=5432', got: %s", output)
	}
}

func TestRunExportHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runExport([]string{"-h"}, &stdout, &stderr)
	if err != nil {
		t.Errorf("runExport -h error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "eval $(varnish export)") {
		t.Errorf("expected usage examples in help, got: %s", output)
	}
}

func TestRunExportNoConfig(t *testing.T) {
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
	err = runExport([]string{}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when no config exists")
	}
}

func TestRunExportMissingVarsWarning(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForExportWithRequired(t, "exportmissing")
	defer cleanupProject()

	// Add some but not all required variables
	store, _ := domain.LoadStore()
	store.Set("exportmissing.db.host", "localhost")
	// db.port is required but not set
	store.Save()

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(projectDir)

	var stdout, stderr bytes.Buffer
	err := runExport([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runExport error: %v", err)
	}

	// Warning goes to stderr as a comment
	if !strings.Contains(stderr.String(), "missing") {
		t.Errorf("expected warning about missing variables, stderr: %s", stderr.String())
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with_underscore", "with_underscore"},
		{"with.dot", "with.dot"},
		{"with/slash", "with/slash"},
		{"with:colon", "with:colon"},
		{"CamelCase123", "CamelCase123"},
		{"with space", "'with space'"},
		{"with'quote", "'with'\\''quote'"},
		{"with\"doublequote", "'with\"doublequote'"},
		{"with$dollar", "'with$dollar'"},
		{"with`backtick", "'with`backtick'"},
		{"", "''"},
		{"has\ttab", "'has\ttab'"},
		{"has\nnewline", "'has\nnewline'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := shellQuote(tt.input)
			if result != tt.expected {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRunExportQuotesSpecialChars(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForExport(t, "exportquote")
	defer cleanupProject()

	store, _ := domain.LoadStore()
	store.Set("exportquote.db.password", "pass'word$pecial")
	store.Save()

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(projectDir)

	var stdout, stderr bytes.Buffer
	err := runExport([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runExport error: %v", err)
	}

	output := stdout.String()
	// Should be properly quoted for shell
	if !strings.Contains(output, "export DB_PASSWORD=") {
		t.Errorf("expected 'export DB_PASSWORD=', got: %s", output)
	}
	// The value should be quoted
	if !strings.Contains(output, "'") {
		t.Errorf("expected quoted value for special characters, got: %s", output)
	}
}

func TestRunExportEmptyStore(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	reg, _ := domain.LoadRegistry()
	reg.Register(projectDir, "emptyexport")
	reg.Save()

	cfg := domain.NewProjectConfig()
	cfg.Project = "emptyexport"
	cfg.Include = []string{} // No patterns
	cfg.Save()

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(projectDir)

	var stdout, stderr bytes.Buffer
	err = runExport([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runExport error: %v", err)
	}

	// Should produce no export statements (empty output)
	output := strings.TrimSpace(stdout.String())
	if output != "" {
		t.Errorf("expected empty output for no variables, got: %s", output)
	}
}

// setupProjectForExport creates a project for testing export command
func setupProjectForExport(t *testing.T, projectName string) (string, func()) {
	t.Helper()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	reg, _ := domain.LoadRegistry()
	reg.Register(projectDir, projectName)
	reg.Save()

	cfg := domain.NewProjectConfig()
	cfg.Project = projectName
	cfg.Include = []string{"db.*", "api.*"}
	cfg.Save()

	return projectDir, func() {
		os.RemoveAll(projectDir)
	}
}

// setupProjectForExportWithRequired creates a project with specific required variables
func setupProjectForExportWithRequired(t *testing.T, projectName string) (string, func()) {
	t.Helper()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	reg, _ := domain.LoadRegistry()
	reg.Register(projectDir, projectName)
	reg.Save()

	cfg := domain.NewProjectConfig()
	cfg.Project = projectName
	// Use specific keys so we can test missing vars
	cfg.Include = []string{"db.host", "db.port"}
	cfg.Save()

	return projectDir, func() {
		os.RemoveAll(projectDir)
	}
}
