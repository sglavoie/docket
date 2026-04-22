package tui

import "github.com/sglavoie/tt/internal/models"

// jumpToTask moves the cursor to the position of the given task in the list.
// Uses the same filtered/search-narrowed groups as getTaskAtCursor so cursor
// positions are consistent.
func (m *PriorityViewModel) jumpToTask(task *models.Task) {
	if m.tabs.Active == priorityTabByScore {
		for i, pt := range m.visibleScoredTasks() {
			if pt.Task.ID == task.ID {
				m.cursor = i
				return
			}
		}
		return
	}

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	// For the Blocked tab, navigate waiting tasks directly
	if m.tabs.Active == priorityTabBlocked {
		for i, pt := range waitingOn {
			if pt.Task.ID == task.ID {
				m.cursor = i
				return
			}
		}
		return
	}

	index := 0
	for _, group := range [][]*PriorityTask{overdue, today, thisWeek, later, backlog} {
		for _, pt := range group {
			if pt.Task.ID == task.ID {
				m.cursor = index
				return
			}
			index++
		}
	}
}

// getParentTitle returns the parent task's title for context.
func (m *PriorityViewModel) getParentTitle(task *models.Task) string {
	if task.ParentID == nil {
		return ""
	}
	if parent := m.project.GetTaskByID(*task.ParentID); parent != nil {
		return parent.Title
	}
	return ""
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *PriorityViewModel) getTaskAtCursor() *PriorityTask {
	if m.totalTasks == 0 {
		return nil
	}

	if m.tabs.Active == priorityTabByScore {
		scored := m.visibleScoredTasks()
		if m.cursor < len(scored) {
			return scored[m.cursor]
		}
		return nil
	}

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	// For the Blocked tab, navigate waiting tasks directly
	if m.tabs.Active == priorityTabBlocked {
		if m.cursor < len(waitingOn) {
			return waitingOn[m.cursor]
		}
		return nil
	}

	cursor := m.cursor
	if cursor < len(overdue) {
		return overdue[cursor]
	}
	cursor -= len(overdue)
	if cursor < len(today) {
		return today[cursor]
	}
	cursor -= len(today)
	if cursor < len(thisWeek) {
		return thisWeek[cursor]
	}
	cursor -= len(thisWeek)
	if cursor < len(later) {
		return later[cursor]
	}
	cursor -= len(later)
	if cursor < len(backlog) {
		return backlog[cursor]
	}
	return nil
}

// clampCursor ensures cursor is within valid range.
func (m *PriorityViewModel) clampCursor() {
	m.ClampCursorTo(m.totalTasks)
}
