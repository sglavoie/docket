package tui

import (
	"sort"
)

// applySorting sorts the task list based on the current sort field.
func (m *AllTasksViewModel) applySorting(tasks []AllTasksItem) {
	if m.sort.Field == SortByDefault {
		return
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		cmp := CompareTasksCrossProject(
			tasks[i].Task, tasks[j].Task, m.sort.Field,
			tasks[i].Project.ID, tasks[j].Project.ID, m.config,
		)
		return ApplySortDirection(cmp, m.sort.Ascending)
	})
}

// sortDirection returns a display string for the current sort direction.
func (m *AllTasksViewModel) sortDirection() string {
	if m.sort.Ascending {
		return "↑"
	}
	return "↓"
}
