package alerts

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestCheckBlockedTasks_BlockedLongEnough(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("blocked", models.StatusBlocked, now.AddDate(0, 0, -10))

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkBlockedTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %d", alerts[0].Severity)
	}
}

func TestCheckBlockedTasks_RecentlyBlocked(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("blocked-recent", models.StatusBlocked, now.AddDate(0, 0, -3))

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkBlockedTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for recently blocked task, got %d", len(alerts))
	}
}

func TestCheckBlockedTasks_NonBlockedIgnored(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("in-progress", models.StatusInProgress, now.AddDate(0, 0, -20))

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkBlockedTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for non-blocked task, got %d", len(alerts))
	}
}

func TestCheckBlockedTasks_UsesActivityLogNotUpdatedAt(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("blocked-edited", models.StatusBlocked, now.AddDate(0, 0, -2))
	task.Activity = []*models.ActivityEntry{{
		Timestamp: now.AddDate(0, 0, -10),
		Field:     "status",
		OldValue:  string(models.StatusInProgress),
		NewValue:  string(models.StatusBlocked),
	}}

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkBlockedTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert (blocked 10 days per activity log), got %d", len(alerts))
	}
}

func TestCheckBlockedTasks_RecentActivityLogBlockedNoAlert(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("blocked-recent-log", models.StatusBlocked, now.AddDate(0, 0, -20))
	task.Activity = []*models.ActivityEntry{{
		Timestamp: now.AddDate(0, 0, -3),
		Field:     "status",
		OldValue:  string(models.StatusTodo),
		NewValue:  string(models.StatusBlocked),
	}}

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkBlockedTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (blocked only 3 days per activity log), got %d", len(alerts))
	}
}

func TestCheckBlockedTasks_CalendarDayNotHours(t *testing.T) {
	// A task blocked 7 calendar days ago but only ~6.5 hours-based days
	// (simulating a DST spring-forward scenario where hours/24 would give 6).
	// With DaysBetween (calendar-day comparison) it should correctly give 7.
	loc := time.FixedZone("Test", -5*3600) // UTC-5
	blockedAt := time.Date(2026, 3, 1, 23, 30, 0, 0, loc)
	now := time.Date(2026, 3, 8, 10, 0, 0, 0, loc) // 7 calendar days later, ~6.4 days in hours

	task := testTask("blocked-dst", models.StatusBlocked, blockedAt)
	cfg := DefaultAlertConfig()
	cfg.BlockedDays = 7

	projects := []ProjectData{testProject("proj-dst", task)}
	alerts := checkBlockedTasks(cfg, projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for task blocked 7 calendar days, got %d", len(alerts))
	}
}

func TestCheckProjectHealth_StaleProject(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("old", models.StatusTodo, now.AddDate(0, 0, -30))

	projects := []ProjectData{testProject("stale-proj", task)}
	alerts := checkProjectHealth(DefaultAlertConfig(), projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].ProjectName != "stale-proj" {
		t.Fatalf("expected project name stale-proj, got %q", alerts[0].ProjectName)
	}
	if alerts[0].Title != "No activity" {
		t.Fatalf("expected project-free title, got %q", alerts[0].Title)
	}
	if strings.Contains(alerts[0].Title, "stale-proj") || strings.Contains(alerts[0].Details, "stale-proj") {
		t.Fatalf("expected row copy to omit project name, got title=%q details=%q", alerts[0].Title, alerts[0].Details)
	}
}

func TestCheckProjectHealth_ActiveProject(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("recent", models.StatusTodo, now.AddDate(0, 0, -3))

	projects := []ProjectData{testProject("active-proj", task)}
	alerts := checkProjectHealth(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for active project, got %d", len(alerts))
	}
}

func TestCheckProjectHealth_EmptyProject(t *testing.T) {
	projects := []ProjectData{testProject("empty")}
	alerts := checkProjectHealth(DefaultAlertConfig(), projects, time.Now())
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for empty project, got %d", len(alerts))
	}
}

func TestCheckProjectHealth_MixedTasks(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	oldTask := testTask("old", models.StatusTodo, now.AddDate(0, 0, -30))
	recentTask := testTask("recent", models.StatusTodo, now.AddDate(0, 0, -1))

	projects := []ProjectData{testProject("mixed", oldTask, recentTask)}
	alerts := checkProjectHealth(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (most recent update is 1 day ago), got %d", len(alerts))
	}
}
