package models

import "testing"

func TestBuildTaskTree_CircularParentSelfReference(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

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
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

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
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

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

func TestBuildTaskTree_CircularParentFullCycleCleared(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

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
	project.BuildTaskTree()

	// Verify no cycles remain: following any task's parent chain must not loop.
	for _, task := range project.Tasks {
		visited := make(map[string]bool)
		current := task
		for current != nil && current.ParentID != nil {
			if visited[current.ID] {
				t.Errorf("cycle still present after BuildTaskTree: task %q (#%d)", task.Title, task.ShortID)
				break
			}
			visited[current.ID] = true
			// Walk via ParentID pointer through the task map
			foundParent := false
			for _, t2 := range project.Tasks {
				if t2.ID == *current.ParentID {
					current = t2
					foundParent = true
					break
				}
			}
			if !foundParent {
				break
			}
		}
	}
}

func TestWouldCreateCycle_SelfCycle(t *testing.T) {
	taskA, err := NewTask("a", 1, TaskTypeTask, "Task A")
	if err != nil {
		t.Fatal(err)
	}
	taskMap := map[string]*Task{taskA.ID: taskA}

	if !WouldCreateCycle(taskMap, taskA.ID, taskA.ID) {
		t.Error("expected self-cycle (A->A) to be detected")
	}
}

func TestWouldCreateCycle_IndirectCycle(t *testing.T) {
	taskA, err := NewTask("a", 1, TaskTypeTask, "Task A")
	if err != nil {
		t.Fatal(err)
	}
	taskB, err := NewTask("b", 2, TaskTypeTask, "Task B")
	if err != nil {
		t.Fatal(err)
	}

	// B is already a child of A
	idA := taskA.ID
	taskB.ParentID = &idA
	taskMap := map[string]*Task{taskA.ID: taskA, taskB.ID: taskB}

	// Setting A's parent to B would create A->B->A cycle
	if !WouldCreateCycle(taskMap, taskA.ID, taskB.ID) {
		t.Error("expected indirect cycle (A->B->A) to be detected")
	}
}

func TestWouldCreateCycle_ValidReparent(t *testing.T) {
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

	// A->B already exists; reparenting C under B should be fine
	idA := taskA.ID
	taskB.ParentID = &idA
	taskMap := map[string]*Task{taskA.ID: taskA, taskB.ID: taskB, taskC.ID: taskC}

	if WouldCreateCycle(taskMap, taskC.ID, taskB.ID) {
		t.Error("reparenting C under B should not create a cycle")
	}
}

func TestAddChild_CycleDetected(t *testing.T) {
	// a (Story) is already the parent of b (Epic) via Parent pointer.
	// Attempting to add a as a child of b would create cycle b->a->b.
	a, err := NewTask("a", 1, TaskTypeStory, "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewTask("b", 2, TaskTypeEpic, "B")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate existing tree: a is b's parent (set via pointer, not AddChild)
	b.Parent = a
	b.Level = 1

	// Epic can have Story children (type check passes), but a is already
	// an ancestor of b so this would create a cycle.
	err = b.AddChild(a)
	if err == nil {
		t.Error("expected error when AddChild would create a circular reference")
	}
}