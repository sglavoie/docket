package planner

import (
	"fmt"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/metrics"
	"github.com/sglavoie/tt/internal/models"
)

// withChildren sets parent.Children to children and returns parent.
func withChildren(parent *models.Task, children ...*models.Task) *models.Task {
	parent.Children = children
	return parent
}

// withEffort sets TaskAttrs.Effort on a task and returns the task.
func withEffort(t *models.Task, e models.EffortEstimate) *models.Task {
	t.EnsureTaskAttrs().Effort = e
	return t
}

// withStoryPoints sets StoryAttrs.StoryPoints on a task and returns the task.
func withStoryPoints(t *models.Task, pts int) *models.Task {
	t.EnsureStoryAttrs().StoryPoints = pts
	return t
}

var refNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// TestForecastZeroVelocity verifies that zero velocity yields no forecast date
// but still populates remaining counts.
func TestForecastZeroVelocity(t *testing.T) {
	child := newTask("c1")
	parent := withChildren(newTask("parent"), child)

	result := ForecastCompletion(parent, 0, refNow)

	if result.RemainingCount != 1 {
		t.Errorf("expected RemainingCount=1, got %d", result.RemainingCount)
	}
	if !result.ForecastDate.IsZero() {
		t.Errorf("expected zero ForecastDate when velocity=0, got %v", result.ForecastDate)
	}
}

// TestForecastAllDone verifies zero remaining when all children are complete.
func TestForecastAllDone(t *testing.T) {
	c1 := withStatus(newTask("c1"), models.StatusDone)
	c2 := withStatus(newTask("c2"), models.StatusDone)
	parent := withChildren(newTask("parent"), c1, c2)

	result := ForecastCompletion(parent, 1.0, refNow)

	if result.RemainingCount != 0 {
		t.Errorf("expected RemainingCount=0, got %d", result.RemainingCount)
	}
	if result.RemainingWeightedDays != 0 {
		t.Errorf("expected RemainingWeightedDays=0, got %f", result.RemainingWeightedDays)
	}
	if !result.ForecastDate.IsZero() {
		t.Errorf("expected zero ForecastDate when nothing remaining, got %v", result.ForecastDate)
	}
}

// TestForecastConfidenceThresholds verifies the confidence levels.
func TestForecastConfidenceThresholds(t *testing.T) {
	cases := []struct {
		completed int
		want      Confidence
	}{
		{0, ConfidenceNone},
		{2, ConfidenceNone},
		{3, ConfidenceLow},
		{9, ConfidenceLow},
		{10, ConfidenceMedium},
		{29, ConfidenceMedium},
		{30, ConfidenceHigh},
		{100, ConfidenceHigh},
	}
	for _, tc := range cases {
		got := confidenceForCount(tc.completed)
		if got != tc.want {
			t.Errorf("confidenceForCount(%d) = %v, want %v", tc.completed, got, tc.want)
		}
	}
}

// TestForecastEffortWeighted verifies that effort-annotated children contribute
// their effort-mapped day counts to RemainingWeightedDays.
func TestForecastEffortWeighted(t *testing.T) {
	small := withEffort(newTask("small"), models.EffortSmall)    // 0.5 days
	large := withEffort(newTask("large"), models.EffortLarge)    // 3.0 days
	medium := withEffort(newTask("medium"), models.EffortMedium) // 1.0 days
	parent := withChildren(newTask("parent"), small, large, medium)

	result := ForecastCompletion(parent, 1.0, refNow)

	wantDays := effortDaysSmall + effortDaysLarge + effortDaysMedium // 0.5 + 3.0 + 1.0 = 4.5
	if result.RemainingWeightedDays != wantDays {
		t.Errorf("expected RemainingWeightedDays=%f, got %f", wantDays, result.RemainingWeightedDays)
	}
	// ForecastDate should be ~4.5 days from now.
	wantDate := refNow.Add(daysToduration(wantDays))
	if !result.ForecastDate.Equal(wantDate) {
		t.Errorf("expected ForecastDate=%v, got %v", wantDate, result.ForecastDate)
	}
}

// TestForecastStoryPoints verifies StoryPoints-based effort.
func TestForecastStoryPoints(t *testing.T) {
	child := withStoryPoints(newTask("story"), 4) // 4 * 0.5 = 2.0 days
	parent := withChildren(newTask("parent"), child)

	result := ForecastCompletion(parent, 1.0, refNow)

	if result.RemainingWeightedDays != 2.0 {
		t.Errorf("expected 2.0 days, got %f", result.RemainingWeightedDays)
	}
}

// TestForecastMixedEffortNoEffort verifies that tasks without effort data fall
// back to EstimatedDays (or 1.0) in the remaining computation.
func TestForecastMixedEffortNoEffort(t *testing.T) {
	withEffortTask := withEffort(newTask("e"), models.EffortMedium) // 1.0 day
	noEffortTask := newTaskWithDuration("ne", 2.0)                  // no effort, uses EstimatedDays

	parent := withChildren(newTask("parent"), withEffortTask, noEffortTask)
	result := ForecastCompletion(parent, 1.0, refNow)

	// 1.0 + 2.0 = 3.0
	if result.RemainingWeightedDays != 3.0 {
		t.Errorf("expected 3.0 days, got %f", result.RemainingWeightedDays)
	}
}

// TestWeightedVelocityNoEffortData verifies fall-back to AvgPerDay.
func TestWeightedVelocityNoEffortData(t *testing.T) {
	throughput := metrics.ThroughputResult{Total: 10, AvgPerDay: 2.0}
	tasks := []*models.Task{
		withStatus(newTask("t1"), models.StatusDone),
		withStatus(newTask("t2"), models.StatusDone),
	}
	got := WeightedVelocity(throughput, tasks)
	if got != 2.0 {
		t.Errorf("expected fallback to AvgPerDay=2.0, got %f", got)
	}
}

// TestWeightedVelocityZeroAvgPerDay returns zero (no data).
func TestWeightedVelocityZeroAvgPerDay(t *testing.T) {
	throughput := metrics.ThroughputResult{Total: 0, AvgPerDay: 0}
	got := WeightedVelocity(throughput, nil)
	if got != 0 {
		t.Errorf("expected 0, got %f", got)
	}
}

// TestWeightedVelocityEffortWeighted verifies effort-based scaling.
// If all completed tasks are "large" (3.0 days each) and throughput is 1 task/day,
// the weighted velocity should be 3.0 effort-days/day.
func TestWeightedVelocityEffortWeighted(t *testing.T) {
	throughput := metrics.ThroughputResult{Total: 10, AvgPerDay: 1.0}

	// Build 5 large completed tasks (3.0 effort-days each, avg = 3.0).
	tasks := make([]*models.Task, 5)
	for i := range tasks {
		tasks[i] = withStatus(
			withEffort(newTask(fmt.Sprintf("t%d", i)), models.EffortLarge),
			models.StatusDone,
		)
	}
	got := WeightedVelocity(throughput, tasks)

	// avg effort per task = 3.0; total effort = 10 * 3.0 = 30; days = 10/1.0 = 10
	// weighted velocity = 30/10 = 3.0
	if got != 3.0 {
		t.Errorf("expected weighted velocity 3.0, got %f", got)
	}
}

// TestWeightedVelocityMixedEffort verifies averaging over mixed effort levels.
func TestWeightedVelocityMixedEffort(t *testing.T) {
	throughput := metrics.ThroughputResult{Total: 2, AvgPerDay: 1.0}

	small := withStatus(withEffort(newTask("s"), models.EffortSmall), models.StatusDone) // 0.5
	large := withStatus(withEffort(newTask("l"), models.EffortLarge), models.StatusDone) // 3.0

	got := WeightedVelocity(throughput, []*models.Task{small, large})

	// avg effort = (0.5 + 3.0)/2 = 1.75; total effort = 2 * 1.75 = 3.5; days = 2/1.0 = 2
	// weighted velocity = 3.5/2 = 1.75
	const want = 1.75
	if abs(got-want) > 1e-9 {
		t.Errorf("expected weighted velocity %f, got %f", want, got)
	}
}

func TestWeightedVelocityWithSummariesIncludesArchivedEffort(t *testing.T) {
	throughput := metrics.ThroughputResult{Total: 2, AvgPerDay: 1.0}
	live := metrics.TaskSummary{
		ID:     "live",
		Status: models.StatusDone,
		Effort: models.EffortSmall,
	}
	archived := metrics.TaskSummary{
		ID:     "archived",
		Status: models.StatusDone,
		Effort: models.EffortLarge,
	}

	got := WeightedVelocityWithSummaries(throughput, []metrics.TaskSummary{live}, []metrics.TaskSummary{archived})

	const want = 1.75
	if abs(got-want) > 1e-9 {
		t.Errorf("expected weighted velocity %f, got %f", want, got)
	}
}

func TestWeightedVelocityPreservesIdeaEffortFallback(t *testing.T) {
	throughput := metrics.ThroughputResult{Total: 1, AvgPerDay: 1.0}
	task := withStatus(newTask("idea-velocity"), models.StatusDone)
	task.TaskAttrs = &models.TaskAttrs{}
	task.EnsureIdeaAttrs().Effort = models.EffortLarge

	got := WeightedVelocity(throughput, []*models.Task{task})

	if got != effortDaysLarge {
		t.Errorf("expected idea effort velocity %f, got %f", effortDaysLarge, got)
	}
}

// TestForecastNilParent verifies the function handles a nil parent without panicking.
func TestForecastNilParent(t *testing.T) {
	result := ForecastCompletion(nil, 1.0, refNow)
	if result == nil {
		t.Fatal("expected non-nil result for nil parent")
	}
	if result.RemainingCount != 0 || result.RemainingWeightedDays != 0 {
		t.Errorf("expected empty result for nil parent, got %+v", result)
	}
}

// TestForecastTinyVelocityNoOverflow verifies the forecast date stays sane
// when velocity is microscopically small (would otherwise overflow Duration).
func TestForecastTinyVelocityNoOverflow(t *testing.T) {
	child := newTask("c1")
	parent := withChildren(newTask("parent"), child)
	result := ForecastCompletion(parent, 1e-30, refNow)
	if result.ForecastDate.IsZero() {
		t.Error("expected non-zero ForecastDate even with tiny velocity")
	}
	// ForecastDate should be capped, not garbage (must be after refNow).
	if !result.ForecastDate.After(refNow) {
		t.Errorf("ForecastDate %v not after refNow %v", result.ForecastDate, refNow)
	}
}
