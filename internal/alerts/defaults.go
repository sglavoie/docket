package alerts

import "time"

// AlertConfig holds all thresholds and toggles for alert evaluation.
type AlertConfig struct {
	// Review schedule
	DailyReviewHour int // Hour of day (0-23) to trigger daily review reminder
	WeeklyReviewDay int // Day of week (0=Sunday, 1=Monday, ..., 6=Saturday)

	// Review timestamps
	LastDailyReviewAt  *time.Time
	LastWeeklyReviewAt *time.Time

	// Per-type enabled toggles
	DailyReviewEnabled   bool
	WeeklyReviewEnabled  bool
	OverdueEnabled       bool
	DueSoonEnabled       bool
	StaleEnabled         bool
	InboxEnabled         bool
	BlockedEnabled       bool
	ProjectHealthEnabled bool

	// Thresholds
	DueSoonDays      int // Days before due date to trigger "due soon"
	StaleDays        int // Days without update to consider a task stale
	InboxThreshold   int // Number of inbox tasks before alerting
	BlockedDays      int // Days in blocked/in_progress before alerting
	ProjectStaleDays int // Days without any task update to flag project health
}

// DefaultAlertConfig returns an AlertConfig with sensible defaults.
func DefaultAlertConfig() AlertConfig {
	return AlertConfig{
		DailyReviewHour: 9,
		WeeklyReviewDay: 1, // Monday

		DailyReviewEnabled:   true,
		WeeklyReviewEnabled:  true,
		OverdueEnabled:       true,
		DueSoonEnabled:       true,
		StaleEnabled:         true,
		InboxEnabled:         true,
		BlockedEnabled:       true,
		ProjectHealthEnabled: true,

		DueSoonDays:      3,
		StaleDays:        14,
		InboxThreshold:   5,
		BlockedDays:      7,
		ProjectStaleDays: 14,
	}
}
