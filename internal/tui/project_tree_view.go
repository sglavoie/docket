package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

func (m *ProjectTreeModel) View() string {
	// If sidebar is enabled, render split-pane layout
	if m.sidebarEnabled {
		return m.renderWithSidebar()
	}

	// Otherwise, render traditional full-width view
	return m.renderFullWidth()
}

// renderFullWidth renders the traditional full-width tree view (sidebar disabled).
func (m *ProjectTreeModel) renderFullWidth() string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("%s - Tasks", m.project.Name)
	b.WriteString(TitleStyle.Render(title))

	// Show archived badge
	if m.project.Archived {
		b.WriteString(" ")
		b.WriteString(ArchivedBadgeStyle.Render("ARCHIVED"))
		if m.project.ArchivedAt != nil {
			b.WriteString(" ")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("since %s", m.project.ArchivedAt.Format(util.DateFormat))))
		}
	}
	b.WriteString("\n")

	// Filter bar
	b.WriteString(m.renderFilterBar())

	// Search bar
	if m.IsSearchMode() {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Search: \"%s\" ", m.SearchQuery())))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n")
	}

	// Focus mode indicator
	if m.focusedTask != nil {
		focusIndicator := fmt.Sprintf("[Focus: #%d %s]", m.focusedTask.ShortID, util.Truncate(m.focusedTask.Title, 40))
		b.WriteString(FocusModeStyle.Render(focusIndicator))
		b.WriteString(" ")
		b.WriteString(HelpStyle.Render("(Esc to exit)"))
		b.WriteString("\n")
	}

	// Multi-select mode indicator
	if m.multiSelectMode {
		b.WriteString(m.renderMultiSelectIndicator(m.width))
	}

	// Tag input prompt
	if m.tagInputMode {
		b.WriteString(SubtitleStyle.Render("Add tag: "))
		b.WriteString(m.tagInput.View())
		b.WriteString("\n")
	}

	// Confirmation prompt
	if m.confirmAction != "" {
		var prompt string
		if m.confirmAction == "delete" && m.deleteTarget != nil {
			childCount := m.deleteTarget.CountChildren()
			if childCount > 0 {
				prompt = fmt.Sprintf("Delete task #%d '%s' and %d children? (y/n)", m.deleteTarget.ShortID, m.deleteTarget.Title, childCount)
			} else {
				prompt = fmt.Sprintf("Delete task #%d '%s'? (y/n)", m.deleteTarget.ShortID, m.deleteTarget.Title)
			}
		} else if m.confirmAction == "bulk-delete" {
			totalChildren := 0
			for _, task := range m.getSelectedTasks() {
				totalChildren += task.CountChildren()
			}
			if totalChildren > 0 {
				prompt = fmt.Sprintf("Delete %d selected tasks and %d children? (y/n)", len(m.selectedTasks), totalChildren)
			} else {
				prompt = fmt.Sprintf("Delete %d selected tasks? (y/n)", len(m.selectedTasks))
			}
		} else {
			prompt = fmt.Sprintf("Are you sure you want to %s this project? (y/n)", m.confirmAction)
		}
		b.WriteString(ConfirmStyle.Render(prompt))
		b.WriteString("\n")
	}

	// Stats by status
	statusCounts := make(map[models.TaskStatus]int)
	for _, task := range m.project.Tasks {
		statusCounts[task.Status]++
	}

	var statusStats []string
	for _, s := range models.AllTaskStatuses() {
		if count := statusCounts[s]; count > 0 {
			style := GetStatusStyle(string(s))
			statusStats = append(statusStats, style.Render(fmt.Sprintf("%s:%d", s.Icon(), count)))
		}
	}

	if storage.ScratchpadExists(m.project) {
		b.WriteString(MutedStyle.Render("[Scratchpad]"))
		b.WriteString(" ")
	}
	b.WriteString(MutedStyle.Render("Tasks: "))
	b.WriteString(strings.Join(statusStats, " "))
	b.WriteString("\n\n")

	visible := m.visibleNodes()

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			b.WriteString(MutedStyle.Render("No tasks yet. Add one with: tt task add <title>"))
		} else {
			b.WriteString(MutedStyle.Render("No tasks match current filters."))
		}
		b.WriteString("\n")
	} else {
		// Calculate available height for task list
		// Extra line needed when filter bar is enabled (filters on own line)
		headerOffset := 7
		if m.filterBarEnabled {
			headerOffset = 8
		}
		availableHeight := m.height - headerOffset

		// Determine visible range
		startIdx := 0
		endIdx := len(visible)

		if len(visible) > availableHeight {
			// Center cursor in view
			halfView := availableHeight / 2
			startIdx = m.cursor - halfView
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + availableHeight
			if endIdx > len(visible) {
				endIdx = len(visible)
				startIdx = endIdx - availableHeight
				if startIdx < 0 {
					startIdx = 0
				}
			}
		}

		// Render visible tasks
		for i := startIdx; i < endIdx; i++ {
			node := visible[i]
			inSelection := m.selectedTasks[node.Task.ID]
			line := m.renderNode(node, i == m.cursor, inSelection)
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(visible) > availableHeight {
			scrollInfo := fmt.Sprintf("Showing %d-%d of %d", startIdx+1, endIdx, len(visible))
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(scrollInfo))
			b.WriteString("\n")
		}
	}

	// Filter and sort status
	b.WriteString("\n")
	var filters []string
	if m.filterType != "" {
		filters = append(filters, fmt.Sprintf("type: %s", m.filterType))
	}
	if m.filterStatus != "" {
		filters = append(filters, fmt.Sprintf("status: %s", m.filterStatus))
	}
	if m.filterPriority == 1 {
		filters = append(filters, "priority: high")
	}
	if len(m.filterTags) > 0 {
		tags := make([]string, 0, len(m.filterTags))
		for tag := range m.filterTags {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		filters = append(filters, fmt.Sprintf("tags: %s", strings.Join(tags, ", ")))
	}
	if len(m.filterSections) > 0 {
		sections := make([]string, 0, len(m.filterSections))
		for section := range m.filterSections {
			sections = append(sections, section)
		}
		sort.Strings(sections)
		filters = append(filters, fmt.Sprintf("sections: %s", strings.Join(sections, ", ")))
	}
	if m.filterDueDate != DueDateAll {
		var dueName string
		switch m.filterDueDate {
		case DueDateOverdue:
			dueName = "overdue"
		case DueDateToday:
			dueName = "today"
		case DueDateThisWeek:
			dueName = "this week"
		case DueDateNone:
			dueName = "no due date"
		}
		filters = append(filters, fmt.Sprintf("due: %s", dueName))
	}
	if m.SearchQuery() != "" {
		filters = append(filters, fmt.Sprintf("search: \"%s\"", m.SearchQuery()))
	}

	// Build status line
	var statusParts []string
	if len(filters) > 0 {
		// Use prominent badge style when filters are active
		filterBadge := FilterBadgeStyle.Render(fmt.Sprintf("[%d filters]", len(filters)))
		filterText := strings.Join(filters, ", ")
		statusParts = append(statusParts, filterBadge+" "+filterText)
	}
	if m.sortField != SortByDefault {
		direction := "↑"
		if !m.sortAscending {
			direction = "↓"
		}
		statusParts = append(statusParts, fmt.Sprintf("Sort: %s %s", m.sortField, direction))
	}
	if len(statusParts) > 0 {
		b.WriteString(strings.Join(statusParts, " | "))
		b.WriteString("\n")
	}

	return b.String()
}

// renderMultiSelectIndicator renders the multi-select mode indicator with adaptive formatting.
// It adjusts the layout based on available width: full help on same line when wide enough,
// wrapped to second line for medium widths, and abbreviated for narrow displays.
func (m *ProjectTreeModel) renderMultiSelectIndicator(width int) string {
	count := len(m.selectedTasks)
	badge := SelectionCountStyle.Render(fmt.Sprintf("[MULTI-SELECT: %d selected]", count))
	badgeWidth := lipgloss.Width(badge)

	fullHelp := "Space: toggle | a: all | v/Esc: exit | e: bulk edit | s/m/b/x/o: status | D: delete | T: tag"
	shortHelp := "Spc:sel a:all v:exit e:edit s/m/b/x/o:status D:del T:tag"

	// Wide terminal: badge and full help on same line
	if width >= badgeWidth+1+len(fullHelp) {
		return badge + " " + MutedStyle.Render(fullHelp) + "\n"
	}

	// Medium width: badge on first line, full help on second line
	if width >= len(fullHelp) {
		return badge + "\n" + MutedStyle.Render(fullHelp) + "\n"
	}

	// Narrow width: badge on first line, short help on second line
	if width >= len(shortHelp) {
		return badge + "\n" + MutedStyle.Render(shortHelp) + "\n"
	}

	// Very narrow: just badge
	return badge + "\n"
}

// renderWithSidebar renders the split-pane view with tree on left and sidebar on right.
func (m *ProjectTreeModel) renderWithSidebar() string {
	// Calculate widths: 55% tree, remainder for sidebar
	treeWidth := (m.width * 55) / 100

	// Ensure minimum width for tree
	if treeWidth < 35 {
		treeWidth = 35
	}

	// Sidebar always takes all remaining space
	sidebarWidth := m.width - treeWidth

	// Ensure minimum width for sidebar
	if sidebarWidth < 25 {
		sidebarWidth = 25
		// If sidebar needs more space, reduce tree width
		treeWidth = m.width - sidebarWidth
	}

	// Render tree content (reuse most of renderFullWidth logic)
	treeContent := m.renderTreeContent(treeWidth)

	// Get currently selected task
	visible := m.visibleNodes()
	var selectedTask *models.Task
	if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
		selectedTask = visible[m.cursor].Task
	}

	// Render sidebar
	// Note: SidebarStyle has border (2 chars) + padding (2 chars) = 4 chars overhead
	// lipgloss .Width() sets CONTENT width, so subtract overhead to get desired total width
	// Add 1 to account for rounding to ensure we use full width
	sidebarContent := m.renderSidebar(selectedTask, sidebarWidth-3, m.height-2)

	// Apply width constraint to tree to ensure it fills its allocated space
	treeContentStyled := lipgloss.NewStyle().Width(treeWidth).Render(treeContent)

	// Join horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, treeContentStyled, sidebarContent)
}

// renderTreeContent renders just the tree portion (for split-pane layout).
func (m *ProjectTreeModel) renderTreeContent(width int) string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("%s - Tasks", m.project.Name)
	b.WriteString(TitleStyle.Render(title))

	// Show archived badge
	if m.project.Archived {
		b.WriteString(" ")
		b.WriteString(ArchivedBadgeStyle.Render("ARCHIVED"))
		if m.project.ArchivedAt != nil {
			b.WriteString(" ")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("since %s", m.project.ArchivedAt.Format(util.DateFormat))))
		}
	}
	b.WriteString("\n")

	// Filter bar
	b.WriteString(m.renderFilterBar())

	// Search bar
	if m.IsSearchMode() {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Search: \"%s\" ", m.SearchQuery())))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n")
	}

	// Focus mode indicator
	if m.focusedTask != nil {
		focusIndicator := fmt.Sprintf("[Focus: #%d %s]", m.focusedTask.ShortID, util.Truncate(m.focusedTask.Title, 40))
		b.WriteString(FocusModeStyle.Render(focusIndicator))
		b.WriteString(" ")
		b.WriteString(HelpStyle.Render("(Esc to exit)"))
		b.WriteString("\n")
	}

	// Multi-select mode indicator
	if m.multiSelectMode {
		b.WriteString(m.renderMultiSelectIndicator(width))
	}

	// Tag input prompt
	if m.tagInputMode {
		b.WriteString(SubtitleStyle.Render("Add tag: "))
		b.WriteString(m.tagInput.View())
		b.WriteString("\n")
	}

	// Confirmation prompt
	if m.confirmAction != "" {
		var prompt string
		if m.confirmAction == "delete" && m.deleteTarget != nil {
			childCount := m.deleteTarget.CountChildren()
			if childCount > 0 {
				prompt = fmt.Sprintf("Delete #%d and %d children? (y/n)", m.deleteTarget.ShortID, childCount)
			} else {
				prompt = fmt.Sprintf("Delete #%d? (y/n)", m.deleteTarget.ShortID)
			}
		} else if m.confirmAction == "bulk-delete" {
			totalChildren := 0
			for _, task := range m.getSelectedTasks() {
				totalChildren += task.CountChildren()
			}
			if totalChildren > 0 {
				prompt = fmt.Sprintf("Delete %d tasks and %d children? (y/n)", len(m.selectedTasks), totalChildren)
			} else {
				prompt = fmt.Sprintf("Delete %d tasks? (y/n)", len(m.selectedTasks))
			}
		} else {
			prompt = fmt.Sprintf("%s project? (y/n)", m.confirmAction)
		}
		b.WriteString(ConfirmStyle.Render(prompt))
		b.WriteString("\n")
	}

	// Stats by status (compact for narrower width)
	statusCounts := make(map[models.TaskStatus]int)
	for _, task := range m.project.Tasks {
		statusCounts[task.Status]++
	}

	var statusStats []string
	for _, s := range models.AllTaskStatuses() {
		if count := statusCounts[s]; count > 0 {
			style := GetStatusStyle(string(s))
			statusStats = append(statusStats, style.Render(fmt.Sprintf("%s:%d", s.Icon(), count)))
		}
	}

	if storage.ScratchpadExists(m.project) {
		b.WriteString(MutedStyle.Render("[Scratchpad]"))
		b.WriteString(" ")
	}
	b.WriteString(MutedStyle.Render("Tasks: "))
	b.WriteString(strings.Join(statusStats, " "))
	b.WriteString("\n\n")

	visible := m.visibleNodes()

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			b.WriteString(MutedStyle.Render("No tasks yet."))
		} else {
			b.WriteString(MutedStyle.Render("No tasks match filters."))
		}
		b.WriteString("\n")
	} else {
		// Calculate available height for task list
		// Extra line needed when filter bar is enabled (filters on own line)
		headerOffset := 7
		if m.filterBarEnabled {
			headerOffset = 8
		}
		availableHeight := m.height - headerOffset

		// Determine visible range
		startIdx := 0
		endIdx := len(visible)

		if len(visible) > availableHeight {
			// Center cursor in view
			halfView := availableHeight / 2
			startIdx = m.cursor - halfView
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + availableHeight
			if endIdx > len(visible) {
				endIdx = len(visible)
				startIdx = endIdx - availableHeight
				if startIdx < 0 {
					startIdx = 0
				}
			}
		}

		// Render visible tasks (with adjusted width)
		for i := startIdx; i < endIdx; i++ {
			node := visible[i]
			inSelection := m.selectedTasks[node.Task.ID]
			line := m.renderNodeWithWidth(node, i == m.cursor, inSelection, width)
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(visible) > availableHeight {
			scrollInfo := fmt.Sprintf("%d-%d/%d", startIdx+1, endIdx, len(visible))
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(scrollInfo))
			b.WriteString("\n")
		}
	}

	// Filter status (compact)
	b.WriteString("\n")
	var filters []string
	if m.filterType != "" {
		filters = append(filters, string(m.filterType))
	}
	if m.filterPriority == 1 {
		filters = append(filters, "!high")
	}
	if len(m.filterTags) > 0 {
		filters = append(filters, fmt.Sprintf("%d tags", len(m.filterTags)))
	}

	if len(filters) > 0 {
		b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[%s]", strings.Join(filters, ","))))
		b.WriteString(" ")
	}
	if m.sortField != SortByDefault {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("sort:%s", m.sortField)))
	}

	return b.String()
}

// renderNodeWithWidth is like renderNode but accepts a custom width for truncation.
func (m *ProjectTreeModel) renderNodeWithWidth(node *TreeNode, selected bool, inSelection bool, maxWidth int) string {
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

	// Cursor
	cursor := "  "
	if selected {
		cursor = "> "
	}

	// Type and status icons with colors
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	// Override status style if task has incomplete dependencies
	if m.project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	// Context nodes (parents shown for hierarchy context) are dimmed
	if node.IsContextNode {
		typeStyle = MutedStyle
		statusStyle = MutedStyle
		expandIcon = MutedStyle.Render(strings.TrimSuffix(expandIcon, " ")) + " "
	}

	typeIcon := typeStyle.Render(task.Type.Icon())
	var statusIcon string
	if task.IsNote() {
		statusIcon = MutedStyle.Render("·")
	} else {
		statusIcon = statusStyle.Render(task.Status.Icon())
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

	// Calculate available width for title, accounting for all other elements
	// Fixed overhead: cursor(2) + checkbox(0 or 4) + expand(2) + typeIcon(~2) + statusIcon(~2) + space(1) + idStr(~5) + space(1) = ~15
	// Plus: indent, progress, and indicators (now measured accurately)
	baseOverhead := 15
	indicatorWidth := lipgloss.Width(indicators)
	progressWidth := lipgloss.Width(progress)
	usedWidth := len(cursor) + len(checkbox) + node.Level*2 + len(expandIcon) + baseOverhead + indicatorWidth + progressWidth
	availableTitleWidth := maxWidth - usedWidth
	if availableTitleWidth < 10 {
		availableTitleWidth = 10
	}
	title := titleStyle.Render(util.Truncate(task.Title, availableTitleWidth))

	return fmt.Sprintf("%s%s%s%s%s %s %s %s%s%s",
		cursor, checkbox, indent, expandIcon, typeIcon, statusIcon, idStr, title, progress, indicators)
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

	// Cursor
	cursor := "  "
	if selected {
		cursor = "> "
	}

	// Type and status icons with colors
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	// Override status style if task has incomplete dependencies
	if m.project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	// Context nodes (parents shown for hierarchy context) are dimmed
	if node.IsContextNode {
		typeStyle = MutedStyle
		statusStyle = MutedStyle
		expandIcon = MutedStyle.Render(strings.TrimSuffix(expandIcon, " ")) + " "
	}

	typeIcon := typeStyle.Render(task.Type.Icon())
	var statusIcon string
	if task.IsNote() {
		// Notes show a neutral dot instead of status icon
		statusIcon = MutedStyle.Render("·")
	} else {
		statusIcon = statusStyle.Render(task.Status.Icon())
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
	title := titleStyle.Render(util.Truncate(task.Title, m.width-node.Level*2-40))

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

	// Section is no longer displayed in tree line (moved to sidebar)

	return fmt.Sprintf("%s%s%s%s%s %s %s %s%s%s",
		cursor, checkbox, indent, expandIcon, typeIcon, statusIcon, idStr, title, progress, indicators)
}

// GetHelpItems returns the help items for the project tree view.
func (m *ProjectTreeModel) GetHelpItems() []HelpItem {
	archiveDesc := "Archive project"
	if m.project.Archived {
		archiveDesc = "Unarchive project"
	}

	return []HelpItem{
		// Navigation
		{Key: ":", Description: "Command palette", Category: "Navigation"},
		{Key: "j/k", Description: "Move cursor", Category: "Navigation"},
		{Key: "Enter", Description: "View task (glow)", Category: "Navigation"},
		{Key: "Space", Description: "Expand/collapse", Category: "Navigation"},
		{Key: "Tab", Description: "Cycle sidebar (hide/view/focus)", Category: "Navigation"},
		{Key: "E", Description: "Expand all", Category: "Navigation"},
		{Key: "c", Description: "Collapse all", Category: "Navigation"},
		{Key: "f", Description: "Focus on subtree", Category: "Navigation"},
		{Key: "g", Description: "Go to first", Category: "Navigation"},
		{Key: "G", Description: "Go to last", Category: "Navigation"},
		{Key: "\\", Description: "Jump to task", Category: "Navigation"},
		{Key: "p", Description: "Priority view", Category: "Navigation"},

		// Sidebar
		{Key: "j/k (sidebar)", Description: "Scroll/navigate dependencies", Category: "Sidebar"},
		{Key: "h/l (sidebar)", Description: "Switch dependency section", Category: "Sidebar"},
		{Key: "Enter (sidebar)", Description: "Jump to dependency", Category: "Sidebar"},
		{Key: "Esc (sidebar)", Description: "Return focus to tree", Category: "Sidebar"},

		// Status Changes
		{Key: "s", Description: "Start (in progress)", Category: "Status Changes"},
		{Key: "m", Description: "Mark complete", Category: "Status Changes"},
		{Key: "b", Description: "Block task", Category: "Status Changes"},
		{Key: "x", Description: "Cancel task", Category: "Status Changes"},
		{Key: "o", Description: "Reopen (todo)", Category: "Status Changes"},

		// Project Cycling
		{Key: "`", Description: "Next project in cycle", Category: "Project Cycling"},
		{Key: "~", Description: "Previous project in cycle", Category: "Project Cycling"},
		{Key: "@", Description: "Configure project cycle", Category: "Project Cycling"},

		// Type Changes
		{Key: "h", Description: "Promote type", Category: "Type Changes"},
		{Key: "l", Description: "Demote type", Category: "Type Changes"},
		{Key: "H", Description: "Cycle leaf left", Category: "Type Changes"},
		{Key: "L", Description: "Cycle leaf right", Category: "Type Changes"},

		// Task Movement
		{Key: "K", Description: "Move task up", Category: "Task Movement"},
		{Key: "J", Description: "Move task down", Category: "Task Movement"},
		{Key: "→", Description: "Indent (nest under above)", Category: "Task Movement"},
		{Key: "←", Description: "Outdent (to parent level)", Category: "Task Movement"},

		// Task Operations
		{Key: "n", Description: "Add new task", Category: "Task Operations"},
		{Key: "i", Description: "Quick capture", Category: "Task Operations"},
		{Key: "ctrl+I", Description: "Bulk capture", Category: "Task Operations"},
		{Key: "O", Description: "Add new note", Category: "Task Operations"},
		{Key: "e", Description: "Edit task", Category: "Task Operations"},
		{Key: "ctrl+D", Description: "Set due date", Category: "Task Operations"},
		{Key: "ctrl+N", Description: "Assign to section", Category: "Task Operations"},
		{Key: "ctrl+O", Description: "Edit in external editor", Category: "Task Operations"},
		{Key: "ctrl+P", Description: "Pin/unpin task", Category: "Task Operations"},
		{Key: "*", Description: "Toggle active flag", Category: "Task Operations"},
		{Key: "R", Description: "View dependencies", Category: "Task Operations"},
		{Key: "ctrl+B", Description: "Manage dependencies", Category: "Task Operations"},
		{Key: "C", Description: "View/add comments", Category: "Task Operations"},
		{Key: "D", Description: "Delete task", Category: "Task Operations"},

		// Views
		{Key: "V", Description: "Cycle view (Active/Inbox/Backlog)", Category: "Views"},
		{Key: "A", Description: "Send task(s) to Active", Category: "Views"},
		{Key: "B", Description: "Send task(s) to Backlog", Category: "Views"},
		{Key: "I", Description: "Send task(s) to Inbox", Category: "Views"},
		{Key: "w", Description: "Inbox processor", Category: "Views"},

		// Filters
		{Key: "/", Description: "Search tasks", Category: "Filters"},
		{Key: "C", Description: "Switch to Completed view", Category: "Filters"},
		{Key: "P", Description: "Filter by priority", Category: "Filters"},
		{Key: "t", Description: "Filter by type", Category: "Filters"},
		{Key: "S", Description: "Filter by status", Category: "Filters"},
		{Key: "T", Description: "Filter by tag", Category: "Filters"},
		{Key: "N", Description: "Filter by section", Category: "Filters"},
		{Key: "u", Description: "Filter by due date", Category: "Filters"},
		{Key: "ctrl+F", Description: "Filter summary", Category: "Filters"},
		{Key: "ctrl+T", Description: "Tag manager", Category: "Filters"},
		{Key: "ctrl+S", Description: "Section manager", Category: "Filters"},

		// Sort
		{Key: "1", Description: "Sort by priority", Category: "Sort"},
		{Key: "2", Description: "Sort by due date", Category: "Sort"},
		{Key: "3", Description: "Sort by created", Category: "Sort"},
		{Key: "4", Description: "Sort by status", Category: "Sort"},
		{Key: "0", Description: "Reset sort", Category: "Sort"},

		// Multi-select
		{Key: "v", Description: "Toggle multi-select mode", Category: "Multi-select"},
		{Key: "a", Description: "Select all visible (in multi-select)", Category: "Multi-select"},

		// Project
		{Key: "ctrl+A", Description: archiveDesc, Category: "Project"},

		// Clipboard
		{Key: "y", Description: "Copy title", Category: "Clipboard"},
		{Key: "Y", Description: "Copy description", Category: "Clipboard"},

		// Filter Bar
		{Key: "F", Description: "Toggle filter bar", Category: "Filter Bar"},
		{Key: "F1", Description: "Switch to Completed view", Category: "Filter Bar"},
		{Key: "F2", Description: "Toggle high priority filter", Category: "Filter Bar"},
		{Key: "F3", Description: "Cycle type filter", Category: "Filter Bar"},
		{Key: "F4", Description: "Cycle status filter", Category: "Filter Bar"},
		{Key: "F5", Description: "Tag filter selector", Category: "Filter Bar"},
		{Key: "F6", Description: "Section filter selector", Category: "Filter Bar"},
		{Key: "F7", Description: "Cycle due date filter", Category: "Filter Bar"},

		// Scratchpad
		{Key: "[", Description: "View scratchpad", Category: "Scratchpad"},
		{Key: "]", Description: "Edit scratchpad", Category: "Scratchpad"},

		// Other
		{Key: "ctrl+R", Description: "Refresh", Category: "Other"},
		{Key: "z", Description: "Undo last change", Category: "Other"},
		{Key: "ctrl+Z", Description: "Undo history", Category: "Other"},
		{Key: "?", Description: "Toggle help", Category: "Other"},
	}
}

// renderFilterBar renders the filter toggle bar with task counts.
// renderViewModeIndicator returns a styled indicator for the current view mode with counts.
func (m *ProjectTreeModel) renderViewModeIndicator() string {
	// Count inbox, backlog, active, and completed items
	var inboxCount, backlogCount, activeCount, completedCount int
	for _, task := range m.project.Tasks {
		if task.IsComplete() {
			completedCount++
			continue
		}
		if task.Section == models.SectionInbox && task.IsRoot() {
			inboxCount++
		} else if task.Section == models.SectionBacklog && task.IsRoot() {
			backlogCount++
		} else if task.Section != models.SectionInbox && task.Section != models.SectionBacklog {
			activeCount++
		}
	}

	// Build indicator for each view with counts
	var parts []string

	// Active view
	activeLabel := fmt.Sprintf("Active[%d]", activeCount)
	if m.viewMode == ViewModeActive {
		parts = append(parts, ViewModeActiveStyle.Render(activeLabel))
	} else {
		parts = append(parts, MutedStyle.Render(activeLabel))
	}

	// Inbox view
	inboxLabel := fmt.Sprintf("Inbox[%d]", inboxCount)
	if m.viewMode == ViewModeInbox {
		parts = append(parts, ViewModeInboxStyle.Render(inboxLabel))
	} else {
		parts = append(parts, MutedStyle.Render(inboxLabel))
	}

	// Backlog view
	backlogLabel := fmt.Sprintf("Backlog[%d]", backlogCount)
	if m.viewMode == ViewModeBacklog {
		parts = append(parts, ViewModeBacklogStyle.Render(backlogLabel))
	} else {
		parts = append(parts, MutedStyle.Render(backlogLabel))
	}

	// Completed view
	completedLabel := fmt.Sprintf("Completed[%d]", completedCount)
	if m.viewMode == ViewModeCompleted {
		parts = append(parts, ViewModeCompletedStyle.Render(completedLabel))
	} else {
		parts = append(parts, MutedStyle.Render(completedLabel))
	}

	return strings.Join(parts, " ")
}

func (m *ProjectTreeModel) renderFilterBar() string {
	var result strings.Builder

	if !m.filterBarEnabled {
		// When filter bar is disabled, just show view mode indicator
		result.WriteString(m.renderViewModeIndicator())
		result.WriteString("\n")
		return result.String()
	}

	// Line 1: Filter badges (F1-F7)
	var badges []string

	// F1: Completed view (switch to Completed view)
	count := m.filterCounts["completed"]
	active := m.viewMode == ViewModeCompleted
	badges = append(badges, m.renderFilterBadge("F1", "Done", count, active))

	// F2: High Priority
	count = m.filterCounts["highPriority"]
	active = m.filterPriority == 1
	badges = append(badges, m.renderFilterBadge("F2", "!Hi", count, active))

	// F3: Type (show current type if filtered)
	typeName := "Type"
	if m.filterType != "" {
		typeName = string(m.filterType)
		count = m.filterCounts["type:"+string(m.filterType)]
	} else {
		count = len(m.project.Tasks)
	}
	active = m.filterType != ""
	badges = append(badges, m.renderFilterBadge("F3", typeName, count, active))

	// F4: Status (show current status if filtered)
	statusName := "Status"
	if m.filterStatus != "" {
		statusName = string(m.filterStatus)
		count = m.filterCounts["status:"+string(m.filterStatus)]
	} else {
		count = len(m.project.Tasks)
	}
	active = m.filterStatus != ""
	badges = append(badges, m.renderFilterBadge("F4", statusName, count, active))

	// F5: Tags (show count of selected tags)
	tagCount := len(m.filterTags)
	if tagCount > 0 {
		badges = append(badges, m.renderFilterBadge("F5", fmt.Sprintf("#%dtag", tagCount), tagCount, true))
	} else {
		badges = append(badges, m.renderFilterBadge("F5", "Tags", 0, false))
	}

	// F6: Section (show count of selected sections)
	sectionCount := len(m.filterSections)
	if sectionCount > 0 {
		badges = append(badges, m.renderFilterBadge("F6", fmt.Sprintf("@%dsec", sectionCount), sectionCount, true))
	} else {
		badges = append(badges, m.renderFilterBadge("F6", "Sect", 0, false))
	}

	// F7: Due Date
	dueName := "Due"
	switch m.filterDueDate {
	case DueDateOverdue:
		dueName = "Ovrd"
		count = m.filterCounts["overdue"]
	case DueDateToday:
		dueName = "Today"
		count = m.filterCounts["today"]
	case DueDateThisWeek:
		dueName = "Week"
		count = m.filterCounts["thisWeek"]
	default:
		count = 0
	}
	active = m.filterDueDate != DueDateAll
	badges = append(badges, m.renderFilterBadge("F7", dueName, count, active))

	result.WriteString(strings.Join(badges, " "))
	result.WriteString("\n")

	// Line 2: View mode indicator
	result.WriteString(m.renderViewModeIndicator())
	result.WriteString("\n")

	return result.String()
}

// renderFilterBadge renders a single filter badge with key, name, count, and active state.
func (m *ProjectTreeModel) renderFilterBadge(key, name string, count int, active bool) string {
	var countStr string
	if count > 0 {
		countStr = fmt.Sprintf("[%d]", count)
	}

	label := fmt.Sprintf("%s:%s%s", key, name, countStr)

	if active {
		return FilterBadgeStyle.Render(label)
	}
	return MutedStyle.Render(label)
}

// NavigateToTask expands parent nodes and positions cursor on the given task.
// If the task is filtered out, it clears filters and switches view modes as needed.
func (m *ProjectTreeModel) NavigateToTask(task *models.Task) {
	if task == nil {
		return
	}

	// First, try to find in current nodes (task may exist but be collapsed)
	for _, node := range m.nodes {
		if node.Task.ID == task.ID {
			// Task exists in tree, just expand ancestors and navigate
			m.expandAncestors(task)
			m.updateVisibility()
			visible := m.visibleNodes()
			for i, n := range visible {
				if n.Task.ID == task.ID {
					m.cursor = i
					return
				}
			}
		}
	}

	// Task not in current tree - need to adjust view to show it
	// Clear focus mode first
	if m.focusedTask != nil {
		m.focusedTask = nil
	}

	// Determine appropriate view mode based on task state
	if task.IsComplete() {
		m.viewMode = ViewModeCompleted
	} else {
		// Switch to view mode that includes the task's section
		switch task.Section {
		case models.SectionInbox:
			m.viewMode = ViewModeInbox
		case models.SectionBacklog:
			m.viewMode = ViewModeBacklog
		default:
			m.viewMode = ViewModeActive
		}
	}

	// Clear all filters that might hide the task
	m.filterType = ""
	m.filterStatus = ""
	m.filterPriority = 0
	m.filterTags = nil
	m.filterSections = nil
	m.filterDueDate = DueDateAll
	m.ClearSearch()

	// Rebuild tree with cleared filters
	m.buildTree()

	// Now expand ancestors and find the task
	m.expandAncestors(task)
	m.updateVisibility()

	visible := m.visibleNodes()
	for i, node := range visible {
		if node.Task.ID == task.ID {
			m.cursor = i
			return
		}
	}
}

// expandAncestors marks all ancestor nodes as expanded.
func (m *ProjectTreeModel) expandAncestors(task *models.Task) {
	// Walk up the parent chain
	current := task.Parent
	for current != nil {
		// Find the node for this parent and expand it
		for _, node := range m.nodes {
			if node.Task.ID == current.ID {
				node.Expanded = true
				break
			}
		}
		current = current.Parent
	}
}

// renderChildrenInOtherSectionsBadge returns a badge showing how many children
// are in sections hidden by the current view mode.
func (m *ProjectTreeModel) renderChildrenInOtherSectionsBadge(task *models.Task) string {
	hiddenSections := m.getHiddenSectionsForViewMode()
	if len(hiddenSections) == 0 {
		return ""
	}

	counts, ok := m.childrenInOtherSections[task.ID]
	if !ok || len(counts) == 0 {
		return ""
	}

	var parts []string
	for _, section := range hiddenSections {
		if count, exists := counts[section]; exists && count > 0 {
			parts = append(parts, fmt.Sprintf("+%d %s", count, section))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return ChildrenBadgeStyle.Render("[" + strings.Join(parts, ", ") + "]")
}
