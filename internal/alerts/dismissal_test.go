package alerts

import (
	"testing"
	"time"
)

// --- DismissalKey ---

func TestDismissalKey_TaskSpecific(t *testing.T) {
	a := Alert{Category: CategoryOverdue, TaskID: "task-123", ProjectID: "proj-1"}
	got := a.DismissalKey()
	want := "overdue:task-123"
	if got != want {
		t.Errorf("DismissalKey() = %q, want %q", got, want)
	}
}

func TestDismissalKey_ProjectLevel(t *testing.T) {
	a := Alert{Category: CategoryProjectHealth, ProjectID: "proj-1"}
	got := a.DismissalKey()
	want := "project_health:proj-1"
	if got != want {
		t.Errorf("DismissalKey() = %q, want %q", got, want)
	}
}

func TestDismissalKey_Singleton(t *testing.T) {
	a := Alert{Category: CategoryDailyReview}
	got := a.DismissalKey()
	want := "daily_review"
	if got != want {
		t.Errorf("DismissalKey() = %q, want %q", got, want)
	}
}

func TestDismissalKey_TaskTakesPrecedenceOverProject(t *testing.T) {
	a := Alert{Category: CategoryBlocked, TaskID: "t1", ProjectID: "p1"}
	got := a.DismissalKey()
	if got != "blocked:t1" {
		t.Errorf("expected task ID to take precedence, got %q", got)
	}
}

// --- FilterDismissed ---

func TestFilterDismissed_NilSnapshot(t *testing.T) {
	result := FilterDismissed(nil, map[string]bool{"x": true})
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFilterDismissed_EmptyDismissedSet(t *testing.T) {
	snap := &AlertSnapshot{
		Groups:     []AlertGroup{{Category: CategoryOverdue, Label: "Overdue", Alerts: []Alert{{Category: CategoryOverdue, TaskID: "t1"}}}},
		TotalCount: 1,
	}
	result := FilterDismissed(snap, nil)
	if result != snap {
		t.Error("expected original snapshot returned when dismissed set is empty")
	}
}

func TestFilterDismissed_PartialRemoval(t *testing.T) {
	snap := &AlertSnapshot{
		Groups: []AlertGroup{{
			Category: CategoryOverdue,
			Label:    "Overdue (3)",
			Alerts: []Alert{
				{Category: CategoryOverdue, TaskID: "t1"},
				{Category: CategoryOverdue, TaskID: "t2"},
				{Category: CategoryOverdue, TaskID: "t3"},
			},
		}},
		TotalCount:  3,
		EvaluatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	dismissed := map[string]bool{"overdue:t2": true}
	result := FilterDismissed(snap, dismissed)

	if len(result.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(result.Groups))
	}
	if len(result.Groups[0].Alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(result.Groups[0].Alerts))
	}
	if result.Groups[0].Label != "Overdue (2)" {
		t.Errorf("expected label 'Overdue (2)', got %q", result.Groups[0].Label)
	}
	if result.TotalCount != 2 {
		t.Errorf("expected total 2, got %d", result.TotalCount)
	}
	if result.EvaluatedAt != snap.EvaluatedAt {
		t.Error("EvaluatedAt should be preserved")
	}
}

func TestFilterDismissed_SingleAlertLabel(t *testing.T) {
	snap := &AlertSnapshot{
		Groups: []AlertGroup{{
			Category: CategoryDueSoon,
			Label:    "Due Soon (2)",
			Alerts: []Alert{
				{Category: CategoryDueSoon, TaskID: "t1"},
				{Category: CategoryDueSoon, TaskID: "t2"},
			},
		}},
		TotalCount: 2,
	}

	dismissed := map[string]bool{"due_soon:t1": true}
	result := FilterDismissed(snap, dismissed)

	if result.Groups[0].Label != "Due Soon" {
		t.Errorf("single-alert group should have no count suffix, got %q", result.Groups[0].Label)
	}
}

func TestFilterDismissed_EntireGroupRemoved(t *testing.T) {
	snap := &AlertSnapshot{
		Groups: []AlertGroup{
			{
				Category: CategoryOverdue,
				Label:    "Overdue",
				Alerts:   []Alert{{Category: CategoryOverdue, TaskID: "t1"}},
			},
			{
				Category: CategoryStale,
				Label:    "Stale",
				Alerts:   []Alert{{Category: CategoryStale, TaskID: "t2"}},
			},
		},
		TotalCount: 2,
	}

	dismissed := map[string]bool{"overdue:t1": true}
	result := FilterDismissed(snap, dismissed)

	if len(result.Groups) != 1 {
		t.Fatalf("expected 1 group remaining, got %d", len(result.Groups))
	}
	if result.Groups[0].Category != CategoryStale {
		t.Errorf("expected stale group to remain, got %s", result.Groups[0].Category)
	}
	if result.TotalCount != 1 {
		t.Errorf("expected total 1, got %d", result.TotalCount)
	}
}

func TestFilterDismissed_AllDismissed(t *testing.T) {
	snap := &AlertSnapshot{
		Groups: []AlertGroup{{
			Category: CategoryDailyReview,
			Label:    "Daily Review",
			Alerts:   []Alert{{Category: CategoryDailyReview}},
		}},
		TotalCount: 1,
	}

	dismissed := map[string]bool{"daily_review": true}
	result := FilterDismissed(snap, dismissed)

	if len(result.Groups) != 0 {
		t.Errorf("expected no groups, got %d", len(result.Groups))
	}
	if result.TotalCount != 0 {
		t.Errorf("expected total 0, got %d", result.TotalCount)
	}
}

// --- ShouldExpireDismissal ---

func TestShouldExpire_DailyReview_BeforeMidnight(t *testing.T) {
	dismissed := time.Date(2026, 2, 28, 15, 0, 0, 0, time.UTC)
	now := time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC)
	if ShouldExpireDismissal("daily_review", dismissed, now) {
		t.Error("should not expire before midnight")
	}
}

func TestShouldExpire_DailyReview_AtMidnight(t *testing.T) {
	dismissed := time.Date(2026, 2, 28, 15, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !ShouldExpireDismissal("daily_review", dismissed, now) {
		t.Error("should expire at midnight")
	}
}

func TestShouldExpire_DailyReview_AfterMidnight(t *testing.T) {
	dismissed := time.Date(2026, 2, 28, 15, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	if !ShouldExpireDismissal("daily_review", dismissed, now) {
		t.Error("should expire after midnight")
	}
}

func TestShouldExpire_WeeklyReview_Before7Days(t *testing.T) {
	dismissed := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 2, 27, 9, 59, 59, 0, time.UTC)
	if ShouldExpireDismissal("weekly_review", dismissed, now) {
		t.Error("should not expire before 7 days")
	}
}

func TestShouldExpire_WeeklyReview_At7Days(t *testing.T) {
	dismissed := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 2, 27, 10, 0, 0, 0, time.UTC)
	if !ShouldExpireDismissal("weekly_review", dismissed, now) {
		t.Error("should expire at 7 days")
	}
}

func TestShouldExpire_Inbox_Before24h(t *testing.T) {
	dismissed := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 1, 9, 59, 59, 0, time.UTC)
	if ShouldExpireDismissal("inbox", dismissed, now) {
		t.Error("should not expire before 24h")
	}
}

func TestShouldExpire_Inbox_At24h(t *testing.T) {
	dismissed := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	if !ShouldExpireDismissal("inbox", dismissed, now) {
		t.Error("should expire at 24h")
	}
}

func TestShouldExpire_ProjectHealth_At24h(t *testing.T) {
	dismissed := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	if !ShouldExpireDismissal("project_health", dismissed, now) {
		t.Error("project_health should expire at 24h")
	}
}

func TestShouldExpire_Overdue_NeverExpires(t *testing.T) {
	dismissed := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	if ShouldExpireDismissal("overdue", dismissed, now) {
		t.Error("overdue should never time-expire")
	}
}

func TestShouldExpire_DueSoon_NeverExpires(t *testing.T) {
	dismissed := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	if ShouldExpireDismissal("due_soon", dismissed, now) {
		t.Error("due_soon should never time-expire")
	}
}

func TestShouldExpire_Stale_NeverExpires(t *testing.T) {
	dismissed := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	if ShouldExpireDismissal("stale", dismissed, now) {
		t.Error("stale should never time-expire")
	}
}

func TestShouldExpire_Blocked_NeverExpires(t *testing.T) {
	dismissed := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	if ShouldExpireDismissal("blocked", dismissed, now) {
		t.Error("blocked should never time-expire")
	}
}
