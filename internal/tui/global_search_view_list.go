package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// renderTaskList renders the filtered task list.
func (m *GlobalSearchModel) renderTaskList(paneHeight int) string {
	if len(m.filtered) == 0 {
		return MutedStyle.Render("No matching tasks.")
	}

	var b strings.Builder
	maxVisible := paneHeight
	scrollOffset := calcScrollOffset(m.cursor, maxVisible)

	// Compute badge widths once per render pass for column alignment.
	var colLayout *ColumnLayout
	if m.listWidth >= MinColumnAlignWidth {
		tasks := make([]*models.Task, 0, len(m.filtered))
		for _, fr := range m.filtered {
			if fr.result.Task != nil {
				tasks = append(tasks, fr.result.Task)
			}
		}
		colLayout = ComputeBadgeWidths(tasks, BadgeWidthOpts{
			IncludeStatus: true,
			IncludeType:   true,
			IncludeStage:  true,
		})
	}

	for i := scrollOffset; i < len(m.filtered) && i < scrollOffset+maxVisible; i++ {
		line := m.renderTaskListRow(i, m.filtered[i], colLayout)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m *GlobalSearchModel) renderTaskListRow(index int, filteredResult filteredSearchResult, colLayout *ColumnLayout) string {
	if filteredResult.result.IsNote() {
		return m.renderNoteSearchRow(index, filteredResult)
	}
	return m.renderTaskSearchRow(index, filteredResult, colLayout)
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

func (m *GlobalSearchModel) renderTaskSearchRow(index int, filteredResult filteredSearchResult, colLayout *ColumnLayout) string {
	task := filteredResult.result.Task
	cursor := m.searchRowCursor(index)
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	if colLayout != nil && colLayout.HasType {
		typeIcon = PadBadge(typeIcon, colLayout.TypeWidth)
	}
	stageBadge := GetStageBadge(task.Stage)
	stageSegment := ""
	if colLayout != nil && colLayout.HasStage {
		if stageBadge == "" {
			stageSegment = EmptyBadge(colLayout.StageWidth) + " "
		} else {
			stageSegment = PadBadge(stageBadge, colLayout.StageWidth) + " "
		}
	} else if stageBadge != "" {
		stageSegment = stageBadge + " "
	}
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	if colLayout != nil && colLayout.HasStatus {
		statusIcon = PadBadge(statusIcon, colLayout.StatusWidth)
	}
	projectSlug := truncateToVisualWidth(filteredResult.result.Project.Slug, 8)
	projectName := MutedStyle.Render(fmt.Sprintf("[%s]", projectSlug))
	shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", task.ShortID))
	prefix := fmt.Sprintf("%s%s %s%s %s %s ", cursor, typeIcon, stageSegment, statusIcon, shortID, projectName)
	descSuffix := ""
	if m.config.IsDescriptionColumnVisible() && len(task.Description) > 0 {
		descSuffix = " " + MutedStyle.Render(IconDescription())
	}
	prefixWidth := lipgloss.Width(prefix) + lipgloss.Width(descSuffix)
	maxTitleLen := max(m.listWidth-prefixWidth-2, 1)
	title := truncateToVisualWidth(task.Title, maxTitleLen)
	titleRendered := GetTitlePriorityStyle(task.Priority).Render(highlightTruncated(title, filteredResult.titlePositions))

	if filteredResult.matchSource == "comment" {
		titleRendered += MutedStyle.Render(" [c]")
	}

	line := prefix + titleRendered + descSuffix
	lineWidth := lipgloss.Width(line)
	if lineWidth > m.listWidth {
		excess := lineWidth - m.listWidth + 3
		if lipgloss.Width(title) > excess {
			title = truncateToVisualWidth(task.Title, maxTitleLen-excess)
			titleRendered = GetTitlePriorityStyle(task.Priority).Render(highlightTruncated(title, filteredResult.titlePositions))
			line = prefix + titleRendered + descSuffix
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
