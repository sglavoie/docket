package tui

import (
	"strings"
	"testing"
)

func TestScoreBodyTerms(t *testing.T) {
	body := "The quick brown fox jumps over the lazy dog"

	// Single term match
	score := scoreBodyTerms(body, "quick")
	if score <= 0 {
		t.Error("expected positive score for matching term 'quick'")
	}

	// Multi-term match: all terms present
	score = scoreBodyTerms(body, "quick fox")
	if score <= 0 {
		t.Error("expected positive score for 'quick fox'")
	}

	// Non-matching term
	score = scoreBodyTerms(body, "zebra")
	if score != 0 {
		t.Errorf("expected 0 score for non-matching term, got %d", score)
	}

	// One matching, one not
	score = scoreBodyTerms(body, "quick zebra")
	if score != 0 {
		t.Errorf("expected 0 when one term doesn't match, got %d", score)
	}

	// Case insensitive
	score = scoreBodyTerms(body, "QUICK")
	if score <= 0 {
		t.Error("expected positive score for case-insensitive match")
	}

	// Empty query
	score = scoreBodyTerms(body, "")
	if score != 1 {
		t.Errorf("expected 1 for empty query, got %d", score)
	}
}

func TestExtractSnippets(t *testing.T) {
	body := "The quick brown fox jumps over the lazy dog. Another sentence with important data here."

	snippet, positions := extractSnippets(body, "fox")
	if snippet == "" {
		t.Error("expected non-empty snippet for matching term")
	}
	if !strings.Contains(snippet, "fox") {
		t.Error("expected snippet to contain 'fox'")
	}
	if len(positions) == 0 {
		t.Error("expected highlight positions for matching term")
	}

	// Non-matching
	snippet, positions = extractSnippets(body, "zebra")
	if snippet == "" {
		t.Error("expected fallback snippet for non-matching term")
	}
	if len(positions) != 0 {
		t.Error("expected no positions for non-matching term")
	}

	// Empty query
	snippet, _ = extractSnippets(body, "")
	if snippet != "" {
		t.Errorf("expected empty snippet for empty query, got %q", snippet)
	}
}

func TestExtractSnippetsMultiTerm(t *testing.T) {
	body := "Alpha beta gamma delta epsilon. Much later: zeta eta theta iota kappa."

	snippet, positions := extractSnippets(body, "alpha zeta")
	if snippet == "" {
		t.Error("expected non-empty snippet for multi-term match")
	}
	if len(positions) == 0 {
		t.Error("expected positions for multi-term match")
	}
	// Both terms should be somewhere in the snippet(s)
	lower := strings.ToLower(snippet)
	if !strings.Contains(lower, "alpha") {
		t.Error("expected snippet to contain 'alpha'")
	}
}
