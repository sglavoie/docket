package metrics

import (
	"fmt"
	"time"
)

// granularity represents the size of time buckets.
type granularity int

const (
	granularityDay granularity = iota
	granularityWeek
	granularityMonth
)

// bucketGranularity returns the appropriate granularity for a time range.
func bucketGranularity(tr TimeRange) granularity {
	switch tr {
	case TimeRange7d, TimeRange30d:
		return granularityDay
	case TimeRange90d:
		return granularityWeek
	default:
		return granularityMonth
	}
}

// bucket represents a single time bucket with start (inclusive) and end (exclusive).
type bucket struct {
	Start time.Time
	End   time.Time
	Label string
}

// timeBuckets generates time buckets from cutoff to now at the given granularity.
// For TimeRangeAll (zero cutoff), it uses the earliest reasonable start.
func timeBuckets(cutoff, now time.Time, g granularity) []bucket {
	if cutoff.IsZero() {
		// For "all time", start 12 months back as a reasonable default
		cutoff = now.AddDate(-1, 0, 0)
	}

	var buckets []bucket
	current := startOfPeriod(cutoff, g)

	for current.Before(now) {
		next := advancePeriod(current, g)
		label := bucketLabel(current, g)
		buckets = append(buckets, bucket{
			Start: current,
			End:   next,
			Label: label,
		})
		current = next
	}

	if len(buckets) == 0 {
		buckets = append(buckets, bucket{
			Start: cutoff,
			End:   now,
			Label: bucketLabel(cutoff, g),
		})
	}

	return buckets
}

// bucketLabels extracts labels from a slice of buckets.
func bucketLabels(buckets []bucket) []string {
	labels := make([]string, len(buckets))
	for i, b := range buckets {
		labels[i] = b.Label
	}
	return labels
}

// assignToBucket returns the bucket index for the given timestamp, or -1 if out of range.
func assignToBucket(t time.Time, buckets []bucket) int {
	if len(buckets) == 0 {
		return -1
	}

	// Binary search for the bucket containing t
	lo, hi := 0, len(buckets)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if t.Before(buckets[mid].Start) {
			hi = mid - 1
		} else if !t.Before(buckets[mid].End) {
			lo = mid + 1
		} else {
			return mid
		}
	}
	return -1
}

// startOfPeriod truncates a time to the start of its period.
func startOfPeriod(t time.Time, g granularity) time.Time {
	switch g {
	case granularityDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case granularityWeek:
		// Truncate to Monday
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		d := t.AddDate(0, 0, -(weekday - 1))
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, t.Location())
	case granularityMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return t
	}
}

// advancePeriod moves to the next period boundary.
func advancePeriod(t time.Time, g granularity) time.Time {
	switch g {
	case granularityDay:
		return t.AddDate(0, 0, 1)
	case granularityWeek:
		return t.AddDate(0, 0, 7)
	case granularityMonth:
		return t.AddDate(0, 1, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}

// bucketLabel returns a short label for the given period start.
func bucketLabel(t time.Time, g granularity) string {
	switch g {
	case granularityDay:
		return fmt.Sprintf("%02d/%02d", t.Month(), t.Day())
	case granularityWeek:
		return fmt.Sprintf("W%02d/%02d", t.Month(), t.Day())
	case granularityMonth:
		return t.Format("Jan")
	default:
		return t.Format("01/02")
	}
}
