package planner

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestForwardScheduleLinearChain verifies that chained tasks are scheduled
// sequentially with correct start/end dates.
func TestForwardScheduleLinearChain(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 1)               // ends day 1
	b := withDep(newTaskWithDuration("b", 2), "a") // starts day 1, ends day 3
	c := withDep(newTaskWithDuration("c", 1), "b") // starts day 3, ends day 4

	sched := ForwardSchedule([]*models.Task{c, b, a}, start, 1.0)
	if len(sched) != 3 {
		t.Fatalf("expected 3 scheduled tasks, got %d", len(sched))
	}
	byID := scheduleByID(sched)

	// a: starts at 0, ends at 1 day
	if !byID["a"].EarliestStart.Equal(start) {
		t.Errorf("a: expected start %v, got %v", start, byID["a"].EarliestStart)
	}
	wantAEnd := start.Add(24 * time.Hour)
	if !byID["a"].EarliestEnd.Equal(wantAEnd) {
		t.Errorf("a: expected end %v, got %v", wantAEnd, byID["a"].EarliestEnd)
	}

	// b: starts where a ends
	if !byID["b"].EarliestStart.Equal(wantAEnd) {
		t.Errorf("b: expected start %v, got %v", wantAEnd, byID["b"].EarliestStart)
	}
}

// TestForwardScheduleVelocityScaling verifies that higher velocity compresses
// the schedule proportionally.
func TestForwardScheduleVelocityScaling(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 4) // 4 days at velocity 1 = 4 days

	sched1 := ForwardSchedule([]*models.Task{a}, start, 1.0)
	sched2 := ForwardSchedule([]*models.Task{a}, start, 2.0) // twice as fast

	dur1 := sched1[0].EarliestEnd.Sub(sched1[0].EarliestStart)
	dur2 := sched2[0].EarliestEnd.Sub(sched2[0].EarliestStart)

	if dur2*2 != dur1 {
		t.Errorf("expected dur2 to be half dur1: dur1=%v dur2=%v", dur1, dur2)
	}
}

// TestForwardScheduleExplicitStartDate verifies that a task with an explicit
// StartDate respects it as a lower bound.
func TestForwardScheduleExplicitStartDate(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	a := newTaskWithDuration("a", 1)
	a.StartDate = &later // explicitly scheduled for Jan 5

	sched := ForwardSchedule([]*models.Task{a}, start, 1.0)
	if len(sched) == 0 {
		t.Fatal("expected scheduled tasks")
	}
	if !sched[0].EarliestStart.Equal(later) {
		t.Errorf("expected start %v, got %v", later, sched[0].EarliestStart)
	}
}

func TestForwardScheduleExplicitStartDateAffectsSlack(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := start.Add(10 * 24 * time.Hour)

	early := newTaskWithDuration("early", 1)
	gated := newTaskWithDuration("gated", 1)
	gated.StartDate = &later

	sched := ForwardSchedule([]*models.Task{early, gated}, start, 1.0)
	byID := scheduleByID(sched)

	if byID["early"].Slack != 10 {
		t.Errorf("expected early task to have 10 days slack, got %v", byID["early"].Slack)
	}
	if byID["early"].IsCritical {
		t.Error("early task should not be critical when a future-dated independent task controls completion")
	}
	if byID["gated"].Slack != 0 {
		t.Errorf("expected gated task to have zero slack, got %v", byID["gated"].Slack)
	}
	if !byID["gated"].IsCritical {
		t.Error("gated task should be critical because its explicit start controls completion")
	}
}

// TestForwardScheduleZeroVelocityFallback verifies that zero/negative velocity
// is treated as 1.0 (no panic, sensible output).
func TestForwardScheduleZeroVelocityFallback(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTaskWithDuration("a", 2)

	schedZero := ForwardSchedule([]*models.Task{a}, start, 0)
	schedOne := ForwardSchedule([]*models.Task{a}, start, 1.0)

	if len(schedZero) != 1 || len(schedOne) != 1 {
		t.Fatal("expected 1 scheduled task")
	}
	if !schedZero[0].EarliestEnd.Equal(schedOne[0].EarliestEnd) {
		t.Errorf("zero velocity should behave like velocity=1: %v vs %v",
			schedZero[0].EarliestEnd, schedOne[0].EarliestEnd)
	}
}

func TestForwardScheduleQualifiedRefDoesNotBleedToPlainID(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	local := newTaskWithDuration("z", 10)
	dependent := withDep(newTaskWithDuration("b", 1), "missing-project:z")

	sched := ForwardSchedule([]*models.Task{local, dependent}, start, 1.0)
	byID := scheduleByID(sched)

	if !byID["b"].EarliestStart.Equal(start) {
		t.Errorf("qualified missing-project:z should not delay b behind local z, got %v", byID["b"].EarliestStart)
	}
}
