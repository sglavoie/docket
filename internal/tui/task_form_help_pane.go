package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/theme"
)

type helpEntry struct {
	key  string
	desc string
}

// renderFormHelpPane renders contextual keybinding hints for the current
// tab + focusIndex as a narrow column. width is the available character width.
func (m *TaskFormModel) renderFormHelpPane(width int) string {
	entries := m.helpPaneEntries()

	title := theme.MutedStyle.Render(m.helpPaneHeader())
	sep := theme.MutedStyle.Render(strings.Repeat("─", width))

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
		sb.WriteString(theme.MutedStyle.Render(padded))
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
		return "Due Date"
	case formTabChildren:
		return "Children"
	case formTabExtras:
		return "Extras"
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
	case formTabExtras:
		return m.extrasHelpEntries()
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
			{key: "0-3", desc: "set priority"},
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
		{key: "Ctrl+S", desc: "save task"},
	}
	entries = append([]helpEntry{{key: "Ctrl+T", desc: "add subtask"}}, entries...)
	return entries
}

func (m *TaskFormModel) extrasHelpEntries() []helpEntry {
	return []helpEntry{
		{key: "a", desc: "toggle attachments"},
		{key: "c", desc: "toggle comments"},
		{key: "Ctrl+L", desc: "add attachment"},
		{key: "Ctrl+Y", desc: "add comment"},
		{key: "Enter", desc: "edit/save item"},
		{key: "Ctrl+D", desc: "delete"},
		{key: "Ctrl+J/K", desc: "reorder attachments"},
		{key: "Ctrl+S", desc: "save task"},
	}
}
