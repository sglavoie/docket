package config

import "time"

// Alert defaults.
const (
	DefaultAlertDailyReviewHour  = 9
	DefaultAlertWeeklyReviewDay  = 1 // Monday
	DefaultAlertDueSoonDays      = 3
	DefaultAlertStaleDays        = 14
	DefaultAlertInboxThreshold   = 5
	DefaultAlertBlockedDays      = 7
	DefaultAlertProjectStaleDays = 14
)

// GetLastDailyReviewAt returns the timestamp of the last daily review, or nil if never completed.
func (c *Config) GetLastDailyReviewAt() *time.Time {
	return c.LastDailyReviewAt
}

// SetLastDailyReviewAt sets the timestamp of the last daily review.
func (c *Config) SetLastDailyReviewAt(t time.Time) {
	c.LastDailyReviewAt = &t
}

// GetLastWeeklyReviewAt returns the timestamp of the last weekly review, or nil if never completed.
func (c *Config) GetLastWeeklyReviewAt() *time.Time {
	return c.LastWeeklyReviewAt
}

// SetLastWeeklyReviewAt sets the timestamp of the last weekly review.
func (c *Config) SetLastWeeklyReviewAt(t time.Time) {
	c.LastWeeklyReviewAt = &t
}

// ClearLastDailyReviewAt resets the last daily review timestamp.
func (c *Config) ClearLastDailyReviewAt() {
	c.LastDailyReviewAt = nil
}

// ClearLastWeeklyReviewAt resets the last weekly review timestamp.
func (c *Config) ClearLastWeeklyReviewAt() {
	c.LastWeeklyReviewAt = nil
}

// GetAlertDailyReviewEnabled returns whether daily review alerts are enabled. True if not set.
func (c *Config) GetAlertDailyReviewEnabled() bool {
	if c.AlertDailyReviewEnabled == nil {
		return true
	}
	return *c.AlertDailyReviewEnabled
}

// GetAlertWeeklyReviewEnabled returns whether weekly review alerts are enabled. True if not set.
func (c *Config) GetAlertWeeklyReviewEnabled() bool {
	if c.AlertWeeklyReviewEnabled == nil {
		return true
	}
	return *c.AlertWeeklyReviewEnabled
}

// GetAlertOverdueEnabled returns whether overdue task alerts are enabled. True if not set.
func (c *Config) GetAlertOverdueEnabled() bool {
	if c.AlertOverdueEnabled == nil {
		return true
	}
	return *c.AlertOverdueEnabled
}

// GetAlertDueSoonEnabled returns whether due-soon task alerts are enabled. True if not set.
func (c *Config) GetAlertDueSoonEnabled() bool {
	if c.AlertDueSoonEnabled == nil {
		return true
	}
	return *c.AlertDueSoonEnabled
}

// GetAlertStaleEnabled returns whether stale task alerts are enabled. True if not set.
func (c *Config) GetAlertStaleEnabled() bool {
	if c.AlertStaleEnabled == nil {
		return true
	}
	return *c.AlertStaleEnabled
}

// GetAlertInboxEnabled returns whether inbox backlog alerts are enabled. True if not set.
func (c *Config) GetAlertInboxEnabled() bool {
	if c.AlertInboxEnabled == nil {
		return true
	}
	return *c.AlertInboxEnabled
}

// GetAlertBlockedEnabled returns whether blocked task alerts are enabled. True if not set.
func (c *Config) GetAlertBlockedEnabled() bool {
	if c.AlertBlockedEnabled == nil {
		return true
	}
	return *c.AlertBlockedEnabled
}

// GetAlertProjectHealthEnabled returns whether project health alerts are enabled. True if not set.
func (c *Config) GetAlertProjectHealthEnabled() bool {
	if c.AlertProjectHealthEnabled == nil {
		return true
	}
	return *c.AlertProjectHealthEnabled
}

// GetAlertDueSoonDays returns the number of days before a due date to trigger "due soon" alerts.
func (c *Config) GetAlertDueSoonDays() int {
	if c.AlertDueSoonDays == nil {
		return DefaultAlertDueSoonDays
	}
	return *c.AlertDueSoonDays
}

// GetAlertStaleDays returns the number of days without updates for a task to be considered stale.
func (c *Config) GetAlertStaleDays() int {
	if c.AlertStaleDays == nil {
		return DefaultAlertStaleDays
	}
	return *c.AlertStaleDays
}

// GetAlertInboxThreshold returns the inbox task count that triggers an inbox backlog alert.
func (c *Config) GetAlertInboxThreshold() int {
	if c.AlertInboxThreshold == nil {
		return DefaultAlertInboxThreshold
	}
	return *c.AlertInboxThreshold
}

// GetAlertBlockedDays returns the number of days a task must be blocked before alerting.
func (c *Config) GetAlertBlockedDays() int {
	if c.AlertBlockedDays == nil {
		return DefaultAlertBlockedDays
	}
	return *c.AlertBlockedDays
}

// GetAlertProjectStaleDays returns the number of days without task updates to flag project health.
func (c *Config) GetAlertProjectStaleDays() int {
	if c.AlertProjectStaleDays == nil {
		return DefaultAlertProjectStaleDays
	}
	return *c.AlertProjectStaleDays
}

// GetAlertDailyReviewHour returns the hour of day (0-23) for daily review reminders.
// Out-of-range stored values fall back to the default to avoid alerts that never fire.
func (c *Config) GetAlertDailyReviewHour() int {
	if c.AlertDailyReviewHour == nil {
		return DefaultAlertDailyReviewHour
	}
	v := *c.AlertDailyReviewHour
	if v < 0 || v > 23 {
		return DefaultAlertDailyReviewHour
	}
	return v
}

// GetAlertWeeklyReviewDay returns the day of week (0=Sunday) for weekly review reminders.
// Out-of-range stored values fall back to the default to avoid alerts that never fire.
func (c *Config) GetAlertWeeklyReviewDay() int {
	if c.AlertWeeklyReviewDay == nil {
		return DefaultAlertWeeklyReviewDay
	}
	v := *c.AlertWeeklyReviewDay
	if v < 0 || v > 6 {
		return DefaultAlertWeeklyReviewDay
	}
	return v
}
