package metrics

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestThroughput(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("counts only done tasks", func(t *testing.T) {
		done := now.AddDate(0, 0, -2)
		canceled := now.AddDate(0, 0, -1)

		tasks := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), nil),
			makeTask(models.StatusDone, timePtr(done), nil),
			makeTask(models.StatusCanceled, timePtr(canceled), nil),
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeThroughput(tasks, cutoff, now, buckets)

		if result.Total != 2 {
			t.Errorf("Total = %d, want 2 (canceled excluded)", result.Total)
		}
	})

	t.Run("average per day", func(t *testing.T) {
		tasks := make([]*models.Task, 7)
		for i := range tasks {
			done := now.AddDate(0, 0, -i)
			tasks[i] = makeTask(models.StatusDone, timePtr(done), nil)
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeThroughput(tasks, cutoff, now, buckets)

		if result.Total != 7 {
			t.Errorf("Total = %d, want 7", result.Total)
		}
		if result.AvgPerDay < 0.9 || result.AvgPerDay > 1.1 {
			t.Errorf("AvgPerDay = %f, want ~1.0", result.AvgPerDay)
		}
	})
}

func TestBlocked(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("single blocked period", func(t *testing.T) {
		blockedAt := now.AddDate(0, 0, -5)
		unblockedAt := now.AddDate(0, 0, -3)

		task := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
			{Timestamp: blockedAt, Field: "status", OldValue: "in_progress", NewValue: "blocked"},
			{Timestamp: unblockedAt, Field: "status", OldValue: "blocked", NewValue: "in_progress"},
		})

		result := computeBlocked([]*models.Task{task}, now, nil)

		if result.TaskCount != 1 {
			t.Errorf("TaskCount = %d, want 1", result.TaskCount)
		}

		expectedHours := 48.0
		actualHours := result.Entries[0].TotalBlocked.Hours()
		if actualHours < expectedHours-1 || actualHours > expectedHours+1 {
			t.Errorf("TotalBlocked = %v hours, want ~%v hours", actualHours, expectedHours)
		}
		if result.Entries[0].BlockedPeriods != 1 {
			t.Errorf("BlockedPeriods = %d, want 1", result.Entries[0].BlockedPeriods)
		}
	})

	t.Run("currently blocked extends to now", func(t *testing.T) {
		blockedAt := now.AddDate(0, 0, -2)

		task := makeTask(models.StatusBlocked, nil, []*models.ActivityEntry{
			{Timestamp: blockedAt, Field: "status", OldValue: "in_progress", NewValue: "blocked"},
		})

		result := computeBlocked([]*models.Task{task}, now, nil)

		if result.TaskCount != 1 {
			t.Errorf("TaskCount = %d, want 1", result.TaskCount)
		}

		expectedHours := 48.0
		actualHours := result.Entries[0].TotalBlocked.Hours()
		if actualHours < expectedHours-1 || actualHours > expectedHours+1 {
			t.Errorf("TotalBlocked = %v hours, want ~%v hours", actualHours, expectedHours)
		}
	})

	t.Run("multiple blocked periods", func(t *testing.T) {
		task := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
			{Timestamp: now.AddDate(0, 0, -10), Field: "status", OldValue: "in_progress", NewValue: "blocked"},
			{Timestamp: now.AddDate(0, 0, -8), Field: "status", OldValue: "blocked", NewValue: "in_progress"},
			{Timestamp: now.AddDate(0, 0, -5), Field: "status", OldValue: "in_progress", NewValue: "blocked"},
			{Timestamp: now.AddDate(0, 0, -3), Field: "status", OldValue: "blocked", NewValue: "in_progress"},
		})

		result := computeBlocked([]*models.Task{task}, now, nil)

		if result.Entries[0].BlockedPeriods != 2 {
			t.Errorf("BlockedPeriods = %d, want 2", result.Entries[0].BlockedPeriods)
		}

		expectedHours := 96.0
		actualHours := result.Entries[0].TotalBlocked.Hours()
		if actualHours < expectedHours-1 || actualHours > expectedHours+1 {
			t.Errorf("TotalBlocked = %v hours, want ~%v hours", actualHours, expectedHours)
		}
	})

	t.Run("pruned end transition is skipped", func(t *testing.T) {
		blockedAt := now.AddDate(0, 0, -5)

		task := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
			{Timestamp: blockedAt, Field: "status", OldValue: "in_progress", NewValue: "blocked"},
		})

		result := computeBlocked([]*models.Task{task}, now, nil)

		if result.TaskCount != 0 {
			t.Errorf("TaskCount = %d, want 0 (pruned period should be skipped)", result.TaskCount)
		}
	})

	t.Run("sorted by total blocked descending", func(t *testing.T) {
		task1 := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
			{Timestamp: now.AddDate(0, 0, -3), Field: "status", OldValue: "in_progress", NewValue: "blocked"},
			{Timestamp: now.AddDate(0, 0, -2), Field: "status", OldValue: "blocked", NewValue: "in_progress"},
		})
		task1.ID = "short-block"

		task2 := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
			{Timestamp: now.AddDate(0, 0, -10), Field: "status", OldValue: "in_progress", NewValue: "blocked"},
			{Timestamp: now.AddDate(0, 0, -2), Field: "status", OldValue: "blocked", NewValue: "in_progress"},
		})
		task2.ID = "long-block"

		result := computeBlocked([]*models.Task{task1, task2}, now, nil)

		if len(result.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result.Entries))
		}
		if result.Entries[0].Task.ID != "long-block" {
			t.Errorf("expected longest-blocked first, got %s", result.Entries[0].Task.ID)
		}
	})
}