package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
)

func (m *ProjectPickerModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Switch Project", ""))
	b.WriteString("\n\n")

	// Search input
	b.WriteString(m.SearchInputView())
	b.WriteString("\n\n")

	totalWidth := m.Width()
	maxVisible := max(m.Height()-10, 5)

	var content string
	if totalWidth >= 80 {
		leftWidth := totalWidth * 55 / 100
		rightWidth := totalWidth - leftWidth - 1 // 1 for divider

		listContent := m.renderList(leftWidth)
		detailContent := m.renderProjectDetail(rightWidth, maxVisible)

		leftStyled := lipgloss.NewStyle().Width(leftWidth).Height(maxVisible).Render(listContent)
		divider := renderVerticalDivider(maxVisible, false)
		rightStyled := lipgloss.NewStyle().Width(rightWidth).Height(maxVisible).Render(detailContent)

		content = lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, divider, rightStyled)
	} else {
		content = m.renderList(totalWidth)
	}

	b.WriteString(content)
	b.WriteString("\n")

	// Footer
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("↑/↓/Ctrl+J/K/N/P navigate · Ctrl+U/D page · Enter select · Esc cancel"), "", totalWidth))

	return tea.NewView(b.String())
}

func (m *ProjectPickerModel) renderList(width int) string {
	var b strings.Builder

	totalItems := len(m.Filtered)

	if totalItems == 0 {
		return MutedStyle.Render("No matching projects.")
	}

	maxVisible := max(m.Height()-10, 5)

	// Calculate max name length from filtered items for aligned columns
	maxNameLen := 0
	for _, item := range m.Filtered {
		if nameWidth := lipgloss.Width(item.Project.Name); nameWidth > maxNameLen {
			maxNameLen = nameWidth
		}
	}
	// Cap to width with room for cursor, context badge, and task count
	cap := max(width-30, 10)
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
			cursor = "→ "
		}

		contextBadge := MutedStyle.Render(fmt.Sprintf("[%s]", item.Project.Context))

		taskCount := fmt.Sprintf("%d tasks", item.Project.CountTasks())

		name := truncateToVisualWidth(item.Project.Name, maxNameLen-2)
		line := fmt.Sprintf("%s%s %s %s", cursor, truncateAndPad(name, maxNameLen), contextBadge, MutedStyle.Render(taskCount))

		if m.Cursor == i {
			line = SelectedItemTextStyle.Render(line)
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

func (m *ProjectPickerModel) renderProjectDetail(width, height int) string {
	if len(m.Filtered) == 0 || m.Cursor >= len(m.Filtered) {
		return MutedStyle.Render("No project selected")
	}

	p := m.Filtered[m.Cursor].Project
	var b strings.Builder
	lines := 0

	// Project name
	b.WriteString(BoldStyle.Render(p.Name))
	b.WriteString("\n")
	lines++

	// Context
	b.WriteString(MutedStyle.Render("Context: ") + p.Context)
	b.WriteString("\n")
	lines++

	// Created date
	b.WriteString(MutedStyle.Render("Created: ") + p.CreatedAt.Format("2006-01-02"))
	b.WriteString("\n")
	lines++

	// Pinned status
	if p.Pinned {
		b.WriteString(MutedStyle.Render("Pinned: ") + "yes")
		b.WriteString("\n")
		lines++
	}

	// Description (word-wrapped, max 5 lines)
	if p.Description != "" {
		b.WriteString("\n")
		lines++
		descLines := wrapText(p.Description, width)
		maxDescLines := 5
		if len(descLines) > maxDescLines {
			descLines = descLines[:maxDescLines]
		}
		for _, dl := range descLines {
			b.WriteString(dl)
			b.WriteString("\n")
			lines++
		}
	}

	// Separator
	b.WriteString("\n")
	lines++
	b.WriteString(MutedStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")
	lines++

	// Task stats
	active := p.CountActiveTasks()
	completed := p.CountCompletedTasks()
	overdue := p.CountOverdueTasks()
	blocked := p.CountBlockedTasks()
	dueSoon := p.CountDueSoonTasks(7)

	b.WriteString(MutedStyle.Render("Active: ") + fmt.Sprintf("%d", active))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("Done: ") + fmt.Sprintf("%d", completed))
	b.WriteString("\n")
	lines++

	if overdue > 0 {
		overdueStr := WarningIconStyle.Render(fmt.Sprintf("%d", overdue))
		b.WriteString(MutedStyle.Render("Overdue: ") + overdueStr)
	} else {
		b.WriteString(MutedStyle.Render("Overdue: ") + "0")
	}
	b.WriteString("  ")
	if blocked > 0 {
		blockedStr := WarningIconStyle.Render(fmt.Sprintf("%d", blocked))
		b.WriteString(MutedStyle.Render("Blocked: ") + blockedStr)
	} else {
		b.WriteString(MutedStyle.Render("Blocked: ") + "0")
	}
	b.WriteString("\n")
	lines++

	if dueSoon > 0 {
		b.WriteString(MutedStyle.Render("Due soon: ") + fmt.Sprintf("%d", dueSoon))
		b.WriteString("\n")
		lines++
	}

	// Section breakdown
	sections := p.GetSections()
	sectionCounts := p.CountTasksBySection()
	if len(sections) > 0 && lines < height-2 {
		b.WriteString("\n")
		lines++
		b.WriteString(MutedStyle.Render(strings.Repeat("─", width)))
		b.WriteString("\n")
		lines++
		b.WriteString(BoldStyle.Render("Sections"))
		b.WriteString("\n")
		lines++
		for _, sec := range sections {
			if lines >= height {
				break
			}
			count := sectionCounts[sec]
			fmt.Fprintf(&b, "  %s %s", MutedStyle.Render(sec+":"), fmt.Sprintf("%d", count))
			b.WriteString("\n")
			lines++
		}
	}

	return b.String()
}
