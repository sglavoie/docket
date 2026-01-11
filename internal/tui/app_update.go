package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global key messages first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear temporary status message on any keypress
		m.statusMessage = ""

		switch msg.String() {
		case "ctrl+c":
			m.saveProjectTreeState()
			m.quitting = true
			return m, tea.Quit
		case "esc":
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

// hasActiveTextInput returns true if the current view has an active text input
// that should receive keyboard input (like search boxes, text fields).
// This is used to determine whether global keyboard shortcuts should be intercepted.
func (m *AppModel) hasActiveTextInput() bool {
	// Check if the current view has an active search input
	switch m.currentView() {
	case ViewProjectTree:
		if m.projectTree != nil && m.projectTree.HasActiveSearch() {
			return true
		}
	case ViewDashboard:
		if m.dashboard != nil && m.dashboard.HasActiveSearch() {
			return true
		}
	case ViewHelp:
		if m.helpView != nil && m.helpView.HasActiveSearch() {
			return true
		}
	case ViewTaskJumper:
		// Task jumper always has active search
		return true
	case ViewGlobalSearch:
		// Global search always has active search
		return true
	case ViewCommandPalette:
		// Command palette always has active search
		return true
	case ViewTagSelector:
		if selector, ok := m.views.Get(ViewTagSelector).(*TagSelectorModel); ok && selector != nil && selector.HasActiveInput() {
			return true
		}
	case ViewParentSelector:
		// Parent selector always has active search
		return true
	case ViewTaskCreator, ViewNoteCreator, ViewQuickCapture, ViewBulkCapture:
		// Form views always have active text input
		return true
	case ViewTaskEditor:
		// Task editor has active text input
		return true
	}
	return false
}

// handleEscKey handles the escape key for view navigation.
// Returns true if the key was handled (should not propagate to views).
func (m *AppModel) handleEscKey() bool {
	// Let views handle esc if they have an active search, focus mode, or sidebar focus
	if m.currentView() == ViewProjectTree && m.projectTree != nil &&
		(m.projectTree.HasActiveSearch() || m.projectTree.IsFocusMode() || m.projectTree.sidebarFocused) {
		return false
	}
	if m.currentView() == ViewDashboard && m.dashboard != nil && m.dashboard.HasActiveSearch() {
		return false
	}
	if m.currentView() == ViewHelp && m.helpView != nil && m.helpView.HasActiveSearch() {
		return false
	}
	if m.currentView() == ViewTaskJumper && m.taskJumper != nil {
		return false
	}
	if m.currentView() == ViewTagSelector {
		if selector, ok := m.views.Get(ViewTagSelector).(*TagSelectorModel); ok && selector != nil && selector.HasActiveInput() {
			return false
		}
	}
	if m.currentView() == ViewCommandPalette {
		if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok && palette != nil && palette.IsActionMode() {
			return false
		}
	}
	if m.currentView() != ViewDashboard {
		closingView := m.currentView()
		m.popView()
		// Rebuild project tree when closing ProjectCycleManager since it reloads
		// all projects in buildProjectInfoList(), invalidating tree's task pointers
		if closingView == ViewProjectCycleManager && m.projectTree != nil {
			m.projectTree.buildTree()
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
	}

	return cmd
}
