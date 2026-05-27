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

// TestValidateRejectsNaNWeight guards against non-finite weights leaking into
// formulas. A NaN weight propagates through formulaWeightedSum to produce
// a NaN raw score, which then poisons sort comparisons and breakdown display.
// Validation must reject NaN at the configuration boundary.
func TestValidateRejectsNaNWeight(t *testing.T) {
	d := ScoringDimension{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: math.NaN(), Source: SourceManual}
	if err := d.Validate(); err == nil {
		t.Error("expected dimension validation to reject NaN weight, got nil")
	}
	profile := ScoringProfile{
		ID: "p", Name: "P", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{d},
	}
	if err := profile.Validate(); err == nil {
		t.Error("expected profile validation to reject NaN weight, got nil")
	}
}

// TestValidateRejectsInfWeight: +Inf weight in weighted_sum yields +Inf raw
// score; -Inf produces -Inf. Both poison ranking/sorting. Inf weights are
// not a meaningful configuration and must be rejected.
func TestValidateRejectsInfWeight(t *testing.T) {
	for _, w := range []float64{math.Inf(1), math.Inf(-1)} {
		d := ScoringDimension{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: w, Source: SourceManual}
		if err := d.Validate(); err == nil {
			t.Errorf("expected validation to reject weight=%v, got nil", w)
		}
	}
}

// TestFormulaRICEAppliesWeights is a regression test: previously formulaRICE
// silently ignored the per-dimension Weight, multiplying raw values directly.
// That meant a user who tuned `Weight: 2` on a RICE dimension saw no change in
// the resulting score — a silent configuration trap. Weight must influence the
// per-dimension contribution to numerator/denominator products. We use Pow
// (weights as exponents) to match formulaMultiply's geometric semantics, since
// the RICE numerator and denominator are themselves products.
func TestFormulaRICEAppliesWeights(t *testing.T) {
	// Numerator: reach=4 (weight 2) -> 4^2 = 16
	// Denominator: effort=2 (weight 1) -> 2
	// Expected: 16 / 2 = 8
	results := []DimensionResult{
		{Value: 4, Weight: 2, Role: RoleNumerator},
		{Value: 2, Weight: 1, Role: RoleDenominator},
	}
	got := formulaRICE(results)
	if math.Abs(got-8) > 1e-9 {
		t.Errorf("expected weighted RICE = 16/2 = 8, got %f (weight on numerator was likely ignored)", got)
	}
}

// TestFormulaRICEWeightOneUnchanged guards against the weight fix regressing
// the existing "weight=1 means raw value" semantic that every test in
// engine_compute_test.go depends on. Pow(v, 1) == v, so behavior must be
// identical to the pre-fix path.
func TestFormulaRICEWeightOneUnchanged(t *testing.T) {
	results := []DimensionResult{
		{Value: 8, Weight: 1, Role: RoleNumerator},
		{Value: 4, Weight: 1, Role: RoleNumerator},
		{Value: 3, Weight: 1, Role: RoleNumerator},
		{Value: 6, Weight: 1, Role: RoleDenominator},
	}
	got := formulaRICE(results)
	expected := (8.0 * 4 * 3) / 6 // = 16
	if math.Abs(got-expected) > 1e-9 {
		t.Errorf("weight=1 RICE should match unweighted result %f, got %f", expected, got)
	}
}

// TestFormulaRICENaNFromWeightReturnsZero mirrors formulaMultiply's safety net:
// Pow(negative, fractional) yields NaN, which would poison sort order if it
// propagated into the score. Treat NaN as 0 (no signal) rather than letting
// it become MaxScoreSentinel via the +Inf branch or silently sort tasks above
// well-formed ones.
func TestFormulaRICENaNFromWeightReturnsZero(t *testing.T) {
	results := []DimensionResult{
		{Value: -2, Weight: 0.5, Role: RoleNumerator}, // Pow(-2, 0.5) = NaN
		{Value: 1, Weight: 1, Role: RoleDenominator},
	}
	got := formulaRICE(results)
	if got != 0 {
		t.Errorf("NaN numerator factor should yield 0, got %f", got)
	}
}

// TestValidateRejectsDerivedMappingEmptyField: a derived dimension with an
// empty Field silently falls back to the dimension's Default for every task
// (getTaskFieldValue returns "" for the empty field, resolveField then yields
// (0, false), and resolveDimension uses the default). That makes the profile
// look like it derives from task data while in fact it never does — a
// configuration trap. Reject empty Field at validation.
func TestValidateRejectsDerivedMappingEmptyField(t *testing.T) {
	d := ScoringDimension{
		ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 1,
		Source:         SourceDerived,
		DerivedMapping: &DerivedMapping{Field: "", ValueMap: map[string]int{"x": 1}},
	}
	if err := d.Validate(); err == nil {
		t.Error("expected validation to reject derived_mapping with empty Field, got nil")
	}
}
