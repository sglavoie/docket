package taskform

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/theme"
)

type helpEntry struct {
	key  string
	desc string
}

// renderFormHelpPane renders contextual keybinding hints for the current
// tab + focusIndex as a narrow column, at the width its own entries need. It is
// reference text, so it is never shortened: the caller measures what comes back
// and drops the whole column when the body cannot spare those columns, which is
// the one honest answer when the alternative is half a key name.
func (m *TaskFormModel) renderFormHelpPane() string {
	entries := m.helpPaneEntries()

	maxKeyW := 0
	for _, e := range entries {
		if w := lipgloss.Width(e.key); w > maxKeyW {
			maxKeyW = w
		}
	}

	lines := make([]string, 0, len(entries))
	width := lipgloss.Width(m.helpPaneHeader())
	for _, e := range entries {
		line := " " + theme.MutedStyle.Render(e.key+strings.Repeat(" ", maxKeyW-lipgloss.Width(e.key)))
		if e.desc != "" {
			line += "  " + e.desc
		}
		lines = append(lines, line)
		width = max(width, lipgloss.Width(line))
	}

	var sb strings.Builder
	sb.WriteString(theme.MutedStyle.Render(m.helpPaneHeader()))
	sb.WriteString("\n")
	sb.WriteString(theme.MutedStyle.Render(strings.Repeat("─", width)))
	for _, line := range lines {
		sb.WriteString("\n")
		sb.WriteString(line)
	}

	return sb.String()
}

// helpPaneHeader returns a field-specific title for the help pane.
func (m *TaskFormModel) helpPaneHeader() string {
	switch m.activeTab {
	case formTabBasics:
		names := []string{"Title", "Description", "Priority", "Parent", chrome.StatusFieldLabel, "Flags", "Tags", "Context"}
		if m.focusIndex < len(names) {
			return names[m.focusIndex]
		}
	case formTabSchedule:
		return []string{"Due Date", "Next check-in", "Repeat", "Defer until"}[m.focusIndex]
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
	case formBasicsContextField:
		return []helpEntry{
			{key: "Enter", desc: "confirm"},
			{key: "empty", desc: "no context"},
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
	case 1: // Next check-in
		return []helpEntry{{key: "Enter", desc: "edit check-in schedule"}}
	case 2: // Repeat
		return []helpEntry{{key: "Enter", desc: "edit check-in schedule"}}
	case 3:
		return []helpEntry{{key: "Enter", desc: "edit or clear deferral"}}
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
	if m.focusIndex >= m.fieldCountForExtras()-2 {
		return []helpEntry{{key: "Enter", desc: "edit task references"}, {key: "empty", desc: "clear the list"}, {key: "Ctrl+S", desc: "save task"}}
	}
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
