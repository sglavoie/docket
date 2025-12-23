package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global key messages first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			if handled := m.handleEscKey(); handled {
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)
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
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return m, cmd
	}

	// Delegate to current view
	cmds = append(cmds, m.delegateToCurrentView(msg))

	return m, tea.Batch(cmds...)
}

// handleEscKey handles the escape key for view navigation.
// Returns true if the key was handled (should not propagate to views).
func (m *AppModel) handleEscKey() bool {
	// Let views handle esc if they have an active search
	if m.currentView() == ViewProjectTree && m.projectTree != nil && m.projectTree.HasActiveSearch() {
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
	if m.currentView() != ViewDashboard {
		m.popView()
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
	}

	return cmd
}
