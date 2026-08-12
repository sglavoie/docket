package app_test

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TestAddTaskWithChildrenCreatesParentAndSubtasks proves the task-form create use
// case commits a parent plus its subtasks as ONE ChangeSet, parents the subtasks,
// and inherits the parent's priority, state, and tags. A blank-title subtask is
// skipped.
func TestAddTaskWithChildrenCreatesParentAndSubtasks(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.AddTaskWithChildren(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "Parent Epic",
		Priority:  2,
		Tags:      []string{"backend"},
		State:     models.StateBacklog,
		Subtasks: []app.AddSubtaskRequest{
			{Title: "child one"},
			{Title: "  "},
			{Title: "child two"},
		},
	})
	if err != nil {
		t.Fatalf("AddTaskWithChildren: %v", err)
	}
	if len(res.Subtasks) != 2 {
		t.Fatalf("subtask results = %d, want 2 (blank skipped)", len(res.Subtasks))
	}
	if _, ok := repo.lastCommit(); !ok {
		t.Fatal("expected exactly one changeset committed")
	}

	parent := project.GetTaskByID(res.Task.TaskID)
	if parent == nil {
		t.Fatalf("parent not committed: %v", parent)
	}
	for _, cr := range res.Subtasks {
		child := project.GetTaskByID(cr.TaskID)
		if child == nil {
			t.Fatalf("subtask %s missing", cr.TaskID)
		}
		if child.ParentID == nil || *child.ParentID != parent.ID {
			t.Errorf("subtask %q not parented to the epic", child.Title)
		}
		if child.Priority != 2 {
			t.Errorf("subtask priority = %d, want inherited 2", child.Priority)
		}
		if !child.HasTag("backend") {
			t.Errorf("subtask tags = %v, want inherited backend", child.Tags)
		}
		if got := child.State(); got != models.StateBacklog {
			t.Errorf("subtask state = %q, want Backlog", got)
		}
	}
}

// TestAddTaskWithChildrenAppliesFormFields proves the create use case applies the
// task form's scheduling, flag, and checklist metadata.
func TestAddTaskWithChildrenAppliesFormFields(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	due := time.Date(2026, 8, 1, 15, 0, 0, 0, time.UTC)
	res, err := svc.AddTaskWithChildren(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "Full task",
		Priority:  3,
		State:     models.StateBacklog,
		DueDate:   &due,
		Pinned:    true,
		ChecklistItems: []models.ChecklistItem{
			{Text: "done step", Done: true},
			{Text: "  ", Done: false},
			{Text: "open step", Done: false},
		},
	})
	if err != nil {
		t.Fatalf("AddTaskWithChildren: %v", err)
	}
	task := project.GetTaskByID(res.Task.TaskID)
	if task == nil {
		t.Fatal("task not committed")
	}
	if task.DueDate == nil {
		t.Error("due date not applied")
	}
	if !task.Pinned || task.PinnedAt == nil {
		t.Error("pinned/pinnedAt not applied")
	}
	if len(task.Checklist) != 2 {
		t.Fatalf("checklist = %d, want 2 (blank filtered)", len(task.Checklist))
	}
	if !task.Checklist[0].Done {
		t.Error("first checklist item should preserve Done=true")
	}
}
