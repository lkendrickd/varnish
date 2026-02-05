package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dk/varnish/internal/config"
	"github.com/dk/varnish/internal/project"
	"github.com/dk/varnish/internal/store"
)

// setupTestEnv creates a temporary varnish environment for testing
func setupTestEnv(t *testing.T) (cleanup func()) {
	t.Helper()

	// Save original home
	origHome := os.Getenv("HOME")

	// Create temp home directory
	tmpHome, err := os.MkdirTemp("", "varnish-test-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}

	// Set HOME to temp directory
	os.Setenv("HOME", tmpHome)

	// Ensure varnish directories exist
	if err := config.EnsureVarnishDir(); err != nil {
		t.Fatalf("failed to ensure varnish dir: %v", err)
	}
	if err := config.EnsureProjectsDir(); err != nil {
		t.Fatalf("failed to ensure projects dir: %v", err)
	}

	return func() {
		os.Setenv("HOME", origHome)
		os.RemoveAll(tmpHome)
	}
}

func TestRunStoreHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := runStore([]string{}, &stdout, &stderr)
	if err != nil {
		t.Errorf("runStore() error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Usage:") {
		t.Error("expected usage info in output")
	}
	if !strings.Contains(output, "store") {
		t.Error("expected 'store' in output")
	}
}

func TestRunStoreSetGet(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	var stdout, stderr bytes.Buffer

	// Set a value (flags before positional args)
	err := runStore([]string{"set", "-g", "test.key", "testvalue"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore set error: %v", err)
	}

	if !strings.Contains(stdout.String(), "set test.key") {
		t.Errorf("expected confirmation, got: %s", stdout.String())
	}

	// Get the value
	stdout.Reset()
	stderr.Reset()

	err = runStore([]string{"get", "-g", "test.key"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore get error: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "testvalue" {
		t.Errorf("got value = %q, want 'testvalue'", strings.TrimSpace(stdout.String()))
	}
}

func TestRunStoreSetKeyValue(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	var stdout, stderr bytes.Buffer

	// Set using key=value syntax (flags before positional args)
	err := runStore([]string{"set", "-g", "test.keyval=myvalue"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore set error: %v", err)
	}

	// Get and verify
	stdout.Reset()
	err = runStore([]string{"get", "-g", "test.keyval"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore get error: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "myvalue" {
		t.Errorf("got value = %q, want 'myvalue'", strings.TrimSpace(stdout.String()))
	}
}

func TestRunStoreList(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a store with some values
	st := store.New()
	st.Set("project.db.host", "localhost")
	st.Set("project.db.port", "5432")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save store: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err := runStore([]string{"list", "-g"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore list error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "project.db.host=localhost") {
		t.Errorf("expected db.host in output, got: %s", output)
	}
	if !strings.Contains(output, "project.db.port=5432") {
		t.Errorf("expected db.port in output, got: %s", output)
	}
}

func TestRunStoreListJSON(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a store with some values
	st := store.New()
	st.Set("test.key", "value")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save store: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err := runStore([]string{"list", "-g", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore list --json error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "variables") {
		t.Errorf("expected JSON with 'variables' field, got: %s", output)
	}
	if !strings.Contains(output, "test.key") {
		t.Errorf("expected test.key in JSON output, got: %s", output)
	}
}

func TestRunStoreDelete(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a store with a value
	st := store.New()
	st.Set("todelete", "value")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save store: %v", err)
	}

	var stdout, stderr bytes.Buffer

	// Delete the key (flags before positional args)
	err := runStore([]string{"delete", "-g", "todelete"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore delete error: %v", err)
	}

	if !strings.Contains(stdout.String(), "deleted todelete") {
		t.Errorf("expected deletion confirmation, got: %s", stdout.String())
	}

	// Verify it's deleted
	stdout.Reset()
	stderr.Reset()

	err = runStore([]string{"get", "-g", "todelete"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when getting deleted key")
	}
}

func TestRunStoreDeleteNotFound(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	var stdout, stderr bytes.Buffer

	err := runStore([]string{"delete", "-g", "nonexistent"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when deleting non-existent key")
	}
}

func TestRunStoreAliases(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a store with values
	st := store.New()
	st.Set("alias.test", "value")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save store: %v", err)
	}

	var stdout, stderr bytes.Buffer

	// Test 'ls' alias for 'list'
	err := runStore([]string{"ls", "-g"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore ls error: %v", err)
	}

	if !strings.Contains(stdout.String(), "alias.test") {
		t.Errorf("expected alias.test in ls output, got: %s", stdout.String())
	}

	// Test 'rm' alias for 'delete'
	stdout.Reset()
	stderr.Reset()

	err = runStore([]string{"rm", "-g", "alias.test"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore rm error: %v", err)
	}

	if !strings.Contains(stdout.String(), "deleted") {
		t.Errorf("expected deletion confirmation, got: %s", stdout.String())
	}
}

func TestRunStoreImport(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a temp .env file
	tmpDir, err := os.MkdirTemp("", "varnish-import-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	envContent := `IMPORT_KEY1=value1
IMPORT_KEY2=value2
`
	envPath := filepath.Join(tmpDir, "import.env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err = runStore([]string{"import", "-g", envPath}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore import error: %v", err)
	}

	if !strings.Contains(stdout.String(), "imported") {
		t.Errorf("expected import confirmation, got: %s", stdout.String())
	}

	// Verify imported values
	stdout.Reset()
	err = runStore([]string{"get", "-g", "import.key1"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("failed to get imported key: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "value1" {
		t.Errorf("imported value = %q, want 'value1'", strings.TrimSpace(stdout.String()))
	}
}

func TestRunStoreUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := runStore([]string{"unknown"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}

	if !strings.Contains(stderr.String(), "unknown") {
		t.Errorf("expected 'unknown' in error output, got: %s", stderr.String())
	}
}

func TestRunStoreSetMissingKey(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := runStore([]string{"set"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when key is missing")
	}
}

func TestRunStoreSetMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := runStore([]string{"set", "-g", "key"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when value is missing")
	}
}

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Shell-style conversions
		{"DATABASE_HOST", "database.host"},
		{"API_KEY", "api.key"},
		{"DB_PASSWORD", "db.password"},
		{"REDIS_URL", "redis.url"},
		{"MY_APP_SECRET_KEY", "my.app.secret.key"},

		// Single words are lowercased (to match envNameToKey behavior)
		{"SIMPLE", "simple"},
		{"HOST", "host"},
		{"PORT", "port"},

		// Should NOT be converted (has lowercase)
		{"Database_Host", "Database_Host"},
		{"mixedCase_Key", "mixedCase_Key"},
		{"db_host", "db_host"},

		// Should NOT be converted (already dot notation)
		{"database.host", "database.host"},
		{"api.key", "api.key"},
		{"some.nested.key", "some.nested.key"},

		// Should NOT be converted (has dots)
		{"MY.KEY", "MY.KEY"},

		// With numbers
		{"DB2_HOST", "db2.host"},
		{"API_V2_KEY", "api.v2.key"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeKey(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRunStoreShellStyleKeys(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	var stdout, stderr bytes.Buffer

	// Set using shell-style key
	err := runStore([]string{"set", "-g", "DATABASE_HOST", "localhost"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore set error: %v", err)
	}

	if !strings.Contains(stdout.String(), "set database.host") {
		t.Errorf("expected 'set database.host', got: %s", stdout.String())
	}

	// Get using shell-style key
	stdout.Reset()
	stderr.Reset()

	err = runStore([]string{"get", "-g", "DATABASE_HOST"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore get error: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "localhost" {
		t.Errorf("got value = %q, want 'localhost'", strings.TrimSpace(stdout.String()))
	}

	// Get using dot notation (should return same value)
	stdout.Reset()
	stderr.Reset()

	err = runStore([]string{"get", "-g", "database.host"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore get error: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "localhost" {
		t.Errorf("got value = %q, want 'localhost'", strings.TrimSpace(stdout.String()))
	}

	// Delete using shell-style key
	stdout.Reset()
	stderr.Reset()

	err = runStore([]string{"delete", "-g", "DATABASE_HOST"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore delete error: %v", err)
	}

	if !strings.Contains(stdout.String(), "deleted database.host") {
		t.Errorf("expected 'deleted database.host', got: %s", stdout.String())
	}
}

func TestRunStoreShellStyleKeyValue(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	var stdout, stderr bytes.Buffer

	// Set using shell-style key=value syntax
	err := runStore([]string{"set", "-g", "API_KEY=my-api-key"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore set error: %v", err)
	}

	if !strings.Contains(stdout.String(), "set api.key") {
		t.Errorf("expected 'set api.key', got: %s", stdout.String())
	}

	// Get and verify
	stdout.Reset()
	err = runStore([]string{"get", "-g", "api.key"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore get error: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "my-api-key" {
		t.Errorf("got value = %q, want 'my-api-key'", strings.TrimSpace(stdout.String()))
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		// Wildcard all
		{"*", "anything", true},
		{"*", "database.host", true},
		{"*", "", true},

		// Prefix match
		{"database.*", "database.host", true},
		{"database.*", "database.port", true},
		{"database.*", "database.user.name", true},
		{"database.*", "api.key", false},
		{"database.*", "database", false},

		// Suffix match
		{"*.host", "database.host", true},
		{"*.host", "redis.host", true},
		{"*.host", "host", false},
		{"*.host", "database.port", false},

		// Exact match
		{"database.host", "database.host", true},
		{"database.host", "database.port", false},
		{"api.key", "api.key", true},
	}

	for _, tt := range tests {
		name := tt.pattern + "_" + tt.s
		t.Run(name, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.s)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
			}
		})
	}
}

func TestDetectProject(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Test when not in a registered directory
	proj := detectProject()
	if proj != "" {
		t.Errorf("detectProject() = %q, want empty when not registered", proj)
	}
}

func TestEnsureIncludePattern(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a project first
	cfg := &project.Config{
		Version:   1,
		Project:   "testproj",
		Include:   []string{},
		Overrides: make(map[string]string),
		Mappings:  make(map[string]string),
		Computed:  make(map[string]string),
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save project: %v", err)
	}

	var stdout bytes.Buffer

	// Add a pattern for a new key
	err := ensureIncludePattern("testproj", "db.host", &stdout)
	if err != nil {
		t.Fatalf("ensureIncludePattern() error: %v", err)
	}

	if !strings.Contains(stdout.String(), "added 'db.*'") {
		t.Errorf("expected 'added db.*' in output, got: %s", stdout.String())
	}

	// Reload and verify
	loaded, err := project.LoadByName("testproj")
	if err != nil {
		t.Fatalf("LoadByName() error: %v", err)
	}

	found := false
	for _, inc := range loaded.Include {
		if inc == "db.*" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'db.*' in includes, got: %v", loaded.Include)
	}

	// Adding the same pattern again should not duplicate
	stdout.Reset()
	err = ensureIncludePattern("testproj", "db.user", &stdout)
	if err != nil {
		t.Fatalf("ensureIncludePattern() error: %v", err)
	}

	// Should not add again (already covered by db.*)
	if strings.Contains(stdout.String(), "added") {
		t.Errorf("should not add duplicate pattern, got: %s", stdout.String())
	}
}

func TestEnsureIncludePatternSimpleKey(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a project
	cfg := &project.Config{
		Version:   1,
		Project:   "simpleproj",
		Include:   []string{},
		Overrides: make(map[string]string),
		Mappings:  make(map[string]string),
		Computed:  make(map[string]string),
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save project: %v", err)
	}

	var stdout bytes.Buffer

	// Add a simple key (no dots)
	err := ensureIncludePattern("simpleproj", "apikey", &stdout)
	if err != nil {
		t.Fatalf("ensureIncludePattern() error: %v", err)
	}

	// Reload and verify - should add "apikey" not ".*"
	loaded, err := project.LoadByName("simpleproj")
	if err != nil {
		t.Fatalf("LoadByName() error: %v", err)
	}

	found := false
	for _, inc := range loaded.Include {
		if inc == "apikey" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'apikey' in includes, got: %v", loaded.Include)
	}
}

func TestRunStoreEncrypt(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a store with some data
	st := store.New()
	st.Set("test.key", "secret-value")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save store: %v", err)
	}

	var stdout, stderr bytes.Buffer

	// Encrypt with --password flag
	err := runStore([]string{"encrypt", "--password", "testpassword"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore encrypt error: %v", err)
	}

	if !strings.Contains(stdout.String(), "encrypted") {
		t.Errorf("expected 'encrypted' in output, got: %s", stdout.String())
	}

	// Verify store is encrypted
	t.Setenv("VARNISH_PASSWORD", "testpassword")
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load encrypted store: %v", err)
	}
	if !loaded.IsEncrypted() {
		t.Error("store should be encrypted")
	}
	val, ok := loaded.Get("test.key")
	if !ok || val != "secret-value" {
		t.Errorf("Get(test.key) = %q, %v; want secret-value, true", val, ok)
	}
}

func TestRunStoreEncryptNoPassword(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	unsetenv(t, "VARNISH_PASSWORD")

	var stdout, stderr bytes.Buffer

	err := runStore([]string{"encrypt"}, &stdout, &stderr)
	if err == nil {
		t.Error("expected error when no password provided")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("expected error to mention password, got: %v", err)
	}
}

func TestRunStoreEncryptAlreadyEncrypted(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	t.Setenv("VARNISH_PASSWORD", "testpassword")

	// Create and encrypt a store
	st := store.New()
	st.Set("test.key", "value")
	if err := st.EnableEncryption(); err != nil {
		t.Fatalf("EnableEncryption() error: %v", err)
	}
	if err := st.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	var stdout, stderr bytes.Buffer

	// Try to encrypt again
	err := runStore([]string{"encrypt", "--password", "testpassword"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runStore encrypt error: %v", err)
	}

	if !strings.Contains(stdout.String(), "already encrypted") {
		t.Errorf("expected 'already encrypted' in output, got: %s", stdout.String())
	}
}
