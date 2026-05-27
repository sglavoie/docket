package alerts

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestCheckDailyReviewReminder_CrossTimezone is a regression test: the
// "completed today" comparison used each timestamp's own location, so a
// review timestamp stored in UTC and surfaced after the user moved into a
// non-UTC zone fired a spurious "pending review" reminder on the same local
// day the review was completed.
func TestCheckDailyReviewReminder_CrossTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("America/New_York not available: %v", err)
	}

	// User just completed the review at 23:00 UTC = 18:00 in NY (same day).
	last := time.Date(2026, 2, 28, 23, 0, 0, 0, time.UTC)
	// Now is one hour later: 00:00 UTC = 19:00 in NY (still same NY day).
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC).In(loc)

	cfg := DefaultAlertConfig()
	cfg.LastDailyReviewAt = &last
	cfg.DailyReviewHour = 9

	alerts := checkDailyReviewReminder(cfg, nil, now)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts (same local day in user's zone), got %d: %+v", len(alerts), alerts)
	}
}

// TestCheckWeeklyReviewReminder_CrossTimezone mirrors the daily test for the
// weekly "review day" path.
func TestCheckWeeklyReviewReminder_CrossTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("America/New_York not available: %v", err)
	}

	// Last review at 23:00 UTC on a Sunday = 18:00 NY (still Sunday).
	last := time.Date(2026, 3, 1, 23, 0, 0, 0, time.UTC) // Sunday UTC
	// Now is 00:30 UTC Monday = 19:30 NY Sunday — still the same NY day.
	now := time.Date(2026, 3, 2, 0, 30, 0, 0, time.UTC).In(loc)

	cfg := DefaultAlertConfig()
	cfg.LastWeeklyReviewAt = &last
	cfg.WeeklyReviewDay = int(time.Sunday)

	alerts := checkWeeklyReviewReminder(cfg, nil, now)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts (review completed earlier on same local day), got %d: %+v", len(alerts), alerts)
	}
}

// TestCheckOverdueTasks_CrossTimezoneDueToday is a regression test: a task
// whose DueDate is stored at midnight UTC on a calendar day equal to the
// user's local "today" must not be flagged as overdue. The previous
// implementation built dueDay in task.DueDate.Location() but compared it
// against `today` in now.Location(); a midnight-UTC due date appeared
// "before" a NY-local today even though both reference the same calendar
// day for the user.
func TestCheckOverdueTasks_CrossTimezoneDueToday(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("America/New_York not available: %v", err)
	}

	// User picked Feb 28 as the due date; stored at midnight UTC.
	dueDate := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	// Now is 09:00 NY on Feb 28 = 14:00 UTC Feb 28.
	now := time.Date(2026, 2, 28, 14, 0, 0, 0, time.UTC).In(loc)

	task := &models.Task{
		ID:      "due-today",
		Title:   "Due today",
		Status:  models.StatusTodo,
		DueDate: &dueDate,
	}
	projects := []ProjectData{{
		Project: &models.Project{ID: "p1", Name: "P1"},
		Tasks:   []*models.Task{task},
	}}

	overdueAlerts := checkOverdueTasks(DefaultAlertConfig(), projects, now)
	if len(overdueAlerts) != 0 {
		t.Errorf("task due today in user's local zone must not be overdue, got %d alerts: %+v", len(overdueAlerts), overdueAlerts)
	}

	dueSoonAlerts := checkDueSoonTasks(DefaultAlertConfig(), projects, now)
	found := false
	for _, a := range dueSoonAlerts {
		if a.TaskID == task.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("task due today in user's local zone should appear in due-soon alerts, got %+v", dueSoonAlerts)
	}
}
