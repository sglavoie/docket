package models

import (
	"testing"
	"time"
)

func assertTaskIDSequence(t *testing.T, label string, tasks []*Task, want []string) {
	t.Helper()
	if len(tasks) != len(want) {
		t.Fatalf("%s length = %d, want %d", label, len(tasks), len(want))
	}
	for i, id := range want {
		if tasks[i].ID != id {
			t.Fatalf("%s[%d] = %q, want %q", label, i, tasks[i].ID, id)
		}
	}
}

func TestMoveTaskUpCompletionBoundary(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("1", 1, "Task 1") // uncompleted
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Complete() // completed

	project.Tasks = []*Task{task1, task2}

	// Try to move completed task2 up above uncompleted task1
	err = project.MoveTaskUpAt(task2, time.Now())
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

	task1, err := NewGenericTask("1", 1, "Task 1") // uncompleted
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Complete() // completed

	project.Tasks = []*Task{task1, task2}

	// Try to move uncompleted task1 down below completed task2
	err = project.MoveTaskDownAt(task1, time.Now())
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

	project.Tasks = []*Task{task1, task2}

	// Move task2 up within completed group - should work
	err = project.MoveTaskUpAt(task2, time.Now())
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

	task1, err := NewGenericTask("1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2}

	// Move task1 down within uncompleted group - should work
	err = project.MoveTaskDownAt(task1, time.Now())
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

	epic, err := NewGenericTask("epic", 1, "Epic")
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
	child2.Complete() // completed child

	epicID := epic.ID
	child1.ParentID = &epicID
	child2.ParentID = &epicID

	project.Tasks = []*Task{epic, child1, child2}
	project.BuildTaskTree()

	// Move completed child2 up - should work because it's not a root task
	err = project.MoveTaskUpAt(child2, time.Now())
	if err != nil {
		t.Errorf("MoveTaskUp returned error: %v", err)
	}

	// Verify children can be reordered regardless of completion status
	siblings := project.GetSiblings(child2)
	if siblings[0].ID != "child2" {
		t.Error("child2 should have moved up (boundary rules don't apply to non-root tasks)")
	}
	assertTaskIDSequence(t, "epic.Children after MoveTaskUp", epic.Children, []string{"child2", "child1"})
}

func TestSwapTasks(t *testing.T) {
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

	// Swap task1 and task3 (non-adjacent)
	if err := project.SwapSiblingTasksAt(task1, task3, time.Now()); err != nil {
		t.Fatalf("SwapSiblingTasksAt: %v", err)
	}

	if project.Tasks[0].ID != "3" || project.Tasks[2].ID != "1" {
		t.Errorf("expected [3, 2, 1] but got [%s, %s, %s]",
			project.Tasks[0].ID, project.Tasks[1].ID, project.Tasks[2].ID)
	}
	// task2 should be unchanged
	if project.Tasks[1].ID != "2" {
		t.Error("task2 should remain in position 1")
	}
}

func TestMoveTaskDownRebuildsParentChildrenOrder(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	epic, err := NewGenericTask("epic", 1, "Epic")
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
	child1.ParentID = &epic.ID
	child2.ParentID = &epic.ID
	project.Tasks = []*Task{epic, child1, child2}
	project.BuildTaskTree()

	err = project.MoveTaskDownAt(child1, time.Now())
	if err != nil {
		t.Errorf("MoveTaskDown returned error: %v", err)
	}

	assertTaskIDSequence(t, "project.Tasks after MoveTaskDown", project.Tasks, []string{"epic", "child2", "child1"})
	assertTaskIDSequence(t, "epic.Children after MoveTaskDown", epic.Children, []string{"child2", "child1"})
	if child1.Parent != epic || child1.Level != 1 {
		t.Fatalf("child1 hierarchy after rebuild: parent=%v level=%d, want epic level 1", child1.Parent, child1.Level)
	}
}

func TestSwapTasksRebuildsStoryChildrenOrder(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	epic, err := NewGenericTask("epic", 1, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	story, err := NewGenericTask("story", 2, "Story")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := NewGenericTask("task1", 3, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("task2", 4, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	story.ParentID = &epic.ID
	task1.ParentID = &story.ID
	task2.ParentID = &story.ID
	project.Tasks = []*Task{epic, story, task1, task2}
	project.BuildTaskTree()

	if err := project.SwapSiblingTasksAt(task1, task2, time.Now()); err != nil {
		t.Fatalf("SwapSiblingTasksAt: %v", err)
	}

	assertTaskIDSequence(t, "project.Tasks after SwapTasks", project.Tasks, []string{"epic", "story", "task2", "task1"})
	assertTaskIDSequence(t, "story.Children after SwapTasks", story.Children, []string{"task2", "task1"})
	if task2.Parent != story || task2.Level != 2 {
		t.Fatalf("task2 hierarchy after rebuild: parent=%v level=%d, want story level 2", task2.Parent, task2.Level)
	}
}

func TestSwapTasksUpdatesCanonicalTimestampsForClonedInputs(t *testing.T) {
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
	stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	task1.UpdatedAt = stale
	task2.UpdatedAt = stale
	project.Tasks = []*Task{task1, task2}

	clone1 := *task1
	clone2 := *task2
	if err := project.SwapSiblingTasksAt(&clone1, &clone2, time.Now()); err != nil {
		t.Fatalf("SwapSiblingTasksAt: %v", err)
	}

	if !task1.UpdatedAt.After(stale) {
		t.Fatal("canonical task1 UpdatedAt was not refreshed")
	}
	if !task2.UpdatedAt.After(stale) {
		t.Fatal("canonical task2 UpdatedAt was not refreshed")
	}
}

func TestSwapTasksNilTask(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*Task{task1}

	// Swap with a task not in the project — should be a no-op
	notInProject, err := NewGenericTask("ghost", 99, "Ghost")
	if err != nil {
		t.Fatal(err)
	}
	if err := project.SwapSiblingTasksAt(task1, notInProject, time.Now()); err != nil {
		t.Fatalf("SwapSiblingTasksAt: %v", err)
	}

	if project.Tasks[0].ID != "1" {
		t.Error("swap with non-existent task should be a no-op")
	}

	if err := project.SwapSiblingTasksAt(nil, task1, time.Now()); err != nil {
		t.Fatalf("SwapSiblingTasksAt: %v", err)
	}
	if err := project.SwapSiblingTasksAt(task1, nil, time.Now()); err != nil {
		t.Fatalf("SwapSiblingTasksAt: %v", err)
	}

	if project.Tasks[0].ID != "1" {
		t.Error("swap with nil task should be a no-op")
	}
}

func TestMoveTaskNilTaskNoOp(t *testing.T) {
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
	project.Tasks = []*Task{task1, task2}

	if err := project.MoveTaskUpAt(nil, time.Now()); err != nil {
		t.Errorf("MoveTaskUp(nil) returned error: %v", err)
	}
	if err := project.MoveTaskDownAt(nil, time.Now()); err != nil {
		t.Errorf("MoveTaskDown(nil) returned error: %v", err)
	}

	if project.Tasks[0].ID != "1" || project.Tasks[1].ID != "2" {
		t.Errorf("nil move should be a no-op, got [%s, %s]", project.Tasks[0].ID, project.Tasks[1].ID)
	}
}
