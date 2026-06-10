package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/scoring"
)

func TestConfig_ScoringProfileCRUD(t *testing.T) {
	cfg := NewConfig()

	// Initially no profiles
	if p := cfg.GetScoringProfile("ice"); p != nil {
		t.Error("expected nil for non-existent profile")
	}

	// Add a profile
	profile := scoring.NewICEProfile()
	if err := cfg.AddScoringProfile(profile); err != nil {
		t.Fatalf("AddScoringProfile failed: %v", err)
	}

	// Retrieve it
	got := cfg.GetScoringProfile("ice")
	if got == nil {
		t.Fatal("expected to find 'ice' profile")
	}
	if got.Name != profile.Name {
		t.Errorf("expected name %q, got %q", profile.Name, got.Name)
	}

	// Duplicate add should fail
	if err := cfg.AddScoringProfile(profile); err == nil {
		t.Error("expected error when adding duplicate profile")
	}

	// Remove it
	if !cfg.RemoveScoringProfile("ice") {
		t.Error("RemoveScoringProfile should return true")
	}
	if cfg.GetScoringProfile("ice") != nil {
		t.Error("profile should be gone after removal")
	}

	// Remove non-existent
	if cfg.RemoveScoringProfile("nonexistent") {
		t.Error("RemoveScoringProfile should return false for non-existent")
	}
}

func TestConfig_ProjectScoringProfile(t *testing.T) {
	cfg := NewConfig()

	// No project scoring profile initially
	if p := cfg.GetProjectScoringProfile("proj1"); p != nil {
		t.Error("expected nil for unassigned project")
	}

	// Add a scoring profile first
	profile := scoring.NewRICEProfile()
	if err := cfg.AddScoringProfile(profile); err != nil {
		t.Fatalf("AddScoringProfile failed: %v", err)
	}

	// Assign to project
	cfg.SetProjectScoringProfile("proj1", "rice")

	got := cfg.GetProjectScoringProfile("proj1")
	if got == nil {
		t.Fatal("expected to find RICE profile for proj1")
	}
	if got.ID != "rice" {
		t.Errorf("expected profile ID 'rice', got %q", got.ID)
	}

	// Different project has no assignment
	if p := cfg.GetProjectScoringProfile("proj2"); p != nil {
		t.Error("expected nil for unassigned proj2")
	}

	// Unset the assignment
	cfg.UnsetProjectScoringProfile("proj1")
	if p := cfg.GetProjectScoringProfile("proj1"); p != nil {
		t.Error("expected nil after unset")
	}

	// Unset on nil map should not panic
	cfg.ProjectScoringProfile = nil
	cfg.UnsetProjectScoringProfile("anything")
	if p := cfg.GetProjectScoringProfile("anything"); p != nil {
		t.Error("expected nil from nil map")
	}
}

func TestConfig_ProjectScoringProfile_DanglingReference(t *testing.T) {
	cfg := NewConfig()

	// Assign a profile that doesn't exist
	cfg.SetProjectScoringProfile("proj1", "nonexistent")

	// Should return nil (dangling reference)
	if p := cfg.GetProjectScoringProfile("proj1"); p != nil {
		t.Error("expected nil for dangling profile reference")
	}
}

func TestConfig_GetEditor(t *testing.T) {
	cfg := NewConfig()

	// Default editor
	if cfg.GetEditor() != "code" {
		t.Errorf("expected default editor 'code', got %q", cfg.GetEditor())
	}

	// Custom editor
	cfg.Editor = "vim"
	if cfg.GetEditor() != "vim" {
		t.Errorf("expected editor 'vim', got %q", cfg.GetEditor())
	}
}

func TestConfig_GetCodeEditor(t *testing.T) {
	cfg := NewConfig()
	cfg.Editor = "vim"

	// Default code editor falls back to the resolved general editor.
	if cfg.GetCodeEditor() != "vim" {
		t.Errorf("expected fallback code editor 'vim', got %q", cfg.GetCodeEditor())
	}

	// Custom code editor
	cfg.CodeEditor = "nvim"
	if cfg.GetCodeEditor() != "nvim" {
		t.Errorf("expected code editor 'nvim', got %q", cfg.GetCodeEditor())
	}
}

func TestConfig_GetExternalEditor(t *testing.T) {
	cfg := NewConfig()
	cfg.Editor = "vim"

	if cfg.GetExternalEditor() != "code" {
		t.Errorf("expected default external editor 'code', got %q", cfg.GetExternalEditor())
	}

	cfg.ExternalEditor = "zed"
	if cfg.GetExternalEditor() != "zed" {
		t.Errorf("expected external editor 'zed', got %q", cfg.GetExternalEditor())
	}
}

func TestConfig_GetAttachmentEditor(t *testing.T) {
	cfg := NewConfig()

	// Default is empty
	if cfg.GetAttachmentEditor() != "" {
		t.Errorf("expected empty default attachment editor, got %q", cfg.GetAttachmentEditor())
	}

	// Custom
	cfg.AttachmentEditor = "open"
	if cfg.GetAttachmentEditor() != "open" {
		t.Errorf("expected 'open', got %q", cfg.GetAttachmentEditor())
	}
}

func TestConfig_LoadWithExtraFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// Write config with unknown fields (simulates future version)
	content := `{
		"version": 1,
		"editor": "vim",
		"unknown_future_field": "some value",
		"another_unknown": 42,
		"nested_unknown": {"key": "value"}
	}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load should handle unknown fields gracefully: %v", err)
	}

	// Known fields should be loaded
	if cfg.Editor != "vim" {
		t.Errorf("expected editor 'vim', got %q", cfg.Editor)
	}
}

func TestConfig_LoadWithNullValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// Write config with null values for pointer fields
	content := `{
		"version": 1,
		"editor": "vim",
		"projects": null,
		"section_colors": null,
		"scoring_profiles": null,
		"project_scoring_profile": null
	}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load should handle null values: %v", err)
	}

	// Nil slices should be initialized
	if cfg.Projects == nil {
		t.Error("Projects should be initialized to empty slice")
	}
	if cfg.SectionColors == nil {
		t.Error("SectionColors should be initialized")
	}
}
