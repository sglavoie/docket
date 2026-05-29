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

	t.Run("excludes tasks completed after now", func(t *testing.T) {
		recent := now.AddDate(0, 0, -1)
		future := now.Add(2 * time.Hour)

		tasks := []*models.Task{
			makeTask(models.StatusDone, timePtr(recent), nil),
			makeTask(models.StatusDone, timePtr(future), nil),
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeThroughput(tasks, cutoff, now, buckets)

		if result.Total != 1 {
			t.Errorf("Total = %d, want 1 (future completion excluded)", result.Total)
		}
		var sum float64
		for _, count := range result.PerBucket {
			sum += count
		}
		if sum != 1 {
			t.Errorf("sum(PerBucket) = %v, want 1", sum)
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

	t.Run("handles blocked activity out of order", func(t *testing.T) {
		blockedAt := now.AddDate(0, 0, -5)
		unblockedAt := now.AddDate(0, 0, -3)

		task := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
			{Timestamp: unblockedAt, Field: "status", OldValue: "blocked", NewValue: "in_progress"},
			{Timestamp: blockedAt, Field: "status", OldValue: "in_progress", NewValue: "blocked"},
		})

		result := computeBlocked([]*models.Task{task}, now, nil)

		if result.TaskCount != 1 {
			t.Fatalf("TaskCount = %d, want 1", result.TaskCount)
		}
		if result.Entries[0].BlockedPeriods != 1 {
			t.Fatalf("BlockedPeriods = %d, want 1", result.Entries[0].BlockedPeriods)
		}

		expectedHours := 48.0
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

	t.Run("equal blocked durations are ordered by ID for stability", func(t *testing.T) {
		// Two tasks blocked for exactly the same duration must produce a
		// deterministic ordering across repeated invocations so the UI list
		// does not shuffle rows on each refresh.
		mkBlocked := func(id string) *models.Task {
			tsk := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
				{Timestamp: now.AddDate(0, 0, -4), Field: "status", OldValue: "in_progress", NewValue: "blocked"},
				{Timestamp: now.AddDate(0, 0, -2), Field: "status", OldValue: "blocked", NewValue: "in_progress"},
			})
			tsk.ID = id
			return tsk
		}

		// Repeat several times to defeat any incidental input-order coincidence.
		inputs := [][]*models.Task{
			{mkBlocked("c"), mkBlocked("a"), mkBlocked("b")},
			{mkBlocked("a"), mkBlocked("b"), mkBlocked("c")},
			{mkBlocked("b"), mkBlocked("c"), mkBlocked("a")},
		}
		for _, in := range inputs {
			result := computeBlocked(in, now, nil)
			if len(result.Entries) != 3 {
				t.Fatalf("expected 3 entries, got %d", len(result.Entries))
			}
			gotIDs := []string{result.Entries[0].Task.ID, result.Entries[1].Task.ID, result.Entries[2].Task.ID}
			wantIDs := []string{"a", "b", "c"}
			for i, w := range wantIDs {
				if gotIDs[i] != w {
					t.Fatalf("expected ID-sorted order %v, got %v", wantIDs, gotIDs)
				}
			}
		}
	})
}

// TestThroughputAllTimeAvgPerDayConsistent verifies that for TimeRangeAll
// (cutoff zero) the AvgPerDay calculation references the same task window
// as Total. Previously Total counted all tasks ever (including those before
// the 12-month bucket span) while AvgPerDay divided by ~365 days, producing
// a vastly inflated average for projects with old data.
func TestThroughputAllTimeAvgPerDayConsistent(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(-2, 0, 0)
	recent := now.AddDate(0, 0, -7)
	tasks := []*models.Task{
		makeTask(models.StatusDone, timePtr(old), nil),
		makeTask(models.StatusDone, timePtr(recent), nil),
	}

	cutoff := TimeRangeAll.Cutoff(now)
	buckets := timeBuckets(cutoff, now, bucketGranularity(TimeRangeAll))
	result := computeThroughput(tasks, cutoff, now, buckets)

	if result.Total != 1 {
		t.Errorf("Total = %d, want 1 (only the in-bucket task)", result.Total)
	}
	var sum float64
	for _, v := range result.PerBucket {
		sum += v
	}
	if int(sum) != result.Total {
		t.Errorf("sum(PerBucket) = %v != Total = %d", sum, result.Total)
	}
}

// TestThroughputBoundaryTaskCountedInBucket guards against tasks completed
// exactly at "now" being included in Total but excluded from PerBucket, which
// happens whenever "now" lands on a period boundary (midnight for daily
// buckets) since the last bucket's End == now.
func TestThroughputBoundaryTaskCountedInBucket(t *testing.T) {
	now := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC) // exactly midnight
	tasks := []*models.Task{
		makeTask(models.StatusDone, timePtr(now), nil),
	}
	cutoff := TimeRange7d.Cutoff(now)
	buckets := timeBuckets(cutoff, now, granularityDay)
	result := computeThroughput(tasks, cutoff, now, buckets)

	if result.Total != 1 {
		t.Fatalf("Total = %d, want 1", result.Total)
	}
	var sum float64
	for _, v := range result.PerBucket {
		sum += v
	}
	if int(sum) != result.Total {
		t.Errorf("sum(PerBucket) = %v, want %d (boundary task must hit final bucket)", sum, result.Total)
	}
}

func TestComputeAllAt_BlockedHonorsTimeRange(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	oldBlocked := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
		{Timestamp: now.AddDate(0, 0, -40), Field: "status", OldValue: "in_progress", NewValue: "blocked"},
		{Timestamp: now.AddDate(0, 0, -35), Field: "status", OldValue: "blocked", NewValue: "in_progress"},
	})
	oldBlocked.ID = "old-blocked"

	recentBlocked := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
		{Timestamp: now.AddDate(0, 0, -5), Field: "status", OldValue: "in_progress", NewValue: "blocked"},
		{Timestamp: now.AddDate(0, 0, -2), Field: "status", OldValue: "blocked", NewValue: "in_progress"},
	})
	recentBlocked.ID = "recent-blocked"

	project := &models.Project{
		ID:    "project-1",
		Name:  "Project 1",
		Tasks: []*models.Task{oldBlocked, recentBlocked},
	}

	result7d := ComputeAllAt([]*models.Project{project}, TimeRange7d, now)
	if result7d.Blocked.TaskCount != 1 {
		t.Fatalf("7d Blocked.TaskCount = %d, want 1", result7d.Blocked.TaskCount)
	}
	if got := result7d.Blocked.Entries[0].Task.ID; got != "recent-blocked" {
		t.Fatalf("7d blocked task = %q, want recent-blocked", got)
	}

	resultAll := ComputeAllAt([]*models.Project{project}, TimeRangeAll, now)
	if resultAll.Blocked.TaskCount != 2 {
		t.Fatalf("all-time Blocked.TaskCount = %d, want 2", resultAll.Blocked.TaskCount)
	}
}
