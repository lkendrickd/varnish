package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	store := NewStore()

	if store.Version != 1 {
		t.Errorf("expected version 1, got %d", store.Version)
	}
	if store.Variables == nil {
		t.Error("expected Variables to be initialized")
	}
	if len(store.Variables) != 0 {
		t.Errorf("expected empty Variables, got %d", len(store.Variables))
	}
}

func TestStoreSetGet(t *testing.T) {
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
			store := NewStore()
			store.Set(tt.key, tt.value)

			got, ok := store.Get(tt.key)
			if ok != tt.wantOK {
				t.Errorf("Get() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantVal {
				t.Errorf("Get() = %q, want %q", got, tt.wantVal)
			}
		})
	}
}

func TestStoreGetNotFound(t *testing.T) {
	store := NewStore()
	store.Set("exists", "value")

	_, ok := store.Get("notexists")
	if ok {
		t.Error("expected ok=false for non-existent key")
	}
}

func TestStoreDelete(t *testing.T) {
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
			store := NewStore()
			for k, v := range tt.setupKeys {
				store.Set(k, v)
			}

			result := store.Delete(tt.deleteKey)
			if result != tt.wantResult {
				t.Errorf("Delete() = %v, want %v", result, tt.wantResult)
			}
			if len(store.Keys()) != tt.wantKeys {
				t.Errorf("Keys() len = %d, want %d", len(store.Keys()), tt.wantKeys)
			}
		})
	}
}

func TestStoreKeys(t *testing.T) {
	store := NewStore()
	store.Set("zebra", "1")
	store.Set("alpha", "2")
	store.Set("middle", "3")

	keys := store.Keys()

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

func TestStoreSaveLoad(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "store.yaml")

	// Create and save store
	store := NewStore()
	store.Set("project.db.host", "localhost")
	store.Set("project.db.port", "5432")

	if err := store.SaveTo(storePath); err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}

	// Load store
	loaded, err := LoadStoreFrom(storePath)
	if err != nil {
		t.Fatalf("LoadStoreFrom() error: %v", err)
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

func TestLoadStoreFromNotExist(t *testing.T) {
	_, err := LoadStoreFrom("/nonexistent/path/store.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestStoreOverwrite(t *testing.T) {
	store := NewStore()
	store.Set("key", "original")

	val, _ := store.Get("key")
	if val != "original" {
		t.Errorf("initial value = %q, want 'original'", val)
	}

	store.Set("key", "updated")
	val, _ = store.Get("key")
	if val != "updated" {
		t.Errorf("updated value = %q, want 'updated'", val)
	}
}
