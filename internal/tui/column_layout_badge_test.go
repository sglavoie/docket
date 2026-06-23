package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

func TestEmptyBadge(t *testing.T) {
	if got := EmptyBadge(0); got != "" {
		t.Errorf("EmptyBadge(0) = %q, want empty string", got)
	}
	if got := EmptyBadge(-3); got != "" {
		t.Errorf("EmptyBadge(-3) = %q, want empty string", got)
	}
	if got := EmptyBadge(4); got != "    " {
		t.Errorf("EmptyBadge(4) = %q, want 4 spaces", got)
	}
}

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
		IncludeType:   true,
	}); got != nil {
		t.Errorf("all-nil tasks should return nil layout, got %+v", got)
	}
}

func TestComputeBadgeWidthsMixed(t *testing.T) {
	t1 := &models.Task{
		Title:    "alpha",
		Type:     models.TaskTypeTask,
		Status:   models.StatusInProgress,
		Priority: 2,
		Stage:    "design",
	}
	t2 := &models.Task{
		Title:    "beta",
		Type:     models.TaskTypeEpic,
		Status:   models.StatusTodo,
		Priority: 5,
		Stage:    "",
	}
	t3 := &models.Task{
		Title:  "gamma",
		Type:   models.TaskTypeBug,
		Status: models.StatusBlocked,
	}

	cl := ComputeBadgeWidths([]*models.Task{t1, t2, nil, t3}, BadgeWidthOpts{
		IncludeStatus: true,
		IncludeType:   true,
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
	if !cl.HasType || cl.TypeWidth <= 0 {
		t.Errorf("type column should be populated, got Has=%v width=%d", cl.HasType, cl.TypeWidth)
	}
	if cl.HasProject {
		t.Error("HasProject should remain false when ProjectNameFor is nil")
	}
}

func TestComputeBadgeWidthsHideLowPriority(t *testing.T) {
	t1 := &models.Task{Priority: 5, Status: models.StatusTodo, Type: models.TaskTypeTask}
	cl := ComputeBadgeWidths([]*models.Task{t1}, BadgeWidthOpts{
		HideLowPriority: true,
	})
	if cl == nil {
		t.Fatal("expected non-nil layout")
	}
	if cl.HasPriority {
		t.Error("HasPriority should be false when low priority is hidden and all tasks are low priority")
	}
	if cl.PriorityWidth != 0 {
		t.Errorf("PriorityWidth = %d, want 0", cl.PriorityWidth)
	}
}

func TestComputeBadgeWidthsAllOptional(t *testing.T) {
	// No options enabled → only priority is measured.
	t1 := &models.Task{Priority: 1, Status: models.StatusTodo, Type: models.TaskTypeTask, Stage: "anything"}
	cl := ComputeBadgeWidths([]*models.Task{t1}, BadgeWidthOpts{})
	if cl == nil {
		t.Fatal("expected non-nil layout")
	}
	if !cl.HasPriority {
		t.Error("HasPriority should be true regardless of opts when priority is shown")
	}
	if cl.HasStatus || cl.HasType || cl.HasProject {
		t.Errorf("optional columns should remain unset when not requested, got %+v", cl)
	}
}

func TestComputeBadgeWidthsPriorityEmojiWidth(t *testing.T) {
	icons.SetUseEmoji(true)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	t1 := &models.Task{Priority: 1, Status: models.StatusTodo, Type: models.TaskTypeTask}
	cl := ComputeBadgeWidths([]*models.Task{t1}, BadgeWidthOpts{})
	if cl == nil {
		t.Fatal("expected non-nil layout")
	}
	if !cl.HasPriority {
		t.Error("HasPriority should be true")
	}
	if cl.PriorityWidth != 2 {
		t.Errorf("PriorityWidth = %d, want 2 (emoji priority icons are visual width 2)", cl.PriorityWidth)
	}
}

func TestComputeBadgeWidthsProject(t *testing.T) {
	t1 := &models.Task{ID: "1", Title: "one", Status: models.StatusTodo, Type: models.TaskTypeTask}
	t2 := &models.Task{ID: "2", Title: "two", Status: models.StatusTodo, Type: models.TaskTypeTask}

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
	if !cl.HasProject {
		t.Error("HasProject should be true when ProjectNameFor returns non-empty names")
	}
	// Truncated to 12, wrapped in "[]" → visible width 14.
	if cl.ProjectWidth != 14 {
		t.Errorf("ProjectWidth = %d, want 14 ('[' + 12 + ']')", cl.ProjectWidth)
	}
}

func TestComputeBadgeWidthsProjectEmptyName(t *testing.T) {
	t1 := &models.Task{ID: "1", Status: models.StatusTodo, Type: models.TaskTypeTask}
	cl := ComputeBadgeWidths([]*models.Task{t1}, BadgeWidthOpts{
		ProjectNameFor: func(*models.Task) string { return "" },
	})
	if cl == nil {
		t.Fatal("expected non-nil layout")
	}
	if cl.HasProject {
		t.Error("HasProject should remain false when project name lookup returns empty")
	}
	if cl.ProjectWidth != 0 {
		t.Errorf("ProjectWidth = %d, want 0", cl.ProjectWidth)
	}
}
