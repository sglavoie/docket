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
			makeTask(models.StateDone, timePtr(done1), []*models.ActivityEntry{
				{Timestamp: start1, Field: "state", OldValue: "todo", NewValue: "in_progress"},
			}),
			makeTask(models.StateDone, timePtr(done2), []*models.ActivityEntry{
				{Timestamp: start2, Field: "state", OldValue: "todo", NewValue: "in_progress"},
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
			makeTask(models.StateDone, timePtr(done), nil),
			makeTask(models.StateDone, timePtr(done), []*models.ActivityEntry{
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
			makeTask(models.StateDone, timePtr(done), []*models.ActivityEntry{
				{Timestamp: first, Field: "state", OldValue: "todo", NewValue: "in_progress"},
				{Timestamp: second, Field: "state", OldValue: "waiting", NewValue: "in_progress"},
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

	t.Run("uses earliest in_progress transition when activity is out of order", func(t *testing.T) {
		first := now.AddDate(0, 0, -10)
		second := now.AddDate(0, 0, -5)
		done := now.AddDate(0, 0, -1)

		tasks := []*models.Task{
			makeTask(models.StateDone, timePtr(done), []*models.ActivityEntry{
				{Timestamp: second, Field: "state", OldValue: "waiting", NewValue: "in_progress"},
				{Timestamp: first, Field: "state", OldValue: "todo", NewValue: "in_progress"},
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

	t.Run("excludes tasks completed after now", func(t *testing.T) {
		recentStart := now.AddDate(0, 0, -3)
		recentDone := now.AddDate(0, 0, -1)
		futureStart := now.AddDate(0, 0, -2)
		futureDone := now.Add(3 * time.Hour)

		tasks := []*models.Task{
			makeTask(models.StateDone, timePtr(recentDone), []*models.ActivityEntry{
				{Timestamp: recentStart, Field: "state", OldValue: "todo", NewValue: "in_progress"},
			}),
			makeTask(models.StateDone, timePtr(futureDone), []*models.ActivityEntry{
				{Timestamp: futureStart, Field: "state", OldValue: "todo", NewValue: "in_progress"},
			}),
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeCycleTime(tasks, cutoff, now, buckets)

		if result.Count != 1 {
			t.Errorf("Count = %d, want 1 (future completion excluded)", result.Count)
		}
		expectedHours := 48.0
		if actualHours := result.Average.Hours(); actualHours < expectedHours-1 || actualHours > expectedHours+1 {
			t.Errorf("Average = %v hours, want ~%v hours", actualHours, expectedHours)
		}
	})

	t.Run("skips nil tasks", func(t *testing.T) {
		start := now.AddDate(0, 0, -3)
		done := now.AddDate(0, 0, -1)
		tasks := []*models.Task{
			nil,
			makeTask(models.StateDone, timePtr(done), []*models.ActivityEntry{
				{Timestamp: start, Field: "state", OldValue: "todo", NewValue: "in_progress"},
			}),
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeCycleTime(tasks, cutoff, now, buckets)

		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
	})
}

func TestPercentileDuration(t *testing.T) {
	tests := []struct {
		name   string
		values []time.Duration
		p      float64
		want   time.Duration
	}{
		{
			name:   "empty",
			values: nil,
			p:      0.5,
			want:   0,
		},
		{
			name:   "single value",
			values: []time.Duration{4 * time.Hour},
			p:      0.9,
			want:   4 * time.Hour,
		},
		{
			name:   "median interpolates even sample",
			values: []time.Duration{2 * time.Hour, 6 * time.Hour},
			p:      0.5,
			want:   4 * time.Hour,
		},
		{
			name:   "p90 interpolates small sample",
			values: []time.Duration{time.Hour, 10 * time.Hour},
			p:      0.9,
			want:   time.Hour + time.Duration(0.9*float64(9*time.Hour)),
		},
		{
			name:   "p90 interpolates larger sample",
			values: []time.Duration{time.Hour, 2 * time.Hour, 10 * time.Hour},
			p:      0.9,
			want:   8*time.Hour + 24*time.Minute,
		},
		{
			name:   "clamps low percentile",
			values: []time.Duration{2 * time.Hour, 6 * time.Hour},
			p:      -1,
			want:   2 * time.Hour,
		},
		{
			name:   "clamps high percentile",
			values: []time.Duration{2 * time.Hour, 6 * time.Hour},
			p:      2,
			want:   6 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := percentileDuration(tt.values, tt.p); got != tt.want {
				t.Fatalf("percentileDuration(%v, %v) = %v, want %v", tt.values, tt.p, got, tt.want)
			}
		})
	}
}
