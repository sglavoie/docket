package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// TestProjectTreeDenseRowKeepsUrgencyHeroInline confirms the dense-row
// hierarchy on the project tree: overdue and due-today urgency text remains
// inline (it is the only date-driven primary anchor), while future-due dates
// drop out of the row and move to the metadata strip.
func TestProjectTreeDenseRowKeepsUrgencyHeroInline(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	today := now
	tomorrow := now.AddDate(0, 0, 1)

	overdue := newTestProjectTreeTask(t, "Overdue urgency row")
	overdue.DueDate = &yesterday
	dueToday := newTestProjectTreeTask(t, "Due today urgency row")
	dueToday.DueDate = &today
	dueTomorrow := newTestProjectTreeTask(t, "Future due row")
	dueTomorrow.DueDate = &tomorrow

	project := newTestProjectTreeProject(t, "Urgency", []*models.Task{overdue, dueToday, dueTomorrow})
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	m.viewMode = ViewModeAll
	m.updateVisibility()
	m.SetSize(120, 24)

	overdueLine := ansi.Strip(m.renderNodeWithWidth(m.findNodeByTaskID(overdue.ID), false, false, 120))
	if !strings.Contains(overdueLine, yesterday.Format("Jan 2")) {
		t.Fatalf("overdue dense row should keep overdue date inline, got %q", overdueLine)
	}

	dueTodayLine := ansi.Strip(m.renderNodeWithWidth(m.findNodeByTaskID(dueToday.ID), false, false, 120))
	if !strings.Contains(dueTodayLine, "Today") {
		t.Fatalf("due-today dense row should keep \"Today\" inline, got %q", dueTodayLine)
	}

	tomorrowLine := ansi.Strip(m.renderNodeWithWidth(m.findNodeByTaskID(dueTomorrow.ID), false, false, 120))
	if strings.Contains(tomorrowLine, tomorrow.Format("Jan 2")) ||
		strings.Contains(tomorrowLine, "Tomorrow") {
		t.Fatalf("future-due dense row must not surface due text inline, got %q", tomorrowLine)
	}
}

// TestProjectTreeDenseRowOmitsSecondaryMetadata verifies the dense row drops
// secondary indicators (tags, description, checklist, comments, recurrence,
// dependencies, progress, etc.) in favor of the selected metadata strip.
func TestProjectTreeDenseRowOmitsSecondaryMetadata(t *testing.T) {
	snoozedUntil := time.Now().AddDate(0, 0, 3)
	task := newTestProjectTreeTask(t, "Rich task with secondary metadata")
	task.Priority = 2
	task.Tags = []string{"alpha", "beta"}
	task.Description = "A descriptive note that should live in the strip, not the row."
	task.Checklist = []models.ChecklistItem{
		{Text: "one", Done: true},
		{Text: "two", Done: false},
	}
	task.Comments = []*models.Comment{{Text: "A comment"}}
	task.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	task.SnoozedUntil = &snoozedUntil

	project := newTestProjectTreeProject(t, "Rich", []*models.Task{task})
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	m.viewMode = ViewModeSnoozed
	m.updateVisibility()
	m.SetSize(140, 24)

	line := ansi.Strip(m.renderNodeWithWidth(m.findNodeByTaskID(task.ID), false, false, 140))

	if strings.Contains(line, "#alpha") || strings.Contains(line, "#beta") {
		t.Fatalf("dense row must not carry tags inline, got %q", line)
	}
	if strings.Contains(line, ansi.Strip(renderDescriptionIndicator(task))) {
		t.Fatalf("dense row must not carry description indicator inline, got %q", line)
	}
	if strings.Contains(line, ansi.Strip(renderChecklistIndicator(task))) {
		t.Fatalf("dense row must not carry checklist indicator inline, got %q", line)
	}
	if strings.Contains(line, ansi.Strip(renderCommentIndicator(task))) {
		t.Fatalf("dense row must not carry comment indicator inline, got %q", line)
	}
	if strings.Contains(line, "↻") {
		t.Fatalf("dense row must not carry recurrence indicator inline, got %q", line)
	}
	if strings.Contains(line, "zzz") {
		t.Fatalf("dense row must not carry snooze indicator inline, got %q", line)
	}
	if !strings.Contains(line, fmt.Sprintf("#%d", task.ShortID)) {
		t.Fatalf("dense row should keep short ID inline, got %q", line)
	}
}

// TestProjectTreeSelectedStripCarriesMovedFields ensures the moved secondary
// metadata appears in the selected-task metadata strip when the cursor sits on
// the rich task.
func TestProjectTreeSelectedStripCarriesMovedFields(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	snoozedUntil := time.Now().AddDate(0, 0, 3)
	task := newTestProjectTreeTask(t, "Selected row carries strip metadata")
	task.Priority = 2
	task.Tags = []string{"alpha", "beta"}
	task.Description = "Has detail text that the strip should surface."
	task.Checklist = []models.ChecklistItem{
		{Text: "one", Done: true},
		{Text: "two", Done: false},
	}
	task.SnoozedUntil = &snoozedUntil

	project := newTestProjectTreeProject(t, "Strip Project", []*models.Task{task})
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	m.viewMode = ViewModeSnoozed
	m.updateVisibility()
	const width = 140
	m.SetSize(width, 24)

	visible := m.visibleNodes()
	for i, node := range visible {
		if node.Task.ID == task.ID {
			m.cursor = i
			break
		}
	}

	rendered := m.renderTreeContent(width, true)
	plain := ansi.Strip(rendered)

	if !strings.Contains(plain, "#alpha #beta") {
		t.Fatalf("metadata strip should surface tag metadata, got:\n%s", plain)
	}
	if !strings.Contains(plain, ansi.Strip(renderDescriptionIndicator(task))) {
		t.Fatalf("metadata strip should surface description indicator, got:\n%s", plain)
	}
	if !strings.Contains(plain, ansi.Strip(renderChecklistIndicator(task))) {
		t.Fatalf("metadata strip should surface checklist indicator, got:\n%s", plain)
	}
	if !strings.Contains(plain, "zzz "+snoozedUntil.Format("Jan 2")) {
		t.Fatalf("metadata strip should surface snooze metadata, got:\n%s", plain)
	}
}

// TestProjectTreeDenseRowWidthStableAcrossSecondaryMetadata locks in the
// hierarchy's stability promise: varying secondary metadata across siblings
// must not change rendered row widths because none of those fields participate
// in the inline format. Mirrors the long-stage test by measuring rendered
// lines inside the View output so card padding applies uniformly.
func TestProjectTreeDenseRowWidthStableAcrossSecondaryMetadata(t *testing.T) {
	titles := []string{"Sibling task A", "Sibling task B", "Sibling task C", "Sibling task D"}
	tasks := make([]*models.Task, 0, len(titles))
	for _, title := range titles {
		tasks = append(tasks, newTestProjectTreeTask(t, title))
	}
	tasks[1].Tags = []string{"alpha", "beta", "gamma"}
	tasks[2].Description = "A long enough description to pressure the row width."
	tasks[3].Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 5}

	project := newTestProjectTreeProject(t, "Stable", tasks)
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	m.viewMode = ViewModeAll
	m.updateVisibility()
	m.SetSize(120, 24)

	plain := ansi.Strip(m.View().Content)
	widths := make(map[int]int)
	for _, line := range strings.Split(plain, "\n") {
		for _, title := range titles {
			if strings.Contains(line, title) {
				widths[lipgloss.Width(line)]++
				break
			}
		}
	}
	if len(widths) == 0 {
		t.Fatalf("expected at least one sibling row, got no matches in:\n%s", plain)
	}
	if len(widths) != 1 {
		t.Fatalf("expected uniform dense-row widths across varied secondary metadata, got %v", widths)
	}
}

// TestProjectTreeCompletedAndCanceledRowsAreQuiet guards the quiet completed/
// canceled contract: rows for done or canceled tasks must not surface any
// urgency hero text inline even when the underlying due date is overdue or due
// today. Those rows are noise, not action.
func TestProjectTreeCompletedAndCanceledRowsAreQuiet(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	today := time.Now()

	done := newTestProjectTreeTask(t, "Completed overdue task")
	done.Status = models.StatusDone
	done.DueDate = &yesterday

	canceled := newTestProjectTreeTask(t, "Canceled overdue task")
	canceled.Status = models.StatusCanceled
	canceled.DueDate = &yesterday

	doneToday := newTestProjectTreeTask(t, "Completed closed task")
	doneToday.Status = models.StatusDone
	doneToday.DueDate = &today

	canceledToday := newTestProjectTreeTask(t, "Canceled closed task")
	canceledToday.Status = models.StatusCanceled
	canceledToday.DueDate = &today

	project := newTestProjectTreeProject(t, "Quiet", []*models.Task{done, canceled, doneToday, canceledToday})
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	m.viewMode = ViewModeAll
	m.updateVisibility()
	const width = 120
	m.SetSize(width, 24)

	for _, task := range []*models.Task{done, canceled, doneToday, canceledToday} {
		node := m.findNodeByTaskID(task.ID)
		if node == nil {
			t.Fatalf("expected node for task %q to exist in view mode all", task.Title)
		}
		line := ansi.Strip(m.renderNodeWithWidth(node, false, false, width))
		if strings.Contains(line, "Overdue") {
			t.Fatalf("quiet %s row should not carry overdue inline, got %q", task.Status, line)
		}
		if strings.Contains(line, yesterday.Format("Jan 2")) {
			t.Fatalf("quiet %s row should not surface due text inline, got %q", task.Status, line)
		}
		if strings.Contains(line, "Today") {
			t.Fatalf("quiet %s row should not carry due-today inline, got %q", task.Status, line)
		}
	}
}
