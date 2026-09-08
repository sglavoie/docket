package app_test

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
)

// TestService_AddTask_NormalizesContext pins the optional-field branch:
// AddTaskRequest.Context takes the same canonicalization a captured ##context
// takes, so the expanded form can hand the raw field value straight through.
func TestService_AddTask_NormalizesContext(t *testing.T) {
	project := newProject("proj-context", "Proj")
	svc := newTestService(newFakeRepo(project))

	res, err := svc.AddTask(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "Ship release",
		Context:   "Client Acme",
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	task := project.GetTaskByID(res.TaskID)
	if task == nil {
		t.Fatal("created task missing from project")
	}
	if task.Context != "client-acme" {
		t.Fatalf("stored context = %q, want client-acme", task.Context)
	}
}

// TestService_AddTask_EmptyContextLeavesFieldUnset guards that an omitted
// context is the unset field rather than a validation error, which is what
// makes clearing the form row a legal create.
func TestService_AddTask_EmptyContextLeavesFieldUnset(t *testing.T) {
	project := newProject("proj-no-context", "Proj")
	svc := newTestService(newFakeRepo(project))

	res, err := svc.AddTask(app.AddTaskRequest{ProjectID: project.ID, Title: "No context"})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	task := project.GetTaskByID(res.TaskID)
	if task == nil {
		t.Fatal("created task missing from project")
	}
	if task.Context != "" {
		t.Fatalf("stored context = %q, want unset", task.Context)
	}
}

// TestService_AddTask_RejectsUnspellableContext keeps the refusal in the
// application layer, so a surface that skips its own mirror still cannot write
// a context the canonical writer would refuse.
func TestService_AddTask_RejectsUnspellableContext(t *testing.T) {
	project := newProject("proj-bad-context", "Proj")
	svc := newTestService(newFakeRepo(project))

	_, err := svc.AddTask(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "Ship release",
		Context:   "!!!",
	})
	if err == nil {
		t.Fatal("an unspellable context must be refused")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Fatalf("refusal should name the context field: %v", err)
	}
	if len(project.Tasks) != 0 {
		t.Fatalf("a refused add wrote %d tasks", len(project.Tasks))
	}
}

// TestService_AddTaskWithChildren_ContextStaysOnTheParent records that a
// subtask inherits priority, state, and tags and nothing else: the context is a
// singleton field the form set on the parent alone.
func TestService_AddTaskWithChildren_ContextStaysOnTheParent(t *testing.T) {
	project := newProject("proj-context-children", "Proj")
	svc := newTestService(newFakeRepo(project))

	res, err := svc.AddTaskWithChildren(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "Parent",
		Context:   "client-acme",
		Subtasks:  []app.AddSubtaskRequest{{Title: "Child"}},
	})
	if err != nil {
		t.Fatalf("AddTaskWithChildren: %v", err)
	}
	parent := project.GetTaskByID(res.Task.TaskID)
	if parent == nil || parent.Context != "client-acme" {
		t.Fatalf("parent context = %v", parent)
	}
	if len(res.Subtasks) != 1 {
		t.Fatalf("subtask results = %d, want 1", len(res.Subtasks))
	}
	child := project.GetTaskByID(res.Subtasks[0].TaskID)
	if child == nil {
		t.Fatal("created subtask missing from project")
	}
	if child.Context != "" {
		t.Fatalf("subtask context = %q, want unset", child.Context)
	}
}
