package alerts

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

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

func TestFormatTimeAgo(t *testing.T) {
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		past     time.Time
		expected string
	}{
		{"just now", now.Add(-10 * time.Second), "just now"},
		{"1 minute", now.Add(-90 * time.Second), "1 minute ago"},
		{"minutes", now.Add(-30 * time.Minute), "30 minutes ago"},
		{"1 hour", now.Add(-90 * time.Minute), "1 hour ago"},
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
