package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestWeeklyReviewTaskRowsUseProjectColor(t *testing.T) {
	previousTheme := activeThemeName
	t.Cleanup(func() { activeThemeName = previousTheme })
	activeThemeName = "dark"

	runtime := newTestConfig()
	project := &models.Project{ID: "project-color", Name: "Alpha", ColorSlot: 4}
	task := &models.Task{
		ID:            "task-color",
		Title:         "Keep the aggregate views consistent",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
	}
	wantBadge := projectColorStyle(project).Render("[Alpha]")

	weeklyReview := NewWeeklyReviewModel(runtime, nil)
	weeklyReview.SetSize(120, 24)

	line := weeklyReview.renderTaskLine(WeeklyReviewTask{Task: task, Project: project}, WeeklySectionStaleTasks, false, 60, nil, 0)
	if !strings.Contains(line, wantBadge) {
		t.Fatalf("Weekly Review task row did not inherit the project identity badge %q: %q", wantBadge, line)
	}
	if got := lipgloss.Width(line); got > 120 {
		t.Fatalf("Weekly Review task row width = %d, want <= 120: %q", got, ansi.Strip(line))
	}
}

func TestWeeklyReviewProjectSummaryColorSelectionAndUnicodeWidth(t *testing.T) {
	previousTheme := activeThemeName
	t.Cleanup(func() { activeThemeName = previousTheme })
	activeThemeName = "dark"

	project := &models.Project{ID: "weekly-project", Name: "界面修复计划", ColorSlot: 9}
	unassigned := *project
	unassigned.ColorSlot = 0
	entry := WeeklyReviewProject{Project: project, InboxCount: 7, OverdueCount: 3, NoActivityDays: 9}
	fallbackEntry := entry
	fallbackEntry.Project = &unassigned

	m := &WeeklyReviewModel{}
	m.width = 60
	unselected := m.renderProjectLine(entry, WeeklySectionInboxBacklog, false)
	if want := projectColorStyle(project).Render(project.Name); !strings.Contains(unselected, want) {
		t.Fatalf("Weekly Review summary did not color the project name %q: %q", want, unselected)
	}

	selected := m.renderProjectLine(entry, WeeklySectionInboxBacklog, true)
	selectedFallback := m.renderProjectLine(fallbackEntry, WeeklySectionInboxBacklog, true)
	if selected != selectedFallback {
		t.Fatalf("selected Weekly Review summary changed with ColorSlot:\ncolored=%q\nfallback=%q", selected, selectedFallback)
	}

	m.width = 30
	for _, isSelected := range []bool{false, true} {
		line := m.renderProjectLine(entry, WeeklySectionInboxBacklog, isSelected)
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("Weekly Review Unicode summary width = %d, want <= %d: %q", got, m.width, ansi.Strip(line))
		}
	}
}
