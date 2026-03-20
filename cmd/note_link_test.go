package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetNoteLinkVars() {
	noteLinkRemove = false
}

func setupNoteLinkTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	resetNoteLinkVars()

	deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := manager.Save(deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	return deps, project
}

// writeNoteToProject writes a note to the project's notes directory on disk,
// so that findNote (which uses the filesystem) can locate it.
func writeNoteToProject(t *testing.T, project *models.Project, note *models.Note) {
	t.Helper()
	notesDir := storage.NotesDir(project)
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		t.Fatalf("failed to create notes dir: %v", err)
	}
	content, err := storage.RenderNoteContent(note)
	if err != nil {
		t.Fatalf("failed to render note: %v", err)
	}
	path := filepath.Join(notesDir, note.Filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write note file: %v", err)
	}
}

func TestRunNoteLink(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		resetNoteLinkVars()
		err := runNoteLink(noteLinkCmd, []string{})
		if err == nil {
			t.Error("expected error for missing note ID and task ID")
		}
	})

	t.Run("rejects with only one arg", func(t *testing.T) {
		resetNoteLinkVars()
		err := runNoteLink(noteLinkCmd, []string{"some-note"})
		if err == nil {
			t.Error("expected error when task ID is missing")
		}
	})

	t.Run("adds link to task", func(t *testing.T) {
		deps, project := setupNoteLinkTest(t)
		defer deps.Cleanup()

		note := models.NewNote("Meeting Notes")
		writeNoteToProject(t, project, note)

		// ShortID 1 is the epic "Setup Infrastructure"
		err := runNoteLink(noteLinkCmd, []string{"Meeting", "1"})
		testutil.AssertNoError(t, err)

		// Reload the note from disk and verify the link was added
		reloaded, err := storage.LoadNote(project, note.Filename)
		testutil.AssertNoError(t, err)

		// Find the task to get its ID
		task := project.GetTaskByShortID(1)
		if task == nil {
			t.Fatal("expected task with ShortID 1")
		}
		if !reloaded.HasLink(task.ID) {
			t.Errorf("expected note to have link to task %s", task.ID)
		}
	})

	t.Run("removes link", func(t *testing.T) {
		deps, project := setupNoteLinkTest(t)
		defer deps.Cleanup()

		// Find the task first to get its ID
		task := project.GetTaskByShortID(1)
		if task == nil {
			t.Fatal("expected task with ShortID 1")
		}

		note := models.NewNote("Sprint Notes")
		note.AddLink(task.ID)
		writeNoteToProject(t, project, note)

		noteLinkRemove = true

		err := runNoteLink(noteLinkCmd, []string{"Sprint", "1"})
		testutil.AssertNoError(t, err)

		// Reload and verify the link was removed
		reloaded, err := storage.LoadNote(project, note.Filename)
		testutil.AssertNoError(t, err)
		if reloaded.HasLink(task.ID) {
			t.Errorf("expected link to task %s to be removed", task.ID)
		}
	})

	t.Run("rejects note not found", func(t *testing.T) {
		deps, _ := setupNoteLinkTest(t)
		defer deps.Cleanup()

		err := runNoteLink(noteLinkCmd, []string{"nonexistent-note-xyz", "1"})
		if err == nil {
			t.Error("expected error for nonexistent note")
		}
	})

	t.Run("rejects task not found", func(t *testing.T) {
		deps, project := setupNoteLinkTest(t)
		defer deps.Cleanup()

		note := models.NewNote("Orphan Note")
		writeNoteToProject(t, project, note)

		err := runNoteLink(noteLinkCmd, []string{"Orphan", "9999"})
		if err == nil {
			t.Error("expected error for nonexistent task")
		}
	})
}
