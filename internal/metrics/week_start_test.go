package metrics

import (
	"testing"
	"time"
)

// TestWeekStartFromWeekEnd pins the mapping between the WeekEndDay config
// abbreviation and the time.Weekday used for weekly bucketing.
func TestWeekStartFromWeekEnd(t *testing.T) {
	cases := []struct {
		in   string
		want time.Weekday
	}{
		{"fri", time.Saturday},
		{"sat", time.Sunday},
		{"sun", time.Monday},
		{"", DefaultWeekStart},
		{"bogus", DefaultWeekStart},
	}
	for _, c := range cases {
		if got := WeekStartFromWeekEnd(c.in); got != c.want {
			t.Errorf("WeekStartFromWeekEnd(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestStartOfPeriodWeeklyHonorsWeekStart verifies that startOfPeriod for
// granularityWeek truncates to the configured week-start day rather than
// always Monday.
func TestStartOfPeriodWeeklyHonorsWeekStart(t *testing.T) {
	// Wednesday, 2025-06-11.
	wed := time.Date(2025, 6, 11, 10, 30, 0, 0, time.UTC)

	cases := []struct {
		weekStart time.Weekday
		want      time.Time
	}{
		// Monday before Wed 2025-06-11 is 2025-06-09.
		{time.Monday, time.Date(2025, 6, 9, 0, 0, 0, 0, time.UTC)},
		// Sunday before Wed 2025-06-11 is 2025-06-08.
		{time.Sunday, time.Date(2025, 6, 8, 0, 0, 0, 0, time.UTC)},
		// Saturday before Wed 2025-06-11 is 2025-06-07.
		{time.Saturday, time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)},
		// Wed itself returns same day at midnight (diff = 0).
		{time.Wednesday, time.Date(2025, 6, 11, 0, 0, 0, 0, time.UTC)},
		// Thursday is 6 days back to the most recent Thursday (2025-06-05).
		{time.Thursday, time.Date(2025, 6, 5, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		got := startOfPeriod(wed, granularityWeek, c.weekStart)
		if !got.Equal(c.want) {
			t.Errorf("startOfPeriod(weekStart=%v) = %v, want %v", c.weekStart, got, c.want)
		}
	}
}

// TestTimeBucketsWithWeekStartNonMonday pins bucketing behavior for a
// non-default week start (Sunday). All bucket Start values must fall on
// the requested weekday.
func TestTimeBucketsWithWeekStartNonMonday(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC) // a Sunday
	cutoff := now.AddDate(0, 0, -90)

	buckets := timeBucketsWithWeekStart(cutoff, now, granularityWeek, time.Sunday)
	if len(buckets) == 0 {
		t.Fatal("expected weekly buckets, got none")
	}
	for i, b := range buckets {
		if b.Start.Weekday() != time.Sunday {
			t.Errorf("bucket %d Start weekday = %v, want Sunday", i, b.Start.Weekday())
		}
		if got := b.End.Sub(b.Start); got != 7*24*time.Hour {
			t.Errorf("bucket %d span = %v, want 7 days", i, got)
		}
	}
}

// TestComputeAllAtWithWeekStartUsesProvidedDay routes through the public
// API and asserts the configured week-start propagates to the labels we
// observe via the per-bucket labels list.
func TestComputeAllAtWithWeekStartUsesProvidedDay(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC) // Sunday
	result := ComputeAllAtWithWeekStart(nil, TimeRange90d, now, time.Sunday)
	if result == nil {
		t.Fatal("nil result")
	}
	// TimeRange90d uses granularityWeek; cross-check via timeBuckets.
	expected := timeBucketsWithWeekStart(TimeRange90d.Cutoff(now), now, granularityWeek, time.Sunday)
	if len(result.Throughput.Labels) != len(expected) {
		t.Fatalf("Labels len = %d, want %d", len(result.Throughput.Labels), len(expected))
	}
	for i, b := range expected {
		if result.Throughput.Labels[i] != b.Label {
			t.Errorf("Labels[%d] = %q, want %q", i, result.Throughput.Labels[i], b.Label)
		}
	}
}

// TestComputeAllAtDefaultsToMonday confirms the existing API surface
// (ComputeAllAt without a week-start argument) keeps Monday as the
// week-start so existing callers see no behavior change.
func TestComputeAllAtDefaultsToMonday(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	result := ComputeAllAt(nil, TimeRange90d, now)
	expected := timeBucketsWithWeekStart(TimeRange90d.Cutoff(now), now, granularityWeek, time.Monday)
	if len(result.Throughput.Labels) != len(expected) {
		t.Fatalf("Labels len = %d, want %d", len(result.Throughput.Labels), len(expected))
	}
	for i, b := range expected {
		if result.Throughput.Labels[i] != b.Label {
			t.Errorf("Labels[%d] = %q, want %q", i, result.Throughput.Labels[i], b.Label)
		}
	}
}
