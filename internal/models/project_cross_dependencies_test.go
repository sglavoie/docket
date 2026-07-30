package models

import "testing"

func TestMoveTaskToProjectClearsCrossProjectDependencies(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	parent, err := NewGenericTask("parent", 1, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	child1, err := NewGenericTask("child1", 2, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewGenericTask("child2", 3, "Child 2")
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewGenericTask("other", 4, "Other Task")
	if err != nil {
		t.Fatal(err)
	}

	parentID := parent.ID
	child1.ParentID = &parentID
	child2.ParentID = &parentID

	_ = child1.AddDependency(child2.ID)
	_ = child1.AddDependency(other.ID)

	sourceProj.Tasks = []*Task{parent, child1, child2, other}
	sourceProj.BuildTaskTree()

	result, err := sourceProj.MoveTaskToProject(parent, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	if len(child1.DependsOn) != 1 || child1.DependsOn[0] != child2.ID {
		t.Errorf("expected child1 to only depend on child2 (both moved), got %v", child1.DependsOn)
	}

	if len(result.ClearedDeps) != 1 || result.ClearedDeps[0] != other.ID {
		t.Errorf("expected 1 cleared dependency (other), got %v", result.ClearedDeps)
	}
}

func TestMoveTaskToProjectClearsDependenciesInSource(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("task1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("task2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := NewGenericTask("task3", 3, "Task 3")
	if err != nil {
		t.Fatal(err)
	}

	_ = task3.AddDependency(task1.ID)
	_ = task3.AddDependency(task2.ID)

	sourceProj.Tasks = []*Task{task1, task2, task3}
	sourceProj.BuildTaskTree()

	_, err = sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	if len(task3.DependsOn) != 1 {
		t.Errorf("expected 1 dependency remaining in task3, got %d", len(task3.DependsOn))
	}
	if task3.DependsOn[0] != task2.ID {
		t.Errorf("expected task3 to still depend on task2, got %s", task3.DependsOn[0])
	}
}

func TestMoveTaskToProjectPreservesCrossProjectRefs(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("task1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("task2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewGenericTask("other", 3, "Other Task")
	if err != nil {
		t.Fatal(err)
	}

	crossRef := "external-proj-id:remote-task-id"
	_ = task1.AddDependency(crossRef)
	_ = task1.AddDependency(other.ID)

	sourceProj.Tasks = []*Task{task1, task2, other}
	sourceProj.BuildTaskTree()

	result, err := sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	if len(task1.DependsOn) != 1 {
		t.Fatalf("expected 1 dependency preserved (cross-project ref), got %d: %v", len(task1.DependsOn), task1.DependsOn)
	}
	if task1.DependsOn[0] != crossRef {
		t.Errorf("expected cross-project ref %q to be preserved, got %q", crossRef, task1.DependsOn[0])
	}

	if len(result.ClearedDeps) != 1 || result.ClearedDeps[0] != other.ID {
		t.Errorf("expected 1 cleared dep (other), got %v", result.ClearedDeps)
	}
}
