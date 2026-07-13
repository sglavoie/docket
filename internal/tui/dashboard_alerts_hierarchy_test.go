package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/util"
)

// urgencyDashboard builds a DashboardModel wired with the given projects so
// alert-hierarchy tests can shape the world directly without touching the
// filesystem.
func urgencyDashboard(t *testing.T, projects ...*models.Project) *DashboardModel {
	t.Helper()
	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	cfg.Projects = projects
	return &DashboardModel{config: cfg, projects: projects}
}

// addDueToday attaches a due-today date to a task. Section is left as the
// caller already configured; only DueDate is touched.
func addDueToday(task *models.Task) {
	now := time.Now()
	task.DueDate = &now
}

// addOverdue attaches a due date in the past to a task.
func addOverdue(task *models.Task) {
	due := time.Now().Add(-48 * time.Hour)
	task.DueDate = &due
}

// completedToday makes a task look like it was completed earlier today so the
// "Today: N done" counter increments.
func completedToday(task *models.Task) {
	task.WorkflowState = models.StateDone
	now := time.Now()
	task.CompletedAt = &now
}

func planToday(t *testing.T, m *DashboardModel, project *models.Project, task *models.Task) {
	t.Helper()
	if m == nil || m.config == nil {
		t.Fatal("dashboard model must have config")
	}
	m.config.DailyPlan = append(m.config.DailyPlan, config.DailyPlanItem{
		TaskID:      task.ID,
		ProjectID:   project.ID,
		PlannedDate: time.Now().Format(util.DateFormat),
	})
}

// TestAlertHierarchyTriageWinsFirstLine verifies the tier-1 contract: any
// inbox backlog must surface as the first alert line, beating overdue/due/
// blocked/progress/summary tiers.
func TestAlertHierarchyTriageWinsFirstLine(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Mixed Signals")
	inbox := testutil.NewTestTask("triage me", models.TaskTypeTask)
	inbox.SetSection(models.SectionInbox)
	overdue := testutil.NewTestTask("late task", models.TaskTypeTask)
	overdue.SetSection(models.SectionNext)
	addOverdue(overdue)
	blocked := testutil.NewTestTask("stuck task", models.TaskTypeTask)
	blocked.SetSection(models.SectionNext)
	blocked.WorkflowState = models.StateWaiting
	planned := testutil.NewTestTask("planned task", models.TaskTypeTask)
	planned.SetSection(models.SectionNext)
	project.Tasks = []*models.Task{inbox, overdue, blocked, planned}

	m := urgencyDashboard(t, project)
	lines := m.computeGlobalAlertLines(nil)
	if len(lines) == 0 {
		t.Fatal("expected at least one alert line")
	}
	first := ansi.Strip(lines[0])
	if !strings.Contains(first, "1 to triage") {
		t.Fatalf("expected triage to win first line, got %q", first)
	}
}

// TestAlertHierarchyOverdueWinsWhenNoTriage verifies tier-1 step 2: with the
// inbox empty, overdue should win first line ahead of due-today, progress,
// blocked, and summary.
func TestAlertHierarchyOverdueWinsWhenNoTriage(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Mixed Signals")
	overdue := testutil.NewTestTask("late task", models.TaskTypeTask)
	overdue.SetSection(models.SectionNext)
	addOverdue(overdue)
	dueToday := testutil.NewTestTask("due today task", models.TaskTypeTask)
	dueToday.SetSection(models.SectionNext)
	addDueToday(dueToday)
	blocked := testutil.NewTestTask("stuck task", models.TaskTypeTask)
	blocked.SetSection(models.SectionNext)
	blocked.WorkflowState = models.StateWaiting
	project.Tasks = []*models.Task{overdue, dueToday, blocked}

	m := urgencyDashboard(t, project)
	lines := m.computeGlobalAlertLines(nil)
	if len(lines) == 0 {
		t.Fatal("expected at least one alert line")
	}
	first := ansi.Strip(lines[0])
	if !strings.Contains(first, "overdue") {
		t.Fatalf("expected overdue to win first line, got %q", first)
	}
	if strings.Contains(first, "to triage") {
		t.Fatalf("triage line should not surface when inbox empty, got %q", first)
	}
}

// TestAlertHierarchyDueTodayWinsWhenNoUrgent verifies tier-1 step 3: with no
// triage and no overdue, due-today should win first line ahead of progress,
// blocked, and summary.
func TestAlertHierarchyDueTodayWinsWhenNoUrgent(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Mixed Signals")
	dueToday := testutil.NewTestTask("due today task", models.TaskTypeTask)
	dueToday.SetSection(models.SectionNext)
	addDueToday(dueToday)
	blocked := testutil.NewTestTask("stuck task", models.TaskTypeTask)
	blocked.SetSection(models.SectionNext)
	blocked.WorkflowState = models.StateWaiting
	project.Tasks = []*models.Task{dueToday, blocked}

	m := urgencyDashboard(t, project)
	lines := m.computeGlobalAlertLines(nil)
	if len(lines) == 0 {
		t.Fatal("expected at least one alert line")
	}
	first := ansi.Strip(lines[0])
	if !strings.Contains(first, "due today") {
		t.Fatalf("expected due-today to win first line, got %q", first)
	}
}

// TestAlertHierarchyTodayProgressBeatsBlocked verifies tier-2 beats tier-3:
// with no triage/overdue/due-today, the Today progress line appears before
// the blocked tier-3 row.
func TestAlertHierarchyTodayProgressBeatsBlocked(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Mixed Signals")
	planned := testutil.NewTestTask("planned task", models.TaskTypeTask)
	planned.SetSection(models.SectionNext)
	done := testutil.NewTestTask("finished task", models.TaskTypeTask)
	done.SetSection(models.SectionCompleted)
	completedToday(done)
	blocked := testutil.NewTestTask("stuck task", models.TaskTypeTask)
	blocked.SetSection(models.SectionWaiting)
	blocked.WorkflowState = models.StateWaiting
	project.Tasks = []*models.Task{planned, done, blocked}

	m := urgencyDashboard(t, project)
	planToday(t, m, project, planned)
	lines := m.computeGlobalAlertLines(nil)
	if len(lines) == 0 {
		t.Fatal("expected at least one alert line")
	}
	first := ansi.Strip(lines[0])
	if !strings.HasPrefix(first, "Today:") {
		t.Fatalf("expected Today progress line first, got %q", first)
	}
	if !strings.Contains(first, "1 planned") {
		t.Fatalf("expected planned count in progress line, got %q", first)
	}
	if !strings.Contains(first, "1 done") {
		t.Fatalf("expected done count in progress line, got %q", first)
	}
	// And the blocked row must follow somewhere lower.
	var sawBlocked bool
	for _, l := range lines[1:] {
		if strings.Contains(ansi.Strip(l), "1 blocked") {
			sawBlocked = true
			break
		}
	}
	if !sawBlocked {
		t.Fatalf("expected blocked tier-3 line after progress, got lines:\n%v", lines)
	}
}

// TestAlertHierarchyBlockedAndBlockersMerge verifies tier-3 produces a single
// muted row that joins blocked-task count with active-blocker count, and that
// it wins first line when no urgency or progress signal exists.
func TestAlertHierarchyBlockedAndBlockersMerge(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Mixed Signals")
	project.Path = filepath.Join(t.TempDir(), "mixed")
	blocked := testutil.NewTestTask("stuck task", models.TaskTypeTask)
	blocked.SetSection(models.SectionWaiting)
	blocked.WorkflowState = models.StateWaiting
	project.Tasks = []*models.Task{blocked}
	if err := storage.SaveBlockers(project, []*models.Blocker{models.NewBlocker("the blocker")}); err != nil {
		t.Fatalf("SaveBlockers: %v", err)
	}

	m := urgencyDashboard(t, project)
	lines := m.computeGlobalAlertLines(nil)
	if len(lines) == 0 {
		t.Fatal("expected at least one alert line")
	}
	first := ansi.Strip(lines[0])
	if !strings.Contains(first, "1 blocked") {
		t.Fatalf("expected merged blocked count in first line, got %q", first)
	}
	if !strings.Contains(first, "1 active blockers") {
		t.Fatalf("expected merged active-blockers count in first line, got %q", first)
	}
	// Confirm they share one row instead of two.
	separateBlockedLines := 0
	for _, l := range lines {
		stripped := ansi.Strip(l)
		if strings.Contains(stripped, "blocked") && !strings.Contains(stripped, "active blockers") {
			separateBlockedLines++
		}
	}
	if separateBlockedLines > 0 {
		t.Fatalf("blocked + active blockers should merge into one line, found %d standalone", separateBlockedLines)
	}
}

// TestAlertHierarchyDropsInboxHotspots verifies the "Inbox hotspots" line is
// no longer emitted (project rows surface per-project inbox counts already, so
// the alerts panel no longer duplicates that metadata).
func TestAlertHierarchyDropsInboxHotspots(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Inboxy")
	for i := 0; i < 6; i++ {
		task := testutil.NewTestTask("inbox", models.TaskTypeTask)
		task.SetSection(models.SectionInbox)
		project.Tasks = append(project.Tasks, task)
	}

	m := urgencyDashboard(t, project)
	lines := m.computeGlobalAlertLines(nil)
	for _, l := range lines {
		if strings.Contains(ansi.Strip(l), "Inbox hotspots") {
			t.Fatalf("expected Inbox hotspots line to be removed, got %q", ansi.Strip(l))
		}
	}
}

// TestAlertHierarchyTriageFoldsSelectedProjectHint verifies the global triage
// line absorbs the selected-project triage CTA instead of emitting a second
// "Press O to triage" line — the redundant double-alert is gone but the O
// shortcut is still discoverable.
func TestAlertHierarchyTriageFoldsSelectedProjectHint(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Inboxy")
	inboxA := testutil.NewTestTask("triage me", models.TaskTypeTask)
	inboxA.SetSection(models.SectionInbox)
	inboxB := testutil.NewTestTask("triage me too", models.TaskTypeTask)
	inboxB.SetSection(models.SectionInbox)
	project.Tasks = []*models.Task{inboxA, inboxB}

	m := urgencyDashboard(t, project)
	lines := m.computeGlobalAlertLines(project)
	if len(lines) == 0 {
		t.Fatal("expected at least one alert line")
	}
	first := ansi.Strip(lines[0])
	if !strings.Contains(first, "2 to triage") {
		t.Fatalf("expected global triage count in first line, got %q", first)
	}
	if !strings.Contains(first, "O for Inboxy") {
		t.Fatalf("expected folded selected-project hint in first line, got %q", first)
	}
	// Confirm the legacy second-line CTA no longer fires.
	for _, l := range lines {
		if strings.Contains(ansi.Strip(l), "Press O to triage") {
			t.Fatalf("expected selected-project triage line to fold into the global triage row, got %q", ansi.Strip(l))
		}
	}
}

// TestAlertHierarchyTodayProgressCombinesPlannedAndDone verifies the progress
// row joins both signals on one line when both are present.
func TestAlertHierarchyTodayProgressCombinesPlannedAndDone(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Mixed Signals")
	planned := testutil.NewTestTask("planned task", models.TaskTypeTask)
	planned.SetSection(models.SectionNext)
	done := testutil.NewTestTask("finished task", models.TaskTypeTask)
	done.SetSection(models.SectionCompleted)
	completedToday(done)
	project.Tasks = []*models.Task{planned, done}

	m := urgencyDashboard(t, project)
	planToday(t, m, project, planned)
	lines := m.computeGlobalAlertLines(nil)
	var progress string
	for _, l := range lines {
		if strings.HasPrefix(ansi.Strip(l), "Today:") {
			progress = ansi.Strip(l)
			break
		}
	}
	if progress == "" {
		t.Fatalf("expected Today: progress line, got lines:\n%v", lines)
	}
	if !strings.Contains(progress, "1 planned") {
		t.Fatalf("expected planned segment in progress line, got %q", progress)
	}
	if !strings.Contains(progress, "1 done") {
		t.Fatalf("expected done segment in progress line, got %q", progress)
	}
}

// TestAlertHierarchyTodayProgressUsesDailyPlanNotNextSection verifies the
// user-visible "Today: N planned" segment follows persisted daily-plan
// membership instead of treating every Next-section task as planned today.
func TestAlertHierarchyTodayProgressUsesDailyPlanNotNextSection(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Daily Plan")
	nextOnly := testutil.NewTestTask("backlog next", models.TaskTypeTask)
	nextOnly.SetSection(models.SectionNext)
	planned := testutil.NewTestTask("actual daily plan", models.TaskTypeTask)
	planned.SetSection(models.SectionWaiting)
	project.Tasks = []*models.Task{nextOnly, planned}

	m := urgencyDashboard(t, project)
	for _, l := range m.computeGlobalAlertLines(nil) {
		if strings.HasPrefix(ansi.Strip(l), "Today:") {
			t.Fatalf("expected no Today progress when DailyPlan is empty, got %q", ansi.Strip(l))
		}
	}

	planToday(t, m, project, planned)
	lines := m.computeGlobalAlertLines(nil)
	var progress string
	for _, l := range lines {
		if strings.HasPrefix(ansi.Strip(l), "Today:") {
			progress = ansi.Strip(l)
			break
		}
	}
	if progress == "" {
		t.Fatalf("expected Today progress from DailyPlan membership, got lines:\n%v", lines)
	}
	if !strings.Contains(progress, "1 planned") {
		t.Fatalf("expected one planned daily-plan task, got %q", progress)
	}
	if strings.Contains(progress, "2 planned") {
		t.Fatalf("Next-section backlog task should not count as planned today, got %q", progress)
	}
}

// TestAlertHierarchyAllTiersOrdering verifies that when every tier has a
// signal, the rendered order is: triage (T1) → overdue (T1) → due-today (T1)
// → Today progress (T2) → blocked+blockers (T3) → completed-this-week (T4).
func TestAlertHierarchyAllTiersOrdering(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("All Tiers")
	inbox := testutil.NewTestTask("triage me", models.TaskTypeTask)
	inbox.SetSection(models.SectionInbox)
	overdue := testutil.NewTestTask("late task", models.TaskTypeTask)
	overdue.SetSection(models.SectionNext)
	addOverdue(overdue)
	dueToday := testutil.NewTestTask("due today task", models.TaskTypeTask)
	dueToday.SetSection(models.SectionNext)
	addDueToday(dueToday)
	planned := testutil.NewTestTask("planned task", models.TaskTypeTask)
	planned.SetSection(models.SectionNext)
	blocked := testutil.NewTestTask("stuck task", models.TaskTypeTask)
	blocked.SetSection(models.SectionWaiting)
	blocked.WorkflowState = models.StateWaiting
	done := testutil.NewTestTask("finished task", models.TaskTypeTask)
	done.SetSection(models.SectionCompleted)
	completedToday(done)
	earlierThisWeek := testutil.NewTestTask("earlier this week", models.TaskTypeTask)
	earlierThisWeek.SetSection(models.SectionCompleted)
	earlierThisWeek.WorkflowState = models.StateDone
	twoDaysAgo := time.Now().Add(-48 * time.Hour)
	earlierThisWeek.CompletedAt = &twoDaysAgo
	project.Tasks = []*models.Task{inbox, overdue, dueToday, planned, blocked, done, earlierThisWeek}

	m := urgencyDashboard(t, project)
	planToday(t, m, project, planned)
	lines := m.computeGlobalAlertLines(nil)
	if len(lines) < 6 {
		t.Fatalf("expected at least 6 alert lines for all-tier scenario, got %d:\n%v", len(lines), lines)
	}

	plain := make([]string, len(lines))
	for i, l := range lines {
		plain[i] = ansi.Strip(l)
	}

	wantOrder := []string{"to triage", "overdue", "due today", "Today:", "blocked", "completed this week"}
	cursor := 0
	for _, l := range plain {
		if cursor >= len(wantOrder) {
			break
		}
		if strings.Contains(l, wantOrder[cursor]) {
			cursor++
		}
	}
	if cursor != len(wantOrder) {
		t.Fatalf("alert ordering violated; got lines:\n%v\nstuck at expected %q", plain, wantOrder[cursor])
	}
}

// TestAlertHierarchyEmptyWhenNothingActionable verifies the panel emits no
// lines (so the compact info card collapses to "Alerts · clear") when every
// tier is empty.
func TestAlertHierarchyEmptyWhenNothingActionable(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	project := testutil.NewTestProject("Clean Slate")
	project.Tasks = []*models.Task{}

	m := urgencyDashboard(t, project)
	lines := m.computeGlobalAlertLines(nil)
	if len(lines) != 0 {
		t.Fatalf("expected no alert lines for empty world, got %v", lines)
	}
}
