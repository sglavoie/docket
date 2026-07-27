package models

import "testing"

func TestProjectCheckAndAutoUnblock(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	blocker1 := newDependencyTestTask(t, "blocker1", 1, "Blocker 1")
	blocker2 := newDependencyTestTask(t, "blocker2", 2, "Blocker 2")
	blocked := newDependencyTestTask(t, "blocked", 3, "Blocked Task")

	_ = blocked.AddDependency(blocker1.ID)
	_ = blocked.AddDependency(blocker2.ID)
	blocked.WorkflowState = StateWaiting

	project.Tasks = []*Task{blocker1, blocker2, blocked}

	project.CheckAndAutoUnblock()
	if blocked.State() != StateWaiting {
		t.Errorf("blocked task should remain blocked, got %s", blocked.State())
	}

	blocker1.Complete()
	project.CheckAndAutoUnblock()
	if blocked.State() != StateWaiting {
		t.Errorf("blocked task should remain blocked (one dep incomplete), got %s", blocked.State())
	}

	blocker2.Complete()
	project.CheckAndAutoUnblock()
	if blocked.State() != StateInbox {
		t.Errorf("blocked task should be unblocked to inbox, got %s", blocked.State())
	}
	if !blocked.IsInState(StateInbox) {
		t.Errorf("auto-unblocked task should be in Inbox for re-triage, got %q", blocked.State())
	}
	if blocked.CompletedAt != nil {
		t.Error("auto-unblocked task should have CompletedAt cleared")
	}
}

func TestProjectCheckAndAutoUnblockWithCanceled(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	blocker := newDependencyTestTask(t, "blocker", 1, "Blocker")
	blocked := newDependencyTestTask(t, "blocked", 2, "Blocked Task")

	_ = blocked.AddDependency(blocker.ID)
	blocked.WorkflowState = StateWaiting

	project.Tasks = []*Task{blocker, blocked}

	blocker.Cancel()
	project.CheckAndAutoUnblock()
	if blocked.State() != StateInbox {
		t.Errorf("blocked task should be unblocked when blocker canceled, got %s", blocked.State())
	}
}

func TestProjectCheckAndAutoUnblockNoDependencies(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	blocked := newDependencyTestTask(t, "blocked", 1, "Blocked Task")
	blocked.WorkflowState = StateWaiting

	project.Tasks = []*Task{blocked}

	project.CheckAndAutoUnblock()
	if blocked.State() != StateWaiting {
		t.Errorf("blocked task without deps should remain blocked, got %s", blocked.State())
	}
}

func TestCheckAndAutoUnblockSkipsCrossProjectDeps(t *testing.T) {
	project := newDependencyTestProject(t, "test-proj")

	blocker := newDependencyTestTask(t, "blocker", 1, "Local Blocker")
	task := newDependencyTestTask(t, "task1", 2, "Blocked Task")

	_ = task.AddDependency(blocker.ID)
	task.DependsOn = append(task.DependsOn, "other-project-id:remote-task-id")
	task.WorkflowState = StateWaiting

	project.Tasks = []*Task{blocker, task}

	blocker.Complete()

	project.CheckAndAutoUnblock()
	if task.State() != StateWaiting {
		t.Errorf("expected task to remain blocked due to cross-project dep, got %s", task.State())
	}
}

func TestCheckAndAutoUnblockCrossProjectDepOnly(t *testing.T) {
	project := newDependencyTestProject(t, "test-proj")

	// Task blocked solely by a cross-project dependency (no local deps).
	// Must remain blocked because we cannot resolve cross-project refs locally.
	task := newDependencyTestTask(t, "task1", 1, "Blocked Task")
	task.DependsOn = []string{"other-project:remote-task"}
	task.WorkflowState = StateWaiting

	project.Tasks = []*Task{task}

	project.CheckAndAutoUnblock()
	if task.State() != StateWaiting {
		t.Errorf("expected task to remain blocked with only cross-project dep, got %s", task.State())
	}
}
