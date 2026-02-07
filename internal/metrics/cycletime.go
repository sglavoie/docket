package metrics

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// computeCycleTime calculates cycle time metrics for completed tasks.
// Cycle time = CompletedAt - first "in_progress" transition timestamp.
// Tasks without an in_progress transition in their activity log are skipped.
//
// Note: activity is pruned to 50 entries (oldest removed), so very old
// transitions may be lost for tasks with extensive histories.
func computeCycleTime(tasks []*models.Task, cutoff, now time.Time, buckets []bucket) CycleTimeResult {
	result := CycleTimeResult{
		Trend:  make([]float64, len(buckets)),
		Labels: bucketLabels(buckets),
	}

	// Collect cycle times per bucket for trend calculation
	bucketDurations := make([][]time.Duration, len(buckets))
	for i := range bucketDurations {
		bucketDurations[i] = []time.Duration{}
	}

	var allDurations []time.Duration

	for _, t := range tasks {
		if t.CompletedAt == nil || t.Status != models.StatusDone {
			continue
		}
		completedAt := *t.CompletedAt

		if !cutoff.IsZero() && completedAt.Before(cutoff) {
			continue
		}

		startTime := findFirstInProgress(t)
		if startTime.IsZero() {
			continue
		}

		cycleTime := completedAt.Sub(startTime)
		if cycleTime < 0 {
			continue
		}

		allDurations = append(allDurations, cycleTime)

		idx := assignToBucket(completedAt, buckets)
		if idx >= 0 {
			bucketDurations[idx] = append(bucketDurations[idx], cycleTime)
		}
	}

	result.Count = len(allDurations)
	if result.Count == 0 {
		return result
	}

	// Sort for median and percentile calculations
	sort.Slice(allDurations, func(i, j int) bool {
		return allDurations[i] < allDurations[j]
	})

	result.Average = averageDuration(allDurations)
	result.Median = percentileDuration(allDurations, 0.5)
	result.P90 = percentileDuration(allDurations, 0.9)

	// Compute per-bucket average in hours for trend
	for i, durations := range bucketDurations {
		if len(durations) > 0 {
			avg := averageDuration(durations)
			result.Trend[i] = avg.Hours()
		}
	}

	return result
}

// findFirstInProgress scans a task's activity log for the earliest
// transition to "in_progress" status.
func findFirstInProgress(t *models.Task) time.Time {
	for _, entry := range t.Activity {
		if entry.Field == "status" && entry.NewValue == string(models.StatusInProgress) {
			return entry.Timestamp
		}
	}
	return time.Time{}
}

// averageDuration returns the mean of a slice of durations.
func averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

// percentileDuration returns the value at the given percentile (0-1) from a sorted slice.
func percentileDuration(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
