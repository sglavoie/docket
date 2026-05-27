package alerts

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestCheckBlockedTasks_NoActivityZeroUpdatedAtSkipped verifies that a
// blocked task with neither an activity-log entry nor a populated
// UpdatedAt does not emit a spurious "Blocked for ~700000 days" alert.
func TestCheckBlockedTasks_NoActivityZeroUpdatedAtSkipped(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:     "blk-zero",
		Title:  "blocked but zero",
		Status: models.StatusBlocked,
		// UpdatedAt intentionally zero, Activity nil
	}
	projects := []ProjectData{{
		Project: &models.Project{ID: "p1", Name: "P1"},
		Tasks:   []*models.Task{task},
	}}

	cfg := DefaultAlertConfig()
	cfg.BlockedDays = 7
	alerts := checkBlockedTasks(cfg, projects, now)
	if len(alerts) != 0 {
		t.Errorf("expected 0 blocked alerts when no activity and UpdatedAt zero, got %d (%+v)", len(alerts), alerts)
	}
}

// TestCheckProjectHealth_AllZeroUpdatedAtSkipped verifies that a project
// whose tasks all have a zero UpdatedAt does not produce a spurious
// project-health alert with an astronomically large "days ago" count.
func TestCheckProjectHealth_AllZeroUpdatedAtSkipped(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	tasks := []*models.Task{
		{ID: "t1", Title: "T1", Status: models.StatusTodo}, // UpdatedAt zero
		{ID: "t2", Title: "T2", Status: models.StatusInProgress},
	}
	projects := []ProjectData{{
		Project: &models.Project{ID: "p1", Name: "P1"},
		Tasks:   tasks,
	}}

	alerts := checkProjectHealth(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Errorf("expected 0 project-health alerts when all task UpdatedAt are zero, got %d (%+v)", len(alerts), alerts)
	}
}

// TestCheckOverdueTasks_TimedDueEarlierToday ensures that a task with HasTime=true
// whose exact due time has already passed today is classified as overdue, not due-soon.
func TestCheckOverdueTasks_TimedDueEarlierToday(t *testing.T) {
	// now = 2026-06-10 14:00; task was due at 09:00 the same day.
	now := time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC)
	dueAt := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:      "timed-overdue",
		Title:   "Timed task past due",
		Status:  models.StatusTodo,
		DueDate: &dueAt,
		HasTime: true,
	}
	projects := []ProjectData{{
		Project: &models.Project{ID: "p1", Name: "P1"},
		Tasks:   []*models.Task{task},
	}}

	cfg := DefaultAlertConfig()
	overdueAlerts := checkOverdueTasks(cfg, projects, now)
	if len(overdueAlerts) != 1 {
		t.Fatalf("expected 1 overdue alert for timed task due earlier today, got %d (%+v)", len(overdueAlerts), overdueAlerts)
	}

	// The same task must NOT appear in the due-soon bucket.
	dueSoonAlerts := checkDueSoonTasks(cfg, projects, now)
	for _, a := range dueSoonAlerts {
		if a.TaskID == task.ID {
			t.Errorf("timed task due earlier today must not appear in due-soon alerts")
		}
	}
}

// TestCheckers_NilProjectSkipped verifies checkers do not nil-deref when a
// ProjectData entry has a nil Project. (Defensive: callers should never pass
// this, but corrupted state shouldn't crash the dashboard.)
func TestCheckers_NilProjectSkipped(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	due := now.AddDate(0, 0, -5)
	taskWithDue := &models.Task{
		ID:        "t-due",
		Title:     "due",
		Status:    models.StatusTodo,
		DueDate:   &due,
		UpdatedAt: now,
		Section:   models.SectionInbox,
	}
	projects := []ProjectData{{
		Project: nil,
		Tasks:   []*models.Task{taskWithDue},
	}}

	cfg := DefaultAlertConfig()
	cfg.InboxThreshold = 1

	if got := checkOverdueTasks(cfg, projects, now); len(got) != 0 {
		t.Errorf("overdue: expected 0 alerts for nil project, got %d", len(got))
	}
	if got := checkDueSoonTasks(cfg, projects, now); len(got) != 0 {
		t.Errorf("due_soon: expected 0 alerts for nil project, got %d", len(got))
	}
	if got := checkStaleTasks(cfg, projects, now); len(got) != 0 {
		t.Errorf("stale: expected 0 alerts for nil project, got %d", len(got))
	}
	if got := checkInboxBacklog(cfg, projects, now); len(got) != 0 {
		t.Errorf("inbox: expected 0 alerts for nil project, got %d", len(got))
	}
	if got := checkBlockedTasks(cfg, projects, now); len(got) != 0 {
		t.Errorf("blocked: expected 0 alerts for nil project, got %d", len(got))
	}
	if got := checkProjectHealth(cfg, projects, now); len(got) != 0 {
		t.Errorf("project_health: expected 0 alerts for nil project, got %d", len(got))
	}
}

// TestCheckers_NilTaskSkipped verifies checkers skip nil entries in pd.Tasks
// rather than nil-derefing on task field access.
func TestCheckers_NilTaskSkipped(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	due := now.AddDate(0, 0, -5)
	realTask := &models.Task{
		ID:        "t-real",
		Title:     "real overdue",
		Status:    models.StatusTodo,
		DueDate:   &due,
		UpdatedAt: now.AddDate(0, 0, -30),
		Section:   models.SectionInbox,
	}
	blockedTask := &models.Task{
		ID:        "t-blocked",
		Title:     "blocked",
		Status:    models.StatusBlocked,
		UpdatedAt: now.AddDate(0, 0, -30),
		Activity: []*models.ActivityEntry{
			{Field: "status", NewValue: string(models.StatusBlocked), Timestamp: now.AddDate(0, 0, -10)},
		},
	}
	projects := []ProjectData{{
		Project: &models.Project{ID: "p1", Name: "P1"},
		Tasks:   []*models.Task{nil, realTask, nil, blockedTask, nil},
	}}

	cfg := DefaultAlertConfig()
	cfg.InboxThreshold = 1
	cfg.BlockedDays = 7

	if got := checkOverdueTasks(cfg, projects, now); len(got) != 1 {
		t.Errorf("overdue: expected 1 alert (nil tasks skipped), got %d", len(got))
	}
	if got := checkStaleTasks(cfg, projects, now); len(got) != 2 {
		t.Errorf("stale: expected 2 alerts (nil tasks skipped), got %d", len(got))
	}
	if got := checkInboxBacklog(cfg, projects, now); len(got) != 1 {
		t.Errorf("inbox: expected 1 alert (nil tasks skipped), got %d", len(got))
	}
	if got := checkBlockedTasks(cfg, projects, now); len(got) != 1 {
		t.Errorf("blocked: expected 1 alert (nil tasks skipped), got %d", len(got))
	}
	if got := checkProjectHealth(cfg, projects, now); len(got) != 1 {
		t.Errorf("project_health: expected 1 alert (nil tasks skipped), got %d", len(got))
	}
}

// TestFindLastBlockedTransition_NilEntrySkipped verifies a nil ActivityEntry
// in the slice does not crash the scan; falls through to the next entry or
// UpdatedAt.
func TestFindLastBlockedTransition_NilEntrySkipped(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	blockedAt := now.AddDate(0, 0, -10)
	task := &models.Task{
		ID:        "blk-nil-activity",
		Title:     "blocked with nil entry",
		Status:    models.StatusBlocked,
		UpdatedAt: now.AddDate(0, 0, -1),
		Activity: []*models.ActivityEntry{
			nil,
			{Field: "status", NewValue: string(models.StatusBlocked), Timestamp: blockedAt},
			nil,
		},
	}
	got := findLastBlockedTransition(task)
	if !got.Equal(blockedAt) {
		t.Errorf("expected blockedAt %v, got %v", blockedAt, got)
	}
}

// TestCheckDailyReviewReminder_OverdueWarningBeatsPastHourInfo is a regression
// test: when the user is several days behind on the daily review AND it is
// past the configured reminder hour, the alert must escalate to "Daily review
// overdue" (warning) rather than stay at the info-level "Daily review pending".
func TestCheckDailyReviewReminder_OverdueWarningBeatsPastHourInfo(t *testing.T) {
	now := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC) // well past the configured hour
	cfg := DefaultAlertConfig()
	cfg.DailyReviewHour = 9
	cfg.LastDailyReviewAt = timePtr(time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)) // ~5 days ago

	alerts := checkDailyReviewReminder(cfg, nil, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Title != "Daily review overdue" {
		t.Errorf("expected 'Daily review overdue' title, got %q", alerts[0].Title)
	}
	if alerts[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity when >26h behind, got %d", alerts[0].Severity)
	}
}

// TestCheckDueSoonTasks_TimedDueLaterToday ensures that a task with HasTime=true
// whose exact due time is still in the future today is classified as due-soon.
func TestCheckDueSoonTasks_TimedDueLaterToday(t *testing.T) {
	// now = 2026-06-10 08:00; task is due at 17:00 the same day.
	now := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	dueAt := time.Date(2026, 6, 10, 17, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:      "timed-due-soon",
		Title:   "Timed task due later",
		Status:  models.StatusTodo,
		DueDate: &dueAt,
		HasTime: true,
	}
	projects := []ProjectData{{
		Project: &models.Project{ID: "p1", Name: "P1"},
		Tasks:   []*models.Task{task},
	}}

	cfg := DefaultAlertConfig()
	dueSoonAlerts := checkDueSoonTasks(cfg, projects, now)
	found := false
	for _, a := range dueSoonAlerts {
		if a.TaskID == task.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("timed task due later today should appear in due-soon alerts")
	}

	// Must NOT appear in overdue.
	overdueAlerts := checkOverdueTasks(cfg, projects, now)
	for _, a := range overdueAlerts {
		if a.TaskID == task.ID {
			t.Errorf("timed task due later today must not appear in overdue alerts")
		}
	}
}
