package models

import "testing"

func TestReorderRootTasksByCompletion(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Create tasks: some completed, some not
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1") // uncompleted
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Complete()                                      // completed
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3") // uncompleted
	if err != nil {
		t.Fatal(err)
	}
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.Complete() // completed

	project.Tasks = []*Task{task1, task2, task3, task4}

	project.ReorderRootTasksByCompletion()

	// Verify order: uncompleted first (task1, task3), then completed (task2, task4)
	if project.Tasks[0].ID != "1" {
		t.Errorf("expected task1 at position 0, got %s", project.Tasks[0].ID)
	}
	if project.Tasks[1].ID != "3" {
		t.Errorf("expected task3 at position 1, got %s", project.Tasks[1].ID)
	}
	if project.Tasks[2].ID != "2" {
		t.Errorf("expected task2 at position 2, got %s", project.Tasks[2].ID)
	}
	if project.Tasks[3].ID != "4" {
		t.Errorf("expected task4 at position 3, got %s", project.Tasks[3].ID)
	}
}

func TestReorderRootTasksByCompletionPreservesRelativeOrder(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Create completed tasks in specific order
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Complete()
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Complete()
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Complete()

	project.Tasks = []*Task{task1, task2, task3}

	project.ReorderRootTasksByCompletion()

	// Order should remain the same (all completed)
	if project.Tasks[0].ID != "1" || project.Tasks[1].ID != "2" || project.Tasks[2].ID != "3" {
		t.Error("relative order of completed tasks should be preserved")
	}
}

func TestReorderRootTasksByCompletionWithChildren(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Create an epic with children, then a root task
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	epic.Complete() // completed epic
	child1, err := NewTask("child1", 2, TaskTypeTask, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewTask("child2", 3, TaskTypeTask, "Child 2")
	if err != nil {
		t.Fatal(err)
	}
	epicID := epic.ID
	child1.ParentID = &epicID
	child2.ParentID = &epicID

	rootTask, err := NewTask("root", 4, TaskTypeTask, "Root Task") // uncompleted root
	if err != nil {
		t.Fatal(err)
	}

	// Original order: epic, child1, child2, rootTask
	project.Tasks = []*Task{epic, child1, child2, rootTask}
	project.BuildTaskTree()

	project.ReorderRootTasksByCompletion()

	// Expected: uncompleted root first, then completed epic with its children
	// rootTask should come first, then epic subtree (epic, child1, child2)
	if project.Tasks[0].ID != "root" {
		t.Errorf("expected rootTask at position 0, got %s", project.Tasks[0].ID)
	}
	if project.Tasks[1].ID != "epic" {
		t.Errorf("expected epic at position 1, got %s", project.Tasks[1].ID)
	}
	// Children should follow their parent
	if project.Tasks[2].ID != "child1" {
		t.Errorf("expected child1 at position 2, got %s", project.Tasks[2].ID)
	}
	if project.Tasks[3].ID != "child2" {
		t.Errorf("expected child2 at position 3, got %s", project.Tasks[3].ID)
	}
}

func TestReorderRootTasksByCompletionEmptyProject(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*Task{}

	// Should not panic on empty project
	project.ReorderRootTasksByCompletion()

	if len(project.Tasks) != 0 {
		t.Error("empty project should remain empty")
	}
}

func TestReorderRootTasksByCompletionAllUncompleted(t *testing.T) {
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

	project.ReorderRootTasksByCompletion()

	// Order should remain unchanged
	if project.Tasks[0].ID != "1" || project.Tasks[1].ID != "2" || project.Tasks[2].ID != "3" {
		t.Error("order of all uncompleted tasks should be preserved")
	}
}
