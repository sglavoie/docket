package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetVisibleSiblings(t *testing.T) {
	// Create a project with siblings
	parent := &models.Task{
		ID:    "parent",
		Title: "Parent",
		Type:  models.TaskTypeEpic,
	}

	child1 := &models.Task{
		ID:            "child1",
		Title:         "First Child",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		ParentID:      &parent.ID,
		Parent:        parent,
	}

	child2 := &models.Task{
		ID:            "child2",
		Title:         "Second Child",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateInbox, // Different section
		ParentID:      &parent.ID,
		Parent:        parent,
	}

	child3 := &models.Task{
		ID:            "child3",
		Title:         "Third Child",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		ParentID:      &parent.ID,
		Parent:        parent,
	}

	parent.Children = []*models.Task{child1, child2, child3}

	project := &models.Project{
		Tasks: []*models.Task{parent, child1, child2, child3},
	}

	// Test in Active view mode (hides Inbox)
	m := &ProjectTreeModel{
		project:  project,
		viewMode: ViewModeActive,
	}
	m.buildTree()

	siblings := m.getVisibleSiblings(child1)

	// child2 is in Inbox section, so should not be visible in Active mode
	// Only child1 and child3 should be visible
	if len(siblings) != 2 {
		t.Errorf("getVisibleSiblings() returned %d siblings, expected 2", len(siblings))
	}

	// Verify correct siblings
	foundChild1 := false
	foundChild3 := false
	for _, sibling := range siblings {
		if sibling.ID == "child1" {
			foundChild1 = true
		}
		if sibling.ID == "child3" {
			foundChild3 = true
		}
	}

	if !foundChild1 || !foundChild3 {
		t.Errorf("getVisibleSiblings() did not return expected siblings")
	}
}

func TestFindVisibleSiblingIndex(t *testing.T) {
	child1 := &models.Task{ID: "child1", Title: "First"}
	child2 := &models.Task{ID: "child2", Title: "Second"}
	child3 := &models.Task{ID: "child3", Title: "Third"}

	siblings := []*models.Task{child1, child2, child3}

	m := &ProjectTreeModel{}

	tests := []struct {
		name     string
		task     *models.Task
		expected int
	}{
		{
			name:     "first sibling",
			task:     child1,
			expected: 0,
		},
		{
			name:     "middle sibling",
			task:     child2,
			expected: 1,
		},
		{
			name:     "last sibling",
			task:     child3,
			expected: 2,
		},
		{
			name:     "not in siblings",
			task:     &models.Task{ID: "other", Title: "Other"},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.findVisibleSiblingIndex(tt.task, siblings)
			if result != tt.expected {
				t.Errorf("findVisibleSiblingIndex() = %d, expected %d", result, tt.expected)
			}
		})
	}
}
