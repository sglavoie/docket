package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

func TestContextNew_CustomContextExists(t *testing.T) {
	t.Run("custom context exists after creation", func(t *testing.T) {
		cfg := testutil.NewTestConfig()

		cfg.SetCustomContext("Work", []string{"id1", "id2"})

		if !cfg.CustomContextExists("Work") {
			t.Error("expected CustomContextExists('Work') to be true")
		}
	})

	t.Run("nonexistent custom context returns false", func(t *testing.T) {
		cfg := testutil.NewTestConfig()

		cfg.SetCustomContext("Work", []string{"id1", "id2"})

		if cfg.CustomContextExists("Nonexistent") {
			t.Error("expected CustomContextExists('Nonexistent') to be false")
		}
	})

	t.Run("GetCustomContext returns correct project IDs", func(t *testing.T) {
		cfg := testutil.NewTestConfig()

		cfg.SetCustomContext("Work", []string{"id1", "id2"})

		projectIDs := cfg.GetCustomContext("Work")
		if projectIDs == nil {
			t.Fatal("expected GetCustomContext('Work') to return non-nil")
		}
		if len(projectIDs) != 2 {
			t.Errorf("expected 2 project IDs, got %d", len(projectIDs))
		}
		if projectIDs[0] != "id1" || projectIDs[1] != "id2" {
			t.Errorf("expected project IDs [id1, id2], got %v", projectIDs)
		}
	})

	t.Run("GetCustomContext returns nil for nonexistent context", func(t *testing.T) {
		cfg := testutil.NewTestConfig()

		projectIDs := cfg.GetCustomContext("DoesNotExist")
		if projectIDs != nil {
			t.Errorf("expected nil for nonexistent context, got %v", projectIDs)
		}
	})
}

func TestContextNew_CaseInsensitive(t *testing.T) {
	t.Run("CustomContextExists is case-insensitive", func(t *testing.T) {
		cfg := testutil.NewTestConfig()

		cfg.SetCustomContext("Work", []string{"id1"})

		if !cfg.CustomContextExists("work") {
			t.Error("expected CustomContextExists('work') to match 'Work' (case-insensitive)")
		}
		if !cfg.CustomContextExists("WORK") {
			t.Error("expected CustomContextExists('WORK') to match 'Work' (case-insensitive)")
		}
		if !cfg.CustomContextExists("Work") {
			t.Error("expected CustomContextExists('Work') to match 'Work'")
		}
	})

	t.Run("GetCustomContext is case-insensitive", func(t *testing.T) {
		cfg := testutil.NewTestConfig()

		cfg.SetCustomContext("Personal", []string{"p1", "p2"})

		projectIDs := cfg.GetCustomContext("personal")
		if projectIDs == nil {
			t.Fatal("expected GetCustomContext('personal') to find 'Personal' (case-insensitive)")
		}
		if len(projectIDs) != 2 {
			t.Errorf("expected 2 project IDs, got %d", len(projectIDs))
		}
	})

	t.Run("SetCustomContext updates existing entry case-insensitively", func(t *testing.T) {
		cfg := testutil.NewTestConfig()

		cfg.SetCustomContext("Work", []string{"id1"})
		cfg.SetCustomContext("work", []string{"id1", "id2", "id3"})

		// Should have updated the existing entry, not created a duplicate
		names := cfg.GetCustomContextNames()
		count := 0
		for _, name := range names {
			if name == "Work" || name == "work" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected exactly 1 context entry for 'work'/'Work', got %d", count)
		}

		projectIDs := cfg.GetCustomContext("work")
		if len(projectIDs) != 3 {
			t.Errorf("expected 3 project IDs after update, got %d", len(projectIDs))
		}
	})
}
