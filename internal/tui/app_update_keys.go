package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *AppModel) handleGlobalKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) { //nolint:gocyclo
	// Handle pending sandbox reset confirmation before clearing status
	if m.pendingSandboxReset {
		switch msg.String() {
		case "y", "Y":
			return true, m.executeSandboxReset()
		case "n", "N", "esc":
			m.pendingSandboxReset = false
			m.pendingSandboxTaskCount = 0
			m.setInfoStatus("Canceled")
			return true, defaultStatusTimeout()
		default:
			// Keep showing the confirmation prompt
			return true, nil
		}
	}

	if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
		return true, cmd
	}

	if handled, cmd := m.handlePendingPrefixes(msg); handled {
		return true, cmd
	}

	// Intercept keys when peek overlay is active
	if m.peekActive {
		if msg.String() == "esc" {
			m.peekTask = nil
			m.peekActive = false
		}
		return true, nil
	}

	// Guard against stale currentProject (e.g., deleted by external CLI command)
	if m.currentView() != ViewDashboard && m.validateCurrentProject() {
		return true, defaultStatusTimeout()
	}

	switch msg.String() {
	case "ctrl+c":
		m.saveProjectTreeState()
		m.saveOutlinerFilterState()
		m.saveCrossOutlinerState()
		m.quitting = true
		return true, tea.Quit
	case "alt+o":
		return true, m.navigateHistoryBack()
	case "alt+i":
		return true, m.navigateHistoryForward()
	case "alt+l":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowMessageLogMsg{} }
		}
	case "|":
		if !m.hasActiveTextInput() {
			return true, m.handleQuickSwitchToggle()
		}
	case "#":
		if !m.hasActiveTextInput() && !m.hasActivePendingPrefix() {
			return true, m.handlePinnedPopupToggle()
		}
	case "esc":
		if handled, cmd := m.handleEscKey(); handled {
			return true, cmd
		}
	case "q":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
			break
		}
		if cmd := m.toggleQueue(); cmd != nil {
			return true, m.batchWithFocusRefresh(cmd)
		}
		return true, nil
	case "w":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
			break
		}
		if cmd := m.toggleDailyPlan(); cmd != nil {
			return true, m.batchWithFocusRefresh(cmd)
		}
	case "u":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
			break
		}
		if cmd := m.handleUndoToggle(); cmd != nil {
			return true, cmd
		}
	case "ctrl+z":
		if !m.hasActiveTextInput() && m.currentProject != nil && m.currentView() != ViewUndoHistory {
			project := m.currentProject
			return true, func() tea.Msg { return ShowUndoHistoryMsg{Project: project} }
		}
	case "ctrl+x":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowProjectPickerMsg{} }
		}
	case ":":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowCommandPaletteMsg{} }
		}
	case ";":
		if !m.hasActiveTextInput() {
			m.pendingSemicolonPrefix = true
			timeout := m.config.GetPopupTimeoutSeconds()
			if timeout <= 0 {
				return true, nil
			}
			return true, func() tea.Msg {
				time.Sleep(time.Duration(timeout) * time.Second)
				return ClearSemicolonPrefixMsg{}
			}
		}
	case "ctrl+,":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return SettingsMsg{} }
		}
	case "ctrl+i":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowGlobalInboxProcessorMsg{} }
		}
	case "ctrl+/":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowGlobalSearchMsg{} }
		}
	case `ctrl+\`:
		if !m.hasActiveTextInput() {
			if m.currentView() == ViewKeybindingExplorer {
				m.popView()
				return true, nil
			}
			return true, func() tea.Msg { return ShowKeybindingExplorerMsg{} }
		}
	case "shift+space":
		if !m.hasActiveTextInput() && !m.hasActiveMultiSelect() {
			task := m.getCurrentCursorTask()
			if task != nil {
				m.peekTask = task
				m.peekActive = true
			}
			return true, nil
		}
	}

	return false, nil
}

func (m *AppModel) handleActiveKeyOverlay(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.showQuickSwitch {
		_, cmd := m.handleQuickSwitchKey(msg)
		return true, cmd
	}
	if m.showPinnedPopup {
		_, cmd := m.handlePinnedPopupKey(msg)
		return true, cmd
	}
	if m.showProjectCycle {
		_, cmd := m.handleProjectCycleKey(msg)
		return true, cmd
	}
	if m.quickNoteOverlay != nil {
		dismiss, cmd := dispatchOverlayKey(m.quickNoteOverlay, msg)
		if dismiss {
			m.quickNoteOverlay = nil
		}
		return true, cmd
	}
	if m.globalProjectPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.globalProjectPicker, msg)
		if dismiss {
			m.globalProjectPicker = nil
		}
		return true, cmd
	}
	if m.globalTaskPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.globalTaskPicker, msg)
		if dismiss {
			m.globalTaskPicker = nil
		}
		return true, cmd
	}
	if m.statusQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.statusQuickPicker, msg)
		if dismiss {
			m.statusQuickPicker = nil
		}
		return true, cmd
	}
	if m.sectionQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.sectionQuickPicker, msg)
		if dismiss {
			m.sectionQuickPicker = nil
		}
		return true, cmd
	}
	if m.linkTypeQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.linkTypeQuickPicker, msg)
		if dismiss {
			m.linkTypeQuickPicker = nil
		}
		return true, cmd
	}
	if m.linkModeQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.linkModeQuickPicker, msg)
		if dismiss {
			m.linkModeQuickPicker = nil
		}
		return true, cmd
	}
	if m.priorityQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.priorityQuickPicker, msg)
		if dismiss {
			m.priorityQuickPicker = nil
		}
		return true, cmd
	}
	if m.dueDateQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.dueDateQuickPicker, msg)
		if dismiss {
			m.dueDateQuickPicker = nil
		}
		return true, cmd
	}
	if m.sectionChangeQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.sectionChangeQuickPicker, msg)
		if dismiss {
			m.sectionChangeQuickPicker = nil
		}
		return true, cmd
	}

	return false, nil
}

func (m *AppModel) handlePendingPrefixes(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.pendingSemicolonTPrefix {
		if isModifierOnly(msg) {
			return true, nil
		}
		m.pendingSemicolonTPrefix = false
		switch msg.String() {
		case "m":
			return true, func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeTaskMoveSource}
			}
		case "N":
			return true, func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeTemplatePick}
			}
		case "d":
			return true, func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeDueDate}
			}
		case "s":
			return true, func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeSectionChange}
			}
		}
		return true, nil
	}

	if !m.pendingSemicolonPrefix {
		return false, nil
	}

	if isModifierOnly(msg) {
		return true, nil
	}
	m.pendingSemicolonPrefix = false
	switch msg.String() {
	case ";":
		return true, func() tea.Msg { return ShowPipelineHubMsg{} }
	case "n":
		return true, func() tea.Msg { return ShowNotificationCenterMsg{} }
	case "d":
		return true, func() tea.Msg { return ShowDailyReviewMsg{} }
	case "r":
		return true, func() tea.Msg { return ShowWeeklyReviewMsg{} }
	case "N":
		return true, func() tea.Msg { return ShowGlobalNotesMsg{} }
	case "q":
		return true, func() tea.Msg { return ShowFocusViewMsg{} }
	case "w":
		return true, func() tea.Msg { return ShowFocusBoardMsg{} }
	case "l":
		return true, func() tea.Msg { return ShowRecentActivityMsg{} }
	case "B":
		return true, func() tea.Msg { return ShowGlobalBlockersMsg{} }
	case "D":
		return true, func() tea.Msg { return ShowDueDateOverviewMsg{} }
	case "m":
		return true, func() tea.Msg { return ShowMetricsDashboardMsg{} }
	case "a":
		return true, func() tea.Msg { return ShowAllTasksMsg{} }
	case "W":
		return true, func() tea.Msg { return ShowWorkflowRulesMsg{} }
	case "k":
		return true, func() tea.Msg { return ShowCrossProjectKanbanMsg{} }
	case "o":
		return true, func() tea.Msg { return ShowGlobalOutlinerMsg{} }
	case "R":
		return true, func() tea.Msg { return ShowRoadmapMsg{} }
	case "i":
		return true, m.resolveQuickCaptureCmd()
	case "I":
		return true, func() tea.Msg {
			return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeBulkCapture}
		}
	case "{":
		return true, func() tea.Msg {
			return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeNoteCreate}
		}
	case ",":
		return true, func() tea.Msg {
			return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeStatusUpdate}
		}
	case "c":
		return true, func() tea.Msg {
			return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeLinkedTask}
		}
	case "t":
		m.pendingSemicolonTPrefix = true
		return true, nil
	case "!":
		return true, func() tea.Msg {
			return ShowGlobalProjectPickerMsg{Purpose: PickerPurposePriorityUpdate}
		}
	}

	return true, nil
}

func (m *AppModel) handleQuickSwitchToggle() tea.Cmd {
	// On dashboard, dismiss tutor prompt if active.
	if m.currentView() == ViewDashboard && !m.config.GetTutorDismissed() {
		return func() tea.Msg { return TutorDismissedMsg{} }
	}
	if m.showQuickSwitch {
		m.showQuickSwitch = false
		return nil
	}
	m.openQuickSwitch()
	if len(m.recentTaskVisits) == 0 {
		m.showQuickSwitch = false
		m.setInfoStatus("No recent tasks for quick-switch")
		return defaultStatusTimeout()
	}
	return nil
}

func (m *AppModel) handlePinnedPopupToggle() tea.Cmd {
	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.setInfoStatus("No pinned tasks — press p on a task to pin it")
		return defaultStatusTimeout()
	}
	m.showPinnedPopup = !m.showPinnedPopup
	m.pinnedPopupCursor = 0
	m.pinnedPopupFilter = ""
	m.pinnedPopupFiltering = false
	m.rebuildPinnedPopupFilter()
	return nil
}

func (m *AppModel) handleUndoToggle() tea.Cmd {
	switch m.currentView() { //nolint:exhaustive
	case ViewFocusView:
		if cmd := m.undoDailyPlanToggle(); cmd != nil {
			return m.batchWithFocusRefresh(cmd)
		}
	case ViewFocusBoard:
		if cmd := m.undoDailyPlanToggle(); cmd != nil {
			return m.batchWithFocusRefresh(cmd)
		}
		if cmd := m.undoQueueToggle(); cmd != nil {
			return m.batchWithFocusRefresh(cmd)
		}
	}

	return nil
}
