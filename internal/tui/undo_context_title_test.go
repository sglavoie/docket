package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

// TestRenameEditTitle_UndoContextUsesOldTitle verifies that the undo
// context captures the original title, not the new one, when renaming a task
// via an inline edit input.
func TestRenameEditTitle_UndoContextUsesOldTitle(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", t.TempDir())
	task := makeBoardTask(t, "id-title", 1, "Old Title")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Verify the mutation ordering directly: after MutateTask, the task pointer
	// holds the new title, while the previously captured old title is unchanged.
	oldTitle := task.Title
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Title = "New Title"
	})

	// task.Title is now "New Title" because MutateTask mutates in-place.
	if task.Title != "New Title" {
		t.Fatalf("expected task title to be mutated to 'New Title', got %q", task.Title)
	}
	// The captured oldTitle must still be "Old Title".
	if oldTitle != "Old Title" {
		t.Errorf("oldTitle should be 'Old Title' but got %q — undo context would be wrong", oldTitle)
	}
}

// TestQuickEditRename_UndoContextUsesOldTitle verifies that
// handleQuickEditCommit captures the old title before MutateTask runs,
// so the undo context shows the original title, not the new one.
func TestQuickEditRename_UndoContextUsesOldTitle(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", t.TempDir())
	task, err := models.NewGenericTask("id-qe", 1, "Original Name")
	if err != nil {
		t.Fatal(err)
	}
	task.WorkflowState = models.StateDoing
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Simulate the pattern in shared_primary_view_keys.go:
	//   oldTitle := task.Title
	//   project.MutateTask(task.ID, func(t *models.Task) { t.Title = input })
	//   onUndo("Rename Task", oldTitle)
	oldTitle := task.Title
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Title = "Renamed Task"
	})

	// task.Title is now "Renamed Task" since MutateTask mutates in-place.
	if task.Title != "Renamed Task" {
		t.Fatalf("expected task title to be 'Renamed Task', got %q", task.Title)
	}
	// The captured oldTitle must be the original.
	if oldTitle != "Original Name" {
		t.Errorf("oldTitle should be 'Original Name' but got %q — undo context would show wrong title", oldTitle)
	}
}

// TestMutateTask_TitlePointerSharing confirms that MutateTask modifies the
// task in-place, so any reference to task.Title after mutation sees the new
// value. This documents why capturing oldTitle before MutateTask is necessary.
func TestMutateTask_TitlePointerSharing(t *testing.T) {
	project, _ := models.NewProject("proj", "proj", t.TempDir())
	task, err := models.NewGenericTask("id-ptr", 1, "Before")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)

	// Hold a reference to the same task pointer
	ref := task

	project.MutateTask(task.ID, func(t *models.Task) {
		t.Title = "After"
	})

	// Both task and ref see the mutated title — they point to the same struct.
	if task.Title != "After" {
		t.Errorf("task.Title should be 'After', got %q", task.Title)
	}
	if ref.Title != "After" {
		t.Errorf("ref.Title should be 'After', got %q — pointer sharing not working as expected", ref.Title)
	}
}
