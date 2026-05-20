package planner

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestChildEffortDays_UnknownReturnsExplicit is a regression test for a bug
// where childEffortDays fell through the EffortEstimate switch when the task
// had TaskAttrs.Effort == EffortUnknown. That violated the documented priority
// (TaskAttrs.Effort > StoryAttrs.StoryPoints > Task.EstimatedDays > default)
// because tasks with explicit "unknown" effort silently picked up StoryPoints
// or EstimatedDays instead of the unknown-effort default.
func TestChildEffortDays_UnknownReturnsExplicit(t *testing.T) {
	task := withEffort(newTask("u"), models.EffortUnknown)
	// Story points / EstimatedDays would otherwise dominate if the switch fell
	// through. Set both to clearly distinguishable values.
	task.EnsureStoryAttrs().StoryPoints = 10 // would yield 5.0 days
	task.EstimatedDays = 7.5                 // would yield 7.5 days

	days, hasEffort := childEffortDays(task)

	if !hasEffort {
		t.Errorf("expected hasEffortData=true when TaskAttrs.Effort is set to EffortUnknown")
	}
	if days != effortDaysUnknown {
		t.Errorf("expected days=%v for EffortUnknown, got %v", effortDaysUnknown, days)
	}
}

// TestChildEffortDays_KnownEffortStillWins guards the priority order: explicit
// known effort levels should continue to take precedence over StoryPoints and
// EstimatedDays.
func TestChildEffortDays_KnownEffortStillWins(t *testing.T) {
	task := withEffort(newTask("m"), models.EffortMedium)
	task.EnsureStoryAttrs().StoryPoints = 10
	task.EstimatedDays = 99

	days, hasEffort := childEffortDays(task)

	if !hasEffort {
		t.Errorf("expected hasEffortData=true for explicit EffortMedium")
	}
	if days != effortDaysMedium {
		t.Errorf("expected %v, got %v", effortDaysMedium, days)
	}
}
