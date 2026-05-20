package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestCustomContextExists_EmptyContextStillExists is a regression test for a
// bug where CustomContextExists returned false for a context whose value was
// an empty or nil slice. The function previously delegated to
// GetCustomContext(name) != nil, which conflated "not in map" with
// "in map but holds nil/empty slice".
//
// Real-world scenario: RemoveProject filters projects out of every custom
// context's ID list; if a context had only that single project the resulting
// slice is empty, but the context still exists logically and must continue to
// block name collisions in `tt context new`.
func TestCustomContextExists_EmptyContextStillExists(t *testing.T) {
	cases := []struct {
		name  string
		value []string
	}{
		{"empty slice", []string{}},
		{"nil slice", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.CustomContexts = map[string][]string{"Work": tc.value}

			if !cfg.CustomContextExists("Work") {
				t.Error("expected CustomContextExists to be true for empty/nil context value")
			}
			// Case-insensitive lookup must agree.
			if !cfg.CustomContextExists("work") {
				t.Error("expected case-insensitive CustomContextExists to find 'work'")
			}
			if !cfg.CustomContextExists("WORK") {
				t.Error("expected case-insensitive CustomContextExists to find 'WORK'")
			}
		})
	}
}

// TestCustomContextExists_AfterRemoveProjectEmptiesContext exercises the
// realistic flow: a custom context contains exactly one project; that project
// is removed; the context now holds an empty slice. The context must still be
// reported as existing so that duplicate-name guards still trip.
func TestCustomContextExists_AfterRemoveProjectEmptiesContext(t *testing.T) {
	cfg := NewConfig()
	cfg.AddProject(&models.Project{ID: "p1", Slug: "proj1", Name: "P1"})
	cfg.CustomContexts = map[string][]string{"Work": {"p1"}}

	if !cfg.RemoveProject("p1") {
		t.Fatal("RemoveProject returned false")
	}

	// After removal, Work should be empty but still in the map.
	if _, present := cfg.CustomContexts["Work"]; !present {
		t.Fatal("Work key should still be in CustomContexts after removing its only project")
	}
	if !cfg.CustomContextExists("Work") {
		t.Error("expected CustomContextExists('Work') to remain true after project removal")
	}
}

// TestCustomContextExists_TrueAbsence verifies the negative path still works
// (no regression in the "missing" case after fixing the empty/nil case).
func TestCustomContextExists_TrueAbsence(t *testing.T) {
	cfg := NewConfig()
	if cfg.CustomContextExists("never-added") {
		t.Error("expected false for never-added context")
	}
	cfg.CustomContexts = map[string][]string{"Work": {"p1"}}
	if cfg.CustomContextExists("Personal") {
		t.Error("expected false for unrelated context key")
	}
}
