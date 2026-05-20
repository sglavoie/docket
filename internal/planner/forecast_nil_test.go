package planner

import (
	"testing"

	"github.com/sglavoie/tt/internal/metrics"
	"github.com/sglavoie/tt/internal/models"
)

// TestWeightedVelocity_NilTaskEntries guards against nil entries in the task
// slice. Previously WeightedVelocity dereferenced t unconditionally and
// panicked when callers passed a loosely-built slice.
func TestWeightedVelocity_NilTaskEntries(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	throughput := metrics.ThroughputResult{Total: 1, AvgPerDay: 1.0}
	completed := withStatus(withEffort(newTask("c"), models.EffortMedium), models.StatusDone)

	_ = WeightedVelocity(throughput, []*models.Task{nil, completed, nil})
}

// TestChildEffortDays_NilTask returns the default day estimate rather than
// panicking on a nil receiver.
func TestChildEffortDays_NilTask(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil task: %v", r)
		}
	}()
	days, hasEffort := childEffortDays(nil)
	if hasEffort {
		t.Errorf("expected hasEffortData=false for nil task")
	}
	if days != effortDaysUnknown {
		t.Errorf("expected default %v days for nil task, got %v", effortDaysUnknown, days)
	}
}

// TestForecastCompletion_NilChildSkipped ensures nil children in the parent's
// Children slice do not panic the forecast.
func TestForecastCompletion_NilChildSkipped(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil child: %v", r)
		}
	}()
	child := withEffort(newTask("c"), models.EffortMedium)
	parent := withChildren(newTask("parent"), nil, child, nil)
	result := ForecastCompletion(parent, 1.0, refNow)
	if result.RemainingCount != 1 {
		t.Errorf("expected RemainingCount=1 (nil children skipped), got %d", result.RemainingCount)
	}
	if result.RemainingWeightedDays != effortDaysMedium {
		t.Errorf("expected %v days, got %v", effortDaysMedium, result.RemainingWeightedDays)
	}
}
