package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/util"
)

// View renders the global search view.
func (m *GlobalSearchModel) View() string {
	var content strings.Builder

	// Search input
	content.WriteString(m.searchInput.View())
	content.WriteString("\n")

	// Show autocomplete suggestions if active
	if m.acMode != acSearchModeNone && len(m.suggestions) > 0 {
		for i, s := range m.suggestions {
			if i == m.acCursor {
				content.WriteString(SelectedStyle.Render("> " + s.display))
			} else {
				content.WriteString(MutedStyle.Render("  " + s.display))
			}
			content.WriteString("\n")
		}
	}
	content.WriteString("\n")

	// Show active filters as badges
	if m.parsedFilters != nil && m.parsedFilters.HasFilters() {
		content.WriteString(m.renderFilterBadges())
		content.WriteString("\n\n")
	}

	// Handle edge cases
	if len(m.allTasks) == 0 {
		content.WriteString(MutedStyle.Render("No tasks found in any project."))
		content.WriteString("\n\n")
		content.WriteString(m.renderHelpText())

		return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Global Search",
			Subtitle: "0 tasks",
			Centered: false,
		})
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		content.WriteString(MutedStyle.Render("Type to search across all tasks..."))
		content.WriteString("\n\n")
		content.WriteString(MutedStyle.Render("Filters: !priority #tag @due $status ^type /section *active"))
		content.WriteString("\n\n")
		content.WriteString(m.renderHelpText())

		return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Global Search",
			Subtitle: fmt.Sprintf("%d tasks", len(m.allTasks)),
			Centered: false,
		})
	}

	if len(m.filtered) == 0 {
		content.WriteString(MutedStyle.Render(fmt.Sprintf("No tasks match \"%s\".", query)))
		content.WriteString("\n\n")
		content.WriteString(m.renderHelpText())

		return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Global Search",
			Subtitle: fmt.Sprintf("0/%d matching", len(m.allTasks)),
			Centered: false,
		})
	}

	// Calculate pane height for split view
	paneHeight := m.height - 10 // Account for card border, title, input, footer
	if paneHeight < 5 {
		paneHeight = 5
	}

	// Adjust for autocomplete and filter badges
	if m.acMode != acSearchModeNone && len(m.suggestions) > 0 {
		paneHeight -= len(m.suggestions)
	}
	if m.parsedFilters != nil && m.parsedFilters.HasFilters() {
		paneHeight -= 2
	}

	// Split pane: list on left, preview on right
	listContent := m.renderTaskList()
	previewContent := m.renderPreview()

	splitPane := renderOverlaySplitPane(listContent, previewContent, m.listWidth, m.previewWidth, paneHeight)
	content.WriteString(splitPane)
	content.WriteString("\n")

	// Footer
	content.WriteString(m.renderHelpText())

	// Build subtitle showing count
	subtitle := fmt.Sprintf("%d/%d matching", len(m.filtered), len(m.allTasks))

	return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
		Title:    "Global Search",
		Subtitle: subtitle,
		Centered: false,
	})
}

// renderFilterBadges renders active filters as styled badges.
func (m *GlobalSearchModel) renderFilterBadges() string {
	if m.parsedFilters == nil {
		return ""
	}

	var badges []string

	// Priority badges
	if len(m.parsedFilters.Priorities) > 0 {
		var pStrs []string
		for _, p := range m.parsedFilters.Priorities {
			pStrs = append(pStrs, fmt.Sprintf("%d", p))
		}
		badges = append(badges, fmt.Sprintf("!%s", strings.Join(pStrs, ",")))
	}

	// Tag badges
	for _, tag := range m.parsedFilters.Tags {
		badges = append(badges, "#"+tag)
	}

	// Status badges
	for _, status := range m.parsedFilters.Statuses {
		badges = append(badges, "$"+string(status))
	}

	// Type badges
	for _, taskType := range m.parsedFilters.Types {
		badges = append(badges, "^"+string(taskType))
	}

	// Due date badge
	if m.parsedFilters.DueDateFilter != "" {
		badges = append(badges, "@"+m.parsedFilters.DueDateFilter)
	}

	// Section badge
	if m.parsedFilters.Section != "" {
		badges = append(badges, "/"+m.parsedFilters.Section)
	}

	// Active badge
	if m.parsedFilters.ActiveOnly {
		badges = append(badges, "*active")
	}

	if len(badges) == 0 {
		return ""
	}

	// Style each badge
	var styled []string
	for _, badge := range badges {
		styled = append(styled, FilterBadgeStyle.Render(badge))
	}

	return "Filters: " + strings.Join(styled, " ")
}

// renderHelpText renders context-appropriate help text.
func (m *GlobalSearchModel) renderHelpText() string {
	if m.acMode != acSearchModeNone && len(m.suggestions) > 0 {
		return HelpStyle.Render("Tab: accept • ↑↓: navigate • Esc: dismiss")
	}
	return HelpStyle.Render(fmt.Sprintf("↑↓ nav • Enter • Esc • %d results", len(m.filtered)))
}

// renderTaskList renders the filtered task list.
func (m *GlobalSearchModel) renderTaskList() string {
	if len(m.filtered) == 0 {
		return MutedStyle.Render("No matching tasks.")
	}

	var b strings.Builder
	maxVisible := m.height - 7 // Account for chrome

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}

	for i := scrollOffset; i < len(m.filtered) && i < scrollOffset+maxVisible; i++ {
		fr := m.filtered[i]
		result := fr.result

		// Cursor indicator
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		// Handle note results
		if result.IsNote() {
			note := result.Note
			projectSlug := util.Truncate(result.Project.Slug, 8)
			projectName := MutedStyle.Render(fmt.Sprintf("[%s]", projectSlug))
			noteIcon := MutedStyle.Render("[N]")
			prefix := fmt.Sprintf("%s%s %s ", cursor, noteIcon, projectName)
			prefixWidth := lipgloss.Width(prefix)
			maxTitleLen := m.listWidth - prefixWidth - 2
			if maxTitleLen < 10 {
				maxTitleLen = 10
			}
			title := util.Truncate(note.Title, maxTitleLen)
			var titleRendered string
			if len(fr.titlePositions) > 0 {
				titleRunes := []rune(title)
				var validPositions []int
				for _, p := range fr.titlePositions {
					if p < len(titleRunes) {
						validPositions = append(validPositions, p)
					}
				}
				titleRendered = HighlightText(title, validPositions, NormalStyle)
			} else {
				titleRendered = title
			}
			line := prefix + titleRendered
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}

		task := result.Task

		// Type icon with badge style
		typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

		// Status icon with badge style
		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

		// Project name (truncated to 8 chars to save space)
		projectSlug := util.Truncate(result.Project.Slug, 8)
		projectName := MutedStyle.Render(fmt.Sprintf("[%s]", projectSlug))

		// Short ID
		shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", task.ShortID))

		// Build prefix to measure its actual visual width
		prefix := fmt.Sprintf("%s%s %s %s %s ", cursor, typeIcon, statusIcon, shortID, projectName)
		prefixWidth := lipgloss.Width(prefix)

		// Title (truncated) - use measured prefix width for accuracy
		// Subtract extra buffer to ensure no wrapping
		maxTitleLen := m.listWidth - prefixWidth - 2
		if maxTitleLen < 10 {
			maxTitleLen = 10
		}
		title := util.Truncate(task.Title, maxTitleLen)

		// Adjust positions for truncated title and apply highlighting
		var titleRendered string
		if len(fr.titlePositions) > 0 {
			// Filter positions to only include those within the truncated title
			titleRunes := []rune(title)
			var validPositions []int
			for _, p := range fr.titlePositions {
				if p < len(titleRunes) {
					validPositions = append(validPositions, p)
				}
			}
			titleRendered = HighlightText(title, validPositions, NormalStyle)
		} else {
			titleRendered = title
		}

		// Build line and verify total width doesn't exceed listWidth
		line := prefix + titleRendered
		lineWidth := lipgloss.Width(line)
		if lineWidth > m.listWidth {
			// Truncate title further if still too wide (recalculate without highlighting)
			excess := lineWidth - m.listWidth + 3
			if len(title) > excess {
				title = util.Truncate(task.Title, maxTitleLen-excess)
				// Re-apply highlighting with new truncated title
				if len(fr.titlePositions) > 0 {
					titleRunes := []rune(title)
					var validPositions []int
					for _, p := range fr.titlePositions {
						if p < len(titleRunes) {
							validPositions = append(validPositions, p)
						}
					}
					titleRendered = HighlightText(title, validPositions, NormalStyle)
				} else {
					titleRendered = title
				}
				line = prefix + titleRendered
			}
		}

		if i == m.cursor {
			line = TaskJumperSelectedStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderPreview renders the preview pane for the selected task.
func (m *GlobalSearchModel) renderPreview() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return MutedStyle.Render("No task selected.")
	}

	result := m.filtered[m.cursor].result
	project := result.Project

	// Handle note preview
	if result.IsNote() {
		var b strings.Builder
		b.WriteString(MutedStyle.Render("Project: "))
		b.WriteString(BoldStyle.Render(project.Name))
		b.WriteString("\n\n")
		b.WriteString(TaskJumperPreviewHeaderStyle.Render("Note: " + result.Note.Title))
		b.WriteString("\n\n")
		if len(result.Note.Tags) > 0 {
			b.WriteString(fmt.Sprintf("%s %s\n", MutedStyle.Render("Tags:"), strings.Join(result.Note.Tags, ", ")))
		}
		if len(result.Note.LinkedTasks) > 0 {
			b.WriteString(fmt.Sprintf("%s %d\n", MutedStyle.Render("Linked tasks:"), len(result.Note.LinkedTasks)))
		}
		b.WriteString("\n")
		body := result.Note.Body
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		b.WriteString(body)
		return b.String()
	}

	task := result.Task
	var b strings.Builder

	// Project header
	b.WriteString(MutedStyle.Render("Project: "))
	b.WriteString(BoldStyle.Render(project.Name))
	b.WriteString("\n\n")

	// Header with type and title (truncate to fit preview width)
	typeStyle := GetTypeStyle(string(task.Type))
	titlePrefix := fmt.Sprintf("%s #%d: ", task.Type.HeadingPrefix(), task.ShortID)
	maxTitleLen := m.previewWidth - len(titlePrefix) - 4 // -4 for padding/margin
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		util.Truncate(task.Title, maxTitleLen))
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(header))
	b.WriteString("\n\n")

	// Status
	statusStyle := GetStatusStyle(string(task.Status))
	b.WriteString(fmt.Sprintf("%s %s %s\n",
		MutedStyle.Render("Status:"),
		statusStyle.Render(task.Status.Icon()),
		statusStyle.Render(string(task.Status))))

	// Priority
	priorityStyle := NormalStyle
	if task.Priority <= 2 {
		priorityStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	}
	b.WriteString(fmt.Sprintf("%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", task.Priority))))

	// Type
	b.WriteString(fmt.Sprintf("%s %s\n",
		MutedStyle.Render("Type:"),
		typeStyle.Render(task.Type.HeadingPrefix())))

	// Tags
	if len(task.Tags) > 0 {
		tagStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
		tags := make([]string, len(task.Tags))
		for i, tag := range task.Tags {
			tags[i] = tagStyle.Render(tag)
		}
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, ", ")))
	}

	// Due date
	if task.DueDate != nil {
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Due:"),
			task.DueDate.Format(util.DateFormat)))
	}

	// Parent reference
	if task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(util.Truncate(parentRef, m.previewWidth-10))))
	}

	// Description (truncated)
	if task.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")

		// Truncate description to fit preview
		maxLines := m.height - 20
		if maxLines < 3 {
			maxLines = 3
		}
		lines := strings.Split(task.Description, "\n")
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, "...")
		}
		// Truncate each line to fit preview width
		for i, line := range lines {
			lines[i] = util.Truncate(line, m.previewWidth-2)
		}
		b.WriteString(strings.Join(lines, "\n"))
	}

	// Children summary
	if len(task.Children) > 0 {
		b.WriteString("\n\n")
		completed := 0
		for _, child := range task.Children {
			if child.IsComplete() {
				completed++
			}
		}
		b.WriteString(fmt.Sprintf("%s %d/%d completed\n",
			MutedStyle.Render("Subtasks:"),
			completed, len(task.Children)))
	}

	return b.String()
}
