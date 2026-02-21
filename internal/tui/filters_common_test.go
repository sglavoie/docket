package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCollectProjectTagNames(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []*models.Task
		expected []string
	}{
		{
			name:     "empty task list",
			tasks:    nil,
			expected: []string{},
		},
		{
			name: "multiple tasks with overlapping tags",
			tasks: []*models.Task{
				{Tags: []string{"backend", "api"}},
				{Tags: []string{"api", "frontend"}},
				{Tags: []string{"backend"}},
			},
			expected: []string{"api", "backend", "frontend"},
		},
		{
			name: "tasks with no tags",
			tasks: []*models.Task{
				{Tags: nil},
				{Tags: []string{}},
			},
			expected: []string{},
		},
		{
			name: "single task single tag",
			tasks: []*models.Task{
				{Tags: []string{"urgent"}},
			},
			expected: []string{"urgent"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CollectProjectTagNames(tc.tasks)
			if len(result) != len(tc.expected) {
				t.Fatalf("got %d tags, want %d: %v", len(result), len(tc.expected), result)
			}
			for i, tag := range result {
				if tag != tc.expected[i] {
					t.Errorf("tag[%d] = %q, want %q", i, tag, tc.expected[i])
				}
			}
		})
	}
}

func TestValidateSavedStringSet(t *testing.T) {
	tests := []struct {
		name     string
		saved    []string
		valid    []string
		expected map[string]bool
	}{
		{
			name:     "all valid",
			saved:    []string{"a", "b"},
			valid:    []string{"a", "b", "c"},
			expected: map[string]bool{"a": true, "b": true},
		},
		{
			name:     "some invalid",
			saved:    []string{"a", "b", "x"},
			valid:    []string{"a", "c"},
			expected: map[string]bool{"a": true},
		},
		{
			name:     "all invalid",
			saved:    []string{"x", "y"},
			valid:    []string{"a", "b"},
			expected: map[string]bool{},
		},
		{
			name:     "empty saved",
			saved:    nil,
			valid:    []string{"a", "b"},
			expected: map[string]bool{},
		},
		{
			name:     "empty valid",
			saved:    []string{"a", "b"},
			valid:    nil,
			expected: map[string]bool{},
		},
		{
			name:     "both empty",
			saved:    nil,
			valid:    nil,
			expected: map[string]bool{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := validateSavedStringSet(tc.saved, tc.valid)
			if len(result) != len(tc.expected) {
				t.Fatalf("got %d entries, want %d: %v", len(result), len(tc.expected), result)
			}
			for k := range tc.expected {
				if !result[k] {
					t.Errorf("missing expected key %q", k)
				}
			}
		})
	}
}
