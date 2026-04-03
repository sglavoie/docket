package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// renderActionList renders the list of available actions for the selected item.
func (m *CommandPaletteModel) renderActionList() string {
	if len(m.availableActions) == 0 {
		return MutedStyle.Render("No actions available.")
	}

	var b strings.Builder
	contentWidth := max(m.listWidth-4, 1)
	paneHeight := max(m.height-8, 5)
	maxVisible := max(paneHeight-2, 3)

	if m.actionCursor < m.actionScrollOffset {
		m.actionScrollOffset = m.actionCursor
	} else if m.actionCursor >= m.actionScrollOffset+maxVisible {
		m.actionScrollOffset = m.actionCursor - maxVisible + 1
	}

	endIdx := min(m.actionScrollOffset+maxVisible, len(m.availableActions))
	for i := m.actionScrollOffset; i < endIdx; i++ {
		line := m.renderActionItem(m.availableActions[i], i == m.actionCursor, contentWidth)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderActionItem renders a single action row.
// Layout: [cursor] [icon] [name] ... [key hint]
func (m *CommandPaletteModel) renderActionItem(action PaletteAction, selected bool, width int) string {
	cursor := "  "
	if selected {
		cursor = "  "
	}

	icon := action.Icon
	if icon == "" {
		icon = "•"
	}
	icon += " "
	iconWidth := lipgloss.Width(icon)

	keyHint := ""
	keyHintLen := 0
	if action.KeyHint != "" {
		keyHint = action.KeyHint
		keyHintLen = lipgloss.Width(keyHint) + 2
	}

	fixedWidth := 2 + iconWidth + keyHintLen + 2
	nameWidth := max(width-fixedWidth, 10)
	name := truncateToVisualWidth(action.Name, nameWidth)
	gap := max(nameWidth-lipgloss.Width(name), 1)

	var line strings.Builder
	line.WriteString(cursor)
	line.WriteString(icon)
	line.WriteString(BoldStyle.Render(name))

	if keyHint != "" {
		line.WriteString(strings.Repeat(" ", gap))
		line.WriteString(HelpKeyStyle.Render(keyHint))
	}

	result := line.String()
	if selected {
		result = TaskJumperSelectedStyle.Render(result)
	}

	return result
}
