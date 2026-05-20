package planner

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestEstimateDuration_ExplicitOverride(t *testing.T) {
	task := &models.Task{EstimatedDays: 4.0}
	if got := EstimateDuration(task); got != 4.0 {
		t.Errorf("got %v, want 4.0", got)
	}
}

func TestEstimateDuration_TaskEffort_Small(t *testing.T) {
	task := &models.Task{TaskAttrs: &models.TaskAttrs{Effort: models.EffortSmall}}
	if got := EstimateDuration(task); got != 0.5 {
		t.Errorf("got %v, want 0.5", got)
	}
}

func TestEstimateDuration_TaskEffort_Medium(t *testing.T) {
	task := &models.Task{TaskAttrs: &models.TaskAttrs{Effort: models.EffortMedium}}
	if got := EstimateDuration(task); got != 1.0 {
		t.Errorf("got %v, want 1.0", got)
	}
}

func TestEstimateDuration_TaskEffort_Large(t *testing.T) {
	task := &models.Task{TaskAttrs: &models.TaskAttrs{Effort: models.EffortLarge}}
	if got := EstimateDuration(task); got != 3.0 {
		t.Errorf("got %v, want 3.0", got)
	}
}

func TestEstimateDuration_TaskEffort_Unknown(t *testing.T) {
	task := &models.Task{TaskAttrs: &models.TaskAttrs{Effort: models.EffortUnknown}}
	if got := EstimateDuration(task); got != 1.0 {
		t.Errorf("got %v, want 1.0", got)
	}
}

func TestEstimateDuration_IdeaEffort_Large(t *testing.T) {
	task := &models.Task{IdeaAttrs: &models.IdeaAttrs{Effort: models.EffortLarge}}
	if got := EstimateDuration(task); got != 3.0 {
		t.Errorf("got %v, want 3.0", got)
	}
}

func TestEstimateDuration_StoryPoints(t *testing.T) {
	task := &models.Task{StoryAttrs: &models.StoryAttrs{StoryPoints: 6}}
	if got := EstimateDuration(task); got != 3.0 {
		t.Errorf("got %v, want 3.0 (6 points * 0.5)", got)
	}
}

func TestEstimateDuration_StoryPoints_One(t *testing.T) {
	task := &models.Task{StoryAttrs: &models.StoryAttrs{StoryPoints: 1}}
	if got := EstimateDuration(task); got != 0.5 {
		t.Errorf("got %v, want 0.5 (1 point * 0.5)", got)
	}
}

func TestEstimateDuration_Default_Fallback(t *testing.T) {
	task := &models.Task{}
	if got := EstimateDuration(task); got != 1.0 {
		t.Errorf("got %v, want 1.0 (default)", got)
	}
}

func TestEstimateDuration_ExplicitOverrides_Effort(t *testing.T) {
	// EstimatedDays takes priority over TaskAttrs.Effort
	task := &models.Task{
		EstimatedDays: 7.0,
		TaskAttrs:     &models.TaskAttrs{Effort: models.EffortSmall},
	}
	if got := EstimateDuration(task); got != 7.0 {
		t.Errorf("got %v, want 7.0 (EstimatedDays overrides effort)", got)
	}
}

func TestEstimateDuration_StoryZeroPoints_DefaultFallback(t *testing.T) {
	// StoryPoints == 0 should fall through to default
	task := &models.Task{StoryAttrs: &models.StoryAttrs{StoryPoints: 0}}
	if got := EstimateDuration(task); got != 1.0 {
		t.Errorf("got %v, want 1.0 (zero story points falls through)", got)
	}
}

func TestTotalRemainingDays_SumsIncompleteChildren(t *testing.T) {
	parent := &models.Task{
		Children: []*models.Task{
			{Status: models.StatusTodo, TaskAttrs: &models.TaskAttrs{Effort: models.EffortSmall}},       // 0.5
			{Status: models.StatusInProgress, TaskAttrs: &models.TaskAttrs{Effort: models.EffortLarge}}, // 3.0
			{Status: models.StatusDone}, // skip
		},
	}
	if got := TotalRemainingDays(parent); got != 3.5 {
		t.Errorf("got %v, want 3.5", got)
	}
}

func TestTotalRemainingDays_AllDone(t *testing.T) {
	parent := &models.Task{
		Children: []*models.Task{
			{Status: models.StatusDone},
			{Status: models.StatusDone},
		},
	}
	if got := TotalRemainingDays(parent); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestTotalRemainingDays_NoChildren(t *testing.T) {
	parent := &models.Task{Children: []*models.Task{}}
	if got := TotalRemainingDays(parent); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// TestTotalRemainingDays_NilParent guards against nil-deref when called with
// nil. Previously the helper panicked accessing parent.Children.
func TestTotalRemainingDays_NilParent(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil parent: %v", r)
		}
	}()
	if got := TotalRemainingDays(nil); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// TestTotalRemainingDays_NilChild skips nil children rather than panicking.
func TestTotalRemainingDays_NilChild(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil child: %v", r)
		}
	}()
	child := newTaskWithDuration("c", 2)
	parent := &models.Task{Children: []*models.Task{nil, child, nil}}
	if got := TotalRemainingDays(parent); got != 2 {
		t.Errorf("got %v, want 2", got)
	}
}

// TestEstimateDuration_NilTask returns the default duration rather than panicking.
func TestEstimateDuration_NilTask(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil task: %v", r)
		}
	}()
	if got := EstimateDuration(nil); got != 1.0 {
		t.Errorf("got %v, want 1.0", got)
	}
}
