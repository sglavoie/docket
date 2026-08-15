package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskMatchesSearch(t *testing.T) {
	task := &models.Task{
		Title:       "Build authentication system",
		Description: "Implement OAuth2 login flow",
		Tags:        []string{"backend", "security"},
		Comments: []*models.Comment{
			{Text: "needs review from team lead"},
		},
	}

	tests := []struct {
		name          string
		query         string
		caseSensitive bool
		want          bool
	}{
		{"empty query matches", "", false, true},
		{"whitespace-only query matches", "   ", false, true},
		{"matches title", "authentication", false, true},
		{"matches title partial", "auth", false, true},
		{"matches title case insensitive", "BUILD", false, true},
		{"case-sensitive query rejects a folded title match", "BUILD", true, false},
		{"case-sensitive query keeps an exact title match", "Build", true, true},
		{"matches description", "OAuth2", false, true},
		{"case-sensitive query keeps an exact description match", "OAuth2", true, true},
		{"case-sensitive query rejects a folded description match", "oauth2", true, false},
		{"does not match tag", "backend", false, false},
		{"does not match comment", "review", false, false},
		{"does not fuzzy match", "bld auth", false, false},
		{"structured-looking text stays literal", "$ready", false, false},
		{"no match returns false", "xyzabc", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TaskMatchesSearch(task, tt.query, tt.caseSensitive)
			if result != tt.want {
				t.Errorf("TaskMatchesSearch() = %v, want %v for query %q", result, tt.want, tt.query)
			}
		})
	}
}

func TestTaskMatchesSearchNilTask(t *testing.T) {
	if TaskMatchesSearch(nil, "", false) {
		t.Fatal("nil task must not match even an empty query")
	}
}

func TestSearchSubstringPositionsAreCaseInsensitiveAndContiguous(t *testing.T) {
	got := searchSubstringPositions("Ship OAuth Login", "OAUTH", false)
	want := []int{5, 6, 7, 8, 9}
	if len(got) != len(want) {
		t.Fatalf("positions = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("positions = %v, want %v", got, want)
		}
	}
	if got := searchSubstringPositions("Ship OAuth Login", "oa log", false); len(got) != 0 {
		t.Fatalf("non-contiguous query positions = %v, want none", got)
	}
}

func TestSearchSubstringPositionsHonorCaseSensitivity(t *testing.T) {
	if got := searchSubstringPositions("Ship OAuth Login", "OAUTH", true); len(got) != 0 {
		t.Fatalf("case-sensitive positions = %v, want none", got)
	}
	got := searchSubstringPositions("Ship OAuth Login", "OAuth", true)
	want := []int{5, 6, 7, 8, 9}
	if !slices.Equal(got, want) {
		t.Fatalf("case-sensitive positions = %v, want %v", got, want)
	}
}

func TestTaskMatchesSearchNilFields(t *testing.T) {
	task := &models.Task{
		Title: "Minimal task",
	}
	if !TaskMatchesSearch(task, "", false) {
		t.Error("empty query should match minimal task")
	}
	if !TaskMatchesSearch(task, "minimal", false) {
		t.Error("title match should work on minimal task")
	}
	if TaskMatchesSearch(task, "nonexistent", false) {
		t.Error("non-matching query should return false for minimal task")
	}
}
