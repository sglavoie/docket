package alerts

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestCheckOverdueTasks_NoTasks(t *testing.T) {
	cfg := DefaultAlertConfig()
	alerts := checkOverdueTasks(cfg, nil, time.Now())
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts with no tasks, got %d", len(alerts))
	}
}

func TestCheckOverdueTasks_OverdueTask(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC)
	task := testTask("overdue task", models.StatusTodo, now)
	task.DueDate = &dueDate

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkOverdueTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Severity != SeverityWarning {
		t.Errorf("expected warning for 3-day overdue, got %d", alerts[0].Severity)
	}
	if alerts[0].TaskID != task.ID {
		t.Errorf("expected task ID %s, got %s", task.ID, alerts[0].TaskID)
	}
}

func TestCheckOverdueTasks_OverdueMoreThan7Days(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	task := testTask("very overdue", models.StatusInProgress, now)
	task.DueDate = &dueDate

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkOverdueTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Severity != SeverityUrgent {
		t.Errorf("expected urgent severity for >7 day overdue, got %d", alerts[0].Severity)
	}
}

func TestCheckOverdueTasks_CompletedTaskIgnored(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	task := testTask("done task", models.StatusDone, now)
	task.DueDate = &dueDate

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkOverdueTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for completed task, got %d", len(alerts))
	}
}

func TestCheckOverdueTasks_NoDueDate(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("no due date", models.StatusTodo, now)

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkOverdueTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for task without due date, got %d", len(alerts))
	}
}

func TestCheckOverdueTasks_DueToday(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	task := testTask("due today", models.StatusTodo, now)
	task.DueDate = &dueDate

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkOverdueTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for task due today (not overdue), got %d", len(alerts))
	}
}

func TestCheckDueSoonTasks_DueWithinHorizon(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	task := testTask("soon task", models.StatusTodo, now)
	task.DueDate = &dueDate

	projects := []ProjectData{testProject("proj1", task)}
	cfg := DefaultAlertConfig()
	alerts := checkDueSoonTasks(cfg, projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

func TestCheckDueSoonTasks_DueToday(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	task := testTask("due today", models.StatusTodo, now)
	task.DueDate = &dueDate

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkDueSoonTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for task due today, got %d", len(alerts))
	}
}

func TestCheckDueSoonTasks_OverdueExcluded(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC)
	task := testTask("overdue", models.StatusTodo, now)
	task.DueDate = &dueDate

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkDueSoonTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for overdue task, got %d", len(alerts))
	}
}

func TestCheckDueSoonTasks_BeyondHorizon(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	task := testTask("far out", models.StatusTodo, now)
	task.DueDate = &dueDate

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkDueSoonTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for far-future task, got %d", len(alerts))
	}
}

func TestCheckStaleTasks_StaleTask(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("stale", models.StatusTodo, now.AddDate(0, 0, -20))

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkStaleTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

func TestCheckStaleTasks_RecentTask(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("recent", models.StatusTodo, now.AddDate(0, 0, -5))

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkStaleTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for recent task, got %d", len(alerts))
	}
}

func TestCheckStaleTasks_CompletedIgnored(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("done stale", models.StatusDone, now.AddDate(0, 0, -30))

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkStaleTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for completed task, got %d", len(alerts))
	}
}

func TestCheckStaleTasks_ExactThresholdBoundary(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("boundary", models.StatusTodo, now.AddDate(0, 0, -14))

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkStaleTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts at exact boundary, got %d", len(alerts))
	}
}

func TestCheckInboxBacklog_AboveThreshold(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	var tasks []*models.Task
	for i := range 6 {
		task := testTask("inbox", models.StatusTodo, now)
		task.ID = "inbox-" + string(rune('a'+i))
		task.Section = models.SectionInbox
		tasks = append(tasks, task)
	}

	projects := []ProjectData{testProject("proj1", tasks...)}
	cfg := DefaultAlertConfig()
	cfg.InboxThreshold = 5
	alerts := checkInboxBacklog(cfg, projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Count != 6 {
		t.Errorf("expected count 6, got %d", alerts[0].Count)
	}
}

func TestCheckInboxBacklog_BelowThreshold(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("inbox1", models.StatusTodo, now)
	task.Section = models.SectionInbox

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkInboxBacklog(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts below threshold, got %d", len(alerts))
	}
}

func TestCheckInboxBacklog_CompletedInboxIgnored(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	var tasks []*models.Task
	for i := range 6 {
		task := testTask("done-inbox", models.StatusDone, now)
		task.ID = "done-" + string(rune('a'+i))
		task.Section = models.SectionInbox
		tasks = append(tasks, task)
	}

	projects := []ProjectData{testProject("proj1", tasks...)}
	alerts := checkInboxBacklog(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for completed inbox tasks, got %d", len(alerts))
	}
}
