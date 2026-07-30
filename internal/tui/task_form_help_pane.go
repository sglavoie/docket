package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type helpEntry struct {
	key  string
	desc string
}

// renderFormHelpPane renders contextual keybinding hints for the current
// tab + focusIndex as a narrow column. width is the available character width.
func (m *TaskFormModel) renderFormHelpPane(width int) string {
	entries := m.helpPaneEntries()

	title := MutedStyle.Render(m.helpPaneHeader())
	sep := MutedStyle.Render(strings.Repeat("─", width))

	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString(sep)

	maxKeyW := 0
	for _, e := range entries {
		if w := lipgloss.Width(e.key); w > maxKeyW {
			maxKeyW = w
		}
	}
	for _, e := range entries {
		sb.WriteString("\n ")
		pad := maxKeyW - lipgloss.Width(e.key)
		padded := e.key + strings.Repeat(" ", pad)
		sb.WriteString(MutedStyle.Render(padded))
		if e.desc != "" {
			sb.WriteString("  ")
			sb.WriteString(e.desc)
		}
	}

	return sb.String()
}

// helpPaneHeader returns a field-specific title for the help pane.
func (m *TaskFormModel) helpPaneHeader() string {
	switch m.activeTab {
	case formTabBasics:
		names := []string{"Title", "Description", "Priority", "Parent", "State", "Flags", "Tags"}
		if m.focusIndex < len(names) {
			return names[m.focusIndex]
		}
	case formTabSchedule:
		switch m.focusIndex {
		case 0:
			return "Due Date"
		case 1:
			return "Recurrence"
		}
	case formTabChildren:
		return "Children"
	}
	return "Keys"
}

func (m *TaskFormModel) helpPaneEntries() []helpEntry {
	switch m.activeTab {
	case formTabBasics:
		return m.basicsHelpEntries()
	case formTabSchedule:
		return m.scheduleHelpEntries()
	case formTabChildren:
		return m.childrenHelpEntries()
	}
	return nil
}

func (m *TaskFormModel) basicsHelpEntries() []helpEntry {
	switch m.focusIndex {
	case 0: // Title
		return []helpEntry{
			{key: "Tab", desc: "next field"},
		}
	case 1: // Description
		return []helpEntry{
			{key: "Enter", desc: "edit"},
			{key: "Ctrl+E", desc: "ext editor"},
		}
	case 2: // Priority
		return []helpEntry{
			{key: "1-5", desc: "set priority"},
			{key: "j/k ↑/↓", desc: "cycle"},
			{key: "Enter", desc: "picker"},
		}
	case 3: // Parent
		return []helpEntry{
			{key: "Enter", desc: "select or clear parent"},
		}
	case 4: // State
		return []helpEntry{
			{key: "Enter", desc: "pick state"},
		}
	case 5: // Flags
		return []helpEntry{
			{key: "Enter", desc: "toggle flags"},
		}
	case 6: // Tags
		return []helpEntry{
			{key: "Enter", desc: "manage tags"},
		}
	}
	return nil
}

func (m *TaskFormModel) scheduleHelpEntries() []helpEntry {
	switch m.focusIndex {
	case 0: // Due
		return []helpEntry{
			{key: "Ctrl+D", desc: "presets"},
			{key: "Enter", desc: "confirm"},
		}
	case 1: // Recurrence
		return []helpEntry{
			{key: "Enter", desc: "edit pattern"},
		}
	}
	return nil
}

func (m *TaskFormModel) childrenHelpEntries() []helpEntry {
	entries := []helpEntry{
		{key: "Enter", desc: "edit/save item"},
		{key: "Ctrl+A", desc: "add item"},
		{key: "Space", desc: "toggle done"},
		{key: "Ctrl+D", desc: "delete"},
		{key: "Ctrl+J/K", desc: "reorder"},
		{key: "Ctrl+O", desc: "toggle optional"},
		{key: "Ctrl+S", desc: "save task"},
	}
	if m.mode == FormModeCreate {
		entries = append([]helpEntry{{key: "Ctrl+T", desc: "add subtask"}}, entries...)
	}
	return entries
}
