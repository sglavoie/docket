package metrics

import (
	"math"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestAverageDurationLargeSetDoesNotOverflow constructs a pathological
// input whose naive sum would overflow time.Duration (int64 ns, ~292y),
// and asserts the running-mean implementation returns the expected
// per-element value without saturating.
func TestAverageDurationLargeSetDoesNotOverflow(t *testing.T) {
	// Each duration is 100 years; 1,000 of them sum to 100,000 years,
	// which is far beyond time.Duration's ~292-year ceiling. The
	// average, however, must remain exactly 100 years.
	const oneYear = 365 * 24 * time.Hour
	per := 100 * oneYear
	durations := make([]time.Duration, 1000)
	for i := range durations {
		durations[i] = per
	}

	got := averageDuration(durations)

	// Allow a tiny relative tolerance for float64 round-off.
	want := float64(per)
	diff := math.Abs(float64(got) - want)
	if diff/want > 1e-9 {
		t.Fatalf("averageDuration overflowed or lost precision: got %v, want %v (rel diff %g)",
			got, per, diff/want)
	}
}

// TestAverageDurationMixedLargeAndSmall ensures the incremental mean
// remains accurate when very large and very small durations are mixed,
// which would have produced wildly wrong results under the naive
// implementation if the running sum saturated mid-stream.
func TestAverageDurationMixedLargeAndSmall(t *testing.T) {
	const oneYear = 365 * 24 * time.Hour
	durations := []time.Duration{
		200 * oneYear,
		200 * oneYear,
		200 * oneYear, // sum here already > 292y
		200 * oneYear,
	}
	got := averageDuration(durations)
	want := 200 * oneYear
	if d := got - want; d < -time.Second || d > time.Second {
		t.Fatalf("averageDuration = %v, want %v", got, want)
	}
}

// TestCycleTimeManyTasksDoesNotOverflow exercises the full
// computeCycleTime path with enough long-cycle tasks that the naive sum
// would overflow. We assert the average remains finite and close to the
// per-task cycle time.
func TestCycleTimeManyTasksDoesNotOverflow(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	// 500 tasks, each with a 20-year cycle time => naive sum = 10,000y
	// which overflows time.Duration.
	const cycle = 20 * 365 * 24 * time.Hour
	tasks := make([]*models.Task, 500)
	for i := range tasks {
		done := now.AddDate(0, 0, -1)
		start := done.Add(-cycle)
		tasks[i] = makeTask(models.StateDone, timePtr(done), []*models.ActivityEntry{
			{Timestamp: start, Field: "state", OldValue: "todo", NewValue: "in_progress"},
		})
	}

	// Use the all-time range so no task is filtered out.
	cutoff := TimeRangeAll.Cutoff(now)
	buckets := timeBuckets(cutoff, now, bucketGranularity(TimeRangeAll))
	result := computeCycleTime(tasks, cutoff, now, buckets)

	if result.Count != len(tasks) {
		t.Fatalf("Count = %d, want %d", result.Count, len(tasks))
	}
	if result.Average <= 0 {
		t.Fatalf("Average must be positive, got %v", result.Average)
	}
	// Should be close to the per-task cycle time.
	wantHours := cycle.Hours()
	gotHours := result.Average.Hours()
	if math.Abs(gotHours-wantHours)/wantHours > 1e-6 {
		t.Fatalf("Average = %v hours, want ~%v hours", gotHours, wantHours)
	}
}

// TestDurationFromFloatSaturates verifies the float->Duration clamp
// returns the max int64 value rather than wrapping around.
func TestDurationFromFloatSaturates(t *testing.T) {
	huge := float64(2) * float64(time.Duration(1<<62))
	got := durationFromFloat(huge)
	if got != time.Duration(1<<63-1) {
		t.Errorf("durationFromFloat(huge) = %v, want MaxInt64", got)
	}

	negHuge := -huge
	got = durationFromFloat(negHuge)
	if got != time.Duration(-1<<63) {
		t.Errorf("durationFromFloat(-huge) = %v, want MinInt64", got)
	}
}
