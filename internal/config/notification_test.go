package config

import (
	"testing"
	"time"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestGetLastDailyReviewAt_Nil(t *testing.T) {
	c := &Config{}
	if c.GetLastDailyReviewAt() != nil {
		t.Error("expected nil for unset LastDailyReviewAt")
	}
}

func TestGetLastDailyReviewAt_Set(t *testing.T) {
	now := time.Now()
	c := &Config{}
	c.SetLastDailyReviewAt(now)
	got := c.GetLastDailyReviewAt()
	if got == nil || !got.Equal(now) {
		t.Errorf("expected %v, got %v", now, got)
	}
}

func TestGetLastWeeklyReviewAt_Nil(t *testing.T) {
	c := &Config{}
	if c.GetLastWeeklyReviewAt() != nil {
		t.Error("expected nil for unset LastWeeklyReviewAt")
	}
}

func TestGetLastWeeklyReviewAt_Set(t *testing.T) {
	now := time.Now()
	c := &Config{}
	c.SetLastWeeklyReviewAt(now)
	got := c.GetLastWeeklyReviewAt()
	if got == nil || !got.Equal(now) {
		t.Errorf("expected %v, got %v", now, got)
	}
}

func TestAlertEnabledGetters_DefaultTrue(t *testing.T) {
	c := &Config{}
	getters := []struct {
		name string
		fn   func() bool
	}{
		{"DailyReview", c.GetAlertDailyReviewEnabled},
		{"WeeklyReview", c.GetAlertWeeklyReviewEnabled},
		{"Overdue", c.GetAlertOverdueEnabled},
		{"DueSoon", c.GetAlertDueSoonEnabled},
		{"Stale", c.GetAlertStaleEnabled},
		{"Inbox", c.GetAlertInboxEnabled},
		{"Blocked", c.GetAlertBlockedEnabled},
		{"ProjectHealth", c.GetAlertProjectHealthEnabled},
	}
	for _, g := range getters {
		if !g.fn() {
			t.Errorf("%s: expected true when nil, got false", g.name)
		}
	}
}

func TestAlertEnabledGetters_ExplicitFalse(t *testing.T) {
	c := &Config{
		AlertDailyReviewEnabled:   boolPtr(false),
		AlertWeeklyReviewEnabled:  boolPtr(false),
		AlertOverdueEnabled:       boolPtr(false),
		AlertDueSoonEnabled:       boolPtr(false),
		AlertStaleEnabled:         boolPtr(false),
		AlertInboxEnabled:         boolPtr(false),
		AlertBlockedEnabled:       boolPtr(false),
		AlertProjectHealthEnabled: boolPtr(false),
	}
	getters := []struct {
		name string
		fn   func() bool
	}{
		{"DailyReview", c.GetAlertDailyReviewEnabled},
		{"WeeklyReview", c.GetAlertWeeklyReviewEnabled},
		{"Overdue", c.GetAlertOverdueEnabled},
		{"DueSoon", c.GetAlertDueSoonEnabled},
		{"Stale", c.GetAlertStaleEnabled},
		{"Inbox", c.GetAlertInboxEnabled},
		{"Blocked", c.GetAlertBlockedEnabled},
		{"ProjectHealth", c.GetAlertProjectHealthEnabled},
	}
	for _, g := range getters {
		if g.fn() {
			t.Errorf("%s: expected false when explicitly set, got true", g.name)
		}
	}
}

func TestAlertThresholdGetters_Defaults(t *testing.T) {
	c := &Config{}
	tests := []struct {
		name     string
		fn       func() int
		expected int
	}{
		{"DueSoonDays", c.GetAlertDueSoonDays, DefaultAlertDueSoonDays},
		{"StaleDays", c.GetAlertStaleDays, DefaultAlertStaleDays},
		{"InboxThreshold", c.GetAlertInboxThreshold, DefaultAlertInboxThreshold},
		{"BlockedDays", c.GetAlertBlockedDays, DefaultAlertBlockedDays},
		{"ProjectStaleDays", c.GetAlertProjectStaleDays, DefaultAlertProjectStaleDays},
		{"DailyReviewHour", c.GetAlertDailyReviewHour, DefaultAlertDailyReviewHour},
		{"WeeklyReviewDay", c.GetAlertWeeklyReviewDay, DefaultAlertWeeklyReviewDay},
	}
	for _, tc := range tests {
		if got := tc.fn(); got != tc.expected {
			t.Errorf("%s: expected %d, got %d", tc.name, tc.expected, got)
		}
	}
}

func TestGetAlertDailyReviewHour_ClampOutOfRange(t *testing.T) {
	cases := []int{-1, -100, 24, 25, 99}
	for _, v := range cases {
		c := &Config{AlertDailyReviewHour: intPtr(v)}
		if got := c.GetAlertDailyReviewHour(); got != DefaultAlertDailyReviewHour {
			t.Errorf("stored=%d: expected default %d, got %d", v, DefaultAlertDailyReviewHour, got)
		}
	}
	// Boundary valid values must pass through unchanged.
	for _, v := range []int{0, 12, 23} {
		c := &Config{AlertDailyReviewHour: intPtr(v)}
		if got := c.GetAlertDailyReviewHour(); got != v {
			t.Errorf("stored=%d: expected pass-through, got %d", v, got)
		}
	}
}

func TestGetAlertWeeklyReviewDay_ClampOutOfRange(t *testing.T) {
	cases := []int{-1, -7, 7, 8, 99}
	for _, v := range cases {
		c := &Config{AlertWeeklyReviewDay: intPtr(v)}
		if got := c.GetAlertWeeklyReviewDay(); got != DefaultAlertWeeklyReviewDay {
			t.Errorf("stored=%d: expected default %d, got %d", v, DefaultAlertWeeklyReviewDay, got)
		}
	}
	// Boundary valid values must pass through unchanged.
	for _, v := range []int{0, 3, 6} {
		c := &Config{AlertWeeklyReviewDay: intPtr(v)}
		if got := c.GetAlertWeeklyReviewDay(); got != v {
			t.Errorf("stored=%d: expected pass-through, got %d", v, got)
		}
	}
}

func TestAlertThresholdGetters_CustomValues(t *testing.T) {
	c := &Config{
		AlertDueSoonDays:      intPtr(7),
		AlertStaleDays:        intPtr(30),
		AlertInboxThreshold:   intPtr(10),
		AlertBlockedDays:      intPtr(14),
		AlertProjectStaleDays: intPtr(21),
		AlertDailyReviewHour:  intPtr(8),
		AlertWeeklyReviewDay:  intPtr(5), // Friday
	}
	tests := []struct {
		name     string
		fn       func() int
		expected int
	}{
		{"DueSoonDays", c.GetAlertDueSoonDays, 7},
		{"StaleDays", c.GetAlertStaleDays, 30},
		{"InboxThreshold", c.GetAlertInboxThreshold, 10},
		{"BlockedDays", c.GetAlertBlockedDays, 14},
		{"ProjectStaleDays", c.GetAlertProjectStaleDays, 21},
		{"DailyReviewHour", c.GetAlertDailyReviewHour, 8},
		{"WeeklyReviewDay", c.GetAlertWeeklyReviewDay, 5},
	}
	for _, tc := range tests {
		if got := tc.fn(); got != tc.expected {
			t.Errorf("%s: expected %d, got %d", tc.name, tc.expected, got)
		}
	}
}
