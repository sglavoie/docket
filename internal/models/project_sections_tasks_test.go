package models

import (
	"testing"
)

func TestProjectGetTasksBySection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.ApplyWorkflowState(StateNext)
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.ApplyWorkflowState(StateNext)
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Complete()
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	// task4 stays in the default active view (ready).

	project.Tasks = []*Task{task1, task2, task3, task4}

	next := project.GetTasksBySection(SectionNext)
	if len(next) != 2 {
		t.Errorf("expected 2 tasks in Next, got %d", len(next))
	}

	completed := project.GetTasksBySection(SectionCompleted)
	if len(completed) != 1 {
		t.Errorf("expected 1 task in Completed, got %d", len(completed))
	}

	// Empty section represents the active view (ready/in-progress states).
	activeView := project.GetTasksBySection("")
	if len(activeView) != 1 {
		t.Errorf("expected 1 task in the active view, got %d", len(activeView))
	}

	inbox := project.GetTasksBySection(SectionInbox)
	if len(inbox) != 0 {
		t.Errorf("expected 0 tasks in Inbox, got %d", len(inbox))
	}

	nonexistent := project.GetTasksBySection("Nonexistent")
	if len(nonexistent) != 0 {
		t.Errorf("expected 0 tasks for nonexistent section, got %d", len(nonexistent))
	}
}

func TestProjectGetTasksBySectionUsesStateBuckets(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// A task still carrying the Next section but started (in_progress state)
	// belongs to the active bucket its state occupies, not to Next.
	started, err := NewTask("1", 1, TaskTypeTask, "Started")
	if err != nil {
		t.Fatal(err)
	}
	started.ApplyWorkflowState(StateInProgress)

	// A canceled task whose legacy section is empty (the ApplyWorkflowState
	// canonical representative) still buckets under Completed.
	canceled, err := NewTask("2", 2, TaskTypeTask, "Canceled")
	if err != nil {
		t.Fatal(err)
	}
	canceled.ApplyWorkflowState(StateCanceled)

	project.Tasks = []*Task{started, canceled}

	if got := len(project.GetTasksBySection(SectionNext)); got != 0 {
		t.Errorf("expected 0 tasks in Next (in-progress task belongs to active bucket), got %d", got)
	}
	if got := len(project.GetTasksBySection("")); got != 1 {
		t.Errorf("expected 1 task in the active bucket, got %d", got)
	}
	if got := len(project.GetTasksBySection(SectionCompleted)); got != 1 {
		t.Errorf("expected canceled task in Completed bucket, got %d", got)
	}
}

func TestProjectCountTasksBySection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.ApplyWorkflowState(StateSomeday)
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.ApplyWorkflowState(StateSomeday)
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Cancel()
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	// task4 stays in the default active view (ready).

	project.Tasks = []*Task{task1, task2, task3, task4}

	counts := project.CountTasksBySection()

	if counts[SectionSomeday] != 2 {
		t.Errorf("expected 2 Someday tasks, got %d", counts[SectionSomeday])
	}
	if counts[SectionCompleted] != 1 {
		t.Errorf("expected 1 Completed task (canceled buckets there), got %d", counts[SectionCompleted])
	}
	// Ready/in-progress states count under the empty active bucket.
	if counts[""] != 1 {
		t.Errorf("expected 1 active-bucket task, got %d", counts[""])
	}
}
