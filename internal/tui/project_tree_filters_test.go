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
