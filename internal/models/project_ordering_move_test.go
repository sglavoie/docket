package models

import "testing"

func TestMoveTaskUpCompletionBoundary(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

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
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

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
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

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

func TestMoveChildTaskIgnoresBoundary(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

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

func TestSwapTasks(t *testing.T) {
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

	// Swap task1 and task3 (non-adjacent)
	project.SwapTasks(task1, task3)

	if project.Tasks[0].ID != "3" || project.Tasks[2].ID != "1" {
		t.Errorf("expected [3, 2, 1] but got [%s, %s, %s]",
			project.Tasks[0].ID, project.Tasks[1].ID, project.Tasks[2].ID)
	}
	// task2 should be unchanged
	if project.Tasks[1].ID != "2" {
		t.Error("task2 should remain in position 1")
	}
}

func TestSwapTasksNilTask(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*Task{task1}

	// Swap with a task not in the project — should be a no-op
	notInProject, err := NewTask("ghost", 99, TaskTypeTask, "Ghost")
	if err != nil {
		t.Fatal(err)
	}
	project.SwapTasks(task1, notInProject)

	if project.Tasks[0].ID != "1" {
		t.Error("swap with non-existent task should be a no-op")
	}
}