package config

// Domain-native accessors backing the primary TUI views (dashboard, Outliner,
// Planner, inbox processors, reviews,
// due-date/metrics/priority/roadmap/pipeline). Each mirrors the semantics of
// its (*Config) counterpart so a view migrated off the facade observes the
// same defaults.

// GetTimeFormat mirrors (*Config).GetTimeFormat: "12h" → "3:04pm", anything
// else (including unset) → "15:04". Safe to call on a nil receiver.
func (s *Settings) GetTimeFormat() string {
	if s != nil && s.TimeFormat == "12h" {
		return "3:04pm"
	}
	return "15:04"
}

// GetWeeklyStaleProjectDays mirrors (*Config).GetWeeklyStaleProjectDays.
func (s *Settings) GetWeeklyStaleProjectDays() int {
	if s.WeeklyStaleProjectDays == nil {
		return 14
	}
	return *s.WeeklyStaleProjectDays
}

// GetWeeklyStaleTaskDays mirrors (*Config).GetWeeklyStaleTaskDays.
func (s *Settings) GetWeeklyStaleTaskDays() int {
	if s.WeeklyStaleTaskDays == nil {
		return 21
	}
	return *s.WeeklyStaleTaskDays
}

// GetWeeklyInboxThreshold mirrors (*Config).GetWeeklyInboxThreshold.
func (s *Settings) GetWeeklyInboxThreshold() int {
	if s.WeeklyInboxThreshold == nil {
		return 5
	}
	return *s.WeeklyInboxThreshold
}

// GetWeeklyBlockedThreshold mirrors (*Config).GetWeeklyBlockedThreshold.
func (s *Settings) GetWeeklyBlockedThreshold() int {
	if s.WeeklyBlockedThreshold == nil {
		return 3
	}
	return *s.WeeklyBlockedThreshold
}

// GetWeeklyInProgressThreshold mirrors (*Config).GetWeeklyInProgressThreshold.
func (s *Settings) GetWeeklyInProgressThreshold() int {
	if s.WeeklyInProgressThreshold == nil {
		return 5
	}
	return *s.WeeklyInProgressThreshold
}

// GetPriorityEscalationDays mirrors (*Config).GetPriorityEscalationDays.
func (s *Settings) GetPriorityEscalationDays() int {
	if s.PriorityEscalationDays == nil {
		return 3
	}
	return *s.PriorityEscalationDays
}

// IsDescriptionColumnVisible reports the fixed Task Workspace default.
func (m *MachineState) IsDescriptionColumnVisible() bool {
	return true
}

// RemoveDurableTaskRefs mirrors (*Config).RemoveDurableTaskRefs on the
// planning domain: it removes entries for the given task IDs from FocusQueue,
// DailyPlan, and FocusDismissedIDs (the durable planning collections) and
// returns the number of entries removed. Machine-local recent visits are
// repaired separately via (*MachineState).RemoveMachineLocalTaskRefs.
func (p *PlanningState) RemoveDurableTaskRefs(taskIDs map[string]bool) int {
	removed := 0
	if len(p.FocusQueue) > 0 {
		filtered := p.FocusQueue[:0]
		for _, item := range p.FocusQueue {
			if taskIDs[item.TaskID] {
				removed++
				continue
			}
			filtered = append(filtered, item)
		}
		p.FocusQueue = filtered
	}
	if len(p.DailyPlan) > 0 {
		filtered := p.DailyPlan[:0]
		for _, item := range p.DailyPlan {
			if taskIDs[item.TaskID] {
				removed++
				continue
			}
			filtered = append(filtered, item)
		}
		p.DailyPlan = filtered
	}
	if len(p.FocusDismissedIDs) > 0 {
		filtered := p.FocusDismissedIDs[:0]
		for _, id := range p.FocusDismissedIDs {
			if taskIDs[id] {
				removed++
				continue
			}
			filtered = append(filtered, id)
		}
		p.FocusDismissedIDs = filtered
	}
	return removed
}

// RemoveDurableTaskLocators removes project-qualified planning entries. The
// legacy dismissed-ID list has no project column, so qualified cleanup leaves
// it unchanged rather than risking deletion of another project's reference.
func (p *PlanningState) RemoveDurableTaskLocators(locators []ProjectTaskLocator) int {
	qualified := make(map[ProjectTaskLocator]bool, len(locators))
	for _, locator := range locators {
		if locator.ProjectID == "" || locator.TaskID == "" {
			continue
		}
		qualified[locator] = true
	}
	removed := 0
	if len(p.FocusQueue) > 0 {
		filtered := p.FocusQueue[:0]
		for _, item := range p.FocusQueue {
			if qualified[ProjectTaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}] {
				removed++
				continue
			}
			filtered = append(filtered, item)
		}
		p.FocusQueue = filtered
	}
	if len(p.DailyPlan) > 0 {
		filtered := p.DailyPlan[:0]
		for _, item := range p.DailyPlan {
			if qualified[ProjectTaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}] {
				removed++
				continue
			}
			filtered = append(filtered, item)
		}
		p.DailyPlan = filtered
	}
	return removed
}

// RemoveMachineLocalTaskRefs mirrors (*Config).RemoveMachineLocalTaskRefs on
// the machine domain: it removes RecentTaskVisits entries for the given task
// IDs and returns the number removed.
func (m *MachineState) RemoveMachineLocalTaskRefs(taskIDs map[string]bool) int {
	removed := 0
	if len(m.RecentTaskVisits) > 0 {
		filtered := m.RecentTaskVisits[:0]
		for _, visit := range m.RecentTaskVisits {
			if taskIDs[visit.TaskID] {
				removed++
				continue
			}
			filtered = append(filtered, visit)
		}
		m.RecentTaskVisits = filtered
	}
	return removed
}

func (m *MachineState) RemoveMachineLocalTaskLocators(locators []ProjectTaskLocator) int {
	qualified := make(map[ProjectTaskLocator]bool, len(locators))
	for _, locator := range locators {
		if locator.ProjectID != "" && locator.TaskID != "" {
			qualified[locator] = true
		}
	}
	removed := 0
	filtered := m.RecentTaskVisits[:0]
	for _, visit := range m.RecentTaskVisits {
		if qualified[ProjectTaskLocator{ProjectID: visit.ProjectID, TaskID: visit.TaskID}] {
			removed++
			continue
		}
		filtered = append(filtered, visit)
	}
	m.RecentTaskVisits = filtered
	return removed
}

// ReassignDurablePlanningProjectFrom updates project ownership only for entries
// that currently name both the source project and one of the moved task IDs.
// This is the project-qualified move seam; ReassignDurablePlanningProject is
// retained below only for compatibility with callers that genuinely lack an
// owning project.
func (p *PlanningState) ReassignDurablePlanningProjectFrom(taskIDs map[string]bool, sourceProjectID, newProjectID string) int {
	if sourceProjectID == "" || newProjectID == "" || sourceProjectID == newProjectID {
		return 0
	}
	updated := 0
	for i := range p.FocusQueue {
		if p.FocusQueue[i].ProjectID == sourceProjectID && taskIDs[p.FocusQueue[i].TaskID] {
			p.FocusQueue[i].ProjectID = newProjectID
			updated++
		}
	}
	for i := range p.DailyPlan {
		if p.DailyPlan[i].ProjectID == sourceProjectID && taskIDs[p.DailyPlan[i].TaskID] {
			p.DailyPlan[i].ProjectID = newProjectID
			updated++
		}
	}
	return updated
}

// ReassignRecentVisitsProjectFrom updates machine-local recent history only for
// entries that currently name both the source project and a moved task ID.
func (m *MachineState) ReassignRecentVisitsProjectFrom(taskIDs map[string]bool, sourceProjectID, newProjectID string) int {
	if sourceProjectID == "" || newProjectID == "" || sourceProjectID == newProjectID {
		return 0
	}
	updated := 0
	for i := range m.RecentTaskVisits {
		if m.RecentTaskVisits[i].ProjectID == sourceProjectID && taskIDs[m.RecentTaskVisits[i].TaskID] {
			m.RecentTaskVisits[i].ProjectID = newProjectID
			updated++
		}
	}
	return updated
}

// ReassignDurablePlanningProject updates project ownership by task ID alone. It
// is retained as a compatibility seam for callers that lack project ownership;
// known-owner production paths must use ReassignDurablePlanningProjectFrom.
func (p *PlanningState) ReassignDurablePlanningProject(taskIDs map[string]bool, newProjectID string) int {
	updated := 0
	for i := range p.FocusQueue {
		if taskIDs[p.FocusQueue[i].TaskID] && p.FocusQueue[i].ProjectID != newProjectID {
			p.FocusQueue[i].ProjectID = newProjectID
			updated++
		}
	}
	for i := range p.DailyPlan {
		if taskIDs[p.DailyPlan[i].TaskID] && p.DailyPlan[i].ProjectID != newProjectID {
			p.DailyPlan[i].ProjectID = newProjectID
			updated++
		}
	}
	return updated
}

// ReassignRecentVisitsProject updates project ownership by task ID alone. It is
// retained as a compatibility seam for callers that lack project ownership;
// known-owner production paths must use ReassignRecentVisitsProjectFrom.
func (m *MachineState) ReassignRecentVisitsProject(taskIDs map[string]bool, newProjectID string) int {
	updated := 0
	for i := range m.RecentTaskVisits {
		if taskIDs[m.RecentTaskVisits[i].TaskID] && m.RecentTaskVisits[i].ProjectID != newProjectID {
			m.RecentTaskVisits[i].ProjectID = newProjectID
			updated++
		}
	}
	return updated
}
