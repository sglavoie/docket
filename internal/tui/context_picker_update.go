package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// Update handles messages.
func (m *ContextPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if model, cmd, handled := m.handleConfirmDiscard(msg); handled {
			return model, cmd
		}
		if model, cmd, handled := m.handleRenameMode(msg); handled {
			return model, cmd
		}
		if model, cmd, handled := m.handleDeleteConfirm(msg); handled {
			return model, cmd
		}
		return m.handleDefaultKey(msg)
	}

	return m, nil
}

func (m *ContextPickerModel) handleConfirmDiscard(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if !m.confirmDiscard {
		return m, nil, false
	}

	switch msg.String() {
	case "y", "Y":
		m.confirmDiscard = false
		m.renameMode = false
		m.renameOldName = ""
	case "n", "N", "esc":
		m.confirmDiscard = false
	}

	return m, nil, true
}

func (m *ContextPickerModel) handleRenameMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if !m.renameMode {
		return m, nil, false
	}

	switch msg.String() {
	case "esc":
		if strings.TrimSpace(m.renameInput.Value()) != m.renameOldName {
			m.confirmDiscard = true
			return m, nil, true
		}
		m.renameMode = false
		m.renameOldName = ""
		return m, nil, true
	case "enter":
		return m.submitRename()
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd, true
	}
}

func (m *ContextPickerModel) submitRename() (tea.Model, tea.Cmd, bool) {
	newName := strings.TrimSpace(m.renameInput.Value())
	if newName == "" || newName == m.renameOldName {
		m.renameMode = false
		m.renameOldName = ""
		return m, nil, true
	}

	oldName := m.renameOldName
	count := 0
	for _, project := range m.config.Projects {
		if project.Context == oldName {
			project.Context = newName
			count++
		}
	}

	m.renameMode = false
	m.renameOldName = ""
	m.refreshContexts()

	return m, func() tea.Msg {
		return ContextRenamedMsg{OldName: oldName, NewName: newName, ProjectCount: count}
	}, true
}

func (m *ContextPickerModel) handleDeleteConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	confirmed, handled := m.confirm.HandleKey(msg.String())
	if !handled {
		return m, nil, false
	}
	if !confirmed {
		return m, nil, true
	}

	if len(m.Filtered) == 0 || m.Cursor >= len(m.Filtered) {
		return m, nil, true
	}

	selected := m.Filtered[m.Cursor]
	if !selected.IsCustom || m.config == nil {
		return m, nil, true
	}

	name := selected.Name
	wasActive := m.currentCustomFilter == name
	if wasActive {
		m.config.FilterCustomContext = ""
		m.currentCustomFilter = ""
	}
	m.config.DeleteCustomContext(name)
	m.config.SetCycleProjectsForContext(customContextID(name), nil)
	m.refreshContexts()
	m.clampCursor()

	return m, func() tea.Msg {
		return CustomContextDeletedMsg{Name: name, WasActive: wasActive}
	}, true
}

func (m *ContextPickerModel) handleDefaultKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		fromCycle := m.fromCycle
		return m, func() tea.Msg {
			return ContextPickerDoneMsg{Canceled: true, FromCycle: fromCycle}
		}
	case "enter":
		if len(m.Filtered) == 0 || m.Cursor >= len(m.Filtered) {
			return m, nil
		}
		selected := m.Filtered[m.Cursor]
		fromCycle := m.fromCycle
		return m, func() tea.Msg {
			return ContextPickerDoneMsg{
				SelectedContext: selected.Name,
				IsCustomContext: selected.IsCustom,
				Canceled:        false,
				FromCycle:       fromCycle,
			}
		}
	case "n":
		return m, func() tea.Msg {
			return ShowCustomContextEditorMsg{EditName: ""}
		}
	case "e":
		if selected, ok := m.selectedContext(); ok && selected.IsCustom {
			return m, func() tea.Msg {
				return ShowCustomContextEditorMsg{EditName: selected.Name}
			}
		}
		return m, nil
	case "d":
		return m.handleDeleteShortcut()
	case "r":
		return m.handleRenameShortcut()
	default:
		handled, cmd := m.HandleKeyMsg(msg)
		if handled {
			return m, cmd
		}
		return m, nil
	}
}

func (m *ContextPickerModel) handleDeleteShortcut() (tea.Model, tea.Cmd) {
	selected, ok := m.selectedContext()
	if !ok {
		return m, nil
	}

	if selected.IsCustom && m.config != nil {
		m.confirm.Show("delete", "Delete this custom context?")
		m.clearStatus()
		return m, nil
	}

	if selected.Name != "" {
		m.setStatus("cannot delete regular contexts.\nRename them with 'r' instead.")
	}
	return m, nil
}

func (m *ContextPickerModel) handleRenameShortcut() (tea.Model, tea.Cmd) {
	selected, ok := m.selectedContext()
	if !ok {
		return m, nil
	}
	if selected.IsCustom {
		m.setStatus("Use 'e' to edit custom contexts")
		return m, nil
	}
	if selected.Name == "" {
		return m, nil
	}

	m.renameMode = true
	m.renameOldName = selected.Name
	m.renameInput = textinput.New()
	m.renameInput.Placeholder = "New context name"
	m.renameInput.SetValue(selected.Name)
	m.renameInput.Focus()
	m.renameInput.CharLimit = 50
	applyThemedTextInput(&m.renameInput)
	m.clearStatus()
	return m, textinput.Blink
}

func (m *ContextPickerModel) selectedContext() (ContextInfo, bool) {
	if len(m.Filtered) == 0 || m.Cursor >= len(m.Filtered) {
		return ContextInfo{}, false
	}
	return m.Filtered[m.Cursor], true
}

func (m *ContextPickerModel) refreshContexts() {
	projects := m.config.GetActiveProjects()
	m.Items = buildContextListWithCustom(projects, m.config)
	m.Filter(m.SearchQuery())
}

func (m *ContextPickerModel) clampCursor() {
	if m.Cursor >= len(m.Filtered) {
		m.Cursor = len(m.Filtered) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}