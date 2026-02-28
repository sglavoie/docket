package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/storage"
)

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global key messages first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle pending sandbox reset confirmation before clearing status
		if m.pendingSandboxReset {
			switch msg.String() {
			case "y", "Y":
				return m, m.executeSandboxReset()
			case "n", "N", "esc":
				m.pendingSandboxReset = false
				m.pendingSandboxTaskCount = 0
				m.statusMessage = "Canceled"
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

		// Intercept keys when semicolon prefix is pending
		if m.pendingSemicolonPrefix {
			m.pendingSemicolonPrefix = false
			switch msg.String() {
			case "n":
				return m, func() tea.Msg { return ShowNotificationCenterMsg{} }
			case "s":
				return m, func() tea.Msg { return SettingsMsg{} }
			case "d":
				return m, func() tea.Msg { return ShowDailyReviewMsg{} }
			case "w":
				return m, func() tea.Msg { return ShowWeeklyReviewMsg{} }
			}
			return m, nil
		}

		// Clear temporary status/error messages on any keypress
		m.statusMessage = ""
		m.err = nil

		switch msg.String() {
		case "ctrl+c":
			m.saveProjectTreeState()
			m.saveOutlinerFilterState()
			m.saveKanbanState()
			m.quitting = true
			return m, tea.Quit
		case "alt+o":
			return m, m.navigateHistoryBack()
		case "alt+i":
			return m, m.navigateHistoryForward()
		case "|":
			if !m.hasActiveTextInput() {
				// On dashboard, dismiss tutor prompt if active
				if m.currentView() == ViewDashboard && !m.config.GetTutorDismissed() {
					return m, func() tea.Msg {
						return TutorDismissedMsg{}
					}
				}
				m.showQuickSwitch = !m.showQuickSwitch
				m.quickSwitchCursor = 0
				if m.showQuickSwitch {
					m.refreshRecentTaskVisits()
				}
				return m, nil
			}
		case "#":
			if !m.hasActiveTextInput() {
				m.pinnedPopupItems = m.collectPinnedTasks()
				if len(m.pinnedPopupItems) == 0 {
					m.statusMessage = "No pinned tasks"
					return m, defaultStatusTimeout()
				}
				m.showPinnedPopup = !m.showPinnedPopup
				m.pinnedPopupCursor = 0
				return m, nil
			}
		case "esc":
			if handled := m.handleEscKey(); handled {
				return m, nil
			}
		case "q":
			// Don't intercept q if there's an active text input
			if m.hasActiveTextInput() {
				break
			}
			// On dashboard, q quits the app
			if m.currentView() == ViewDashboard {
				m.saveProjectTreeState()
				m.saveOutlinerFilterState()
				m.saveKanbanState()
				m.quitting = true
				return m, tea.Quit
			}
			if handled := m.handleEscKey(); handled {
				return m, nil
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
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)

	case QuitMsg:
		m.saveProjectTreeState()
		m.saveOutlinerFilterState()
		m.saveKanbanState()
		m.quitting = true
		return m, tea.Quit

	case spinner.TickMsg:
		// Only update spinner if we're in loading state
		if m.isLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case SaveStartMsg:
		m.isLoading = true
		m.loadingMessage = msg.Message
		m.err = nil
		return m, m.spinner.Tick()

	case SaveCompleteMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.statusMessage = msg.SuccessMessage
		m.refreshProjectTree()
		// Refresh kanban if open (async save reloads project from disk)
		if m.kanban != nil {
			m.kanban.loadTasks()
		}
		// Refresh all-tasks if open (cross-project view may show stale data)
		if m.allTasksView != nil {
			m.allTasksView.loadAllTasks()
		}
		// Refresh outliner if open (async save reloads project from disk)
		if m.outliner != nil {
			_ = storage.LoadProject(m.outliner.project)
			m.outliner.rebuildRows()
		}
		return m, defaultStatusTimeout()

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

func (m *AppModel) currentViewCapabilities() ViewCapabilities {
	view := m.views.Get(m.currentView())
	if view == nil {
		return nil
	}
	if caps, ok := view.(ViewCapabilities); ok {
		return caps
	}
	return nil
}

// hasActiveTextInput returns true if the current view has an active text input
// that should receive keyboard input (like search boxes, text fields).
// This is used to determine whether global keyboard shortcuts should be intercepted.
func (m *AppModel) hasActiveTextInput() bool {
	if caps := m.currentViewCapabilities(); caps != nil {
		return caps.HasTextInput()
	}
	return false
}

// handleEscKey handles the escape/q key for view navigation.
// Returns true if the key was handled (should not propagate to views).
func (m *AppModel) handleEscKey() bool {
	if m.pendingSemicolonPrefix {
		m.pendingSemicolonPrefix = false
		return true
	}
	if caps := m.currentViewCapabilities(); caps != nil && caps.ShouldCaptureEsc() {
		return false
	}
	if m.currentView() != ViewDashboard {
		closingView := m.currentView()
		m.popView()
		// Persist kanban filter state and refresh project tree when closing Kanban
		if closingView == ViewKanban {
			m.saveKanbanState()
			m.reloadAndRefreshProjectTree()
		}
		// Restore previous project when popping a stacked project tree
		if closingView == ViewProjectTree && m.currentView() == ViewProjectTree && len(m.projectStack) > 0 {
			prevID := m.projectStack[len(m.projectStack)-1]
			m.projectStack = m.projectStack[:len(m.projectStack)-1]
			if prev := m.config.GetProject(prevID); prev != nil {
				m.switchToProject(prev, true)
			}
		}
		return true
	}
	return false
}

// handleWindowSize updates dimensions for all views.
func (m *AppModel) handleWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height
	m.views.SetSize(msg.Width, msg.Height-2)
	// Re-apply full terminal height for views that render their own footer
	for _, vt := range m.views.Types() {
		v := m.views.Get(vt)
		if v == nil {
			continue
		}
		if caps, ok := v.(ViewCapabilities); ok && caps.IsFullHeight() {
			v.SetSize(msg.Width, msg.Height)
		}
	}
}

// delegateToCurrentView forwards messages to the active view model.
func (m *AppModel) delegateToCurrentView(msg tea.Msg) tea.Cmd {
	currentView := m.currentView()
	cmd := m.views.Update(currentView, msg)

	// Keep typed pointers in sync for views that need direct access
	switch currentView {
	case ViewDashboard:
		if v := m.views.Get(ViewDashboard); v != nil {
			m.dashboard = v.(*DashboardModel)
		}
	case ViewProjectTree:
		if v := m.views.Get(ViewProjectTree); v != nil {
			m.projectTree = v.(*ProjectTreeModel)
		}
	case ViewHelp:
		if v := m.views.Get(ViewHelp); v != nil {
			m.helpView = v.(*HelpViewModel)
		}
	case ViewTaskJumper:
		if v := m.views.Get(ViewTaskJumper); v != nil {
			m.taskJumper = v.(*TaskJumperModel)
		}
	case ViewInboxProcessor:
		if v := m.views.Get(ViewInboxProcessor); v != nil {
			m.inboxProcessor = v.(*InboxProcessorModel)
		}
	case ViewGlobalInboxProcessor:
		if v := m.views.Get(ViewGlobalInboxProcessor); v != nil {
			m.globalInboxProcessor = v.(*GlobalInboxProcessorModel)
		}
	case ViewAllTasks:
		if v := m.views.Get(ViewAllTasks); v != nil {
			m.allTasksView = v.(*AllTasksViewModel)
		}
	case ViewOutliner:
		if v := m.views.Get(ViewOutliner); v != nil {
			m.outliner = v.(*OutlinerModel)
		}
	}

	return cmd
}
