package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestComputeBadgeWidthsEmpty(t *testing.T) {
	if got := ComputeBadgeWidths(nil, BadgeWidthOpts{IncludeStatus: true}); got != nil {
		t.Errorf("nil tasks should return nil layout, got %+v", got)
	}
	if got := ComputeBadgeWidths([]*models.Task{}, BadgeWidthOpts{IncludeStatus: true}); got != nil {
		t.Errorf("empty tasks should return nil layout, got %+v", got)
	}
}

func TestComputeBadgeWidthsAllNil(t *testing.T) {
	if got := ComputeBadgeWidths([]*models.Task{nil, nil, nil}, BadgeWidthOpts{
		IncludeStatus: true,
	}); got != nil {
		t.Errorf("all-nil tasks should return nil layout, got %+v", got)
	}
}

func TestComputeBadgeWidthsMixed(t *testing.T) {
	t1 := &models.Task{
		Title:         "alpha",
		WorkflowState: models.StateDoing,
		Priority:      2,
	}
	t2 := &models.Task{
		Title:         "beta",
		WorkflowState: models.StateBacklog,
		Priority:      models.PriorityLow,
	}
	t3 := &models.Task{
		Title:         "gamma",
		WorkflowState: models.StateWaiting,
	}

	cl := ComputeBadgeWidths([]*models.Task{t1, t2, nil, t3}, BadgeWidthOpts{
		IncludeStatus: true,
	})
	if cl == nil {
		t.Fatal("expected non-nil layout for mixed tasks")
	}

	if !cl.HasPriority {
		t.Error("HasPriority should be true (t1 has priority 2)")
	}
	if cl.PriorityWidth != 1 {
		t.Errorf("PriorityWidth = %d, want 1", cl.PriorityWidth)
	}
	if !cl.HasStatus || cl.StatusWidth <= 0 {
		t.Errorf("status column should be populated, got Has=%v width=%d", cl.HasStatus, cl.StatusWidth)
	}
	if cl.ProjectWidth != 0 {
		t.Error("ProjectWidth should remain zero when ProjectNameFor is nil")
	}
}

func TestComputeBadgeWidthsAllOptional(t *testing.T) {
	// No options enabled → only priority is measured.
	t1 := &models.Task{Priority: 1, WorkflowState: models.StateBacklog}
	cl := ComputeBadgeWidths([]*models.Task{t1}, BadgeWidthOpts{})
	if cl == nil {
		t.Fatal("expected non-nil layout")
	}
	if !cl.HasPriority {
		t.Error("HasPriority should be true regardless of opts when priority is shown")
	}
	if cl.HasStatus || cl.ProjectWidth != 0 {
		t.Errorf("optional columns should remain unset when not requested, got %+v", cl)
	}
}

func TestComputeBadgeWidthsProject(t *testing.T) {
	t1 := &models.Task{ID: "1", Title: "one", WorkflowState: models.StateBacklog}
	t2 := &models.Task{ID: "2", Title: "two", WorkflowState: models.StateBacklog}

	names := map[string]string{
		"1": "short",
		"2": "much-longer-project-name-that-should-be-truncated",
	}

	cl := ComputeBadgeWidths([]*models.Task{t1, t2}, BadgeWidthOpts{
		ProjectNameFor: func(task *models.Task) string { return names[task.ID] },
		MaxProjectLen:  12,
	})
	if cl == nil {
		t.Fatal("expected non-nil layout")
	}
	// Truncated to 12, wrapped in "[]" → visible width 14.
	if cl.ProjectWidth != 14 {
		t.Errorf("ProjectWidth = %d, want 14 ('[' + 12 + ']')", cl.ProjectWidth)
	}
}

func TestComputeBadgeWidthsProjectEmptyName(t *testing.T) {
	t1 := &models.Task{ID: "1", WorkflowState: models.StateBacklog}
	cl := ComputeBadgeWidths([]*models.Task{t1}, BadgeWidthOpts{
		ProjectNameFor: func(*models.Task) string { return "" },
	})
	if cl == nil {
		t.Fatal("expected non-nil layout")
	}
	if cl.ProjectWidth != 0 {
		t.Errorf("ProjectWidth = %d, want 0", cl.ProjectWidth)
	}
}
