package scoring

import (
	"math"
	"testing"
)

// TestFormulaMultiplyNaNReturnsZero is a regression test: previously,
// math.Pow(negative, fractional) returned NaN and formulaMultiply mapped that
// to MaxScoreSentinel, artificially sorting an undefined-score task to the
// top of the ranking. NaN should map to 0 instead so the task doesn't get a
// nonsense boost.
func TestFormulaMultiplyNaNReturnsZero(t *testing.T) {
	results := []DimensionResult{
		{Value: -2, Weight: 0.5}, // Pow(-2, 0.5) = NaN
		{Value: 5, Weight: 1},
	}
	got := formulaMultiply(results)
	if got != 0 {
		t.Errorf("NaN multiplier should yield 0, got %f", got)
	}
}

// TestFormulaMultiplyInfStillSentinel ensures Pow producing +Inf (e.g. from
// Pow(0, -1)) keeps the MaxScoreSentinel semantic — "zero effort, infinite
// value" — consistent with the RICE divide-by-zero treatment.
func TestFormulaMultiplyInfReturnsSentinel(t *testing.T) {
	results := []DimensionResult{
		{Value: 0, Weight: -1}, // Pow(0, -1) = +Inf
	}
	got := formulaMultiply(results)
	if got != MaxScoreSentinel {
		t.Errorf("Inf multiplier should yield MaxScoreSentinel (%f), got %f", MaxScoreSentinel, got)
	}
}

// TestFormulaMultiplyNegativeBaseIntegerExponent verifies that a negative
// value with an integer weight (legal Pow result) is preserved verbatim.
func TestFormulaMultiplyNegativeBaseIntegerExponent(t *testing.T) {
	results := []DimensionResult{
		{Value: -3, Weight: 2}, // Pow(-3, 2) = 9
		{Value: 2, Weight: 1},
	}
	got := formulaMultiply(results)
	if math.Abs(got-18) > 1e-9 {
		t.Errorf("expected 18, got %f", got)
	}
}

// TestFormulaRICEZeroOverZeroReturnsZero is a regression test: 0/0 is
// mathematically undefined and previously returned MaxScoreSentinel,
// artificially boosting a "no reach AND no effort" task to the very top.
// 0 numerator with 0 denominator should return 0 (no value).
func TestFormulaRICEZeroOverZeroReturnsZero(t *testing.T) {
	results := []DimensionResult{
		{Value: 0, Weight: 1, Role: RoleNumerator},
		{Value: 0, Weight: 1, Role: RoleDenominator},
	}
	got := formulaRICE(results)
	if got != 0 {
		t.Errorf("0/0 should yield 0, got %f", got)
	}
}

// TestFormulaRICEPositiveOverZeroStillSentinel preserves the existing semantic
// for genuinely zero-effort items (positive value, zero cost ⇒ top of list).
func TestFormulaRICEPositiveOverZeroStillSentinel(t *testing.T) {
	results := []DimensionResult{
		{Value: 5, Weight: 1, Role: RoleNumerator},
		{Value: 0, Weight: 1, Role: RoleDenominator},
	}
	got := formulaRICE(results)
	if got != MaxScoreSentinel {
		t.Errorf("positive/0 should yield MaxScoreSentinel (%f), got %f", MaxScoreSentinel, got)
	}
}

// TestFormulaWSJFZeroOverZeroReturnsZero mirrors the RICE 0/0 fix: an item
// whose value and cost are both zero is not the top of the queue.
func TestFormulaWSJFZeroOverZeroReturnsZero(t *testing.T) {
	results := []DimensionResult{
		{Value: 0, Weight: 1, Role: RoleNumerator},
		{Value: 0, Weight: 1, Role: RoleDenominator},
	}
	got := formulaWSJF(results)
	if got != 0 {
		t.Errorf("0/0 should yield 0, got %f", got)
	}
}
