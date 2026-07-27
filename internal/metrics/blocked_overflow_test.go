package metrics

import (
	"math"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestBlockedAggregatesDoNotOverflow ensures TotalBlocked and AverageBlocked
// stay finite even when the cumulative blocked time would saturate
// time.Duration (int64 nanoseconds). Each task is blocked for ~50 years;
// with 20 tasks the naive int64 sum overflows.
func TestBlockedAggregatesDoNotOverflow(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	const blocked = 50 * 365 * 24 * time.Hour

	tasks := make([]*models.Task, 20)
	for i := range tasks {
		start := now.Add(-blocked - time.Hour)
		end := now.Add(-time.Hour)
		tasks[i] = makeTask(models.StateDone, timePtr(now), []*models.ActivityEntry{
			{Timestamp: start, Field: "state", OldValue: "in_progress", NewValue: "waiting"},
			{Timestamp: end, Field: "state", OldValue: "waiting", NewValue: "in_progress"},
		})
		tasks[i].ID = string(rune('a' + i))
	}

	result := computeBlocked(tasks, now, nil)

	if result.TaskCount != len(tasks) {
		t.Fatalf("TaskCount = %d, want %d", result.TaskCount, len(tasks))
	}
	// TotalBlocked should saturate to MaxInt64 rather than wrap negative.
	if result.TotalBlocked <= 0 {
		t.Errorf("TotalBlocked = %v, expected positive saturated value", result.TotalBlocked)
	}
	// AverageBlocked should be close to the per-task blocked duration.
	gotHours := result.AverageBlocked.Hours()
	wantHours := blocked.Hours()
	if math.Abs(gotHours-wantHours)/wantHours > 1e-6 {
		t.Errorf("AverageBlocked = %v hours, want ~%v hours", gotHours, wantHours)
	}
}

func TestBlockedEntryTotalDoesNotOverflow(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	const blocked = 50 * 365 * 24 * time.Hour

	var activity []*models.ActivityEntry
	for i := range 8 {
		start := time.Date(1500+i*60, 1, 1, 0, 0, 0, 0, time.UTC)
		end := start.Add(blocked)
		activity = append(activity,
			&models.ActivityEntry{Timestamp: start, Field: "state", OldValue: "in_progress", NewValue: "waiting"},
			&models.ActivityEntry{Timestamp: end, Field: "state", OldValue: "waiting", NewValue: "in_progress"},
		)
	}

	task := makeTask(models.StateDone, timePtr(now), activity)
	result := computeBlocked([]*models.Task{task}, now, nil)

	if result.TaskCount != 1 {
		t.Fatalf("TaskCount = %d, want 1", result.TaskCount)
	}
	if got := result.Entries[0].TotalBlocked; got != time.Duration(1<<63-1) {
		t.Fatalf("TotalBlocked = %v, want saturated MaxInt64 duration", got)
	}
	if got := result.Entries[0].BlockedPeriods; got != 8 {
		t.Fatalf("BlockedPeriods = %d, want 8", got)
	}
}
