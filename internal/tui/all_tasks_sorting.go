package tui

import (
	"fmt"
	"sort"
)

// applySorting sorts the task list based on the current sort field.
func (m *AllTasksViewModel) applySorting(tasks []AllTasksItem) {
	if m.sortField == SortByDefault {
		return
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		cmp := CompareTasksCrossProject(
			tasks[i].Task, tasks[j].Task, m.sortField,
			tasks[i].Project.ID, tasks[j].Project.ID, m.config,
		)
		return ApplySortDirection(cmp, m.sortAscending)
	})
}

// cycleSortField toggles sort direction if same field, or switches field if different.
func (m *AllTasksViewModel) cycleSortField(newField SortField) {
	if m.sortField == newField {
		m.sortAscending = !m.sortAscending
	} else {
		m.sortField = newField
		m.sortAscending = true
	}
	m.applyFiltersAndGroup()
	m.clampCursor()
	m.statusMessage = fmt.Sprintf("Sort: %s %s", newField, m.sortDirection())
}

// sortDirection returns a display string for the current sort direction.
func (m *AllTasksViewModel) sortDirection() string {
	if m.sortAscending {
		return "↑"
	}
	return "↓"
}
