package metrics

import (
	"slices"
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

	// For TimeRangeAll the explicit cutoff is zero but the bucket range is
	// capped at the last ~12 months. Use the bucket span as the effective
	// cutoff so the overall Count/Average/Median match the per-bucket Trend.
	effectiveCutoff := cutoff
	if effectiveCutoff.IsZero() && len(buckets) > 0 {
		effectiveCutoff = buckets[0].Start
	}

	for _, t := range tasks {
		if t.CompletedAt == nil || t.Status != models.StatusDone {
			continue
		}
		completedAt := *t.CompletedAt

		if (!effectiveCutoff.IsZero() && completedAt.Before(effectiveCutoff)) || completedAt.After(now) {
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
	slices.Sort(allDurations)

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
	var earliest time.Time
	for _, entry := range t.Activity {
		if entry.Field == "status" && entry.NewValue == string(models.StatusInProgress) {
			if earliest.IsZero() || entry.Timestamp.Before(earliest) {
				earliest = entry.Timestamp
			}
		}
	}
	return earliest
}

// averageDuration returns the mean of a slice of durations.
//
// A naive sum-then-divide implementation overflows time.Duration (int64
// nanoseconds, ~292 years) for large sets of long-running tasks. We use a
// numerically stable incremental mean (Welford's update) in float64. The
// result is clamped to the int64 range before being converted back to
// time.Duration so callers always receive a well-defined value even for
// pathological inputs.
func averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var mean float64
	for i, d := range durations {
		// Incremental mean: m_n = m_{n-1} + (x_n - m_{n-1}) / n
		mean += (float64(d) - mean) / float64(i+1)
	}
	return durationFromFloat(mean)
}

// durationFromFloat converts a nanosecond count expressed as float64 to a
// time.Duration, clamping to the representable int64 range. This guards
// against overflow when accumulators legitimately exceed int64 ns (~292y).
func durationFromFloat(ns float64) time.Duration {
	const maxNs = float64(1<<63 - 1)
	const minNs = -maxNs - 1
	switch {
	case ns >= maxNs:
		return time.Duration(1<<63 - 1)
	case ns <= minNs:
		return time.Duration(-1 << 63)
	default:
		return time.Duration(ns)
	}
}

// percentileDuration returns the value at the given percentile (0-1) from a sorted slice.
func percentileDuration(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}

	pos := float64(len(sorted)-1) * p
	lower := int(pos)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	if lower == upper {
		return sorted[lower]
	}

	lowerNs := float64(sorted[lower])
	upperNs := float64(sorted[upper])
	frac := pos - float64(lower)
	return durationFromFloat(lowerNs + (upperNs-lowerNs)*frac)
}
