package alerts

import (
	"testing"
	"time"
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
