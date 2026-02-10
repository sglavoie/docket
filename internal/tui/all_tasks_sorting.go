package tui

import (
	"fmt"
	"sort"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// applySorting sorts the task list based on the current sort field.
func (m *AllTasksViewModel) applySorting(tasks []AllTasksItem) {
	if m.sortField == SortByDefault {
		return
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		less := m.compareTasks(tasks[i], tasks[j])
		if !m.sortAscending {
			return !less
		}
		return less
	})
}

// compareTasks compares two tasks based on the current sort field.
func (m *AllTasksViewModel) compareTasks(a, b AllTasksItem) bool {
	switch m.sortField {
	case SortByPriority:
		return a.Task.Priority < b.Task.Priority

	case SortByDueDate:
		if a.Task.DueDate == nil && b.Task.DueDate == nil {
			return false
		}
		if a.Task.DueDate == nil {
			return false
		}
		if b.Task.DueDate == nil {
			return true
		}
		return a.Task.DueDate.Before(*b.Task.DueDate)

	case SortByCreatedAt:
		return a.Task.CreatedAt.Before(b.Task.CreatedAt)

	case SortByStatus:
		return allTasksStatusOrder(a.Task.Status) < allTasksStatusOrder(b.Task.Status)

	case SortByScore:
		pa := m.config.GetProjectScoringProfile(a.Project.ID)
		pb := m.config.GetProjectScoringProfile(b.Project.ID)
		var sa, sb float64
		if pa != nil {
			sa = scoring.ComputeScore(a.Task, pa)
		}
		if pb != nil {
			sb = scoring.ComputeScore(b.Task, pb)
		}
		return sa > sb // Higher score first
	}

	return false
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

// allTasksStatusOrder returns the sort order for a status (workflow order).
func allTasksStatusOrder(status models.TaskStatus) int {
	switch status {
	case models.StatusTodo:
		return 0
	case models.StatusInProgress:
		return 1
	case models.StatusReview:
		return 2
	case models.StatusBlocked:
		return 3
	case models.StatusDone:
		return 4
	case models.StatusCanceled:
		return 5
	default:
		return 6
	}
}
