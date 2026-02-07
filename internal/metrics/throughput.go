package metrics

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// computeThroughput calculates throughput metrics (done tasks only).
// Canceled tasks are excluded since they don't represent delivered work.
func computeThroughput(tasks []*models.Task, cutoff, now time.Time, buckets []bucket) ThroughputResult {
	result := ThroughputResult{
		PerBucket: make([]float64, len(buckets)),
		Labels:    bucketLabels(buckets),
	}

	for _, t := range tasks {
		if t.CompletedAt == nil || t.Status != models.StatusDone {
			continue
		}
		completedAt := *t.CompletedAt

		if !cutoff.IsZero() && completedAt.Before(cutoff) {
			continue
		}

		result.Total++

		idx := assignToBucket(completedAt, buckets)
		if idx >= 0 {
			result.PerBucket[idx]++
		}
	}

	// Compute average per day
	var days float64
	if cutoff.IsZero() {
		// For all-time, use the span from earliest bucket to now
		if len(buckets) > 0 {
			days = now.Sub(buckets[0].Start).Hours() / 24
		}
	} else {
		days = now.Sub(cutoff).Hours() / 24
	}
	if days > 0 {
		result.AvgPerDay = float64(result.Total) / days
	}

	return result
}
