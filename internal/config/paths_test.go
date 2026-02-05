package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVarnishDir(t *testing.T) {
	dir, err := VarnishDir()
	if err != nil {
		t.Fatalf("VarnishDir() error: %v", err)
	}

	if dir == "" {
		t.Error("expected non-empty directory path")
	}

	if !strings.HasSuffix(dir, ".varnish") {
		t.Errorf("VarnishDir() = %q, expected to end with '.varnish'", dir)
	}
}

func TestStorePath(t *testing.T) {
	path, err := StorePath()
	if err != nil {
		t.Fatalf("StorePath() error: %v", err)
	}

	if !strings.HasSuffix(path, "store.yaml") {
		t.Errorf("StorePath() = %q, expected to end with 'store.yaml'", path)
	}

	if !strings.Contains(path, ".varnish") {
		t.Errorf("StorePath() = %q, expected to contain '.varnish'", path)
	}
}

func TestConfigPath(t *testing.T) {
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error: %v", err)
	}

	if !strings.HasSuffix(path, "config.yaml") {
		t.Errorf("ConfigPath() = %q, expected to end with 'config.yaml'", path)
	}
}

func TestRegistryPath(t *testing.T) {
	path := RegistryPath()

	if path == "" {
		t.Error("expected non-empty registry path")
	}

	if !strings.HasSuffix(path, "registry.yaml") {
		t.Errorf("RegistryPath() = %q, expected to end with 'registry.yaml'", path)
	}
}

func TestProjectsDir(t *testing.T) {
	dir := ProjectsDir()

	if dir == "" {
		t.Error("expected non-empty projects directory")
	}

	if !strings.HasSuffix(dir, "projects") {
		t.Errorf("ProjectsDir() = %q, expected to end with 'projects'", dir)
	}
}

func TestProjectConfigPathFor(t *testing.T) {
	tests := []struct {
		project string
		wantEnd string
	}{
		{"myapp", "projects/myapp.yaml"},
		{"test-project", "projects/test-project.yaml"},
		{"simple", "projects/simple.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.project, func(t *testing.T) {
			path := ProjectConfigPathFor(tt.project)
			if !strings.HasSuffix(path, tt.wantEnd) {
				t.Errorf("ProjectConfigPathFor(%q) = %q, want suffix %q", tt.project, path, tt.wantEnd)
			}
		})
	}
}

func TestEnsureVarnishDir(t *testing.T) {
	// This test uses the real home directory
	// In a production test suite, you might want to mock this
	err := EnsureVarnishDir()
	if err != nil {
		t.Fatalf("EnsureVarnishDir() error: %v", err)
	}

	dir, _ := VarnishDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("failed to stat varnish dir: %v", err)
	}

	if !info.IsDir() {
		t.Error("expected varnish dir to be a directory")
	}

	// Check permissions (should be 0700)
	perm := info.Mode().Perm()
	if perm != PermDir {
		t.Errorf("varnish dir permissions = %o, want %o", perm, PermDir)
	}
}

func TestEnsureProjectsDir(t *testing.T) {
	err := EnsureProjectsDir()
	if err != nil {
		t.Fatalf("EnsureProjectsDir() error: %v", err)
	}

	dir := ProjectsDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("failed to stat projects dir: %v", err)
	}

	if !info.IsDir() {
		t.Error("expected projects dir to be a directory")
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		content string
		perm    os.FileMode
	}{
		{
			name:    "simple content",
			content: "hello world",
			perm:    0644,
		},
		{
			name:    "yaml content",
			content: "version: 1\nkey: value\n",
			perm:    0600,
		},
		{
			name:    "empty content",
			content: "",
			perm:    0644,
		},
		{
			name:    "multiline content",
			content: "line1\nline2\nline3\n",
			perm:    0600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name+".txt")

			err := AtomicWrite(path, []byte(tt.content), tt.perm)
			if err != nil {
				t.Fatalf("AtomicWrite() error: %v", err)
			}

			// Verify file exists
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("failed to stat written file: %v", err)
			}

			// Verify permissions
			if info.Mode().Perm() != tt.perm {
				t.Errorf("file permissions = %o, want %o", info.Mode().Perm(), tt.perm)
			}

			// Verify content
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read written file: %v", err)
			}

			if string(data) != tt.content {
				t.Errorf("file content = %q, want %q", string(data), tt.content)
			}
		})
	}
}

func TestAtomicWriteOverwrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "overwrite.txt")

	// Write initial content
	if err := AtomicWrite(path, []byte("initial"), 0644); err != nil {
		t.Fatalf("first AtomicWrite() error: %v", err)
	}

	// Overwrite with new content
	if err := AtomicWrite(path, []byte("updated"), 0644); err != nil {
		t.Fatalf("second AtomicWrite() error: %v", err)
	}

	// Verify new content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != "updated" {
		t.Errorf("file content = %q, want 'updated'", string(data))
	}
}

func TestAtomicWriteInvalidDir(t *testing.T) {
	// Try to write to a non-existent directory
	err := AtomicWrite("/nonexistent/dir/file.txt", []byte("test"), 0644)
	if err == nil {
		t.Error("expected error when writing to non-existent directory")
	}
}

func TestConstants(t *testing.T) {
	// Verify constants have expected values
	if DirName != ".varnish" {
		t.Errorf("DirName = %q, want '.varnish'", DirName)
	}
	if StoreFileName != "store.yaml" {
		t.Errorf("StoreFileName = %q, want 'store.yaml'", StoreFileName)
	}
	if RegistryFileName != "registry.yaml" {
		t.Errorf("RegistryFileName = %q, want 'registry.yaml'", RegistryFileName)
	}
	if ProjectsDirName != "projects" {
		t.Errorf("ProjectsDirName = %q, want 'projects'", ProjectsDirName)
	}
	if PermSecure != 0600 {
		t.Errorf("PermSecure = %o, want 0600", PermSecure)
	}
	if PermDir != 0700 {
		t.Errorf("PermDir = %o, want 0700", PermDir)
	}
	if PermConfig != 0644 {
		t.Errorf("PermConfig = %o, want 0644", PermConfig)
	}
}
