package metrics

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestCycleTime(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("basic cycle time", func(t *testing.T) {
		start1 := now.AddDate(0, 0, -5)
		done1 := now.AddDate(0, 0, -2)
		start2 := now.AddDate(0, 0, -3)
		done2 := now.AddDate(0, 0, -1)

		tasks := []*models.Task{
			makeTask(models.StatusDone, timePtr(done1), []*models.ActivityEntry{
				{Timestamp: start1, Field: "status", OldValue: "todo", NewValue: "in_progress"},
			}),
			makeTask(models.StatusDone, timePtr(done2), []*models.ActivityEntry{
				{Timestamp: start2, Field: "status", OldValue: "todo", NewValue: "in_progress"},
			}),
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeCycleTime(tasks, cutoff, now, buckets)

		if result.Count != 2 {
			t.Errorf("Count = %d, want 2", result.Count)
		}

		expectedAvgHours := 60.0
		actualAvgHours := result.Average.Hours()
		if actualAvgHours < expectedAvgHours-1 || actualAvgHours > expectedAvgHours+1 {
			t.Errorf("Average = %v hours, want ~%v hours", actualAvgHours, expectedAvgHours)
		}
	})

	t.Run("skips tasks without in_progress transition", func(t *testing.T) {
		done := now.AddDate(0, 0, -1)

		tasks := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), nil),
			makeTask(models.StatusDone, timePtr(done), []*models.ActivityEntry{
				{Timestamp: now.AddDate(0, 0, -3), Field: "priority", OldValue: "3", NewValue: "1"},
			}),
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeCycleTime(tasks, cutoff, now, buckets)

		if result.Count != 0 {
			t.Errorf("Count = %d, want 0 (no in_progress transitions)", result.Count)
		}
	})

	t.Run("uses first in_progress transition", func(t *testing.T) {
		first := now.AddDate(0, 0, -10)
		second := now.AddDate(0, 0, -5)
		done := now.AddDate(0, 0, -1)

		tasks := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), []*models.ActivityEntry{
				{Timestamp: first, Field: "status", OldValue: "todo", NewValue: "in_progress"},
				{Timestamp: second, Field: "status", OldValue: "blocked", NewValue: "in_progress"},
			}),
		}

		cutoff := TimeRange30d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeCycleTime(tasks, cutoff, now, buckets)

		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}

		expectedHours := 9.0 * 24
		actualHours := result.Average.Hours()
		if actualHours < expectedHours-1 || actualHours > expectedHours+1 {
			t.Errorf("Average = %v hours, want ~%v hours", actualHours, expectedHours)
		}
	})
}