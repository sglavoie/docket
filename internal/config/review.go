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
