package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// newDenseRowFocusBoard returns a FocusBoardModel seeded for dense-row rendering
// tests. The caller still needs to assign m.filtered, m.cursor, and width.
func newDenseRowFocusBoard(t *testing.T) *FocusBoardModel {
	t.Helper()
	m := newTestFocusBoard()
	m.tabs = TabState{Active: boardTabToday, Names: focusBoardTabNames}
	m.SetSize(160, 24)
	return m
}

func newDenseRowProject(t *testing.T) *models.Project {
	t.Helper()
	project, err := models.NewProject("dense-focus", "dnf", "/tmp/test-dense-focus")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	return project
}

func newDenseRowTask(t *testing.T, id string, shortID int, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.WorkflowState = models.StateReady
	return task
}

// TestFocusBoardDenseRowShowsParkedSnippet verifies the Today cockpit surfaces
// the parked marker inline, matching the shared list-view treatment. The focus
// board has no sidebar, so the marker always renders in the row.
func TestFocusBoardDenseRowShowsParkedSnippet(t *testing.T) {
	task := newDenseRowTask(t, "parked-focus-id", 9, "Parked focus row")
	task.ParkedNote = "waiting on review"

	project := newDenseRowProject(t)
	m := newDenseRowFocusBoard(t)

	row := boardRow{Task: task, Project: project, Source: project.Name}
	line := ansi.Strip(m.renderRow(row, false, 100, "1.", false, nil))
	if !strings.Contains(line, "⏸ waiting on review") {
		t.Fatalf("focus board dense row should show parked snippet inline, got %q", line)
	}
}

// TestFocusBoardDenseRowShowsParkedFallbackForBlankNote verifies the fallback
// parked marker renders when there is no note.
func TestFocusBoardDenseRowShowsParkedFallbackForBlankNote(t *testing.T) {
	parkedAt := time.Now()
	task := newDenseRowTask(t, "parked-focus-blank-id", 10, "Blank parked focus row")
	task.ParkedAt = &parkedAt

	project := newDenseRowProject(t)
	m := newDenseRowFocusBoard(t)

	row := boardRow{Task: task, Project: project, Source: project.Name}
	line := ansi.Strip(m.renderRow(row, false, 100, "1.", false, nil))
	if !strings.Contains(line, "⏸ parked") {
		t.Fatalf("focus board dense row should show fallback parked snippet inline, got %q", line)
	}
}

// TestFocusBoardDenseRowKeepsUrgencyHeroInline confirms the dense-row hierarchy
// on the focus board: overdue and due-today urgency text remains inline (the
// only date-driven primary anchor), while future-due dates drop out of the row
// and move to the metadata strip.
func TestFocusBoardDenseRowKeepsUrgencyHeroInline(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	today := now
	tomorrow := now.AddDate(0, 0, 1)

	overdue := newDenseRowTask(t, "overdue-id", 1, "Overdue urgency row")
	overdue.DueDate = &yesterday

	dueToday := newDenseRowTask(t, "due-today-id", 2, "Due today urgency row")
	dueToday.DueDate = &today

	dueTomorrow := newDenseRowTask(t, "due-tomorrow-id", 3, "Future due row")
	dueTomorrow.DueDate = &tomorrow

	project := newDenseRowProject(t)
	m := newDenseRowFocusBoard(t)

	rowOverdue := boardRow{Task: overdue, Project: project, Source: project.Name}
	rowDueToday := boardRow{Task: dueToday, Project: project, Source: project.Name}
	rowDueTomorrow := boardRow{Task: dueTomorrow, Project: project, Source: project.Name}

	const maxTitleLen = 100

	overdueLine := ansi.Strip(m.renderRow(rowOverdue, false, maxTitleLen, "1.", false, nil))
	if !strings.Contains(overdueLine, "yesterday") {
		t.Fatalf("overdue dense row should keep overdue date inline, got %q", overdueLine)
	}

	dueTodayLine := ansi.Strip(m.renderRow(rowDueToday, false, maxTitleLen, "2.", false, nil))
	if !strings.Contains(dueTodayLine, "today") {
		t.Fatalf("due-today dense row should keep \"today\" inline, got %q", dueTodayLine)
	}

	tomorrowLine := ansi.Strip(m.renderRow(rowDueTomorrow, false, maxTitleLen, "3.", false, nil))
	if strings.Contains(tomorrowLine, tomorrow.Format("Jan 2")) ||
		strings.Contains(tomorrowLine, "tomorrow") {
		t.Fatalf("future-due dense row must not surface due text inline, got %q", tomorrowLine)
	}
}

// TestFocusBoardDenseRowOmitsSecondaryMetadata verifies the dense row drops
// secondary indicators (description, checklist, recurrence) in favor of the
// selected metadata strip below the rows.
func TestFocusBoardDenseRowOmitsSecondaryMetadata(t *testing.T) {
	task := newDenseRowTask(t, "rich-id", 7, "Rich task with secondary metadata")
	task.Priority = 2
	task.Description = "A descriptive note that should live in the strip, not the row."
	task.Checklist = []models.ChecklistItem{
		{Text: "one", Done: true},
		{Text: "two", Done: false},
	}
	task.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}

	project := newDenseRowProject(t)
	m := newDenseRowFocusBoard(t)

	row := boardRow{Task: task, Project: project, Source: project.Name}
	line := ansi.Strip(m.renderRow(row, false, 100, "1.", false, nil))

	if strings.Contains(line, ansi.Strip(renderDescriptionIndicator(task))) {
		t.Fatalf("dense row must not carry description indicator inline, got %q", line)
	}
	if strings.Contains(line, ansi.Strip(renderChecklistIndicator(task))) {
		t.Fatalf("dense row must not carry checklist indicator inline, got %q", line)
	}
	if strings.Contains(line, ansi.Strip(renderChecklistBar(task, m.width))) {
		t.Fatalf("dense row must not carry checklist progress bar inline, got %q", line)
	}
	if strings.Contains(line, "↻") {
		t.Fatalf("dense row must not carry recurrence indicator inline, got %q", line)
	}
}

// TestFocusBoardFutureDueDropsInlineDate guards the future-due suppression
// promise: a non-urgent due date (e.g., tomorrow, next week) must not produce
// any inline due-date text in the row.
func TestFocusBoardFutureDueDropsInlineDate(t *testing.T) {
	nextWeek := time.Now().AddDate(0, 0, 7)

	task := newDenseRowTask(t, "future-id", 4, "Task due next week")
	task.DueDate = &nextWeek

	project := newDenseRowProject(t)
	m := newDenseRowFocusBoard(t)

	row := boardRow{Task: task, Project: project, Source: project.Name}
	line := ansi.Strip(m.renderRow(row, false, 100, "1.", false, nil))

	if strings.Contains(line, nextWeek.Format("Jan 2")) {
		t.Fatalf("future-due dense row must not show absolute date inline, got %q", line)
	}
	if strings.Contains(line, "tomorrow") {
		t.Fatalf("future-due dense row must not show relative date inline, got %q", line)
	}
}

// TestFocusBoardCarriedAndFirstPlanStyling locks in the focus-specific row
// anchors that must remain inline: the carried-over marker on Today and the
// first-plan scroll-right indicator with row number.
func TestFocusBoardCarriedAndFirstPlanStyling(t *testing.T) {
	task := newDenseRowTask(t, "carried-id", 5, "Carried first-plan task")

	project := newDenseRowProject(t)
	m := newDenseRowFocusBoard(t)
	m.carriedOverIDs = map[TaskIdentity]bool{taskIdentityFor(task, project): true}

	row := boardRow{Task: task, Project: project, Source: project.Name}
	line := ansi.Strip(m.renderRow(row, false, 100, "1.", true, nil))

	if !strings.Contains(line, "(carried)") {
		t.Fatalf("carried-over Today row should surface \"(carried)\" marker, got %q", line)
	}
	if !strings.Contains(line, IconScrollRight) {
		t.Fatalf("first-plan row should surface the scroll-right indicator, got %q", line)
	}
	if !strings.Contains(line, "1.") {
		t.Fatalf("first-plan row should display the row number, got %q", line)
	}
}

// TestFocusBoardCompletedAndCanceledRowsAreQuiet guards the quiet completed/
// canceled contract: rows for done or canceled tasks must not surface urgency
// hero text inline even when their due date is overdue or due today.
func TestFocusBoardCompletedAndCanceledRowsAreQuiet(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	today := time.Now()

	done := newDenseRowTask(t, "done-id", 8, "Completed overdue task")
	done.WorkflowState = models.StateDone
	done.DueDate = &yesterday

	canceled := newDenseRowTask(t, "canceled-id", 9, "Canceled overdue task")
	canceled.WorkflowState = models.StateCanceled
	canceled.DueDate = &yesterday

	doneToday := newDenseRowTask(t, "done-today-id", 10, "Completed closed task")
	doneToday.WorkflowState = models.StateDone
	doneToday.DueDate = &today

	canceledToday := newDenseRowTask(t, "canceled-today-id", 12, "Canceled closed task")
	canceledToday.WorkflowState = models.StateCanceled
	canceledToday.DueDate = &today

	project := newDenseRowProject(t)
	m := newDenseRowFocusBoard(t)
	// Use Backlog presentation so the row is not dimmed solely because Today
	// dims terminal tasks retained in its plan.
	m.tabs = TabState{Active: boardTabBacklog, Names: focusBoardTabNames}

	for _, task := range []*models.Task{done, canceled, doneToday, canceledToday} {
		row := boardRow{Task: task, Project: project, Source: project.Name}
		line := ansi.Strip(m.renderRow(row, false, 100, "", false, nil))
		if strings.Contains(line, "Overdue") {
			t.Fatalf("quiet %s row should not carry overdue inline, got %q", models.LegacyStatusForState(task.State()), line)
		}
		if strings.Contains(line, yesterday.Format("Jan 2")) {
			t.Fatalf("quiet %s row should not surface due date inline, got %q", models.LegacyStatusForState(task.State()), line)
		}
		if strings.Contains(line, "today") {
			t.Fatalf("quiet %s row should not carry due-today inline, got %q", models.LegacyStatusForState(task.State()), line)
		}
	}
}

// TestFocusBoardSelectedStripCarriesMovedFields ensures the moved secondary
// metadata (description, checklist, future-due date) lives in the strip when
// the cursor sits on the rich task.
func TestFocusBoardSelectedStripCarriesMovedFields(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	tomorrow := time.Now().AddDate(0, 0, 1)

	task := newDenseRowTask(t, "strip-id", 11, "Strip carries moved fields")
	task.Priority = 2
	task.DueDate = &tomorrow
	task.Description = "Detail text that should live in the strip."
	task.Checklist = []models.ChecklistItem{
		{Text: "one", Done: true},
		{Text: "two", Done: false},
	}

	project := newDenseRowProject(t)
	m := newDenseRowFocusBoard(t)

	row := boardRow{Task: task, Project: project, Source: project.Name}
	m.filtered = []boardRow{row}
	m.todayRows = []boardRow{row}
	m.cursor = 0

	plain := ansi.Strip(m.View().Content)

	if !strings.Contains(plain, ansi.Strip(renderDescriptionIndicator(task))) {
		t.Fatalf("metadata strip should surface description indicator, got:\n%s", plain)
	}
	if !strings.Contains(plain, ansi.Strip(renderChecklistIndicator(task))) {
		t.Fatalf("metadata strip should surface checklist indicator, got:\n%s", plain)
	}
	// Future-due date no longer appears inline; the strip must carry it.
	if !strings.Contains(plain, tomorrow.Format("Jan 2")) && !strings.Contains(plain, "tomorrow") {
		t.Fatalf("metadata strip should surface future-due date, got:\n%s", plain)
	}
}
