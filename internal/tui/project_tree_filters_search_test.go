package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetAllTags(t *testing.T) {
	project := &models.Project{
		Tasks: []*models.Task{
			{
				ID:   "task1",
				Tags: []string{"backend", "urgent"},
			},
			{
				ID:   "task2",
				Tags: []string{"frontend", "urgent"},
			},
			{
				ID:   "task3",
				Tags: []string{"backend"},
			},
			{
				ID:   "task4",
				Tags: []string{},
			},
		},
	}

	m := &ProjectTreeModel{project: project}
	tags := m.getAllTags()

	// Should get unique sorted tags
	expected := []string{"backend", "frontend", "urgent"}

	if len(tags) != len(expected) {
		t.Errorf("getAllTags() returned %d tags, expected %d", len(tags), len(expected))
		return
	}

	for i, tag := range expected {
		if tags[i] != tag {
			t.Errorf("getAllTags()[%d] = %q, expected %q", i, tags[i], tag)
		}
	}
}

func TestGetAllTagsEmpty(t *testing.T) {
	project := &models.Project{
		Tasks: []*models.Task{
			{ID: "task1", Tags: []string{}},
			{ID: "task2", Tags: []string{}},
		},
	}

	m := &ProjectTreeModel{project: project}
	tags := m.getAllTags()

	if len(tags) != 0 {
		t.Errorf("getAllTags() returned %d tags, expected 0", len(tags))
	}
}

func TestMatchesSearch(t *testing.T) {
	task := &models.Task{
		Title:       "Build authentication system",
		Description: "Implement OAuth2 login flow",
		Tags:        []string{"backend", "security"},
	}

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{
			name:  "empty query matches",
			query: "",
			want:  true,
		},
		{
			name:  "matches title",
			query: "authentication",
			want:  true,
		},
		{
			name:  "matches title partial",
			query: "auth",
			want:  true,
		},
		{
			name:  "matches title case insensitive",
			query: "BUILD",
			want:  true,
		},
		{
			name:  "matches description",
			query: "OAuth2",
			want:  true,
		},
		{
			name:  "matches tag",
			query: "backend",
			want:  true,
		},
		{
			name:  "matches tag partial",
			query: "back",
			want:  true,
		},
		{
			name:  "no match returns false",
			query: "xyzabc",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ProjectTreeModel{}
			m.InitSearchInput("", 100, 40)
			m.SetSearchQuery(tt.query)
			result := m.matchesSearch(task)
			if result != tt.want {
				t.Errorf("matchesSearch() = %v, want %v for query %q", result, tt.want, tt.query)
			}
		})
	}
}

func TestHasMatchingDescendant(t *testing.T) {
	// Create hierarchy: parent -> child -> grandchild
	grandchild := &models.Task{
		ID:          "grandchild",
		Title:       "Implement authentication",
		Description: "",
		Tags:        []string{},
		Children:    []*models.Task{},
	}

	child := &models.Task{
		ID:          "child",
		Title:       "Build feature",
		Description: "",
		Tags:        []string{},
		Children:    []*models.Task{grandchild},
	}

	parent := &models.Task{
		ID:          "parent",
		Title:       "Epic task",
		Description: "",
		Tags:        []string{},
		Children:    []*models.Task{child},
	}

	tests := []struct {
		name  string
		task  *models.Task
		query string
		want  bool
	}{
		{
			name:  "direct child matches",
			task:  parent,
			query: "feature",
			want:  true,
		},
		{
			name:  "grandchild matches",
			task:  parent,
			query: "authentication",
			want:  true,
		},
		{
			name:  "no descendant matches",
			task:  parent,
			query: "xyzabc",
			want:  false,
		},
		{
			name:  "task with no children",
			task:  grandchild,
			query: "xyzabc",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ProjectTreeModel{}
			m.InitSearchInput("", 100, 40)
			m.SetSearchQuery(tt.query)
			result := m.hasMatchingDescendant(tt.task)
			if result != tt.want {
				t.Errorf("hasMatchingDescendant() = %v, want %v for query %q", result, tt.want, tt.query)
			}
		})
	}
}
