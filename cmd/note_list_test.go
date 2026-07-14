package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetNoteListVars() {
	noteListTag = ""
	_ = noteListCmd.Flags().Set("json", "false")
	_ = noteListCmd.Flags().Set("quiet", "false")
	noteGlobal = false
}

func setupNoteListTest(t *testing.T) (*testDeps, *models.Project) {
	t.Helper()
	resetNoteListVars()

	deps, project := setupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := saveTestRuntime(manager, deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	return deps, project
}

func TestRunNoteList(t *testing.T) {
	t.Run("runs without error for project with no notes", func(t *testing.T) {
		deps, _ := setupNoteListTest(t)
		defer deps.Cleanup()

		err := runNoteList(noteListCmd, []string{})
		testutil.AssertNoError(t, err)
	})
}

func TestNoteListJSONNilSlices(t *testing.T) {
	deps, project := setupNoteListTest(t)
	defer deps.Cleanup()

	// Create a note with nil Tags and nil LinkedTasks (as would happen
	// when parsed from YAML with omitempty and no tags/linked_tasks fields).
	note := models.NewNote("Test Note")
	note.Tags = nil
	note.LinkedTasks = nil
	err := storage.SaveNote(project, note)
	testutil.AssertNoError(t, err)

	// Capture JSON output
	_ = noteListCmd.Flags().Set("json", "true")
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runNoteList(noteListCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	testutil.AssertNoError(t, err)
	output := buf.String()

	// Verify nil slices are serialized as [] not null
	if strings.Contains(output, `"tags": null`) {
		t.Error("tags should be [] not null in JSON output")
	}
	if strings.Contains(output, `"linked_tasks": null`) {
		t.Error("linked_tasks should be [] not null in JSON output")
	}
	if !strings.Contains(output, `"tags": []`) {
		t.Error("expected tags: [] in JSON output")
	}
	if !strings.Contains(output, `"linked_tasks": []`) {
		t.Error("expected linked_tasks: [] in JSON output")
	}
}
