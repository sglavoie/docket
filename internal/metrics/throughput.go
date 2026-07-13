package metrics

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// computeThroughput calculates throughput metrics (done tasks only).
// Canceled tasks are excluded since they don't represent delivered work.
func computeThroughput(tasks []*models.Task, cutoff, now time.Time, buckets []bucket) ThroughputResult {
	return computeThroughputSummaries(TaskSummariesFromTasks(tasks), cutoff, now, buckets)
}

func computeThroughputSummaries(tasks []TaskSummary, cutoff, now time.Time, buckets []bucket) ThroughputResult {
	result := ThroughputResult{
		PerBucket: make([]float64, len(buckets)),
		Labels:    bucketLabels(buckets),
	}

	// For TimeRangeAll the explicit cutoff is zero but the bucket range is
	// capped at the last ~12 months. Use the bucket span as the effective
	// cutoff so Total, PerBucket, and AvgPerDay reference the same window
	// — otherwise AvgPerDay divides an all-time total by ~365 days, vastly
	// overstating recent throughput.
	effectiveCutoff := cutoff
	if effectiveCutoff.IsZero() && len(buckets) > 0 {
		effectiveCutoff = buckets[0].Start
	}

	for _, t := range tasks {
		if t.CompletedAt == nil || t.WorkflowState != models.StateDone {
			continue
		}
		completedAt := *t.CompletedAt

		if (!effectiveCutoff.IsZero() && completedAt.Before(effectiveCutoff)) || completedAt.After(now) {
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
	} else if calendarDays, ok := wallClockAlignedCalendarDays(cutoff, now); ok {
		days = calendarDays
	} else {
		days = now.Sub(cutoff).Hours() / 24
	}
	if days > 0 {
		result.AvgPerDay = float64(result.Total) / days
	}

	return result
}

func wallClockAlignedCalendarDays(start, end time.Time) (float64, bool) {
	end = end.In(start.Location())
	sh, sm, ss := start.Clock()
	eh, em, es := end.Clock()
	if sh != eh || sm != em || ss != es || start.Nanosecond() != end.Nanosecond() {
		return 0, false
	}

	sy, sMonth, sd := start.Date()
	ey, eMonth, ed := end.Date()
	startDate := time.Date(sy, sMonth, sd, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(ey, eMonth, ed, 0, 0, 0, 0, time.UTC)
	days := endDate.Sub(startDate).Hours() / 24
	return days, days > 0
}
