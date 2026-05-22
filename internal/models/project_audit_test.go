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

func newAuditTestTask(t *testing.T, id string, shortID int, taskType TaskType, title string) *Task {
	t.Helper()

	task, err := NewTask(id, shortID, taskType, title)
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
	project.EnsureSection("Sprint-1")

	epic := newAuditTestTask(t, "epic-1", 1, TaskTypeEpic, "Epic")
	story := newAuditTestTask(t, "story-1", 2, TaskTypeStory, "Story")
	bug := newAuditTestTask(t, "bug-1", 3, TaskTypeBug, "Bug")

	story.ParentID = &epic.ID
	bug.ParentID = &story.ID
	story.Section = SectionNext
	bug.SetSection("Sprint-1")
	bug.DependsOn = []string{epic.ID, "other-project:external-blocker"}
	story.RelatedTo = []string{bug.ID, "other-project:external-related"}

	project.Tasks = []*Task{epic, story, bug}
	project.BuildTaskTree()

	if err := AuditProject(project); err != nil {
		t.Fatalf("AuditProject() error = %v, want nil", err)
	}
}

func TestAuditProjectAllowsLegacyCustomSectionInSectionField(t *testing.T) {
	project := newAuditTestProject(t)
	project.EnsureSection("Backlog")

	task := newAuditTestTask(t, "task-1", 1, TaskTypeTask, "Legacy Section")
	task.Section = "Backlog"
	project.Tasks = []*Task{task}

	if err := AuditProject(project); err != nil {
		t.Fatalf("AuditProject() error = %v, want nil", err)
	}
}

func TestAuditProjectRejectsEmptyTaskID(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "", 1, TaskTypeTask, "Missing ID")
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "empty task ID")
}

func TestAuditProjectRejectsDuplicateTaskID(t *testing.T) {
	project := newAuditTestProject(t)
	first := newAuditTestTask(t, "dup", 1, TaskTypeTask, "First")
	second := newAuditTestTask(t, "dup", 2, TaskTypeBug, "Second")
	project.Tasks = []*Task{first, second}

	requireAuditErrorContains(t, project, "duplicate task ID")
}

func TestAuditProjectRejectsCyclicParentGraph(t *testing.T) {
	project := newAuditTestProject(t)
	parent := newAuditTestTask(t, "parent", 1, TaskTypeStory, "Parent")
	child := newAuditTestTask(t, "child", 2, TaskTypeTask, "Child")
	parent.ParentID = &child.ID
	child.ParentID = &parent.ID
	project.Tasks = []*Task{parent, child}

	requireAuditErrorContains(t, project, "cyclic parent chain")
}

func TestAuditProjectRejectsIllegalParentChildType(t *testing.T) {
	project := newAuditTestProject(t)
	parent := newAuditTestTask(t, "parent", 1, TaskTypeTask, "Leaf Parent")
	child := newAuditTestTask(t, "child", 2, TaskTypeStory, "Story Child")
	child.ParentID = &parent.ID
	project.Tasks = []*Task{parent, child}

	requireAuditErrorContains(t, project, "cannot be child")
}

func TestAuditProjectRejectsSelfDependency(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "task-1", 1, TaskTypeTask, "Self Dep")
	task.DependsOn = []string{task.ID}
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "depends on itself")
}

func TestAuditProjectRejectsDanglingLocalDependency(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "task-1", 1, TaskTypeTask, "Dangling Dep")
	task.DependsOn = []string{"missing-task"}
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "depends on missing local task")
}

func TestAuditProjectRejectsDependencyCycle(t *testing.T) {
	project := newAuditTestProject(t)
	taskA := newAuditTestTask(t, "a", 1, TaskTypeTask, "A")
	taskB := newAuditTestTask(t, "b", 2, TaskTypeTask, "B")
	taskC := newAuditTestTask(t, "c", 3, TaskTypeTask, "C")
	taskA.DependsOn = []string{taskB.ID}
	taskB.DependsOn = []string{taskC.ID}
	taskC.DependsOn = []string{taskA.ID}
	project.Tasks = []*Task{taskA, taskB, taskC}

	requireAuditErrorContains(t, project, "cyclic dependency")
}

func TestAuditProjectRejectsSelfRelatedRef(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "task-1", 1, TaskTypeTask, "Self Related")
	task.RelatedTo = []string{task.ID}
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "related to itself")
}

func TestAuditProjectRejectsDanglingLocalRelatedRef(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "task-1", 1, TaskTypeTask, "Dangling Related")
	task.RelatedTo = []string{"missing-task"}
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "related to missing local task")
}

func TestAuditProjectRejectsDualSectionMembership(t *testing.T) {
	project := newAuditTestProject(t)
	task := newAuditTestTask(t, "task-1", 1, TaskTypeTask, "Dual Section")
	task.Section = SectionNext
	task.CustomSection = "Sprint-1"
	project.Tasks = []*Task{task}

	requireAuditErrorContains(t, project, "multiple section buckets")
}
