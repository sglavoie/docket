package fuzzy

import (
	"slices"
	"testing"
)

func TestCaseSensitiveQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"empty", "", false},
		{"all lowercase", "auth login", false},
		{"leading uppercase", "Auth", true},
		{"interior uppercase", "oAuth", true},
		{"digits and punctuation only", "alpha#42", false},
		{"non-ASCII uppercase", "École", true},
		{"non-ASCII lowercase", "école", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CaseSensitiveQuery(tt.query); got != tt.want {
				t.Errorf("CaseSensitiveQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestSmartCaseSelectsMatcher(t *testing.T) {
	if SmartCase("auth").CaseSensitive() {
		t.Error("all-lowercase query produced a case-sensitive matcher")
	}
	if !SmartCase("Auth").CaseSensitive() {
		t.Error("uppercase-carrying query produced a case-insensitive matcher")
	}
	if (Matcher{}).CaseSensitive() {
		t.Error("zero-value matcher is case-sensitive, which would change every non-search call site")
	}
}

func TestMatcherCaseSensitiveScoring(t *testing.T) {
	sensitive := NewMatcher(true)
	insensitive := NewMatcher(false)

	if score := sensitive.Score("capitalize", "API"); score != 0 {
		t.Errorf("case-sensitive Score(capitalize, API) = %d, want 0", score)
	}
	if score := sensitive.Score("Ship API client", "API"); score <= 0 {
		t.Errorf("case-sensitive Score on an exact-case text = %d, want a match", score)
	}
	if score := insensitive.Score("capitalize", "API"); score <= 0 {
		t.Errorf("case-insensitive Score(capitalize, API) = %d, want a match", score)
	}

	if score := sensitive.ScoreTerms("Ship API client", "API ship"); score != 0 {
		t.Errorf("case-sensitive ScoreTerms with one folded term = %d, want 0", score)
	}
	if score := sensitive.ScoreTerms("Ship API client", "API Ship"); score <= 0 {
		t.Errorf("case-sensitive ScoreTerms with exact-case terms = %d, want a match", score)
	}
}

func TestMatcherPositionsIndexOriginalRunes(t *testing.T) {
	score, positions := NewMatcher(true).ScoreWithPositions("Ship OAuth Login", "OAuth")
	if score <= 0 {
		t.Fatalf("case-sensitive ScoreWithPositions score = %d, want a match", score)
	}
	if want := []int{5, 6, 7, 8, 9}; !slices.Equal(positions, want) {
		t.Fatalf("positions = %v, want %v", positions, want)
	}
	if score, _ := NewMatcher(true).ScoreWithPositions("Ship OAuth Login", "oauth"); score != 0 {
		t.Fatalf("case-sensitive ScoreWithPositions on a folded pattern = %d, want 0", score)
	}

	score, positions = NewMatcher(true).ScoreTermsWithPositions("Ship OAuth Login", "OAuth Ship")
	if score <= 0 || len(positions) != 9 {
		t.Fatalf("case-sensitive ScoreTermsWithPositions = %d, %v, want a match over both terms", score, positions)
	}
}

// The package-level functions are the case-insensitive path every non-search
// call site still uses, so they must stay identical to the zero-value matcher.
func TestPackageFunctionsRemainCaseInsensitive(t *testing.T) {
	insensitive := NewMatcher(false)
	for _, pattern := range []string{"API", "api", "aPi"} {
		if got, want := Score("capitalize", pattern), insensitive.Score("capitalize", pattern); got != want {
			t.Errorf("Score(capitalize, %q) = %d, want %d", pattern, got, want)
		}
		if got, want := ScoreTerms("Ship API client", pattern), insensitive.ScoreTerms("Ship API client", pattern); got != want {
			t.Errorf("ScoreTerms(_, %q) = %d, want %d", pattern, got, want)
		}
		gotScore, gotPositions := ScoreWithPositions("Ship OAuth Login", pattern)
		wantScore, wantPositions := insensitive.ScoreWithPositions("Ship OAuth Login", pattern)
		if gotScore != wantScore || !slices.Equal(gotPositions, wantPositions) {
			t.Errorf("ScoreWithPositions(_, %q) = %d %v, want %d %v", pattern, gotScore, gotPositions, wantScore, wantPositions)
		}
		gotScore, gotPositions = ScoreTermsWithPositions("Ship OAuth Login", pattern)
		wantScore, wantPositions = insensitive.ScoreTermsWithPositions("Ship OAuth Login", pattern)
		if gotScore != wantScore || !slices.Equal(gotPositions, wantPositions) {
			t.Errorf("ScoreTermsWithPositions(_, %q) = %d %v, want %d %v", pattern, gotScore, gotPositions, wantScore, wantPositions)
		}
	}
}
