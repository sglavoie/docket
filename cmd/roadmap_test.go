package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/metrics"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/planner"
	"github.com/sglavoie/tt/internal/storage"
)

// TestRoadmapWeekStartPropagation pins the wiring between cfg.GetWeekEndDay()
// and metrics.WeekStartFromWeekEnd that runRoadmap performs at the
// ComputeAllAtWithWeekStart call site. The full Cobra path is awkward to
// drive in a unit test (it touches disk-backed config and project storage),
// so we exercise just the cfg→metrics handshake.
func TestRoadmapWeekStartPropagation(t *testing.T) {
	cases := []struct {
		weekEnd string
		want    time.Weekday
	}{
		{"sun", time.Monday},   // default
		{"fri", time.Saturday}, // week ends Fri → starts Sat
		{"sat", time.Sunday},
	}
	for _, c := range cases {
		cfg := config.NewSettings()
		cfg.WeekEndDay = c.weekEnd
		got := metrics.WeekStartFromWeekEnd(cfg.GetWeekEndDay())
		if got != c.want {
			t.Errorf("WeekEndDay=%q: got week-start %v, want %v", c.weekEnd, got, c.want)
		}
	}
}

// TestRoadmapWeekStartUnsetDefaults verifies that an unset WeekEndDay (the
// zero value) flows through GetWeekEndDay() (which returns "sun") and lands
// on Monday as the week-start — matching metrics.DefaultWeekStart.
func TestRoadmapWeekStartUnsetDefaults(t *testing.T) {
	cfg := config.NewSettings()
	// WeekEndDay deliberately left empty.
	got := metrics.WeekStartFromWeekEnd(cfg.GetWeekEndDay())
	if got != time.Monday {
		t.Errorf("unset WeekEndDay should default to Monday week-start, got %v", got)
	}
}

func TestRoadmapStorageSummariesForMetricsPreservesEffortFields(t *testing.T) {
	startedAt := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	completedAt := time.Date(2026, 5, 2, 17, 0, 0, 0, time.UTC)
	got := roadmapStorageSummariesForMetrics([]storage.ArchiveTaskSummary{
		{
			ID:             "archived",
			WorkflowState:  models.StateDone,
			CompletedAt:    &completedAt,
			FirstStartedAt: &startedAt,
			EstimatedDays:  5,
			Effort:         models.EffortLarge,
			StoryPoints:    8,
		},
	})

	if len(got) != 1 {
		t.Fatalf("got %d summaries, want 1", len(got))
	}
	if got[0].ID != "archived" ||
		got[0].WorkflowState != models.StateDone ||
		got[0].CompletedAt != &completedAt ||
		got[0].FirstStartedAt != &startedAt ||
		got[0].EstimatedDays != 5 ||
		got[0].Effort != models.EffortLarge ||
		got[0].StoryPoints != 8 {
		t.Fatalf("summary fields were not preserved: %+v", got[0])
	}

	velocity := planner.WeightedVelocityWithSummaries(metrics.ThroughputResult{Total: 1, AvgPerDay: 1}, got)
	if velocity != 3 {
		t.Fatalf("velocity = %v, want 3 from archived large effort summary", velocity)
	}
}
