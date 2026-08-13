package tui

import (
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TestRenderPinnedTasksBar_CJKTitleVisualWidth verifies that titles with
// double-width (CJK) runes are truncated/padded by visual width, not rune
// count. A bug used rune count which caused CJK-heavy titles to overflow the
// allotted column.
func TestRenderPinnedTasksBar_CJKTitleVisualWidth(t *testing.T) {
	project, err := models.NewProject("p1", "Proj", "/tmp/p1")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	pinnedAt := time.Now()
	task, err := models.NewGenericTask("id-cjk", 1, "日本語タイトルテスト")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Pinned = true
	task.PinnedAt = &pinnedAt
	project.Tasks = []*models.Task{task}

	m := newTestAppModel()
	setTestAppProjects(m, project)
	m.width = 80

	const barWidth = 40
	bar := m.renderPinnedTasksBar(barWidth)
	if bar == "" {
		t.Fatal("expected non-empty pinned bar")
	}

	if got := lipgloss.Width(bar); got > barWidth {
		t.Fatalf("pinned bar visual width %d exceeds requested %d: %q", got, barWidth, bar)
	}
}

func TestCollectPinnedTasksUsesVisibleNewestFirstProjection(t *testing.T) {
	project, err := models.NewProject("p1", "Proj", "/tmp/p1")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	now := time.Now()
	older := now.Add(-2 * time.Hour)
	newer := now.Add(-time.Hour)
	tomorrow := now.AddDate(0, 0, 1)
	oldTask := &models.Task{ID: "old", Title: "old", ShortID: 1, Pinned: true, PinnedAt: &older, WorkflowState: models.StateBacklog}
	newTask := &models.Task{ID: "new", Title: "new", ShortID: 2, Pinned: true, PinnedAt: &newer, WorkflowState: models.StateBacklog}
	deferred := &models.Task{ID: "deferred", Title: "deferred", ShortID: 3, Pinned: true, PinnedAt: &newer, DeferredUntil: &tomorrow, WorkflowState: models.StateBacklog}
	done := &models.Task{ID: "done", Title: "done", ShortID: 4, Pinned: true, PinnedAt: &newer, WorkflowState: models.StateDone}
	project.Tasks = []*models.Task{oldTask, deferred, done, newTask}

	m := newTestAppModel()
	setTestAppProjects(m, project)
	got := m.collectPinnedTasks()
	if len(got) != 2 || got[0].Task != newTask || got[1].Task != oldTask {
		t.Fatalf("pinned tasks = %+v, want visible open tasks newest first", got)
	}
}
