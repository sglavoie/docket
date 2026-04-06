package tui

import (
	"fmt"
	"strings"
	"time"

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

	// Build indent using tree connectors (├── , └── , │   ).
	// For leaf nodes, extend the connector line horizontally through the
	// expand icon gap so the line reaches the type icon without dead space.
	var indent string
	expandIcon := "  "
	if node.HasFilteredChildren {
		if node.Expanded {
			expandIcon = "▼ "
		} else {
			expandIcon = "▶ "
		}
		indent = MutedStyle.Render(node.TreePrefix)
	} else if node.TreePrefix != "" {
		runes := []rune(node.TreePrefix)
		if runes[len(runes)-1] == ' ' {
			runes[len(runes)-1] = '─'
		}
		indent = MutedStyle.Render(string(runes))
		expandIcon = MutedStyle.Render("─") + " "
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

	stageBadge := GetStageBadge(task.Stage)
	var stageSegment string
	if stageBadge != "" {
		stageSegment = stageBadge + " "
	}

	// Priority segment (inline, before stage)
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

	// Title
	titleStyle := NormalStyle
	if node.IsContextNode {
		// Context nodes are always dimmed
		titleStyle = MutedStyle
	} else if selected {
		if task.Active {
			titleStyle = ActiveStyle.Bold(true)
		} else if task.Pinned {
			titleStyle = SuccessBoldStyle
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

	// Build indicators in semantic groups for visual clustering
	// Organization group: section badge, children-elsewhere, short ID, custom section
	var orgParts []string
	if node.IsContextNode || m.viewMode == ViewModeCompleted || m.viewMode == ViewModeAll {
		sectionName := task.Section
		if sectionName == "" {
			sectionName = "Active"
		}
		if m.viewMode == ViewModeAll {
			sectionStyle := GetViewModeStyleForSection(task.Section)
			orgParts = append(orgParts, sectionStyle.Render("@ "+sectionName))
		} else {
			orgParts = append(orgParts, SectionBadgeStyle.Render("@ "+sectionName))
		}
	}
	if !node.IsContextNode && len(task.Children) > 0 {
		if childBadge := m.renderChildrenInOtherSectionsBadge(task); childBadge != "" {
			orgParts = append(orgParts, childBadge)
		}
	}
	if m.columnConfig.IsVisible(ColCustomSection) && task.CustomSection != "" && !m.sidebar.IsEnabled() {
		csColor := m.project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		orgParts = append(orgParts, csStyle.Render("@ "+task.CustomSection))
	}
	orgGroup := JoinGroupParts(orgParts...)

	// Lifecycle group: due date, snooze, recurrence
	var lifecycleParts []string
	if m.columnConfig.IsVisible(ColDueDate) {
		if dueStr := formatDueDate(task); dueStr != "" {
			lifecycleParts = append(lifecycleParts, dueStr)
		}
	}
	if m.columnConfig.IsVisible(ColSnooze) && task.SnoozedUntil != nil {
		snoozeStr := task.SnoozedUntil.Format("Jan 2")
		if task.SnoozedUntil.Year() != time.Now().Year() {
			snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
		}
		lifecycleParts = append(lifecycleParts, MutedStyle.Render("zzz "+snoozeStr))
	}
	if m.columnConfig.IsVisible(ColRecurrence) && task.IsRecurring() {
		recurStr := "↻"
		if task.Recurrence.Streak > 0 {
			recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
		}
		lifecycleParts = append(lifecycleParts, MutedStyle.Render(recurStr))
	}
	lifecycleGroup := JoinGroupParts(lifecycleParts...)

	// Structure group: dependencies, score
	var structureParts []string
	if m.columnConfig.IsVisible(ColDependencies) && !m.sidebar.IsEnabled() {
		if s := renderDependencyIndicators(task, m.project); s != "" {
			structureParts = append(structureParts, s)
		}
	}
	if m.columnConfig.IsVisible(ColScore) && !m.sidebar.IsEnabled() && m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
			if badge := RenderScoreBadge(task, profile); badge != "" {
				structureParts = append(structureParts, badge)
			}
		}
	}
	structureGroup := JoinGroupParts(structureParts...)

	// Content group: comments, attachments, checklist, tags
	var contentParts []string
	if m.columnConfig.IsVisible(ColComments) {
		if s := renderCommentIndicator(task); s != "" {
			contentParts = append(contentParts, s)
		}
	}
	if m.columnConfig.IsVisible(ColAttachments) {
		if s := renderAttachmentIndicator(task); s != "" {
			contentParts = append(contentParts, s)
		}
	}
	if m.columnConfig.IsVisible(ColChecklist) {
		if s := renderChecklistIndicator(task); s != "" {
			contentParts = append(contentParts, s)
		}
	}
	if m.columnConfig.IsVisible(ColTags) && len(task.Tags) > 0 && !m.sidebar.IsEnabled() {
		contentParts = append(contentParts, renderTruncatedTags(task.Tags, 2, 8))
	}
	contentGroup := JoinGroupParts(contentParts...)

	indicators := JoinIndicatorGroups(orgGroup, lifecycleGroup, structureGroup, contentGroup)
	if indicators != "" {
		indicators = " " + indicators
	}

	// Calculate available width for title, accounting for all other elements.
	progressWidth := lipgloss.Width(progress)
	var baseUsed int
	if m.colLayout != nil {
		// Fixed-width path: compute exact overhead from column layout.
		// format: cursor(3) + checkbox + indent(actual prefix) + expandIcon(2) + typeIcon(1) +
		//         space(1) + statusIcon(1) + space(1) + prioritySegment + stageSegment + idStr
		priorityW := 0
		if m.colLayout.HasPriority {
			priorityW = m.colLayout.PriorityWidth + 1
		}
		idW := 0
		if m.colLayout.HasShortID {
			idW = m.colLayout.ShortIDWidth + 1
		}
		baseUsed = 3 + lipgloss.Width(checkbox) + lipgloss.Width(node.TreePrefix) + 2 + 1 + 1 + 1 + 1 + priorityW + lipgloss.Width(stageSegment) + idW + progressWidth
	} else {
		// Approximate path (pre-existing logic).
		baseOverhead := 16
		baseUsed = 3 + len(checkbox) + lipgloss.Width(node.TreePrefix) + len(expandIcon) + baseOverhead + progressWidth
	}
	availableTitleWidth := maxWidth - baseUsed - lipgloss.Width(indicators)

	// If title would be squeezed below 20 chars, progressively drop indicator groups
	if availableTitleWidth < 20 {
		indicators = JoinIndicatorGroups(orgGroup, lifecycleGroup, structureGroup)
		if indicators != "" {
			indicators = " " + indicators
		}
		availableTitleWidth = maxWidth - baseUsed - lipgloss.Width(indicators)
	}
	if availableTitleWidth < 20 {
		indicators = JoinIndicatorGroups(orgGroup, lifecycleGroup)
		if indicators != "" {
			indicators = " " + indicators
		}
		availableTitleWidth = maxWidth - baseUsed - lipgloss.Width(indicators)
	}
	if availableTitleWidth < 20 {
		indicators = JoinIndicatorGroups(orgGroup)
		if indicators != "" {
			indicators = " " + indicators
		}
		availableTitleWidth = maxWidth - baseUsed - lipgloss.Width(indicators)
	}
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

	content := fmt.Sprintf("%s%s%s%s %s %s%s%s%s%s%s",
		checkbox, indent, expandIcon, typeIcon, statusIcon, prioritySegment, stageSegment, idStr, title, progress, indicators)
	isDimmed := task.Dimmed && !node.IsContextNode
	return RenderSelectableRow(selected, task.Active, task.Pinned, m.cursorFlash, content, maxWidth, isDimmed)
}

// renderQuickEditNode renders a task node in quick edit mode.
func (m *ProjectTreeModel) renderQuickEditNode(node *TreeNode, maxWidth int) string {
	task := node.Task

	// Build indent using tree connectors
	indent := MutedStyle.Render(node.TreePrefix)
	indentWidth := lipgloss.Width(node.TreePrefix)

	// Type and status icons
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

	stageBadge := GetStageBadge(task.Stage)
	stageSegment := ""
	if stageBadge != "" {
		stageSegment = stageBadge + " "
	}

	priorityBadge := renderPriorityBadge(task.Priority)
	prioritySegment := ""
	if priorityBadge != "" {
		prioritySegment = priorityBadge + " "
	}

	// Task ID
	idStr := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))

	// Calculate width for the text input
	priorityW := lipgloss.Width(prioritySegment)
	stageW := lipgloss.Width(stageSegment)
	prefixWidth := indentWidth + 2 + 1 + 1 + priorityW + stageW + 5 + 1 // indent + typeIcon + space + statusIcon + space + priority + stage + idStr + space
	inputWidth := max(maxWidth-prefixWidth-4, 20)
	input := m.quickEdit.GetInput()
	input.SetWidth(inputWidth)
	m.quickEdit.UpdateInput(input)

	// Render the input
	inputView := m.quickEdit.GetInput().View()

	// Build prefix
	prefix := fmt.Sprintf("%s%s %s %s%s%s ", indent, typeIcon, statusIcon, prioritySegment, stageSegment, idStr)

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
