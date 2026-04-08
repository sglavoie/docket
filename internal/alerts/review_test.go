package alerts

import (
	"testing"
	"time"
)

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
	now := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
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
	now := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.WeeklyReviewDay = 1
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
	now := time.Date(2026, 2, 23, 14, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.WeeklyReviewDay = 1
	cfg.LastWeeklyReviewAt = timePtr(time.Date(2026, 2, 23, 9, 0, 0, 0, time.UTC))

	alerts := checkWeeklyReviewReminder(cfg, nil, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts when reviewed today, got %d", len(alerts))
	}
}

func TestCheckWeeklyReviewReminder_RecentNotOnDay(t *testing.T) {
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	cfg := DefaultAlertConfig()
	cfg.WeeklyReviewDay = 1
	cfg.LastWeeklyReviewAt = timePtr(time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC))

	alerts := checkWeeklyReviewReminder(cfg, nil, now)
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (recent and not review day), got %d", len(alerts))
	}
}
