package scoring

import (
	"math"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestComputeScoreWeightedSum(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{ID: "impact", Name: "Impact", Min: 1, Max: 5, Default: 3, Weight: 2, Source: SourceManual},
			{ID: "urgency", Name: "Urgency", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
		},
	}

	task := &models.Task{
		ScoreDimensions: map[string]int{"impact": 4, "urgency": 2},
	}
	score := ComputeScore(task, &profile)
	// 4*2 + 2*1 = 10
	if score != 10 {
		t.Errorf("expected 10, got %f", score)
	}
}

func TestComputeScoreWeightedSumDefaults(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{ID: "impact", Name: "Impact", Min: 1, Max: 5, Default: 3, Weight: 2, Source: SourceManual},
			{ID: "urgency", Name: "Urgency", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
		},
	}

	task := &models.Task{} // nil ScoreDimensions
	score := ComputeScore(task, &profile)
	// 3*2 + 3*1 = 9
	if score != 9 {
		t.Errorf("expected 9, got %f", score)
	}
}

func TestComputeScoreMultiply(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaMultiply,
		Dimensions: []ScoringDimension{
			{ID: "a", Name: "A", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
			{ID: "b", Name: "B", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
			{ID: "c", Name: "C", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
		},
	}

	task := &models.Task{
		ScoreDimensions: map[string]int{"a": 3, "b": 4, "c": 5},
	}
	score := ComputeScore(task, &profile)
	// 3 * 4 * 5 = 60
	if score != 60 {
		t.Errorf("expected 60, got %f", score)
	}
}

func TestComputeScoreRICE(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaRICE,
		Dimensions: []ScoringDimension{
			{ID: "reach", Name: "Reach", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "impact", Name: "Impact", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "confidence", Name: "Confidence", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "effort", Name: "Effort", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleDenominator},
		},
	}

	task := &models.Task{
		ScoreDimensions: map[string]int{"reach": 8, "impact": 4, "confidence": 3, "effort": 6},
	}
	score := ComputeScore(task, &profile)
	// (8 * 4 * 3) / 6 = 96 / 6 = 16
	if score != 16 {
		t.Errorf("expected 16, got %f", score)
	}
}

func TestComputeScoreRICEDivisionByZero(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaRICE,
		Dimensions: []ScoringDimension{
			{ID: "reach", Name: "Reach", Min: 0, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "effort", Name: "Effort", Min: 0, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleDenominator},
		},
	}

	task := &models.Task{
		ScoreDimensions: map[string]int{"reach": 5, "effort": 0},
	}
	score := ComputeScore(task, &profile)
	if score != 9999 {
		t.Errorf("expected 9999 for division by zero, got %f", score)
	}
}

func TestComputeScoreWSJF(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWSJF,
		Dimensions: []ScoringDimension{
			{ID: "bv", Name: "Business Value", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "tc", Name: "Time Criticality", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "rr", Name: "Risk Reduction", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "js", Name: "Job Size", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleDenominator},
		},
	}

	task := &models.Task{
		ScoreDimensions: map[string]int{"bv": 8, "tc": 6, "rr": 4, "js": 3},
	}
	score := ComputeScore(task, &profile)
	// (8 + 6 + 4) / 3 = 18 / 3 = 6
	if score != 6 {
		t.Errorf("expected 6, got %f", score)
	}
}

func TestComputeScoreWSJFDivisionByZero(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWSJF,
		Dimensions: []ScoringDimension{
			{ID: "bv", Name: "BV", Min: 0, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
			{ID: "js", Name: "JS", Min: 0, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleDenominator},
		},
	}

	task := &models.Task{
		ScoreDimensions: map[string]int{"bv": 5, "js": 0},
	}
	score := ComputeScore(task, &profile)
	if score != 9999 {
		t.Errorf("expected 9999 for division by zero, got %f", score)
	}
}

func TestComputeScoreICE(t *testing.T) {
	profile := NewICEProfile()
	task := &models.Task{
		ScoreDimensions: map[string]int{"impact": 7, "confidence": 8, "ease": 6},
	}
	score := ComputeScore(task, &profile)
	// 7 * 8 * 6 = 336
	if score != 336 {
		t.Errorf("expected 336, got %f", score)
	}
}

func TestDerivedMappingBugSeverity(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{
				ID: "severity", Name: "Severity", Min: 1, Max: 5, Default: 2,
				Weight: 1, Source: SourceDerived,
				DerivedMapping: &DerivedMapping{
					Field:    "bug.severity",
					ValueMap: map[string]int{"critical": 5, "major": 4, "minor": 2, "cosmetic": 1},
				},
			},
		},
	}

	task := &models.Task{
		BugAttrs: &models.BugAttrs{Severity: "critical"},
	}
	score := ComputeScore(task, &profile)
	if score != 5 {
		t.Errorf("expected 5 for critical severity, got %f", score)
	}

	task.BugAttrs.Severity = "minor"
	score = ComputeScore(task, &profile)
	if score != 2 {
		t.Errorf("expected 2 for minor severity, got %f", score)
	}
}

func TestDerivedMappingTaskEffort(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{
				ID: "effort", Name: "Effort", Min: 1, Max: 5, Default: 3,
				Weight: 1, Source: SourceDerived,
				DerivedMapping: &DerivedMapping{
					Field:    "task.effort",
					ValueMap: map[string]int{"small": 1, "medium": 3, "large": 5, "unknown": 3},
				},
			},
		},
	}

	task := &models.Task{
		TaskAttrs: &models.TaskAttrs{Effort: "large"},
	}
	score := ComputeScore(task, &profile)
	if score != 5 {
		t.Errorf("expected 5 for large effort, got %f", score)
	}
}

func TestDerivedMappingPriority(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{
				ID: "priority_inv", Name: "Priority (inverted)", Min: 1, Max: 5, Default: 3,
				Weight: 1, Source: SourceDerived,
				DerivedMapping: &DerivedMapping{
					Field:    "priority",
					ValueMap: map[string]int{"1": 5, "2": 4, "3": 3, "4": 2, "5": 1},
				},
			},
		},
	}

	task := &models.Task{Priority: 1}
	score := ComputeScore(task, &profile)
	if score != 5 {
		t.Errorf("expected 5 for priority 1 (inverted), got %f", score)
	}

	task.Priority = 5
	score = ComputeScore(task, &profile)
	if score != 1 {
		t.Errorf("expected 1 for priority 5 (inverted), got %f", score)
	}
}

func TestDerivedMappingStoryPointsPassthrough(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{
				ID: "points", Name: "Story Points", Min: 1, Max: 100, Default: 5,
				Weight: 1, Source: SourceDerived,
				DerivedMapping: &DerivedMapping{
					Field:    "story.points",
					ValueMap: map[string]int{}, // empty = passthrough
				},
			},
		},
	}

	task := &models.Task{
		StoryAttrs: &models.StoryAttrs{StoryPoints: 13},
	}
	score := ComputeScore(task, &profile)
	if score != 13 {
		t.Errorf("expected 13 for passthrough, got %f", score)
	}
}

func TestDerivedMappingFallsBackToDefault(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{
				ID: "effort", Name: "Effort", Min: 1, Max: 5, Default: 3,
				Weight: 1, Source: SourceDerived,
				DerivedMapping: &DerivedMapping{
					Field:    "task.effort",
					ValueMap: map[string]int{"small": 1, "medium": 3, "large": 5},
				},
			},
		},
	}

	// Task has no TaskAttrs, so derived resolution fails -> default
	task := &models.Task{}
	score := ComputeScore(task, &profile)
	if score != 3 {
		t.Errorf("expected default 3, got %f", score)
	}
}

func TestManualOverrideTakesPrecedence(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{
				ID: "effort", Name: "Effort", Min: 1, Max: 5, Default: 3,
				Weight: 1, Source: SourceDerived,
				DerivedMapping: &DerivedMapping{
					Field:    "task.effort",
					ValueMap: map[string]int{"small": 1, "medium": 3, "large": 5},
				},
			},
		},
	}

	// Derived would give 5 (large), but manual override says 2
	task := &models.Task{
		TaskAttrs:       &models.TaskAttrs{Effort: "large"},
		ScoreDimensions: map[string]int{"effort": 2},
	}
	score := ComputeScore(task, &profile)
	if score != 2 {
		t.Errorf("expected manual override 2, got %f", score)
	}
}

func TestClampValues(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{ID: "x", Name: "X", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
		},
	}

	// Value above max should be clamped
	task := &models.Task{
		ScoreDimensions: map[string]int{"x": 100},
	}
	score := ComputeScore(task, &profile)
	if score != 5 {
		t.Errorf("expected clamped 5, got %f", score)
	}

	// Value below min should be clamped
	task.ScoreDimensions["x"] = -10
	score = ComputeScore(task, &profile)
	if score != 1 {
		t.Errorf("expected clamped 1, got %f", score)
	}
}

func TestScoreBreakdown(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 2, Source: SourceManual},
			{ID: "b", Name: "B", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
		},
	}

	task := &models.Task{
		ScoreDimensions: map[string]int{"a": 4},
	}
	results := ScoreBreakdown(task, &profile)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Value != 4 || results[0].Source != "manual" {
		t.Errorf("dim a: expected value=4 source=manual, got value=%d source=%s", results[0].Value, results[0].Source)
	}
	if results[1].Value != 5 || results[1].Source != "default" {
		t.Errorf("dim b: expected value=5 source=default, got value=%d source=%s", results[1].Value, results[1].Source)
	}
}

func TestMultiplyWithWeights(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaMultiply,
		Dimensions: []ScoringDimension{
			{ID: "a", Name: "A", Min: 1, Max: 10, Default: 5, Weight: 2, Source: SourceManual},
			{ID: "b", Name: "B", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
		},
	}

	task := &models.Task{
		ScoreDimensions: map[string]int{"a": 3, "b": 4},
	}
	score := ComputeScore(task, &profile)
	// 3^2 * 4^1 = 9 * 4 = 36
	expected := 36.0
	if math.Abs(score-expected) > 0.01 {
		t.Errorf("expected %f, got %f", expected, score)
	}
}

func TestNilScoreDimensions(t *testing.T) {
	profile := NewRICEProfile()
	task := &models.Task{} // nil ScoreDimensions
	score := ComputeScore(task, &profile)
	// Should use all defaults without panicking
	if score == 0 {
		t.Error("expected non-zero score from defaults")
	}
}

func TestIdeaValueDerived(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{
				ID: "value", Name: "Value", Min: 1, Max: 5, Default: 3,
				Weight: 1, Source: SourceDerived,
				DerivedMapping: &DerivedMapping{
					Field:    "idea.value",
					ValueMap: map[string]int{"low": 1, "medium": 3, "high": 5},
				},
			},
		},
	}

	task := &models.Task{
		IdeaAttrs: &models.IdeaAttrs{Value: "high"},
	}
	score := ComputeScore(task, &profile)
	if score != 5 {
		t.Errorf("expected 5 for high value, got %f", score)
	}
}

func TestEpicScopeDerived(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{
				ID: "scope", Name: "Scope", Min: 1, Max: 5, Default: 3,
				Weight: 1, Source: SourceDerived,
				DerivedMapping: &DerivedMapping{
					Field:    "epic.scope",
					ValueMap: map[string]int{"defined": 1, "evolving": 3, "undefined": 5},
				},
			},
		},
	}

	task := &models.Task{
		EpicAttrs: &models.EpicAttrs{Scope: "defined"},
	}
	score := ComputeScore(task, &profile)
	if score != 1 {
		t.Errorf("expected 1 for defined scope, got %f", score)
	}
}

func TestFormatScore(t *testing.T) {
	if s := FormatScore(84.7); s != "84" {
		t.Errorf("expected '84', got '%s'", s)
	}
	if s := FormatScore(0.0); s != "0" {
		t.Errorf("expected '0', got '%s'", s)
	}
}

func TestFormatBreakdown(t *testing.T) {
	results := []DimensionResult{
		{DimensionName: "Impact", Value: 4, Source: "manual", Weight: 2, Role: RoleNumerator},
		{DimensionName: "Effort", Value: 3, Source: "derived", Weight: 1, Role: RoleDenominator},
	}
	out := FormatBreakdown(results, FormulaRICE, 1.33, 55.0)
	if out == "" {
		t.Error("expected non-empty breakdown")
	}
}

func TestDefaultScore(t *testing.T) {
	iceProfile := NewICEProfile()
	iceDefault := DefaultScore(&iceProfile)
	// ICE default: 5 * 5 * 5 = 125
	if iceDefault != 125 {
		t.Errorf("ICE default: expected 125, got %f", iceDefault)
	}

	riceProfile := NewRICEProfile()
	riceDefault := DefaultScore(&riceProfile)
	// RICE default: (5 * 3 * 3) / 5 = 9
	if riceDefault != 9 {
		t.Errorf("RICE default: expected 9, got %f", riceDefault)
	}

	wsjfProfile := NewWSJFProfile()
	wsjfDefault := DefaultScore(&wsjfProfile)
	// WSJF default: (5 + 5 + 5) / 5 = 3
	if wsjfDefault != 3 {
		t.Errorf("WSJF default: expected 3, got %f", wsjfDefault)
	}
}

func TestScorePercentDefaultIs150(t *testing.T) {
	presets := []struct {
		name    string
		profile ScoringProfile
	}{
		{"ICE", NewICEProfile()},
		{"RICE", NewRICEProfile()},
		{"WSJF", NewWSJFProfile()},
	}

	for _, p := range presets {
		defScore := DefaultScore(&p.profile)
		pct := ScorePercent(defScore, &p.profile)
		if math.Abs(pct-150) > 0.01 {
			t.Errorf("%s: default score should map to 150, got %f", p.name, pct)
		}
	}
}

func TestScorePercentBounds(t *testing.T) {
	presets := []struct {
		name    string
		profile ScoringProfile
	}{
		{"ICE", NewICEProfile()},
		{"RICE", NewRICEProfile()},
		{"WSJF", NewWSJFProfile()},
	}

	for _, p := range presets {
		minS := MinScore(&p.profile)
		maxS := MaxScore(&p.profile)

		pctMin := ScorePercent(minS, &p.profile)
		if math.Abs(pctMin) > 0.01 {
			t.Errorf("%s: min score should map to 0%%, got %f", p.name, pctMin)
		}

		pctMax := ScorePercent(maxS, &p.profile)
		if math.Abs(pctMax-300) > 0.01 {
			t.Errorf("%s: max score should map to 300, got %f", p.name, pctMax)
		}
	}
}

func TestScorePercentAboveDefault(t *testing.T) {
	// For ICE (multiplicative), one dimension at 6 instead of 5 should produce
	// noticeably more than 153 (which linear would give).
	iceProfile := NewICEProfile()
	// Score with one dim=6, others=5: 6*5*5 = 150
	task := &models.Task{
		ScoreDimensions: map[string]int{"impact": 6},
	}
	score := ComputeScore(task, &iceProfile)
	pct := ScorePercent(score, &iceProfile)

	// With log scaling, expect roughly 162 (not 153 from linear)
	if pct <= 153 {
		t.Errorf("ICE log scaling: one dim at 6 should produce pct > 153, got %f", pct)
	}
	if pct > 180 {
		t.Errorf("ICE log scaling: one dim at 6 shouldn't exceed 180, got %f", pct)
	}
}

func TestScorePercentBelowDefault(t *testing.T) {
	iceProfile := NewICEProfile()
	// Score with one dim=3, others=5: 3*5*5 = 75
	task := &models.Task{
		ScoreDimensions: map[string]int{"impact": 3},
	}
	score := ComputeScore(task, &iceProfile)
	pct := ScorePercent(score, &iceProfile)

	// Linear below default: (75 - 1) / (125 - 1) * 150 ≈ 89.5
	if pct < 75 || pct > 105 {
		t.Errorf("ICE below default: expected pct around 90, got %f", pct)
	}
}

func TestScorePercentDegenerate(t *testing.T) {
	// All dimensions with same min/max/default
	profile := ScoringProfile{
		ID: "degen", Name: "Degen", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{ID: "a", Name: "A", Min: 5, Max: 5, Default: 5, Weight: 1, Source: SourceManual},
		},
	}
	pct := ScorePercent(5, &profile)
	if pct != 150 {
		t.Errorf("degenerate profile: expected 150, got %f", pct)
	}
}

func TestNormalizedScore(t *testing.T) {
	profile := NewICEProfile()
	task := &models.Task{
		ScoreDimensions: map[string]int{"impact": 7, "confidence": 8, "ease": 6},
	}
	score := ComputeScore(task, &profile)
	expected := ScorePercent(score, &profile)
	actual := NormalizedScore(task, &profile)
	if actual != expected {
		t.Errorf("NormalizedScore should equal ScorePercent(ComputeScore(...)): got %f, expected %f", actual, expected)
	}
}

func TestPresetRICEScore(t *testing.T) {
	profile := NewRICEProfile()
	task := &models.Task{
		ScoreDimensions: map[string]int{"reach": 10, "impact": 5, "confidence": 4},
		TaskAttrs:       &models.TaskAttrs{Effort: "small"},
	}
	score := ComputeScore(task, &profile)
	// reach=10, impact=5, confidence=4, effort derived from "small"=2
	// (10 * 5 * 4) / 2 = 100
	if score != 100 {
		t.Errorf("expected 100, got %f", score)
	}
}

func TestPresetWSJFScore(t *testing.T) {
	profile := NewWSJFProfile()
	task := &models.Task{
		ScoreDimensions: map[string]int{"business_value": 8, "time_criticality": 6, "risk_reduction": 4, "job_size": 3},
	}
	score := ComputeScore(task, &profile)
	// manual override for job_size=3 takes precedence over derived
	// (8 + 6 + 4) / 3 = 6
	if score != 6 {
		t.Errorf("expected 6, got %f", score)
	}
}

func TestIdeaEffortDerived(t *testing.T) {
	profile := ScoringProfile{
		ID: "test", Name: "Test", Formula: FormulaWeightedSum,
		Dimensions: []ScoringDimension{
			{
				ID: "effort", Name: "Effort", Min: 1, Max: 5, Default: 3,
				Weight: 1, Source: SourceDerived,
				DerivedMapping: &DerivedMapping{
					Field:    "idea.effort",
					ValueMap: map[string]int{"small": 1, "medium": 3, "large": 5, "unknown": 3},
				},
			},
		},
	}

	task := &models.Task{
		IdeaAttrs: &models.IdeaAttrs{Effort: "small"},
	}
	score := ComputeScore(task, &profile)
	if score != 1 {
		t.Errorf("expected 1 for small idea effort, got %f", score)
	}
}
