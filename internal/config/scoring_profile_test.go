package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/scoring"
)

func TestConfig_ScoringProfileCRUD(t *testing.T) {
	cfg := NewAutomationConfig()

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
	cfg := NewAutomationConfig()

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
	cfg := NewAutomationConfig()

	// Assign a profile that doesn't exist
	cfg.SetProjectScoringProfile("proj1", "nonexistent")

	// Should return nil (dangling reference)
	if p := cfg.GetProjectScoringProfile("proj1"); p != nil {
		t.Error("expected nil for dangling profile reference")
	}
}
