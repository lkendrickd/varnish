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

func TestRunCheckBasic(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForCheck(t, "checktest")
	defer cleanupProject()

	st, _ := store.Load()
	st.Set("checktest.db.host", "localhost")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save store: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runCheck([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCheck error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected checkmarks in output, got: %s", output)
	}
	if !strings.Contains(output, "All checks passed") {
		t.Errorf("expected 'All checks passed', got: %s", output)
	}
}

func TestRunCheckShowsProjectName(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForCheck(t, "namedproject")
	defer cleanupProject()

	store, _ := store.Load()
	store.Set("namedproject.key", "value")
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runCheck([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCheck error: %v", err)
	}

	if !strings.Contains(stdout.String(), "namedproject") {
		t.Errorf("expected project name in output, got: %s", stdout.String())
	}
}

func TestRunCheckMissingVarsWarning(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForCheckWithRequired(t, "checkmissing")
	defer cleanupProject()

	// Add some but not all required variables
	store, _ := store.Load()
	store.Set("checkmissing.db.host", "localhost")
	// db.port is in the include but not set - should show as missing
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runCheck([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCheck error: %v", err)
	}

	// Check passes but shows warnings about missing vars
	output := stdout.String()
	if !strings.Contains(output, "All checks passed") {
		t.Errorf("expected 'All checks passed', got: %s", output)
	}
}

func TestRunCheckStrictMode(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForCheckWithRequired(t, "checkstrict")
	defer cleanupProject()

	// Add some but not all required variables
	store, _ := store.Load()
	store.Set("checkstrict.db.host", "localhost")
	// db.port is required but not set
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runCheck([]string{"--strict"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error in strict mode with missing variables")
		return
	}
	if !strings.Contains(err.Error(), "check failed") {
		t.Errorf("expected 'check failed' error, got: %v", err)
	}

	// Should show errors in stderr
	if !strings.Contains(stderr.String(), "Errors:") {
		t.Errorf("expected errors section in stderr, got: %s", stderr.String())
	}
}

func TestRunCheckStrictModeAllPresent(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, cleanupProject := setupProjectForCheck(t, "checkstrictok")
	defer cleanupProject()

	store, _ := store.Load()
	store.Set("checkstrictok.db.host", "localhost")
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runCheck([]string{"--strict"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCheck --strict error when all vars present: %v", err)
	}

	if !strings.Contains(stdout.String(), "All checks passed") {
		t.Errorf("expected 'All checks passed', got: %s", stdout.String())
	}
}

func TestRunCheckNoConfig(t *testing.T) {
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
	err = runCheck([]string{}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when no config exists")
	}
	if !strings.Contains(err.Error(), "varnish init") {
		t.Errorf("expected helpful error message, got: %v", err)
	}
}

func TestRunCheckNoIncludePatterns(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	reg, _ := registry.Load()
	reg.Register(projectDir, "noincludes")
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	cfg := project.New()
	cfg.Project = "noincludes"
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
	err = runCheck([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCheck error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Warnings:") || !strings.Contains(output, "no include patterns") {
		t.Errorf("expected warning about no include patterns, got: %s", output)
	}
}

func TestRunCheckComputedValues(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	reg, _ := registry.Load()
	reg.Register(projectDir, "checkcomputed")
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	cfg := project.New()
	cfg.Project = "checkcomputed"
	cfg.Include = []string{"db.*"}
	cfg.Computed = map[string]string{
		"DATABASE_URL": "postgres://${db.user}@${db.host}/${db.name}",
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	store, _ := store.Load()
	store.Set("checkcomputed.db.host", "localhost")
	store.Set("checkcomputed.db.user", "admin")
	store.Set("checkcomputed.db.name", "mydb")
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runCheck([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCheck error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "computed value(s) checked") {
		t.Errorf("expected computed values check message, got: %s", output)
	}
}

func TestRunCheckHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runCheck([]string{"-h"}, &stdout, &stderr)
	if err != nil {
		t.Errorf("runCheck -h error: %v", err)
	}
	// Help should be shown without error
}

func TestContainsUnresolvedVar(t *testing.T) {
	tests := []struct {
		template string
		resolved map[string]string
		expected bool
	}{
		{
			"postgres://${db.user}@${db.host}/${db.name}",
			map[string]string{"db.user": "u", "db.host": "h", "db.name": "n"},
			false,
		},
		{
			"postgres://${db.user}@${db.host}/${db.name}",
			map[string]string{"db.host": "h", "db.name": "n"},
			true, // missing db.user
		},
		{
			"simple string no vars",
			map[string]string{},
			false,
		},
		{
			"value is ${missing} here",
			map[string]string{},
			true,
		},
		{
			"prefix${found}suffix",
			map[string]string{"found": "value"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			result := containsUnresolvedVar(tt.template, tt.resolved)
			if result != tt.expected {
				t.Errorf("containsUnresolvedVar(%q, %v) = %v, want %v",
					tt.template, tt.resolved, result, tt.expected)
			}
		})
	}
}

// setupProjectForCheck creates a project for testing check command
func setupProjectForCheck(t *testing.T, projectName string) (string, func()) {
	t.Helper()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	reg, _ := registry.Load()
	reg.Register(projectDir, projectName)
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	cfg := project.New()
	cfg.Project = projectName
	cfg.Include = []string{"db.*"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	return projectDir, func() {
		os.RemoveAll(projectDir)
	}
}

// setupProjectForCheckWithRequired creates a project with specific required variables
func setupProjectForCheckWithRequired(t *testing.T, projectName string) (string, func()) {
	t.Helper()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	reg, _ := registry.Load()
	reg.Register(projectDir, projectName)
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	cfg := project.New()
	cfg.Project = projectName
	// Use specific keys instead of wildcards so we can test missing vars
	cfg.Include = []string{"db.host", "db.port"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	return projectDir, func() {
		os.RemoveAll(projectDir)
	}
}
