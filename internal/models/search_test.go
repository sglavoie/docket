package models

import (
	"strings"
	"testing"
)

func TestMatchesSearchQuery_TitleMatch(t *testing.T) {
	task := &Task{
		Title:       "Implement user authentication",
		Description: "Add login and logout functionality",
	}

	tests := []struct {
		query    string
		expected bool
	}{
		{"user", true},
		{"USER", true},
		{"authentication", true},
		{"implement", true},
		{"Implement User", true},
		{"login", true}, // matches description
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := task.MatchesSearchQuery(strings.ToLower(tt.query))
			if result != tt.expected {
				t.Errorf("matchesSearchQuery(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestMatchesSearchQuery_DescriptionMatch(t *testing.T) {
	task := &Task{
		Title:       "Fix bug in API",
		Description: "The authentication token is not being validated correctly",
	}

	tests := []struct {
		query    string
		expected bool
	}{
		{"authentication", true},
		{"token", true},
		{"validated", true},
		{"CORRECTLY", true},
		{"bug", true}, // matches title
		{"api", true}, // matches title
		{"database", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := task.MatchesSearchQuery(strings.ToLower(tt.query))
			if result != tt.expected {
				t.Errorf("matchesSearchQuery(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

// TestMatchesSearchQuery_ResumeNoteMatch holds the resume note inside the
// model-level matcher: it is prose the task carries, so a query naming only what
// the note says still finds the task, and a query naming nothing any of the
// three fields says still misses it.
func TestMatchesSearchQuery_ResumeNoteMatch(t *testing.T) {
	task := &Task{
		Title:       "Fix bug in API",
		Description: "The authentication token is not being validated correctly",
		ResumeNote:  "stopped at the failing refresh case",
	}

	tests := []struct {
		query    string
		expected bool
	}{
		{"refresh case", true},
		{"STOPPED", true},
		{"failing", true},
		{"bug", true},            // matches title
		{"authentication", true}, // matches description
		{"database", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			if result := task.MatchesSearchQuery(strings.ToLower(tt.query)); result != tt.expected {
				t.Errorf("MatchesSearchQuery(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}

	// A task carrying no note behaves exactly as it did before the field existed.
	bare := &Task{Title: task.Title, Description: task.Description}
	if bare.MatchesSearchQuery("failing") {
		t.Error("a task with no resume note matched the note-only query")
	}
}

func TestMatchesSearchQuery_EmptyQuery(t *testing.T) {
	task := &Task{
		Title:       "Some task",
		Description: "Some description",
	}

	result := task.MatchesSearchQuery("")
	if !result {
		t.Errorf("matchesSearchQuery with empty query should match all tasks")
	}
}

func TestMatchesSearchQuery_EmptyTask(t *testing.T) {
	task := &Task{
		Title:       "",
		Description: "",
	}

	tests := []struct {
		query    string
		expected bool
	}{
		{"", true},
		{"anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := task.MatchesSearchQuery(strings.ToLower(tt.query))
			if result != tt.expected {
				t.Errorf("matchesSearchQuery(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestMatchesSearchQuery_SpecialCharacters(t *testing.T) {
	task := &Task{
		Title:       "Fix C++ compiler error",
		Description: "Handle edge case with [brackets] and (parentheses)",
	}

	tests := []struct {
		query    string
		expected bool
	}{
		{"c++", true},
		{"[brackets]", true},
		{"(parentheses)", true},
		{"edge case", true},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := task.MatchesSearchQuery(strings.ToLower(tt.query))
			if result != tt.expected {
				t.Errorf("matchesSearchQuery(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestMatchesSearchQuery_PartialMatch(t *testing.T) {
	task := &Task{
		Title:       "Implement authentication",
		Description: "Add OAuth2 support",
	}

	tests := []struct {
		query    string
		expected bool
	}{
		{"auth", true},    // partial of authentication
		{"ment", true},    // middle of implement
		{"OAuth", true},   // matches description
		{"OAuth2", true},  // exact match
		{"OAuth3", false}, // not present
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := task.MatchesSearchQuery(strings.ToLower(tt.query))
			if result != tt.expected {
				t.Errorf("matchesSearchQuery(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestSearchAcrossTasks(t *testing.T) {
	tasks := []*Task{
		{Title: "Implement login", Description: "User authentication flow"},
		{Title: "Fix database bug", Description: "Connection pool issue"},
		{Title: "Add logout button", Description: "UI improvement"},
		{Title: "Write tests", Description: "Unit tests for auth module"},
	}

	query := strings.ToLower("auth")
	var matches []*Task
	for _, task := range tasks {
		if task.MatchesSearchQuery(query) {
			matches = append(matches, task)
		}
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 matches for 'auth', got %d", len(matches))
	}

	// Verify correct tasks matched
	expectedTitles := map[string]bool{
		"Implement login": true, // description contains "authentication"
		"Write tests":     true, // description contains "auth"
	}

	for _, m := range matches {
		if !expectedTitles[m.Title] {
			t.Errorf("unexpected match: %s", m.Title)
		}
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	task := &Task{
		Title:       "FiX BuG In ApI",
		Description: "ThIs Is A MiXeD CaSe DeScRiPtIoN",
	}

	queries := []string{
		"fix bug in api",
		"FIX BUG IN API",
		"Fix Bug In API",
		"mixed case",
		"MIXED CASE",
		"MiXeD cAsE",
	}

	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			result := task.MatchesSearchQuery(strings.ToLower(query))
			if !result {
				t.Errorf("MatchesSearchQuery(%q) should match (case-insensitive)", query)
			}
		})
	}
}

func TestMatchesSearchQuery_TagsNotSearched(t *testing.T) {
	task := &Task{
		Title:       "Simple task",
		Description: "A simple description",
		Tags:        []string{"backend", "urgent", "api-related"},
	}

	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{"tag not in title or description", "backend", false},
		{"tag not in title or description", "urgent", false},
		{"tag with hyphen", "api-related", false},
		{"partial tag match", "api", false},
		{"title still matches", "simple", true},
		{"description still matches", "description", true},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.query, func(t *testing.T) {
			result := task.MatchesSearchQuery(strings.ToLower(tt.query))
			if result != tt.expected {
				t.Errorf("MatchesSearchQuery(%q) = %v, want %v (tags are not searched)", tt.query, result, tt.expected)
			}
		})
	}
}

func TestMatchesSearchQuery_NoMatchReturnsFalse(t *testing.T) {
	task := &Task{
		Title:       "Implement feature X",
		Description: "Add functionality for users",
	}

	noMatchQueries := []string{
		"database",
		"migration",
		"test",
		"xyz123",
		"completely unrelated",
	}

	for _, query := range noMatchQueries {
		t.Run(query, func(t *testing.T) {
			result := task.MatchesSearchQuery(strings.ToLower(query))
			if result {
				t.Errorf("MatchesSearchQuery(%q) should return false for non-matching query", query)
			}
		})
	}
}
