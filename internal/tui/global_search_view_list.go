package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderTaskList renders the filtered task list.
func (m *GlobalSearchModel) renderTaskList(paneHeight int) string {
	if len(m.filtered) == 0 {
		return MutedStyle.Render("No matching tasks.")
	}

	var b strings.Builder
	maxVisible := paneHeight
	scrollOffset := calcScrollOffset(m.cursor, maxVisible)

	for i := scrollOffset; i < len(m.filtered) && i < scrollOffset+maxVisible; i++ {
		line := m.renderTaskListRow(i, m.filtered[i])
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m *GlobalSearchModel) renderTaskListRow(index int, filteredResult filteredSearchResult) string {
	if filteredResult.result.IsNote() {
		return m.renderNoteSearchRow(index, filteredResult)
	}
	return m.renderTaskSearchRow(index, filteredResult)
}

func (m *GlobalSearchModel) renderNoteSearchRow(index int, filteredResult filteredSearchResult) string {
	note := filteredResult.result.Note
	cursor := m.searchRowCursor(index)
	projectSlug := truncateToVisualWidth(filteredResult.result.Project.Slug, 8)
	projectName := MutedStyle.Render(fmt.Sprintf("[%s]", projectSlug))
	noteIcon := MutedStyle.Render("[N]")
	prefix := fmt.Sprintf("%s%s %s ", cursor, noteIcon, projectName)
	prefixWidth := lipgloss.Width(prefix)
	maxTitleLen := max(m.listWidth-prefixWidth-2, 1)
	title := truncateToVisualWidth(note.Title, maxTitleLen)
	titleRendered := highlightTruncated(title, filteredResult.titlePositions)
	line := prefix + titleRendered

	lineWidth := lipgloss.Width(line)
	if lineWidth > m.listWidth {
		excess := lineWidth - m.listWidth + 1
		if lipgloss.Width(title) > excess {
			title = truncateToVisualWidth(note.Title, maxTitleLen-excess)
			titleRendered = highlightTruncated(title, filteredResult.titlePositions)
			line = prefix + titleRendered
		}
	}

	if index == m.cursor {
		return TaskJumperSelectedStyle.Width(m.listWidth).Render(line)
	}

	return line
}

func (m *GlobalSearchModel) renderTaskSearchRow(index int, filteredResult filteredSearchResult) string {
	task := filteredResult.result.Task
	cursor := m.searchRowCursor(index)
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	stageBadge := GetStageBadge(task.Stage)
	stageSegment := ""
	if stageBadge != "" {
		stageSegment = stageBadge + " "
	}
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	projectSlug := truncateToVisualWidth(filteredResult.result.Project.Slug, 8)
	projectName := MutedStyle.Render(fmt.Sprintf("[%s]", projectSlug))
	shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", task.ShortID))
	prefix := fmt.Sprintf("%s%s %s%s %s %s ", cursor, typeIcon, stageSegment, statusIcon, shortID, projectName)
	prefixWidth := lipgloss.Width(prefix)
	maxTitleLen := max(m.listWidth-prefixWidth-2, 1)
	title := truncateToVisualWidth(task.Title, maxTitleLen)
	titleRendered := GetTitlePriorityStyle(task.Priority).Render(highlightTruncated(title, filteredResult.titlePositions))

	if filteredResult.matchSource == "comment" {
		titleRendered += MutedStyle.Render(" [c]")
	}

	line := prefix + titleRendered
	lineWidth := lipgloss.Width(line)
	if lineWidth > m.listWidth {
		excess := lineWidth - m.listWidth + 3
		if lipgloss.Width(title) > excess {
			title = truncateToVisualWidth(task.Title, maxTitleLen-excess)
			titleRendered = GetTitlePriorityStyle(task.Priority).Render(highlightTruncated(title, filteredResult.titlePositions))
			line = prefix + titleRendered
		}
	}

	if index == m.cursor {
		return TaskJumperSelectedStyle.Width(m.listWidth).Render(line)
	}

	return line
}

func (m *GlobalSearchModel) searchRowCursor(index int) string {
	if index == m.cursor {
		return "  "
	}
	return "  "
}
