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

	task := &models.Task{}
	score := ComputeScore(task, &profile)
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
	if score != 336 {
		t.Errorf("expected 336, got %f", score)
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
	expected := 36.0
	if math.Abs(score-expected) > 0.01 {
		t.Errorf("expected %f, got %f", expected, score)
	}
}

func TestNilScoreDimensions(t *testing.T) {
	profile := NewRICEProfile()
	task := &models.Task{}
	score := ComputeScore(task, &profile)
	if score == 0 {
		t.Error("expected non-zero score from defaults")
	}
}

func TestPresetRICEScore(t *testing.T) {
	profile := NewRICEProfile()
	task := &models.Task{
		ScoreDimensions: map[string]int{"reach": 10, "impact": 5, "confidence": 4},
		TaskAttrs:       &models.TaskAttrs{Effort: "small"},
	}
	score := ComputeScore(task, &profile)
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
	if score != 6 {
		t.Errorf("expected 6, got %f", score)
	}
}
