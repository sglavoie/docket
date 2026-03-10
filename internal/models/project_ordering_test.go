package models

import "testing"

func TestReorderRootTasksByCompletion(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

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
	project := NewProject("test", "Test", "/tmp/test")

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
	project := NewProject("test", "Test", "/tmp/test")

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
	project := NewProject("test", "Test", "/tmp/test")
	project.Tasks = []*Task{}

	// Should not panic on empty project
	project.ReorderRootTasksByCompletion()

	if len(project.Tasks) != 0 {
		t.Error("empty project should remain empty")
	}
}

func TestReorderRootTasksByCompletionAllUncompleted(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

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

func TestMoveTaskUpCompletionBoundary(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1") // uncompleted
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Complete() // completed

	project.Tasks = []*Task{task1, task2}

	// Try to move completed task2 up above uncompleted task1
	err = project.MoveTaskUp(task2)
	if err != nil {
		t.Errorf("MoveTaskUp returned error: %v", err)
	}

	// task2 should NOT have moved (boundary protection)
	if project.Tasks[0].ID != "1" {
		t.Error("completed task should not move above uncompleted task")
	}
	if project.Tasks[1].ID != "2" {
		t.Error("completed task should remain in place")
	}
}

func TestMoveTaskDownCompletionBoundary(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1") // uncompleted
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Complete() // completed

	project.Tasks = []*Task{task1, task2}

	// Try to move uncompleted task1 down below completed task2
	err = project.MoveTaskDown(task1)
	if err != nil {
		t.Errorf("MoveTaskDown returned error: %v", err)
	}

	// task1 should NOT have moved (boundary protection)
	if project.Tasks[0].ID != "1" {
		t.Error("uncompleted task should remain in place")
	}
	if project.Tasks[1].ID != "2" {
		t.Error("completed task should not move above uncompleted task")
	}
}

func TestMoveTaskUpWithinCompletedGroup(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

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

	project.Tasks = []*Task{task1, task2}

	// Move task2 up within completed group - should work
	err = project.MoveTaskUp(task2)
	if err != nil {
		t.Errorf("MoveTaskUp returned error: %v", err)
	}

	if project.Tasks[0].ID != "2" {
		t.Error("task2 should have moved up within completed group")
	}
	if project.Tasks[1].ID != "1" {
		t.Error("task1 should have moved down within completed group")
	}
}

func TestMoveTaskDownWithinUncompletedGroup(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2}

	// Move task1 down within uncompleted group - should work
	err = project.MoveTaskDown(task1)
	if err != nil {
		t.Errorf("MoveTaskDown returned error: %v", err)
	}

	if project.Tasks[0].ID != "2" {
		t.Error("task2 should have moved up")
	}
	if project.Tasks[1].ID != "1" {
		t.Error("task1 should have moved down within uncompleted group")
	}
}

func TestRelocateTaskToEndOfSection(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	// Task at index 0 (before Next tasks) gets moved to Next section
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = SectionNext // just moved to Next
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionNext
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = SectionInbox

	project.Tasks = []*Task{task1, task2, task3}

	// task1 is before task2 in the slice but both are in Next
	// Relocate task1 to end of Next section
	project.RelocateTaskToEndOfSection("1", SectionNext)

	// task1 should now be after task2
	if project.Tasks[0].ID != "2" {
		t.Errorf("expected task2 at position 0, got %s", project.Tasks[0].ID)
	}
	if project.Tasks[1].ID != "1" {
		t.Errorf("expected task1 at position 1, got %s", project.Tasks[1].ID)
	}
	if project.Tasks[2].ID != "3" {
		t.Errorf("expected task3 at position 2, got %s", project.Tasks[2].ID)
	}
}

func TestRelocateTaskToEndOfSectionAlreadyAtEnd(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = SectionNext
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionNext

	project.Tasks = []*Task{task1, task2}

	// task2 is already the last Next task — no-op
	project.RelocateTaskToEndOfSection("2", SectionNext)

	if project.Tasks[0].ID != "1" || project.Tasks[1].ID != "2" {
		t.Error("order should not change when task is already at end of section")
	}
}

func TestRelocateTaskToEndOfSectionNoOtherTasks(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = SectionNext
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionInbox

	project.Tasks = []*Task{task1, task2}

	// task1 is the only Next task — no-op
	project.RelocateTaskToEndOfSection("1", SectionNext)

	if project.Tasks[0].ID != "1" || project.Tasks[1].ID != "2" {
		t.Error("order should not change when task is only one in section")
	}
}

func TestRelocateTaskToEndOfSectionInterleaved(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	// Interleaved sections: moved task is before several Next tasks
	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = SectionNext // just moved here
	task2, err := NewTask("2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = SectionInbox
	task3, err := NewTask("3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Section = SectionNext
	task4, err := NewTask("4", 4, TaskTypeTask, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.Section = SectionInbox
	task5, err := NewTask("5", 5, TaskTypeTask, "Task 5")
	if err != nil {
		t.Fatal(err)
	}
	task5.Section = SectionNext

	project.Tasks = []*Task{task1, task2, task3, task4, task5}

	project.RelocateTaskToEndOfSection("1", SectionNext)

	// task1 should end up after task5 (last Next task at index 4)
	ids := make([]string, len(project.Tasks))
	for i, t := range project.Tasks {
		ids[i] = t.ID
	}
	// Expected: [2, 3, 4, 5, 1]
	expected := []string{"2", "3", "4", "5", "1"}
	for i, id := range expected {
		if ids[i] != id {
			t.Errorf("position %d: expected %s, got %s (full: %v)", i, id, ids[i], ids)
			break
		}
	}
}

func TestMoveChildTaskIgnoresBoundary(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	child1, err := NewTask("child1", 2, TaskTypeTask, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewTask("child2", 3, TaskTypeTask, "Child 2")
	if err != nil {
		t.Fatal(err)
	}
	child2.Complete() // completed child

	epicID := epic.ID
	child1.ParentID = &epicID
	child2.ParentID = &epicID

	project.Tasks = []*Task{epic, child1, child2}
	project.BuildTaskTree()

	// Move completed child2 up - should work because it's not a root task
	err = project.MoveTaskUp(child2)
	if err != nil {
		t.Errorf("MoveTaskUp returned error: %v", err)
	}

	// Verify children can be reordered regardless of completion status
	siblings := project.GetSiblings(child2)
	if siblings[0].ID != "child2" {
		t.Error("child2 should have moved up (boundary rules don't apply to non-root tasks)")
	}
}
