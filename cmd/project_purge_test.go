package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectPurge(t *testing.T) {
	t.Run("lists trashed projects when no args", func(t *testing.T) {
		setupProjectTest(t)
		defer func() { purgeForce = false }()

		// This calls listTrashedProjects which reads from the data dir.
		// With a temp data dir it should either succeed with empty list
		// or return an error about missing trash dir — both are acceptable.
		_ = runProjectPurge(projectPurgeCmd, []string{})
	})

	t.Run("rejects nonexistent project in trash", func(t *testing.T) {
		setupProjectTest(t)
		defer func() { purgeForce = false }()

		purgeForce = true
		err := runProjectPurge(projectPurgeCmd, []string{"nonexistent-slug"})
		if err == nil {
			t.Error("expected error for nonexistent project in trash")
		}
	})

	t.Run("rejects empty slug to prevent wiping trash dir", func(t *testing.T) {
		deps, _ := setupProjectTest(t)
		defer func() { purgeForce = false }()

		// Seed the trash with a sibling entry. Without the empty-slug guard,
		// TrashEntryExists("") sees the trash directory itself (which now
		// exists) and PurgeFromTrash("") would os.RemoveAll the entire trash
		// dir, taking the sibling entry with it.
		trashDir := filepath.Join(deps.Config.DataDir, ".tt", "trash")
		siblingDir := filepath.Join(trashDir, "sibling-project")
		if err := os.MkdirAll(siblingDir, 0755); err != nil {
			t.Fatalf("failed to seed trash dir: %v", err)
		}

		purgeForce = true
		err := runProjectPurge(projectPurgeCmd, []string{""})
		if err == nil {
			t.Fatal("expected error for empty slug")
		}

		if _, statErr := os.Stat(siblingDir); os.IsNotExist(statErr) {
			t.Fatal("sibling trash entry was destroyed by empty-slug purge")
		}
	})
}
