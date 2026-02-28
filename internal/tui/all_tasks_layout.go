package tui

// toggleLayout switches between list and kanban mode.
func (m *AllTasksViewModel) toggleLayout() {
	if m.layoutMode == ATLayoutList {
		// Flat grouping produces one column which is degenerate — auto-switch.
		if m.groupBy == ATGroupByFlat {
			m.groupBy = ATGroupByProject
			m.applyFiltersAndGroup()
			m.clampCursor()
			m.statusMessage = "Switched to Project grouping for kanban"
		}
		m.layoutMode = ATLayoutKanban
		m.initKanbanColumns()
	} else {
		m.layoutMode = ATLayoutList
		m.syncKanbanToList()
	}
}

// initKanbanColumns sets up per-group column state and positions the active
// column to whichever group contains the current list cursor.
func (m *AllTasksViewModel) initKanbanColumns() {
	m.kanbanColumns = make(map[string]*ColumnState)
	for _, g := range m.groups {
		m.kanbanColumns[g.Name] = &ColumnState{}
	}

	// Find which group the list cursor sits in and set kanbanActiveCol.
	pos := 0
	for gi, group := range m.groups {
		headerPos := pos
		pos++ // past header
		end := pos
		if !m.collapsedGroups[group.Name] {
			end = pos + len(group.Tasks)
		}
		if m.cursor >= headerPos && m.cursor < end {
			m.kanbanActiveCol = gi
			// If cursor was on a task (not header), set column cursor too.
			if m.cursor > headerPos && !m.collapsedGroups[group.Name] {
				m.kanbanColumns[group.Name].Cursor = m.cursor - headerPos - 1
			}
			break
		}
		pos = end
	}
	m.kanbanColScroll = 0
	m.ensureActiveKanbanColumnVisible()
}

// syncKanbanToList positions the list cursor on the task selected in the active
// kanban column when switching back to list mode.
func (m *AllTasksViewModel) syncKanbanToList() {
	if m.kanbanActiveCol < 0 || m.kanbanActiveCol >= len(m.groups) {
		return
	}
	group := m.groups[m.kanbanActiveCol]
	cs := m.kanbanColumns[group.Name]

	pos := 0
	for gi, g := range m.groups {
		if gi == m.kanbanActiveCol {
			if cs != nil && cs.Cursor >= 0 && cs.Cursor < len(g.Tasks) {
				pos += 1 + cs.Cursor // header + offset
			}
			break
		}
		pos++ // header
		if !m.collapsedGroups[g.Name] {
			pos += len(g.Tasks)
		}
	}
	m.cursor = pos
	m.clampCursor()
}

// getKanbanTaskAtCursor returns the task under the kanban column cursor.
func (m *AllTasksViewModel) getKanbanTaskAtCursor() *CrossProjectTask {
	if m.kanbanActiveCol < 0 || m.kanbanActiveCol >= len(m.groups) {
		return nil
	}
	group := m.groups[m.kanbanActiveCol]
	cs := m.kanbanColumns[group.Name]
	if cs == nil || cs.Cursor < 0 || cs.Cursor >= len(group.Tasks) {
		return nil
	}
	return &group.Tasks[cs.Cursor]
}
