package planner

import (
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
