package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// ShowPriorityTagFilterSelectorMsg requests opening the tag filter selector for priority view.
type ShowPriorityTagFilterSelectorMsg struct{}

// PriorityTagFilterSelectorDoneMsg is sent when the priority tag filter selector completes.
type PriorityTagFilterSelectorDoneMsg struct {
	SelectedTags []string
	Canceled     bool
}

// PriorityFilterState contains filter options for the priority view.
type PriorityFilterState struct {
	Type     models.TaskType   // filter by type (epic/story/task/bug/idea)
	Status   models.TaskStatus // filter by status
	Priority int               // 0=all, 1=high only (<=2)
	Tags     map[string]bool   // filter by tag names (AND logic)
	DueDate  DueDateFilter     // filter by due date
	Attrs    map[string]string // type.attribute -> value
	ScoreMin float64           // minimum score threshold (0 = no filter)
}

// PriorityFilterBarComponent encapsulates filter bar state for the priority view.
type PriorityFilterBarComponent struct {
	enabled bool
	counts  map[string]int
}

// NewPriorityFilterBarComponent creates a new filter bar component for priority view.
func NewPriorityFilterBarComponent() *PriorityFilterBarComponent {
	return &PriorityFilterBarComponent{
		enabled: false,
		counts:  make(map[string]int),
	}
}

// passesFilters checks if a task passes all current priority view filters.
func (m *PriorityViewModel) passesFilters(task *models.Task) bool {
	if m.filter.Type != "" && task.Type != m.filter.Type {
		return false
	}
	if m.filter.Status != "" && task.Status != m.filter.Status {
		return false
	}
	if m.filter.Priority == 1 && task.Priority > 2 {
		return false
	}
	if len(m.filter.Tags) > 0 {
		for tag := range m.filter.Tags {
			if !task.HasTag(tag) {
				return false
			}
		}
	}
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
	if !passesAttrFilters(task, m.filter.Attrs) {
		return false
	}
	if m.filter.ScoreMin > 0 && m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
			if scoring.ComputeScore(task, profile) < m.filter.ScoreMin {
				return false
			}
		}
	}
	return true
}

// applyFiltersToGroup returns a filtered slice of priority tasks.
func (m *PriorityViewModel) applyFiltersToGroup(tasks []*PriorityTask) []*PriorityTask {
	if !m.HasActiveFilters() {
		return tasks
	}
	var result []*PriorityTask
	for _, pt := range tasks {
		if m.passesFilters(pt.Task) {
			result = append(result, pt)
		}
	}
	return result
}

// HasActiveFilters returns true if any filter is currently active.
func (m *PriorityViewModel) HasActiveFilters() bool {
	return m.filter.Type != "" ||
		m.filter.Status != "" ||
		m.filter.Priority != 0 ||
		len(m.filter.Tags) > 0 ||
		m.filter.DueDate != DueDateAll ||
		len(m.filter.Attrs) > 0 ||
		m.filter.ScoreMin > 0 ||
		m.actionableFilter
}

// ClearAllFilters resets all filters to their default state.
func (m *PriorityViewModel) ClearAllFilters() {
	m.filter.Type = ""
	m.filter.Status = ""
	m.filter.Priority = 0
	m.filter.Tags = make(map[string]bool)
	m.filter.DueDate = DueDateAll
	m.filter.Attrs = make(map[string]string)
	m.filter.ScoreMin = 0
	m.actionableFilter = false
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *PriorityViewModel) renderActiveFilterIndicator() string {
	var parts []string

	if m.filter.Priority == 1 {
		parts = append(parts, "!Hi")
	}
	if m.filter.Type != "" {
		parts = append(parts, "type:"+string(m.filter.Type))
	}
	if m.filter.Status != "" {
		parts = append(parts, "status:"+string(m.filter.Status))
	}
	if len(m.filter.Tags) > 0 {
		var tagNames []string
		for tag := range m.filter.Tags {
			tagNames = append(tagNames, "#"+tag)
		}
		parts = append(parts, strings.Join(tagNames, " "))
	}
	switch m.filter.DueDate {
	case DueDateOverdue:
		parts = append(parts, "due:overdue")
	case DueDateToday:
		parts = append(parts, "due:today")
	case DueDateThisWeek:
		parts = append(parts, "due:week")
	case DueDateNone:
		parts = append(parts, "due:none")
	}
	if m.actionableFilter {
		parts = append(parts, "ready")
	}
	if len(m.filter.Attrs) > 0 {
		var attrParts []string
		for k, v := range m.filter.Attrs {
			attrParts = append(attrParts, k+"="+v)
		}
		parts = append(parts, "attr:"+strings.Join(attrParts, ","))
	}
	if m.filter.ScoreMin > 0 {
		parts = append(parts, fmt.Sprintf("score>=%.0f", m.filter.ScoreMin))
	}

	if len(parts) == 0 {
		return ""
	}

	return FilterBadgeStyle.Render("Filters: " + strings.Join(parts, " "))
}

// buildPriorityFilterPrefixHints builds dynamic filter hints for priority view.
func (m *PriorityViewModel) buildPriorityFilterPrefixHints() []PrefixHint {
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

// handlePriorityFilterPrefix handles the second key after pressing F (filter prefix).
func (m *PriorityViewModel) handlePriorityFilterPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "p":
		if m.filter.Priority == 1 {
			m.filter.Priority = 0
		} else {
			m.filter.Priority = 1
		}
		m.loadActionableTasks()
		return m, nil
	case "t":
		m.pendingPrefix = "Ft"
		return m, nil
	case "s":
		m.pendingPrefix = "Fs"
		return m, nil
	case "#":
		return m, func() tea.Msg {
			return ShowPriorityTagFilterSelectorMsg{}
		}
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "a":
		m.pendingPrefix = "Fa"
		return m, nil
	case "r":
		m.actionableFilter = !m.actionableFilter
		m.loadActionableTasks()
		return m, nil
	case "S":
		m.pendingPrefix = "FS"
		return m, nil
	case "x":
		m.ClearAllFilters()
		m.loadActionableTasks()
		return m, nil
	}
	return m, nil
}

// handlePriorityFilterTypePrefix handles filter type sub-prefix.
func (m *PriorityViewModel) handlePriorityFilterTypePrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterStatusPrefix handles filter status sub-prefix.
func (m *PriorityViewModel) handlePriorityFilterStatusPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "t":
		m.filter.Status = models.StatusTodo
	case "s":
		m.filter.Status = models.StatusInProgress
	case "r":
		m.filter.Status = models.StatusReview
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterDuePrefix handles filter due-date sub-prefix.
func (m *PriorityViewModel) handlePriorityFilterDuePrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterAttrPrefix handles the attribute filter sub-prefix.
func (m *PriorityViewModel) handlePriorityFilterAttrPrefix(key string) (tea.Model, tea.Cmd) {
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
		m.loadActionableTasks()
		return m, nil
	case "0":
		m.filter.Attrs = make(map[string]string)
		m.loadActionableTasks()
		return m, nil
	}
	return m, nil
}

// handlePriorityFilterAttrSeverityPrefix handles bug severity value selection.
func (m *PriorityViewModel) handlePriorityFilterAttrSeverityPrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterAttrReproduciblePrefix handles bug reproducible value selection.
func (m *PriorityViewModel) handlePriorityFilterAttrReproduciblePrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterAttrEffortPrefix handles effort value selection.
func (m *PriorityViewModel) handlePriorityFilterAttrEffortPrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterAttrCategoryPrefix handles idea category value selection.
func (m *PriorityViewModel) handlePriorityFilterAttrCategoryPrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterAttrValuePrefix handles idea value selection.
func (m *PriorityViewModel) handlePriorityFilterAttrValuePrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterAttrWorktypePrefix handles task worktype value selection.
func (m *PriorityViewModel) handlePriorityFilterAttrWorktypePrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterAttrScopePrefix handles epic scope value selection.
func (m *PriorityViewModel) handlePriorityFilterAttrScopePrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterAttrPointsPrefix handles story points value selection.
func (m *PriorityViewModel) handlePriorityFilterAttrPointsPrefix(key string) (tea.Model, tea.Cmd) {
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
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterScorePrefix handles the score threshold sub-prefix.
func (m *PriorityViewModel) handlePriorityFilterScorePrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "1":
		m.filter.ScoreMin = 10
	case "2":
		m.filter.ScoreMin = 25
	case "3":
		m.filter.ScoreMin = 50
	case "4":
		m.filter.ScoreMin = 75
	case "5":
		m.filter.ScoreMin = 100
	case "6":
		m.filter.ScoreMin = 150
	case "7":
		m.filter.ScoreMin = 200
	case "8":
		m.filter.ScoreMin = 250
	case "9":
		m.filter.ScoreMin = 300
	case "c":
		// Apply persisted custom value from config
		if m.config != nil && m.config.ScoreFilterCustom != nil {
			m.filter.ScoreMin = *m.config.ScoreFilterCustom
		}
	case "0":
		m.filter.ScoreMin = 0
	default:
		return m, nil
	}
	m.loadActionableTasks()
	return m, nil
}
