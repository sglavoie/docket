package planner

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestCriticalPathLinear verifies the critical path for A(1) -> B(2) -> C(3).
// Expected length = 1+2+3 = 6; all tasks on critical path.
func TestCriticalPathLinear(t *testing.T) {
	a := newTaskWithDuration("a", 1)
	b := withDep(newTaskWithDuration("b", 2), "a")
	c := withDep(newTaskWithDuration("c", 3), "b")

	cp := CriticalPath([]*models.Task{a, b, c})

	const wantLen = 6.0
	if cp.Length != wantLen {
		t.Errorf("expected length %.1f, got %.1f", wantLen, cp.Length)
	}
	for _, id := range []string{"a", "b", "c"} {
		if !cp.OnPath[id] {
			t.Errorf("expected task %q on critical path", id)
		}
		if cp.Slack[id] != 0 {
			t.Errorf("expected zero slack for %q, got %f", id, cp.Slack[id])
		}
	}
}

// TestCriticalPathDiamond verifies slack for the non-critical path.
// A(1) -> B(1) -> D(1)
// A(1) -> C(3) -> D(1)
// Critical path: A->C->D length = 1+3+1 = 5; B has slack.
func TestCriticalPathDiamond(t *testing.T) {
	a := newTaskWithDuration("a", 1)
	b := withDep(newTaskWithDuration("b", 1), "a")
	c := withDep(newTaskWithDuration("c", 3), "a")
	d := withDep(newTaskWithDuration("d", 1), "b", "c")

	cp := CriticalPath([]*models.Task{a, b, c, d})

	const wantLen = 5.0
	if cp.Length != wantLen {
		t.Errorf("expected length %.1f, got %.1f", wantLen, cp.Length)
	}
	// a, c, d are on the critical path.
	for _, id := range []string{"a", "c", "d"} {
		if !cp.OnPath[id] {
			t.Errorf("expected %q on critical path", id)
		}
	}
	// b has slack = 2 (can start 2 days late without delaying the project).
	if cp.OnPath["b"] {
		t.Errorf("b should not be on critical path")
	}
	if cp.Slack["b"] <= 0 {
		t.Errorf("b should have positive slack, got %f", cp.Slack["b"])
	}
}

// TestCriticalPathNoDeps verifies single-task critical path.
func TestCriticalPathNoDeps(t *testing.T) {
	a := newTaskWithDuration("a", 2.5)
	cp := CriticalPath([]*models.Task{a})
	if cp.Length != 2.5 {
		t.Errorf("expected length 2.5, got %f", cp.Length)
	}
	if !cp.OnPath["a"] {
		t.Errorf("a should be on critical path")
	}
}

// TestCriticalPathDefaultDuration verifies fallback to 1.0 when EstimatedDays == 0.
func TestCriticalPathDefaultDuration(t *testing.T) {
	a := newTask("a") // EstimatedDays = 0, should default to 1.0
	b := withDep(newTask("b"), "a")
	cp := CriticalPath([]*models.Task{a, b})
	if cp.Length != 2.0 {
		t.Errorf("expected length 2.0, got %f", cp.Length)
	}
}

// TestCriticalPathSlackIsZeroForCriticalTasks verifies the slack invariant:
// slack == 0 iff the task is on the critical path.
func TestCriticalPathSlackIsZeroForCriticalTasks(t *testing.T) {
	a := newTaskWithDuration("a", 1)
	b := withDep(newTaskWithDuration("b", 1), "a") // short path
	c := withDep(newTaskWithDuration("c", 5), "a") // long path -> critical
	d := withDep(newTaskWithDuration("d", 1), "b", "c")

	cp := CriticalPath([]*models.Task{a, b, c, d})
	for id, onPath := range cp.OnPath {
		if onPath && cp.Slack[id] != 0 {
			t.Errorf("task %q on critical path but has non-zero slack %f", id, cp.Slack[id])
		}
	}
	for id, slack := range cp.Slack {
		if !cp.OnPath[id] && slack == 0 {
			t.Errorf("task %q not on critical path but has zero slack", id)
		}
	}
}

// TestCriticalPathWithEffort verifies CPM uses effort-based durations
// from TaskAttrs when EstimatedDays is not set.
func TestCriticalPathWithEffort(t *testing.T) {
	a := newTask("a")
	a.TaskAttrs = &models.TaskAttrs{Effort: models.EffortLarge} // 5 days
	b := withDep(newTask("b"), "a")
	b.TaskAttrs = &models.TaskAttrs{Effort: models.EffortSmall} // 0.5 days

	cp := CriticalPath([]*models.Task{a, b})

	// large=3.0 + small=0.5 = 3.5
	if abs(cp.Length-3.5) > 1e-9 {
		t.Errorf("expected length 3.5, got %f", cp.Length)
	}
	if !cp.OnPath["a"] || !cp.OnPath["b"] {
		t.Errorf("both tasks should be on the critical path")
	}
}
