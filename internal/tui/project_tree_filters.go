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

	// Get root tasks - either the focused task or actual root tasks
	var rootTasks []*models.Task
	if m.focusedTask != nil {
		// In focus mode, the focused task is the only "root"
		rootTasks = []*models.Task{m.focusedTask}
	} else {
		rootTasks = m.project.GetRootTasks()
	}

	// Build nodes recursively
	for _, task := range rootTasks {
		m.addTaskNode(task, 0, expandedState)
	}

	m.sortNodes()
	m.updateVisibility()
	m.calculateFilterCounts()
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
	// Reset context node flags
	for _, node := range m.nodes {
		node.IsContextNode = false
	}

	// Pass 1: Determine intrinsic visibility (ignoring parent state)
	// This determines which nodes would be visible based on their own properties
	intrinsicVisible := make(map[string]bool)
	for _, node := range m.nodes {
		intrinsicVisible[node.Task.ID] = m.passesFilters(node)
	}

	// Pass 2: Mark ancestors of intrinsically visible nodes as context nodes
	// This ensures parent hierarchy is shown even when parent is in different section
	for _, node := range m.nodes {
		if intrinsicVisible[node.Task.ID] {
			m.markAncestorsAsContext(node, intrinsicVisible)
		}
	}

	// Pass 2.5: Compute HasFilteredChildren for each node
	// A node has filtered children if any child is intrinsically visible or is a context node
	for _, node := range m.nodes {
		node.HasFilteredChildren = false
		for _, child := range node.Task.Children {
			if childNode := m.findNodeByTaskID(child.ID); childNode != nil {
				if intrinsicVisible[child.ID] || childNode.IsContextNode {
					node.HasFilteredChildren = true
					break
				}
			}
		}
	}

	// Pass 3: Apply final visibility respecting expand/collapse state
	for i, node := range m.nodes {
		parentLevel := node.Level - 1
		visible := intrinsicVisible[node.Task.ID] || node.IsContextNode

		// If we're visible, check if parent is expanded (unless parent is a context node)
		if visible && parentLevel >= 0 {
			for j := i - 1; j >= 0; j-- {
				if m.nodes[j].Level == parentLevel {
					// Parent must be visible and expanded for child to be visible
					if !m.nodes[j].Visible || !m.nodes[j].Expanded {
						visible = false
					}
					break
				}
			}
		}
		node.Visible = visible
	}
}

// passesFilters checks if a node passes all current filters (view mode, status, etc.)
// without considering parent visibility. This determines "intrinsic" visibility.
func (m *ProjectTreeModel) passesFilters(node *TreeNode) bool {
	// Handle completion status based on view mode
	if m.viewMode == ViewModeCompleted {
		// In Completed view, ONLY show completed tasks
		if !node.Task.IsComplete() {
			return false
		}
		// Apply section filter for sub-filtering by original section
		if len(m.filter.Sections) > 0 {
			if !m.filter.Sections[node.Task.Section] {
				return false
			}
		}
	} else {
		// In all other views, HIDE completed tasks
		if node.Task.IsComplete() {
			return false
		}
		// Apply view mode section logic
		switch m.viewMode {
		case ViewModeActive:
			// Hide Inbox, Next, Waiting, and Someday tasks
			if node.Task.Section == models.SectionInbox || node.Task.Section == models.SectionNext || node.Task.Section == models.SectionWaiting || node.Task.Section == models.SectionSomeday {
				return false
			}
		case ViewModeNext:
			// Only show Next tasks
			if node.Task.Section != models.SectionNext {
				return false
			}
		case ViewModeInbox:
			// Only show Inbox tasks
			if node.Task.Section != models.SectionInbox {
				return false
			}
		case ViewModeWaiting:
			// Only show Waiting tasks
			if node.Task.Section != models.SectionWaiting {
				return false
			}
		case ViewModeSomeday:
			// Only show Someday tasks
			if node.Task.Section != models.SectionSomeday {
				return false
			}
		}
	}

	// Apply type filter
	if m.filter.Type != "" && node.Task.Type != m.filter.Type {
		return false
	}

	// Apply status filter
	if m.filter.Status != "" && node.Task.Status != m.filter.Status {
		return false
	}

	// Apply priority filter (1 = high priority only, showing tasks with priority <= 2)
	if m.filter.Priority == 1 && node.Task.Priority > 2 {
		return false
	}

	// Apply tag filter (AND logic: task must have ALL selected tags)
	if len(m.filter.Tags) > 0 {
		for tag := range m.filter.Tags {
			if !node.Task.HasTag(tag) {
				return false
			}
		}
	}

	// Apply section filter (OR logic: task must be in ANY selected section)
	// In Completed view, section filter is already applied above for sub-filtering
	// In other views, view mode already controls section visibility
	if m.viewMode != ViewModeCompleted && len(m.filter.Sections) > 0 {
		if !m.filter.Sections[node.Task.Section] {
			return false
		}
	}

	// Apply due date filter
	if m.filter.DueDate != DueDateAll {
		switch m.filter.DueDate {
		case DueDateOverdue:
			if !isOverdue(node.Task) {
				return false
			}
		case DueDateToday:
			if !isDueToday(node.Task) {
				return false
			}
		case DueDateThisWeek:
			if !isDueThisWeek(node.Task) {
				return false
			}
		case DueDateNone:
			if node.Task.DueDate != nil {
				return false
			}
		}
	}

	// Apply actionable filter (hide tasks not ready to work on)
	if m.filter.Actionable {
		// Hide tasks with incomplete dependencies
		if m.project.HasIncompleteDependencies(node.Task) {
			return false
		}
		// Hide manually-blocked tasks
		if node.Task.Status == models.StatusBlocked {
			return false
		}
		// Hide Inbox/Next/Waiting/Someday tasks (GTD-aligned - not immediately actionable)
		if node.Task.Section == models.SectionInbox || node.Task.Section == models.SectionNext || node.Task.Section == models.SectionWaiting || node.Task.Section == models.SectionSomeday {
			return false
		}
	}

	// Apply search filter
	if m.SearchQuery() != "" {
		if !m.matchesSearch(node.Task) && !m.hasMatchingDescendant(node.Task) {
			return false
		}
	}

	return true
}

// markAncestorsAsContext walks up the ancestor chain and marks invisible ancestors
// as context nodes so they appear (dimmed) to preserve hierarchy visibility.
func (m *ProjectTreeModel) markAncestorsAsContext(node *TreeNode, intrinsicVisible map[string]bool) {
	task := node.Task
	if task.Parent == nil {
		return
	}

	// Find parent node
	for _, n := range m.nodes {
		if n.Task.ID == *task.ParentID {
			// Only mark as context if parent isn't intrinsically visible
			if !intrinsicVisible[n.Task.ID] && !n.IsContextNode {
				n.IsContextNode = true
			}
			// Recursively mark grandparents
			m.markAncestorsAsContext(n, intrinsicVisible)
			break
		}
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

// getSearchMatchPositions returns match positions for the title if there's a search query.
// This is called during rendering to get positions for highlighting.
func (m *ProjectTreeModel) getSearchMatchPositions(task *models.Task) []int {
	if m.SearchQuery() == "" {
		return nil
	}
	query := strings.TrimSpace(m.SearchQuery())
	_, positions := fuzzy.ScoreTermsWithPositions(task.Title, query)
	return positions
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

// getVisibleSiblings returns siblings of the given task that are currently visible.
// This filters siblings using the same visibility logic as the tree display,
// ensuring movement bounds checking matches what the user sees.
func (m *ProjectTreeModel) getVisibleSiblings(task *models.Task) []*models.Task {
	allSiblings := m.project.GetSiblings(task)
	var visibleSiblings []*models.Task
	for _, sibling := range allSiblings {
		for _, node := range m.nodes {
			if node.Task.ID == sibling.ID && m.passesFilters(node) {
				visibleSiblings = append(visibleSiblings, sibling)
				break
			}
		}
	}
	return visibleSiblings
}

// findVisibleSiblingIndex returns the index of task among its visible siblings, or -1 if not found.
func (m *ProjectTreeModel) findVisibleSiblingIndex(task *models.Task, visibleSiblings []*models.Task) int {
	for i, s := range visibleSiblings {
		if s.ID == task.ID {
			return i
		}
	}
	return -1
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

// cycleFilter cycles through filter values, returning the next value in the sequence.
// When current is the zero value, returns the first value. When current is the last value,
// returns the zero value. If current is not found, returns the zero value.
func cycleFilter[T comparable](current T, values []T) T {
	var zero T
	if current == zero {
		if len(values) > 0 {
			return values[0]
		}
		return zero
	}
	for i, v := range values {
		if v == current {
			if i+1 < len(values) {
				return values[i+1]
			}
			return zero
		}
	}
	return zero
}

// calculateFilterCounts calculates task counts for each filter category.
// This enables the filter bar to show how many tasks match each filter.
func (m *ProjectTreeModel) calculateFilterCounts() {
	counts := make(map[string]int)
	for _, task := range m.project.Tasks {
		// Count completed tasks
		if task.IsComplete() {
			counts["completed"]++
		}
		// Count high priority tasks (priority 1-2)
		if task.Priority <= 2 {
			counts["highPriority"]++
		}
		// Count by type
		counts["type:"+string(task.Type)]++
		// Count by status
		counts["status:"+string(task.Status)]++
		// Count by due date category
		if isOverdue(task) {
			counts["overdue"]++
		}
		if isDueToday(task) {
			counts["today"]++
		}
		if isDueThisWeek(task) {
			counts["thisWeek"]++
		}
		// Count by section
		if task.Section != "" {
			counts["section:"+task.Section]++
		}
		// Count by tag
		for _, tag := range task.Tags {
			counts["tag:"+tag]++
		}
		// Count actionable tasks (ready to work on)
		if !task.IsComplete() &&
			!m.project.HasIncompleteDependencies(task) &&
			task.Status != models.StatusBlocked &&
			task.Section != models.SectionInbox &&
			task.Section != models.SectionNext &&
			task.Section != models.SectionWaiting &&
			task.Section != models.SectionSomeday {
			counts["actionable"]++
		}
	}

	// Only set counts if filterBar component is initialized
	if m.filterBar != nil {
		m.filterBar.SetCounts(counts)

		// Calculate children in other sections for each task with children
		childrenInOtherSections := make(map[string]map[string]int)
		for _, task := range m.project.Tasks {
			if len(task.Children) > 0 {
				childrenInOtherSections[task.ID] = m.countChildrenBySection(task)
			}
		}
		m.filterBar.SetChildrenInOtherSections(childrenInOtherSections)
	}
}

// countChildrenBySection recursively counts all descendants by their section.
// Returns a map of section name -> count. Empty section is normalized to "Active".
func (m *ProjectTreeModel) countChildrenBySection(task *models.Task) map[string]int {
	counts := make(map[string]int)
	for _, child := range task.Children {
		if child.IsNote() {
			continue
		}
		section := child.Section
		if section == "" {
			section = "Active"
		}
		counts[section]++
		// Recursively count descendants
		for s, c := range m.countChildrenBySection(child) {
			counts[s] += c
		}
	}
	return counts
}

// getHiddenSectionsForViewMode returns the sections that are hidden by the current view mode.
// Returns nil when all sections are visible (no filtering).
func (m *ProjectTreeModel) getHiddenSectionsForViewMode() []string {
	switch m.viewMode {
	case ViewModeActive:
		return []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, models.SectionSomeday}
	case ViewModeNext:
		return []string{models.SectionInbox, models.SectionWaiting, models.SectionSomeday, "Active"}
	case ViewModeInbox:
		return []string{models.SectionNext, models.SectionWaiting, models.SectionSomeday, "Active"}
	case ViewModeWaiting:
		return []string{models.SectionInbox, models.SectionNext, models.SectionSomeday, "Active"}
	case ViewModeSomeday:
		return []string{models.SectionInbox, models.SectionNext, models.SectionWaiting, "Active"}
	}
	return nil
}

// findNodeByTaskID returns the TreeNode for the given task ID, or nil if not found.
func (m *ProjectTreeModel) findNodeByTaskID(id string) *TreeNode {
	for _, node := range m.nodes {
		if node.Task.ID == id {
			return node
		}
	}
	return nil
}
