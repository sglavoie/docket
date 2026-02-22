package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// Kanban tab names and indices.
var kanbanTabNames = []string{"All", "Actionable", "Due Soon", "High Pri", "Epics", "Stories", "Tasks", "Bugs"}

const (
	kanbanTabAll        = 0
	kanbanTabActionable = 1
	kanbanTabDueSoon    = 2
	kanbanTabHighPri    = 3
	kanbanTabEpics      = 4
	kanbanTabStories    = 5
	kanbanTabTasks      = 6
	kanbanTabBugs       = 7
)

// KanbanFilterState contains filter options for the kanban view.
// Unlike project tree, kanban doesn't need section filter (columns ARE sections)
// or completed filter (kanban only shows active tasks).
type KanbanFilterState struct {
	CommonFilterState        // embedded common filter fields
	Search            string // text search query
}

// NewKanbanFilterState returns an initialized KanbanFilterState with allocated maps.
func NewKanbanFilterState() KanbanFilterState {
	return KanbanFilterState{CommonFilterState: newFilterState()}
}

// NewKanbanFilterBarComponent creates a new filter bar component for kanban.
func NewKanbanFilterBarComponent() *FilterBarComponent {
	fb := NewFilterBarComponent()
	fb.SetEnabled(false) // Filter bar off by default for kanban
	return fb
}

// passesFilters checks if a task passes all current filters.
// Returns true if the task should be displayed.
func (m *KanbanViewModel) passesFilters(task *models.Task) bool {
	// Apply custom section filter (only in section mode)
	if IsSectionGroupBy(m.groupBy) {
		if csFilter := m.activeCustomSectionFilter(); csFilter != "" {
			if task.CustomSection != csFilter {
				return false
			}
		}
	}

	// Apply common filters (type, status, priority, tags, due date, attrs, score)
	if !passesCommonFilters(task, &m.filter.CommonFilterState, m.Project.ID, m.config) {
		return false
	}

	// Apply search filter
	if m.filter.Search != "" {
		if !m.matchesSearch(task) {
			return false
		}
	}

	// Apply actionable filter (hide blocked tasks and tasks with incomplete dependencies)
	if m.filter.Actionable {
		if !m.isActionable(task) {
			return false
		}
	}

	// Apply "Due Soon" tab filter (overdue + today + this week)
	if m.activeTab == kanbanTabDueSoon {
		if !task.IsOverdue() && !task.IsDueThisWeek() {
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
	m.filter.Actionable = !m.filter.Actionable
	m.loadTasks()
	if m.filter.Actionable {
		m.statusMessage = "Actionable filter: ON (hiding blocked/dependent)"
	} else {
		m.statusMessage = "Actionable filter: OFF"
	}
}

// matchesSearch returns true if the task matches the search query.
func (m *KanbanViewModel) matchesSearch(task *models.Task) bool {
	return TaskMatchesSearch(task, m.filter.Search)
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
		if task.IsOverdue() {
			counts["overdue"]++
		}
		if task.IsDueToday() {
			counts["today"]++
		}
		if task.IsDueThisWeek() {
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

// HasActiveFilters returns true if any filter is currently active.
func (m *KanbanViewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter.CommonFilterState) || m.filter.Search != ""
}

// ClearAllFilters resets all filters to their default state.
func (m *KanbanViewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter.CommonFilterState)
	m.filter.Search = ""
}

// applyTabPreset resets manual filters and applies the preset for the given tab.
func (m *KanbanViewModel) applyTabPreset(tab int) {
	m.activeTab = tab
	// Reset manual filters
	m.ClearAllFilters()

	switch tab {
	case kanbanTabActionable:
		m.filter.Actionable = true
	case kanbanTabDueSoon:
		// Handled in passesFilters via dueSoonFilter flag
		// No standard filter field; we use a special flag
	case kanbanTabHighPri:
		m.filter.Priority = 1
	case kanbanTabEpics:
		m.filter.Type = models.TaskTypeEpic
	case kanbanTabStories:
		m.filter.Type = models.TaskTypeStory
	case kanbanTabTasks:
		m.filter.Type = models.TaskTypeTask
	case kanbanTabBugs:
		m.filter.Type = models.TaskTypeBug
	}

	m.loadTasks()
}

// buildKanbanFilterPrefixHints builds dynamic filter hints for kanban view.
func (m *KanbanViewModel) buildKanbanFilterPrefixHints() []PrefixHint {
	completedDetail := "off"
	if m.completedViewMode {
		completedDetail = "on"
	}

	priorityDetail := "(all)"
	if m.filter.Priority == 1 {
		priorityDetail = "!Hi"
	}

	typeDetail := "(all)"
	if m.filter.Type != "" {
		typeDetail = string(m.filter.Type)
	}

	statusDetail := "(all)"
	if m.filter.Status != "" {
		statusDetail = string(m.filter.Status)
	}

	tagDetail := "(none)"
	if len(m.filter.Tags) > 0 {
		tagDetail = fmt.Sprintf("%d tags", len(m.filter.Tags))
	}

	dueDetail := "(all)"
	switch m.filter.DueDate {
	case DueDateOverdue:
		dueDetail = "overdue"
	case DueDateToday:
		dueDetail = "today"
	case DueDateThisWeek:
		dueDetail = "week"
	case DueDateNone:
		dueDetail = "no-date"
	}

	readyDetail := "off"
	if m.filter.Actionable {
		readyDetail = "on"
	}

	attrDetail := "(none)"
	if len(m.filter.Attrs) > 0 {
		attrDetail = fmt.Sprintf("%d attrs", len(m.filter.Attrs))
	}

	scoreDetail := "(off)"
	if m.filter.ScoreMin > 0 {
		scoreDetail = fmt.Sprintf(">=%.0f", m.filter.ScoreMin)
	}

	return []PrefixHint{
		{Key: "c", Description: "completed", Detail: completedDetail, Active: m.completedViewMode},
		{Key: "p", Description: "priority", Detail: priorityDetail, Active: m.filter.Priority != 0},
		{Key: "t", Description: "type", Detail: typeDetail, Active: m.filter.Type != ""},
		{Key: "s", Description: "status", Detail: statusDetail, Active: m.filter.Status != ""},
		{Key: "#", Description: "tags", Detail: tagDetail, Active: len(m.filter.Tags) > 0},
		{Key: "d", Description: "due-date", Detail: dueDetail, Active: m.filter.DueDate != DueDateAll},
		{Key: "a", Description: "attribute", Detail: attrDetail, Active: len(m.filter.Attrs) > 0},
		{Key: "S", Description: "score", Detail: scoreDetail, Active: m.filter.ScoreMin > 0},
		{Key: "r", Description: "ready", Detail: readyDetail, Active: m.filter.Actionable},
		{Key: "x", Description: "clear-all"},
	}
}

// handleKanbanFilterPrefix handles the second key after pressing F (filter prefix) in kanban.
func (m *KanbanViewModel) handleKanbanFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific keys
	switch key {
	case "c":
		m.toggleCompletedView()
		return m, nil
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "r":
		m.toggleActionableFilter()
		return m, nil
	case "x":
		// Override common x to also clear view-specific state
		m.ClearAllFilters()
		m.ClearSearch()
		m.loadTasks()
		return m, nil
	}

	// Common filter prefix keys
	result := handleCommonFilterPrefix(key, &m.filter.CommonFilterState, func() tea.Cmd {
		tags := BuildTagList(m.Project.Tasks, true)
		return func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewKanban, Title: "Filter by Tags"}
		}
	})
	if cmd := applyCommonFilterResult(result, func(p string) { m.pendingPrefix = p }, m.loadTasks); cmd != nil {
		return m, cmd
	}
	return m, nil
}

// handleKanbanFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, Fas-Fap, FS).
func (m *KanbanViewModel) handleKanbanFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	applyFilterSubPrefix(prefix, key, &m.filter.CommonFilterState, m.config,
		func(p string) { m.pendingPrefix = p }, m.scoreFilterInput.Enter, m.loadTasks)
	return m, nil
}
