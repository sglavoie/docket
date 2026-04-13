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

// ReassignTaskProject updates the ProjectID for a set of task IDs across the
// FocusQueue, DailyPlan, and RecentTaskVisits. This must be called after a
// cross-project task move so that persisted references point to the new project.
// Returns the number of entries updated.
func (c *Config) ReassignTaskProject(taskIDs map[string]bool, newProjectID string) int {
	updated := 0
	for i := range c.FocusQueue {
		if taskIDs[c.FocusQueue[i].TaskID] && c.FocusQueue[i].ProjectID != newProjectID {
			c.FocusQueue[i].ProjectID = newProjectID
			updated++
		}
	}
	for i := range c.DailyPlan {
		if taskIDs[c.DailyPlan[i].TaskID] && c.DailyPlan[i].ProjectID != newProjectID {
			c.DailyPlan[i].ProjectID = newProjectID
			updated++
		}
	}
	for i := range c.RecentTaskVisits {
		if taskIDs[c.RecentTaskVisits[i].TaskID] && c.RecentTaskVisits[i].ProjectID != newProjectID {
			c.RecentTaskVisits[i].ProjectID = newProjectID
			updated++
		}
	}
	return updated
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
