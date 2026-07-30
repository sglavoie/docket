package models

import "testing"

func TestHasIncompleteDependencies(t *testing.T) {
	project := newDependencyTestProject(t, "test-project")

	blocker1 := newDependencyTestTask(t, "blocker1", 1, "Blocker 1")
	blocker2 := newDependencyTestTask(t, "blocker2", 2, "Blocker 2")
	dependent := newDependencyTestTask(t, "dependent", 3, "Dependent Task")

	project.Tasks = []*Task{blocker1, blocker2, dependent}

	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with no dependencies should return false")
	}

	_ = dependent.AddDependency(blocker1.ID)
	_ = dependent.AddDependency(blocker2.ID)

	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with incomplete dependencies should return true")
	}

	blocker1.Complete()
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with one incomplete dependency should return true")
	}

	blocker2.Complete()
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with all dependencies complete should return false")
	}

	if err := blocker2.ApplyStateTransition(StateInbox); err != nil {
		t.Fatal(err)
	}
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with dependency returned to Inbox should return true")
	}
	blocker2.Cancel()
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with canceled dependency should return false (canceled is complete)")
	}
}

func TestGetIncompleteDependencies(t *testing.T) {
	project := newDependencyTestProject(t, "test-project")

	blocker1 := newDependencyTestTask(t, "blocker1", 1, "Blocker 1")
	blocker2 := newDependencyTestTask(t, "blocker2", 2, "Blocker 2")
	blocker3 := newDependencyTestTask(t, "blocker3", 3, "Blocker 3")
	dependent := newDependencyTestTask(t, "dependent", 4, "Dependent Task")

	project.Tasks = []*Task{blocker1, blocker2, blocker3, dependent}

	incomplete := project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 0 {
		t.Errorf("expected 0 incomplete deps, got %d", len(incomplete))
	}

	_ = dependent.AddDependency(blocker1.ID)
	_ = dependent.AddDependency(blocker2.ID)
	_ = dependent.AddDependency(blocker3.ID)

	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 3 {
		t.Errorf("expected 3 incomplete deps, got %d", len(incomplete))
	}

	blocker1.Complete()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 2 {
		t.Errorf("expected 2 incomplete deps, got %d", len(incomplete))
	}

	blocker2.Cancel()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 1 {
		t.Errorf("expected 1 incomplete dep, got %d", len(incomplete))
	}

	if incomplete[0].ID != blocker3.ID {
		t.Errorf("expected remaining incomplete to be blocker3, got %s", incomplete[0].ID)
	}

	blocker3.Complete()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 0 {
		t.Errorf("expected 0 incomplete deps, got %d", len(incomplete))
	}
}

func TestHasIncompleteDependenciesWithNonExistentDependency(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	blocker := newDependencyTestTask(t, "blocker", 1, "Blocker")
	dependent := newDependencyTestTask(t, "dependent", 2, "Dependent Task")

	_ = dependent.AddDependency(blocker.ID)
	dependent.DependsOn = append(dependent.DependsOn, "non-existent-id")

	project.Tasks = []*Task{blocker, dependent}

	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with incomplete real dependency should return true")
	}

	blocker.Complete()
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task should not have incomplete deps when real blocker is done and other dep is non-existent")
	}
}

func TestGetIncompleteDependenciesWithNonExistentDependency(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	blocker := newDependencyTestTask(t, "blocker", 1, "Blocker")
	dependent := newDependencyTestTask(t, "dependent", 2, "Dependent Task")

	_ = dependent.AddDependency(blocker.ID)
	dependent.DependsOn = append(dependent.DependsOn, "non-existent-id")

	project.Tasks = []*Task{blocker, dependent}

	incomplete := project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 1 {
		t.Errorf("expected 1 incomplete dep (non-existent skipped), got %d", len(incomplete))
	}
	if len(incomplete) > 0 && incomplete[0].ID != blocker.ID {
		t.Errorf("expected incomplete dep to be blocker, got %s", incomplete[0].ID)
	}

	blocker.Complete()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 0 {
		t.Errorf("expected 0 incomplete deps after completing blocker, got %d", len(incomplete))
	}
}

func TestHasIncompleteDependenciesCrossProjectRef(t *testing.T) {
	project := newDependencyTestProject(t, "test-proj")

	task := newDependencyTestTask(t, "task1", 1, "Task 1")
	task.DependsOn = []string{"other-project-id:remote-task-id"}
	project.Tasks = []*Task{task}

	if !project.HasIncompleteDependencies(task) {
		t.Error("expected HasIncompleteDependencies to return true for cross-project dep")
	}
}
