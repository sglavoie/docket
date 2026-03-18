package models

import (
	"fmt"
	"testing"
)

func TestProjectGetSiblings(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	// Create tasks: two root tasks, one with children
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

	epic, err := NewTask("epic", 10, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	child1, err := NewTask("child1", 11, TaskTypeTask, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewTask("child2", 12, TaskTypeTask, "Child 2")
	if err != nil {
		t.Fatal(err)
	}

	epicID := epic.ID
	child1.ParentID = &epicID
	child2.ParentID = &epicID

	project.Tasks = []*Task{task1, task2, epic, child1, task3, child2}

	// Root tasks have 4 siblings (task1, task2, task3, epic)
	rootSiblings := project.GetSiblings(task1)
	if len(rootSiblings) != 4 {
		t.Errorf("expected 4 root siblings, got %d", len(rootSiblings))
	}

	// Children have 2 siblings
	childSiblings := project.GetSiblings(child1)
	if len(childSiblings) != 2 {
		t.Errorf("expected 2 child siblings, got %d", len(childSiblings))
	}
}

func TestProjectMoveTaskUp(t *testing.T) {
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

	// Move task2 up (should swap with task1)
	err = project.MoveTaskUp(task2)
	if err != nil {
		t.Errorf("MoveTaskUp returned error: %v", err)
	}

	// Check order: should be task2, task1, task3
	if project.Tasks[0].ID != "2" {
		t.Errorf("expected task2 at position 0, got %s", project.Tasks[0].ID)
	}
	if project.Tasks[1].ID != "1" {
		t.Errorf("expected task1 at position 1, got %s", project.Tasks[1].ID)
	}

	// Move task2 up again (already at top, should be no-op)
	err = project.MoveTaskUp(task2)
	if err != nil {
		t.Errorf("MoveTaskUp at top returned error: %v", err)
	}
	if project.Tasks[0].ID != "2" {
		t.Error("task2 should still be at top")
	}
}

func TestProjectMoveTaskDown(t *testing.T) {
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

	// Move task2 down (should swap with task3)
	err = project.MoveTaskDown(task2)
	if err != nil {
		t.Errorf("MoveTaskDown returned error: %v", err)
	}

	// Check order: should be task1, task3, task2
	if project.Tasks[1].ID != "3" {
		t.Errorf("expected task3 at position 1, got %s", project.Tasks[1].ID)
	}
	if project.Tasks[2].ID != "2" {
		t.Errorf("expected task2 at position 2, got %s", project.Tasks[2].ID)
	}

	// Move task2 down again (already at bottom, should be no-op)
	err = project.MoveTaskDown(task2)
	if err != nil {
		t.Errorf("MoveTaskDown at bottom returned error: %v", err)
	}
	if project.Tasks[2].ID != "2" {
		t.Error("task2 should still be at bottom")
	}
}

func TestProjectMoveTaskUpWithChildren(t *testing.T) {
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

	epicID := epic.ID
	child1.ParentID = &epicID
	child2.ParentID = &epicID

	project.Tasks = []*Task{epic, child1, child2}
	project.BuildTaskTree()

	// Move child2 up (swap with child1)
	err = project.MoveTaskUp(child2)
	if err != nil {
		t.Errorf("MoveTaskUp returned error: %v", err)
	}

	// Get children order
	siblings := project.GetSiblings(child2)
	if siblings[0].ID != "child2" {
		t.Errorf("expected child2 first, got %s", siblings[0].ID)
	}
}

func TestProjectIndentTask(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("task", 2, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{epic, task}
	project.BuildTaskTree()

	// Indent task under epic
	err = project.IndentTask(task)
	if err != nil {
		t.Errorf("IndentTask returned error: %v", err)
	}

	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Error("task should now be child of epic")
	}
}

func TestProjectOutdentTask(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	story, err := NewTask("story", 2, TaskTypeStory, "Story")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("task", 3, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}

	epicID := epic.ID
	storyID := story.ID
	story.ParentID = &epicID
	task.ParentID = &storyID

	project.Tasks = []*Task{epic, story, task}
	project.BuildTaskTree()

	// Outdent task (should become child of epic)
	err = project.OutdentTask(task)
	if err != nil {
		t.Errorf("OutdentTask returned error: %v", err)
	}

	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Error("task should now be child of epic")
	}

	// Outdent story (should become root)
	err = project.OutdentTask(story)
	if err != nil {
		t.Errorf("OutdentTask returned error: %v", err)
	}

	if story.ParentID != nil {
		t.Error("story should now be root")
	}
}

func TestIndentTaskReturnsErrorWhenNoTaskAbove(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task}
	project.BuildTaskTree()

	// First task has nothing above it
	err = project.IndentTask(task)
	if err == nil {
		t.Error("expected error when indenting first task, got nil")
	}
}

func TestIndentTaskReturnsErrorWhenTypeInvalid(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 2, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2}
	project.BuildTaskTree()

	// Epic cannot be child of Task
	err = project.IndentTask(task2)
	if err == nil {
		t.Error("expected error when indenting epic under task, got nil")
	}
}

func TestBuildTaskTree_MaxNestingDepth(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	// Create a chain of 8 tasks nested sequentially: depth 0 → 1 → ... → 7
	const chainLen = 8
	tasks := make([]*Task, chainLen)
	for i := range chainLen {
		task, err := NewTask(fmt.Sprintf("task-%d", i), i+1, TaskTypeTask, fmt.Sprintf("Task depth %d", i))
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			parentID := tasks[i-1].ID
			task.ParentID = &parentID
		}
		tasks[i] = task
	}

	project.Tasks = tasks
	project.BuildTaskTree()

	// Tasks at depth 0 through MaxNestingDepth (6) should keep their hierarchy
	for i := 0; i <= MaxNestingDepth; i++ {
		if tasks[i].Level != i {
			t.Errorf("task at intended depth %d: got Level=%d, want %d", i, tasks[i].Level, i)
		}
		if i == 0 {
			if tasks[i].ParentID != nil {
				t.Errorf("root task should have nil ParentID")
			}
		} else {
			if tasks[i].ParentID == nil || *tasks[i].ParentID != tasks[i-1].ID {
				t.Errorf("task at depth %d should still have parent %s", i, tasks[i-1].ID)
			}
		}
	}

	// Tasks beyond MaxNestingDepth should be detached to root
	for i := MaxNestingDepth + 1; i < chainLen; i++ {
		if tasks[i].Level != 0 {
			t.Errorf("task at intended depth %d: got Level=%d, want 0 (should be detached)", i, tasks[i].Level)
		}
		if tasks[i].ParentID != nil {
			t.Errorf("task at intended depth %d: ParentID should be nil after detach, got %s", i, *tasks[i].ParentID)
		}
	}
}

func TestBuildTaskTree_MaxNestingDepthRelevelSubtree(t *testing.T) {
	// Regression: when a deeply nested task with children is detached to root,
	// its children must be re-leveled relative to the new root (level 0),
	// not retain their original (stale) levels.
	project := NewProject("test", "Test", "/tmp/test")

	// Create chain: 0 -> 1 -> ... -> MaxNestingDepth+1 -> child_a, child_b
	chainLen := MaxNestingDepth + 2 // one past the cap
	tasks := make([]*Task, chainLen)
	for i := range chainLen {
		task, err := NewTask(fmt.Sprintf("chain-%d", i), i+1, TaskTypeTask, fmt.Sprintf("Chain %d", i))
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			parentID := tasks[i-1].ID
			task.ParentID = &parentID
		}
		tasks[i] = task
	}

	// Add two children to the task that will be detached
	detachedRoot := tasks[chainLen-1]
	childA, err := NewTask("child-a", 100, TaskTypeTask, "Child A")
	if err != nil {
		t.Fatal(err)
	}
	childB, err := NewTask("child-b", 101, TaskTypeTask, "Child B")
	if err != nil {
		t.Fatal(err)
	}
	detachedID := detachedRoot.ID
	childA.ParentID = &detachedID
	childB.ParentID = &detachedID

	project.Tasks = append(tasks, childA, childB)
	project.BuildTaskTree()

	// The detached root should be at level 0
	if detachedRoot.Level != 0 {
		t.Errorf("detached root: got Level=%d, want 0", detachedRoot.Level)
	}
	// Its children should be at level 1 (not the original deep level)
	if childA.Level != 1 {
		t.Errorf("child A of detached root: got Level=%d, want 1", childA.Level)
	}
	if childB.Level != 1 {
		t.Errorf("child B of detached root: got Level=%d, want 1", childB.Level)
	}
}

func TestBuildTaskTree_CircularParentSelfReference(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task, err := NewTask("task-1", 1, TaskTypeTask, "Self-referencing task")
	if err != nil {
		t.Fatal(err)
	}
	// Set task as its own parent (corrupt data)
	selfID := task.ID
	task.ParentID = &selfID

	project.Tasks = []*Task{task}
	// Should not panic or infinite-loop
	project.BuildTaskTree()

	// Task should have been detached to root
	if task.ParentID != nil {
		t.Error("self-referencing task should have ParentID cleared")
	}
	if task.Parent != nil {
		t.Error("self-referencing task should have nil Parent")
	}
}

func TestBuildTaskTree_CircularParentMutualReference(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	taskA, err := NewTask("a", 1, TaskTypeTask, "Task A")
	if err != nil {
		t.Fatal(err)
	}
	taskB, err := NewTask("b", 2, TaskTypeTask, "Task B")
	if err != nil {
		t.Fatal(err)
	}

	// Create a mutual parent cycle: A -> B -> A
	idA := taskA.ID
	idB := taskB.ID
	taskA.ParentID = &idB
	taskB.ParentID = &idA

	project.Tasks = []*Task{taskA, taskB}
	// Should not panic or infinite-loop
	project.BuildTaskTree()

	// At least one of the tasks must have had its parent cleared to break the cycle
	if taskA.ParentID != nil && taskB.ParentID != nil {
		t.Error("circular parent chain should have been broken (at least one ParentID should be nil)")
	}
}

func TestBuildTaskTree_CircularParentLongChain(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	taskA, err := NewTask("a", 1, TaskTypeTask, "Task A")
	if err != nil {
		t.Fatal(err)
	}
	taskB, err := NewTask("b", 2, TaskTypeTask, "Task B")
	if err != nil {
		t.Fatal(err)
	}
	taskC, err := NewTask("c", 3, TaskTypeTask, "Task C")
	if err != nil {
		t.Fatal(err)
	}

	// Create a 3-node cycle: A -> B -> C -> A
	idA := taskA.ID
	idB := taskB.ID
	idC := taskC.ID
	taskA.ParentID = &idB
	taskB.ParentID = &idC
	taskC.ParentID = &idA

	project.Tasks = []*Task{taskA, taskB, taskC}
	// Should not panic or infinite-loop
	project.BuildTaskTree()

	// At least one of the tasks must have had its parent cleared
	nilCount := 0
	for _, task := range project.Tasks {
		if task.ParentID == nil {
			nilCount++
		}
	}
	if nilCount == 0 {
		t.Error("circular parent chain should have been broken (at least one ParentID should be nil)")
	}
}

func TestOutdentTaskReturnsErrorAtRoot(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task}
	project.BuildTaskTree()

	err = project.OutdentTask(task)
	if err == nil {
		t.Error("expected error when outdenting root task, got nil")
	}
}

func TestBuildTaskTree_OrphanedDependencyCleanup(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1, err := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task-2", 2, TaskTypeTask, "Task 2")
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
