package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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
