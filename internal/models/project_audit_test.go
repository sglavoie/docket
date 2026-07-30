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
	story.ApplyWorkflowState(StateNext)
	bug.ApplyWorkflowState(StateWaiting)
	bug.DependsOn = []string{epic.ID, "other-project:external-blocker"}

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

func TestAuditProjectRejectsSelfDependency(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "task-1", 1, "", "Self Dep")
	task.DependsOn = []string{task.ID}
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "depends on itself")
}

func TestAuditProjectRejectsDanglingLocalDependency(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "task-1", 1, "", "Dangling Dep")
	task.DependsOn = []string{"missing-task"}
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "depends on missing local task")
}

func TestAuditProjectRejectsDependencyCycle(t *testing.T) {
	project := newAuditTestProject(t)
	taskA := newAuditTestTask(t, "a", 1, "", "A")
	taskB := newAuditTestTask(t, "b", 2, "", "B")
	taskC := newAuditTestTask(t, "c", 3, "", "C")
	taskA.DependsOn = []string{taskB.ID}
	taskB.DependsOn = []string{taskC.ID}
	taskC.DependsOn = []string{taskA.ID}
	project.Tasks = []*Task{taskA, taskB, taskC}

	requireAuditErrorContains(t, project, "cyclic dependency")
}

func TestAuditProjectRejectsInvalidWorkflowState(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "task-1", 1, "", "Invalid State")
	task.WorkflowState = WorkflowState("sprint-1")
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "invalid workflow state")
}
