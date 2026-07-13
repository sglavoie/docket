package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskCommentsSaveCommentMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.addMode = true
	model.textInput.SetValue("New comment")

	cmd := model.saveComment()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveComment")
	}

	// Verify state was mutated synchronously before cmd() runs
	if model.addMode {
		t.Fatal("expected addMode to be false after saveComment()")
	}
	if len(model.task.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(model.task.Comments))
	}
	if model.task.Comments[0].Text != "New comment" {
		t.Fatalf("expected comment text 'New comment', got %q", model.task.Comments[0].Text)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.cursor)
	}
	if model.err != nil {
		t.Fatalf("expected no error, got %v", model.err)
	}
}

func TestTaskCommentsSaveCommentEmptyText(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.addMode = true
	model.textInput.SetValue("   ")

	cmd := model.saveComment()
	if cmd != nil {
		t.Fatal("expected nil command for empty comment")
	}
	if model.err == nil {
		t.Fatal("expected error for empty comment text")
	}
}

func TestTaskCommentsDeleteMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "comment 1", CreatedAt: time.Now()},
		{Text: "comment 2", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.cursor = 1

	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}

	// Verify state was mutated synchronously before cmd() runs
	if len(model.task.Comments) != 1 {
		t.Fatalf("expected 1 comment after delete, got %d", len(model.task.Comments))
	}
	if model.task.Comments[0].Text != "comment 1" {
		t.Fatalf("expected remaining comment to be 'comment 1', got %q", model.task.Comments[0].Text)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0 after deleting last comment, got %d", model.cursor)
	}
}

func TestTaskCommentsSaveEditMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	created := time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local)
	task.Comments = []*models.Comment{
		{Text: "original text", CreatedAt: created},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0
	model.editTextInput.SetValue("edited text")

	cmd := model.saveEditedComment()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveEditedComment")
	}

	// Verify state was mutated synchronously before cmd() runs
	if model.editMode {
		t.Fatal("expected editMode to be false after saveEditedComment()")
	}
	if model.task.Comments[0].Text != "edited text" {
		t.Fatalf("expected comment text 'edited text', got %q", model.task.Comments[0].Text)
	}
	// Timestamp should be preserved
	if !model.task.Comments[0].CreatedAt.Equal(created) {
		t.Fatalf("expected timestamp to be preserved, got %v", model.task.Comments[0].CreatedAt)
	}
	if model.err != nil {
		t.Fatalf("expected no error, got %v", model.err)
	}
}

func TestTaskCommentsRenderEditModeOOB(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "only comment", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0

	// Simulate external deletion of all comments
	task.Comments = nil

	// View() must not panic when editIndex is out of bounds
	view := model.View()
	if view.Content == "" {
		t.Fatal("expected non-empty view output")
	}
}

func TestTaskCommentsEditExternalEditorFromEditModeOOB(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "only comment", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0

	// Simulate external deletion of all comments
	task.Comments = nil

	// Must not panic; should cancel edit mode and return an error cmd
	cmd := model.editCommentInExternalEditorFromEditMode()
	if cmd == nil {
		t.Fatal("expected non-nil error command when editIndex is out of bounds")
	}
	if model.editMode {
		t.Fatal("expected editMode to be canceled when comment was deleted externally")
	}
}

func TestTaskCommentsSaveEditOOBAfterExternalDeletion(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "original", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0
	model.editTextInput.SetValue("edited text")

	// Simulate external deletion of all comments
	task.Comments = nil

	// saveEditedComment should detect OOB and return error cmd
	cmd := model.saveEditedComment()
	if cmd == nil {
		t.Fatal("expected non-nil error command when editIndex is out of bounds")
	}
	if model.editMode {
		t.Fatal("expected editMode to be canceled when comment was deleted externally")
	}
}

func TestTaskCommentsHasUnsavedEditChangesIgnoresSurroundingWhitespace(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	// Stored comment text has trailing whitespace (e.g., from prior editor save).
	task.Comments = []*models.Comment{
		{Text: "hello world\n", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.editMode = true
	model.editIndex = 0
	// User opens edit; textarea is populated with the stored value but trims on input.
	model.editTextInput.SetValue("hello world")

	if model.hasUnsavedEditChanges() {
		t.Fatal("expected no unsaved changes when input differs from stored text only by surrounding whitespace")
	}
}

func TestTaskCommentsToggleResolvedMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Comments = []*models.Comment{
		{Text: "comment", CreatedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskCommentsModel(task, project)
	model.coordinator = newTestSecondaryCoordinators(project).Subview
	model.cursor = 0

	cmd := model.toggleResolved()
	if cmd == nil {
		t.Fatal("expected non-nil command from toggleResolved")
	}

	// Verify state was mutated synchronously before cmd() runs
	if !model.task.Comments[0].Resolved {
		t.Fatal("expected comment to be resolved after toggleResolved()")
	}
}
