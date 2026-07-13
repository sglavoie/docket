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

// AddToFocusQueue appends a Focus Queue entry for the task if it is not already
// present, returning true when the queue changed. It is the config-aware
// primitive that the legacy active:true migration and the set_active workflow
// action route through: Focus Queue membership is the canonical replacement for
// the retired Task.Active flag under the workflow-state model. A duplicate
// (same TaskID) or an empty TaskID is a no-op.
func (c *Config) AddToFocusQueue(taskID, projectID string) bool {
	if taskID == "" {
		return false
	}
	for _, item := range c.FocusQueue {
		if item.TaskID == taskID {
			return false
		}
	}
	c.FocusQueue = append(c.FocusQueue, FocusQueueItem{TaskID: taskID, ProjectID: projectID})
	return true
}

// RemoveFromFocusQueue removes any Focus Queue entry for the task, returning
// true when the queue changed. It is the symmetric primitive to AddToFocusQueue
// used by the set_active(false) workflow action. An empty TaskID or an empty
// queue is a no-op.
func (c *Config) RemoveFromFocusQueue(taskID string) bool {
	if taskID == "" || len(c.FocusQueue) == 0 {
		return false
	}
	filtered := c.FocusQueue[:0]
	removed := false
	for _, item := range c.FocusQueue {
		if item.TaskID == taskID {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	c.FocusQueue = filtered
	return removed
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
//
// It is the convenience union of ReassignDurablePlanningProject (Focus Queue and
// Daily Plan, which are durable planning state) and ReassignRecentVisitsProject
// (recent visits, which are machine-local UI state). A caller that must commit the
// durable planning refs transactionally while updating recent visits separately —
// as the cross-project move CLI does — should call the two narrower methods
// directly rather than this union.
func (c *Config) ReassignTaskProject(taskIDs map[string]bool, newProjectID string) int {
	return c.ReassignDurablePlanningProject(taskIDs, newProjectID) +
		c.ReassignRecentVisitsProject(taskIDs, newProjectID)
}

// ReassignDurablePlanningProject updates the ProjectID for a set of task IDs across
// the durable planning collections only — FocusQueue and DailyPlan. It deliberately
// excludes the machine-local RecentTaskVisits so a cross-project move can fold the
// durable planning refs into the same atomic transaction as the task write while
// leaving recent-visit repair to a separate, non-authoritative update. Returns the
// number of entries updated.
func (c *Config) ReassignDurablePlanningProject(taskIDs map[string]bool, newProjectID string) int {
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
	return updated
}

// ReassignRecentVisitsProject updates the ProjectID for a set of task IDs in the
// machine-local RecentTaskVisits collection only. It is applied separately from the
// durable planning refs — after a successful durable commit — because recent visits
// are non-authoritative UI state whose loss does not corrupt the workspace. Returns
// the number of entries updated.
func (c *Config) ReassignRecentVisitsProject(taskIDs map[string]bool, newProjectID string) int {
	updated := 0
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
//
// It is the convenience union of RemoveDurableTaskRefs (Focus Queue, Daily Plan,
// and dismissals, which are durable planning state) and RemoveMachineLocalTaskRefs
// (recent visits, which are machine-local UI state). A caller that must commit the
// durable planning cleanup transactionally while repairing recent visits separately
// — as the task deletion CLI does — should call the two narrower methods directly
// rather than this union.
func (c *Config) RemoveTaskRefs(taskIDs map[string]bool) int {
	return c.RemoveDurableTaskRefs(taskIDs) + c.RemoveMachineLocalTaskRefs(taskIDs)
}

// RemoveDurableTaskRefs removes entries for the given task IDs from the durable
// planning collections only — FocusQueue, DailyPlan, and FocusDismissedIDs. It
// deliberately excludes the machine-local RecentTaskVisits so a task deletion can
// fold the durable planning cleanup into the same atomic transaction as the task
// removal while leaving recent-visit repair to a separate, non-authoritative update.
// Returns the number of entries removed.
func (c *Config) RemoveDurableTaskRefs(taskIDs map[string]bool) int {
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

// RemoveMachineLocalTaskRefs removes entries for the given task IDs from the
// machine-local RecentTaskVisits collection only. It is applied separately from the
// durable planning cleanup — after a successful durable commit — because recent
// visits are non-authoritative UI state whose loss does not corrupt the workspace.
// Returns the number of entries removed.
func (c *Config) RemoveMachineLocalTaskRefs(taskIDs map[string]bool) int {
	removed := 0
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
	return removed
}

// GetFocusBoardDefaultMode returns the default focus mode for Today.
// Returns "list" if not set.
func (c *Config) GetFocusBoardDefaultMode() string {
	if c.FocusBoardDefaultMode == "" {
		return "list"
	}
	return c.FocusBoardDefaultMode
}

// SetFocusBoardDefaultMode sets the default focus mode for Today.
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
