package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderNodeWithWidth is like renderNode but accepts a custom width for truncation.
func (m *ProjectTreeModel) renderNodeWithWidth(node *TreeNode, selected bool, inSelection bool, maxWidth int) string {
	task := node.Task

	// Check if this task is being quick-edited
	if m.quickEditMode && m.quickEditTask != nil && task.ID == m.quickEditTask.ID {
		return m.renderQuickEditNode(node, maxWidth)
	}

	// Build indent
	indent := strings.Repeat(TreeIndent, node.Level)

	// Expand/collapse indicator
	expandIcon := "  "
	if len(task.Children) > 0 {
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

	// Type and status icons with colors
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	// Override status style if task has incomplete dependencies
	if m.project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	// Context nodes (parents shown for hierarchy context) are dimmed
	var typeIcon string
	if node.IsContextNode {
		typeStyle = MutedStyle
		statusStyle = MutedStyle
		expandIcon = MutedStyle.Render(strings.TrimSuffix(expandIcon, " ")) + " "
		typeIcon = typeStyle.Render(task.Type.Icon())
	} else {
		typeIcon = GetTypeBadge(string(task.Type), task.Type.Icon())
	}

	var statusIcon string
	if task.IsNote() {
		statusIcon = MutedStyle.Render("·")
	} else if node.IsContextNode {
		statusIcon = statusStyle.Render(task.Status.Icon())
	} else if m.project.HasIncompleteDependencies(task) {
		statusIcon = BlockedByDepsStyle.Render(task.Status.Icon())
	} else {
		statusIcon = GetStatusBadge(string(task.Status), task.Status.Icon())
	}

	// Task ID
	idStr := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))

	// Title
	titleStyle := NormalStyle
	if node.IsContextNode {
		// Context nodes are always dimmed
		titleStyle = MutedStyle
	} else if selected {
		if task.Active {
			titleStyle = BoldStyle.Foreground(lipgloss.Color("226"))
		} else if task.Pinned {
			titleStyle = BoldStyle.Foreground(ColorSuccess)
		} else {
			titleStyle = BoldStyle.Foreground(ColorPrimary)
		}
	} else if task.Active {
		titleStyle = ActiveStyle
	} else if task.Pinned {
		titleStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	}
	if inSelection && !selected && !node.IsContextNode {
		titleStyle = SelectionStyle
	}
	if task.Status == models.StatusCancelled && !node.IsContextNode {
		titleStyle = CancelledStyle
	}

	// Progress for parent tasks
	progress := ""
	if task.HasProgressChildren() {
		pct := task.GetProgress()
		progress = MutedStyle.Render(fmt.Sprintf(" [%.0f%%]", pct))
	}

	// Build indicators FIRST so we can account for their width when truncating the title
	indicators := ""

	// Section badge for context nodes (show which section the parent is actually in)
	// Also show section badge in Completed view to indicate where the task was originally
	if node.IsContextNode || m.viewMode == ViewModeCompleted {
		sectionName := task.Section
		if sectionName == "" {
			sectionName = "Active"
		}
		indicators += SectionBadgeStyle.Render("@ "+sectionName) + " "
	}

	// Children in other sections badge (only for non-context nodes with children)
	if !node.IsContextNode && len(task.Children) > 0 {
		if childBadge := m.renderChildrenInOtherSectionsBadge(task); childBadge != "" {
			indicators += childBadge + " "
		}
	}

	// Priority indicator (compact)
	if !task.IsNote() && task.Priority <= 2 {
		indicators += formatPriorityBadge(task.Priority) + " "
	}

	// Due date display (compact)
	if dueStr := formatDueDate(task); dueStr != "" {
		indicators += dueStr + " "
	}

	// Recurrence indicator
	if task.IsRecurring() {
		indicators += MutedStyle.Render("↻") + " "
	}

	// Dependency indicators (compact)
	if len(task.DependsOn) > 0 {
		indicators += DependencyStyle.Render(fmt.Sprintf("→%d", len(task.DependsOn))) + " "
	}
	if blockedBy := m.project.GetBlockedByTasks(task); len(blockedBy) > 0 {
		indicators += DependencyStyle.Render(fmt.Sprintf("←%d", len(blockedBy))) + " "
	}

	// Comment indicator (compact)
	if len(task.Comments) > 0 {
		indicators += MutedStyle.Render(fmt.Sprintf("💬%d", len(task.Comments))) + " "
	}

	// Tag badges (compact, show first 2 tags + count if more)
	if len(task.Tags) > 0 {
		maxTags := 2
		maxTagLen := 8
		var tagParts []string
		for i, tag := range task.Tags {
			if i >= maxTags {
				break
			}
			displayTag := tag
			if len(displayTag) > maxTagLen {
				displayTag = displayTag[:maxTagLen-1] + "…"
			}
			tagParts = append(tagParts, "#"+displayTag)
		}
		tagStr := TagInlineStyle.Render(strings.Join(tagParts, " "))
		if len(task.Tags) > maxTags {
			tagStr += MutedStyle.Render(fmt.Sprintf(" +%d", len(task.Tags)-maxTags))
		}
		indicators += tagStr + " "
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

	content := fmt.Sprintf("%s%s%s%s %s %s %s%s%s",
		checkbox, indent, expandIcon, typeIcon, statusIcon, idStr, title, progress, indicators)
	return RenderSelectableRow(selected, task.Active, content, maxWidth)
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
	m.quickEditInput.Width = inputWidth

	// Render the input
	inputView := m.quickEditInput.View()

	// Build prefix
	prefix := fmt.Sprintf("%s%s %s %s ", indent, typeIcon, statusIcon, idStr)

	// Build the edit line
	editLine := prefix + inputView

	// Add preview line if in full edit mode (not rename-only)
	if !m.quickEditRename && m.quickEditPreview != nil && m.quickEditOriginal != nil {
		previewText := renderQuickEditPreview(m.quickEditPreview, m.quickEditOriginal)
		if previewText != "" {
			previewIndent := strings.Repeat(" ", len(indent)+2)
			editLine += "\n" + previewIndent + MutedStyle.Render(previewText)
		}
	}

	// Add help hint
	helpIndent := strings.Repeat(" ", len(indent)+2)
	if m.quickEditRename {
		editLine += "\n" + helpIndent + HelpStyle.Render("Enter: save · Esc: cancel")
	} else {
		editLine += "\n" + helpIndent + HelpStyle.Render("!N priority · #tag · -#tag · @date · /section · $status | Enter: save · Esc: cancel")
	}

	return editLine
}

func (m *ProjectTreeModel) renderNode(node *TreeNode, selected bool, inSelection bool) string {
	task := node.Task

	// Build indent
	indent := strings.Repeat(TreeIndent, node.Level)

	// Expand/collapse indicator
	expandIcon := "  "
	if len(task.Children) > 0 {
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

	// Type and status icons with colors
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	// Override status style if task has incomplete dependencies
	if m.project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	// Context nodes (parents shown for hierarchy context) are dimmed
	var typeIcon string
	if node.IsContextNode {
		typeStyle = MutedStyle
		statusStyle = MutedStyle
		expandIcon = MutedStyle.Render(strings.TrimSuffix(expandIcon, " ")) + " "
		typeIcon = typeStyle.Render(task.Type.Icon())
	} else {
		typeIcon = GetTypeBadge(string(task.Type), task.Type.Icon())
	}

	var statusIcon string
	if task.IsNote() {
		// Notes show a neutral dot instead of status icon
		statusIcon = MutedStyle.Render("·")
	} else if node.IsContextNode {
		statusIcon = statusStyle.Render(task.Status.Icon())
	} else if m.project.HasIncompleteDependencies(task) {
		statusIcon = BlockedByDepsStyle.Render(task.Status.Icon())
	} else {
		statusIcon = GetStatusBadge(string(task.Status), task.Status.Icon())
	}

	// Task ID
	idStr := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))

	// Title
	titleStyle := NormalStyle
	if node.IsContextNode {
		// Context nodes are always dimmed
		titleStyle = MutedStyle
	} else if selected {
		if task.Active {
			titleStyle = BoldStyle.Foreground(lipgloss.Color("226")) // Bright yellow
		} else if task.Pinned {
			titleStyle = BoldStyle.Foreground(ColorSuccess)
		} else {
			titleStyle = BoldStyle.Foreground(ColorPrimary)
		}
	} else if task.Active {
		titleStyle = ActiveStyle
	} else if task.Pinned {
		titleStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	}
	if inSelection && !selected && !node.IsContextNode {
		titleStyle = SelectionStyle
	}
	if task.Status == models.StatusCancelled && !node.IsContextNode {
		titleStyle = CancelledStyle
	}
	truncatedTitle := util.Truncate(task.Title, m.width-node.Level*2-42)

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

	// Progress for parent tasks (only show if has non-note children)
	progress := ""
	if task.HasProgressChildren() {
		pct := task.GetProgress()
		progress = MutedStyle.Render(fmt.Sprintf(" [%.0f%%]", pct))
	}

	// Priority, due date, and dependency indicators
	indicators := ""

	// Section badge for context nodes (show which section the parent is actually in)
	// Also show section badge in Completed view to indicate where the task was originally
	if node.IsContextNode || m.viewMode == ViewModeCompleted {
		sectionName := task.Section
		if sectionName == "" {
			sectionName = "Active"
		}
		indicators += SectionBadgeStyle.Render("@ "+sectionName) + " "
	}

	// Children in other sections badge (only for non-context nodes with children)
	if !node.IsContextNode && len(task.Children) > 0 {
		if childBadge := m.renderChildrenInOtherSectionsBadge(task); childBadge != "" {
			indicators += childBadge + " "
		}
	}

	// Priority indicator with level number (skip for notes)
	if !task.IsNote() {
		switch task.Priority {
		case 1:
			indicators += PriorityHighStyle.Render("!1") + " "
		case 2:
			indicators += PriorityMediumStyle.Render("!2") + " "
		case 3:
			indicators += PriorityLowStyle.Render("!3") + " "
		case 4:
			indicators += PriorityLowStyle.Render("!4") + " "
		case 5:
			indicators += PriorityLowStyle.Render("!5") + " "
		}
	}

	// Due date display (includes overdue styling internally)
	if dueStr := formatDueDate(task); dueStr != "" {
		indicators += dueStr + " "
	}

	// Dependency indicator
	if len(task.DependsOn) > 0 {
		// Check if all dependencies are completed
		blockingTasks := m.project.GetBlockingTasks(task)
		allCompleted := true
		for _, dep := range blockingTasks {
			if !dep.IsComplete() {
				allCompleted = false
				break
			}
		}
		depStyle := DependencyStyle
		if allCompleted {
			depStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
		}
		indicators += depStyle.Render(fmt.Sprintf("→%d", len(task.DependsOn))) + " "
	}

	// Reverse dependency indicator (tasks that depend on this task)
	if blockedBy := m.project.GetBlockedByTasks(task); len(blockedBy) > 0 {
		indicators += DependencyStyle.Render(fmt.Sprintf("←%d", len(blockedBy))) + " "
	}

	// Comment indicator
	if len(task.Comments) > 0 {
		indicators += MutedStyle.Render(fmt.Sprintf("💬%d", len(task.Comments))) + " "
	}

	// Tag badges (compact, show first 2 tags + count if more)
	if len(task.Tags) > 0 {
		maxTags := 2
		maxTagLen := 8
		var tagParts []string
		for i, tag := range task.Tags {
			if i >= maxTags {
				break
			}
			displayTag := tag
			if len(displayTag) > maxTagLen {
				displayTag = displayTag[:maxTagLen-1] + "…"
			}
			tagParts = append(tagParts, "#"+displayTag)
		}
		tagStr := TagInlineStyle.Render(strings.Join(tagParts, " "))
		if len(task.Tags) > maxTags {
			tagStr += MutedStyle.Render(fmt.Sprintf(" +%d", len(task.Tags)-maxTags))
		}
		indicators += tagStr + " "
	}

	// Add leading space to indicators if non-empty (separates from title/progress)
	if indicators != "" {
		indicators = " " + indicators
	}

	// Section is no longer displayed in tree line (moved to sidebar)

	content := fmt.Sprintf("%s%s%s%s %s %s %s%s%s",
		checkbox, indent, expandIcon, typeIcon, statusIcon, idStr, title, progress, indicators)
	return RenderSelectableRow(selected, task.Active, content, m.width)
}
