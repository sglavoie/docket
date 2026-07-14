package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// TestOutlinerDenseRowKeepsUrgencyHeroInline guards the dense-row hierarchy:
// overdue and due-today urgency text remains the only date-driven anchor that
// stays inline. A future-due ("tomorrow") row must not show due text inline —
// it moves to the metadata strip.
func TestOutlinerDenseRowKeepsUrgencyHeroInline(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	today := now
	tomorrow := now.AddDate(0, 0, 1)

	overdue := renderFixtureTask(t, "overdue-task", 1, models.TaskTypeTask,
		"Overdue task", models.StatusTodo, 3, models.SectionNext)
	overdue.DueDate = &yesterday

	dueToday := renderFixtureTask(t, "due-today-task", 2, models.TaskTypeTask,
		"Due today task", models.StatusTodo, 3, models.SectionNext)
	dueToday.DueDate = &today

	dueTomorrow := renderFixtureTask(t, "due-tomorrow-task", 3, models.TaskTypeTask,
		"Future due task", models.StatusTodo, 3, models.SectionNext)
	dueTomorrow.DueDate = &tomorrow

	project := renderFixtureProject(t, "proj-urgency", "urg", "Urgency Project",
		[]*models.Task{overdue, dueToday, dueTomorrow})

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(120, 18)
	m.project = project
	m.rebuildRows()

	overdueLine := ansi.Strip(findOutlinerRowLine(t, m, overdue.ID))
	if !strings.Contains(overdueLine, yesterday.Format("Jan 2")) {
		t.Fatalf("overdue dense row should keep overdue date inline, got %q", overdueLine)
	}

	dueTodayLine := ansi.Strip(findOutlinerRowLine(t, m, dueToday.ID))
	if !strings.Contains(dueTodayLine, "Today") {
		t.Fatalf("due-today dense row should keep \"Today\" inline, got %q", dueTodayLine)
	}

	tomorrowLine := ansi.Strip(findOutlinerRowLine(t, m, dueTomorrow.ID))
	if strings.Contains(tomorrowLine, tomorrow.Format("Jan 2")) ||
		strings.Contains(tomorrowLine, "Tomorrow") {
		t.Fatalf("future-due dense row must not surface due text inline, got %q", tomorrowLine)
	}
}

// TestOutlinerDenseRowOmitsSecondaryMetadata verifies that the dense-row
// hierarchy drops the secondary indicators (tags, description, checklist,
// recurrence, snooze, dependencies, score, comments, decisions, attachments,
// progress) from the inline row in favor of the metadata strip below it.
func TestOutlinerDenseRowOmitsSecondaryMetadata(t *testing.T) {
	snoozedUntil := time.Now().AddDate(0, 0, 3)
	task := renderFixtureTask(t, "rich-task", 7, models.TaskTypeTask,
		"Rich task with secondary metadata", models.StatusTodo, 2, models.SectionNext)
	task.Tags = []string{"alpha", "beta"}
	task.Description = "A descriptive note that should live in the strip, not the row."
	task.Checklist = []models.ChecklistItem{
		{Text: "one", Done: true},
		{Text: "two", Done: false},
	}
	task.Comments = []*models.Comment{{Text: "A comment"}}
	task.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	task.SnoozedUntil = &snoozedUntil

	project := renderFixtureProject(t, "proj-rich", "rch", "Rich Project", []*models.Task{task})

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(140, 18)
	m.project = project
	m.activePerspective = PerspectiveSnoozed
	m.rebuildRows()

	line := ansi.Strip(findOutlinerRowLine(t, m, task.ID))

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
	if !strings.Contains(line, "#7") {
		t.Fatalf("dense row should keep short ID inline, got %q", line)
	}
}

// TestOutlinerSelectedStripCarriesMovedFields ensures the moved secondary
// metadata appears in the selected-task metadata strip when the columns are
// visible. The strip is the only place the user can read tags, description,
// and checklist now that those fields have left the dense row.
func TestOutlinerSelectedStripCarriesMovedFields(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	snoozedUntil := time.Now().AddDate(0, 0, 3)
	task := renderFixtureTask(t, "strip-task", 9, models.TaskTypeTask,
		"Selected row carries strip metadata", models.StatusTodo, 2, models.SectionNext)
	task.Tags = []string{"alpha", "beta"}
	task.Description = "Has detail text that the strip should surface."
	task.Checklist = []models.ChecklistItem{
		{Text: "one", Done: true},
		{Text: "two", Done: false},
	}
	task.SnoozedUntil = &snoozedUntil

	project := renderFixtureProject(t, "proj-strip", "stp", "Strip Project", []*models.Task{task})

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	const width = 140
	m.SetSize(width, 18)
	m.project = project
	m.activePerspective = PerspectiveSnoozed
	m.rebuildRows()
	m.cursor = findOutlinerRowIndex(t, m.rows, task.ID)

	rendered := m.renderOutlinerTreeContent(width)
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

// TestOutlinerCompletedAndCanceledDueTodayRowsAreQuiet guards the quiet
// completed/canceled contract for due-today tasks. IsDueToday intentionally
// does not check completion status, so the row renderer must apply the quiet
// state policy before showing inline urgency.
func TestOutlinerCompletedAndCanceledDueTodayRowsAreQuiet(t *testing.T) {
	today := time.Now()

	done := renderFixtureTask(t, "done-today", 13, models.TaskTypeTask,
		"Completed due today task", models.StatusDone, 2, models.SectionCompleted)
	done.DueDate = &today

	canceled := renderFixtureTask(t, "canceled-today", 14, models.TaskTypeTask,
		"Canceled due today task", models.StatusCanceled, 2, models.SectionCompleted)
	canceled.DueDate = &today

	project := renderFixtureProject(t, "proj-quiet", "qit", "Quiet Project", []*models.Task{done, canceled})

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	const width = 140
	m.SetSize(width, 18)
	m.project = project

	for i, task := range []*models.Task{done, canceled} {
		line := ansi.Strip(m.renderOutlinerTaskRow(outlinerRow{task: task}, false, width, width, i, nil))
		if strings.Contains(line, "Today") {
			t.Fatalf("quiet %s row should not carry due-today inline, got %q", models.LegacyStatusForState(task.State()), line)
		}
	}
}

func TestOutlinerDenseRowShowsParkedSnippet(t *testing.T) {
	task := renderFixtureTask(t, "parked-dense", 21, models.TaskTypeTask,
		"Parked dense row task", models.StatusTodo, 3, models.SectionNext)
	task.ParkedNote = "resume pairing"

	project := renderFixtureProject(t, "proj-parked-dense", "prd", "Parked Dense Project", []*models.Task{task})

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.sidebar.SetEnabled(false)
	m.SetSize(120, 18)
	m.project = project
	m.rebuildRows()

	line := ansi.Strip(findOutlinerRowLine(t, m, task.ID))
	if !strings.Contains(line, "⏸ resume pairing") {
		t.Fatalf("dense row should show parked snippet inline, got %q", line)
	}
}

// TestOutlinerDenseRowSuppressesParkedSnippetWhenSidebarEnabled verifies the
// sidebar-aware placement from issue #3: when the outliner sidebar is visible,
// the parked marker is dropped from the dense row (it renders in the sidebar
// Task card instead).
func TestOutlinerDenseRowSuppressesParkedSnippetWhenSidebarEnabled(t *testing.T) {
	task := renderFixtureTask(t, "parked-dense-sidebar", 25, models.TaskTypeTask,
		"Parked sidebar dense row task", models.StatusTodo, 3, models.SectionNext)
	task.ParkedNote = "resume pairing"

	project := renderFixtureProject(t, "proj-parked-dense-sidebar", "prs", "Parked Sidebar Project", []*models.Task{task})

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(120, 18)
	m.project = project
	m.rebuildRows()

	if !m.sidebar.IsEnabled() {
		t.Fatal("sidebar should be enabled by default for this test")
	}

	line := ansi.Strip(findOutlinerRowLine(t, m, task.ID))
	if strings.Contains(line, "⏸") || strings.Contains(line, "resume pairing") {
		t.Fatalf("dense row must not show parked snippet when sidebar is enabled, got %q", line)
	}

	// The parked line moves to the sidebar Task card, which shares
	// renderTaskHeaderContent.
	card := ansi.Strip(renderTaskHeaderContent(task, false, true))
	if !strings.Contains(card, "⏸ Parked") || !strings.Contains(card, "resume pairing") {
		t.Fatalf("sidebar Task card should show parked line, got %q", card)
	}
}

func TestOutlinerDenseRowShowsParkedFallbackForBlankNote(t *testing.T) {
	parkedAt := time.Now()
	task := renderFixtureTask(t, "parked-dense-blank", 23, models.TaskTypeTask,
		"Blank-note parked dense row task", models.StatusTodo, 3, models.SectionNext)
	task.ParkedAt = &parkedAt

	project := renderFixtureProject(t, "proj-parked-dense-blank", "pdb", "Blank Parked Dense Project", []*models.Task{task})

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.sidebar.SetEnabled(false)
	m.SetSize(120, 18)
	m.project = project
	m.rebuildRows()

	line := ansi.Strip(findOutlinerRowLine(t, m, task.ID))
	if !strings.Contains(line, "⏸ parked") {
		t.Fatalf("dense row should show fallback parked snippet inline, got %q", line)
	}
}

func TestOutlinerDenseRowParkedSnippetStaysWithinWidth(t *testing.T) {
	task := renderFixtureTask(t, "parked-dense-narrow", 22, models.TaskTypeTask,
		"Parked dense row task with a long title", models.StatusTodo, 3, models.SectionNext)
	task.ParkedNote = "long resume note that must fit into the dense row width budget"

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.sidebar.SetEnabled(false)
	const width = 48
	m.SetSize(width, 18)

	line := m.renderOutlinerTaskRow(outlinerRow{task: task}, false, width, width, 0, nil)
	if got := lipgloss.Width(ansi.Strip(line)); got > width {
		t.Fatalf("parked dense row width = %d, want <= %d, stripped=%q", got, width, ansi.Strip(line))
	}
}

func TestOutlinerDenseRowParkedSnippetExpandsWithRowWidth(t *testing.T) {
	task := renderFixtureTask(t, "parked-dense-responsive", 24, models.TaskTypeTask,
		"Parked dense row task", models.StatusTodo, 3, models.SectionNext)
	task.ParkedNote = "resume alpha beta gamma delta marker epsilon zeta"

	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.sidebar.SetEnabled(false)

	narrow := ansi.Strip(m.renderOutlinerTaskRow(outlinerRow{task: task}, false, 54, 54, 0, nil))
	wide := ansi.Strip(m.renderOutlinerTaskRow(outlinerRow{task: task}, false, 120, 120, 0, nil))

	if strings.Contains(narrow, "delta marker") {
		t.Fatalf("narrow dense row exposed too much parked note text, got %q", narrow)
	}
	if !strings.Contains(wide, "delta marker") {
		t.Fatalf("wide dense row should expose more parked note text, got %q", wide)
	}
}

// TestOutlinerDenseRowWidthStableAcrossSecondaryMetadata locks in the dense
// hierarchy's stability promise: varying secondary metadata (tags, snooze,
// recurrence, description) on individual tasks must not shift sibling row
// widths because none of those fields participate in the inline format.
func TestOutlinerDenseRowWidthStableAcrossSecondaryMetadata(t *testing.T) {
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	const width = 120
	m.SetSize(width, 18)

	const titleA = "Sibling task A"
	const titleB = "Sibling task B"
	const titleC = "Sibling task C"
	const titleD = "Sibling task D"

	plain := renderFixtureTask(t, "stable-plain", 1, models.TaskTypeTask,
		titleA, models.StatusTodo, 3, models.SectionNext)
	tagged := renderFixtureTask(t, "stable-tagged", 2, models.TaskTypeTask,
		titleB, models.StatusTodo, 3, models.SectionNext)
	tagged.Tags = []string{"alpha", "beta", "gamma"}
	described := renderFixtureTask(t, "stable-described", 3, models.TaskTypeTask,
		titleC, models.StatusTodo, 3, models.SectionNext)
	described.Description = "A long enough description to pressure the row width."
	recurring := renderFixtureTask(t, "stable-recurring", 4, models.TaskTypeTask,
		titleD, models.StatusTodo, 3, models.SectionNext)
	recurring.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 5}

	project := renderFixtureProject(t, "proj-stable", "stb", "Stable Project",
		[]*models.Task{plain, tagged, described, recurring})
	m.project = project
	m.rebuildRows()

	widths := make(map[int]int)
	for i, row := range m.rows {
		if row.task == nil {
			continue
		}
		line := m.renderOutlinerTaskRow(row, false, width, width, i, nil)
		widths[lipgloss.Width(ansi.Strip(line))]++
	}
	if len(widths) != 1 {
		t.Fatalf("expected uniform dense-row widths across varied secondary metadata, got %v", widths)
	}
}

func findOutlinerRowLine(t *testing.T, m *OutlinerModel, taskID string) string {
	t.Helper()
	const width = 140
	for i, row := range m.rows {
		if row.task != nil && row.task.ID == taskID {
			return m.renderOutlinerTaskRow(row, false, width, width, i, nil)
		}
	}
	t.Fatalf("no row found for task %q", taskID)
	return ""
}
