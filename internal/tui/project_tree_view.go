package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func (m *ProjectTreeModel) View() string {
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
		count := len(m.selectedTasks)
		b.WriteString(SelectionCountStyle.Render(fmt.Sprintf("[MULTI-SELECT: %d selected]", count)))
		b.WriteString(" ")
		b.WriteString(MutedStyle.Render("Space: toggle | a: all | v/Esc: exit | s/m/b/x/o: status | D: delete | T: tag"))
		b.WriteString("\n")
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
		availableHeight := m.height - 8

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
	if !m.showCompleted {
		filters = append(filters, "hiding completed")
	}
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
	if selected {
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
	if inSelection && !selected {
		titleStyle = SelectionStyle
	}
	if task.Status == models.StatusCancelled {
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

	// Section indicator (only show if not empty and not Backlog)
	if task.Section != "" && task.Section != models.SectionBacklog {
		sectionColor := m.project.GetSectionColor(task.Section)
		badgeStyle := GetSectionColorStyle(sectionColor)
		indicators += badgeStyle.Render("[" + task.Section + "]")
	}

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
		{Key: "j/k", Description: "Move cursor", Category: "Navigation"},
		{Key: "Enter", Description: "View task (glow)", Category: "Navigation"},
		{Key: "Space/Tab", Description: "Expand/collapse", Category: "Navigation"},
		{Key: "E", Description: "Expand all", Category: "Navigation"},
		{Key: "c", Description: "Collapse all", Category: "Navigation"},
		{Key: "f", Description: "Focus on subtree", Category: "Navigation"},

		// Status Changes
		{Key: "s", Description: "Start (in progress)", Category: "Status Changes"},
		{Key: "m", Description: "Mark complete", Category: "Status Changes"},
		{Key: "b", Description: "Block task", Category: "Status Changes"},
		{Key: "x", Description: "Cancel task", Category: "Status Changes"},
		{Key: "o", Description: "Reopen (todo)", Category: "Status Changes"},
		{Key: "`", Description: "Cycle status forward", Category: "Status Changes"},
		{Key: "~", Description: "Cycle status backward", Category: "Status Changes"},

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
		{Key: "O", Description: "Add new note", Category: "Task Operations"},
		{Key: "e", Description: "Edit task", Category: "Task Operations"},
		{Key: "ctrl+D", Description: "Set due date", Category: "Task Operations"},
		{Key: "ctrl+N", Description: "Assign to section", Category: "Task Operations"},
		{Key: "ctrl+O", Description: "Edit in external editor", Category: "Task Operations"},
		{Key: "ctrl+P", Description: "Pin/unpin task", Category: "Task Operations"},
		{Key: "*", Description: "Toggle active flag", Category: "Task Operations"},
		{Key: "B", Description: "View dependencies", Category: "Task Operations"},
		{Key: "ctrl+B", Description: "Manage dependencies", Category: "Task Operations"},
		{Key: "D", Description: "Delete task", Category: "Task Operations"},

		// Filters
		{Key: "/", Description: "Search tasks", Category: "Filters"},
		{Key: "d", Description: "Toggle completed", Category: "Filters"},
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
		{Key: "A", Description: "Deselect all (in multi-select)", Category: "Multi-select"},

		// Project
		{Key: "A", Description: archiveDesc + " (normal mode)", Category: "Project"},

		// Clipboard
		{Key: "y", Description: "Copy title", Category: "Clipboard"},
		{Key: "Y", Description: "Copy description", Category: "Clipboard"},

		// Other
		{Key: "r", Description: "Refresh", Category: "Other"},
		{Key: "z", Description: "Undo last change", Category: "Other"},
		{Key: "\\", Description: "Jump to task", Category: "Navigation"},
		{Key: "g", Description: "Go to first", Category: "Navigation"},
		{Key: "G", Description: "Go to last", Category: "Navigation"},
		{Key: "p", Description: "Priority view", Category: "Navigation"},
		{Key: "?", Description: "Toggle help", Category: "Other"},
	}
}

// NavigateToTask expands parent nodes and positions cursor on the given task.
func (m *ProjectTreeModel) NavigateToTask(task *models.Task) {
	if task == nil {
		return
	}

	// Expand all ancestors
	m.expandAncestors(task)
	m.updateVisibility()

	// Find the task in visible nodes and set cursor
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
