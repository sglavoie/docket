package tui

import tea "charm.land/bubbletea/v2"

func (m *RecentActivityModel) Init() tea.Cmd {
	return nil
}

func (m *RecentActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadTasks, m.clampCursor, m.setStatus); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

func (m *RecentActivityModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Handle search mode input
	if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		m.applyFilter()
		m.clampCursor()
	}); handled {
		return model, cmd
	}

	// Handle pending filter prefix keys
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		return m.handleRecentActivityPrefixKey(msg.String())
	}

	switch msg.String() {
	case "esc":
		if RunEscCascade(m.escSteps()) {
			return m, nil
		}
		if m.projectScope != nil {
			return m, func() tea.Msg {
				return CloseProjectRecentActivityMsg{}
			}
		}
		return m, func() tea.Msg {
			return CloseRecentActivityMsg{}
		}

	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.filtered) {
			dt := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return RecentActivitySelectMsg(dt)
			}
		}

	case "ctrl+.":
		if m.cursor >= 0 && m.cursor < len(m.filtered) {
			dt := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return OpenQuickNoteMsg(dt)
			}
		}

	case "g":
		m.pendingPrefix = "g"
		return m, nil

	case "p":
		if m.cursor >= 0 && m.cursor < len(m.filtered) {
			dt := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return ShowRecentActivityPreviewMsg{Task: dt, Project: dt.Project}
			}
		}

	case "tab":
		m.tabs.Cycle(1)
		m.applyFilter()
		m.clampCursor()
		return m, nil
	case "shift+tab":
		m.tabs.Cycle(-1)
		m.applyFilter()
		m.clampCursor()
		return m, nil
	case "shift+left", "shift+right":
		if handleTabCycleKey(msg.String(), &m.tabs, func() {
			m.applyFilter()
			m.clampCursor()
		}) {
			return m, nil
		}

	case "r":
		m.loadTasks()
		m.clampCursor()

	case "F":
		m.pendingPrefix = "F"
		return m, nil

	case "/":
		return m, m.ActivateSearch()

	case "?":
		viewType := ViewRecentActivity
		viewName := "Recent Activity"
		if m.projectScope != nil {
			viewType = ViewProjectRecentActivity
			viewName = "Project Activity"
		}
		return m, helpCmd(viewType, viewName, m.GetHelpItems())
	}

	if handled, cmd := handleListNavigation(msg.String(), &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	return m, nil
}

// handleRecentActivityPrefixKey handles prefix key sequences for recent activity.
func (m *RecentActivityModel) handleRecentActivityPrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

	if key == "esc" {
		return m, nil
	}

	switch prefix {
	case "g":
		if key == "g" {
			m.cursor = 0
		}
		return m, nil
	case "F":
		return m.handleRecentActivityFilterPrefix(key)
	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		return m.handleRecentActivityFilterSubPrefix(prefix, key)
	}

	return m, nil
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *RecentActivityModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	// All filter categories enabled for recent activity
	hints := buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true)
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}
