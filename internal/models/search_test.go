package models

import (
	"strings"
	"testing"
)

// matchesSearchQuery checks if a task matches the search query.
// This is the same logic used in cmd/search.go and internal/tui/global_search.go.
func matchesSearchQuery(task *Task, queryLower string) bool {
	if strings.Contains(strings.ToLower(task.Title), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(task.Description), queryLower) {
		return true
	}
	return false
}

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
			result := matchesSearchQuery(task, strings.ToLower(tt.query))
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
			result := matchesSearchQuery(task, strings.ToLower(tt.query))
			if result != tt.expected {
				t.Errorf("matchesSearchQuery(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestMatchesSearchQuery_EmptyQuery(t *testing.T) {
	task := &Task{
		Title:       "Some task",
		Description: "Some description",
	}

	result := matchesSearchQuery(task, "")
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
			result := matchesSearchQuery(task, strings.ToLower(tt.query))
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
			result := matchesSearchQuery(task, strings.ToLower(tt.query))
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
			result := matchesSearchQuery(task, strings.ToLower(tt.query))
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
		if matchesSearchQuery(task, query) {
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
			result := matchesSearchQuery(task, strings.ToLower(query))
			if !result {
				t.Errorf("matchesSearchQuery(%q) should match (case-insensitive)", query)
			}
		})
	}
}
