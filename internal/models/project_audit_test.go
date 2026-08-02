package models

import (
	"strings"
	"testing"
)

func newAuditTestProject(t *testing.T) *Project {
	t.Helper()

	project, err := NewProject("audit-project", "Audit Project", ".")
	if err != nil {
		t.Fatal(err)
	}

	return project
}

func newAuditTestTask(t *testing.T, id string, shortID int, taskType string, title string) *Task {
	t.Helper()

	task, err := NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatal(err)
	}

	return task
}

func requireAuditErrorContains(t *testing.T, project *Project, want string) {
	t.Helper()

	err := AuditProject(project)
	if err == nil {
		t.Fatalf("AuditProject() error = nil, want substring %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("AuditProject() error = %q, want substring %q", err.Error(), want)
	}
}

func TestAuditProjectValidProject(t *testing.T) {
	project := newAuditTestProject(t)

	epic := newAuditTestTask(t, "epic-1", 1, "", "Epic")
	story := newAuditTestTask(t, "story-1", 2, "", "Story")
	bug := newAuditTestTask(t, "bug-1", 3, "", "Bug")

	story.ParentID = &epic.ID
	bug.ParentID = &story.ID
	story.ApplyWorkflowState(StateBacklog)
	bug.ApplyWorkflowState(StateWaiting)

	project.Tasks = []*Task{epic, story, bug}
	project.BuildTaskTree()

	if err := AuditProject(project); err != nil {
		t.Fatalf("AuditProject() error = %v, want nil", err)
	}
}

func TestAuditProjectRejectsEmptyTaskID(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "", 1, "", "Missing ID")
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "empty task ID")
}

func TestAuditProjectRejectsDuplicateTaskID(t *testing.T) {
	project := newAuditTestProject(t)
	first := newAuditTestTask(t, "dup", 1, "", "First")
	second := newAuditTestTask(t, "dup", 2, "", "Second")
	project.Tasks = []*Task{first, second}

	requireAuditErrorContains(t, project, "duplicate task ID")
}

func TestAuditProjectRejectsCyclicParentGraph(t *testing.T) {
	project := newAuditTestProject(t)
	parent := newAuditTestTask(t, "parent", 1, "", "Parent")
	child := newAuditTestTask(t, "child", 2, "", "Child")
	parent.ParentID = &child.ID
	child.ParentID = &parent.ID
	project.Tasks = []*Task{parent, child}

	requireAuditErrorContains(t, project, "cyclic parent chain")
}

func TestAuditProjectRejectsInvalidWorkflowState(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "task-1", 1, "", "Invalid State")
	task.WorkflowState = WorkflowState("sprint-1")
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "invalid workflow state")
}
