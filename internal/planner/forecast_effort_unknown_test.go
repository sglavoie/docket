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

// TestChildEffortDays_IdeaEffort verifies IdeaAttrs.Effort is recognized.
// Previously childEffortDays only consulted TaskAttrs.Effort and StoryAttrs,
// silently falling back to EstimatedDays/default for ideas with explicit
// effort levels. This made forecasting inconsistent with EstimateDuration
// (which already handled IdeaAttrs.Effort).
func TestChildEffortDays_IdeaEffort(t *testing.T) {
	task := newTask("idea")
	task.EnsureIdeaAttrs().Effort = models.EffortLarge
	// If childEffortDays falls through to EstimatedDays, this would dominate.
	task.EstimatedDays = 99

	days, hasEffort := childEffortDays(task)
	if !hasEffort {
		t.Errorf("expected hasEffortData=true for IdeaAttrs.Effort")
	}
	if days != effortDaysLarge {
		t.Errorf("expected days=%v (large idea effort), got %v", effortDaysLarge, days)
	}
}

// TestChildEffortDays_IdeaEffortAllVariants covers the full mapping for
// IdeaAttrs.Effort so a regression in any branch is caught.
func TestChildEffortDays_IdeaEffortAllVariants(t *testing.T) {
	cases := []struct {
		eff  models.EffortEstimate
		want float64
	}{
		{models.EffortSmall, effortDaysSmall},
		{models.EffortMedium, effortDaysMedium},
		{models.EffortLarge, effortDaysLarge},
		{models.EffortUnknown, effortDaysUnknown},
	}
	for _, tc := range cases {
		task := newTask("i")
		task.EnsureIdeaAttrs().Effort = tc.eff
		days, hasEffort := childEffortDays(task)
		if !hasEffort {
			t.Errorf("eff=%v: expected hasEffortData=true", tc.eff)
		}
		if days != tc.want {
			t.Errorf("eff=%v: expected %v, got %v", tc.eff, tc.want, days)
		}
	}
}

// TestChildEffortDays_TaskEffortBeatsIdea verifies that TaskAttrs.Effort still
// takes precedence over IdeaAttrs.Effort (a task should rarely carry both,
// but the priority must remain stable).
func TestChildEffortDays_TaskEffortBeatsIdea(t *testing.T) {
	task := withEffort(newTask("dual"), models.EffortSmall)
	task.EnsureIdeaAttrs().Effort = models.EffortLarge

	days, hasEffort := childEffortDays(task)
	if !hasEffort {
		t.Errorf("expected hasEffortData=true")
	}
	if days != effortDaysSmall {
		t.Errorf("TaskAttrs should win: expected %v, got %v", effortDaysSmall, days)
	}
}
