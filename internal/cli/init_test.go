package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dk/varnish/internal/crypto"
	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/store"
)

// unsetenv removes an env var and registers cleanup to restore it.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	orig, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if exists {
			os.Setenv(key, orig)
		}
	})
}

func TestRunInitBasic(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a temp project directory with a .env file
	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	// Create .env file
	envContent := "DB_HOST=localhost\nDB_PORT=5432\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Change to project directory
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runInit([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit error: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "parsed 2 variables") {
		t.Errorf("expected 'parsed 2 variables' in output, got: %s", output)
	}
	if !strings.Contains(output, "registered") {
		t.Errorf("expected 'registered' in output, got: %s", output)
	}
}

func TestRunInitWithProjectName(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	envContent := "APP_KEY=secret\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runInit([]string{"--project", "myapp"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit error: %v", err)
	}

	if !strings.Contains(stdout.String(), "myapp") {
		t.Errorf("expected 'myapp' in output, got: %s", stdout.String())
	}

	// Verify project config was created
	cfg, err := project.LoadByName("myapp")
	if err != nil {
		t.Fatalf("failed to load project config: %v", err)
	}
	if cfg.Project != "myapp" {
		t.Errorf("project name = %q, want 'myapp'", cfg.Project)
	}
}

func TestRunInitFromExampleEnv(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	// Create only example.env, not .env
	envContent := "API_URL=http://localhost\nAPI_KEY=\n"
	if err := os.WriteFile(filepath.Join(projectDir, "example.env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write example.env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runInit([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit error: %v", err)
	}

	if !strings.Contains(stdout.String(), "example.env") {
		t.Errorf("expected 'example.env' in output, got: %s", stdout.String())
	}
}

func TestRunInitNoEnvFile(t *testing.T) {
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
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runInit([]string{}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when no .env file exists")
	}

	if !strings.Contains(stderr.String(), "no .env") {
		t.Errorf("expected helpful error message, got: %s", stderr.String())
	}
}

func TestRunInitWithFromFlag(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	// Create custom .env file with different name
	envContent := "CUSTOM_VAR=value\n"
	customEnvPath := filepath.Join(projectDir, "config.env")
	if err := os.WriteFile(customEnvPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write config.env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runInit([]string{"-f", "config.env"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit error: %v", err)
	}

	if !strings.Contains(stdout.String(), "config.env") {
		t.Errorf("expected 'config.env' in output, got: %s", stdout.String())
	}
}

func TestRunInitAlreadyExists(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	envContent := "VAR=value\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer

	// First init
	err = runInit([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("first init error: %v", err)
	}

	// Second init without --force should fail
	stdout.Reset()
	stderr.Reset()
	err = runInit([]string{}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error on second init without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestRunInitForce(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	envContent := "VAR=value\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer

	// First init
	err = runInit([]string{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("first init error: %v", err)
	}

	// Second init with --force should succeed
	stdout.Reset()
	stderr.Reset()
	err = runInit([]string{"--force"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("init --force error: %v", err)
	}
}

func TestRunInitSync(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	// Initial .env with two vars
	envContent := "VAR1=value1\nVAR2=value2\n"
	envPath := filepath.Join(projectDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer

	// First init
	err = runInit([]string{"--project", "synctest"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("first init error: %v", err)
	}

	// Update .env to remove VAR2
	envContent = "VAR1=updated\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to update .env: %v", err)
	}

	// Re-init with --sync --force
	stdout.Reset()
	stderr.Reset()
	err = runInit([]string{"--project", "synctest", "--sync", "--force"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("init --sync error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "removed") {
		t.Errorf("expected 'removed' in sync output, got: %s", output)
	}
}

func TestRunInitNoImport(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	envContent := "IMPORTED_VAR=should_not_be_stored\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runInit([]string{"--project", "noimport", "--no-import"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit error: %v", err)
	}

	// Verify value was NOT imported into store
	store, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load store: %v", err)
	}

	if _, exists := store.Get("noimport.imported.var"); exists {
		t.Error("variable should not have been imported with --no-import")
	}
}

func TestRunInitEncrypt(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	t.Setenv(crypto.PasswordEnvVar, "testpassword")

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	envContent := "SECRET_KEY=mysecret\nAPI_TOKEN=token123\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runInit([]string{"--project", "enctest", "--encrypt"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit error: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "encryption enabled") {
		t.Errorf("expected 'encryption enabled' in output, got: %s", output)
	}

	// Verify store is encrypted
	st, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load store: %v", err)
	}

	if !st.IsEncrypted() {
		t.Error("store should be encrypted after init --encrypt")
	}

	// Verify values were still imported
	val, ok := st.Get("enctest.secret.key")
	if !ok || val != "mysecret" {
		t.Errorf("encrypted store secret.key = %q, ok=%v, want 'mysecret'", val, ok)
	}
}

func TestRunInitEncryptWithPasswordFlag(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Ensure no password env var - we'll use --password flag instead
	unsetenv(t, crypto.PasswordEnvVar)

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	envContent := "SECRET_KEY=mysecret\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runInit([]string{"--project", "pwdtest", "--encrypt", "--password", "mypassword"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInit error: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "encryption enabled") {
		t.Errorf("expected 'encryption enabled' in output, got: %s", output)
	}

	// Verify store is encrypted (need to set password to load)
	t.Setenv(crypto.PasswordEnvVar, "mypassword")
	st, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load store: %v", err)
	}
	if !st.IsEncrypted() {
		t.Error("store should be encrypted")
	}
}

func TestRunInitEncryptNoPassword(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	unsetenv(t, crypto.PasswordEnvVar)

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	envContent := "VAR=value\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = runInit([]string{"--project", "nopasstest", "--encrypt"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when using --encrypt without VARNISH_PASSWORD")
	}

	if !strings.Contains(err.Error(), "password") && !strings.Contains(err.Error(), "VARNISH_PASSWORD") {
		t.Errorf("error should mention password, got: %v", err)
	}
}

func TestRunInitEncryptAlreadyEncrypted(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	t.Setenv(crypto.PasswordEnvVar, "testpassword")

	projectDir, err := os.MkdirTemp("", "varnish-project-*")
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	defer os.RemoveAll(projectDir)

	envContent := "VAR=value\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer

	// First init with --encrypt
	err = runInit([]string{"--project", "alreadyenc", "--encrypt"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("first init error: %v", err)
	}

	// Second init with --encrypt --force should succeed but not print "encryption enabled" again
	stdout.Reset()
	stderr.Reset()

	// Update .env to trigger a change
	envContent = "VAR=updated\n"
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to update .env: %v", err)
	}

	err = runInit([]string{"--project", "alreadyenc", "--encrypt", "--force"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("second init error: %v", err)
	}

	output := stdout.String()
	// Should not say "encryption enabled" since it was already encrypted
	if strings.Contains(output, "encryption enabled") {
		t.Errorf("should not say 'encryption enabled' when already encrypted, got: %s", output)
	}

	// Store should still be encrypted
	st, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load store: %v", err)
	}

	if !st.IsEncrypted() {
		t.Error("store should still be encrypted")
	}
}
