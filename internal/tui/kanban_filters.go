package tui

import (
	"fmt"
	"strings"

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
	Attrs    map[string]string // type.attribute -> value (e.g., "bug.severity" -> "critical")
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
	// Apply custom section filter
	if csFilter := m.activeCustomSectionFilter(); csFilter != "" {
		if task.CustomSection != csFilter {
			return false
		}
	}

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

	// Apply attribute filters
	if !passesAttrFilters(task, m.filter.Attrs) {
		return false
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
	return m.filter.Type != "" ||
		m.filter.Status != "" ||
		m.filter.Priority != 0 ||
		len(m.filter.Tags) > 0 ||
		m.filter.DueDate != DueDateAll ||
		m.filter.Search != "" ||
		len(m.filter.Attrs) > 0
}

// ClearAllFilters resets all filters to their default state.
func (m *KanbanViewModel) ClearAllFilters() {
	m.filter.Type = ""
	m.filter.Status = ""
	m.filter.Priority = 0
	m.filter.Tags = make(map[string]bool)
	m.filter.DueDate = DueDateAll
	m.filter.Search = ""
	m.filter.Attrs = make(map[string]string)
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

	return []PrefixHint{
		{Key: "c", Description: "completed", Detail: completedDetail, Active: m.completedViewMode},
		{Key: "p", Description: "priority", Detail: priorityDetail, Active: m.filter.Priority != 0},
		{Key: "t", Description: "type", Detail: typeDetail, Active: m.filter.Type != ""},
		{Key: "s", Description: "status", Detail: statusDetail, Active: m.filter.Status != ""},
		{Key: "#", Description: "tags", Detail: tagDetail, Active: len(m.filter.Tags) > 0},
		{Key: "d", Description: "due-date", Detail: dueDetail, Active: m.filter.DueDate != DueDateAll},
		{Key: "a", Description: "attribute", Detail: attrDetail, Active: len(m.filter.Attrs) > 0},
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
		return m, func() tea.Msg {
			return ShowKanbanTagFilterSelectorMsg{}
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

// handleKanbanFilterTypePrefix handles filter type sub-prefix in kanban.
func (m *KanbanViewModel) handleKanbanFilterTypePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "e":
		m.filter.Type = models.TaskTypeEpic
	case "s":
		m.filter.Type = models.TaskTypeStory
	case "t":
		m.filter.Type = models.TaskTypeTask
	case "b":
		m.filter.Type = models.TaskTypeBug
	case "i":
		m.filter.Type = models.TaskTypeIdea
	case "0":
		m.filter.Type = ""
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterStatusPrefix handles filter status sub-prefix in kanban.
func (m *KanbanViewModel) handleKanbanFilterStatusPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "t":
		m.filter.Status = models.StatusTodo
	case "s":
		m.filter.Status = models.StatusInProgress
	case "b":
		m.filter.Status = models.StatusBlocked
	case "d":
		m.filter.Status = models.StatusDone
	case "x":
		m.filter.Status = models.StatusCanceled
	case "0":
		m.filter.Status = ""
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterDuePrefix handles filter due-date sub-prefix in kanban.
func (m *KanbanViewModel) handleKanbanFilterDuePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "o":
		m.filter.DueDate = DueDateOverdue
	case "t":
		m.filter.DueDate = DueDateToday
	case "w":
		m.filter.DueDate = DueDateThisWeek
	case "n":
		m.filter.DueDate = DueDateNone
	case "0":
		m.filter.DueDate = DueDateAll
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterAttrPrefix handles the attribute filter sub-prefix in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		m.pendingPrefix = "Fas"
		return m, nil
	case "r":
		m.pendingPrefix = "Far"
		return m, nil
	case "e":
		m.pendingPrefix = "Fae"
		return m, nil
	case "c":
		m.pendingPrefix = "Fac"
		return m, nil
	case "v":
		m.pendingPrefix = "Fav"
		return m, nil
	case "w":
		m.pendingPrefix = "Faw"
		return m, nil
	case "o":
		m.pendingPrefix = "Fao"
		return m, nil
	case "p":
		m.pendingPrefix = "Fap"
		return m, nil
	case "V":
		if m.filter.Attrs["idea.validated"] == "true" {
			delete(m.filter.Attrs, "idea.validated")
		} else {
			m.filter.Attrs["idea.validated"] = "true"
			m.filter.Type = models.TaskTypeIdea
		}
		m.loadTasks()
		return m, nil
	case "0":
		m.filter.Attrs = make(map[string]string)
		m.loadTasks()
		return m, nil
	}
	return m, nil
}

// handleKanbanFilterAttrSeverityPrefix handles bug severity value selection in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrSeverityPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "c":
		m.filter.Attrs["bug.severity"] = string(models.BugSeverityCritical)
		m.filter.Type = models.TaskTypeBug
	case "j":
		m.filter.Attrs["bug.severity"] = string(models.BugSeverityMajor)
		m.filter.Type = models.TaskTypeBug
	case "m":
		m.filter.Attrs["bug.severity"] = string(models.BugSeverityMinor)
		m.filter.Type = models.TaskTypeBug
	case "o":
		m.filter.Attrs["bug.severity"] = string(models.BugSeverityCosmetic)
		m.filter.Type = models.TaskTypeBug
	case "0":
		delete(m.filter.Attrs, "bug.severity")
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterAttrReproduciblePrefix handles bug reproducible value selection in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrReproduciblePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y":
		m.filter.Attrs["bug.reproducible"] = string(models.BugReproducibleYes)
		m.filter.Type = models.TaskTypeBug
	case "n":
		m.filter.Attrs["bug.reproducible"] = string(models.BugReproducibleNo)
		m.filter.Type = models.TaskTypeBug
	case "s":
		m.filter.Attrs["bug.reproducible"] = string(models.BugReproducibleSometimes)
		m.filter.Type = models.TaskTypeBug
	case "0":
		delete(m.filter.Attrs, "bug.reproducible")
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterAttrEffortPrefix handles effort value selection in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrEffortPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		m.filter.Attrs["effort"] = string(models.EffortSmall)
	case "m":
		m.filter.Attrs["effort"] = string(models.EffortMedium)
	case "l":
		m.filter.Attrs["effort"] = string(models.EffortLarge)
	case "u":
		m.filter.Attrs["effort"] = string(models.EffortUnknown)
	case "0":
		delete(m.filter.Attrs, "effort")
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterAttrCategoryPrefix handles idea category value selection in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrCategoryPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "f":
		m.filter.Attrs["idea.category"] = string(models.IdeaCategoryFeature)
		m.filter.Type = models.TaskTypeIdea
	case "i":
		m.filter.Attrs["idea.category"] = string(models.IdeaCategoryImprovement)
		m.filter.Type = models.TaskTypeIdea
	case "r":
		m.filter.Attrs["idea.category"] = string(models.IdeaCategoryRefactor)
		m.filter.Type = models.TaskTypeIdea
	case "s":
		m.filter.Attrs["idea.category"] = string(models.IdeaCategoryResearch)
		m.filter.Type = models.TaskTypeIdea
	case "0":
		delete(m.filter.Attrs, "idea.category")
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterAttrValuePrefix handles idea value selection in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrValuePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "l":
		m.filter.Attrs["idea.value"] = string(models.IdeaValueLow)
		m.filter.Type = models.TaskTypeIdea
	case "m":
		m.filter.Attrs["idea.value"] = string(models.IdeaValueMedium)
		m.filter.Type = models.TaskTypeIdea
	case "h":
		m.filter.Attrs["idea.value"] = string(models.IdeaValueHigh)
		m.filter.Type = models.TaskTypeIdea
	case "0":
		delete(m.filter.Attrs, "idea.value")
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterAttrWorktypePrefix handles task worktype value selection in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrWorktypePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "d":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeDevelopment)
		m.filter.Type = models.TaskTypeTask
	case "o":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeDocumentation)
		m.filter.Type = models.TaskTypeTask
	case "t":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeTesting)
		m.filter.Type = models.TaskTypeTask
	case "v":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeDevOps)
		m.filter.Type = models.TaskTypeTask
	case "e":
		m.filter.Attrs["task.worktype"] = string(models.WorkTypeDesign)
		m.filter.Type = models.TaskTypeTask
	case "0":
		delete(m.filter.Attrs, "task.worktype")
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterAttrScopePrefix handles epic scope value selection in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrScopePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "d":
		m.filter.Attrs["epic.scope"] = string(models.EpicScopeDefined)
		m.filter.Type = models.TaskTypeEpic
	case "e":
		m.filter.Attrs["epic.scope"] = string(models.EpicScopeEvolving)
		m.filter.Type = models.TaskTypeEpic
	case "u":
		m.filter.Attrs["epic.scope"] = string(models.EpicScopeUndefined)
		m.filter.Type = models.TaskTypeEpic
	case "0":
		delete(m.filter.Attrs, "epic.scope")
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}

// handleKanbanFilterAttrPointsPrefix handles story points value selection in kanban.
func (m *KanbanViewModel) handleKanbanFilterAttrPointsPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "1":
		m.filter.Attrs["story.points"] = "1"
		m.filter.Type = models.TaskTypeStory
	case "2":
		m.filter.Attrs["story.points"] = "2"
		m.filter.Type = models.TaskTypeStory
	case "3":
		m.filter.Attrs["story.points"] = "3"
		m.filter.Type = models.TaskTypeStory
	case "5":
		m.filter.Attrs["story.points"] = "5"
		m.filter.Type = models.TaskTypeStory
	case "8":
		m.filter.Attrs["story.points"] = "8"
		m.filter.Type = models.TaskTypeStory
	case "0":
		delete(m.filter.Attrs, "story.points")
	default:
		return m, nil
	}
	m.loadTasks()
	return m, nil
}
