package config

// GetReviewDueSoonDays returns the number of days for the "Due Soon" section.
// Returns 7 if not set, 0 means hide the section.
func (c *Config) GetReviewDueSoonDays() int {
	if c.ReviewDueSoonDays == nil {
		return 7
	}
	return *c.ReviewDueSoonDays
}

// GetReviewFutureDays returns the minimum days for the "Future" section.
// Tasks due in >= this many days appear in Future. Returns 30 if not set, 0 means hide the section.
func (c *Config) GetReviewFutureDays() int {
	if c.ReviewFutureDays == nil {
		return 30
	}
	return *c.ReviewFutureDays
}

// GetReviewOldDays returns the number of days for the "Old" section (based on UpdatedAt).
// Returns 14 if not set, 0 means hide the section.
func (c *Config) GetReviewOldDays() int {
	if c.ReviewOldDays == nil {
		return 14
	}
	return *c.ReviewOldDays
}

// GetReviewCompletedHours returns the number of hours for the "Completed" section.
// Returns 24 if not set, 0 means hide the section.
func (c *Config) GetReviewCompletedHours() int {
	if c.ReviewCompletedHours == nil {
		return 24
	}
	return *c.ReviewCompletedHours
}

// GetRecentActivityLimit returns the max number of tasks in the recent activity view.
// Returns 20 if not set.
func (c *Config) GetRecentActivityLimit() int {
	if c.RecentActivityLimit == nil {
		return 20
	}
	return *c.RecentActivityLimit
}

// GetRecentActivityShowAll returns whether to include completed tasks in recent activity.
// Returns true if not set.
func (c *Config) GetRecentActivityShowAll() bool {
	if c.RecentActivityShowAll == nil {
		return true
	}
	return *c.RecentActivityShowAll
}

// GetWeeklyStaleProjectDays returns the number of days for stale project detection.
// Returns 14 if not set.
func (c *Config) GetWeeklyStaleProjectDays() int {
	if c.WeeklyStaleProjectDays == nil {
		return 14
	}
	return *c.WeeklyStaleProjectDays
}

// GetWeeklyStaleTaskDays returns the number of days for stale task detection.
// Returns 21 if not set.
func (c *Config) GetWeeklyStaleTaskDays() int {
	if c.WeeklyStaleTaskDays == nil {
		return 21
	}
	return *c.WeeklyStaleTaskDays
}

// GetWeeklyInboxThreshold returns the inbox count threshold for weekly review.
// Returns 5 if not set.
func (c *Config) GetWeeklyInboxThreshold() int {
	if c.WeeklyInboxThreshold == nil {
		return 5
	}
	return *c.WeeklyInboxThreshold
}

// GetWeeklyBlockedThreshold returns the blocked count threshold for weekly review.
// Returns 3 if not set.
func (c *Config) GetWeeklyBlockedThreshold() int {
	if c.WeeklyBlockedThreshold == nil {
		return 3
	}
	return *c.WeeklyBlockedThreshold
}

// GetWeeklyInProgressThreshold returns the in-progress count threshold for weekly review.
// Returns 5 if not set.
func (c *Config) GetWeeklyInProgressThreshold() int {
	if c.WeeklyInProgressThreshold == nil {
		return 5
	}
	return *c.WeeklyInProgressThreshold
}

// GetLinkedTaskJumpToNew returns whether to jump to new task after linked creation.
// Returns false (stay on source task) if not set.
func (c *Config) GetLinkedTaskJumpToNew() bool {
	if c.LinkedTaskJumpToNew == nil {
		return false
	}
	return *c.LinkedTaskJumpToNew
}

// GetOutlinerCollapsedSections returns the default collapsed sections for the outliner.
// Returns an empty map if not set.
func (c *Config) GetOutlinerCollapsedSections() map[string]bool {
	if c.OutlinerCollapsedSections == nil {
		return make(map[string]bool)
	}
	return c.OutlinerCollapsedSections
}
