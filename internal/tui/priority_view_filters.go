package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

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
	return passesCommonFilters(task, &m.filter, m.project.ID, m.config)
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
	return hasActiveCommonFilters(&m.filter) || m.actionableFilter
}

// ClearAllFilters resets all filters to their default state.
func (m *PriorityViewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
	m.actionableFilter = false
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *PriorityViewModel) renderActiveFilterIndicator() string {
	parts := commonFilterParts(&m.filter)
	if m.actionableFilter {
		parts = append(parts, "ready")
	}
	return renderFilterBadge(parts)
}

// buildPriorityFilterPrefixHints builds dynamic filter hints for priority view.
func (m *PriorityViewModel) buildPriorityFilterPrefixHints() []PrefixHint {
	hints := buildCommonFilterPrefixHints(&m.filter, true, true)

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

// refreshAndClamp reloads data after filter changes.
func (m *PriorityViewModel) refreshAndClamp() { m.loadActionableTasks() }

func (m *PriorityViewModel) buildTagCmd() tea.Cmd {
	var tasks []*models.Task
	for _, task := range m.project.Tasks {
		if !task.Status.IsComplete() {
			tasks = append(tasks, task)
		}
	}
	tags := buildTagListFromTasks(tasks)
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
