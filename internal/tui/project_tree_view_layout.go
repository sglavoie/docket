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

// renderFullWidth renders the traditional full-width tree view (sidebar disabled).
func (m *ProjectTreeModel) renderFullWidth() string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("%s - Tasks", m.project.Name)
	b.WriteString(TitleStyle.Render(title))
	if m.project.Context != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" [%s]", m.project.Context)))
	}
	if storage.ScratchpadExists(m.project) {
		b.WriteString(" 🗒️")
	}

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
	if m.confirmAction != ConfirmNone {
		var prompt string
		if m.confirmAction == ConfirmDeleteRecurring && m.deleteTarget != nil {
			prompt = fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
				m.deleteTarget.ShortID, m.deleteTarget.Title)
		} else if m.confirmAction == ConfirmDelete && m.deleteTarget != nil {
			childCount := m.deleteTarget.CountChildren()
			if childCount > 0 {
				prompt = fmt.Sprintf("Delete task #%d '%s' and %d children? (y/n)", m.deleteTarget.ShortID, m.deleteTarget.Title, childCount)
			} else {
				prompt = fmt.Sprintf("Delete task #%d '%s'? (y/n)", m.deleteTarget.ShortID, m.deleteTarget.Title)
			}
		} else if m.confirmAction == ConfirmBulkDelete {
			totalChildren := 0
			for _, task := range m.getSelectedTasks() {
				totalChildren += task.CountChildren()
			}
			if totalChildren > 0 {
				prompt = fmt.Sprintf("Delete %d selected tasks and %d children? (y/n)", len(m.selectedTasks), totalChildren)
			} else {
				prompt = fmt.Sprintf("Delete %d selected tasks? (y/n)", len(m.selectedTasks))
			}
		} else if m.confirmAction == ConfirmBulkStatus {
			prompt = fmt.Sprintf("%d tasks have incomplete deps, complete %d anyway? (y/n)",
				m.bulkOp.GetBlockedCount(), len(m.bulkOp.GetTasks()))
		} else if m.confirmAction == ConfirmArchiveTasks {
			count := m.project.CountCompletedTasks()
			prompt = fmt.Sprintf("Archive %d completed task(s) to completed.md? (y/n)", count)
		} else if m.confirmAction == ConfirmTurnIntoNote && m.noteTarget != nil {
			childCount := len(m.noteTarget.Children)
			if childCount > 0 {
				prompt = fmt.Sprintf("Turn #%d '%s' into note? %d children will be reparented. (y/n)",
					m.noteTarget.ShortID, m.noteTarget.Title, childCount)
			} else {
				prompt = fmt.Sprintf("Turn #%d '%s' into note? (y/n)", m.noteTarget.ShortID, m.noteTarget.Title)
			}
		} else {
			// For ConfirmArchive/ConfirmUnarchive - format as string for display
			var action string
			switch m.confirmAction {
			case ConfirmArchive:
				action = "archive"
			case ConfirmUnarchive:
				action = "unarchive"
			}
			prompt = fmt.Sprintf("Are you sure you want to %s this project? (y/n)", action)
		}
		b.WriteString(ConfirmStyle.Render(prompt))
		b.WriteString("\n")
	}

	// Prefix key popup (which-key style)
	if popup := m.renderPrefixPopup(); popup != "" {
		b.WriteString(popup)
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
	b.WriteString("\n")

	visible := m.visibleNodes()

	// Build card content (task list + scroll indicator)
	var cardContent strings.Builder

	// Card has 4 chars horizontal overhead (2 border + 2 padding)
	cardContentWidth := m.width - 4

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			cardContent.WriteString(MutedStyle.Render("No tasks yet. Add one with: tt task add <title>"))
		} else {
			cardContent.WriteString(MutedStyle.Render("No tasks match current filters."))
		}
	} else {
		// Calculate available height for task list
		// Extra line needed when filter bar is enabled (filters on own line)
		// Card border consumes 2 lines (top + bottom)
		headerOffset := 7
		if m.filterBar.IsEnabled() {
			headerOffset = 8
		}
		availableHeight := m.height - headerOffset - 2 // -2 for card borders

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
			line := m.renderNodeWithWidth(node, i == m.cursor, inSelection, cardContentWidth)
			cardContent.WriteString(line)
			if i < endIdx-1 {
				cardContent.WriteString("\n")
			}
		}

		// Scroll indicator inside card
		if len(visible) > availableHeight {
			scrollInfo := fmt.Sprintf("Showing %d-%d of %d", startIdx+1, endIdx, len(visible))
			cardContent.WriteString("\n")
			cardContent.WriteString(MutedStyle.Render(scrollInfo))
		}
	}

	// Render the card with view mode as title
	// Full-width mode has no sidebar, so always focused
	// Use section color accent if exactly one section is filtered
	sectionAccent := m.getSingleFilteredSectionColor()
	b.WriteString(renderSidebarCard(m.getViewModeTitle(), cardContent.String(), m.width, true, sectionAccent))
	b.WriteString("\n")

	// Filter and sort status - outside the card
	var filters []string
	if m.filter.Type != "" {
		filters = append(filters, fmt.Sprintf("type: %s", m.filter.Type))
	}
	if m.filter.Status != "" {
		filters = append(filters, fmt.Sprintf("status: %s", m.filter.Status))
	}
	if m.filter.Priority == 1 {
		filters = append(filters, "priority: high")
	}
	if len(m.filter.Tags) > 0 {
		tags := make([]string, 0, len(m.filter.Tags))
		for tag := range m.filter.Tags {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		filters = append(filters, fmt.Sprintf("tags: %s", strings.Join(tags, ", ")))
	}
	if len(m.filter.Sections) > 0 {
		sections := make([]string, 0, len(m.filter.Sections))
		for section := range m.filter.Sections {
			sections = append(sections, section)
		}
		sort.Strings(sections)
		filters = append(filters, fmt.Sprintf("sections: %s", strings.Join(sections, ", ")))
	}
	if m.filter.DueDate != DueDateAll {
		var dueName string
		switch m.filter.DueDate {
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

	fullHelp := "Space: toggle | a: all | gX: deselect | v/Esc: exit | e: bulk edit | ,s/d/b/x/o: status | D: delete | T: tag"
	shortHelp := "Spc:sel a:all gX:desel v:exit e:edit ,s/d/b/x/o:status D:del T:tag"

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
	// Calculate widths: 55% tree, 1 for divider, remainder for sidebar
	treeWidth := (m.width * 55) / 100

	// Ensure minimum width for tree
	if treeWidth < 35 {
		treeWidth = 35
	}

	// Account for divider (1 char)
	dividerWidth := 1

	// Sidebar takes remaining space after tree and divider
	sidebarWidth := m.width - treeWidth - dividerWidth

	// Ensure minimum width for sidebar
	if sidebarWidth < 25 {
		sidebarWidth = 25
		// If sidebar needs more space, reduce tree width
		treeWidth = m.width - sidebarWidth - dividerWidth
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

	// Render vertical divider
	divider := m.renderVerticalDivider(m.height - 2)

	// Join horizontally: tree | divider | sidebar
	return lipgloss.JoinHorizontal(lipgloss.Top, treeContentStyled, divider, sidebarContent)
}

// renderVerticalDivider renders a vertical line for split-pane separation.
func (m *ProjectTreeModel) renderVerticalDivider(height int) string {
	if height <= 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < height; i++ {
		b.WriteString("│")
		if i < height-1 {
			b.WriteString("\n")
		}
	}
	return MutedStyle.Render(b.String())
}

// renderTreeContent renders just the tree portion (for split-pane layout).
func (m *ProjectTreeModel) renderTreeContent(width int) string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("%s - Tasks", m.project.Name)
	b.WriteString(TitleStyle.Render(title))
	if m.project.Context != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" [%s]", m.project.Context)))
	}
	if storage.ScratchpadExists(m.project) {
		b.WriteString(" 🗒️")
	}

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
	if m.confirmAction != ConfirmNone {
		var prompt string
		if m.confirmAction == ConfirmDeleteRecurring && m.deleteTarget != nil {
			prompt = fmt.Sprintf("Delete recurring #%d? (d)elete / (s)top recurrence / (n)o", m.deleteTarget.ShortID)
		} else if m.confirmAction == ConfirmDelete && m.deleteTarget != nil {
			childCount := m.deleteTarget.CountChildren()
			if childCount > 0 {
				prompt = fmt.Sprintf("Delete #%d and %d children? (y/n)", m.deleteTarget.ShortID, childCount)
			} else {
				prompt = fmt.Sprintf("Delete #%d? (y/n)", m.deleteTarget.ShortID)
			}
		} else if m.confirmAction == ConfirmBulkDelete {
			totalChildren := 0
			for _, task := range m.getSelectedTasks() {
				totalChildren += task.CountChildren()
			}
			if totalChildren > 0 {
				prompt = fmt.Sprintf("Delete %d tasks and %d children? (y/n)", len(m.selectedTasks), totalChildren)
			} else {
				prompt = fmt.Sprintf("Delete %d tasks? (y/n)", len(m.selectedTasks))
			}
		} else if m.confirmAction == ConfirmBulkStatus {
			prompt = fmt.Sprintf("%d have incomplete deps, complete %d? (y/n)",
				m.bulkOp.GetBlockedCount(), len(m.bulkOp.GetTasks()))
		} else if m.confirmAction == ConfirmArchiveTasks {
			count := m.project.CountCompletedTasks()
			prompt = fmt.Sprintf("Archive %d completed task(s)? (y/n)", count)
		} else if m.confirmAction == ConfirmTurnIntoNote && m.noteTarget != nil {
			childCount := len(m.noteTarget.Children)
			if childCount > 0 {
				prompt = fmt.Sprintf("Turn #%d into note? %d children reparented. (y/n)",
					m.noteTarget.ShortID, childCount)
			} else {
				prompt = fmt.Sprintf("Turn #%d into note? (y/n)", m.noteTarget.ShortID)
			}
		} else {
			// For ConfirmArchive/ConfirmUnarchive - format as string for display
			var action string
			switch m.confirmAction {
			case ConfirmArchive:
				action = "archive"
			case ConfirmUnarchive:
				action = "unarchive"
			}
			prompt = fmt.Sprintf("%s project? (y/n)", action)
		}
		b.WriteString(ConfirmStyle.Render(prompt))
		b.WriteString("\n")
	}

	// Prefix key popup (which-key style)
	if popup := m.renderPrefixPopup(); popup != "" {
		b.WriteString(popup)
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
	b.WriteString("\n")

	visible := m.visibleNodes()

	// Build card content (task list + scroll indicator)
	var cardContent strings.Builder

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			cardContent.WriteString(MutedStyle.Render("No tasks yet."))
		} else {
			cardContent.WriteString(MutedStyle.Render("No tasks match filters."))
		}
	} else {
		// Calculate available height for task list
		// Extra line needed when filter bar is enabled (filters on own line)
		// Card border consumes 2 lines (top + bottom)
		headerOffset := 7
		if m.filterBar.IsEnabled() {
			headerOffset = 8
		}
		availableHeight := m.height - headerOffset - 2 // -2 for card borders

		// Card has 4 chars horizontal overhead (2 border + 2 padding)
		cardContentWidth := width - 4

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

		// Render visible tasks (with adjusted width for card content)
		for i := startIdx; i < endIdx; i++ {
			node := visible[i]
			inSelection := m.selectedTasks[node.Task.ID]
			line := m.renderNodeWithWidth(node, i == m.cursor, inSelection, cardContentWidth)
			cardContent.WriteString(line)
			if i < endIdx-1 {
				cardContent.WriteString("\n")
			}
		}

		// Scroll indicator inside card
		if len(visible) > availableHeight {
			scrollInfo := fmt.Sprintf("%d-%d/%d", startIdx+1, endIdx, len(visible))
			cardContent.WriteString("\n")
			cardContent.WriteString(MutedStyle.Render(scrollInfo))
		}
	}

	// Render the card with view mode as title
	// Main tree is focused when sidebar is not focused
	// Use section color accent if exactly one section is filtered
	focused := !m.sidebar.IsFocused()
	sectionAccent := m.getSingleFilteredSectionColor()
	b.WriteString(renderSidebarCard(m.getViewModeTitle(), cardContent.String(), width, focused, sectionAccent))
	b.WriteString("\n")

	// Filter status (compact) - outside the card
	var filters []string
	if m.filter.Type != "" {
		filters = append(filters, string(m.filter.Type))
	}
	if m.filter.Priority == 1 {
		filters = append(filters, "!high")
	}
	if len(m.filter.Tags) > 0 {
		filters = append(filters, fmt.Sprintf("%d tags", len(m.filter.Tags)))
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
