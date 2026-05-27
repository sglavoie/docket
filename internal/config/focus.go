package config

// FocusQueueItem stores a task reference for the focus queue.
type FocusQueueItem struct {
	TaskID    string `json:"task_id"`
	ProjectID string `json:"project_id"`
}

// DailyPlanItem stores a task reference for the daily plan.
type DailyPlanItem struct {
	TaskID      string `json:"task_id"`
	ProjectID   string `json:"project_id"`
	PlannedDate string `json:"planned_date"` // YYYY-MM-DD
}

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

// RemoveTaskRefs removes entries for the given task IDs from FocusQueue,
// DailyPlan, RecentTaskVisits, and FocusDismissedIDs. This must be called
// when tasks are deleted or archived so that stale references don't persist
// in config.
// Returns the number of entries removed.
func (c *Config) RemoveTaskRefs(taskIDs map[string]bool) int {
	removed := 0
	if len(c.FocusQueue) > 0 {
		filtered := c.FocusQueue[:0]
		for _, item := range c.FocusQueue {
			if taskIDs[item.TaskID] {
				removed++
				continue
			}
			filtered = append(filtered, item)
		}
		c.FocusQueue = filtered
	}
	if len(c.DailyPlan) > 0 {
		filtered := c.DailyPlan[:0]
		for _, item := range c.DailyPlan {
			if taskIDs[item.TaskID] {
				removed++
				continue
			}
			filtered = append(filtered, item)
		}
		c.DailyPlan = filtered
	}
	if len(c.RecentTaskVisits) > 0 {
		filtered := c.RecentTaskVisits[:0]
		for _, visit := range c.RecentTaskVisits {
			if taskIDs[visit.TaskID] {
				removed++
				continue
			}
			filtered = append(filtered, visit)
		}
		c.RecentTaskVisits = filtered
	}
	if len(c.FocusDismissedIDs) > 0 {
		filtered := c.FocusDismissedIDs[:0]
		for _, id := range c.FocusDismissedIDs {
			if taskIDs[id] {
				removed++
				continue
			}
			filtered = append(filtered, id)
		}
		c.FocusDismissedIDs = filtered
	}
	return removed
}

// GetFocusBoardDefaultMode returns the default focus mode for the Focus Board.
// Returns "list" if not set.
func (c *Config) GetFocusBoardDefaultMode() string {
	if c.FocusBoardDefaultMode == "" {
		return "list"
	}
	return c.FocusBoardDefaultMode
}

// SetFocusBoardDefaultMode sets the default focus mode for the Focus Board.
func (c *Config) SetFocusBoardDefaultMode(mode string) {
	c.FocusBoardDefaultMode = mode
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
