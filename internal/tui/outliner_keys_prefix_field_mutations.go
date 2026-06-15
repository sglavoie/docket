package tui

import tea "charm.land/bubbletea/v2"

// handlePerspectivePrefix handles the P prefix key for perspective switching.
func (m *OutlinerModel) handlePerspectivePrefix(key string) (tea.Model, tea.Cmd) {
	// Perspective switching (P-prefix: P0-P9)
	if key >= "0" && key <= "9" {
		perspKey := int(key[0] - '0')
		if m.project == nil {
			return m.crossHandlePerspectiveJump(perspKey)
		}
		m.activePerspective = perspKey
		m.resetViewScrollOffset()
		m.rebuildRows()
		return m, nil
	}

	return m, nil
}

// handleStagePrefixOutliner handles the s prefix key for setting workflow stage.
func (m *OutlinerModel) handleStagePrefixOutliner(key string) (tea.Model, tea.Cmd) {
	if m.project == nil {
		if cmd, ok := m.crossHandleStageMutation(key); ok {
			return m, cmd
		}
		return m, nil
	}
	if cmd, ok := handleStagePrefix(key, m, m.config.Stages); ok {
		return m, cmd
	}
	return m, nil
}

// handlePriorityPrefixOutliner handles the ! prefix key for setting priority.
func (m *OutlinerModel) handlePriorityPrefixOutliner(key string) (tea.Model, tea.Cmd) {
	if m.project == nil {
		if cmd, ok := m.crossHandlePriorityMutation(key); ok {
			return m, cmd
		}
		return m, nil
	}
	if cmd, ok := handlePriorityPrefix(key, m); ok {
		return m, cmd
	}
	return m, nil
}

// handleDueDatePrefixOutliner handles the d prefix key for setting due dates.
func (m *OutlinerModel) handleDueDatePrefixOutliner(key string) (tea.Model, tea.Cmd) {
	if refs := m.getSelectedTaskProjectRefs(); len(refs) > 0 {
		if key == "d" {
			return m, func() tea.Msg { return SetDueDateMsg{TaskRefs: refs} }
		}
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok && m.project == nil {
			return m.crossApplyBulkDueDate(refs, days, isClear)
		}
	}
	if m.project == nil {
		if cmd, ok := m.crossHandleDueDateMutation(key); ok {
			return m, cmd
		}
		return m, nil
	}
	if cmd, ok := handleDueDatePrefix(key, m); ok {
		return m, cmd
	}
	return m, nil
}

// handleSnoozePrefixOutliner handles the s prefix key for snoozing tasks.
func (m *OutlinerModel) handleSnoozePrefixOutliner(key string) (tea.Model, tea.Cmd) {
	if refs := m.getSelectedTaskProjectRefs(); len(refs) > 0 {
		if key == "d" {
			return m, func() tea.Msg { return SetSnoozeDateMsg{TaskRefs: refs} }
		}
		if snoozeDate, desc, ok := resolveQuickSnoozeDate(key); ok && m.project == nil {
			return m.crossApplyBulkSnooze(refs, snoozeDate, desc)
		}
	}
	if m.project == nil {
		if cmd, ok := m.crossHandleSnoozeMutation(key); ok {
			return m, cmd
		}
		return m, nil
	}
	if cmd, ok := handleSnoozePrefix(key, m); ok {
		return m, cmd
	}
	return m, nil
}

// handleSkipRecurrenceOutliner handles the ,k key for skipping the next
// occurrence of a recurring task, routing to the cross-project variant when
// no single project is active.
func (m *OutlinerModel) handleSkipRecurrenceOutliner() (tea.Model, tea.Cmd) {
	if m.project == nil {
		cmd, _ := m.crossHandleSkipRecurrence()
		return m, cmd
	}
	cmd, _ := handleSkipRecurrence(m)
	return m, cmd
}

// handleDeferRecurrenceOutliner handles the ,D prefix for deferring a
// recurring task, routing to the cross-project variant when no single project
// is active.
func (m *OutlinerModel) handleDeferRecurrenceOutliner(key string) (tea.Model, tea.Cmd) {
	if m.project == nil {
		cmd, _ := m.crossHandleDeferRecurrence(key)
		return m, cmd
	}
	cmd, _ := handleDeferRecurrence(key, m)
	return m, cmd
}
