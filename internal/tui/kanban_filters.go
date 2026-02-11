package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/fuzzy"
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
	if m.actionableFilter {
		if !m.isActionable(task) {
			return false
		}
	}

	// Apply "Due Soon" tab filter (overdue + today + this week)
	if m.activeTab == kanbanTabDueSoon {
		if !isDueSoon(task) {
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

// HasActiveFilters returns true if any filter is currently active.
func (m *KanbanViewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter.CommonFilterState) || m.filter.Search != ""
}

// ClearAllFilters resets all filters to their default state.
func (m *KanbanViewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter.CommonFilterState)
	m.filter.Search = ""
}

// toggleKanbanPriorityFilter toggles the high priority filter.
func (m *KanbanViewModel) toggleKanbanPriorityFilter() {
	if m.filter.Priority == 1 {
		m.filter.Priority = 0
	} else {
		m.filter.Priority = 1
	}
}

// applyTabPreset resets manual filters and applies the preset for the given tab.
func (m *KanbanViewModel) applyTabPreset(tab int) {
	m.activeTab = tab
	// Reset manual filters
	m.ClearAllFilters()
	m.actionableFilter = false

	switch tab {
	case kanbanTabActionable:
		m.actionableFilter = true
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

// isDueSoon returns true if a task is overdue, due today, or due this week.
func isDueSoon(task *models.Task) bool {
	return isOverdue(task) || isDueToday(task) || isDueThisWeek(task)
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
	if m.actionableFilter {
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
		{Key: "r", Description: "ready", Detail: readyDetail, Active: m.actionableFilter},
		{Key: "x", Description: "clear-all"},
	}
}

// handleKanbanFilterPrefix handles the second key after pressing F (filter prefix) in kanban.
func (m *KanbanViewModel) handleKanbanFilterPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "c":
		// Toggle completed view
		m.toggleCompletedView()
		return m, nil
	case "p":
		// Toggle high priority filter
		m.toggleKanbanPriorityFilter()
		m.loadTasks()
		return m, nil
	case "t":
		// Open type sub-popup
		m.pendingPrefix = "Ft"
		return m, nil
	case "s":
		// Open status sub-popup
		m.pendingPrefix = "Fs"
		return m, nil
	case "#":
		// Tag filter selector
		tags := buildKanbanTagList(m)
		return m, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewKanban, Title: "Filter by Tags"}
		}
	case "d":
		// Open due-date sub-popup
		m.pendingPrefix = "Fd"
		return m, nil
	case "a":
		// Open attribute sub-popup
		m.pendingPrefix = "Fa"
		return m, nil
	case "r":
		// Toggle actionable filter
		m.toggleActionableFilter()
		return m, nil
	case "S":
		// Open score threshold sub-popup
		m.pendingPrefix = "FS"
		return m, nil
	case "x":
		// Clear all filters
		m.ClearAllFilters()
		m.actionableFilter = false
		m.searchInput.SetValue("")
		m.loadTasks()
		return m, nil
	}
	return m, nil
}

// handleKanbanFilterScorePrefix handles the third key after pressing FS (filter score sub-prefix) in kanban.
func (m *KanbanViewModel) handleKanbanFilterScorePrefix(key string) (tea.Model, tea.Cmd) {
	if key == "C" {
		m.enterScoreFilterInputMode()
		return m, nil
	}
	if handleFilterScoreKey(key, &m.filter.CommonFilterState, m.config) {
		m.loadTasks()
	}
	return m, nil
}

// enterScoreFilterInputMode initializes the score filter text input for kanban.
func (m *KanbanViewModel) enterScoreFilterInputMode() {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "enter value"
	ti.CharLimit = 10
	ti.Width = 12
	if m.config != nil && m.config.ScoreFilterCustom != nil {
		ti.SetValue(fmt.Sprintf("%.0f", *m.config.ScoreFilterCustom))
	}
	ti.Focus()
	m.scoreFilterInput = ti
	m.scoreFilterInputMode = true
}

// handleKanbanScoreFilterInput handles key events while the score filter text input is active in kanban.
func (m *KanbanViewModel) handleKanbanScoreFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val, err := strconv.ParseFloat(m.scoreFilterInput.Value(), 64)
		if err == nil && val > 0 {
			m.config.ScoreFilterCustom = &val
			if m.configManager != nil {
				_ = m.configManager.Save(m.config)
			}
			m.filter.ScoreMin = val
			m.loadTasks()
		}
		m.scoreFilterInputMode = false
		return m, nil
	case "esc":
		m.scoreFilterInputMode = false
		return m, nil
	}
	var cmd tea.Cmd
	m.scoreFilterInput, cmd = m.scoreFilterInput.Update(msg)
	return m, cmd
}

// handleKanbanFilterTypePrefix handles filter type sub-prefix in kanban.
func (m *KanbanViewModel) handleKanbanFilterTypePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterTypeKey(key, &m.filter.CommonFilterState) {
		m.loadTasks()
	}
	return m, nil
}

// handleKanbanFilterStatusPrefix handles filter status sub-prefix in kanban.
func (m *KanbanViewModel) handleKanbanFilterStatusPrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterStatusKey(key, &m.filter.CommonFilterState) {
		m.loadTasks()
	}
	return m, nil
}

// handleKanbanFilterDuePrefix handles filter due-date sub-prefix in kanban.
func (m *KanbanViewModel) handleKanbanFilterDuePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterDueDateKey(key, &m.filter.CommonFilterState) {
		m.loadTasks()
	}
	return m, nil
}

// handleKanbanFilterAttrPrefix handles the attribute filter sub-prefix in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrPrefix(key string) (tea.Model, tea.Cmd) {
	newPrefix, handled := handleFilterAttrKey(key, &m.filter.CommonFilterState)
	if !handled {
		return m, nil
	}
	if newPrefix != "" {
		m.pendingPrefix = newPrefix
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterAttrSubPrefix handles attribute value sub-prefixes in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	if handleFilterAttrSubKey(prefix, key, &m.filter.CommonFilterState) {
		m.loadTasks()
	}
	return m, nil
}

// buildKanbanTagList scans all non-completed tasks and builds a sorted list of tags with counts.
func buildKanbanTagList(kanban *KanbanViewModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, task := range kanban.Project.Tasks {
		if task.Status.IsComplete() {
			continue
		}
		for _, tag := range task.Tags {
			tagCounts[tag]++
		}
	}

	tags := make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		tags = append(tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}
