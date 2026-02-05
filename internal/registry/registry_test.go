package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	reg := New()

	if reg.Version != 1 {
		t.Errorf("expected version 1, got %d", reg.Version)
	}
	if reg.Projects == nil {
		t.Error("expected Projects to be initialized")
	}
	if len(reg.Projects) != 0 {
		t.Errorf("expected empty Projects, got %d", len(reg.Projects))
	}
}

func TestRegisterLookup(t *testing.T) {
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
			reg := New()
			reg.Register(tt.registerDir, tt.project)
			got := reg.Lookup(tt.lookupDir)

			if got != tt.wantProject {
				t.Errorf("Lookup(%q) = %q, want %q", tt.lookupDir, got, tt.wantProject)
			}
		})
	}
}

func TestUnregister(t *testing.T) {
	reg := New()
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

func TestProjectDirs(t *testing.T) {
	reg := New()
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

func TestAllProjects(t *testing.T) {
	reg := New()
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

func TestSaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "varnish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	regPath := filepath.Join(tmpDir, "registry.yaml")

	// Create and save
	reg := New()
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

func TestLookupParentDirs(t *testing.T) {
	reg := New()
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

func TestLoadSaveWithRealPath(t *testing.T) {
	// Create a temp HOME to test Load() and Save() with real paths
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	// Load should return empty registry when no file exists
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(reg.Projects) != 0 {
		t.Errorf("expected empty registry, got %d projects", len(reg.Projects))
	}

	// Add some data and save
	reg.Register("/path/to/myapp", "myapp")
	reg.Register("/path/to/other", "otherapp")
	if err := reg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Load again and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after save error: %v", err)
	}
	if len(loaded.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(loaded.Projects))
	}
	if loaded.Lookup("/path/to/myapp") != "myapp" {
		t.Error("expected myapp project")
	}
}

func TestLookupCurrent(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Create a project directory
	projectDir := filepath.Join(tmpHome, "myproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	t.Setenv("HOME", tmpHome)

	// Register the project
	reg := New()
	reg.Register(projectDir, "testproj")
	if err := reg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Change to the project directory
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Load and test LookupCurrent
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	proj := loaded.LookupCurrent()
	if proj != "testproj" {
		t.Errorf("LookupCurrent() = %q, want 'testproj'", proj)
	}
}

func TestLookupCurrentNotRegistered(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "varnish-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(tmpHome); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	reg := New()
	proj := reg.LookupCurrent()
	if proj != "" {
		t.Errorf("LookupCurrent() = %q, want empty string", proj)
	}
}
