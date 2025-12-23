package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

func (m *ProjectTreeModel) buildTree() {
	// Preserve expanded state before rebuilding
	expandedState := make(map[string]bool)
	for _, node := range m.nodes {
		expandedState[node.Task.ID] = node.Expanded
	}

	m.nodes = []*TreeNode{}

	// Get root tasks
	rootTasks := m.project.GetRootTasks()

	// Build nodes recursively
	for _, task := range rootTasks {
		m.addTaskNode(task, 0, expandedState)
	}

	m.sortNodes()
	m.updateVisibility()
}

func (m *ProjectTreeModel) addTaskNode(task *models.Task, level int, expandedState map[string]bool) {
	// Use preserved state if available, otherwise default to expanded
	// This ensures newly-parented tasks (that just gained children) show their children
	expanded, hasState := expandedState[task.ID]
	if !hasState {
		expanded = true
	}

	node := &TreeNode{
		Task:     task,
		Level:    level,
		Expanded: expanded,
		Visible:  true,
	}
	m.nodes = append(m.nodes, node)

	// Add children
	for _, child := range task.Children {
		m.addTaskNode(child, level+1, expandedState)
	}
}

func (m *ProjectTreeModel) updateVisibility() {
	parentExpanded := make(map[int]bool)
	parentExpanded[-1] = true

	for i, node := range m.nodes {
		// Check if parent is expanded
		parentLevel := node.Level - 1
		visible := true

		if parentLevel >= 0 {
			// Find parent node
			for j := i - 1; j >= 0; j-- {
				if m.nodes[j].Level == parentLevel {
					visible = m.nodes[j].Expanded && m.nodes[j].Visible
					break
				}
			}
		}

		// Apply filters
		if visible && !m.showCompleted && node.Task.IsComplete() {
			visible = false
		}

		if visible && m.filterType != "" && node.Task.Type != m.filterType {
			visible = false
		}

		// Apply status filter
		if visible && m.filterStatus != "" && node.Task.Status != m.filterStatus {
			visible = false
		}

		// Apply priority filter (1 = high priority only, showing tasks with priority <= 2)
		if visible && m.filterPriority == 1 && node.Task.Priority > 2 {
			visible = false
		}

		// Apply tag filter (AND logic: task must have ALL selected tags)
		if visible && len(m.filterTags) > 0 {
			for tag := range m.filterTags {
				if !node.Task.HasTag(tag) {
					visible = false
					break
				}
			}
		}

		// Apply section filter (OR logic: task must be in ANY selected section)
		if visible && len(m.filterSections) > 0 {
			if !m.filterSections[node.Task.Section] {
				visible = false
			}
		}

		// Apply due date filter
		if visible && m.filterDueDate != DueDateAll {
			switch m.filterDueDate {
			case DueDateOverdue:
				visible = isOverdue(node.Task)
			case DueDateToday:
				visible = isDueToday(node.Task)
			case DueDateThisWeek:
				visible = isDueThisWeek(node.Task)
			case DueDateNone:
				visible = node.Task.DueDate == nil
			}
		}

		// Apply search filter
		if visible && m.SearchQuery() != "" {
			if !m.matchesSearch(node.Task) && !m.hasMatchingDescendant(node.Task) {
				visible = false
			}
		}

		node.Visible = visible
	}
}

func (m *ProjectTreeModel) matchesSearch(task *models.Task) bool {
	if m.SearchQuery() == "" {
		return true
	}
	query := strings.TrimSpace(m.SearchQuery())

	// Check title
	if fuzzy.MatchTerms(task.Title, query) {
		return true
	}
	// Check tags
	for _, tag := range task.Tags {
		if fuzzy.MatchTerms(tag, query) {
			return true
		}
	}
	// Check description
	if fuzzy.MatchTerms(task.Description, query) {
		return true
	}
	return false
}

func (m *ProjectTreeModel) hasMatchingDescendant(task *models.Task) bool {
	for _, child := range task.Children {
		if m.matchesSearch(child) || m.hasMatchingDescendant(child) {
			return true
		}
	}
	return false
}

// isOverdue returns true if the task is past its due date and not completed.
func isOverdue(task *models.Task) bool {
	if task.DueDate == nil || task.Status.IsComplete() {
		return false
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
	return dueDay.Before(today)
}

// isDueToday returns true if the task is due today.
func isDueToday(task *models.Task) bool {
	if task.DueDate == nil {
		return false
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
	return dueDay.Equal(today)
}

// isDueThisWeek returns true if the task is due within the next 7 days.
func isDueThisWeek(task *models.Task) bool {
	if task.DueDate == nil {
		return false
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekEnd := today.AddDate(0, 0, 7)
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
	return !dueDay.Before(today) && dueDay.Before(weekEnd)
}

// formatDueDate returns a styled due date string for inline display.
func formatDueDate(task *models.Task) string {
	if task.DueDate == nil || task.Status.IsComplete() {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	var dateStr string
	var style lipgloss.Style

	switch {
	case dueDay.Before(today):
		// Overdue
		dateStr = task.DueDate.Format("Jan 2")
		style = OverdueStyle
	case dueDay.Equal(today):
		// Due today
		dateStr = "Today"
		style = DueTodayStyle
	case dueDay.Equal(today.AddDate(0, 0, 1)):
		// Due tomorrow
		dateStr = "Tomorrow"
		style = DueDateStyle
	default:
		// Future date - show as "Jan 2" or "Jan 2 '06" if different year
		if task.DueDate.Year() != now.Year() {
			dateStr = task.DueDate.Format("Jan 2 '06")
		} else {
			dateStr = task.DueDate.Format("Jan 2")
		}
		style = DueDateStyle
	}

	return style.Render("📅 " + dateStr)
}

func (m *ProjectTreeModel) visibleNodes() []*TreeNode {
	var visible []*TreeNode
	for _, node := range m.nodes {
		if node.Visible {
			visible = append(visible, node)
		}
	}
	return visible
}

// getAllTags returns all unique tags from the project's tasks.
func (m *ProjectTreeModel) getAllTags() []string {
	tagSet := make(map[string]bool)
	for _, task := range m.project.Tasks {
		for _, tag := range task.Tags {
			tagSet[tag] = true
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}
