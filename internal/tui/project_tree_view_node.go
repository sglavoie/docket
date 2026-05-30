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

	if m.colLayout != nil && m.colLayout.HasStatus {
		statusIcon = PadBadge(statusIcon, m.colLayout.StatusWidth)
	}
	if m.colLayout != nil && m.colLayout.HasType {
		typeIcon = PadBadge(typeIcon, m.colLayout.TypeWidth)
	}

	stageBadge := GetStageBadge(task.Stage)
	var stageSegment string
	if m.colLayout != nil && m.colLayout.HasStage {
		if stageBadge == "" {
			stageSegment = EmptyBadge(m.colLayout.StageWidth) + " "
		} else {
			stageSegment = PadBadge(stageBadge, m.colLayout.StageWidth) + " "
		}
	} else if stageBadge != "" {
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
	groups := [4]string{
		m.nodeOrgGroup(task, node),
		m.nodeLifecycleGroup(task),
		m.nodeStructureGroup(task),
		m.nodeContentGroup(task),
	}

	indicators := JoinIndicatorGroups(groups[:]...)
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
		baseUsed = 3 + 2 + lipgloss.Width(checkbox) + lipgloss.Width(node.TreePrefix) + 2 + 1 + 1 + 1 + 1 + priorityW + lipgloss.Width(stageSegment) + idW + progressWidth
	} else {
		// Approximate path (pre-existing logic).
		baseOverhead := 16
		baseUsed = 3 + 2 + len(checkbox) + lipgloss.Width(node.TreePrefix) + len(expandIcon) + baseOverhead + progressWidth
	}
	availableTitleWidth := maxWidth - baseUsed - lipgloss.Width(indicators)

	// If title would be squeezed below 20 chars, progressively drop trailing groups
	for drop := 1; drop < len(groups) && availableTitleWidth < 20; drop++ {
		indicators = JoinIndicatorGroups(groups[:len(groups)-drop]...)
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

	content := fmt.Sprintf("%s%s%s%s%s %s %s%s%s%s%s%s",
		queueIndicator, checkbox, indent, expandIcon, typeIcon, statusIcon, prioritySegment, stageSegment, idStr, title, progress, indicators)
	isDimmed := task.Dimmed && !node.IsContextNode
	isOverdue := task.IsOverdue()
	isBlocked := task.Status == models.StatusBlocked || m.project.HasIncompleteDependencies(task)
	row := RenderSelectableRow(selected, task.Active, task.Pinned, m.cursorFlash, isOverdue, isBlocked, content, maxWidth, isDimmed)
	if !zebraEnabled && !selected && !isDimmed && !isOverdue && !isBlocked && node.TopGroupIndex%2 == 1 {
		row = TreeGroupOddBgStyle.Width(maxWidth).Render(row)
	}
	return row
}

func (m *ProjectTreeModel) nodeOrgGroup(task *models.Task, node *TreeNode) string {
	var parts []string
	if node.IsContextNode || m.viewMode == ViewModeCompleted || m.viewMode == ViewModeAll {
		sectionName := task.Section
		if sectionName == "" && task.CustomSection != "" {
			sectionName = task.CustomSection
		} else if sectionName == "" {
			sectionName = "Active"
		}
		effectiveSection := task.Section
		if effectiveSection == "" && task.CustomSection != "" {
			effectiveSection = task.CustomSection
		}
		if m.viewMode == ViewModeAll {
			parts = append(parts, GetViewModeStyleForSection(effectiveSection).Render("@ "+sectionName))
		} else {
			parts = append(parts, SectionBadgeStyle.Render("@ "+sectionName))
		}
	}
	if !node.IsContextNode && len(task.Children) > 0 {
		if childBadge := m.renderChildrenInOtherSectionsBadge(task); childBadge != "" {
			parts = append(parts, childBadge)
		}
	}
	if m.columnConfig.IsVisible(ColCustomSection) && task.CustomSection != "" && !m.sidebar.IsEnabled() {
		csColor := m.project.GetSectionColor(task.CustomSection)
		parts = append(parts, GetSectionColorStyle(csColor).Render("@ "+task.CustomSection))
	}
	return JoinGroupParts(parts...)
}

func (m *ProjectTreeModel) nodeLifecycleGroup(task *models.Task) string {
	var parts []string
	if m.columnConfig.IsVisible(ColDueDate) {
		if dueStr := formatDueDate(task, m.config.GetTimeFormat()); dueStr != "" {
			parts = append(parts, dueStr)
		}
	}
	if m.columnConfig.IsVisible(ColSnooze) && task.SnoozedUntil != nil {
		snoozeStr := task.SnoozedUntil.Format("Jan 2")
		if task.SnoozedUntil.Year() != time.Now().Year() {
			snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
		}
		parts = append(parts, MutedStyle.Render("zzz "+snoozeStr))
	}
	if m.columnConfig.IsVisible(ColRecurrence) && task.IsRecurring() {
		recurStr := "↻"
		if task.Recurrence.Streak > 0 {
			recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
		}
		parts = append(parts, MutedStyle.Render(recurStr))
	}
	return JoinGroupParts(parts...)
}

func (m *ProjectTreeModel) nodeStructureGroup(task *models.Task) string {
	var parts []string
	if m.columnConfig.IsVisible(ColDependencies) && !m.sidebar.IsEnabled() {
		if s := renderDependencyIndicators(task, m.project); s != "" {
			parts = append(parts, s)
		}
	}
	return JoinGroupParts(parts...)
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
	if m.columnConfig.IsVisible(ColScore) && m.config != nil && m.project != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
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
