package fuzzy

import (
	"slices"
	"testing"
)

// TestScoreWithPositions_BasicMatch verifies that the returned positions
// align with the matched characters in the (lowercased) text.
func TestScoreWithPositions_BasicMatch(t *testing.T) {
	score, positions := NewMatcher(false).ScoreWithPositions("hello world", "hlo")
	if score == 0 {
		t.Fatalf("expected non-zero score for matching pattern")
	}
	// "hello world" lowercased is the same; match positions should be
	// indices of 'h', 'l', 'o' as a subsequence — the greedy match picks
	// 'h' at 0, 'l' at 2 (or 3), 'o' at 4.
	if len(positions) != 3 {
		t.Fatalf("expected 3 positions, got %d (%v)", len(positions), positions)
	}
	// All positions must be within text bounds
	for _, p := range positions {
		if p < 0 || p >= len("hello world") {
			t.Errorf("position %d out of bounds for text length %d", p, len("hello world"))
		}
	}
	// Positions must be strictly increasing (subsequence property).
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Errorf("positions not strictly increasing: %v", positions)
		}
	}
}

func TestScoreWithPositions_EmptyPattern(t *testing.T) {
	score, positions := NewMatcher(false).ScoreWithPositions("anything", "")
	if score != 1 {
		t.Errorf("empty pattern: expected score=1, got %d", score)
	}
	if positions != nil {
		t.Errorf("empty pattern: expected nil positions, got %v", positions)
	}
}

func TestScoreWithPositions_NoMatch(t *testing.T) {
	score, positions := NewMatcher(false).ScoreWithPositions("hello", "xyz")
	if score != 0 {
		t.Errorf("no match: expected score=0, got %d", score)
	}
	if positions != nil {
		t.Errorf("no match: expected nil positions, got %v", positions)
	}
}

func TestScoreWithPositions_EmptyText(t *testing.T) {
	score, positions := NewMatcher(false).ScoreWithPositions("", "pattern")
	if score != 0 {
		t.Errorf("empty text: expected score=0, got %d", score)
	}
	if positions != nil {
		t.Errorf("empty text: expected nil positions, got %v", positions)
	}
}

func TestScoreWithPositionsPrefersLaterWordBoundaryMatch(t *testing.T) {
	score, positions := NewMatcher(false).ScoreWithPositions("axb-b", "ab")
	if score == 0 {
		t.Fatalf("expected non-zero score for matching pattern")
	}
	want := []int{0, 4}
	if !slices.Equal(positions, want) {
		t.Fatalf("positions = %v, want %v", positions, want)
	}
}

func TestScoreTermsWithPositions_AllTermsMatch(t *testing.T) {
	score, positions := NewMatcher(false).ScoreTermsWithPositions("need a way to go", "need way")
	if score == 0 {
		t.Fatalf("expected non-zero score when both terms match")
	}
	if len(positions) == 0 {
		t.Fatalf("expected positions to be returned")
	}
	// Positions must be sorted and unique.
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Errorf("positions not strictly increasing (must be sorted, deduplicated): %v", positions)
		}
	}
}

func TestScoreTermsWithPositions_OneTermMissing(t *testing.T) {
	score, positions := NewMatcher(false).ScoreTermsWithPositions("hello world", "hello xyz")
	if score != 0 {
		t.Errorf("expected score=0 when a term is missing, got %d", score)
	}
	if positions != nil {
		t.Errorf("expected nil positions on miss, got %v", positions)
	}
}

func TestScoreTermsWithPositions_EmptyQuery(t *testing.T) {
	score, positions := NewMatcher(false).ScoreTermsWithPositions("anything", "")
	if score != 1 {
		t.Errorf("expected score=1 for empty query, got %d", score)
	}
	if positions != nil {
		t.Errorf("expected nil positions for empty query, got %v", positions)
	}

	score, positions = NewMatcher(false).ScoreTermsWithPositions("anything", "   ")
	if score != 1 {
		t.Errorf("expected score=1 for whitespace query, got %d", score)
	}
	if positions != nil {
		t.Errorf("expected nil positions for whitespace query, got %v", positions)
	}
}

func TestScoreTermsWithPositions_DeduplicatesOverlap(t *testing.T) {
	// "aa" and "ab" both match in "aabc"; their positions overlap at index 0.
	// The merged result must not contain duplicates.
	_, positions := NewMatcher(false).ScoreTermsWithPositions("aabc", "aa ab")
	seen := make(map[int]struct{})
	for _, p := range positions {
		if _, exists := seen[p]; exists {
			t.Errorf("duplicate position %d in result %v", p, positions)
		}
		seen[p] = struct{}{}
	}
	// Quick equality sanity check that the slice is sorted.
	sorted := append([]int(nil), positions...)
	for i := 1; i < len(sorted); i++ {
		if sorted[i] < sorted[i-1] {
			t.Errorf("positions not sorted: %v", positions)
			break
		}
	}
}
