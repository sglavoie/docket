package scoring

import (
	"math"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestComputeScore_NilProfile guards against a nil profile pointer; previously
// ComputeScore dereferenced profile.Dimensions and crashed.
func TestComputeScore_NilProfile(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil profile: %v", r)
		}
	}()
	task := &models.Task{}
	if got := ComputeScore(task, nil); got != 0 {
		t.Errorf("expected 0 for nil profile, got %f", got)
	}
}

// TestComputeScore_NilTask should fall back to defaults rather than panicking
// (resolveDimension previously dereferenced task.ScoreDimensions unguarded).
func TestComputeScore_NilTask(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil task: %v", r)
		}
	}()
	profile := NewICEProfile()
	got := ComputeScore(nil, &profile)
	// ICE default product (5*5*5 = 125).
	if got != 125 {
		t.Errorf("expected default ICE score 125 for nil task, got %f", got)
	}
}

// TestScoreBreakdown_NilInputs returns a sane empty/default breakdown.
func TestScoreBreakdown_NilInputs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	results := ScoreBreakdown(nil, nil)
	if len(results) != 0 {
		t.Errorf("expected empty breakdown for nil inputs, got %d entries", len(results))
	}
}

// TestNormalizedScore_NilProfile must not panic and must return 0.
func TestNormalizedScore_NilProfile(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	task := &models.Task{}
	got := NormalizedScore(task, nil)
	if got != 0 {
		t.Errorf("expected 0 for nil profile, got %f", got)
	}
}

// TestScorePercent_NilProfile must not panic and must return 0.
func TestScorePercent_NilProfile(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	if got := ScorePercent(10, nil); got != 0 {
		t.Errorf("expected 0 for nil profile, got %f", got)
	}
}

// TestMaxScoreNilProfile / MinScore / DefaultScore handle nil safely.
func TestMaxScoreNilProfile(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	if got := MaxScore(nil); got != 0 {
		t.Errorf("expected 0 for nil profile MaxScore, got %f", got)
	}
	if got := MinScore(nil); got != 0 {
		t.Errorf("expected 0 for nil profile MinScore, got %f", got)
	}
	if got := DefaultScore(nil); got != 0 {
		t.Errorf("expected 0 for nil profile DefaultScore, got %f", got)
	}
}

// TestFormulaWSJFAllZeroDenominatorWeights verifies the denominator guard
// catches the case where the sum of denominator (value * weight) is zero,
// even when values are non-zero but weights are zero. Without this guard,
// dividing by zero would silently produce Inf.
func TestFormulaWSJFAllZeroDenominatorWeights(t *testing.T) {
	results := []DimensionResult{
		{Value: 5, Weight: 1, Role: RoleNumerator},
		{Value: 5, Weight: 0, Role: RoleDenominator},
	}
	got := formulaWSJF(results)
	if got != MaxScoreSentinel {
		t.Errorf("expected MaxScoreSentinel (%f) when denominator sum is 0, got %f", MaxScoreSentinel, got)
	}
}

// TestFormulaMultiplyZeroWeight ensures a dimension with weight 0 contributes
// a neutral factor (Pow(value, 0) = 1) without triggering the Inf/NaN guard.
func TestFormulaMultiplyZeroWeight(t *testing.T) {
	results := []DimensionResult{
		{Value: 5, Weight: 0},
		{Value: 4, Weight: 1},
	}
	got := formulaMultiply(results)
	if math.Abs(got-4) > 1e-9 {
		t.Errorf("expected 4 (weight-0 dim contributes 1), got %f", got)
	}
}
