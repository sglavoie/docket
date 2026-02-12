package metrics

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// helper to create a time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}

// helper to make a task with activity
func makeTask(status models.TaskStatus, completedAt *time.Time, activity []*models.ActivityEntry) *models.Task {
	return &models.Task{
		ID:          "test-task",
		Status:      status,
		CompletedAt: completedAt,
		Activity:    activity,
	}
}

func TestTimeRange(t *testing.T) {
	t.Run("Label", func(t *testing.T) {
		tests := []struct {
			tr    TimeRange
			label string
		}{
			{TimeRange7d, "7 days"},
			{TimeRange30d, "30 days"},
			{TimeRange90d, "90 days"},
			{TimeRangeAll, "All time"},
		}
		for _, tt := range tests {
			if got := tt.tr.Label(); got != tt.label {
				t.Errorf("TimeRange(%d).Label() = %q, want %q", tt.tr, got, tt.label)
			}
		}
	})

	t.Run("Days", func(t *testing.T) {
		if got := TimeRange7d.Days(); got != 7 {
			t.Errorf("TimeRange7d.Days() = %d, want 7", got)
		}
		if got := TimeRangeAll.Days(); got != 0 {
			t.Errorf("TimeRangeAll.Days() = %d, want 0", got)
		}
	})

	t.Run("Cutoff", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		cutoff := TimeRange30d.Cutoff(now)
		expected := now.AddDate(0, 0, -30)
		if !cutoff.Equal(expected) {
			t.Errorf("Cutoff = %v, want %v", cutoff, expected)
		}

		allCutoff := TimeRangeAll.Cutoff(now)
		if !allCutoff.IsZero() {
			t.Errorf("TimeRangeAll.Cutoff should be zero, got %v", allCutoff)
		}
	})

	t.Run("Next and Prev cycle", func(t *testing.T) {
		if got := TimeRange7d.Next(); got != TimeRange30d {
			t.Errorf("7d.Next() = %v, want 30d", got)
		}
		if got := TimeRangeAll.Next(); got != TimeRange7d {
			t.Errorf("All.Next() = %v, want 7d", got)
		}
		if got := TimeRange7d.Prev(); got != TimeRangeAll {
			t.Errorf("7d.Prev() = %v, want All", got)
		}
		if got := TimeRange30d.Prev(); got != TimeRange7d {
			t.Errorf("30d.Prev() = %v, want 7d", got)
		}
	})
}

func TestBuckets(t *testing.T) {
	t.Run("daily buckets for 7d range", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		cutoff := now.AddDate(0, 0, -7)
		buckets := timeBuckets(cutoff, now, granularityDay)

		// Should have 7-8 daily buckets depending on alignment
		if len(buckets) < 7 || len(buckets) > 8 {
			t.Errorf("expected 7-8 daily buckets, got %d", len(buckets))
		}

		// Verify buckets are contiguous
		for i := 1; i < len(buckets); i++ {
			if !buckets[i].Start.Equal(buckets[i-1].End) {
				t.Errorf("bucket %d start (%v) != bucket %d end (%v)",
					i, buckets[i].Start, i-1, buckets[i-1].End)
			}
		}
	})

	t.Run("weekly buckets for 90d range", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		cutoff := now.AddDate(0, 0, -90)
		buckets := timeBuckets(cutoff, now, granularityWeek)

		// 90 days / 7 = ~13 weeks
		if len(buckets) < 12 || len(buckets) > 14 {
			t.Errorf("expected 12-14 weekly buckets, got %d", len(buckets))
		}
	})

	t.Run("monthly buckets for all-time", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		buckets := timeBuckets(time.Time{}, now, granularityMonth)

		// All-time defaults to 12 months back
		if len(buckets) < 12 || len(buckets) > 13 {
			t.Errorf("expected 12-13 monthly buckets, got %d", len(buckets))
		}
	})

	t.Run("assignToBucket", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		cutoff := now.AddDate(0, 0, -7)
		buckets := timeBuckets(cutoff, now, granularityDay)

		// A timestamp within the range should find a bucket
		mid := now.AddDate(0, 0, -3)
		idx := assignToBucket(mid, buckets)
		if idx < 0 {
			t.Errorf("expected valid bucket index for %v, got %d", mid, idx)
		}

		// A timestamp before the range should return -1
		before := cutoff.AddDate(0, 0, -5)
		idx = assignToBucket(before, buckets)
		if idx != -1 {
			t.Errorf("expected -1 for out-of-range timestamp, got %d", idx)
		}
	})
}

func TestCompletion(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("basic completion rate", func(t *testing.T) {
		done1 := now.AddDate(0, 0, -2)
		done2 := now.AddDate(0, 0, -3)
		canceled := now.AddDate(0, 0, -1)

		tasks := []*models.Task{
			makeTask(models.StatusDone, timePtr(done1), nil),
			makeTask(models.StatusDone, timePtr(done2), nil),
			makeTask(models.StatusCanceled, timePtr(canceled), nil),
			makeTask(models.StatusTodo, nil, nil), // not resolved
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeCompletion(tasks, cutoff, now, buckets)

		if result.Total != 3 {
			t.Errorf("Total = %d, want 3", result.Total)
		}
		if result.Completed != 2 {
			t.Errorf("Completed = %d, want 2", result.Completed)
		}
		expected := 2.0 / 3.0
		if result.Rate < expected-0.01 || result.Rate > expected+0.01 {
			t.Errorf("Rate = %f, want ~%f", result.Rate, expected)
		}
	})

	t.Run("excludes tasks before cutoff", func(t *testing.T) {
		old := now.AddDate(0, 0, -60)
		recent := now.AddDate(0, 0, -2)

		tasks := []*models.Task{
			makeTask(models.StatusDone, timePtr(old), nil),
			makeTask(models.StatusDone, timePtr(recent), nil),
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeCompletion(tasks, cutoff, now, buckets)

		if result.Total != 1 {
			t.Errorf("Total = %d, want 1 (should exclude old task)", result.Total)
		}
	})

	t.Run("zero tasks", func(t *testing.T) {
		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeCompletion(nil, cutoff, now, buckets)

		if result.Total != 0 || result.Rate != 0 {
			t.Errorf("expected zero metrics, got Total=%d Rate=%f", result.Total, result.Rate)
		}
	})
}

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

		// Task 1: 3 days, Task 2: 2 days → avg = 2.5 days
		expectedAvgHours := 60.0 // 2.5 days in hours
		actualAvgHours := result.Average.Hours()
		if actualAvgHours < expectedAvgHours-1 || actualAvgHours > expectedAvgHours+1 {
			t.Errorf("Average = %v hours, want ~%v hours", actualAvgHours, expectedAvgHours)
		}
	})

	t.Run("skips tasks without in_progress transition", func(t *testing.T) {
		done := now.AddDate(0, 0, -1)

		tasks := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), nil), // no activity
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

		// Should use first transition: 9 days cycle time
		expectedHours := 9.0 * 24
		actualHours := result.Average.Hours()
		if actualHours < expectedHours-1 || actualHours > expectedHours+1 {
			t.Errorf("Average = %v hours, want ~%v hours", actualHours, expectedHours)
		}
	})
}

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

		// 2 days + 2 days = 4 days
		expectedHours := 96.0
		actualHours := result.Entries[0].TotalBlocked.Hours()
		if actualHours < expectedHours-1 || actualHours > expectedHours+1 {
			t.Errorf("TotalBlocked = %v hours, want ~%v hours", actualHours, expectedHours)
		}
	})

	t.Run("pruned end transition is skipped", func(t *testing.T) {
		// Task was blocked but is now done (not blocked) — the unblock transition was pruned
		blockedAt := now.AddDate(0, 0, -5)

		task := makeTask(models.StatusDone, timePtr(now), []*models.ActivityEntry{
			{Timestamp: blockedAt, Field: "status", OldValue: "in_progress", NewValue: "blocked"},
			// No unblock transition — pruned. Task is StatusDone, not StatusBlocked.
		})

		result := computeBlocked([]*models.Task{task}, now, nil)

		// Should skip the partial period since task is not currently blocked
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

func TestComputeAll(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	done := now.AddDate(0, 0, -2)
	start := now.AddDate(0, 0, -4)

	project := &models.Project{
		ID:   "test-project",
		Name: "Test",
		Tasks: []*models.Task{
			makeTask(models.StatusDone, timePtr(done), []*models.ActivityEntry{
				{Timestamp: start, Field: "status", OldValue: "todo", NewValue: "in_progress"},
			}),
			makeTask(models.StatusTodo, nil, nil),
		},
	}

	result := ComputeAllAt([]*models.Project{project}, TimeRange30d, now)

	if result.Completion.Total != 1 {
		t.Errorf("Completion.Total = %d, want 1", result.Completion.Total)
	}
	if result.Throughput.Total != 1 {
		t.Errorf("Throughput.Total = %d, want 1", result.Throughput.Total)
	}
	if result.CycleTime.Count != 1 {
		t.Errorf("CycleTime.Count = %d, want 1", result.CycleTime.Count)
	}
	if result.TimeRange != TimeRange30d {
		t.Errorf("TimeRange = %v, want TimeRange30d", result.TimeRange)
	}
	if result.IsGlobal {
		t.Error("IsGlobal should be false for single project")
	}
}

func TestComputeAllMultiProject(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	done := now.AddDate(0, 0, -1)

	projects := []*models.Project{
		{ID: "p1", Tasks: []*models.Task{makeTask(models.StatusDone, timePtr(done), nil)}},
		{ID: "p2", Tasks: []*models.Task{makeTask(models.StatusDone, timePtr(done), nil)}},
	}

	result := ComputeAllAt(projects, TimeRange7d, now)

	if !result.IsGlobal {
		t.Error("IsGlobal should be true for multiple projects")
	}
	if result.Completion.Total != 2 {
		t.Errorf("Completion.Total = %d, want 2", result.Completion.Total)
	}
}

func TestComputeAllExtraTasks(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("zero extra args is backward compatible", func(t *testing.T) {
		done := now.AddDate(0, 0, -1)
		project := &models.Project{
			ID:    "p1",
			Tasks: []*models.Task{makeTask(models.StatusDone, timePtr(done), nil)},
		}

		result := ComputeAllAt([]*models.Project{project}, TimeRange7d, now)

		if result.Completion.Total != 1 {
			t.Errorf("Completion.Total = %d, want 1", result.Completion.Total)
		}
		if result.Throughput.Total != 1 {
			t.Errorf("Throughput.Total = %d, want 1", result.Throughput.Total)
		}
	})

	t.Run("extra tasks counted in completion and throughput", func(t *testing.T) {
		done := now.AddDate(0, 0, -1)
		project := &models.Project{
			ID:    "p1",
			Tasks: []*models.Task{makeTask(models.StatusDone, timePtr(done), nil)},
		}

		archived := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), nil),
			makeTask(models.StatusDone, timePtr(done), nil),
		}

		result := ComputeAllAt([]*models.Project{project}, TimeRange7d, now, archived)

		if result.Completion.Total != 3 {
			t.Errorf("Completion.Total = %d, want 3", result.Completion.Total)
		}
		if result.Throughput.Total != 3 {
			t.Errorf("Throughput.Total = %d, want 3", result.Throughput.Total)
		}
	})

	t.Run("extra tasks with activity contribute to cycle time", func(t *testing.T) {
		start := now.AddDate(0, 0, -5)
		done := now.AddDate(0, 0, -1)

		project := &models.Project{
			ID:    "p1",
			Tasks: []*models.Task{},
		}

		archived := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), []*models.ActivityEntry{
				{Timestamp: start, Field: "status", OldValue: "todo", NewValue: "in_progress"},
			}),
		}

		result := ComputeAllAt([]*models.Project{project}, TimeRange7d, now, archived)

		if result.CycleTime.Count != 1 {
			t.Errorf("CycleTime.Count = %d, want 1", result.CycleTime.Count)
		}
	})

	t.Run("extra tasks without activity skipped for cycle time", func(t *testing.T) {
		done := now.AddDate(0, 0, -1)

		project := &models.Project{
			ID:    "p1",
			Tasks: []*models.Task{},
		}

		archived := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), nil),
		}

		result := ComputeAllAt([]*models.Project{project}, TimeRange7d, now, archived)

		if result.CycleTime.Count != 0 {
			t.Errorf("CycleTime.Count = %d, want 0 (no activity)", result.CycleTime.Count)
		}
	})
}
