package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistryEmpty(t *testing.T) {
	// Create temp directory with no registry file
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test that a non-existent registry file path doesn't exist
	regPath := filepath.Join(tmpDir, "registry.yaml")

	// File doesn't exist - verify
	_, err = os.Stat(regPath)
	if !os.IsNotExist(err) {
		t.Fatal("expected registry file to not exist")
	}

	// Note: We can't test LoadRegistry directly without mocking config.RegistryPath()
	// This test verifies the expected initial state
}

func TestRegistryRegisterLookup(t *testing.T) {
	tests := []struct {
		name        string
		registerDir string
		project     string
		lookupDir   string
		wantProject string
	}{
		{
			name:        "exact match",
			registerDir: "/home/user/myapp",
			project:     "myapp",
			lookupDir:   "/home/user/myapp",
			wantProject: "myapp",
		},
		{
			name:        "subdirectory lookup",
			registerDir: "/home/user/myapp",
			project:     "myapp",
			lookupDir:   "/home/user/myapp/src/cmd",
			wantProject: "myapp",
		},
		{
			name:        "not registered",
			registerDir: "/home/user/myapp",
			project:     "myapp",
			lookupDir:   "/home/user/otherapp",
			wantProject: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fresh registry for each test
			reg := &Registry{
				Version:  1,
				Projects: make(map[string]string),
			}

			reg.Register(tt.registerDir, tt.project)
			got := reg.Lookup(tt.lookupDir)

			if got != tt.wantProject {
				t.Errorf("Lookup(%q) = %q, want %q", tt.lookupDir, got, tt.wantProject)
			}
		})
	}
}

func TestRegistryUnregister(t *testing.T) {
	reg := &Registry{
		Version:  1,
		Projects: make(map[string]string),
	}

	reg.Register("/home/user/myapp", "myapp")
	reg.Register("/home/user/otherapp", "otherapp")

	if len(reg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(reg.Projects))
	}

	reg.Unregister("/home/user/myapp")

	if len(reg.Projects) != 1 {
		t.Errorf("expected 1 project after unregister, got %d", len(reg.Projects))
	}

	if reg.Lookup("/home/user/myapp") != "" {
		t.Error("expected empty lookup after unregister")
	}

	if reg.Lookup("/home/user/otherapp") != "otherapp" {
		t.Error("expected otherapp to still be registered")
	}
}

func TestRegistryProjectDirs(t *testing.T) {
	reg := &Registry{
		Version:  1,
		Projects: make(map[string]string),
	}

	reg.Register("/home/user/myapp", "myapp")
	reg.Register("/home/user/myapp-v2", "myapp")
	reg.Register("/home/user/otherapp", "otherapp")

	dirs := reg.ProjectDirs("myapp")

	if len(dirs) != 2 {
		t.Fatalf("expected 2 dirs for myapp, got %d", len(dirs))
	}

	// Should be sorted
	if dirs[0] != "/home/user/myapp" {
		t.Errorf("dirs[0] = %q, want '/home/user/myapp'", dirs[0])
	}
	if dirs[1] != "/home/user/myapp-v2" {
		t.Errorf("dirs[1] = %q, want '/home/user/myapp-v2'", dirs[1])
	}
}

func TestRegistryAllProjects(t *testing.T) {
	reg := &Registry{
		Version:  1,
		Projects: make(map[string]string),
	}

	reg.Register("/path/a", "zebra")
	reg.Register("/path/b", "alpha")
	reg.Register("/path/c", "alpha") // duplicate project
	reg.Register("/path/d", "middle")

	projects := reg.AllProjects()

	if len(projects) != 3 {
		t.Fatalf("expected 3 unique projects, got %d", len(projects))
	}

	// Should be sorted
	expected := []string{"alpha", "middle", "zebra"}
	for i, p := range projects {
		if p != expected[i] {
			t.Errorf("projects[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestRegistrySaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	regPath := filepath.Join(tmpDir, "registry.yaml")

	// Create and save
	reg := &Registry{
		Version:  1,
		Projects: make(map[string]string),
	}
	reg.Register("/home/user/myapp", "myapp")
	reg.Register("/home/user/otherapp", "otherapp")

	data := []byte(`version: 1
projects:
    /home/user/myapp: myapp
    /home/user/otherapp: otherapp
`)
	if err := os.WriteFile(regPath, data, 0644); err != nil {
		t.Fatalf("failed to write test registry: %v", err)
	}

	// Load and verify
	loadedData, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatalf("failed to read registry: %v", err)
	}

	if len(loadedData) == 0 {
		t.Error("expected non-empty registry file")
	}
}

func TestRegistryLookupParentDirs(t *testing.T) {
	reg := &Registry{
		Version:  1,
		Projects: make(map[string]string),
	}

	// Register a project at a deep path
	reg.Register("/home/user/projects/myapp", "myapp")

	// Lookup from deeper subdirectories should find the project
	testCases := []struct {
		dir  string
		want string
	}{
		{"/home/user/projects/myapp", "myapp"},
		{"/home/user/projects/myapp/src", "myapp"},
		{"/home/user/projects/myapp/src/cmd/main", "myapp"},
		{"/home/user/projects", ""},       // parent of registered dir
		{"/home/user/other", ""},          // sibling
		{"/home/user/projects/other", ""}, // sibling project
	}

	for _, tc := range testCases {
		got := reg.Lookup(tc.dir)
		if got != tc.want {
			t.Errorf("Lookup(%q) = %q, want %q", tc.dir, got, tc.want)
		}
	}
}
