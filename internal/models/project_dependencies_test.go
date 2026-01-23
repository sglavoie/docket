package models

import "testing"

func TestProjectGetBlockingTasks(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	// Task 3 depends on Task 1 and Task 2
	task3.AddDependency(task1.ID)
	task3.AddDependency(task2.ID)

	project.Tasks = []*Task{task1, task2, task3}

	blockers := project.GetBlockingTasks(task3)
	if len(blockers) != 2 {
		t.Errorf("expected 2 blocking tasks, got %d", len(blockers))
	}

	// Task 1 has no dependencies
	blockers = project.GetBlockingTasks(task1)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blocking tasks for task1, got %d", len(blockers))
	}
}

func TestProjectGetBlockedByTasks(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	// Task 2 and Task 3 depend on Task 1
	task2.AddDependency(task1.ID)
	task3.AddDependency(task1.ID)

	project.Tasks = []*Task{task1, task2, task3}

	blocked := project.GetBlockedByTasks(task1)
	if len(blocked) != 2 {
		t.Errorf("expected task1 to block 2 tasks, got %d", len(blocked))
	}

	// Task 3 blocks no one
	blocked = project.GetBlockedByTasks(task3)
	if len(blocked) != 0 {
		t.Errorf("expected task3 to block 0 tasks, got %d", len(blocked))
	}
}

func TestProjectHasCircularDependency(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	project.Tasks = []*Task{task1, task2, task3}

	// No dependencies yet - no circular dependency
	if project.HasCircularDependency(task1.ID, task2.ID) {
		t.Error("should not detect circular dependency when no deps exist")
	}

	// Task 1 depends on Task 2
	task1.AddDependency(task2.ID)

	// Adding Task 2 -> Task 1 would create cycle
	if !project.HasCircularDependency(task2.ID, task1.ID) {
		t.Error("should detect circular dependency: task2 -> task1 -> task2")
	}

	// Adding Task 2 -> Task 3 would not create cycle
	if project.HasCircularDependency(task2.ID, task3.ID) {
		t.Error("should not detect circular dependency for task2 -> task3")
	}

	// Test transitive: Task 1 -> Task 2, Task 2 -> Task 3, adding Task 3 -> Task 1 creates cycle
	task2.AddDependency(task3.ID)
	if !project.HasCircularDependency(task3.ID, task1.ID) {
		t.Error("should detect transitive circular dependency: task3 -> task1 -> task2 -> task3")
	}
}

func TestProjectCheckAndAutoUnblock(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	blocker1 := NewTask("blocker1", 1, TaskTypeTask, "Blocker 1")
	blocker2 := NewTask("blocker2", 2, TaskTypeTask, "Blocker 2")
	blocked := NewTask("blocked", 3, TaskTypeTask, "Blocked Task")

	// Blocked task depends on both blockers
	blocked.AddDependency(blocker1.ID)
	blocked.AddDependency(blocker2.ID)
	blocked.Status = StatusBlocked

	project.Tasks = []*Task{blocker1, blocker2, blocked}

	// Neither blocker is done yet - blocked should stay blocked
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusBlocked {
		t.Errorf("blocked task should remain blocked, got %s", blocked.Status)
	}

	// Complete blocker1 - still blocked (blocker2 not done)
	blocker1.Complete()
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusBlocked {
		t.Errorf("blocked task should remain blocked (one dep incomplete), got %s", blocked.Status)
	}

	// Complete blocker2 - now blocked should auto-unblock to todo
	blocker2.Complete()
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusTodo {
		t.Errorf("blocked task should be unblocked to todo, got %s", blocked.Status)
	}
}

func TestProjectCheckAndAutoUnblockWithCancelled(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	blocker := NewTask("blocker", 1, TaskTypeTask, "Blocker")
	blocked := NewTask("blocked", 2, TaskTypeTask, "Blocked Task")

	blocked.AddDependency(blocker.ID)
	blocked.Status = StatusBlocked

	project.Tasks = []*Task{blocker, blocked}

	// Cancel blocker (considered complete)
	blocker.Cancel()
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusTodo {
		t.Errorf("blocked task should be unblocked when blocker cancelled, got %s", blocked.Status)
	}
}

func TestProjectCheckAndAutoUnblockNoDependencies(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	// Blocked task with no dependencies should not be auto-unblocked
	blocked := NewTask("blocked", 1, TaskTypeTask, "Blocked Task")
	blocked.Status = StatusBlocked

	project.Tasks = []*Task{blocked}

	project.CheckAndAutoUnblock()
	if blocked.Status != StatusBlocked {
		t.Errorf("blocked task without deps should remain blocked, got %s", blocked.Status)
	}
}

func TestHasIncompleteDependencies(t *testing.T) {
	project := NewProject("test-project", "Test Project", "/tmp/test")

	blocker1 := NewTask("blocker1", 1, TaskTypeTask, "Blocker 1")
	blocker2 := NewTask("blocker2", 2, TaskTypeTask, "Blocker 2")
	dependent := NewTask("dependent", 3, TaskTypeTask, "Dependent Task")

	project.Tasks = []*Task{blocker1, blocker2, dependent}

	// No dependencies - should return false
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with no dependencies should return false")
	}

	// Add dependencies - both incomplete
	dependent.AddDependency(blocker1.ID)
	dependent.AddDependency(blocker2.ID)

	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with incomplete dependencies should return true")
	}

	// Complete one blocker - still has incomplete deps
	blocker1.Complete()
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with one incomplete dependency should return true")
	}

	// Complete second blocker - no incomplete deps
	blocker2.Complete()
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with all dependencies complete should return false")
	}

	// Test with cancelled dependency
	blocker2.Reopen()
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with reopened dependency should return true")
	}
	blocker2.Cancel()
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with cancelled dependency should return false (cancelled is complete)")
	}
}

func TestGetIncompleteDependencies(t *testing.T) {
	project := NewProject("test-project", "Test Project", "/tmp/test")

	blocker1 := NewTask("blocker1", 1, TaskTypeTask, "Blocker 1")
	blocker2 := NewTask("blocker2", 2, TaskTypeTask, "Blocker 2")
	blocker3 := NewTask("blocker3", 3, TaskTypeTask, "Blocker 3")
	dependent := NewTask("dependent", 4, TaskTypeTask, "Dependent Task")

	project.Tasks = []*Task{blocker1, blocker2, blocker3, dependent}

	// No dependencies
	incomplete := project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 0 {
		t.Errorf("expected 0 incomplete deps, got %d", len(incomplete))
	}

	// Add three dependencies
	dependent.AddDependency(blocker1.ID)
	dependent.AddDependency(blocker2.ID)
	dependent.AddDependency(blocker3.ID)

	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 3 {
		t.Errorf("expected 3 incomplete deps, got %d", len(incomplete))
	}

	// Complete one
	blocker1.Complete()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 2 {
		t.Errorf("expected 2 incomplete deps, got %d", len(incomplete))
	}

	// Cancel another
	blocker2.Cancel()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 1 {
		t.Errorf("expected 1 incomplete dep, got %d", len(incomplete))
	}

	// Verify the remaining incomplete is blocker3
	if incomplete[0].ID != blocker3.ID {
		t.Errorf("expected remaining incomplete to be blocker3, got %s", incomplete[0].ID)
	}

	// Complete the last one
	blocker3.Complete()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 0 {
		t.Errorf("expected 0 incomplete deps, got %d", len(incomplete))
	}
}
