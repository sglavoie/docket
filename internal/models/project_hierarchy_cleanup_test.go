package models

import "testing"

func TestBuildTaskTree_OrphanedParentClearsParentID(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("task-1", 1, "Orphaned child")
	if err != nil {
		t.Fatal(err)
	}
	// Set ParentID to a non-existent task
	orphanedParent := "nonexistent-parent-id"
	task1.ParentID = &orphanedParent

	task2, err := NewGenericTask("task-2", 2, "Normal root")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2}
	project.BuildTaskTree()

	// ParentID should be cleared so the task becomes a true root
	if task1.ParentID != nil {
		t.Error("expected orphaned task ParentID to be cleared, but it is still set")
	}
	if task1.Parent != nil {
		t.Error("expected orphaned task Parent to be nil")
	}
	if !task1.IsRoot() {
		t.Error("expected orphaned task to be treated as root after BuildTaskTree")
	}
}

func TestReorderRootTasksByCompletion_NoDataLossOnOrphanedParent(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("task-1", 1, "Normal root")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("task-2", 2, "Orphaned child")
	if err != nil {
		t.Fatal(err)
	}
	// Set ParentID to a non-existent task (simulating corrupt data)
	orphanedParent := "nonexistent-parent-id"
	task2.ParentID = &orphanedParent

	project.Tasks = []*Task{task1, task2}

	// ReorderRootTasksByCompletion calls BuildTaskTree internally,
	// which should clear the orphaned ParentID before reordering.
	project.ReorderRootTasksByCompletion()

	// Both tasks must survive - no data loss
	if len(project.Tasks) != 2 {
		t.Fatalf("expected 2 tasks after reorder, got %d (data loss!)", len(project.Tasks))
	}

	// The orphaned task should now be a root
	found := false
	for _, task := range project.Tasks {
		if task.ID == task2.ID {
			found = true
			if task.ParentID != nil {
				t.Error("orphaned task should have ParentID cleared after reorder")
			}
		}
	}
	if !found {
		t.Error("orphaned task was lost during ReorderRootTasksByCompletion")
	}
}

func TestBuildTaskTree_OrphanedDependencyCleanup(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("task-1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("task-2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	// task1 depends on task2 (valid) and a nonexistent task (orphaned)
	task1.DependsOn = []string{task2.ID, "nonexistent-task-id"}

	// task1 also has a cross-project dependency (should be preserved)
	crossRef := "other-project:other-task"
	task1.DependsOn = append(task1.DependsOn, crossRef)

	project.Tasks = []*Task{task1, task2}
	project.BuildTaskTree()

	// Should have 2 dependencies: task2 (valid) and cross-project ref (preserved)
	if len(task1.DependsOn) != 2 {
		t.Fatalf("expected 2 dependencies after cleanup, got %d: %v", len(task1.DependsOn), task1.DependsOn)
	}

	// Valid same-project dep should remain
	foundValid := false
	foundCross := false
	for _, dep := range task1.DependsOn {
		if dep == task2.ID {
			foundValid = true
		}
		if dep == crossRef {
			foundCross = true
		}
	}
	if !foundValid {
		t.Error("expected valid dependency on task2 to be preserved")
	}
	if !foundCross {
		t.Error("expected cross-project dependency to be preserved")
	}
}

func TestBuildTaskTreeBreaksDependencyCycles(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	taskA, _ := NewGenericTask("a", 1, "A")
	taskB, _ := NewGenericTask("b", 2, "B")
	taskC, _ := NewGenericTask("c", 3, "C")

	// Inject a cycle directly: A->B->C->A (bypassing AddDependency self-check)
	taskA.DependsOn = []string{taskB.ID}
	taskB.DependsOn = []string{taskC.ID}
	taskC.DependsOn = []string{taskA.ID}

	project.Tasks = []*Task{taskA, taskB, taskC}
	project.BuildTaskTree()

	// After BuildTaskTree, no task should have a cycle in its DependsOn
	totalDeps := len(taskA.DependsOn) + len(taskB.DependsOn) + len(taskC.DependsOn)
	if totalDeps >= 3 {
		t.Errorf("expected cycle to be broken (total deps < 3), got %d total DependsOn entries", totalDeps)
	}
}

func TestBuildTaskTreeBreaksSelfDependencyCycle(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	taskA, _ := NewGenericTask("a", 1, "A")
	// Inject self-dependency directly
	taskA.DependsOn = []string{taskA.ID}

	project.Tasks = []*Task{taskA}
	project.BuildTaskTree()

	if len(taskA.DependsOn) != 0 {
		t.Errorf("expected self-dependency to be removed, got DependsOn: %v", taskA.DependsOn)
	}
}

// TestBuildTaskTree_PrunesMalformedCrossProjectRefs verifies that a malformed
// cross-project reference (e.g. "other-project:" with an empty task id) is
// pruned by BuildTaskTree. Previously IsCrossProject reported true for such
// values, which caused pruneOrphanedRefs to preserve them indefinitely even
// though no lookup could ever resolve them.
func TestBuildTaskTree_PrunesMalformedCrossProjectRefs(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	taskA, err := NewGenericTask("aaa", 1, "Task A")
	if err != nil {
		t.Fatal(err)
	}

	// Malformed cross-project refs have a project id but no task id.
	taskA.DependsOn = []string{"other-project:"}

	project.Tasks = []*Task{taskA}
	project.BuildTaskTree()

	if len(taskA.DependsOn) != 0 {
		t.Errorf("expected malformed cross-project DependsOn to be pruned, got %v", taskA.DependsOn)
	}
}
