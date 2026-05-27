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
