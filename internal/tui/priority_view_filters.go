package tui

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// PriorityFilterState contains filter options for the priority view.
// All common filter fields are embedded from CommonFilterState.
type PriorityFilterState struct {
	CommonFilterState // embedded common filter fields
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
	return passesCommonFilters(task, &m.filter.CommonFilterState, m.project.ID, m.config)
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
	return hasActiveCommonFilters(&m.filter.CommonFilterState) || m.actionableFilter
}

// ClearAllFilters resets all filters to their default state.
func (m *PriorityViewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter.CommonFilterState)
	m.actionableFilter = false
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *PriorityViewModel) renderActiveFilterIndicator() string {
	parts := commonFilterParts(&m.filter.CommonFilterState)
	if m.actionableFilter {
		parts = append(parts, "ready")
	}
	return renderFilterBadge(parts)
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
		tags := buildPriorityTagList(m)
		return m, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewPriorityView, Title: "Filter by Tags"}
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
	if handleFilterTypeKey(key, &m.filter.CommonFilterState) {
		m.loadActionableTasks()
	}
	return m, nil
}

// handlePriorityFilterStatusPrefix handles filter status sub-prefix.
func (m *PriorityViewModel) handlePriorityFilterStatusPrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterStatusKey(key, &m.filter.CommonFilterState) {
		m.loadActionableTasks()
	}
	return m, nil
}

// handlePriorityFilterDuePrefix handles filter due-date sub-prefix.
func (m *PriorityViewModel) handlePriorityFilterDuePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterDueDateKey(key, &m.filter.CommonFilterState) {
		m.loadActionableTasks()
	}
	return m, nil
}

// handlePriorityFilterAttrPrefix handles the attribute filter sub-prefix.
func (m *PriorityViewModel) handlePriorityFilterAttrPrefix(key string) (tea.Model, tea.Cmd) {
	newPrefix, handled := handleFilterAttrKey(key, &m.filter.CommonFilterState)
	if !handled {
		return m, nil
	}
	if newPrefix != "" {
		m.pendingPrefix = newPrefix
		return m, nil
	}
	m.loadActionableTasks()
	return m, nil
}

// handlePriorityFilterAttrSubPrefix handles attribute value sub-prefixes.
func (m *PriorityViewModel) handlePriorityFilterAttrSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	if handleFilterAttrSubKey(prefix, key, &m.filter.CommonFilterState) {
		m.loadActionableTasks()
	}
	return m, nil
}

// handlePriorityFilterScorePrefix handles the score threshold sub-prefix.
func (m *PriorityViewModel) handlePriorityFilterScorePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterScoreKey(key, &m.filter.CommonFilterState, m.config) {
		m.loadActionableTasks()
	}
	return m, nil
}

// buildPriorityTagList scans all non-completed tasks and builds a sorted list of tags with counts.
func buildPriorityTagList(priority *PriorityViewModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, task := range priority.project.Tasks {
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
