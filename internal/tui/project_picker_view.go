package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/util"
)

func (m *ProjectPickerModel) View() string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Switch Project", ""))
	b.WriteString("\n\n")

	// Search input
	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	// Render the list
	b.WriteString(m.renderList())
	b.WriteString("\n")

	// Footer
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("↑/↓ navigate • Enter select • Esc cancel"), "", m.Width()))

	return b.String()
}

func (m *ProjectPickerModel) renderList() string {
	var b strings.Builder

	maxVisible := m.Height() - 10
	if maxVisible < 5 {
		maxVisible = 5
	}

	totalItems := len(m.Filtered)

	if totalItems == 0 {
		return MutedStyle.Render("No matching projects.")
	}

	// Calculate max name length from filtered items for aligned columns
	maxNameLen := 0
	for _, item := range m.Filtered {
		if n := len(item.Project.Name); n > maxNameLen {
			maxNameLen = n
		}
	}
	// Cap to terminal width with room for cursor, context badge, and task count
	cap := m.Width() - 30
	if cap < 10 {
		cap = 10
	}
	if maxNameLen > cap {
		maxNameLen = cap
	}
	// Add minimal padding after the longest name
	maxNameLen += 2

	scrollOffset := calcScrollOffset(m.Cursor, maxVisible)

	for i, item := range m.Filtered {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}

		cursor := "  "
		if m.Cursor == i {
			cursor = "> "
		}

		contextBadge := MutedStyle.Render(fmt.Sprintf("[%s]", item.Project.Context))

		taskCount := fmt.Sprintf("%d tasks", item.Project.CountTasks())

		name := util.Truncate(item.Project.Name, maxNameLen-2)

		line := fmt.Sprintf("%s%-*s %s %s", cursor, maxNameLen, name, contextBadge, MutedStyle.Render(taskCount))

		if m.Cursor == i {
			line = BoldStyle.Foreground(ColorPrimary).Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	if totalItems > maxVisible {
		remaining := totalItems - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... %d more", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}
