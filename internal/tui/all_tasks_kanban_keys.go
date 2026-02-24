package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleKanbanKeyMsg handles keyboard input specific to kanban layout mode.
// It processes column/row navigation, task movement between columns, and
// delegates back for unhandled keys. Returns (model, cmd, handled).
func (m *AllTasksViewModel) handleKanbanKeyMsg(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	// Column navigation
	case "h":
		m.moveKanbanColumnLeft()
		return m, nil, true

	case "l":
		m.moveKanbanColumnRight()
		return m, nil, true

	// Vertical navigation within active column
	case "up", "k":
		m.moveKanbanCursorUp()
		return m, nil, true

	case "down", "j":
		m.moveKanbanCursorDown()
		return m, nil, true

	case "G":
		m.jumpKanbanToLast()
		return m, nil, true

	case "ctrl+u", "pgup":
		m.pageKanbanUp()
		return m, nil, true

	case "ctrl+d", "pgdown":
		m.pageKanbanDown()
		return m, nil, true

	// Jump to next/prev non-empty column
	case "n":
		m.jumpToNextNonEmptyColumn(+1)
		return m, nil, true

	case "N":
		m.jumpToNextNonEmptyColumn(-1)
		return m, nil, true

	// Task movement between columns
	case "{":
		return m.moveKanbanTask(-1)

	case "}":
		return m.moveKanbanTask(+1)
	}

	return m, nil, false
}

// moveKanbanColumnLeft moves focus to the previous column.
// When all columns fit on screen, empty columns are skipped.
func (m *AllTasksViewModel) moveKanbanColumnLeft() {
	if len(m.groups) <= 1 {
		return
	}
	if len(m.groups) > m.maxVisibleKanbanColumns() {
		if m.kanbanActiveCol > 0 {
			m.kanbanActiveCol--
		}
	} else {
		m.kanbanActiveCol = m.findNextNonEmptyKanbanColumn(-1)
	}
	m.ensureActiveKanbanColumnVisible()
}

// moveKanbanColumnRight moves focus to the next column.
// When all columns fit on screen, empty columns are skipped.
func (m *AllTasksViewModel) moveKanbanColumnRight() {
	if len(m.groups) <= 1 {
		return
	}
	if len(m.groups) > m.maxVisibleKanbanColumns() {
		if m.kanbanActiveCol < len(m.groups)-1 {
			m.kanbanActiveCol++
		}
	} else {
		m.kanbanActiveCol = m.findNextNonEmptyKanbanColumn(+1)
	}
	m.ensureActiveKanbanColumnVisible()
}

// findNextNonEmptyKanbanColumn searches for the next non-empty column in the given direction.
func (m *AllTasksViewModel) findNextNonEmptyKanbanColumn(direction int) int {
	for i := m.kanbanActiveCol + direction; i >= 0 && i < len(m.groups); i += direction {
		if len(m.groups[i].Tasks) > 0 {
			return i
		}
	}
	return m.kanbanActiveCol
}

// moveKanbanCursorUp moves the cursor up within the active column.
func (m *AllTasksViewModel) moveKanbanCursorUp() {
	cs := m.activeKanbanColumnState()
	if cs != nil && cs.Cursor > 0 {
		cs.Cursor--
	}
}

// moveKanbanCursorDown moves the cursor down within the active column.
func (m *AllTasksViewModel) moveKanbanCursorDown() {
	cs := m.activeKanbanColumnState()
	if cs == nil {
		return
	}
	taskCount := m.activeKanbanTaskCount()
	if cs.Cursor < taskCount-1 {
		cs.Cursor++
	}
}

// jumpKanbanToLast jumps cursor to the last task in the active column.
func (m *AllTasksViewModel) jumpKanbanToLast() {
	cs := m.activeKanbanColumnState()
	if cs == nil {
		return
	}
	taskCount := m.activeKanbanTaskCount()
	if taskCount > 0 {
		cs.Cursor = taskCount - 1
	}
}

// pageKanbanUp moves the cursor up by half a page within the active column.
func (m *AllTasksViewModel) pageKanbanUp() {
	cs := m.activeKanbanColumnState()
	if cs == nil {
		return
	}
	jump := HalfPage(m.kanbanHeight)
	cs.Cursor -= jump
	if cs.Cursor < 0 {
		cs.Cursor = 0
	}
}

// pageKanbanDown moves the cursor down by half a page within the active column.
func (m *AllTasksViewModel) pageKanbanDown() {
	cs := m.activeKanbanColumnState()
	if cs == nil {
		return
	}
	taskCount := m.activeKanbanTaskCount()
	jump := HalfPage(m.kanbanHeight)
	cs.Cursor += jump
	if cs.Cursor >= taskCount {
		cs.Cursor = taskCount - 1
	}
	if cs.Cursor < 0 {
		cs.Cursor = 0
	}
}

// jumpToNextNonEmptyColumn jumps to the next (direction=+1) or previous
// (direction=-1) column that has at least one task.
func (m *AllTasksViewModel) jumpToNextNonEmptyColumn(direction int) {
	next := m.findNextNonEmptyKanbanColumn(direction)
	if next != m.kanbanActiveCol {
		m.kanbanActiveCol = next
		m.ensureActiveKanbanColumnVisible()
	}
}

// moveKanbanTask moves the task under cursor to an adjacent column.
// Only works for Status, Section, and Priority grouping modes.
func (m *AllTasksViewModel) moveKanbanTask(direction int) (tea.Model, tea.Cmd, bool) {
	// Validate grouping mode supports movement
	if !m.kanbanGroupBySupportsMove() {
		m.statusMessage = fmt.Sprintf("Cannot move tasks between %s groups", m.groupBy)
		return m, nil, true
	}

	// Validate target column exists
	targetCol := m.kanbanActiveCol + direction
	if targetCol < 0 || targetCol >= len(m.groups) {
		return m, nil, true
	}

	item := m.getKanbanTaskAtCursor()
	if item == nil {
		return m, nil, true
	}

	targetGroup := m.groups[targetCol]
	project := item.Project

	// Backup before mutation
	if err := m.projectService.Backup(project); err != nil {
		m.statusMessage = fmt.Sprintf("Failed to backup: %v", err)
		return m, nil, true
	}

	// Apply the property change based on grouping mode
	taskID := item.Task.ID
	project.MutateTask(taskID, func(t *models.Task) {
		switch m.groupBy {
		case ATGroupByStatus:
			newStatus := models.TaskStatus(targetGroup.Name)
			if t.Status.IsComplete() && !newStatus.IsComplete() {
				t.Reopen()
			}
			if t.Status != newStatus {
				t.ApplyTransition(newStatus)
			}
		case ATGroupBySection:
			section := targetGroup.Name
			if section == "(none)" {
				section = ""
			}
			t.Section = section
		case ATGroupByPriority:
			priority := parsePriorityFromGroupName(targetGroup.Name)
			t.Priority = priority
		}
	})

	// Save
	if err := m.projectService.Save(project); err != nil {
		m.statusMessage = fmt.Sprintf("Failed to save: %v", err)
		return m, nil, true
	}

	// Reload data and reposition cursor in target column
	m.loadAllTasks()
	m.kanbanActiveCol = targetCol
	if targetCol >= len(m.groups) {
		m.kanbanActiveCol = len(m.groups) - 1
	}
	m.reinitKanbanColumns()
	// Position cursor at end of target column (where moved task likely landed)
	if m.kanbanActiveCol >= 0 && m.kanbanActiveCol < len(m.groups) {
		group := m.groups[m.kanbanActiveCol]
		cs := m.kanbanColumns[group.Name]
		if cs != nil {
			// Try to find the moved task in the target column
			for i, t := range group.Tasks {
				if t.Task.ID == taskID {
					cs.Cursor = i
					break
				}
			}
		}
	}
	m.ensureActiveKanbanColumnVisible()
	return m, nil, true
}

// kanbanGroupBySupportsMove returns true if the current grouping mode
// supports moving tasks between columns via {/}.
func (m *AllTasksViewModel) kanbanGroupBySupportsMove() bool {
	switch m.groupBy {
	case ATGroupByStatus, ATGroupBySection, ATGroupByPriority:
		return true
	default:
		return false
	}
}

// parsePriorityFromGroupName extracts a priority integer from a group name like "P1", "P2", etc.
func parsePriorityFromGroupName(name string) int {
	if name == "Unset" {
		return models.PriorityDefault
	}
	if len(name) == 2 && name[0] == 'P' && name[1] >= '1' && name[1] <= '5' {
		return int(name[1] - '0')
	}
	return models.PriorityDefault
}

// activeKanbanColumnState returns the ColumnState for the active kanban column.
func (m *AllTasksViewModel) activeKanbanColumnState() *ColumnState {
	if m.kanbanActiveCol < 0 || m.kanbanActiveCol >= len(m.groups) {
		return nil
	}
	group := m.groups[m.kanbanActiveCol]
	cs := m.kanbanColumns[group.Name]
	if cs == nil {
		cs = &ColumnState{}
		m.kanbanColumns[group.Name] = cs
	}
	return cs
}

// activeKanbanTaskCount returns the number of tasks in the active kanban column.
func (m *AllTasksViewModel) activeKanbanTaskCount() int {
	if m.kanbanActiveCol < 0 || m.kanbanActiveCol >= len(m.groups) {
		return 0
	}
	return len(m.groups[m.kanbanActiveCol].Tasks)
}

// reinitKanbanColumns rebuilds column state for all groups, preserving cursor
// positions where possible.
func (m *AllTasksViewModel) reinitKanbanColumns() {
	old := m.kanbanColumns
	m.kanbanColumns = make(map[string]*ColumnState)
	for _, g := range m.groups {
		if prev, ok := old[g.Name]; ok {
			prev.ValidateCursor(len(g.Tasks))
			m.kanbanColumns[g.Name] = prev
		} else {
			m.kanbanColumns[g.Name] = &ColumnState{}
		}
	}
}
