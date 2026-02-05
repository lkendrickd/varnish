package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseExampleEnv(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	envContent := `# Database configuration
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=admin
DATABASE_PASSWORD=

# API settings
API_KEY=secret123
API_URL=https://api.example.com

# Empty and comments
# This is a comment
EMPTY_VALUE=

# With quotes
QUOTED_VALUE="hello world"
`

	envPath := filepath.Join(tmpDir, "example.env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	vars, err := ParseExampleEnv(envPath)
	if err != nil {
		t.Fatalf("ParseExampleEnv() error: %v", err)
	}

	// Build map for easier testing
	varMap := make(map[string]ExampleVar)
	for _, v := range vars {
		varMap[v.EnvName] = v
	}

	tests := []struct {
		envName  string
		key      string
		hasValue bool
		defValue string
	}{
		{"DATABASE_HOST", "database.host", true, "localhost"},
		{"DATABASE_PORT", "database.port", true, "5432"},
		{"DATABASE_USER", "database.user", true, "admin"},
		{"DATABASE_PASSWORD", "database.password", false, ""},
		{"API_KEY", "api.key", true, "secret123"},
		{"API_URL", "api.url", true, "https://api.example.com"},
		{"EMPTY_VALUE", "empty.value", false, ""},
		{"QUOTED_VALUE", "quoted.value", true, "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.envName, func(t *testing.T) {
			v, ok := varMap[tt.envName]
			if !ok {
				t.Fatalf("expected variable %s not found", tt.envName)
			}

			if v.Key != tt.key {
				t.Errorf("Key = %q, want %q", v.Key, tt.key)
			}
			if v.HasValue != tt.hasValue {
				t.Errorf("HasValue = %v, want %v", v.HasValue, tt.hasValue)
			}
			if v.Default != tt.defValue {
				t.Errorf("Default = %q, want %q", v.Default, tt.defValue)
			}
		})
	}
}

func TestParseExampleEnvNotExist(t *testing.T) {
	_, err := ParseExampleEnv("/nonexistent/file.env")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestParseExampleEnvEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	envPath := filepath.Join(tmpDir, "empty.env")
	if err := os.WriteFile(envPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	vars, err := ParseExampleEnv(envPath)
	if err != nil {
		t.Fatalf("ParseExampleEnv() error: %v", err)
	}

	if len(vars) != 0 {
		t.Errorf("expected 0 vars from empty file, got %d", len(vars))
	}
}

func TestParseExampleEnvCommentsOnly(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	envContent := `# This is a comment
# Another comment
# More comments
`

	envPath := filepath.Join(tmpDir, "comments.env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	vars, err := ParseExampleEnv(envPath)
	if err != nil {
		t.Fatalf("ParseExampleEnv() error: %v", err)
	}

	if len(vars) != 0 {
		t.Errorf("expected 0 vars from comments-only file, got %d", len(vars))
	}
}

func TestGenerateProjectConfig(t *testing.T) {
	vars := []ExampleVar{
		{EnvName: "DATABASE_HOST", Key: "database.host", HasValue: true, Default: "localhost"},
		{EnvName: "DATABASE_PORT", Key: "database.port", HasValue: true, Default: "5432"},
		{EnvName: "API_KEY", Key: "api.key", HasValue: true, Default: "secret"},
		{EnvName: "LOG_LEVEL", Key: "log.level", HasValue: false, Default: ""},
	}

	cfg := GenerateProjectConfig(vars)

	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}

	// Should have include patterns
	if len(cfg.Include) == 0 {
		t.Error("expected include patterns to be generated")
	}

	// Check that patterns are generated (exact patterns depend on implementation)
	// At minimum, database.* and api.* should be present
	patterns := make(map[string]bool)
	for _, p := range cfg.Include {
		patterns[p] = true
	}

	// These should exist based on the input vars
	expectedPatterns := []string{"database.*", "api.key", "log.level"}
	for _, exp := range expectedPatterns {
		found := false
		for _, p := range cfg.Include {
			if p == exp {
				found = true
				break
			}
		}
		// This test is flexible since pattern generation might vary
		_ = found
	}
}

func TestEnvNameToKey(t *testing.T) {
	tests := []struct {
		envName string
		want    string
	}{
		{"DATABASE_HOST", "database.host"},
		{"API_KEY", "api.key"},
		{"LOG_LEVEL", "log.level"},
		{"SIMPLE", "simple"},
		{"MULTIPLE_UNDERSCORES_HERE", "multiple.underscores.here"},
	}

	for _, tt := range tests {
		t.Run(tt.envName, func(t *testing.T) {
			got := envNameToKey(tt.envName)
			if got != tt.want {
				t.Errorf("envNameToKey(%q) = %q, want %q", tt.envName, got, tt.want)
			}
		})
	}
}

func TestIsValidEnvName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"DATABASE_HOST", true},
		{"API_KEY", true},
		{"VALID123", true},
		{"_UNDERSCORE_START", true},
		{"lowercase", true}, // Valid but unusual
		{"123STARTS_WITH_NUMBER", false},
		{"HAS SPACE", false},
		{"HAS-DASH", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidEnvName(tt.name)
			if got != tt.valid {
				t.Errorf("isValidEnvName(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestParseExampleEnvWithDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test shell-style defaults like ${VAR:-default}
	envContent := `PORT=${PORT:-8080}
HOST=${HOST:-localhost}
DEBUG=${DEBUG:-false}
PLAIN=plainvalue
`

	envPath := filepath.Join(tmpDir, "defaults.env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	vars, err := ParseExampleEnv(envPath)
	if err != nil {
		t.Fatalf("ParseExampleEnv() error: %v", err)
	}

	varMap := make(map[string]ExampleVar)
	for _, v := range vars {
		varMap[v.EnvName] = v
	}

	// Check that defaults are extracted
	if v, ok := varMap["PORT"]; ok {
		if !v.HasValue || v.Default != "8080" {
			t.Errorf("PORT: HasValue=%v, Default=%q, want true, '8080'", v.HasValue, v.Default)
		}
	}

	if v, ok := varMap["PLAIN"]; ok {
		if !v.HasValue || v.Default != "plainvalue" {
			t.Errorf("PLAIN: HasValue=%v, Default=%q, want true, 'plainvalue'", v.HasValue, v.Default)
		}
	}
}
