package models

import (
	"fmt"
	"testing"
)

func TestProjectGetBlockingTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}

	// Task 3 depends on Task 1 and Task 2
	_ = task3.AddDependency(task1.ID)
	_ = task3.AddDependency(task2.ID)

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
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}

	// Task 2 and Task 3 depend on Task 1
	_ = task2.AddDependency(task1.ID)
	_ = task3.AddDependency(task1.ID)

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
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2, task3}

	// No dependencies yet - no circular dependency
	if project.HasCircularDependency(task1.ID, task2.ID) {
		t.Error("should not detect circular dependency when no deps exist")
	}

	// Task 1 depends on Task 2
	_ = task1.AddDependency(task2.ID)

	// Adding Task 2 -> Task 1 would create cycle
	if !project.HasCircularDependency(task2.ID, task1.ID) {
		t.Error("should detect circular dependency: task2 -> task1 -> task2")
	}

	// Adding Task 2 -> Task 3 would not create cycle
	if project.HasCircularDependency(task2.ID, task3.ID) {
		t.Error("should not detect circular dependency for task2 -> task3")
	}

	// Test transitive: Task 1 -> Task 2, Task 2 -> Task 3, adding Task 3 -> Task 1 creates cycle
	_ = task2.AddDependency(task3.ID)
	if !project.HasCircularDependency(task3.ID, task1.ID) {
		t.Error("should detect transitive circular dependency: task3 -> task1 -> task2 -> task3")
	}
}

func TestProjectCheckAndAutoUnblock(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	blocker1, err := NewTask("blocker1", 1, TaskTypeTask, "Blocker 1")
	if err != nil {
		t.Fatal(err)
	}
	blocker2, err := NewTask("blocker2", 2, TaskTypeTask, "Blocker 2")
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := NewTask("blocked", 3, TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}

	// Blocked task depends on both blockers
	_ = blocked.AddDependency(blocker1.ID)
	_ = blocked.AddDependency(blocker2.ID)
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

	// Complete blocker2 - now blocked should auto-unblock to todo via Reopen()
	blocker2.Complete()
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusTodo {
		t.Errorf("blocked task should be unblocked to todo, got %s", blocked.Status)
	}
	// Verify Reopen() lifecycle side-effects fired
	if blocked.Section != SectionInbox {
		t.Errorf("auto-unblocked task should be in Inbox for re-triage, got %q", blocked.Section)
	}
	if blocked.CompletedAt != nil {
		t.Error("auto-unblocked task should have CompletedAt cleared")
	}
}

func TestProjectCheckAndAutoUnblockWithCanceled(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	blocker, err := NewTask("blocker", 1, TaskTypeTask, "Blocker")
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := NewTask("blocked", 2, TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}

	_ = blocked.AddDependency(blocker.ID)
	blocked.Status = StatusBlocked

	project.Tasks = []*Task{blocker, blocked}

	// Cancel blocker (considered complete)
	blocker.Cancel()
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusTodo {
		t.Errorf("blocked task should be unblocked when blocker canceled, got %s", blocked.Status)
	}
}

func TestProjectCheckAndAutoUnblockNoDependencies(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Blocked task with no dependencies should not be auto-unblocked
	blocked, err := NewTask("blocked", 1, TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}
	blocked.Status = StatusBlocked

	project.Tasks = []*Task{blocked}

	project.CheckAndAutoUnblock()
	if blocked.Status != StatusBlocked {
		t.Errorf("blocked task without deps should remain blocked, got %s", blocked.Status)
	}
}

func TestHasIncompleteDependencies(t *testing.T) {
	project, err := NewProject("test-project", "Test Project", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	blocker1, err := NewTask("blocker1", 1, TaskTypeTask, "Blocker 1")
	if err != nil {
		t.Fatal(err)
	}
	blocker2, err := NewTask("blocker2", 2, TaskTypeTask, "Blocker 2")
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := NewTask("dependent", 3, TaskTypeTask, "Dependent Task")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{blocker1, blocker2, dependent}

	// No dependencies - should return false
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with no dependencies should return false")
	}

	// Add dependencies - both incomplete
	_ = dependent.AddDependency(blocker1.ID)
	_ = dependent.AddDependency(blocker2.ID)

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

	// Test with canceled dependency
	blocker2.Reopen()
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with reopened dependency should return true")
	}
	blocker2.Cancel()
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with canceled dependency should return false (canceled is complete)")
	}
}

func TestGetIncompleteDependencies(t *testing.T) {
	project, err := NewProject("test-project", "Test Project", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	blocker1, err := NewTask("blocker1", 1, TaskTypeTask, "Blocker 1")
	if err != nil {
		t.Fatal(err)
	}
	blocker2, err := NewTask("blocker2", 2, TaskTypeTask, "Blocker 2")
	if err != nil {
		t.Fatal(err)
	}
	blocker3, err := NewTask("blocker3", 3, TaskTypeTask, "Blocker 3")
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := NewTask("dependent", 4, TaskTypeTask, "Dependent Task")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{blocker1, blocker2, blocker3, dependent}

	// No dependencies
	incomplete := project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 0 {
		t.Errorf("expected 0 incomplete deps, got %d", len(incomplete))
	}

	// Add three dependencies
	_ = dependent.AddDependency(blocker1.ID)
	_ = dependent.AddDependency(blocker2.ID)
	_ = dependent.AddDependency(blocker3.ID)

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

func TestProjectHasCircularDependencyDeepChain(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	tasks := make([]*Task, 5)
	for i := range 5 {
		var err error
		tasks[i], err = NewTask(fmt.Sprintf("%d", i), i+1, TaskTypeTask, fmt.Sprintf("Task %d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	project.Tasks = tasks

	// Chain: 0 -> 1 -> 2 -> 3 -> 4 (task 0 depends on 1, which depends on 2, etc.)
	_ = tasks[0].AddDependency(tasks[1].ID)
	_ = tasks[1].AddDependency(tasks[2].ID)
	_ = tasks[2].AddDependency(tasks[3].ID)
	_ = tasks[3].AddDependency(tasks[4].ID)

	// Adding 4 -> 0 would create 5-node cycle: 0 -> 1 -> 2 -> 3 -> 4 -> 0
	if !project.HasCircularDependency(tasks[4].ID, tasks[0].ID) {
		t.Error("should detect 5-node circular dependency")
	}

	// Adding 4 -> 2 would create 3-node cycle: 2 -> 3 -> 4 -> 2
	if !project.HasCircularDependency(tasks[4].ID, tasks[2].ID) {
		t.Error("should detect cycle 4 -> 2 -> 3 -> 4")
	}

	// Adding 4 -> 3 would create 2-node cycle: 3 -> 4 -> 3
	if !project.HasCircularDependency(tasks[4].ID, tasks[3].ID) {
		t.Error("should detect cycle 4 -> 3 -> 4")
	}

	// Adding 1 -> 4 would NOT create a cycle (4 doesn't depend on anything that leads to 1)
	if project.HasCircularDependency(tasks[1].ID, tasks[4].ID) {
		t.Error("should not detect cycle for 1 -> 4")
	}
}

func TestProjectHasCircularDependencyBranching(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Create branching dependency graph:
	//     0 -> 1 -> 3
	//     0 -> 2 -> 3
	tasks := make([]*Task, 4)
	for i := range 4 {
		var err error
		tasks[i], err = NewTask(fmt.Sprintf("%d", i), i+1, TaskTypeTask, fmt.Sprintf("Task %d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	project.Tasks = tasks

	_ = tasks[0].AddDependency(tasks[1].ID)
	_ = tasks[0].AddDependency(tasks[2].ID)
	_ = tasks[1].AddDependency(tasks[3].ID)
	_ = tasks[2].AddDependency(tasks[3].ID)

	// Adding 3 -> 0 would create cycle through either branch
	if !project.HasCircularDependency(tasks[3].ID, tasks[0].ID) {
		t.Error("should detect cycle in branching graph")
	}

	// Adding 3 -> 1 would create cycle: 1 -> 3 -> 1
	if !project.HasCircularDependency(tasks[3].ID, tasks[1].ID) {
		t.Error("should detect cycle 3 -> 1 -> 3")
	}

	// Adding 1 -> 2 would NOT create a cycle
	if project.HasCircularDependency(tasks[1].ID, tasks[2].ID) {
		t.Error("should not detect cycle for 1 -> 2")
	}
}

func TestProjectHasCircularDependencyWithNonExistentTask(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	// Task 1 depends on Task 2, Task 2 depends on non-existent task
	_ = task1.AddDependency(task2.ID)
	task2.DependsOn = append(task2.DependsOn, "non-existent-id")

	project.Tasks = []*Task{task1, task2}

	// task2 -> task1 -> task2 is a real cycle, should still be detected
	// even though task2 also has a non-existent dependency
	if !project.HasCircularDependency(task2.ID, task1.ID) {
		t.Error("should detect cycle task2 -> task1 -> task2 despite non-existent dep in chain")
	}

	// HasCircularDependency("non-existent-id", task1.ID) checks if adding
	// non-existent-id -> task1 would create a cycle. It calls
	// dependsOnTransitive(task1.ID, "non-existent-id", ...) which traverses
	// task1 -> task2 -> "non-existent-id" and finds sourceID == targetID, so
	// it correctly detects a potential cycle (the non-existent ID is in the chain).
	// This should not panic.
	_ = project.HasCircularDependency("non-existent-id", task1.ID)

	// Adding a dep from a real task to non-existent should not detect a cycle
	if project.HasCircularDependency(task1.ID, "non-existent-id") {
		t.Error("should not detect cycle when target dependency does not exist")
	}
}

func TestHasIncompleteDependenciesWithNonExistentDependency(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	blocker, err := NewTask("blocker", 1, TaskTypeTask, "Blocker")
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := NewTask("dependent", 2, TaskTypeTask, "Dependent Task")
	if err != nil {
		t.Fatal(err)
	}

	// Dependent depends on a real blocker and a non-existent task
	_ = dependent.AddDependency(blocker.ID)
	dependent.DependsOn = append(dependent.DependsOn, "non-existent-id")

	project.Tasks = []*Task{blocker, dependent}

	// Non-existent dep should be skipped; only the real incomplete blocker counts
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with incomplete real dependency should return true")
	}

	// Complete the real blocker - non-existent dep should not count as incomplete
	blocker.Complete()
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task should not have incomplete deps when real blocker is done and other dep is non-existent")
	}
}

func TestGetIncompleteDependenciesWithNonExistentDependency(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	blocker, err := NewTask("blocker", 1, TaskTypeTask, "Blocker")
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := NewTask("dependent", 2, TaskTypeTask, "Dependent Task")
	if err != nil {
		t.Fatal(err)
	}

	// Dependent depends on a real blocker and a non-existent task
	_ = dependent.AddDependency(blocker.ID)
	dependent.DependsOn = append(dependent.DependsOn, "non-existent-id")

	project.Tasks = []*Task{blocker, dependent}

	// Should return only the real incomplete dependency, skipping non-existent
	incomplete := project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 1 {
		t.Errorf("expected 1 incomplete dep (non-existent skipped), got %d", len(incomplete))
	}
	if len(incomplete) > 0 && incomplete[0].ID != blocker.ID {
		t.Errorf("expected incomplete dep to be blocker, got %s", incomplete[0].ID)
	}

	// Complete the blocker - should return empty (non-existent still skipped)
	blocker.Complete()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 0 {
		t.Errorf("expected 0 incomplete deps after completing blocker, got %d", len(incomplete))
	}
}

func TestHasIncompleteDependenciesCrossProjectRef(t *testing.T) {
	project, err := NewProject("test-proj", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}

	// Add a cross-project qualified reference directly (bypasses AddDependency self-check)
	task.DependsOn = []string{"other-project-id:remote-task-id"}
	project.Tasks = []*Task{task}

	// Cross-project deps cannot be resolved locally and should be treated as incomplete
	if !project.HasIncompleteDependencies(task) {
		t.Error("expected HasIncompleteDependencies to return true for cross-project dep")
	}
}

func TestCheckAndAutoUnblockSkipsCrossProjectDeps(t *testing.T) {
	project, err := NewProject("test-proj", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	blocker, err := NewTask("blocker", 1, TaskTypeTask, "Local Blocker")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("task1", 2, TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}

	// Task depends on a local blocker and a cross-project dep
	_ = task.AddDependency(blocker.ID)
	task.DependsOn = append(task.DependsOn, "other-project-id:remote-task-id")
	task.Status = StatusBlocked

	project.Tasks = []*Task{blocker, task}

	// Complete local blocker
	blocker.Complete()

	// Auto-unblock should NOT unblock because cross-project dep is unresolvable
	project.CheckAndAutoUnblock()
	if task.Status != StatusBlocked {
		t.Errorf("expected task to remain blocked due to cross-project dep, got %s", task.Status)
	}
}
