package domain

import (
	"testing"
)

func TestNewResolver(t *testing.T) {
	store := NewStore()
	cfg := NewProjectConfig()
	cfg.Project = "test"

	resolver := NewResolver(store, cfg)

	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestResolverResolveBasic(t *testing.T) {
	store := NewStore()
	store.Set("myapp.database.host", "localhost")
	store.Set("myapp.database.port", "5432")

	cfg := NewProjectConfig()
	cfg.Project = "myapp"
	cfg.Include = []string{"database.*"}

	resolver := NewResolver(store, cfg)
	vars := resolver.Resolve()

	if len(vars) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(vars))
	}

	// Check that vars are resolved correctly
	varMap := make(map[string]ResolvedVar)
	for _, v := range vars {
		varMap[v.EnvName] = v
	}

	if v, ok := varMap["DATABASE_HOST"]; !ok {
		t.Error("expected DATABASE_HOST")
	} else {
		if v.Value != "localhost" {
			t.Errorf("DATABASE_HOST = %q, want 'localhost'", v.Value)
		}
		if v.Source != "store" {
			t.Errorf("DATABASE_HOST source = %q, want 'store'", v.Source)
		}
	}

	if v, ok := varMap["DATABASE_PORT"]; !ok {
		t.Error("expected DATABASE_PORT")
	} else {
		if v.Value != "5432" {
			t.Errorf("DATABASE_PORT = %q, want '5432'", v.Value)
		}
	}
}

func TestResolverResolveWithOverrides(t *testing.T) {
	store := NewStore()
	store.Set("myapp.database.host", "localhost")
	store.Set("myapp.database.name", "production_db")

	cfg := NewProjectConfig()
	cfg.Project = "myapp"
	cfg.Include = []string{"database.*"}
	cfg.Overrides = map[string]string{
		"database.name": "dev_db", // Override the store value
	}

	resolver := NewResolver(store, cfg)
	vars := resolver.Resolve()

	varMap := make(map[string]ResolvedVar)
	for _, v := range vars {
		varMap[v.EnvName] = v
	}

	if v, ok := varMap["DATABASE_NAME"]; !ok {
		t.Error("expected DATABASE_NAME")
	} else {
		if v.Value != "dev_db" {
			t.Errorf("DATABASE_NAME = %q, want 'dev_db' (override)", v.Value)
		}
		if v.Source != "override" {
			t.Errorf("DATABASE_NAME source = %q, want 'override'", v.Source)
		}
	}
}

func TestResolverResolveWithMappings(t *testing.T) {
	store := NewStore()
	store.Set("myapp.database.url", "postgres://localhost/db")

	cfg := NewProjectConfig()
	cfg.Project = "myapp"
	cfg.Include = []string{"database.*"}
	cfg.Mappings = map[string]string{
		"database.url": "DB_CONNECTION_STRING", // Custom env var name
	}

	resolver := NewResolver(store, cfg)
	vars := resolver.Resolve()

	if len(vars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(vars))
	}

	if vars[0].EnvName != "DB_CONNECTION_STRING" {
		t.Errorf("EnvName = %q, want 'DB_CONNECTION_STRING'", vars[0].EnvName)
	}
}

func TestResolverResolveWithComputed(t *testing.T) {
	store := NewStore()
	store.Set("myapp.database.host", "localhost")
	store.Set("myapp.database.port", "5432")
	store.Set("myapp.database.name", "mydb")

	cfg := NewProjectConfig()
	cfg.Project = "myapp"
	cfg.Include = []string{"database.*"}
	cfg.Computed = map[string]string{
		"DATABASE_URL": "postgres://${database.host}:${database.port}/${database.name}",
	}

	resolver := NewResolver(store, cfg)
	vars := resolver.Resolve()

	varMap := make(map[string]ResolvedVar)
	for _, v := range vars {
		varMap[v.EnvName] = v
	}

	if v, ok := varMap["DATABASE_URL"]; !ok {
		t.Error("expected DATABASE_URL computed var")
	} else {
		expected := "postgres://localhost:5432/mydb"
		if v.Value != expected {
			t.Errorf("DATABASE_URL = %q, want %q", v.Value, expected)
		}
		if v.Source != "computed" {
			t.Errorf("DATABASE_URL source = %q, want 'computed'", v.Source)
		}
	}
}

func TestResolverMissingVars(t *testing.T) {
	store := NewStore()
	store.Set("myapp.database.host", "localhost")
	// Note: database.port is NOT in the store

	cfg := NewProjectConfig()
	cfg.Project = "myapp"
	cfg.Include = []string{"database.host", "database.port"}

	resolver := NewResolver(store, cfg)
	missing := resolver.MissingVars()

	// database.port should be missing
	found := false
	for _, m := range missing {
		if m == "database.port" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected 'database.port' in missing vars, got %v", missing)
	}
}

func TestResolverEmptyProject(t *testing.T) {
	store := NewStore()
	store.Set("database.host", "localhost") // No project prefix

	cfg := NewProjectConfig()
	cfg.Project = "" // Empty project
	cfg.Include = []string{"database.*"}

	resolver := NewResolver(store, cfg)
	vars := resolver.Resolve()

	// Should resolve without project prefix
	if len(vars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(vars))
	}
}

func TestResolverMultipleIncludePatterns(t *testing.T) {
	store := NewStore()
	store.Set("myapp.database.host", "localhost")
	store.Set("myapp.cache.host", "redis")
	store.Set("myapp.api.key", "secret")

	cfg := NewProjectConfig()
	cfg.Project = "myapp"
	cfg.Include = []string{"database.*", "cache.*", "api.*"}

	resolver := NewResolver(store, cfg)
	vars := resolver.Resolve()

	if len(vars) != 3 {
		t.Errorf("expected 3 vars, got %d", len(vars))
	}
}

func TestResolverKeyToEnvName(t *testing.T) {
	// Create a minimal resolver to test the method
	store := NewStore()
	cfg := NewProjectConfig()
	resolver := NewResolver(store, cfg)

	tests := []struct {
		key  string
		want string
	}{
		{"database.host", "DATABASE_HOST"},
		{"api.key", "API_KEY"},
		{"log.level", "LOG_LEVEL"},
		{"simple", "SIMPLE"},
		{"multiple.dots.here", "MULTIPLE_DOTS_HERE"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			// We test indirectly through Resolve since keyToEnvName is unexported
			store.Set(tt.key, "testvalue")
			cfg.Include = []string{tt.key}
			vars := resolver.Resolve()

			found := false
			for _, v := range vars {
				if v.EnvName == tt.want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected env name %q for key %q", tt.want, tt.key)
			}
		})
	}
}

func TestResolverGlobMatching(t *testing.T) {
	store := NewStore()
	store.Set("myapp.database.host", "localhost")
	store.Set("myapp.database.port", "5432")
	store.Set("myapp.database.user", "admin")
	store.Set("myapp.cache.host", "redis") // Should NOT match database.*

	cfg := NewProjectConfig()
	cfg.Project = "myapp"
	cfg.Include = []string{"database.*"}

	resolver := NewResolver(store, cfg)
	vars := resolver.Resolve()

	// Should have 3 database vars, not cache
	if len(vars) != 3 {
		t.Errorf("expected 3 vars, got %d", len(vars))
	}

	for _, v := range vars {
		if v.Key == "cache.host" {
			t.Error("cache.host should not be included with database.* pattern")
		}
	}
}
