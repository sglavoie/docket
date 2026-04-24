package tui

import (
	"fmt"
	"runtime/debug"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// keyInterceptor is implemented by overlay widgets that intercept key presses
// and return (inputCmd, dismissed, resultMsg).
type keyInterceptor interface {
	handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg)
}

// dispatchOverlayKey forwards a key to an overlay and assembles the resulting
// tea.Cmd. The caller is responsible for nil-checking the overlay before
// calling and for niling the field when dismiss is true.
func dispatchOverlayKey(overlay keyInterceptor, msg tea.KeyPressMsg) (dismiss bool, cmd tea.Cmd) {
	inputCmd, dismissed, resultMsg := overlay.handleKey(msg)
	var cmds []tea.Cmd
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	if resultMsg != nil {
		captured := resultMsg
		cmds = append(cmds, func() tea.Msg { return captured })
	}
	return dismissed, tea.Batch(cmds...)
}

func (m *AppModel) Update(msg tea.Msg) (outModel tea.Model, outCmd tea.Cmd) { //nolint:gocyclo
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			logging.Warn("TUI panic in Update: %v\n%s", r, stack)
			m.err = fmt.Errorf("internal error: %v", r)
			if m.currentView() != ViewDashboard {
				m.viewStack = []ViewType{ViewDashboard}
			}
			m.pendingSemicolonPrefix = false
			m.pendingSemicolonTPrefix = false
			m.sectionQuickPicker = nil
			m.linkTypeQuickPicker = nil
			m.linkModeQuickPicker = nil
			m.isLoading = false
			m.showQuickSwitch = false
			m.quickSwitchFiltering = false
			m.quickSwitchShowCompleted = false
			m.showPinnedPopup = false
			m.pinnedPopupFiltering = false
			m.showProjectCycle = false
			m.pendingSandboxReset = false
			m.pendingBulkTasks = nil
			m.pendingBulkSectionTasks = nil
			m.pendingBulkNotes = nil
			m.statusMessage = ""
			m.loadingMessage = ""
			outModel = m
			outCmd = nil
		}
	}()

	var cmds []tea.Cmd

	// Handle global key messages first
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Handle pending sandbox reset confirmation before clearing status
		if m.pendingSandboxReset {
			switch msg.String() {
			case "y", "Y":
				return m, m.executeSandboxReset()
			case "n", "N", "esc":
				m.pendingSandboxReset = false
				m.pendingSandboxTaskCount = 0
				m.setInfoStatus("Canceled")
				return m, defaultStatusTimeout()
			default:
				// Keep showing the confirmation prompt
				return m, nil
			}
		}

		// Intercept keys when quick-switch popup is open
		if m.showQuickSwitch {
			return m.handleQuickSwitchKey(msg)
		}

		// Intercept keys when pinned tasks popup is open
		if m.showPinnedPopup {
			return m.handlePinnedPopupKey(msg)
		}

		// Intercept keys when project cycle popup is open
		if m.showProjectCycle {
			return m.handleProjectCycleKey(msg)
		}

		// Intercept keys when quick note overlay is active
		if m.quickNoteOverlay != nil {
			dismiss, cmd := dispatchOverlayKey(m.quickNoteOverlay, msg)
			if dismiss {
				m.quickNoteOverlay = nil
			}
			return m, cmd
		}

		// Intercept keys when global project picker overlay is active
		if m.globalProjectPicker != nil {
			dismiss, cmd := dispatchOverlayKey(m.globalProjectPicker, msg)
			if dismiss {
				m.globalProjectPicker = nil
			}
			return m, cmd
		}

		// Intercept keys when global task picker overlay is active
		if m.globalTaskPicker != nil {
			dismiss, cmd := dispatchOverlayKey(m.globalTaskPicker, msg)
			if dismiss {
				m.globalTaskPicker = nil
			}
			return m, cmd
		}

		// Intercept keys when status quick picker overlay is active
		if m.statusQuickPicker != nil {
			dismiss, cmd := dispatchOverlayKey(m.statusQuickPicker, msg)
			if dismiss {
				m.statusQuickPicker = nil
			}
			return m, cmd
		}

		// Intercept keys when section quick picker overlay is active
		if m.sectionQuickPicker != nil {
			dismiss, cmd := dispatchOverlayKey(m.sectionQuickPicker, msg)
			if dismiss {
				m.sectionQuickPicker = nil
			}
			return m, cmd
		}

		// Intercept keys when link type quick picker overlay is active
		if m.linkTypeQuickPicker != nil {
			dismiss, cmd := dispatchOverlayKey(m.linkTypeQuickPicker, msg)
			if dismiss {
				m.linkTypeQuickPicker = nil
			}
			return m, cmd
		}

		// Intercept keys when link mode quick picker overlay is active
		if m.linkModeQuickPicker != nil {
			dismiss, cmd := dispatchOverlayKey(m.linkModeQuickPicker, msg)
			if dismiss {
				m.linkModeQuickPicker = nil
			}
			return m, cmd
		}

		// Intercept keys when priority quick picker overlay is active
		if m.priorityQuickPicker != nil {
			dismiss, cmd := dispatchOverlayKey(m.priorityQuickPicker, msg)
			if dismiss {
				m.priorityQuickPicker = nil
			}
			return m, cmd
		}

		// Intercept keys when due date quick picker overlay is active
		if m.dueDateQuickPicker != nil {
			dismiss, cmd := dispatchOverlayKey(m.dueDateQuickPicker, msg)
			if dismiss {
				m.dueDateQuickPicker = nil
			}
			return m, cmd
		}

		// Intercept keys when section change quick picker overlay is active
		if m.sectionChangeQuickPicker != nil {
			dismiss, cmd := dispatchOverlayKey(m.sectionChangeQuickPicker, msg)
			if dismiss {
				m.sectionChangeQuickPicker = nil
			}
			return m, cmd
		}

		// Intercept keys when semicolon-t sub-prefix is pending (;t → "m" move, "N" template, "d" due date, "s" section)
		if m.pendingSemicolonTPrefix {
			// Ignore modifier-only key events (e.g. Shift pressed alone)
			// so the popup stays open while typing capital letters.
			if isModifierOnly(msg) {
				return m, nil
			}
			m.pendingSemicolonTPrefix = false
			switch msg.String() {
			case "m":
				return m, func() tea.Msg {
					return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeTaskMoveSource}
				}
			case "N":
				return m, func() tea.Msg {
					return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeTemplatePick}
				}
			case "d":
				return m, func() tea.Msg {
					return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeDueDate}
				}
			case "s":
				return m, func() tea.Msg {
					return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeSectionChange}
				}
			}
			return m, nil
		}

		// Intercept keys when semicolon prefix is pending
		if m.pendingSemicolonPrefix {
			// Ignore modifier-only key events (e.g. Shift pressed alone)
			// so the popup stays open while typing capital letters.
			if isModifierOnly(msg) {
				return m, nil
			}
			m.pendingSemicolonPrefix = false
			switch msg.String() {
			case "n":
				return m, func() tea.Msg { return ShowNotificationCenterMsg{} }
			case "d":
				return m, func() tea.Msg { return ShowDailyReviewMsg{} }
			case "w":
				return m, func() tea.Msg { return ShowWeeklyReviewMsg{} }
			case "N":
				return m, func() tea.Msg { return ShowGlobalNotesMsg{} }
			case "q":
				return m, func() tea.Msg { return ShowFocusViewMsg{} }
			case "f":
				return m, func() tea.Msg { return ShowFocusBoardMsg{} }
			case "l":
				return m, func() tea.Msg { return ShowRecentActivityMsg{} }
			case "B":
				return m, func() tea.Msg { return ShowGlobalBlockersMsg{} }
			case "D":
				return m, func() tea.Msg { return ShowDueDateOverviewMsg{} }
			case "m":
				return m, func() tea.Msg { return ShowMetricsDashboardMsg{} }
			case "a":
				return m, func() tea.Msg { return ShowAllTasksMsg{} }
			case "W":
				return m, func() tea.Msg { return ShowWorkflowRulesMsg{} }
			case "k":
				return m, func() tea.Msg { return ShowCrossProjectKanbanMsg{} }
			case "o":
				return m, func() tea.Msg { return ShowGlobalOutlinerMsg{} }
			case "i":
				return m, m.resolveQuickCaptureCmd()
			case "I":
				return m, func() tea.Msg {
					return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeBulkCapture}
				}
			case "{":
				return m, func() tea.Msg {
					return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeNoteCreate}
				}
			case ",":
				return m, func() tea.Msg {
					return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeStatusUpdate}
				}
			case "c":
				return m, func() tea.Msg {
					return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeLinkedTask}
				}
			case "t":
				m.pendingSemicolonTPrefix = true
				return m, nil
			case "!":
				return m, func() tea.Msg {
					return ShowGlobalProjectPickerMsg{Purpose: PickerPurposePriorityUpdate}
				}
			}
			return m, nil
		}

		// Intercept keys when peek overlay is active
		if m.peekActive {
			if msg.String() == "esc" {
				m.peekTask = nil
				m.peekActive = false
			}
			return m, nil
		}

		// Guard against stale currentProject (e.g., deleted by external CLI command)
		if m.currentView() != ViewDashboard && m.validateCurrentProject() {
			return m, defaultStatusTimeout()
		}

		switch msg.String() {
		case "ctrl+c":
			m.saveProjectTreeState()
			m.saveOutlinerFilterState()
			m.saveCrossOutlinerState()
			m.quitting = true
			return m, tea.Quit
		case "alt+o":
			return m, m.navigateHistoryBack()
		case "alt+i":
			return m, m.navigateHistoryForward()
		case "alt+l":
			if !m.hasActiveTextInput() {
				return m, func() tea.Msg { return ShowMessageLogMsg{} }
			}
		case "|":
			if !m.hasActiveTextInput() {
				// On dashboard, dismiss tutor prompt if active
				if m.currentView() == ViewDashboard && !m.config.GetTutorDismissed() {
					return m, func() tea.Msg {
						return TutorDismissedMsg{}
					}
				}
				if m.showQuickSwitch {
					m.showQuickSwitch = false
				} else {
					m.openQuickSwitch()
					if len(m.recentTaskVisits) == 0 {
						m.showQuickSwitch = false
						m.setInfoStatus("No recent tasks for quick-switch")
						return m, defaultStatusTimeout()
					}
				}
				return m, nil
			}
		case "#":
			if !m.hasActiveTextInput() && !m.hasActivePendingPrefix() {
				m.pinnedPopupItems = m.collectPinnedTasks()
				if len(m.pinnedPopupItems) == 0 {
					m.setInfoStatus("No pinned tasks — press p on a task to pin it")
					return m, defaultStatusTimeout()
				}
				m.showPinnedPopup = !m.showPinnedPopup
				m.pinnedPopupCursor = 0
				m.pinnedPopupFilter = ""
				m.pinnedPopupFiltering = false
				m.rebuildPinnedPopupFilter()
				return m, nil
			}
		case "esc":
			if handled, cmd := m.handleEscKey(); handled {
				return m, cmd
			}
		case "q":
			// Don't intercept q if there's an active text input or pending prefix
			if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
				break
			}
			if cmd := m.toggleQueue(); cmd != nil {
				return m, m.batchWithFocusRefresh(cmd)
			}
			// Non-task views (including dashboard): q is a no-op
			return m, nil
		case "w":
			if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
				break
			}
			if cmd := m.toggleDailyPlan(); cmd != nil {
				return m, m.batchWithFocusRefresh(cmd)
			}
			// Fall through to view's own w handler (inbox processor, waiting, etc.)
		case "u":
			if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
				break
			}
			switch m.currentView() { //nolint:exhaustive
			case ViewFocusView:
				if cmd := m.undoDailyPlanToggle(); cmd != nil {
					return m, m.batchWithFocusRefresh(cmd)
				}
			case ViewFocusBoard:
				if cmd := m.undoDailyPlanToggle(); cmd != nil {
					return m, m.batchWithFocusRefresh(cmd)
				}
				if cmd := m.undoQueueToggle(); cmd != nil {
					return m, m.batchWithFocusRefresh(cmd)
				}
			}
		case "ctrl+z":
			if !m.hasActiveTextInput() {
				if m.currentProject != nil && m.currentView() != ViewUndoHistory {
					project := m.currentProject
					return m, func() tea.Msg { return ShowUndoHistoryMsg{Project: project} }
				}
			}
		case "ctrl+x":
			// Open project picker if no view has an active text input
			if !m.hasActiveTextInput() {
				return m, func() tea.Msg {
					return ShowProjectPickerMsg{}
				}
			}
		case ":":
			// Open command palette if no view has an active text input
			if !m.hasActiveTextInput() {
				return m, func() tea.Msg {
					return ShowCommandPaletteMsg{}
				}
			}
		case ";":
			if !m.hasActiveTextInput() {
				m.pendingSemicolonPrefix = true
				timeout := m.config.GetPopupTimeoutSeconds()
				if timeout <= 0 {
					return m, nil
				}
				return m, func() tea.Msg {
					time.Sleep(time.Duration(timeout) * time.Second)
					return ClearSemicolonPrefixMsg{}
				}
			}
		case "ctrl+,":
			if !m.hasActiveTextInput() {
				return m, func() tea.Msg { return SettingsMsg{} }
			}
		case "ctrl+i":
			if !m.hasActiveTextInput() {
				return m, func() tea.Msg { return ShowGlobalInboxProcessorMsg{} }
			}
		case "ctrl+/":
			if !m.hasActiveTextInput() {
				return m, func() tea.Msg { return ShowGlobalSearchMsg{} }
			}
		case `ctrl+\`:
			if !m.hasActiveTextInput() {
				if m.currentView() == ViewKeybindingExplorer {
					m.popView()
					return m, nil
				}
				return m, func() tea.Msg { return ShowKeybindingExplorerMsg{} }
			}
		case "shift+space":
			if !m.hasActiveTextInput() && !m.hasActiveMultiSelect() {
				task := m.getCurrentCursorTask()
				if task != nil {
					m.peekTask = task
					m.peekActive = true
				}
			}
		}

	case tea.KeyboardEnhancementsMsg:
		m.keyboardEnhancements = msg
		return m, upgradeKittyKeyboard()

	case tea.KeyReleaseMsg:
		if msg.String() == "shift+space" && m.peekActive {
			m.peekTask = nil
			m.peekActive = false
		}

	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)

	case QuitMsg:
		m.saveProjectTreeState()
		m.saveOutlinerFilterState()
		m.saveCrossOutlinerState()
		m.quitting = true
		return m, tea.Quit

	case spinner.TickMsg:
		// Only update spinner if we're in loading state
		if m.isLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case ClearSemicolonPrefixMsg:
		if m.pendingSemicolonPrefix || m.pendingSemicolonTPrefix {
			m.pendingSemicolonPrefix = false
			m.pendingSemicolonTPrefix = false
			m.setInfoStatus("prefix cleared")
			return m, defaultStatusTimeout()
		}
		return m, nil

	case periodicSaveMsg:
		m.saveProjectTreeState()
		m.saveOutlinerFilterState()
		return m, tea.Tick(periodicSaveInterval, func(time.Time) tea.Msg {
			return periodicSaveMsg{}
		})

	case outlinerFilterStateChangedMsg:
		// Persist filter state immediately so navigation (e.g. go drill-down
		// from cross-outliner) reads the up-to-date cleared/changed values.
		switch m.currentView() { //nolint:exhaustive
		case ViewCrossProjectOutliner:
			m.saveCrossOutlinerState()
		default:
			m.saveOutlinerFilterState()
		}
		return m, nil

	case SaveStartMsg:
		m.isLoading = true
		m.loadingMessage = msg.Message
		m.err = nil
		return m, tea.Batch(
			m.spinner.Tick(),
			tea.Tick(10*time.Second, func(time.Time) tea.Msg { return SaveTimeoutMsg{} }),
		)

	case SaveTimeoutMsg:
		if m.isLoading {
			loadMsg := m.loadingMessage
			m.isLoading = false
			m.loadingMessage = ""
			if loadMsg != "" {
				m.err = fmt.Errorf("save timed out — %s", loadMsg)
			} else {
				m.err = fmt.Errorf("save timed out")
			}
			return m, statusTimeout(5 * time.Second)
		}
		return m, nil

	case SaveCompleteMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.setSuccessStatus(msg.SuccessMessage)
		m.invalidateFilterCaches()
		m.refreshProjectTree()
		// Refresh all-tasks if open (cross-project view may show stale data)
		if m.allTasksView != nil {
			m.allTasksView.loadAllTasks()
		}
		// Refresh outliner if open (async save reloads project from disk)
		if m.outliner != nil {
			if err := storage.LoadProject(m.outliner.project); err != nil {
				logging.Warn("failed to load project %s: %v", m.outliner.project.Name, err)
			}
			m.outliner.rebuildRows()
		}
		// Refresh cross-project views that may show stale data
		var saveCmds []tea.Cmd
		saveCmds = append(saveCmds, defaultStatusTimeout())
		if m.crossOutliner != nil {
			saveCmds = append(saveCmds, m.crossOutliner.reloadCrossProjects())
		}
		if v := m.views.Get(ViewFocusView); v != nil {
			if fv, ok := v.(*FocusViewModel); ok {
				fv.loadTasks()
			}
		}
		if v := m.views.Get(ViewFocusBoard); v != nil {
			if fb, ok := v.(*FocusBoardModel); ok {
				fb.loadItems()
				fb.clampCursor()
			}
		}
		if v := m.views.Get(ViewRecentActivity); v != nil {
			if ra, ok := v.(*RecentActivityModel); ok {
				ra.loadTasks()
			}
		}
		if m.globalInboxProcessor != nil {
			m.globalInboxProcessor.loadAllInboxTasks()
		}
		// Refresh kanban views so column distribution reflects saved changes.
		for _, vt := range []ViewType{ViewKanban, ViewCrossProjectKanban} {
			if v := m.views.Get(vt); v != nil {
				if kb, ok := v.(*KanbanModel); ok {
					kb.reloadData()
					kb.clampCursor()
				}
			}
		}
		return m, tea.Batch(saveCmds...)

	case SaveErrorMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.err = msg.Err
		return m, statusTimeout(5 * time.Second)

	case NotesRefreshMsg:
		// Rebuild global search index when notes change
		if m.globalSearch != nil {
			m.globalSearch.Rebuild()
		}
		// Mark notes search as stale so it reloads on next activation
		if v := m.views.Get(ViewNotesSearch); v != nil {
			if ns, ok := v.(*NotesSearchModel); ok {
				ns.MarkStale()
			}
		}
	}

	// Try each handler group in sequence
	if handled, cmd := m.handleProjectMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTaskMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleViewMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTagMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleSectionMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleInboxMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleProjectCycleMessages(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleExportMessages(msg); handled {
		return m, cmd
	}

	// Delegate to current view
	cmd := m.delegateToCurrentView(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// resolveQuickCaptureProject returns the best project for quick capture,
// preferring the dashboard selection when on the dashboard view.
func (m *AppModel) resolveQuickCaptureProject() tea.Msg {
	return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeQuickCapture}
}

func (m *AppModel) resolveQuickCaptureCmd() tea.Cmd {
	return func() tea.Msg { return m.resolveQuickCaptureProject() }
}

// isModifierOnly returns true when the key event is a bare modifier press
// (Shift, Ctrl, Alt, etc.) with no accompanying character.
func isModifierOnly(msg tea.KeyPressMsg) bool {
	switch msg.Code {
	case tea.KeyLeftShift, tea.KeyRightShift,
		tea.KeyLeftCtrl, tea.KeyRightCtrl,
		tea.KeyLeftAlt, tea.KeyRightAlt,
		tea.KeyLeftSuper, tea.KeyRightSuper,
		tea.KeyLeftHyper, tea.KeyRightHyper,
		tea.KeyLeftMeta, tea.KeyRightMeta:
		return true
	}
	return false
}
