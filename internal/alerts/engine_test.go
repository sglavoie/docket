package alerts

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// Helper to create a minimal project with tasks for testing.
func testProject(name string, tasks ...*models.Task) ProjectData {
	return ProjectData{
		Project: &models.Project{ID: "proj-" + name, Name: name},
		Tasks:   tasks,
	}
}

// Helper to create a task with specific fields set.
func testTask(title string, status models.TaskStatus, updatedAt time.Time) *models.Task {
	return &models.Task{
		ID:        "task-" + title,
		Title:     title,
		Status:    status,
		UpdatedAt: updatedAt,
	}
}

func timePtr(t time.Time) *time.Time { return &t }

// --- checkDailyReviewReminder ---

func TestCheckDailyReviewReminder_NeverCompleted(t *testing.T) {
	cfg := DefaultAlertConfig()
	cfg.LastDailyReviewAt = nil
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)

	alerts := checkDailyReviewReminder(cfg, nil, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %d", alerts[0].Severity)
	}
	if alerts[0].Title != "Daily review never completed" {
		t.Errorf("unexpected title: %s", alerts[0].Title)
	}
}

func TestCheckDailyReviewReminder_CompletedToday(t *testing.T) {
	now := time.Date(2026, 2, 28, 14, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.LastDailyReviewAt = timePtr(time.Date(2026, 2, 28, 8, 30, 0, 0, time.UTC))

	alerts := checkDailyReviewReminder(cfg, nil, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts when reviewed today, got %d", len(alerts))
	}
}

func TestCheckDailyReviewReminder_PastConfiguredHour(t *testing.T) {
	now := time.Date(2026, 2, 28, 11, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.DailyReviewHour = 9
	cfg.LastDailyReviewAt = timePtr(time.Date(2026, 2, 27, 10, 0, 0, 0, time.UTC))

	alerts := checkDailyReviewReminder(cfg, nil, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Title != "Daily review pending" {
		t.Errorf("unexpected title: %s", alerts[0].Title)
	}
	if alerts[0].Severity != SeverityInfo {
		t.Errorf("expected info severity, got %d", alerts[0].Severity)
	}
}

func TestCheckDailyReviewReminder_BeforeConfiguredHourButWithin26h(t *testing.T) {
	now := time.Date(2026, 2, 28, 7, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.DailyReviewHour = 9
	cfg.LastDailyReviewAt = timePtr(time.Date(2026, 2, 27, 10, 0, 0, 0, time.UTC))

	alerts := checkDailyReviewReminder(cfg, nil, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (within 26h and before configured hour), got %d", len(alerts))
	}
}

func TestCheckDailyReviewReminder_Over26hFallback(t *testing.T) {
	now := time.Date(2026, 2, 28, 7, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.DailyReviewHour = 9
	// Last review was 2 days ago
	cfg.LastDailyReviewAt = timePtr(time.Date(2026, 2, 26, 4, 0, 0, 0, time.UTC))

	alerts := checkDailyReviewReminder(cfg, nil, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert (26h fallback), got %d", len(alerts))
	}
	if alerts[0].Title != "Daily review overdue" {
		t.Errorf("unexpected title: %s", alerts[0].Title)
	}
	if alerts[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity for overdue, got %d", alerts[0].Severity)
	}
}

// --- checkWeeklyReviewReminder ---

func TestCheckWeeklyReviewReminder_NeverCompleted(t *testing.T) {
	cfg := DefaultAlertConfig()
	cfg.LastWeeklyReviewAt = nil

	alerts := checkWeeklyReviewReminder(cfg, nil, time.Now())
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Title != "Weekly review never completed" {
		t.Errorf("unexpected title: %s", alerts[0].Title)
	}
}

func TestCheckWeeklyReviewReminder_Over8Days(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC) // Saturday
	cfg := DefaultAlertConfig()
	cfg.LastWeeklyReviewAt = timePtr(time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC))

	alerts := checkWeeklyReviewReminder(cfg, nil, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Title != "Weekly review overdue" {
		t.Errorf("unexpected title: %s", alerts[0].Title)
	}
}

func TestCheckWeeklyReviewReminder_OnConfiguredDay(t *testing.T) {
	// Monday Feb 23, 2026
	now := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.WeeklyReviewDay = 1 // Monday
	// Last review was last Monday
	cfg.LastWeeklyReviewAt = timePtr(time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC))

	alerts := checkWeeklyReviewReminder(cfg, nil, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert on configured review day, got %d", len(alerts))
	}
	if alerts[0].Title != "Weekly review day" {
		t.Errorf("unexpected title: %s", alerts[0].Title)
	}
}

func TestCheckWeeklyReviewReminder_ReviewedToday(t *testing.T) {
	now := time.Date(2026, 2, 23, 14, 0, 0, 0, time.UTC) // Monday
	cfg := DefaultAlertConfig()
	cfg.WeeklyReviewDay = 1
	cfg.LastWeeklyReviewAt = timePtr(time.Date(2026, 2, 23, 9, 0, 0, 0, time.UTC))

	alerts := checkWeeklyReviewReminder(cfg, nil, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts when reviewed today, got %d", len(alerts))
	}
}

func TestCheckWeeklyReviewReminder_RecentNotOnDay(t *testing.T) {
	// Wednesday, review was Monday (2 days ago), well within 8 days
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC) // Wednesday
	cfg := DefaultAlertConfig()
	cfg.WeeklyReviewDay = 1
	cfg.LastWeeklyReviewAt = timePtr(time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC))

	alerts := checkWeeklyReviewReminder(cfg, nil, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (recent and not review day), got %d", len(alerts))
	}
}

// --- checkOverdueTasks ---

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

// --- checkDueSoonTasks ---

func TestCheckDueSoonTasks_DueWithinHorizon(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) // 1 day from now
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
	dueDate := time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC) // past
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
	dueDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC) // 15 days out
	task := testTask("far out", models.StatusTodo, now)
	task.DueDate = &dueDate

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkDueSoonTasks(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts for far-future task, got %d", len(alerts))
	}
}

// --- checkStaleTasks ---

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
	// Exactly 14 days ago — should NOT trigger (not strictly before threshold)
	task := testTask("boundary", models.StatusTodo, now.AddDate(0, 0, -14))

	projects := []ProjectData{testProject("proj1", task)}
	alerts := checkStaleTasks(DefaultAlertConfig(), projects, now)
	// AddDate(-14) and threshold is now.AddDate(0,0,-14), so task.UpdatedAt == threshold
	// Before check is strict: task.UpdatedAt.Before(threshold) → false
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts at exact boundary, got %d", len(alerts))
	}
}

// --- checkInboxBacklog ---

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

// --- checkBlockedTasks ---

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

// --- checkProjectHealth ---

func TestCheckProjectHealth_StaleProject(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	task := testTask("old", models.StatusTodo, now.AddDate(0, 0, -30))

	projects := []ProjectData{testProject("stale-proj", task)}
	alerts := checkProjectHealth(DefaultAlertConfig(), projects, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
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

// --- Evaluate (full integration) ---

func TestEvaluate_AllDisabled(t *testing.T) {
	cfg := DefaultAlertConfig()
	cfg.DailyReviewEnabled = false
	cfg.WeeklyReviewEnabled = false
	cfg.OverdueEnabled = false
	cfg.DueSoonEnabled = false
	cfg.StaleEnabled = false
	cfg.InboxEnabled = false
	cfg.BlockedEnabled = false
	cfg.ProjectHealthEnabled = false

	snapshot := Evaluate(cfg, nil)
	if snapshot.TotalCount != 0 {
		t.Errorf("expected 0 total alerts when all disabled, got %d", snapshot.TotalCount)
	}
	if len(snapshot.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(snapshot.Groups))
	}
}

func TestEvaluate_MixedScenario(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.LastDailyReviewAt = nil
	cfg.LastWeeklyReviewAt = timePtr(now.AddDate(0, 0, -1))

	dueDate := time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC)
	overdueTask := testTask("overdue", models.StatusTodo, now)
	overdueTask.DueDate = &dueDate

	staleTask := testTask("stale", models.StatusInProgress, now.AddDate(0, 0, -20))

	projects := []ProjectData{testProject("proj1", overdueTask, staleTask)}
	snapshot := Evaluate(cfg, projects)

	if snapshot.TotalCount < 3 {
		t.Errorf("expected at least 3 alerts (daily review + overdue + stale), got %d", snapshot.TotalCount)
	}

	categoryFound := map[AlertCategory]bool{}
	for _, g := range snapshot.Groups {
		categoryFound[g.Category] = true
	}
	if !categoryFound[CategoryDailyReview] {
		t.Error("expected daily review alert")
	}
	if !categoryFound[CategoryOverdue] {
		t.Error("expected overdue alert")
	}
	if !categoryFound[CategoryStale] {
		t.Error("expected stale alert")
	}
}

func TestEvaluate_GroupLabelIncludesCount(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.DailyReviewEnabled = false
	cfg.WeeklyReviewEnabled = false
	cfg.StaleEnabled = false
	cfg.InboxEnabled = false
	cfg.BlockedEnabled = false
	cfg.ProjectHealthEnabled = false
	cfg.DueSoonEnabled = false
	cfg.LastDailyReviewAt = timePtr(now)
	cfg.LastWeeklyReviewAt = timePtr(now)

	due1 := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	due2 := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	t1 := testTask("od1", models.StatusTodo, now)
	t1.DueDate = &due1
	t2 := testTask("od2", models.StatusTodo, now)
	t2.DueDate = &due2

	projects := []ProjectData{testProject("proj1", t1, t2)}
	snapshot := Evaluate(cfg, projects)

	if len(snapshot.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(snapshot.Groups))
	}
	if snapshot.Groups[0].Label != "Overdue (2)" {
		t.Errorf("expected label 'Overdue (2)', got '%s'", snapshot.Groups[0].Label)
	}
}

func TestEvaluate_SingleAlertNoCountInLabel(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.DailyReviewEnabled = false
	cfg.WeeklyReviewEnabled = false
	cfg.StaleEnabled = false
	cfg.InboxEnabled = false
	cfg.BlockedEnabled = false
	cfg.ProjectHealthEnabled = false
	cfg.DueSoonEnabled = false

	due := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	task := testTask("single-overdue", models.StatusTodo, now)
	task.DueDate = &due

	projects := []ProjectData{testProject("proj1", task)}
	snapshot := Evaluate(cfg, projects)

	if len(snapshot.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(snapshot.Groups))
	}
	if snapshot.Groups[0].Label != "Overdue" {
		t.Errorf("expected label 'Overdue' (no count for single), got '%s'", snapshot.Groups[0].Label)
	}
}

func TestEvaluate_EmptyProjects(t *testing.T) {
	cfg := DefaultAlertConfig()
	cfg.DailyReviewEnabled = false
	cfg.WeeklyReviewEnabled = false

	snapshot := Evaluate(cfg, nil)
	if snapshot.TotalCount != 0 {
		t.Errorf("expected 0 total with no projects and reviews disabled, got %d", snapshot.TotalCount)
	}
}

func TestEvaluate_MultipleProjects(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.DailyReviewEnabled = false
	cfg.WeeklyReviewEnabled = false
	cfg.StaleEnabled = false
	cfg.InboxEnabled = false
	cfg.BlockedEnabled = false
	cfg.ProjectHealthEnabled = false
	cfg.DueSoonEnabled = false

	due := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	t1 := testTask("od-p1", models.StatusTodo, now)
	t1.DueDate = &due
	t2 := testTask("od-p2", models.StatusInProgress, now)
	t2.DueDate = &due

	projects := []ProjectData{
		testProject("proj1", t1),
		testProject("proj2", t2),
	}
	snapshot := Evaluate(cfg, projects)
	if snapshot.TotalCount != 2 {
		t.Errorf("expected 2 overdue alerts from 2 projects, got %d", snapshot.TotalCount)
	}
}

// --- formatTimeAgo ---

func TestFormatTimeAgo(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		past     time.Time
		expected string
	}{
		{"minutes", now.Add(-30 * time.Minute), "30 minutes ago"},
		{"hours", now.Add(-5 * time.Hour), "5 hours ago"},
		{"yesterday", now.Add(-36 * time.Hour), "yesterday"},
		{"days", now.Add(-72 * time.Hour), "3 days ago"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatTimeAgo(now, tc.past)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}
