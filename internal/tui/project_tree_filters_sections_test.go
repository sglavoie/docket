package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetHiddenSectionsForViewMode(t *testing.T) {
	tests := []struct {
		name     string
		viewMode ViewMode
		expected []string
	}{
		{
			name:     "active mode hides special sections",
			viewMode: ViewModeActive,
			expected: []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionSomeday, models.SectionReview, models.SectionStandby, models.SectionCompleted},
		},
		{
			name:     "next mode hides other sections",
			viewMode: ViewModeNext,
			expected: []string{models.SectionInbox, models.SectionWaiting, models.SectionSomeday, models.SectionReview, models.SectionStandby, models.SectionCompleted, "Active"},
		},
		{
			name:     "inbox mode",
			viewMode: ViewModeInbox,
			expected: []string{models.SectionNext, models.SectionWaiting, models.SectionSomeday, models.SectionReview, models.SectionStandby, models.SectionCompleted, "Active"},
		},
		{
			name:     "waiting mode",
			viewMode: ViewModeWaiting,
			expected: []string{models.SectionInbox, models.SectionNext, models.SectionSomeday, models.SectionReview, models.SectionStandby, models.SectionCompleted, "Active"},
		},
		{
			name:     "someday mode",
			viewMode: ViewModeSomeday,
			expected: []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionReview, models.SectionStandby, models.SectionCompleted, "Active"},
		},
		{
			name:     "review mode hides other sections",
			viewMode: ViewModeReview,
			expected: []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionSomeday, models.SectionStandby, models.SectionCompleted, "Active"},
		},
		{
			name:     "standby mode hides other sections",
			viewMode: ViewModeStandby,
			expected: []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionSomeday, models.SectionReview, models.SectionCompleted, "Active"},
		},
		{
			name:     "all mode hides only completed",
			viewMode: ViewModeAll,
			expected: []string{models.SectionCompleted},
		},
		{
			name:     "completed mode shows all",
			viewMode: ViewModeCompleted,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ProjectTreeModel{viewMode: tt.viewMode}
			result := m.getHiddenSectionsForViewMode()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("getHiddenSectionsForViewMode() = %v, expected nil", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("getHiddenSectionsForViewMode() returned %d sections, expected %d", len(result), len(tt.expected))
				return
			}

			// Check all expected sections are present
			expectedMap := make(map[string]bool)
			for _, s := range tt.expected {
				expectedMap[s] = true
			}

			for _, s := range result {
				if !expectedMap[s] {
					t.Errorf("getHiddenSectionsForViewMode() contains unexpected section %q", s)
				}
			}
		})
	}
}

func TestCountChildrenBySection(t *testing.T) {
	// Create test tasks
	parentTask := &models.Task{
		ID:       "parent",
		Section:  "Active",
		Type:     models.TaskTypeEpic,
		Children: []*models.Task{},
	}

	child1 := &models.Task{
		ID:       "child1",
		Section:  models.SectionNext,
		Type:     models.TaskTypeTask,
		Children: []*models.Task{},
	}

	child2 := &models.Task{
		ID:       "child2",
		Section:  models.SectionInbox,
		Type:     models.TaskTypeTask,
		Children: []*models.Task{},
	}

	child3 := &models.Task{
		ID:       "child3",
		Section:  models.SectionNext,
		Type:     models.TaskTypeTask,
		Children: []*models.Task{},
	}

	grandchild := &models.Task{
		ID:       "grandchild",
		Section:  models.SectionWaiting,
		Type:     models.TaskTypeTask,
		Children: []*models.Task{},
	}

	childEmptySection := &models.Task{
		ID:       "empty-section",
		Section:  "", // Empty section should be normalized to "Active"
		Type:     models.TaskTypeTask,
		Children: []*models.Task{},
	}

	parentTask.Children = []*models.Task{child1, child2, child3, childEmptySection}
	child1.Children = []*models.Task{grandchild}

	m := &ProjectTreeModel{}

	tests := []struct {
		name     string
		task     *models.Task
		expected map[string]int
	}{
		{
			name: "count all children by section",
			task: parentTask,
			expected: map[string]int{
				models.SectionNext:    2, // child1 and child3
				models.SectionInbox:   1, // child2
				models.SectionWaiting: 1, // grandchild
				"Active":              1, // childEmptySection (empty normalized to Active)
			},
		},
		{
			name: "task with single child in one section",
			task: child1,
			expected: map[string]int{
				models.SectionWaiting: 1,
			},
		},
		{
			name:     "task with no children returns empty",
			task:     child2,
			expected: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.countChildrenBySection(tt.task)

			if len(result) != len(tt.expected) {
				t.Errorf("countChildrenBySection() returned %d sections, expected %d. Got: %v, Expected: %v",
					len(result), len(tt.expected), result, tt.expected)
				return
			}

			for section, expectedCount := range tt.expected {
				if result[section] != expectedCount {
					t.Errorf("countChildrenBySection() section %q = %d, expected %d",
						section, result[section], expectedCount)
				}
			}
		})
	}
}

func TestCountChildrenBySectionCountsNonGTDAsActive(t *testing.T) {
	parent := &models.Task{
		ID:       "parent",
		Type:     models.TaskTypeEpic,
		Children: []*models.Task{},
	}

	activeChild := &models.Task{
		ID:       "active",
		Type:     models.TaskTypeTask,
		Children: []*models.Task{},
	}

	customChild := &models.Task{
		ID:       "custom",
		Type:     models.TaskTypeTask,
		Children: []*models.Task{},
	}
	customChild.SetSection("Sprint-1")

	parent.Children = []*models.Task{activeChild, customChild}

	m := &ProjectTreeModel{}
	got := m.countChildrenBySection(parent)

	if got["Active"] != 2 {
		t.Fatalf("expected 2 Active children, got %d", got["Active"])
	}
	if got["Sprint-1"] != 0 {
		t.Fatalf("expected no Sprint-1 child count, got %d", got["Sprint-1"])
	}
	if _, ok := got[""]; ok {
		t.Fatalf("expected empty sections to avoid empty-key counts, got %v", got)
	}
}
