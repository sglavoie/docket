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

// TestFormulaMultiplyZeroProductThenInfReturnsZero is a regression test:
// when the running product is already 0 (from an earlier zero-valued factor
// with Weight==1) and a later factor produces +Inf (e.g. Pow(0, -1)),
// the mathematical result is 0 * Inf = NaN, which should map to 0.
// Previously the Inf branch unconditionally returned MaxScoreSentinel,
// artificially boosting a zero-product task to the top of the ranking.
func TestFormulaMultiplyZeroProductThenInfReturnsZero(t *testing.T) {
	results := []DimensionResult{
		{Value: 0, Weight: 1},  // product *= 0 -> product = 0
		{Value: 0, Weight: -1}, // Pow(0, -1) = +Inf
	}
	got := formulaMultiply(results)
	if got != 0 {
		t.Errorf("zero-product * Inf should yield 0 (NaN-equivalent), got %f", got)
	}
}

// TestFormulaMultiplyZeroFromPowThenInfReturnsZero exercises the same
// undefined 0*Inf case where the zero arises from Pow(0, positive) rather
// than the Weight==1 shortcut.
func TestFormulaMultiplyZeroFromPowThenInfReturnsZero(t *testing.T) {
	results := []DimensionResult{
		{Value: 0, Weight: 2},  // Pow(0, 2) = 0 -> product = 0
		{Value: 0, Weight: -1}, // Pow(0, -1) = +Inf
	}
	got := formulaMultiply(results)
	if got != 0 {
		t.Errorf("zero-product * Inf (via Pow) should yield 0, got %f", got)
	}
}

// TestFormulaMultiplyInfThenZeroStillSentinel confirms the ordering matters:
// when Inf arises before any zero factor, the legacy "zero effort, infinite
// value" semantic is preserved (returns sentinel immediately).
func TestFormulaMultiplyInfThenZeroStillSentinel(t *testing.T) {
	results := []DimensionResult{
		{Value: 0, Weight: -1}, // Pow(0, -1) = +Inf, product was 1.0 -> sentinel
		{Value: 0, Weight: 1},  // never reached
	}
	got := formulaMultiply(results)
	if got != MaxScoreSentinel {
		t.Errorf("leading Inf should still yield sentinel, got %f", got)
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

// TestFormulaMultiplyEmptyResultsReturnsZero guards against the neutral
// multiplicative identity (1.0) silently boosting a malformed profile with no
// dimensions above legitimate zero-scoring tasks. Empty input should match
// formulaWeightedSum's behavior and return 0.
func TestFormulaMultiplyEmptyResultsReturnsZero(t *testing.T) {
	if got := formulaMultiply(nil); got != 0 {
		t.Errorf("empty multiply should yield 0, got %f", got)
	}
	if got := formulaMultiply([]DimensionResult{}); got != 0 {
		t.Errorf("empty slice multiply should yield 0, got %f", got)
	}
}

// TestFormulaRICEEmptyResultsReturnsZero mirrors the multiply fix: 1/1 = 1
// for an empty profile should not silently rank tasks above zero-scoring ones.
func TestFormulaRICEEmptyResultsReturnsZero(t *testing.T) {
	if got := formulaRICE(nil); got != 0 {
		t.Errorf("empty rice should yield 0, got %f", got)
	}
	if got := formulaRICE([]DimensionResult{}); got != 0 {
		t.Errorf("empty slice rice should yield 0, got %f", got)
	}
}

// TestScorePercentNaNAndInf ensures pathological raw scores never leak NaN or
// unclamped Inf into the normalized space (which would poison ranking and
// sort comparisons).
func TestScorePercentNaNAndInf(t *testing.T) {
	profile := NewICEProfile()
	if got := ScorePercent(math.NaN(), &profile); got != 0 {
		t.Errorf("NaN score should normalize to 0, got %f", got)
	}
	maxN := float64(MaxNormalizedScoreValue)
	if got := ScorePercent(math.Inf(1), &profile); got != maxN {
		t.Errorf("+Inf score should normalize to %f, got %f", maxN, got)
	}
	if got := ScorePercent(math.Inf(-1), &profile); got != 0 {
		t.Errorf("-Inf score should normalize to 0, got %f", got)
	}
}

// TestValidateRejectsRICEDegenerateDenominator guards against a denominator
// dimension with Max == 0 in a RICE profile. Such a dim multiplies the
// denominator product to 0, making formulaRICE always return MaxScoreSentinel
// for MinScore, MaxScore, and DefaultScore — which poisons ScorePercent
// (degenerate range, defS far above maxS) and silently makes every task
// score the same. Validation should reject this at profile-add time.
func TestValidateRejectsRICEDegenerateDenominator(t *testing.T) {
	profile := ScoringProfile{
		ID: "rice-bad", Name: "BadRICE", Formula: FormulaRICE,
		Dimensions: []ScoringDimension{
			{ID: "n", Name: "N", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "d", Name: "D", Min: 0, Max: 0, Default: 0, Weight: 1, Source: SourceManual, Role: RoleDenominator},
		},
	}
	if err := profile.Validate(); err == nil {
		t.Error("expected validation error for RICE denominator with Max=0, got nil")
	}
}

// TestValidateRejectsRICEAnyZeroMaxDenominator confirms that RICE rejects
// even a single zero-max denominator when there are multiple denominators,
// because the product is dominated by zero.
func TestValidateRejectsRICEAnyZeroMaxDenominator(t *testing.T) {
	profile := ScoringProfile{
		ID: "rice-multi", Name: "MultiDenRICE", Formula: FormulaRICE,
		Dimensions: []ScoringDimension{
			{ID: "n", Name: "N", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "d1", Name: "D1", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleDenominator},
			{ID: "d2", Name: "D2", Min: 0, Max: 0, Default: 0, Weight: 1, Source: SourceManual, Role: RoleDenominator},
		},
	}
	if err := profile.Validate(); err == nil {
		t.Error("expected validation error: RICE rejects any zero-max denominator (product * 0 = 0)")
	}
}

// TestValidateRejectsWSJFAllZeroMaxDenominators verifies WSJF rejects a
// profile where every denominator has Max == 0 (sum is always 0, formula
// returns MaxScoreSentinel). A mix with at least one nonzero-max denominator
// is allowed since the sum can still be > 0.
func TestValidateRejectsWSJFAllZeroMaxDenominators(t *testing.T) {
	allZero := ScoringProfile{
		ID: "wsjf-bad", Name: "BadWSJF", Formula: FormulaWSJF,
		Dimensions: []ScoringDimension{
			{ID: "n", Name: "N", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "d", Name: "D", Min: 0, Max: 0, Default: 0, Weight: 1, Source: SourceManual, Role: RoleDenominator},
		},
	}
	if err := allZero.Validate(); err == nil {
		t.Error("expected validation error: WSJF with all zero-max denominators")
	}
}

// TestValidateAcceptsRICEWithLegitimateZeroMinDenominator confirms the new
// validation does not over-reach: a denominator with Min=0 but Max>0 is
// allowed (user can opt to set it to 0 and get the "zero effort = top"
// semantic by intent, but the profile itself is still well-formed).
func TestValidateAcceptsRICEWithLegitimateZeroMinDenominator(t *testing.T) {
	profile := ScoringProfile{
		ID: "rice-ok", Name: "OKRICE", Formula: FormulaRICE,
		Dimensions: []ScoringDimension{
			{ID: "n", Name: "N", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "d", Name: "D", Min: 0, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleDenominator},
		},
	}
	if err := profile.Validate(); err != nil {
		t.Errorf("Min=0/Max>0 denominator should be valid, got: %v", err)
	}
}
