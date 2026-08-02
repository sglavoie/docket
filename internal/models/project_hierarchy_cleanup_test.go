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
	orphanedParent := "nonexistent-parent-id"
	task1.ParentID = &orphanedParent

	task2, err := NewGenericTask("task-2", 2, "Normal root")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2}
	project.BuildTaskTree()

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
	orphanedParent := "nonexistent-parent-id"
	task2.ParentID = &orphanedParent

	project.Tasks = []*Task{task1, task2}
	project.ReorderRootTasksByCompletion()

	if len(project.Tasks) != 2 {
		t.Fatalf("expected 2 tasks after reorder, got %d (data loss!)", len(project.Tasks))
	}

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
