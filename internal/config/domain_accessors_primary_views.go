package config

// Domain-native accessors backing the primary TUI views (dashboard, all-tasks,
// outliner, project tree, kanban, focus board/view, inbox processors, reviews,
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

// ReassignDurablePlanningProject updates project ownership for moved task IDs
// in the durable planning collections.
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

// ReassignRecentVisitsProject updates project ownership for moved task IDs in
// machine-local recent history.
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
