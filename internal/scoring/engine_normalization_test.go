package scoring

import (
	"math"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestDefaultScore(t *testing.T) {
	iceProfile := NewICEProfile()
	iceDefault := DefaultScore(&iceProfile)
	if iceDefault != 125 {
		t.Errorf("ICE default: expected 125, got %f", iceDefault)
	}

	riceProfile := NewRICEProfile()
	riceDefault := DefaultScore(&riceProfile)
	if riceDefault != 9 {
		t.Errorf("RICE default: expected 9, got %f", riceDefault)
	}

	wsjfProfile := NewWSJFProfile()
	wsjfDefault := DefaultScore(&wsjfProfile)
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
	iceProfile := NewICEProfile()
	task := &models.Task{
		ScoreDimensions: map[string]int{"impact": 6},
	}
	score := ComputeScore(task, &iceProfile)
	pct := ScorePercent(score, &iceProfile)

	if pct <= 153 {
		t.Errorf("ICE log scaling: one dim at 6 should produce pct > 153, got %f", pct)
	}
	if pct > 180 {
		t.Errorf("ICE log scaling: one dim at 6 shouldn't exceed 180, got %f", pct)
	}
}

func TestScorePercentBelowDefault(t *testing.T) {
	iceProfile := NewICEProfile()
	task := &models.Task{
		ScoreDimensions: map[string]int{"impact": 3},
	}
	score := ComputeScore(task, &iceProfile)
	pct := ScorePercent(score, &iceProfile)

	if pct < 75 || pct > 105 {
		t.Errorf("ICE below default: expected pct around 90, got %f", pct)
	}
}

func TestScorePercentDegenerate(t *testing.T) {
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

func TestScorePercentEdgeCases(t *testing.T) {
	t.Run("defS==0 with positive score uses linear fallback", func(t *testing.T) {
		profile := ScoringProfile{
			ID: "zero-def", Name: "ZeroDef", Formula: FormulaMultiply,
			Dimensions: []ScoringDimension{
				{ID: "a", Name: "A", Min: 0, Max: 10, Default: 0, Weight: 1, Source: SourceManual},
				{ID: "b", Name: "B", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
			},
		}
		pct := ScorePercent(25, &profile)
		if math.IsNaN(pct) || math.IsInf(pct, 0) {
			t.Fatalf("ScorePercent returned NaN or Inf: %f", pct)
		}
		maxN := float64(MaxNormalizedScoreValue)
		mid := maxN / 2
		if pct < mid || pct > maxN {
			t.Errorf("expected pct in [%f, %f], got %f", mid, maxN, pct)
		}
	})

	t.Run("all scores equal (minS==defS==maxS)", func(t *testing.T) {
		profile := ScoringProfile{
			ID: "flat", Name: "Flat", Formula: FormulaWeightedSum,
			Dimensions: []ScoringDimension{
				{ID: "a", Name: "A", Min: 7, Max: 7, Default: 7, Weight: 1, Source: SourceManual},
				{ID: "b", Name: "B", Min: 3, Max: 3, Default: 3, Weight: 2, Source: SourceManual},
			},
		}
		pct := ScorePercent(13, &profile)
		mid := float64(MaxNormalizedScoreValue) / 2
		if math.Abs(pct-mid) > 0.01 {
			t.Errorf("expected %f when all scores equal, got %f", mid, pct)
		}
	})

	t.Run("score below minS clamps to 0", func(t *testing.T) {
		profile := ScoringProfile{
			ID: "clamp-low", Name: "ClampLow", Formula: FormulaWeightedSum,
			Dimensions: []ScoringDimension{
				{ID: "a", Name: "A", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
			},
		}
		pct := ScorePercent(-10, &profile)
		if pct != 0 {
			t.Errorf("expected 0 for score far below minS, got %f", pct)
		}
	})

	t.Run("score above maxS clamps to maxN", func(t *testing.T) {
		profile := ScoringProfile{
			ID: "clamp-high", Name: "ClampHigh", Formula: FormulaWeightedSum,
			Dimensions: []ScoringDimension{
				{ID: "a", Name: "A", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
			},
		}
		pct := ScorePercent(9999, &profile)
		maxN := float64(MaxNormalizedScoreValue)
		if pct != maxN {
			t.Errorf("expected %f for score far above maxS, got %f", maxN, pct)
		}
	})

	t.Run("score at default maps to mid even when defS==minS", func(t *testing.T) {
		// Profile where Min == Default but Max > Default for the dimension.
		// MinScore == DefaultScore but MaxScore is larger, so the "below
		// default" branch has zero range. Score equal to defS should still
		// map to mid (150), not 0.
		profile := ScoringProfile{
			ID: "no-below", Name: "NoBelow", Formula: FormulaWeightedSum,
			Dimensions: []ScoringDimension{
				{ID: "a", Name: "A", Min: 3, Max: 10, Default: 3, Weight: 1, Source: SourceManual},
			},
		}
		defS := DefaultScore(&profile)
		pct := ScorePercent(defS, &profile)
		mid := float64(MaxNormalizedScoreValue) / 2
		if math.Abs(pct-mid) > 0.01 {
			t.Errorf("score at default should map to mid (%f), got %f", mid, pct)
		}
	})

	t.Run("zero score with multiplicative formula", func(t *testing.T) {
		profile := ScoringProfile{
			ID: "mult-zero", Name: "MultZero", Formula: FormulaMultiply,
			Dimensions: []ScoringDimension{
				{ID: "a", Name: "A", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
				{ID: "b", Name: "B", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual},
			},
		}
		pct := ScorePercent(0, &profile)
		if pct < 0 {
			t.Errorf("expected pct >= 0, got %f", pct)
		}
		mid := float64(MaxNormalizedScoreValue) / 2
		if pct > mid {
			t.Errorf("expected pct <= %f (mid) for zero score, got %f", mid, pct)
		}
	})
}
