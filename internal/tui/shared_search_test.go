package tui

import (
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
		name  string
		query string
		want  bool
	}{
		{"empty query matches", "", true},
		{"whitespace-only query matches", "   ", true},
		{"matches title", "authentication", true},
		{"matches title partial", "auth", true},
		{"matches title case insensitive", "BUILD", true},
		{"matches description", "OAuth2", true},
		{"does not match tag", "backend", false},
		{"does not match comment", "review", false},
		{"does not fuzzy match", "bld auth", false},
		{"structured-looking text stays literal", "$ready", false},
		{"no match returns false", "xyzabc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TaskMatchesSearch(task, tt.query)
			if result != tt.want {
				t.Errorf("TaskMatchesSearch() = %v, want %v for query %q", result, tt.want, tt.query)
			}
		})
	}
}

func TestTaskMatchesSearchNilTask(t *testing.T) {
	if TaskMatchesSearch(nil, "") {
		t.Fatal("nil task must not match even an empty query")
	}
}

func TestSearchSubstringPositionsAreCaseInsensitiveAndContiguous(t *testing.T) {
	got := searchSubstringPositions("Ship OAuth Login", "OAUTH")
	want := []int{5, 6, 7, 8, 9}
	if len(got) != len(want) {
		t.Fatalf("positions = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("positions = %v, want %v", got, want)
		}
	}
	if got := searchSubstringPositions("Ship OAuth Login", "oa log"); len(got) != 0 {
		t.Fatalf("non-contiguous query positions = %v, want none", got)
	}
}

func TestTaskMatchesSearchNilFields(t *testing.T) {
	task := &models.Task{
		Title: "Minimal task",
	}
	if !TaskMatchesSearch(task, "") {
		t.Error("empty query should match minimal task")
	}
	if !TaskMatchesSearch(task, "minimal") {
		t.Error("title match should work on minimal task")
	}
	if TaskMatchesSearch(task, "nonexistent") {
		t.Error("non-matching query should return false for minimal task")
	}
}
