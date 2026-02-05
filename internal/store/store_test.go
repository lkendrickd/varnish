package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dk/varnish/internal/crypto"
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

func TestIsEncrypted(t *testing.T) {
	s := New()
	if s.IsEncrypted() {
		t.Error("new store should not be encrypted")
	}
}

func TestEnableEncryptionNoPassword(t *testing.T) {
	unsetenv(t, crypto.PasswordEnvVar)

	s := New()
	err := s.EnableEncryption()
	if err == nil {
		t.Error("EnableEncryption() should fail without password")
	}
}

func TestEnableEncryptionWithPassword(t *testing.T) {
	t.Setenv(crypto.PasswordEnvVar, "testpassword")

	s := New()
	if err := s.EnableEncryption(); err != nil {
		t.Fatalf("EnableEncryption() error = %v", err)
	}

	if !s.IsEncrypted() {
		t.Error("store should be encrypted after EnableEncryption()")
	}
}

func TestSaveLoadEncrypted(t *testing.T) {
	t.Setenv(crypto.PasswordEnvVar, "testpassword")

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "store.yaml")

	// Create, encrypt, and save store
	s := New()
	s.Set("secret.key", "secret-value")
	s.Set("another.secret", "another-secret-value")

	if err := s.EnableEncryption(); err != nil {
		t.Fatalf("EnableEncryption() error: %v", err)
	}

	if err := s.SaveTo(storePath); err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}

	// Verify file is encrypted (starts with magic bytes)
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if !crypto.IsEncrypted(data) {
		t.Error("saved file should be encrypted")
	}

	// File should NOT contain plain text secrets
	if strings.Contains(string(data), "secret-value") {
		t.Error("encrypted file contains plaintext secret")
	}

	// Load store
	loaded, err := LoadFrom(storePath)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	// Verify
	if !loaded.IsEncrypted() {
		t.Error("loaded store should be marked as encrypted")
	}

	val, ok := loaded.Get("secret.key")
	if !ok || val != "secret-value" {
		t.Errorf("loaded secret.key = %q, ok=%v, want 'secret-value'", val, ok)
	}

	val, ok = loaded.Get("another.secret")
	if !ok || val != "another-secret-value" {
		t.Errorf("loaded another.secret = %q, ok=%v, want 'another-secret-value'", val, ok)
	}
}

func TestLoadEncryptedWithoutPassword(t *testing.T) {
	// First create an encrypted store with password
	t.Setenv(crypto.PasswordEnvVar, "testpassword")

	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "store.yaml")

	s := New()
	s.Set("key", "value")
	if err := s.EnableEncryption(); err != nil {
		t.Fatalf("EnableEncryption() error: %v", err)
	}
	if err := s.SaveTo(storePath); err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}

	// Now try to load without password
	unsetenv(t, crypto.PasswordEnvVar)

	_, err = LoadFrom(storePath)
	if err == nil {
		t.Error("LoadFrom() should fail without password for encrypted store")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("error should mention password, got: %v", err)
	}
}

func TestLoadEncryptedWithWrongPassword(t *testing.T) {
	// Create an encrypted store with one password
	t.Setenv(crypto.PasswordEnvVar, "correctpassword")

	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "store.yaml")

	s := New()
	s.Set("key", "value")
	if err := s.EnableEncryption(); err != nil {
		t.Fatalf("EnableEncryption() error: %v", err)
	}
	if err := s.SaveTo(storePath); err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}

	// Try to load with wrong password (t.Setenv handles cleanup)
	t.Setenv(crypto.PasswordEnvVar, "wrongpassword")

	_, err = LoadFrom(storePath)
	if err == nil {
		t.Error("LoadFrom() should fail with wrong password")
	}
}

func TestLen(t *testing.T) {
	s := New()
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}

	s.Set("key1", "val1")
	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1", s.Len())
	}

	s.Set("key2", "val2")
	s.Set("key3", "val3")
	if s.Len() != 3 {
		t.Errorf("Len() = %d, want 3", s.Len())
	}

	s.Delete("key1")
	if s.Len() != 2 {
		t.Errorf("Len() = %d, want 2", s.Len())
	}
}

func TestLoadSaveWithRealPath(t *testing.T) {
	// Create a temp HOME to test Load() and Save() with real paths
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	// Load should return empty store when no file exists
	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s.Len() != 0 {
		t.Errorf("expected empty store, got %d vars", s.Len())
	}

	// Add some data and save
	s.Set("test.key", "test-value")
	s.Set("another.key", "another-value")
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Load again and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after save error: %v", err)
	}
	if loaded.Len() != 2 {
		t.Errorf("expected 2 vars, got %d", loaded.Len())
	}
	val, ok := loaded.Get("test.key")
	if !ok || val != "test-value" {
		t.Errorf("Get(test.key) = %q, %v; want test-value, true", val, ok)
	}
}

func TestLoadSaveEncryptedWithRealPath(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)
	t.Setenv(crypto.PasswordEnvVar, "testpassword")

	// Create and save encrypted store
	s := New()
	s.Set("secret.key", "secret-value")
	if err := s.EnableEncryption(); err != nil {
		t.Fatalf("EnableEncryption() error: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Load and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !loaded.IsEncrypted() {
		t.Error("expected encrypted store")
	}
	val, ok := loaded.Get("secret.key")
	if !ok || val != "secret-value" {
		t.Errorf("Get(secret.key) = %q, %v; want secret-value, true", val, ok)
	}
}

func TestUnencryptedStoreRemainsReadable(t *testing.T) {
	unsetenv(t, crypto.PasswordEnvVar)

	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "store.yaml")

	// Create and save unencrypted store
	s := New()
	s.Set("plain.key", "plain-value")

	if err := s.SaveTo(storePath); err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}

	// File should be plain YAML
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if crypto.IsEncrypted(data) {
		t.Error("unencrypted store should not have magic bytes")
	}

	if !strings.Contains(string(data), "plain-value") {
		t.Error("unencrypted store should contain plaintext")
	}

	// Load should work without password
	loaded, err := LoadFrom(storePath)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	if loaded.IsEncrypted() {
		t.Error("loaded unencrypted store should not be marked as encrypted")
	}

	val, ok := loaded.Get("plain.key")
	if !ok || val != "plain-value" {
		t.Errorf("loaded plain.key = %q, ok=%v, want 'plain-value'", val, ok)
	}
}

func TestRemove(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	// Create store
	s := New()
	s.Set("test.key", "test-value")
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify store exists
	storePath := filepath.Join(tmpHome, ".varnish", "store.yaml")
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Fatal("store file should exist after Save()")
	}

	// Remove store
	if err := Remove(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	// Verify store is gone
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Error("store file should not exist after Remove()")
	}

	// Remove again should not error (idempotent)
	if err := Remove(); err != nil {
		t.Errorf("Remove() on non-existent store should not error: %v", err)
	}
}
