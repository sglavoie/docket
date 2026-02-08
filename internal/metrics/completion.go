package metrics

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// computeCompletion calculates completion rate metrics.
// "Resolved" includes both done and canceled tasks (both represent resolved work).
// Rate = done / (done + canceled), measuring how much resolved work was actually completed.
func computeCompletion(tasks []*models.Task, cutoff, now time.Time, buckets []bucket) CompletionResult {
	result := CompletionResult{
		Trend:  make([]float64, len(buckets)),
		Labels: bucketLabels(buckets),
	}

	// Track per-bucket totals for rate calculation
	bucketDone := make([]int, len(buckets))
	bucketResolved := make([]int, len(buckets))

	for _, t := range tasks {
		if t.CompletedAt == nil {
			continue
		}
		completedAt := *t.CompletedAt

		// Skip tasks completed before the cutoff (unless all-time)
		if !cutoff.IsZero() && completedAt.Before(cutoff) {
			continue
		}

		if !t.Status.IsComplete() {
			continue
		}

		result.Total++
		if t.Status == models.StatusDone {
			result.Completed++
		}

		idx := assignToBucket(completedAt, buckets)
		if idx >= 0 {
			bucketResolved[idx]++
			if t.Status == models.StatusDone {
				bucketDone[idx]++
			}
		}
	}

	// Compute overall rate
	if result.Total > 0 {
		result.Rate = float64(result.Completed) / float64(result.Total)
	}

	// Compute per-bucket rates
	for i := range buckets {
		if bucketResolved[i] > 0 {
			result.Trend[i] = float64(bucketDone[i]) / float64(bucketResolved[i])
		}
	}

	return result
}
