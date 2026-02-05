package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	s := New()

	if s.Version != 1 {
		t.Errorf("expected version 1, got %d", s.Version)
	}
	if s.Variables == nil {
		t.Error("expected Variables to be initialized")
	}
	if len(s.Variables) != 0 {
		t.Errorf("expected empty Variables, got %d", len(s.Variables))
	}
}

func TestSetGet(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantOK  bool
		wantVal string
	}{
		{
			name:    "simple key",
			key:     "database.host",
			value:   "localhost",
			wantOK:  true,
			wantVal: "localhost",
		},
		{
			name:    "empty value",
			key:     "empty.key",
			value:   "",
			wantOK:  true,
			wantVal: "",
		},
		{
			name:    "value with special chars",
			key:     "password",
			value:   "p@ss=word!",
			wantOK:  true,
			wantVal: "p@ss=word!",
		},
		{
			name:    "namespaced key",
			key:     "myapp.database.host",
			value:   "db.example.com",
			wantOK:  true,
			wantVal: "db.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New()
			s.Set(tt.key, tt.value)

			got, ok := s.Get(tt.key)
			if ok != tt.wantOK {
				t.Errorf("Get() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantVal {
				t.Errorf("Get() = %q, want %q", got, tt.wantVal)
			}
		})
	}
}

func TestGetNotFound(t *testing.T) {
	s := New()
	s.Set("exists", "value")

	_, ok := s.Get("notexists")
	if ok {
		t.Error("expected ok=false for non-existent key")
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name       string
		setupKeys  map[string]string
		deleteKey  string
		wantResult bool
		wantKeys   int
	}{
		{
			name:       "delete existing key",
			setupKeys:  map[string]string{"key1": "val1", "key2": "val2"},
			deleteKey:  "key1",
			wantResult: true,
			wantKeys:   1,
		},
		{
			name:       "delete non-existent key",
			setupKeys:  map[string]string{"key1": "val1"},
			deleteKey:  "nonexistent",
			wantResult: false,
			wantKeys:   1,
		},
		{
			name:       "delete from empty store",
			setupKeys:  map[string]string{},
			deleteKey:  "key",
			wantResult: false,
			wantKeys:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New()
			for k, v := range tt.setupKeys {
				s.Set(k, v)
			}

			result := s.Delete(tt.deleteKey)
			if result != tt.wantResult {
				t.Errorf("Delete() = %v, want %v", result, tt.wantResult)
			}
			if len(s.Keys()) != tt.wantKeys {
				t.Errorf("Keys() len = %d, want %d", len(s.Keys()), tt.wantKeys)
			}
		})
	}
}

func TestKeys(t *testing.T) {
	s := New()
	s.Set("zebra", "1")
	s.Set("alpha", "2")
	s.Set("middle", "3")

	keys := s.Keys()

	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}

	// Keys should be sorted
	expected := []string{"alpha", "middle", "zebra"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestSaveLoad(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "store.yaml")

	// Create and save store
	s := New()
	s.Set("project.db.host", "localhost")
	s.Set("project.db.port", "5432")

	if err := s.SaveTo(storePath); err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}

	// Load store
	loaded, err := LoadFrom(storePath)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	// Verify
	if loaded.Version != 1 {
		t.Errorf("loaded version = %d, want 1", loaded.Version)
	}

	val, ok := loaded.Get("project.db.host")
	if !ok || val != "localhost" {
		t.Errorf("loaded db.host = %q, ok=%v, want 'localhost'", val, ok)
	}

	val, ok = loaded.Get("project.db.port")
	if !ok || val != "5432" {
		t.Errorf("loaded db.port = %q, ok=%v, want '5432'", val, ok)
	}
}

func TestLoadFromNotExist(t *testing.T) {
	_, err := LoadFrom("/nonexistent/path/store.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestOverwrite(t *testing.T) {
	s := New()
	s.Set("key", "original")

	val, _ := s.Get("key")
	if val != "original" {
		t.Errorf("initial value = %q, want 'original'", val)
	}

	s.Set("key", "updated")
	val, _ = s.Get("key")
	if val != "updated" {
		t.Errorf("updated value = %q, want 'updated'", val)
	}
}
