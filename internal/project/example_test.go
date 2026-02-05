package project

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

	envContent := `# This is a comment
DATABASE_HOST=localhost
DATABASE_PORT=5432
API_KEY=${API_KEY:-}
LOG_LEVEL=${LOG_LEVEL:-info}
export EXPORTED_VAR=exported_value
EMPTY_VAR=
`

	envPath := filepath.Join(tmpDir, "example.env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write example.env: %v", err)
	}

	vars, err := ParseExampleEnv(envPath)
	if err != nil {
		t.Fatalf("ParseExampleEnv() error: %v", err)
	}

	if len(vars) != 6 {
		t.Fatalf("expected 6 vars, got %d", len(vars))
	}

	// Check specific vars
	varMap := make(map[string]ExampleVar)
	for _, v := range vars {
		varMap[v.EnvName] = v
	}

	// DATABASE_HOST=localhost
	if v, ok := varMap["DATABASE_HOST"]; !ok {
		t.Error("expected DATABASE_HOST")
	} else {
		if v.Key != "database.host" {
			t.Errorf("DATABASE_HOST key = %q, want 'database.host'", v.Key)
		}
		if v.Default != "localhost" {
			t.Errorf("DATABASE_HOST default = %q, want 'localhost'", v.Default)
		}
		if !v.HasValue {
			t.Error("DATABASE_HOST should have value")
		}
	}

	// LOG_LEVEL=${LOG_LEVEL:-info}
	if v, ok := varMap["LOG_LEVEL"]; !ok {
		t.Error("expected LOG_LEVEL")
	} else {
		if v.Default != "info" {
			t.Errorf("LOG_LEVEL default = %q, want 'info'", v.Default)
		}
		if !v.HasValue {
			t.Error("LOG_LEVEL should have value")
		}
	}

	// API_KEY=${API_KEY:-} (empty default)
	if v, ok := varMap["API_KEY"]; !ok {
		t.Error("expected API_KEY")
	} else {
		if v.HasValue {
			t.Error("API_KEY should not have value (empty default)")
		}
	}
}

func TestParseExampleEnvNotExist(t *testing.T) {
	_, err := ParseExampleEnv("/nonexistent/path/example.env")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestEnvNameToKey(t *testing.T) {
	tests := []struct {
		envName string
		want    string
	}{
		{"DATABASE_HOST", "database.host"},
		{"LOG_LEVEL", "log.level"},
		{"API_KEY", "api.key"},
		{"SIMPLE", "simple"},
		{"AWS_ACCESS_KEY_ID", "aws.access.key.id"},
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

func TestGenerateConfig(t *testing.T) {
	vars := []ExampleVar{
		{EnvName: "DATABASE_HOST", Key: "database.host", Default: "localhost", HasValue: true},
		{EnvName: "DATABASE_PORT", Key: "database.port", Default: "5432", HasValue: true},
		{EnvName: "API_KEY", Key: "api.key", Default: "", HasValue: false},
		{EnvName: "LOG_LEVEL", Key: "log.level", Default: "info", HasValue: true},
	}

	cfg := GenerateConfig(vars)

	if cfg.Version != 1 {
		t.Errorf("version = %d, want 1", cfg.Version)
	}

	// database.* should be grouped (2 vars with same prefix)
	foundDatabaseGlob := false
	for _, inc := range cfg.Include {
		if inc == "database.*" {
			foundDatabaseGlob = true
		}
	}
	if !foundDatabaseGlob {
		t.Errorf("expected 'database.*' in include, got %v", cfg.Include)
	}

	// Single-occurrence prefixes should be literal
	foundApiKey := false
	foundLogLevel := false
	for _, inc := range cfg.Include {
		if inc == "api.key" {
			foundApiKey = true
		}
		if inc == "log.level" {
			foundLogLevel = true
		}
	}
	if !foundApiKey {
		t.Errorf("expected 'api.key' in include, got %v", cfg.Include)
	}
	if !foundLogLevel {
		t.Errorf("expected 'log.level' in include, got %v", cfg.Include)
	}
}

func TestIsValidEnvName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"VALID_NAME", true},
		{"valid_name", true},
		{"_UNDERSCORE_START", true},
		{"NAME123", true},
		{"123_STARTS_WITH_NUM", false},
		{"HAS-DASH", false},
		{"HAS.DOT", false},
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

func TestExtractDefault(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{"${VAR:-default}", "default", true},
		{"${VAR-default}", "default", true},
		{"${VAR:-}", "", true},
		{"${VAR}", "", false},
		{"plain_value", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := extractDefault(tt.input)
			if ok != tt.wantOK {
				t.Errorf("extractDefault(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("extractDefault(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"quoted"`, "quoted"},
		{`'single'`, "single"},
		{"no quotes", "no quotes"},
		{`"`, `"`},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := trimQuotes(tt.input)
			if got != tt.want {
				t.Errorf("trimQuotes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
