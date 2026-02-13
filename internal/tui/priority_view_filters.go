package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
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
	hints := buildCommonFilterPrefixHints(&m.filter.CommonFilterState, true, true)

	readyDetail := "off"
	if m.actionableFilter {
		readyDetail = "on"
	}

	// Insert "r" (ready) hint before the last "x" (clear-all) hint
	readyHint := PrefixHint{Key: "r", Description: "ready", Detail: readyDetail, Active: m.actionableFilter}
	if len(hints) > 0 {
		hints = append(hints[:len(hints)-1], readyHint, hints[len(hints)-1])
	} else {
		hints = append(hints, readyHint)
	}

	return hints
}

// commonFilterView implementation for PriorityViewModel.

func (m *PriorityViewModel) getFilter() *CommonFilterState   { return &m.filter.CommonFilterState }
func (m *PriorityViewModel) getConfig() *config.Config       { return m.config }
func (m *PriorityViewModel) setFilterPendingPrefix(p string) { m.pendingPrefix = p }
func (m *PriorityViewModel) refreshAndClamp()                { m.loadActionableTasks() }
func (m *PriorityViewModel) buildTagCmd() tea.Cmd {
	tags := buildPriorityTagList(m)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewPriorityView, Title: "Filter by Tags"}
	}
}

// handlePriorityFilterPrefix handles the second key after pressing F (filter prefix).
func (m *PriorityViewModel) handlePriorityFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific keys
	switch key {
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "r":
		m.actionableFilter = !m.actionableFilter
		m.loadActionableTasks()
		return m, nil
	case "x":
		m.ClearAllFilters()
		m.loadActionableTasks()
		return m, nil
	}

	// Common filter prefix keys
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handlePriorityFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, Fas-Fap, FS).
func (m *PriorityViewModel) handlePriorityFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
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
