package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global key messages first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear temporary status/error messages on any keypress
		m.statusMessage = ""
		m.err = nil

		switch msg.String() {
		case "ctrl+c":
			m.saveProjectTreeState()
			m.quitting = true
			return m, tea.Quit
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
				m.quitting = true
				return m, tea.Quit
			}
			if handled := m.handleEscKey(); handled {
				return m, nil
			}
		case ":":
			// Open command palette if no view has an active text input
			if !m.hasActiveTextInput() {
				return m, func() tea.Msg {
					return ShowCommandPaletteMsg{}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)

	case QuitMsg:
		m.saveProjectTreeState()
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
	if caps := m.currentViewCapabilities(); caps != nil && caps.ShouldCaptureEsc() {
		return false
	}
	if m.currentView() != ViewDashboard {
		closingView := m.currentView()
		m.popView()
		// Rebuild project tree when closing ProjectCycleManager since it reloads
		// all projects in buildProjectInfoList(), invalidating tree's task pointers
		if closingView == ViewProjectCycleManager {
			if m.projectTree != nil {
				m.projectTree.buildTree()
			}
			// Refresh dashboard to re-sort with pinned projects first
			if m.dashboard != nil {
				m.dashboard.Refresh(m.config)
			}
		}
		// Refresh project tree when closing Kanban since tasks may have changed
		if closingView == ViewKanban {
			m.reloadAndRefreshProjectTree()
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
	// Views that skip the external status bar get the full terminal height
	if m.helpView != nil {
		m.helpView.SetSize(msg.Width, msg.Height)
	}
	for _, vt := range []ViewType{ViewCommandPalette, ViewTutor} {
		if v := m.views.Get(vt); v != nil {
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
	}

	return cmd
}
