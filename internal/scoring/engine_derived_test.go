package scoring

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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
					ValueMap: map[string]int{},
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

	task := &models.Task{
		ScoreDimensions: map[string]int{"x": 100},
	}
	score := ComputeScore(task, &profile)
	if score != 5 {
		t.Errorf("expected clamped 5, got %f", score)
	}

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
