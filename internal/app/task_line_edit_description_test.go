package app_test

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestTaskLineEditCommitsDescriptionWithTheTitlePatch is what lets the external
// editor route its title through the grammar: the body it returns rides the same
// request, so title, tags, destination, and description are one transaction and
// one undo step rather than two.
func TestTaskLineEditCommitsDescriptionWithTheTitlePatch(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "Before")
	task := project.Tasks[0]
	task.Description = "Original body."
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	seeded, err := ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("UndoCount: %v", err)
	}

	description := "Body written in the editor."
	destination := app.CaptureDestinationBacklog
	result, err := app.NewServiceWithArtifacts(repo, nil, nil, nil).EditTaskLine(app.TaskLineEditRequest{
		Source:      app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:       "After",
		Description: &description,
		Destination: &destination,
		TagsToAdd:   []string{"urgent"},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if !result.Changed || result.TransactionID == "" {
		t.Fatalf("result = %+v, want a committed change", result)
	}

	committed := reloadedTaskLineEditTask(t, project, task.ID)
	if committed.Title != "After" || committed.Description != description {
		t.Fatalf("committed title/description = %q/%q", committed.Title, committed.Description)
	}
	if !committed.HasTag("urgent") || committed.State() != models.StateBacklog {
		t.Fatalf("committed tags/state = %v/%s", committed.Tags, committed.State())
	}

	after, err := ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("UndoCount: %v", err)
	}
	if after-seeded != 1 {
		t.Fatalf("edit recorded %d undo entries, want 1", after-seeded)
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	undone := reloadedTaskLineEditTask(t, project, task.ID)
	if undone.Title != "Before" || undone.Description != "Original body." || undone.HasTag("urgent") {
		t.Fatalf("one undo left title=%q description=%q tags=%v", undone.Title, undone.Description, undone.Tags)
	}
}

// TestTaskLineEditReportsADescriptionOnlySaveAsChanged keeps a body-only edit off
// the no-op path, where the shell would report "No task changes" and drop it.
func TestTaskLineEditReportsADescriptionOnlySaveAsChanged(t *testing.T) {
	dataDir := t.TempDir()
	commitSvc := storage.NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir})

	project := realTestProject(t, dataDir, "alpha", "Unchanged")
	task := project.Tasks[0]
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	description := "Only the body moved."
	result, err := app.NewServiceWithArtifacts(repo, nil, nil, nil).EditTaskLine(app.TaskLineEditRequest{
		Source:      app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:       "Unchanged",
		Description: &description,
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if !result.Changed {
		t.Fatal("a description-only save reported no change")
	}
	if got := reloadedTaskLineEditTask(t, project, task.ID).Description; got != description {
		t.Fatalf("committed description = %q, want %q", got, description)
	}
}

func reloadedTaskLineEditTask(t *testing.T, project *models.Project, taskID string) *models.Task {
	t.Helper()
	task := loadTaskLineEditProject(t, project).GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("reloaded task %s is missing", taskID)
	}
	return task
}
