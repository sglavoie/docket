package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg.DataDir == "" {
		t.Error("DataDir should not be empty")
	}
	if cfg.Editor != "code" {
		t.Errorf("expected default Editor 'code', got '%s'", cfg.Editor)
	}
	if cfg.DefaultContext != "personal" {
		t.Errorf("expected default Context 'personal', got '%s'", cfg.DefaultContext)
	}
	if !cfg.FzfEnabled {
		t.Error("FzfEnabled should default to true")
	}
	if !cfg.GlowEnabled {
		t.Error("GlowEnabled should default to true")
	}
	if cfg.Projects == nil {
		t.Error("Projects should not be nil")
	}
	if cfg.SectionColors == nil {
		t.Error("NewConfig should initialize SectionColors map")
	}
	if cfg.Version != 1 {
		t.Errorf("expected Version 1, got %d", cfg.Version)
	}
}

func TestConfig_Projects(t *testing.T) {
	cfg := NewConfig()

	project1 := &models.Project{
		ID:   "p1",
		Slug: "project-one",
		Name: "Project One",
	}
	project2 := &models.Project{
		ID:   "p2",
		Slug: "project-two",
		Name: "Project Two",
	}

	cfg.AddProject(project1)
	cfg.AddProject(project2)

	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(cfg.Projects))
	}

	found := cfg.GetProject("p1")
	if found == nil || found.ID != "p1" {
		t.Error("GetProject by ID failed")
	}

	found = cfg.GetProject("project-two")
	if found == nil || found.ID != "p2" {
		t.Error("GetProject by slug failed")
	}

	found = cfg.GetProject("Project One")
	if found != nil {
		t.Error("GetProject by name should return nil (only ID and slug are supported)")
	}

	found = cfg.GetProject("non-existent")
	if found != nil {
		t.Error("expected nil for non-existent project")
	}

	removed := cfg.RemoveProject("p1")
	if !removed {
		t.Error("RemoveProject should return true")
	}
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project after removal, got %d", len(cfg.Projects))
	}

	removed = cfg.RemoveProject("non-existent")
	if removed {
		t.Error("RemoveProject should return false for non-existent")
	}
}

func TestConfig_CurrentProject(t *testing.T) {
	cfg := NewConfig()

	project := &models.Project{
		ID:   "p1",
		Slug: "test-project",
		Name: "Test Project",
	}
	cfg.AddProject(project)

	if cfg.GetCurrentProject() != nil {
		t.Error("expected nil current project initially")
	}

	if !cfg.SetCurrentProject("p1") {
		t.Error("SetCurrentProject should return true")
	}
	if cfg.CurrentProject != "p1" {
		t.Errorf("expected CurrentProject 'p1', got '%s'", cfg.CurrentProject)
	}

	current := cfg.GetCurrentProject()
	if current == nil || current.ID != "p1" {
		t.Error("GetCurrentProject failed")
	}

	if cfg.SetCurrentProject("non-existent") {
		t.Error("SetCurrentProject should return false for non-existent")
	}
	if cfg.CurrentProject != "p1" {
		t.Error("CurrentProject should not change on failed set")
	}

	cfg.RemoveProject("p1")
	if cfg.CurrentProject != "" {
		t.Error("CurrentProject should be cleared when project is removed")
	}
}

func TestConfig_GetActiveArchivedProjects(t *testing.T) {
	cfg := NewConfig()

	active1 := &models.Project{ID: "a1", Name: "Active 1", Archived: false}
	active2 := &models.Project{ID: "a2", Name: "Active 2", Archived: false}
	archived := &models.Project{ID: "ar1", Name: "Archived 1", Archived: true}

	cfg.AddProject(active1)
	cfg.AddProject(active2)
	cfg.AddProject(archived)

	activeProjects := cfg.GetActiveProjects()
	if len(activeProjects) != 2 {
		t.Errorf("expected 2 active projects, got %d", len(activeProjects))
	}

	archivedProjects := cfg.GetArchivedProjects()
	if len(archivedProjects) != 1 {
		t.Errorf("expected 1 archived project, got %d", len(archivedProjects))
	}
}

func TestConfig_EnsureDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data", "nested")

	cfg := NewConfig()
	cfg.DataDir = dataDir

	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir failed: %v", err)
	}

	info, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		t.Error("data directory was not created")
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestConfig_GetProjectPath(t *testing.T) {
	cfg := NewConfig()
	cfg.DataDir = "/home/user/.tt"

	path := cfg.GetProjectPath("my-project")
	expected := "/home/user/.tt/projects/my-project"

	if path != expected {
		t.Errorf("expected path '%s', got '%s'", expected, path)
	}
}
