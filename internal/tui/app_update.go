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
			inputCmd, dismiss, resultMsg := m.quickNoteOverlay.handleKey(msg)
			if dismiss {
				m.quickNoteOverlay = nil
			}
			var batchCmds []tea.Cmd
			if inputCmd != nil {
				batchCmds = append(batchCmds, inputCmd)
			}
			if resultMsg != nil {
				captured := resultMsg
				batchCmds = append(batchCmds, func() tea.Msg { return captured })
			}
			return m, tea.Batch(batchCmds...)
		}

		// Intercept keys when semicolon prefix is pending
		if m.pendingSemicolonPrefix {
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
			case "b":
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
			case "o":
				return m, func() tea.Msg { return ShowGlobalOutlinerMsg{} }
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
			if !m.hasActiveTextInput() {
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
			// Don't intercept q if there's an active text input
			if m.hasActiveTextInput() {
				break
			}
			if cmd := m.toggleQueue(); cmd != nil {
				return m, m.batchWithFocusRefresh(cmd)
			}
			// Non-task views (including dashboard): q is a no-op
			return m, nil
		case "w":
			if m.hasActiveTextInput() {
				break
			}
			if cmd := m.toggleDailyPlan(); cmd != nil {
				return m, m.batchWithFocusRefresh(cmd)
			}
			// Fall through to view's own w handler (inbox processor, waiting, etc.)
		case "u":
			if m.hasActiveTextInput() {
				break
			}
			switch m.currentView() {
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
				if m.currentProject != nil {
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
		if m.pendingSemicolonPrefix {
			m.pendingSemicolonPrefix = false
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
		if v := m.views.Get(ViewRecentActivity); v != nil {
			if ra, ok := v.(*RecentActivityModel); ok {
				ra.loadTasks()
			}
		}
		if m.globalInboxProcessor != nil {
			m.globalInboxProcessor.loadAllInboxTasks()
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

	// Delegate to current view
	cmd := m.delegateToCurrentView(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
