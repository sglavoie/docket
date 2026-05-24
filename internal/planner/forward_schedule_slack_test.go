package planner

import (
	"math"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestForwardScheduleSlackVelocityScaling guards against a bug where the slack
// reported by ForwardSchedule was taken directly from CriticalPath (which works
// in unscaled days) without dividing by velocity, leaving slack out-of-sync
// with the velocity-scaled EarliestStart/EarliestEnd timestamps.
func TestForwardScheduleSlackVelocityScaling(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// a -> c (long chain: 4 + 6 = 10 raw days)
	// b alone (3 raw days) -> slack of 7 raw days at velocity 1.0
	a := newTaskWithDuration("a", 4)
	c := withDep(newTaskWithDuration("c", 6), "a")
	b := newTaskWithDuration("b", 3)

	schedV1 := ForwardSchedule([]*models.Task{a, b, c}, start, 1.0)
	schedV2 := ForwardSchedule([]*models.Task{a, b, c}, start, 2.0)

	slackV1 := scheduleByID(schedV1)["b"].Slack
	slackV2 := scheduleByID(schedV2)["b"].Slack

	// At velocity 2.0 the schedule is compressed 2x; slack must compress too.
	if math.Abs(slackV1-2*slackV2) > 1e-9 {
		t.Errorf("expected slack at v=2 to be half slack at v=1 (got %.3f vs %.3f)",
			slackV2, slackV1)
	}
}
