package models

import "testing"

func TestReorderRootTasksByCompletion(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Create tasks: some completed, some not
	task1, err := NewGenericTask("1", 1, "Task 1") // uncompleted
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Complete()                               // completed
	task3, err := NewGenericTask("3", 3, "Task 3") // uncompleted
	if err != nil {
		t.Fatal(err)
	}
	task4, err := NewGenericTask("4", 4, "Task 4")
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
	task1, err := NewGenericTask("1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Complete()
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Complete()
	task3, err := NewGenericTask("3", 3, "Task 3")
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
	epic, err := NewGenericTask("epic", 1, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	epic.Complete() // completed epic
	child1, err := NewGenericTask("child1", 2, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewGenericTask("child2", 3, "Child 2")
	if err != nil {
		t.Fatal(err)
	}
	epicID := epic.ID
	child1.ParentID = &epicID
	child2.ParentID = &epicID

	rootTask, err := NewGenericTask("root", 4, "Root Task") // uncompleted root
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

// TestReorderRootTasksByCompletionKeepsNonContiguousSubtreeInPlace pins the
// stable partition: a subtree split by an unrelated root is left split rather
// than gathered, because gathering it would move the bystander.
func TestReorderRootTasksByCompletionKeepsNonContiguousSubtreeInPlace(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	parent, err := NewGenericTask("parent", 1, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	bystander, err := NewGenericTask("bystander", 2, "Bystander")
	if err != nil {
		t.Fatal(err)
	}
	child, err := NewGenericTask("child", 3, "Child")
	if err != nil {
		t.Fatal(err)
	}
	parentID := parent.ID
	child.ParentID = &parentID

	project.Tasks = []*Task{parent, bystander, child}
	project.ReorderRootTasksByCompletion()

	want := []string{"parent", "bystander", "child"}
	for i, id := range want {
		if project.Tasks[i].ID != id {
			t.Fatalf("task order = %v, want %v", taskIDs(project.Tasks), want)
		}
	}
}

func taskIDs(tasks []*Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
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

	task1, err := NewGenericTask("1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := NewGenericTask("3", 3, "Task 3")
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
