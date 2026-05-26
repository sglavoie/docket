package metrics

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

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
			makeTask(models.StatusTodo, nil, nil),
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

	t.Run("excludes tasks completed after now", func(t *testing.T) {
		recent := now.AddDate(0, 0, -1)
		futureDone := now.Add(2 * time.Hour)
		futureCanceled := now.Add(4 * time.Hour)

		tasks := []*models.Task{
			makeTask(models.StatusDone, timePtr(recent), nil),
			makeTask(models.StatusDone, timePtr(futureDone), nil),
			makeTask(models.StatusCanceled, timePtr(futureCanceled), nil),
		}

		cutoff := TimeRange7d.Cutoff(now)
		buckets := timeBuckets(cutoff, now, granularityDay)
		result := computeCompletion(tasks, cutoff, now, buckets)

		if result.Total != 1 {
			t.Errorf("Total = %d, want 1 (future completions excluded)", result.Total)
		}
		if result.Completed != 1 {
			t.Errorf("Completed = %d, want 1 (future completions excluded)", result.Completed)
		}
		if result.Rate != 1 {
			t.Errorf("Rate = %f, want 1", result.Rate)
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
