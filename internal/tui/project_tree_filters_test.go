package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestIsOverdue(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	tests := []struct {
		name string
		task *models.Task
		want bool
	}{
		{
			name: "overdue task returns true",
			task: &models.Task{
				DueDate: &yesterday,
				Status:  models.StatusTodo,
			},
			want: true,
		},
		{
			name: "completed overdue task returns false",
			task: &models.Task{
				DueDate: &yesterday,
				Status:  models.StatusDone,
			},
			want: false,
		},
		{
			name: "future due date returns false",
			task: &models.Task{
				DueDate: &tomorrow,
				Status:  models.StatusTodo,
			},
			want: false,
		},
		{
			name: "nil due date returns false",
			task: &models.Task{
				DueDate: nil,
				Status:  models.StatusTodo,
			},
			want: false,
		},
		{
			name: "due today returns false",
			task: &models.Task{
				DueDate: &now,
				Status:  models.StatusTodo,
			},
			want: false,
		},
		{
			name: "in progress overdue returns true",
			task: &models.Task{
				DueDate: &yesterday,
				Status:  models.StatusInProgress,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOverdue(tt.task)
			if got != tt.want {
				t.Errorf("isOverdue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDueToday(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	tests := []struct {
		name string
		task *models.Task
		want bool
	}{
		{
			name: "due today returns true",
			task: &models.Task{
				DueDate: &now,
			},
			want: true,
		},
		{
			name: "due yesterday returns false",
			task: &models.Task{
				DueDate: &yesterday,
			},
			want: false,
		},
		{
			name: "due tomorrow returns false",
			task: &models.Task{
				DueDate: &tomorrow,
			},
			want: false,
		},
		{
			name: "nil due date returns false",
			task: &models.Task{
				DueDate: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDueToday(tt.task)
			if got != tt.want {
				t.Errorf("isDueToday() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDueThisWeek(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	in3Days := now.AddDate(0, 0, 3)
	in6Days := now.AddDate(0, 0, 6)
	in7Days := now.AddDate(0, 0, 7)
	in8Days := now.AddDate(0, 0, 8)

	tests := []struct {
		name string
		task *models.Task
		want bool
	}{
		{
			name: "due in 3 days returns true",
			task: &models.Task{
				DueDate: &in3Days,
			},
			want: true,
		},
		{
			name: "due in 6 days returns true",
			task: &models.Task{
				DueDate: &in6Days,
			},
			want: true,
		},
		{
			name: "due in 7 days returns false",
			task: &models.Task{
				DueDate: &in7Days,
			},
			want: false,
		},
		{
			name: "due in 8 days returns false",
			task: &models.Task{
				DueDate: &in8Days,
			},
			want: false,
		},
		{
			name: "overdue returns false",
			task: &models.Task{
				DueDate: &yesterday,
			},
			want: false,
		},
		{
			name: "due today returns true",
			task: &models.Task{
				DueDate: &now,
			},
			want: true,
		},
		{
			name: "nil due date returns false",
			task: &models.Task{
				DueDate: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDueThisWeek(tt.task)
			if got != tt.want {
				t.Errorf("isDueThisWeek() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCycleFilter(t *testing.T) {
	t.Run("string filters", func(t *testing.T) {
		values := []string{"a", "b", "c"}

		tests := []struct {
			name    string
			current string
			want    string
		}{
			{"empty to first", "", "a"},
			{"first to second", "a", "b"},
			{"second to third", "b", "c"},
			{"last wraps to empty", "c", ""},
			{"unknown wraps to empty", "x", ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := cycleFilter(tt.current, values)
				if got != tt.want {
					t.Errorf("cycleFilter(%q, values) = %q, want %q", tt.current, got, tt.want)
				}
			})
		}
	})

	t.Run("int filters", func(t *testing.T) {
		values := []int{1, 2, 3}

		tests := []struct {
			name    string
			current int
			want    int
		}{
			{"zero to first", 0, 1},
			{"first to second", 1, 2},
			{"last wraps to zero", 3, 0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := cycleFilter(tt.current, values)
				if got != tt.want {
					t.Errorf("cycleFilter(%d, values) = %d, want %d", tt.current, got, tt.want)
				}
			})
		}
	})

	t.Run("empty values", func(t *testing.T) {
		got := cycleFilter("a", []string{})
		if got != "" {
			t.Errorf("cycleFilter with empty values = %q, want empty", got)
		}
	})
}

func TestCollapseExpandVisibility(t *testing.T) {
	// Create a project with parent-child hierarchy
	project := &models.Project{
		Name: "test",
	}

	parent := &models.Task{
		ID:    "parent-id",
		Title: "Parent Task",
		Type:  models.TaskTypeTask,
	}
	child := &models.Task{
		ID:       "child-id",
		Title:    "Child Task",
		Type:     models.TaskTypeTask,
		ParentID: &parent.ID,
		Parent:   parent,
	}
	parent.Children = []*models.Task{child}
	project.Tasks = []*models.Task{parent, child}

	// Create ProjectTreeModel
	model := &ProjectTreeModel{
		project: project,
	}

	// Build the tree
	model.buildTree()

	// Verify initial state - both should be visible
	visible := model.visibleNodes()
	if len(visible) != 2 {
		t.Fatalf("Expected 2 visible nodes, got %d", len(visible))
	}

	// Find parent node
	var parentNode *TreeNode
	for _, n := range model.nodes {
		if n.Task.ID == "parent-id" {
			parentNode = n
			break
		}
	}
	if parentNode == nil {
		t.Fatal("Parent node not found")
	}

	// Verify parent is expanded by default
	if !parentNode.Expanded {
		t.Error("Parent should be expanded by default")
	}

	// Collapse the parent
	parentNode.Expanded = false
	model.updateVisibility()

	// Check visibility after collapse
	visible = model.visibleNodes()
	if len(visible) != 1 {
		t.Errorf("Expected 1 visible node after collapse, got %d", len(visible))
		for _, n := range visible {
			t.Logf("  Visible: %s (Level=%d, Expanded=%v, Visible=%v)", n.Task.Title, n.Level, n.Expanded, n.Visible)
		}
	}

	// The visible node should be the parent
	if visible[0].Task.ID != "parent-id" {
		t.Errorf("Expected parent to be visible, got %s", visible[0].Task.Title)
	}

	// Expand the parent
	parentNode.Expanded = true
	model.updateVisibility()

	// Both should be visible again
	visible = model.visibleNodes()
	if len(visible) != 2 {
		t.Errorf("Expected 2 visible nodes after expand, got %d", len(visible))
	}
}

func TestCollapseGrandchildVisibility(t *testing.T) {
	// Create a project with grandparent-parent-child hierarchy
	project := &models.Project{
		Name: "test",
	}

	grandparent := &models.Task{
		ID:    "grandparent-id",
		Title: "Grandparent Task",
		Type:  models.TaskTypeEpic,
	}
	parent := &models.Task{
		ID:       "parent-id",
		Title:    "Parent Task",
		Type:     models.TaskTypeTask,
		ParentID: &grandparent.ID,
		Parent:   grandparent,
	}
	child := &models.Task{
		ID:       "child-id",
		Title:    "Child Task",
		Type:     models.TaskTypeTask,
		ParentID: &parent.ID,
		Parent:   parent,
	}
	parent.Children = []*models.Task{child}
	grandparent.Children = []*models.Task{parent}
	project.Tasks = []*models.Task{grandparent, parent, child}

	// Create ProjectTreeModel
	model := &ProjectTreeModel{
		project: project,
	}

	// Build the tree
	model.buildTree()

	// Verify initial state - all 3 should be visible
	visible := model.visibleNodes()
	if len(visible) != 3 {
		t.Fatalf("Expected 3 visible nodes, got %d", len(visible))
	}

	// Find grandparent node
	var grandparentNode *TreeNode
	for _, n := range model.nodes {
		if n.Task.ID == "grandparent-id" {
			grandparentNode = n
			break
		}
	}
	if grandparentNode == nil {
		t.Fatal("Grandparent node not found")
	}

	// Collapse the grandparent
	grandparentNode.Expanded = false
	model.updateVisibility()

	// Only grandparent should be visible
	visible = model.visibleNodes()
	if len(visible) != 1 {
		t.Errorf("Expected 1 visible node after collapse, got %d", len(visible))
		for _, n := range visible {
			t.Logf("  Visible: %s (Level=%d, Expanded=%v, Visible=%v)", n.Task.Title, n.Level, n.Expanded, n.Visible)
		}
	}
	if visible[0].Task.ID != "grandparent-id" {
		t.Errorf("Expected grandparent to be visible, got %s", visible[0].Task.Title)
	}
}

func TestCollapseWithViewModeActive(t *testing.T) {
	// Create a project with parent and child both in Active section (empty section)
	project := &models.Project{
		Name: "test",
	}

	parent := &models.Task{
		ID:      "parent-id",
		Title:   "Parent Task",
		Type:    models.TaskTypeEpic,
		Section: "", // Active section
	}
	child := &models.Task{
		ID:       "child-id",
		Title:    "Child Task",
		Type:     models.TaskTypeTask,
		ParentID: &parent.ID,
		Parent:   parent,
		Section:  "", // Active section
	}
	parent.Children = []*models.Task{child}
	project.Tasks = []*models.Task{parent, child}

	// Create ProjectTreeModel with view mode
	model := &ProjectTreeModel{
		project:  project,
		viewMode: ViewModeActive,
	}

	// Build the tree
	model.buildTree()

	t.Logf("Initial nodes:")
	for _, n := range model.nodes {
		t.Logf("  %s: Level=%d, Expanded=%v, Visible=%v", n.Task.Title, n.Level, n.Expanded, n.Visible)
	}

	// Verify initial state - both should be visible
	visible := model.visibleNodes()
	if len(visible) != 2 {
		t.Fatalf("Expected 2 visible nodes, got %d", len(visible))
	}

	// Find parent node
	var parentNode *TreeNode
	for _, n := range model.nodes {
		if n.Task.ID == "parent-id" {
			parentNode = n
			break
		}
	}

	// Collapse the parent
	t.Log("Collapsing parent...")
	parentNode.Expanded = false
	model.updateVisibility()

	t.Logf("After collapse:")
	for _, n := range model.nodes {
		t.Logf("  %s: Level=%d, Expanded=%v, Visible=%v", n.Task.Title, n.Level, n.Expanded, n.Visible)
	}

	// Check visibility after collapse
	visible = model.visibleNodes()
	if len(visible) != 1 {
		t.Errorf("Expected 1 visible node after collapse, got %d", len(visible))
	}
}

func TestFormatDueDate(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)
	nextWeek := today.AddDate(0, 0, 7)
	nextYear := today.AddDate(1, 0, 0)

	tests := []struct {
		name        string
		task        *models.Task
		shouldEmpty bool
		contains    string
	}{
		{
			name: "no due date returns empty",
			task: &models.Task{
				DueDate: nil,
				Status:  models.StatusTodo,
			},
			shouldEmpty: true,
		},
		{
			name: "completed task returns empty",
			task: &models.Task{
				DueDate: &today,
				Status:  models.StatusDone,
			},
			shouldEmpty: true,
		},
		{
			name: "cancelled task returns empty",
			task: &models.Task{
				DueDate: &today,
				Status:  models.StatusCancelled,
			},
			shouldEmpty: true,
		},
		{
			name: "due today contains Today",
			task: &models.Task{
				DueDate: &today,
				Status:  models.StatusTodo,
			},
			contains: "Today",
		},
		{
			name: "due tomorrow contains Tomorrow",
			task: &models.Task{
				DueDate: &tomorrow,
				Status:  models.StatusTodo,
			},
			contains: "Tomorrow",
		},
		{
			name: "overdue shows date",
			task: &models.Task{
				DueDate: &yesterday,
				Status:  models.StatusTodo,
			},
			contains: yesterday.Format("Jan 2"),
		},
		{
			name: "next week shows date",
			task: &models.Task{
				DueDate: &nextWeek,
				Status:  models.StatusInProgress,
			},
			contains: nextWeek.Format("Jan 2"),
		},
		{
			name: "next year includes year",
			task: &models.Task{
				DueDate: &nextYear,
				Status:  models.StatusTodo,
			},
			contains: nextYear.Format("'06"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDueDate(tt.task)
			if tt.shouldEmpty {
				if result != "" {
					t.Errorf("formatDueDate() = %q, expected empty", result)
				}
				return
			}
			if result == "" {
				t.Errorf("formatDueDate() returned empty, expected to contain %q", tt.contains)
			}
		})
	}
}

func TestGetHiddenSectionsForViewMode(t *testing.T) {
	tests := []struct {
		name     string
		viewMode ViewMode
		expected []string
	}{
		{
			name:     "active mode hides special sections",
			viewMode: ViewModeActive,
			expected: []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionSomeday},
		},
		{
			name:     "next mode hides other sections",
			viewMode: ViewModeNext,
			expected: []string{models.SectionInbox, models.SectionWaiting, models.SectionSomeday, "Active"},
		},
		{
			name:     "inbox mode",
			viewMode: ViewModeInbox,
			expected: []string{models.SectionNext, models.SectionWaiting, models.SectionSomeday, "Active"},
		},
		{
			name:     "waiting mode",
			viewMode: ViewModeWaiting,
			expected: []string{models.SectionInbox, models.SectionNext, models.SectionSomeday, "Active"},
		},
		{
			name:     "someday mode",
			viewMode: ViewModeSomeday,
			expected: []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, "Active"},
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

	note := &models.Task{
		ID:       "note",
		Section:  models.SectionInbox,
		Type:     models.TaskTypeNote, // Notes should be excluded
		Children: []*models.Task{},
	}

	childEmptySection := &models.Task{
		ID:       "empty-section",
		Section:  "", // Empty section should be normalized to "Active"
		Type:     models.TaskTypeTask,
		Children: []*models.Task{},
	}

	parentTask.Children = []*models.Task{child1, child2, child3, note, childEmptySection}
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
				models.SectionInbox:   1, // child2 (note is excluded)
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

func TestGetVisibleSiblings(t *testing.T) {
	// Create a project with siblings
	parent := &models.Task{
		ID:    "parent",
		Title: "Parent",
		Type:  models.TaskTypeEpic,
	}

	child1 := &models.Task{
		ID:       "child1",
		Title:    "First Child",
		Type:     models.TaskTypeTask,
		Section:  "",
		ParentID: &parent.ID,
		Parent:   parent,
	}

	child2 := &models.Task{
		ID:       "child2",
		Title:    "Second Child",
		Type:     models.TaskTypeTask,
		Section:  models.SectionInbox, // Different section
		ParentID: &parent.ID,
		Parent:   parent,
	}

	child3 := &models.Task{
		ID:       "child3",
		Title:    "Third Child",
		Type:     models.TaskTypeTask,
		Section:  "",
		ParentID: &parent.ID,
		Parent:   parent,
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
