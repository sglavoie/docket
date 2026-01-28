package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// ShowKanbanTagFilterSelectorMsg requests opening the tag filter selector for kanban.
type ShowKanbanTagFilterSelectorMsg struct{}

// KanbanTagFilterSelectorDoneMsg is sent when the kanban tag filter selector completes.
type KanbanTagFilterSelectorDoneMsg struct {
	SelectedTags []string
	Canceled     bool
}

// KanbanFilterState contains filter options for the kanban view.
// Unlike project tree, kanban doesn't need section filter (columns ARE sections)
// or completed filter (kanban only shows active tasks).
type KanbanFilterState struct {
	Type     models.TaskType   // filter by type (epic/story/task/bug/idea)
	Status   models.TaskStatus // filter by status (todo/in_progress/blocked)
	Priority int               // 0=all, 1=high only (<=2)
	Tags     map[string]bool   // filter by tag names (AND logic)
	DueDate  DueDateFilter     // filter by due date
	Search   string            // text search query
}

// KanbanFilterBarComponent encapsulates filter bar state for the kanban view.
type KanbanFilterBarComponent struct {
	enabled bool           // Toggle filter bar visibility
	counts  map[string]int // Cached task counts per filter category
}

// NewKanbanFilterBarComponent creates a new filter bar component for kanban.
func NewKanbanFilterBarComponent() *KanbanFilterBarComponent {
	return &KanbanFilterBarComponent{
		enabled: false, // Filter bar off by default for kanban
		counts:  make(map[string]int),
	}
}

// IsEnabled returns whether the filter bar is visible.
func (f *KanbanFilterBarComponent) IsEnabled() bool {
	return f.enabled
}

// SetEnabled sets the filter bar visibility.
func (f *KanbanFilterBarComponent) SetEnabled(enabled bool) {
	f.enabled = enabled
}

// Toggle toggles the filter bar visibility.
func (f *KanbanFilterBarComponent) Toggle() {
	f.enabled = !f.enabled
}

// GetCounts returns the cached filter counts.
func (f *KanbanFilterBarComponent) GetCounts() map[string]int {
	return f.counts
}

// SetCounts updates the cached filter counts.
func (f *KanbanFilterBarComponent) SetCounts(counts map[string]int) {
	f.counts = counts
}

// passesFilters checks if a task passes all current filters.
// Returns true if the task should be displayed.
func (m *KanbanViewModel) passesFilters(task *models.Task) bool {
	// Apply type filter
	if m.filter.Type != "" && task.Type != m.filter.Type {
		return false
	}

	// Apply status filter
	if m.filter.Status != "" && task.Status != m.filter.Status {
		return false
	}

	// Apply priority filter (1 = high priority only, showing tasks with priority <= 2)
	if m.filter.Priority == 1 && task.Priority > 2 {
		return false
	}

	// Apply tag filter (AND logic: task must have ALL selected tags)
	if len(m.filter.Tags) > 0 {
		for tag := range m.filter.Tags {
			if !task.HasTag(tag) {
				return false
			}
		}
	}

	// Apply due date filter
	if m.filter.DueDate != DueDateAll {
		switch m.filter.DueDate {
		case DueDateOverdue:
			if !isOverdue(task) {
				return false
			}
		case DueDateToday:
			if !isDueToday(task) {
				return false
			}
		case DueDateThisWeek:
			if !isDueThisWeek(task) {
				return false
			}
		case DueDateNone:
			if task.DueDate != nil {
				return false
			}
		}
	}

	// Apply search filter
	if m.filter.Search != "" {
		if !m.matchesSearch(task) {
			return false
		}
	}

	// Apply actionable filter (hide blocked tasks and tasks with incomplete dependencies)
	if m.actionableFilter {
		if !m.isActionable(task) {
			return false
		}
	}

	return true
}

// isActionable returns true if a task is ready to work on.
// A task is actionable if it's not blocked and has no incomplete dependencies.
func (m *KanbanViewModel) isActionable(task *models.Task) bool {
	// Blocked tasks are not actionable
	if task.Status == models.StatusBlocked {
		return false
	}

	// Tasks with incomplete dependencies are not actionable
	if m.Project.HasIncompleteDependencies(task) {
		return false
	}

	return true
}

// toggleActionableFilter toggles the actionable filter.
func (m *KanbanViewModel) toggleActionableFilter() {
	m.actionableFilter = !m.actionableFilter
	m.loadTasks()
	if m.actionableFilter {
		m.statusMessage = "Actionable filter: ON (hiding blocked/dependent)"
	} else {
		m.statusMessage = "Actionable filter: OFF"
	}
}

// matchesSearch returns true if the task matches the search query.
// Matches against title, description, and tags using fuzzy matching.
func (m *KanbanViewModel) matchesSearch(task *models.Task) bool {
	if m.filter.Search == "" {
		return true
	}
	query := strings.TrimSpace(m.filter.Search)

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

// calculateFilterCounts calculates task counts for each filter category.
// This enables the filter bar to show how many tasks match each filter.
func (m *KanbanViewModel) calculateFilterCounts() {
	if m.filterBar == nil {
		return
	}

	counts := make(map[string]int)
	for _, task := range m.Project.Tasks {
		// Only count non-completed tasks (kanban excludes completed)
		if task.Status.IsComplete() {
			continue
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
		if task.DueDate == nil {
			counts["noDueDate"]++
		}
		// Count by tag
		for _, tag := range task.Tags {
			counts["tag:"+tag]++
		}
	}

	m.filterBar.SetCounts(counts)
}

// renderKanbanFilterBar renders the filter bar for the kanban view.
func (m *KanbanViewModel) renderKanbanFilterBar() string {
	if m.filterBar == nil || !m.filterBar.IsEnabled() {
		return ""
	}

	var result strings.Builder

	// Line 1: Filter badges (F2-F5, F7 - skip F1/F6/F8 which don't apply to kanban)
	var badges []string

	// F2: High Priority
	count := m.filterBar.GetCounts()["highPriority"]
	active := m.filter.Priority == 1
	badges = append(badges, m.renderKanbanFilterBadge("F2", "!Hi", count, active))

	// F3: Type (show current type if filtered)
	typeName := "Type"
	if m.filter.Type != "" {
		typeName = string(m.filter.Type)
		count = m.filterBar.GetCounts()["type:"+string(m.filter.Type)]
	} else {
		count = 0
	}
	active = m.filter.Type != ""
	badges = append(badges, m.renderKanbanFilterBadge("F3", typeName, count, active))

	// F4: Status (show current status if filtered)
	statusName := "Status"
	if m.filter.Status != "" {
		statusName = string(m.filter.Status)
		count = m.filterBar.GetCounts()["status:"+string(m.filter.Status)]
	} else {
		count = 0
	}
	active = m.filter.Status != ""
	badges = append(badges, m.renderKanbanFilterBadge("F4", statusName, count, active))

	// F5: Tags (show count of selected tags)
	tagCount := len(m.filter.Tags)
	if tagCount > 0 {
		badges = append(badges, m.renderKanbanFilterBadge("F5", fmt.Sprintf("#%dtag", tagCount), tagCount, true))
	} else {
		badges = append(badges, m.renderKanbanFilterBadge("F5", "Tags", 0, false))
	}

	// F7: Due Date
	dueName := "Due"
	switch m.filter.DueDate {
	case DueDateOverdue:
		dueName = "Ovrd"
		count = m.filterBar.GetCounts()["overdue"]
	case DueDateToday:
		dueName = "Today"
		count = m.filterBar.GetCounts()["today"]
	case DueDateThisWeek:
		dueName = "Week"
		count = m.filterBar.GetCounts()["thisWeek"]
	case DueDateNone:
		dueName = "None"
		count = m.filterBar.GetCounts()["noDueDate"]
	default:
		count = 0
	}
	active = m.filter.DueDate != DueDateAll
	badges = append(badges, m.renderKanbanFilterBadge("F7", dueName, count, active))

	// Search indicator
	if m.filter.Search != "" {
		searchBadge := fmt.Sprintf("/%s", m.filter.Search)
		if len(searchBadge) > 15 {
			searchBadge = searchBadge[:12] + "..."
		}
		badges = append(badges, FilterBadgeStyle.Render(searchBadge))
	}

	result.WriteString(strings.Join(badges, " "))
	result.WriteString("\n")

	// Line 2: Column counts header (already shown in column headers, but summarize here)
	result.WriteString(m.renderKanbanColumnCounts())
	result.WriteString("\n")

	return result.String()
}

// renderKanbanFilterBadge renders a single filter badge.
func (m *KanbanViewModel) renderKanbanFilterBadge(key, name string, count int, active bool) string {
	var countStr string
	if count > 0 {
		countStr = fmt.Sprintf("[%d]", count)
	}

	label := fmt.Sprintf("%s:%s%s", key, name, countStr)

	if active {
		return FilterBadgeStyle.Render(label)
	}
	return FilterBadgeInactiveStyle.Render(label)
}

// renderKanbanColumnCounts renders the column task counts summary.
func (m *KanbanViewModel) renderKanbanColumnCounts() string {
	var parts []string
	for _, section := range KanbanColumnOrder {
		count := len(m.Tasks[section])
		name := sectionDisplayName(section)
		style := GetViewModeStyleForSection(section)
		parts = append(parts, style.Render(fmt.Sprintf("%s[%d]", name, count)))
	}
	return strings.Join(parts, " ")
}

// HasActiveFilters returns true if any filter is currently active.
func (m *KanbanViewModel) HasActiveFilters() bool {
	return m.filter.Type != "" ||
		m.filter.Status != "" ||
		m.filter.Priority != 0 ||
		len(m.filter.Tags) > 0 ||
		m.filter.DueDate != DueDateAll ||
		m.filter.Search != ""
}

// ClearAllFilters resets all filters to their default state.
func (m *KanbanViewModel) ClearAllFilters() {
	m.filter.Type = ""
	m.filter.Status = ""
	m.filter.Priority = 0
	m.filter.Tags = make(map[string]bool)
	m.filter.DueDate = DueDateAll
	m.filter.Search = ""
}

// getAllTags returns all unique tags from the project's tasks.
func (m *KanbanViewModel) getAllTags() []string {
	tagSet := make(map[string]bool)
	for _, task := range m.Project.Tasks {
		// Only consider non-completed tasks
		if task.Status.IsComplete() {
			continue
		}
		for _, tag := range task.Tags {
			tagSet[tag] = true
		}
	}

	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags
}

// cycleKanbanTypeFilter cycles through task types for filtering.
func (m *KanbanViewModel) cycleKanbanTypeFilter() {
	// Cycle order: "" -> epic -> story -> task -> bug -> idea -> ""
	types := []models.TaskType{
		models.TaskTypeEpic,
		models.TaskTypeStory,
		models.TaskTypeTask,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}
	m.filter.Type = cycleFilter(m.filter.Type, types)
}

// cycleKanbanStatusFilter cycles through task statuses for filtering.
func (m *KanbanViewModel) cycleKanbanStatusFilter() {
	// Cycle order: "" -> todo -> in_progress -> blocked -> ""
	statuses := []models.TaskStatus{
		models.StatusTodo,
		models.StatusInProgress,
		models.StatusBlocked,
	}
	m.filter.Status = cycleFilter(m.filter.Status, statuses)
}

// cycleKanbanDueDateFilter cycles through due date filter options.
func (m *KanbanViewModel) cycleKanbanDueDateFilter() {
	// Cycle order: all -> overdue -> today -> week -> none -> all
	switch m.filter.DueDate {
	case DueDateAll:
		m.filter.DueDate = DueDateOverdue
	case DueDateOverdue:
		m.filter.DueDate = DueDateToday
	case DueDateToday:
		m.filter.DueDate = DueDateThisWeek
	case DueDateThisWeek:
		m.filter.DueDate = DueDateNone
	case DueDateNone:
		m.filter.DueDate = DueDateAll
	}
}

// toggleKanbanPriorityFilter toggles the high priority filter.
func (m *KanbanViewModel) toggleKanbanPriorityFilter() {
	if m.filter.Priority == 1 {
		m.filter.Priority = 0
	} else {
		m.filter.Priority = 1
	}
}
