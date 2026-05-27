package metrics

import (
	"testing"
	"time"
)

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

		if len(buckets) < 7 || len(buckets) > 8 {
			t.Errorf("expected 7-8 daily buckets, got %d", len(buckets))
		}

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

		if len(buckets) < 12 || len(buckets) > 14 {
			t.Errorf("expected 12-14 weekly buckets, got %d", len(buckets))
		}
	})

	t.Run("monthly buckets for all-time", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		buckets := timeBuckets(time.Time{}, now, granularityMonth)

		if len(buckets) < 12 || len(buckets) > 13 {
			t.Errorf("expected 12-13 monthly buckets, got %d", len(buckets))
		}
	})

	t.Run("assignToBucket", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		cutoff := now.AddDate(0, 0, -7)
		buckets := timeBuckets(cutoff, now, granularityDay)

		mid := now.AddDate(0, 0, -3)
		idx := assignToBucket(mid, buckets)
		if idx < 0 {
			t.Errorf("expected valid bucket index for %v, got %d", mid, idx)
		}

		before := cutoff.AddDate(0, 0, -5)
		idx = assignToBucket(before, buckets)
		if idx != -1 {
			t.Errorf("expected -1 for out-of-range timestamp, got %d", idx)
		}
	})

	// Guards a regression where a timestamp exactly equal to the last
	// bucket's End (which equals "now" whenever "now" sits on the period
	// boundary, e.g. midnight for daily buckets) was being silently
	// dropped from per-bucket trends while still counted in totals.
	t.Run("assignToBucket includes timestamp on final boundary", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC) // exactly midnight
		cutoff := now.AddDate(0, 0, -3)
		buckets := timeBuckets(cutoff, now, granularityDay)
		if len(buckets) == 0 {
			t.Fatal("expected at least one bucket")
		}
		last := len(buckets) - 1
		if !buckets[last].End.Equal(now) {
			t.Fatalf("setup precondition: last bucket End %v should equal now %v", buckets[last].End, now)
		}
		got := assignToBucket(now, buckets)
		if got != last {
			t.Errorf("assignToBucket(now) = %d, want %d (final bucket)", got, last)
		}
	})
}
