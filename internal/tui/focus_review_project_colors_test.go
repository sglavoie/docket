package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestDailyReviewWeeklyReviewTaskRowsUseProjectColor(t *testing.T) {
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
	item := CrossProjectTask{Task: task, Project: project}
	wantBadge := projectColorStyle(project).Render("[Alpha]")

	dailyReview := NewDailyReviewModel(runtime, nil)
	dailyReview.SetSize(120, 24)

	weeklyReview := NewWeeklyReviewModel(runtime, nil)
	weeklyReview.SetSize(120, 24)

	tests := []struct {
		name   string
		render func() string
	}{
		{
			name: "Daily Review",
			render: func() string {
				return dailyReview.renderTaskLine(item, SectionInProgress, false, 60, nil, 0)
			},
		},
		{
			name: "Weekly Review task",
			render: func() string {
				return weeklyReview.renderTaskLine(WeeklyReviewTask{Task: task, Project: project}, WeeklySectionStaleTasks, false, 60, nil, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := tt.render()
			if !strings.Contains(line, wantBadge) {
				t.Fatalf("%s row did not inherit the project identity badge %q: %q", tt.name, wantBadge, line)
			}
			if got := lipgloss.Width(line); got > 120 {
				t.Fatalf("%s row width = %d, want <= 120: %q", tt.name, got, ansi.Strip(line))
			}
		})
	}
}

func TestFocusBoardProjectColorPreservesRowSemantics(t *testing.T) {
	previousTheme := activeThemeName
	t.Cleanup(func() { activeThemeName = previousTheme })
	activeThemeName = "dark"

	project := &models.Project{ID: "focus-project", Name: "Focus", ColorSlot: 6}
	unassigned := *project
	unassigned.ColorSlot = 0
	task := &models.Task{
		ID:            "focus-task",
		Title:         "Focus board identity",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
	}
	m := newTestFocusBoard()
	m.SetSize(100, 24)
	m.tabs.Active = boardTabToday
	m.carriedOverIDs[taskIdentityFor(task, project)] = true

	row := boardRow{Task: task, Project: project, Source: project.Name}
	wantBadge := projectColorStyle(project).Render("[Focus]")
	normal := m.renderRow(row, false, 60, "1.", false, nil)
	if !strings.Contains(normal, wantBadge) {
		t.Fatalf("Focus Board row did not use the project identity badge %q: %q", wantBadge, normal)
	}
	if !strings.Contains(ansi.Strip(normal), "(carried)") {
		t.Fatalf("Focus Board color change dropped the carried-over marker: %q", ansi.Strip(normal))
	}

	// A selected row is intentionally restyled as one high-contrast unit, so
	// its result must be independent of the stored project slot.
	selected := m.renderRow(row, true, 60, "1.", false, nil)
	selectedFallback := m.renderRow(boardRow{Task: task, Project: &unassigned, Source: unassigned.Name}, true, 60, "1.", false, nil)
	if selected != selectedFallback {
		t.Fatalf("selected Focus Board row changed with ColorSlot:\ncolored=%q\nfallback=%q", selected, selectedFallback)
	}

	// Dimmed and completed rows likewise retain their whole-row quiet styling.
	dimmedTask := *task
	dimmedTask.Dimmed = true
	dimmed := m.renderRow(boardRow{Task: &dimmedTask, Project: project, Source: project.Name}, false, 60, "", false, nil)
	dimmedFallback := m.renderRow(boardRow{Task: &dimmedTask, Project: &unassigned, Source: unassigned.Name}, false, 60, "", false, nil)
	if dimmed != dimmedFallback {
		t.Fatalf("dimmed Focus Board row changed with ColorSlot:\ncolored=%q\nfallback=%q", dimmed, dimmedFallback)
	}

	completedTask := *task
	completedTask.WorkflowState = models.StateDone
	completed := m.renderRow(boardRow{Task: &completedTask, Project: project, Source: project.Name}, false, 60, "", false, nil)
	completedFallback := m.renderRow(boardRow{Task: &completedTask, Project: &unassigned, Source: unassigned.Name}, false, 60, "", false, nil)
	if completed != completedFallback {
		t.Fatalf("completed Focus Board row changed with ColorSlot:\ncolored=%q\nfallback=%q", completed, completedFallback)
	}

	// Urgency heroes demote supporting badges, but project identity is not an
	// urgency badge and remains visible.
	heroTask := *task
	heroTask.Priority = 1
	heroSegments := m.buildBoardRowSegments(
		boardRow{Task: &heroTask, Project: project, Source: project.Name},
		false,
		60,
		"",
		false,
		nil,
	)
	if heroSegments.projectBadge != wantBadge {
		t.Fatalf("hero-demoted Focus Board row lost project color: got %q, want %q", heroSegments.projectBadge, wantBadge)
	}
	if got, want := heroSegments.statusIcon, demoteBadge(GetStateBadge(heroTask.State())); got != want {
		t.Fatalf("hero-demoted status = %q, want %q", got, want)
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

func TestRecentActivityColorsOnlyGlobalRowsAndPreviews(t *testing.T) {
	previousTheme := activeThemeName
	t.Cleanup(func() { activeThemeName = previousTheme })
	activeThemeName = "dark"

	runtime := newTestConfig()
	project := &models.Project{ID: "activity-project", Name: "Activity", ColorSlot: 12}
	unassigned := *project
	unassigned.ColorSlot = 0
	task := &models.Task{
		ID:            "activity-task",
		ShortID:       42,
		Title:         "Review recent changes",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
	}
	item := CrossProjectTask{Task: task, Project: project}
	fallbackItem := CrossProjectTask{Task: task, Project: &unassigned}

	global := &RecentActivityModel{runtime: runtime, queueIDs: map[TaskIdentity]bool{}, dailyPlanIDs: map[TaskIdentity]bool{}}
	global.SetSize(120, 24)
	globalLine := global.renderTaskLine(item, false, nil, 0)
	if want := projectColorStyle(project).Render("[Activity]"); !strings.Contains(globalLine, want) {
		t.Fatalf("global Recent Activity row did not use project identity badge %q: %q", want, globalLine)
	}

	local := &RecentActivityModel{
		runtime:      runtime,
		queueIDs:     map[TaskIdentity]bool{},
		dailyPlanIDs: map[TaskIdentity]bool{},
		projectScope: project,
	}
	local.SetSize(120, 24)
	localLine := local.renderTaskLine(item, false, nil, 0)
	localFallback := local.renderTaskLine(fallbackItem, false, nil, 0)
	if localLine != localFallback {
		t.Fatalf("project-scoped Recent Activity row changed with ColorSlot:\ncolored=%q\nfallback=%q", localLine, localFallback)
	}
	if strings.Contains(ansi.Strip(localLine), "[Activity]") {
		t.Fatalf("project-scoped Recent Activity unexpectedly rendered a project badge: %q", ansi.Strip(localLine))
	}

	globalPreview := NewRecentActivityPreviewModel(item, nil).renderContent()
	if want := projectColorStyle(project).Bold(true).Render(project.Name); !strings.Contains(globalPreview, want) {
		t.Fatalf("global Recent Activity preview did not color the project name %q: %q", want, globalPreview)
	}

	localPreview := NewRecentActivityPreviewModel(item, project).renderContent()
	localPreviewFallback := NewRecentActivityPreviewModel(fallbackItem, &unassigned).renderContent()
	if localPreview != localPreviewFallback {
		t.Fatalf("project-scoped Recent Activity preview changed with ColorSlot:\ncolored=%q\nfallback=%q", localPreview, localPreviewFallback)
	}
	if want := BoldStyle.Render(project.Name); !strings.Contains(localPreview, want) {
		t.Fatalf("project-scoped Recent Activity preview lost its existing bold project name %q: %q", want, localPreview)
	}
}
