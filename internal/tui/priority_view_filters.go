package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// passesFilters checks if a task passes all current priority view filters.
func (m *PriorityViewModel) passesFilters(task *models.Task) bool {
	if !passesCommonFilters(task, &m.filter, m.project.ID, m.config) {
		return false
	}
	if m.filter.Actionable {
		if m.project.HasIncompleteDependencies(task) {
			return false
		}
		if task.Status == models.StatusBlocked {
			return false
		}
		if task.Section == models.SectionInbox || task.Section == models.SectionNext ||
			task.Section == models.SectionWaiting || task.Section == models.SectionSomeday ||
			task.Section == models.SectionReview || task.Section == models.SectionStandby {
			return false
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
	return hasActiveCommonFilters(&m.filter)
}

// ClearAllFilters resets all filters to their default state.
func (m *PriorityViewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *PriorityViewModel) renderActiveFilterIndicator() string {
	return renderCommonFilterIndicator(&m.filter)
}

// refreshAndClamp reloads data after filter changes.
func (m *PriorityViewModel) refreshAndClamp() { m.loadActionableTasks(); m.clampCursor() }

func (m *PriorityViewModel) buildTagCmd() tea.Cmd {
	var tasks []*models.Task
	for _, task := range m.project.Tasks {
		if !task.Status.IsComplete() {
			tasks = append(tasks, task)
		}
	}
	return buildTagCmdFromTasks(tasks, m.filter.Tags, ViewPriorityView, "Filter by Tags")
}

// handlePriorityFilterPrefix handles the second key after pressing F (filter prefix).
func (m *PriorityViewModel) handlePriorityFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific keys
	switch key {
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "r":
		m.filter.Actionable = !m.filter.Actionable
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
