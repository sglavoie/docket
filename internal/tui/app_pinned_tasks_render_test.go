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
	task, err := models.NewTask("id-cjk", 1, models.TaskTypeTask, "日本語タイトルテスト")
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
