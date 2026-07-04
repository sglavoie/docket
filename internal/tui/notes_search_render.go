package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View renders the notes search view.
func (m *NotesSearchModel) View() tea.View {
	var b strings.Builder
	query := strings.TrimSpace(m.searchInput.Value())

	var subtitle string
	switch {
	case query == "":
		subtitle = fmt.Sprintf("%d notes", len(m.notes))
	case len(m.filtered) == 0:
		subtitle = fmt.Sprintf("0/%d matching", len(m.notes))
	default:
		subtitle = fmt.Sprintf("%d/%d matching", len(m.filtered), len(m.notes))
	}

	title := fmt.Sprintf("Notes Search - %s", m.project.Name)
	availableSubtitleWidth := max(m.width-lipgloss.Width(title)-2, 1)
	b.WriteString(renderFullscreenTitle(title, truncateToVisualWidth(subtitle, availableSubtitleWidth)))
	b.WriteString("\n\n")
	b.WriteString(m.searchInput.View())
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		if query == "" {
			b.WriteString(MutedStyle.Render("No notes found."))
		} else {
			noMatch := truncateToVisualWidth(fmt.Sprintf("No notes match \"%s\".", query), m.width)
			b.WriteString(MutedStyle.Render(noMatch))
		}
		b.WriteString("\n")
		b.WriteString(renderStatusBarFooter(HelpStyle.Render("Esc close"), "", m.width))
		return tea.NewView(b.String())
	}

	paneHeight := max(m.height-8, 5)
	listContent := m.renderNoteList(paneHeight)
	previewContent := m.renderPreview()
	b.WriteString(renderOverlaySplitPane(listContent, previewContent, m.listWidth, m.previewWidth, paneHeight))
	b.WriteString("\n")
	b.WriteString(renderStatusBarFooter(
		HelpStyle.Render(fmt.Sprintf("up/down nav · Enter select · Esc close · %d results", len(m.filtered))),
		"",
		m.width,
	))

	return tea.NewView(b.String())
}

// renderNoteList renders the filtered note list.
func (m *NotesSearchModel) renderNoteList(paneHeight int) string {
	if len(m.filtered) == 0 {
		return MutedStyle.Render("No matching notes.")
	}

	var b strings.Builder
	startIdx, endIdx := persistentIndexWindow(m.cursor, len(m.filtered), paneHeight, &m.scrollOffset)
	for index := startIdx; index < endIdx; index++ {
		line := m.renderNoteListRow(index)
		if index == m.cursor {
			line = TaskJumperSelectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m *NotesSearchModel) renderNoteListRow(index int) string {
	result := m.filtered[index]
	note := result.note
	prefix := fmt.Sprintf("  %s ", MutedStyle.Render("[N]"))
	prefixWidth := lipgloss.Width(prefix)
	maxTitleLen := max(m.listWidth-prefixWidth-2, 1)
	title := truncateToVisualWidth(note.Title, maxTitleLen)
	titleRendered := highlightTruncated(title, result.titlePositions)
	line := prefix + titleRendered

	if lipgloss.Width(line) <= m.listWidth {
		return line
	}

	excess := lipgloss.Width(line) - m.listWidth
	if lipgloss.Width(title) <= excess {
		return line
	}

	title = truncateToVisualWidth(note.Title, max(maxTitleLen-excess, 1))
	return prefix + highlightTruncated(title, result.titlePositions)
}

// renderPreview renders the preview pane for the selected note.
func (m *NotesSearchModel) renderPreview() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return MutedStyle.Render("No note selected.")
	}

	result := m.filtered[m.cursor]
	note := result.note

	var b strings.Builder
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(truncateToVisualWidth(note.Title, max(m.previewWidth-2, 1))))
	b.WriteString("\n\n")

	if len(note.Tags) > 0 {
		fmt.Fprintf(&b, "%s %s\n", MutedStyle.Render("Tags:"), strings.Join(note.Tags, ", "))
	}
	if len(note.LinkedTasks) > 0 {
		b.WriteString(MutedStyle.Render("Linked tasks:"))
		b.WriteString("\n")
		for _, taskID := range note.LinkedTasks {
			taskTitle := "[deleted]"
			taskLine := MutedStyle.Render(fmt.Sprintf("  → %s", taskTitle))
			if task := m.project.GetTaskByID(taskID); task != nil {
				taskLine = MutedStyle.Render("  → ") + TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(task.Title)
			}
			b.WriteString(taskLine)
			b.WriteString("\n")
		}
	}
	if note.Pinned {
		fmt.Fprintf(&b, "%s Yes\n", MutedStyle.Render("Pinned:"))
	}
	fmt.Fprintf(&b, "%s %s\n", MutedStyle.Render("Modified:"), formatNoteAge(note.ModifiedAt))

	if result.bodySnippet != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Match"))
		b.WriteString("\n")
		b.WriteString(m.renderPreviewSnippet(result))
		return b.String()
	}

	if note.Body != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Content"))
		b.WriteString("\n")
		rendered := renderMarkdown(note.Body, m.previewWidth-2)
		lines := strings.Split(rendered, "\n")
		maxLines := max(m.height-15, 3)
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, "...")
		}
		b.WriteString(strings.Join(lines, "\n"))
	}

	return b.String()
}

func (m *NotesSearchModel) renderPreviewSnippet(result notesSearchResult) string {
	lines := strings.Split(result.bodySnippet, "\n")
	for index, line := range lines {
		lines[index] = truncateToVisualWidth(line, m.previewWidth-2)
	}

	snippetText := strings.Join(lines, "\n")
	if len(result.bodyPositions) == 0 {
		return snippetText
	}

	snippetRunes := len([]rune(snippetText))
	validPositions := make([]int, 0, len(result.bodyPositions))
	for _, position := range result.bodyPositions {
		if position < snippetRunes {
			validPositions = append(validPositions, position)
		}
	}
	if len(validPositions) == 0 {
		return snippetText
	}

	return HighlightText(snippetText, validPositions, NormalStyle)
}
