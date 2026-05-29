package planner

import (
	"math"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestSerialScheduleNoDeps verifies independent tasks are chained sequentially.
func TestSerialScheduleNoDeps(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 2)
	b := newTaskWithDuration("b", 3)
	c := newTaskWithDuration("c", 1)

	sched := SerialSchedule([]*models.Task{c, b, a}, start, 1.0)
	if len(sched) != 3 {
		t.Fatalf("expected 3 scheduled tasks, got %d", len(sched))
	}

	// Verify each task starts where the previous one ends.
	for i := 1; i < len(sched); i++ {
		if !sched[i].EarliestStart.Equal(sched[i-1].EarliestEnd) {
			t.Errorf("task %d start (%v) != task %d end (%v)",
				i, sched[i].EarliestStart, i-1, sched[i-1].EarliestEnd)
		}
	}

	// First task starts at startDate.
	if !sched[0].EarliestStart.Equal(start) {
		t.Errorf("first task should start at %v, got %v", start, sched[0].EarliestStart)
	}
}

// TestSerialScheduleWithDeps verifies deps are respected AND no overlap.
func TestSerialScheduleWithDeps(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 2)               // independent
	b := newTaskWithDuration("b", 1)               // independent
	c := withDep(newTaskWithDuration("c", 3), "a") // depends on a

	sched := SerialSchedule([]*models.Task{c, b, a}, start, 1.0)
	if len(sched) != 3 {
		t.Fatalf("expected 3 scheduled tasks, got %d", len(sched))
	}
	byID := scheduleByID(sched)

	// c must start after a ends.
	if byID["c"].EarliestStart.Before(byID["a"].EarliestEnd) {
		t.Errorf("c started before a ended: c.start=%v a.end=%v",
			byID["c"].EarliestStart, byID["a"].EarliestEnd)
	}

	// No two tasks overlap.
	for i := range sched {
		for j := i + 1; j < len(sched); j++ {
			si, sj := sched[i], sched[j]
			if si.EarliestStart.Before(sj.EarliestEnd) && sj.EarliestStart.Before(si.EarliestEnd) {
				t.Errorf("overlap between %s [%v,%v) and %s [%v,%v)",
					si.Task.ID, si.EarliestStart, si.EarliestEnd,
					sj.Task.ID, sj.EarliestStart, sj.EarliestEnd)
			}
		}
	}
}

// TestSerialSchedulePriority verifies higher-priority tasks are scheduled first.
func TestSerialSchedulePriority(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	low := newTaskWithDuration("low", 1)
	low.Priority = 3
	high := newTaskWithDuration("high", 1)
	high.Priority = 1
	med := newTaskWithDuration("med", 1)
	med.Priority = 2

	sched := SerialSchedule([]*models.Task{low, med, high}, start, 1.0)
	if len(sched) != 3 {
		t.Fatalf("expected 3, got %d", len(sched))
	}

	// Should be ordered: high, med, low.
	if sched[0].Task.ID != "high" {
		t.Errorf("expected first task to be 'high', got %q", sched[0].Task.ID)
	}
	if sched[1].Task.ID != "med" {
		t.Errorf("expected second task to be 'med', got %q", sched[1].Task.ID)
	}
	if sched[2].Task.ID != "low" {
		t.Errorf("expected third task to be 'low', got %q", sched[2].Task.ID)
	}
}

// TestSerialScheduleUnsetPriorityTreatedAsDefault guards against a bug where
// tasks with Priority=0 (the model's "unset" sentinel) sorted ahead of
// Priority=1 (the most critical bucket) in SerialSchedule. Tasks parsed from
// markdown can carry Priority=0, and treating them as more critical than
// genuinely-critical tasks would silently re-order user-prioritized work.
func TestSerialScheduleUnsetPriorityTreatedAsDefault(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	unset := newTaskWithDuration("unset", 1) // Priority defaults to 0 via direct struct init
	unset.Priority = 0
	critical := newTaskWithDuration("critical", 1)
	critical.Priority = 1
	medium := newTaskWithDuration("medium", 1)
	medium.Priority = 3 // explicit default

	sched := SerialSchedule([]*models.Task{unset, critical, medium}, start, 1.0)
	if len(sched) != 3 {
		t.Fatalf("expected 3, got %d", len(sched))
	}

	// critical (p=1) must come first; unset (p=0 -> default 3) must NOT.
	if sched[0].Task.ID != "critical" {
		t.Errorf("expected first task to be 'critical' (p=1), got %q (priority=0 tasks must not jump ahead)",
			sched[0].Task.ID)
	}
	// unset (p=0) should be treated as medium (p=3); tie-broken by ID against medium (p=3).
	// IDs sort: "medium" < "unset", so medium should come before unset.
	if sched[1].Task.ID != "medium" {
		t.Errorf("expected second task to be 'medium', got %q", sched[1].Task.ID)
	}
	if sched[2].Task.ID != "unset" {
		t.Errorf("expected third task to be 'unset' (p=0 treated as default), got %q", sched[2].Task.ID)
	}
}

func TestSerialScheduleNaNVelocityFallback(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 2)
	b := newTaskWithDuration("b", 1)

	schedNaN := SerialSchedule([]*models.Task{a, b}, start, math.NaN())
	schedOne := SerialSchedule([]*models.Task{a, b}, start, 1.0)

	if len(schedNaN) != 2 || len(schedOne) != 2 {
		t.Fatal("expected 2 scheduled tasks")
	}
	for i := range schedNaN {
		if schedNaN[i].Task.ID != schedOne[i].Task.ID ||
			!schedNaN[i].EarliestStart.Equal(schedOne[i].EarliestStart) ||
			!schedNaN[i].EarliestEnd.Equal(schedOne[i].EarliestEnd) ||
			schedNaN[i].Slack != schedOne[i].Slack ||
			schedNaN[i].IsCritical != schedOne[i].IsCritical {
			t.Errorf("NaN velocity should behave like velocity=1 at index %d: %+v vs %+v",
				i, schedNaN[i], schedOne[i])
		}
	}
}

func TestSerialScheduleQualifiedRefDoesNotBleedToPlainID(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	local := newTaskWithDuration("z", 1)
	local.Priority = 3
	dependent := withDep(newTaskWithDuration("b", 1), "missing-project:z")
	dependent.Priority = 1

	sched := SerialSchedule([]*models.Task{local, dependent}, start, 1.0)
	if len(sched) != 2 {
		t.Fatalf("expected 2 scheduled tasks, got %d", len(sched))
	}
	if sched[0].Task.ID != "b" {
		t.Errorf("qualified missing-project:z should not force local z before high-priority b, got %v", taskIDsFromSchedule(sched))
	}
}

func taskIDsFromSchedule(sched []ScheduledTask) []string {
	ids := make([]string, len(sched))
	for i, s := range sched {
		ids[i] = s.Task.ID
	}
	return ids
}
