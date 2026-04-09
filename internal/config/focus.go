package config

// GetFocusQueue returns the persisted focus queue items.
func (c *Config) GetFocusQueue() []FocusQueueItem {
	return c.FocusQueue
}

// SetFocusQueue replaces the entire focus queue.
func (c *Config) SetFocusQueue(items []FocusQueueItem) {
	c.FocusQueue = items
}

// GetDailyPlan returns the persisted daily plan items.
func (c *Config) GetDailyPlan() []DailyPlanItem {
	return c.DailyPlan
}

// SetDailyPlan replaces the entire daily plan.
func (c *Config) SetDailyPlan(items []DailyPlanItem) {
	c.DailyPlan = items
}

// GetFocusDismissedIDs returns the persisted dismissed suggestion IDs.
func (c *Config) GetFocusDismissedIDs() []string {
	return c.FocusDismissedIDs
}

// SetFocusDismissedIDs replaces the entire dismissed ID list.
func (c *Config) SetFocusDismissedIDs(ids []string) {
	c.FocusDismissedIDs = ids
}

// maxFocusDismissedIDs is the maximum number of dismissed suggestion IDs to persist.
// When exceeded, the oldest entries (at the end of the slice) are dropped.
const maxFocusDismissedIDs = 200

// PruneFocusDismissedIDs trims the dismissed IDs list to the maximum allowed size.
// Returns true if any IDs were removed.
func (c *Config) PruneFocusDismissedIDs() bool {
	if len(c.FocusDismissedIDs) <= maxFocusDismissedIDs {
		return false
	}
	c.FocusDismissedIDs = c.FocusDismissedIDs[:maxFocusDismissedIDs]
	return true
}
