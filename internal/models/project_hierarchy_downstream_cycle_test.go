package models

import "testing"

// TestBuildTaskTree_DownstreamCycleDoesNotDetachUpstreamTask verifies that
// when a parent-pointer cycle exists downstream of a task (e.g. A -> B with a
// B <-> C mutual cycle), the upstream task A is not wrongly detached. The
// previous implementation cleared A's ParentID whenever any cycle was detected
// during the walk from A, even though A itself was not part of the cycle.
func TestBuildTaskTree_DownstreamCycleDoesNotDetachUpstreamTask(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	taskA, err := NewGenericTask("a", 1, "Task A")
	if err != nil {
		t.Fatal(err)
	}
	taskB, err := NewGenericTask("b", 2, "Task B")
	if err != nil {
		t.Fatal(err)
	}
	taskC, err := NewGenericTask("c", 3, "Task C")
	if err != nil {
		t.Fatal(err)
	}

	// Topology: A -> B, B -> C, C -> B (B <-> C cycle, A is upstream).
	idB := taskB.ID
	idC := taskC.ID
	taskA.ParentID = &idB
	taskB.ParentID = &idC
	taskC.ParentID = &idB

	project.Tasks = []*Task{taskA, taskB, taskC}
	project.BuildTaskTree()

	// A's parent must be preserved because A is not part of the cycle.
	if taskA.ParentID == nil {
		t.Error("task A's parent was wrongly cleared even though A was not part of the downstream cycle")
	}
	if taskA.ParentID != nil && *taskA.ParentID != taskB.ID {
		t.Errorf("task A's parent should still point to B, got %q", *taskA.ParentID)
	}

	// The B <-> C cycle must have been broken: at least one of B, C should be a root.
	if taskB.ParentID != nil && taskC.ParentID != nil {
		t.Error("downstream cycle B <-> C should have been broken (B or C should be root)")
	}

	// Verify no cycles remain: following any task's parent chain must not loop.
	for _, task := range project.Tasks {
		visited := map[string]bool{}
		cur := task
		for cur != nil && cur.ParentID != nil {
			if visited[cur.ID] {
				t.Errorf("cycle still present after BuildTaskTree starting from %q", task.ID)
				break
			}
			visited[cur.ID] = true
			var next *Task
			for _, t2 := range project.Tasks {
				if t2.ID == *cur.ParentID {
					next = t2
					break
				}
			}
			cur = next
		}
	}
}
