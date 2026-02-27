package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderNodeWithWidth is like renderNode but accepts a custom width for truncation.
func (m *ProjectTreeModel) renderNodeWithWidth(node *TreeNode, selected bool, inSelection bool, maxWidth int) string {
	task := node.Task

	// Check if this task is being quick-edited
	if m.quickEdit.IsActive() && m.quickEdit.GetTask() != nil && task.ID == m.quickEdit.GetTask().ID {
		return m.renderQuickEditNode(node, maxWidth)
	}

	// Build indent
	indent := strings.Repeat(TreeIndent, node.Level)

	// Expand/collapse indicator (only show if node has filtered children)
	expandIcon := "  "
	if node.HasFilteredChildren {
		if node.Expanded {
			expandIcon = "▼ "
		} else {
			expandIcon = "▶ "
		}
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
	statusStyle := GetStatusStyle(string(task.Status))

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
		statusIcon = statusStyle.Render(task.Status.Icon())
	} else if m.project.HasIncompleteDependencies(task) {
		statusIcon = BlockedByDepsStyle.Render(task.Status.Icon())
	} else {
		statusIcon = GetStatusBadge(string(task.Status), task.Status.Icon())
	}

	// Task ID (includes trailing space when visible for spacing in the format string)
	idStr := ""
	if m.columnConfig.IsVisible(ColShortID) {
		idStr = MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)) + " "
	}

	// Title
	titleStyle := NormalStyle
	if node.IsContextNode {
		// Context nodes are always dimmed
		titleStyle = MutedStyle
	} else if selected {
		if task.Active {
			titleStyle = ActiveStyle.Bold(true)
		} else if task.Pinned {
			titleStyle = BoldStyle.Foreground(ColorSuccess)
		} else {
			titleStyle = SelectedItemTextStyle
		}
	} else if task.Active {
		titleStyle = ActiveStyle
	} else if task.Pinned {
		titleStyle = SuccessStyle
	}
	if inSelection && !selected && !node.IsContextNode {
		titleStyle = SelectionStyle
	}
	if task.Status == models.StatusCanceled && !node.IsContextNode {
		titleStyle = CanceledStyle
	}

	// Progress for parent tasks
	progress := ""
	if m.columnConfig.IsVisible(ColProgress) {
		if frac := renderProgressFraction(task); frac != "" {
			progress = " " + frac
		}
	}

	// Build indicators FIRST so we can account for their width when truncating the title
	indicators := ""

	// Section badge for context nodes (show which section the parent is actually in)
	// Also show section badge in Completed view to indicate where the task was originally
	// Also show section badge in All view since tasks from all sections are shown together
	if node.IsContextNode || m.viewMode == ViewModeCompleted || m.viewMode == ViewModeAll {
		sectionName := task.Section
		if sectionName == "" {
			sectionName = "Active"
		}
		// Use colored style in All view to match the GTD section colors
		if m.viewMode == ViewModeAll {
			sectionStyle := GetViewModeStyleForSection(task.Section)
			indicators += sectionStyle.Render("@ "+sectionName) + " "
		} else {
			indicators += SectionBadgeStyle.Render("@ "+sectionName) + " "
		}
	}

	// Children in other sections badge (only for non-context nodes with children)
	if !node.IsContextNode && len(task.Children) > 0 {
		if childBadge := m.renderChildrenInOtherSectionsBadge(task); childBadge != "" {
			indicators += childBadge + " "
		}
	}

	// Priority indicator (compact)
	if m.columnConfig.ShouldShowPriority(task.Priority) {
		indicators += renderPriorityBadge(task.Priority) + " "
	}

	// Due date display (compact)
	if m.columnConfig.IsVisible(ColDueDate) {
		if dueStr := formatDueDate(task); dueStr != "" {
			indicators += dueStr + " "
		}
	}

	// Snooze indicator (shown in snoozed view)
	if m.columnConfig.IsVisible(ColSnooze) && task.SnoozedUntil != nil {
		snoozeStr := task.SnoozedUntil.Format("Jan 2")
		if task.SnoozedUntil.Year() != time.Now().Year() {
			snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
		}
		indicators += MutedStyle.Render("zzz "+snoozeStr) + " "
	}

	// Recurrence indicator (with streak count when > 0)
	if m.columnConfig.IsVisible(ColRecurrence) && task.IsRecurring() {
		recurStr := "↻"
		if task.Recurrence.Streak > 0 {
			recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
		}
		indicators += MutedStyle.Render(recurStr) + " "
	}

	// Dependency indicators (compact, only when sidebar is hidden)
	if m.columnConfig.IsVisible(ColDependencies) && !m.sidebar.IsEnabled() {
		if s := renderDependencyIndicators(task, m.project); s != "" {
			indicators += s + " "
		}
	}

	// Comment indicator
	if m.columnConfig.IsVisible(ColComments) {
		if s := renderCommentIndicator(task); s != "" {
			indicators += s + " "
		}
	}

	// Attachment indicator
	if m.columnConfig.IsVisible(ColAttachments) {
		if s := renderAttachmentIndicator(task); s != "" {
			indicators += s + " "
		}
	}

	// Checklist indicator
	if m.columnConfig.IsVisible(ColChecklist) {
		if s := renderChecklistIndicator(task); s != "" {
			indicators += s + " "
		}
	}

	// Custom section badge (only when sidebar is hidden)
	if m.columnConfig.IsVisible(ColCustomSection) && task.CustomSection != "" && !m.sidebar.IsEnabled() {
		csColor := m.project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		indicators += csStyle.Render("@ "+task.CustomSection) + " "
	}

	// Score badge (when sidebar is hidden and scoring profile assigned)
	if m.columnConfig.IsVisible(ColScore) && !m.sidebar.IsEnabled() && m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
			indicators += RenderScoreBadge(task, profile) + " "
		}
	}

	// Tag badges (compact, show first 2 tags + count if more)
	if m.columnConfig.IsVisible(ColTags) && len(task.Tags) > 0 && !m.sidebar.IsEnabled() {
		indicators += renderTruncatedTags(task.Tags, 2, 8) + " "
	}

	// Add leading space to indicators if non-empty (separates from title/progress)
	if indicators != "" {
		indicators = " " + indicators
	}

	// Calculate available width for title, accounting for all other elements
	// Fixed overhead: selection prefix(3) + checkbox(0 or 4) + expand(2) + typeIcon(~2) + statusIcon(~2) + space(1) + idStr(~5) + space(1) = ~16
	// Plus: indent, progress, and indicators (now measured accurately)
	baseOverhead := 16
	indicatorWidth := lipgloss.Width(indicators)
	progressWidth := lipgloss.Width(progress)
	usedWidth := 3 + len(checkbox) + node.Level*2 + len(expandIcon) + baseOverhead + indicatorWidth + progressWidth
	availableTitleWidth := maxWidth - usedWidth
	if availableTitleWidth < 10 {
		availableTitleWidth = 10
	}
	truncatedTitle := util.Truncate(task.Title, availableTitleWidth)

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

	content := fmt.Sprintf("%s%s%s%s %s %s%s%s%s",
		checkbox, indent, expandIcon, typeIcon, statusIcon, idStr, title, progress, indicators)
	return RenderSelectableRow(selected, task.Active, task.Pinned, m.cursorFlash, content, maxWidth)
}

// renderQuickEditNode renders a task node in quick edit mode.
func (m *ProjectTreeModel) renderQuickEditNode(node *TreeNode, maxWidth int) string {
	task := node.Task

	// Build indent
	indent := strings.Repeat(TreeIndent, node.Level)

	// Type and status icons
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

	// Task ID
	idStr := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))

	// Calculate width for the text input
	prefixWidth := len(indent) + 2 + 2 + 2 + 1 + 5 + 1 // indent + typeIcon + statusIcon + space + idStr + space
	inputWidth := maxWidth - prefixWidth - 4
	if inputWidth < 20 {
		inputWidth = 20
	}
	input := m.quickEdit.GetInput()
	input.Width = inputWidth
	m.quickEdit.UpdateInput(input)

	// Render the input
	inputView := m.quickEdit.GetInput().View()

	// Build prefix
	prefix := fmt.Sprintf("%s%s %s %s ", indent, typeIcon, statusIcon, idStr)

	// Build the edit line
	editLine := prefix + inputView

	// Add preview line if in full edit mode (not rename-only)
	if !m.quickEdit.IsRenameOnly() && m.quickEdit.GetPreview() != nil && m.quickEdit.GetOriginal() != nil {
		previewText := renderQuickEditPreview(m.quickEdit.GetPreview(), m.quickEdit.GetOriginal())
		if previewText != "" {
			previewIndent := strings.Repeat(" ", len(indent)+2)
			editLine += "\n" + previewIndent + MutedStyle.Render(previewText)
		}
	}

	// Add help hint
	helpIndent := strings.Repeat(" ", len(indent)+2)
	if m.quickEdit.IsRenameOnly() {
		editLine += "\n" + helpIndent + HelpStyle.Render("Enter: save · Esc: cancel")
	} else {
		editLine += "\n" + helpIndent + HelpStyle.Render("!N priority · #tag · -#tag · @date · /section · $status · Enter: save · Esc: cancel")
	}

	return editLine
}
