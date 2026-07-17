package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// renderNodeWithWidth is like renderNode but accepts a custom width for truncation.
func (m *ProjectTreeModel) renderNodeWithWidth(node *TreeNode, selected bool, inSelection bool, maxWidth int) string { //nolint:gocyclo
	task := node.Task

	// Check if this task is being quick-edited
	if m.quickEdit.IsActive() && m.quickEdit.GetTask() != nil && task.ID == m.quickEdit.GetTask().ID {
		return m.renderQuickEditNode(node, maxWidth)
	}

	// Build indent using depth-only padding; parent rows still show fold chevrons.
	indent := MutedStyle.Render(node.TreePrefix)
	expandIcon := "  "
	if node.HasFilteredChildren {
		if node.Expanded {
			expandIcon = IconExpanded + " "
		} else {
			expandIcon = IconCollapsed + " "
		}
	}

	// Queue / daily-plan indicator: always 2 chars wide. w takes precedence over q.
	queueIndicator := "  "
	if m.dailyPlanIDs[task.ID] {
		queueIndicator = ActiveStyle.Render("w") + " "
	} else if m.queueIDs[task.ID] {
		queueIndicator = ActiveStyle.Render("q") + " "
	}

	// Selection checkbox (only in multi-select mode)
	checkbox := ""
	if m.multiSelectMode {
		if inSelection {
			checkbox = SelectionCheckStyle.Render("[x]") + " "
		} else {
			checkbox = MutedStyle.Render("[ ]") + " "
		}
	}

	// Status icon with color
	statusStyle := GetStateStyle(task.State())

	// Override status style if task has incomplete dependencies
	if m.project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	// Context nodes (parents shown for hierarchy context) are dimmed
	var typeIcon string
	if node.IsContextNode {
		statusStyle = MutedStyle
		expandIcon = MutedStyle.Render(strings.TrimSuffix(expandIcon, " ")) + " "
		typeIcon = MutedStyle.Render(task.Type.Icon())
	} else {
		typeIcon = GetTypeBadge(string(task.Type), task.Type.Icon())
	}

	var statusIcon string
	if node.IsContextNode {
		statusIcon = statusStyle.Render(task.State().Icon())
	} else if m.project.HasIncompleteDependencies(task) {
		statusIcon = BlockedByDepsStyle.Render(task.State().Icon())
	} else {
		statusIcon = GetStateBadge(task.State())
	}

	if m.colLayout != nil && m.colLayout.HasStatus {
		statusIcon = PadBadge(statusIcon, m.colLayout.StatusWidth)
	}
	if m.colLayout != nil && m.colLayout.HasType {
		typeIcon = PadBadge(typeIcon, m.colLayout.TypeWidth)
	}

	// Priority segment (inline, before short ID)
	var prioritySegment string
	if m.colLayout != nil && m.colLayout.HasPriority {
		var priorityBadge string
		if m.columnConfig.ShouldShowPriority(task.Priority) {
			priorityBadge = renderPriorityBadge(task.Priority)
		}
		prioritySegment = PadBadge(priorityBadge, m.colLayout.PriorityWidth) + " "
	} else if m.columnConfig.ShouldShowPriority(task.Priority) {
		prioritySegment = renderPriorityBadge(task.Priority) + " "
	}

	// Task ID (includes trailing space when visible for spacing in the format string)
	idStr := ""
	if m.columnConfig.IsVisible(ColShortID) {
		rendered := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))
		if m.colLayout != nil && m.colLayout.HasShortID {
			idStr = PadBadge(rendered, m.colLayout.ShortIDWidth) + " "
		} else {
			idStr = rendered + " "
		}
	}

	titleStyle := TaskTitleStyleFor(task, TaskTitleStyleOptions{
		CursorSelected: selected,
		Muted:          node.IsContextNode,
		BulkSelected:   inSelection,
	})

	// Dense-row hierarchy: keep only the urgency hero text (overdue/due-today)
	// inline. Every other secondary indicator — section badge, future due,
	// snooze, recurrence, progress, dependencies, score, tags, comments,
	// description, attachments, checklist, decisions — moves to the selected
	// metadata strip below the row.
	indicators := ""
	if m.columnConfig.IsVisible(ColDueDate) && dueIsInlineHero(task) {
		if dueStr := formatDueDate(task, m.runtime.Settings().GetTimeFormat()); dueStr != "" {
			indicators = " " + dueStr
		}
	}

	// Calculate available width for title, accounting for all other elements.
	var baseUsed int
	if m.colLayout != nil {
		// Fixed-width path: compute exact overhead from column layout.
		// format: cursor(3) + checkbox + indent(actual prefix) + expandIcon(2) + typeIcon(1) +
		//         space(1) + statusIcon(1) + space(1) + prioritySegment + idStr
		priorityW := 0
		if m.colLayout.HasPriority {
			priorityW = m.colLayout.PriorityWidth + 1
		}
		idW := 0
		if m.colLayout.HasShortID {
			idW = m.colLayout.ShortIDWidth + 1
		}
		baseUsed = 3 + 2 + lipgloss.Width(checkbox) + lipgloss.Width(node.TreePrefix) + 2 + 1 + 1 + 1 + 1 + priorityW + idW
	} else {
		// Approximate path (pre-existing logic).
		baseOverhead := 16
		baseUsed = 3 + 2 + len(checkbox) + lipgloss.Width(node.TreePrefix) + len(expandIcon) + baseOverhead
	}
	availableTitleWidth := maxWidth - baseUsed - lipgloss.Width(indicators)
	availableTitleWidth = max(availableTitleWidth, 20)
	truncatedTitle := truncateToVisualWidth(task.Title, availableTitleWidth)

	// Apply search highlighting if there's an active search query
	var title string
	if positions := m.getSearchMatchPositions(task); len(positions) > 0 {
		// Filter positions to only include those within the truncated title
		titleRunes := []rune(truncatedTitle)
		var validPositions []int
		for _, p := range positions {
			if p < len(titleRunes) {
				validPositions = append(validPositions, p)
			}
		}
		title = HighlightText(truncatedTitle, validPositions, titleStyle)
	} else {
		title = titleStyle.Render(truncatedTitle)
	}

	content := fmt.Sprintf("%s%s%s%s%s %s %s%s%s%s",
		queueIndicator, checkbox, indent, expandIcon, typeIcon, statusIcon, prioritySegment, idStr, title, indicators)
	isDimmed := task.Dimmed && !node.IsContextNode
	isOverdue := task.IsOverdue()
	isBlocked := task.IsInState(models.StateWaiting) || m.project.HasIncompleteDependencies(task)
	row := RenderSelectableRow(selected, task.Pinned, m.cursorFlash, isOverdue, isBlocked, content, maxWidth, isDimmed)
	if !zebraEnabled && !selected && !isDimmed && !isOverdue && !isBlocked && node.TopGroupIndex%2 == 1 {
		row = TreeGroupOddBgStyle.Width(maxWidth).Render(row)
	}
	return row
}

func (m *ProjectTreeModel) nodeContentGroup(task *models.Task) string {
	var parts []string
	if m.columnConfig.IsVisible(ColComments) {
		if s := renderCommentIndicator(task); s != "" {
			parts = append(parts, s)
		}
	}
	if m.columnConfig.IsVisible(ColDescription) {
		if s := renderDescriptionIndicator(task); s != "" {
			parts = append(parts, s)
		}
	}
	if m.columnConfig.IsVisible(ColAttachments) {
		if s := renderGitHubIssueIndicator(task); s != "" {
			parts = append(parts, s)
		}
		if s := renderAttachmentIndicator(task); s != "" {
			parts = append(parts, s)
		}
	}
	if m.columnConfig.IsVisible(ColChecklist) {
		if s := renderChecklistIndicator(task); s != "" {
			parts = append(parts, s)
		}
	}
	if m.columnConfig.IsVisible(ColDecisions) {
		if s := renderDecisionIndicator(task); s != "" {
			parts = append(parts, s)
		}
	}
	if m.columnConfig.IsVisible(ColScore) && m.runtime != nil && m.project != nil {
		if profile := m.runtime.Automation().GetProjectScoringProfile(m.project.ID); profile != nil {
			if score := RenderScoreBadge(task, profile); score != "" {
				parts = append(parts, score)
			}
		}
	}
	if m.columnConfig.IsVisible(ColTags) && len(task.Tags) > 0 && !m.sidebar.IsEnabled() {
		parts = append(parts, renderTruncatedTags(task.Tags, 2, 8))
	}
	return JoinGroupParts(parts...)
}

// renderQuickEditNode renders a task node in quick edit mode.
func (m *ProjectTreeModel) renderQuickEditNode(node *TreeNode, maxWidth int) string {
	task := node.Task

	// Build indent using depth-only padding.
	indent := MutedStyle.Render(node.TreePrefix)
	indentWidth := lipgloss.Width(node.TreePrefix)

	// Type and status icons
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := GetStateBadge(task.State())

	priorityBadge := renderPriorityBadge(task.Priority)
	prioritySegment := ""
	if priorityBadge != "" {
		prioritySegment = priorityBadge + " "
	}

	// Task ID
	idStr := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))

	// Calculate width for the text input
	priorityW := lipgloss.Width(prioritySegment)
	prefixWidth := indentWidth + 2 + 1 + 1 + priorityW + 5 + 1 // indent + typeIcon + space + statusIcon + space + priority + idStr + space
	inputWidth := max(maxWidth-prefixWidth-4, 20)
	input := m.quickEdit.GetInput()
	input.SetWidth(inputWidth)
	m.quickEdit.UpdateInput(input)

	// Render the input
	inputView := m.quickEdit.GetInput().View()

	// Build prefix
	prefix := fmt.Sprintf("%s%s %s %s%s ", indent, typeIcon, statusIcon, prioritySegment, idStr)

	// Build the edit line
	editLine := prefix + inputView

	// Add preview line if in full edit mode (not rename-only)
	if !m.quickEdit.IsRenameOnly() && m.quickEdit.GetPreview() != nil && m.quickEdit.GetOriginal() != nil {
		previewText := renderQuickEditPreview(m.quickEdit.GetPreview(), m.quickEdit.GetOriginal())
		if previewText != "" {
			previewIndent := strings.Repeat(" ", indentWidth+2)
			editLine += "\n" + previewIndent + MutedStyle.Render(previewText)
		}
	}

	// Add help hint
	helpIndent := strings.Repeat(" ", indentWidth+2)
	if m.quickEdit.IsRenameOnly() {
		editLine += "\n" + helpIndent + HelpStyle.Render("Enter: save · Esc: cancel")
	} else {
		editLine += "\n" + helpIndent + HelpStyle.Render("!N priority · #tag · -#tag · @date · /section · $status · Enter: save · Esc: cancel")
	}

	return editLine
}
