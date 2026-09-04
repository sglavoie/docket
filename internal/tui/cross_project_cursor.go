package tui

func (m *OutlinerModel) preserveCrossOutlinerCursorForReload() {
	if m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return
	}
	row := m.rows[m.Cursor]
	if !row.isTaskRow() || row.task == nil || row.ProjectID == "" || row.task.ID == "" {
		return
	}
	m.population.captureReloadCursor(row.ProjectID, row.task.ID)
}

// restoreCrossOutlinerCursorAfterReload consumes the captured cursor identity
// after the accepted load rebuilds rows. If the task disappeared or is hidden,
// focus falls back to the nearest task around the old numeric row, preferring a
// following task at equal distance. When no tasks remain, the ordinary navigable
// row clamp still permits an empty section or project header to receive focus.
func (m *OutlinerModel) restoreCrossOutlinerCursorAfterReload(fallbackCursor, fallbackScrollOffset int) {
	projectID, taskID, headerProjectID := m.population.takeReloadTarget()
	m.ViewScrollOffset = fallbackScrollOffset
	if headerProjectID != "" {
		if !m.moveCursorToProjectHeader(headerProjectID) {
			// The successor went away too, so the top of the projection — a section
			// header — is the only landing spot left that still means something.
			m.Cursor = 0
			m.clampCursor()
		}
		return
	}
	if projectID == "" || taskID == "" {
		m.Cursor = fallbackCursor
		m.clampCursor()
		return
	}
	if m.moveCursorToProjectTask(taskID, projectID) {
		return
	}
	m.moveCursorToNearestTask(fallbackCursor)
}

// projectHeaderAnchorAfterRemoval names the project sub-header the cursor should
// hold once the named projects' rows leave: the first surviving header below the
// topmost departing one, else the closest surviving header above it. With no
// survivor it returns a departing project's own ID, which stops resolving after
// the reload and therefore lands the cursor on the owning section header rather
// than an arbitrary row. The second result is false when none of the named
// projects has a header on screen at all.
func (m *OutlinerModel) projectHeaderAnchorAfterRemoval(projectIDs ...string) (string, bool) {
	leaving := make(map[string]bool, len(projectIDs))
	for _, projectID := range projectIDs {
		if projectID != "" {
			leaving[projectID] = true
		}
	}
	index := -1
	for candidate, row := range m.rows {
		if row.IsProjectHeader && leaving[row.ProjectID] {
			index = candidate
			break
		}
	}
	if index < 0 {
		return "", false
	}
	for _, row := range m.rows[index+1:] {
		if row.IsProjectHeader && row.ProjectID != "" && !leaving[row.ProjectID] {
			return row.ProjectID, true
		}
	}
	for candidate := index - 1; candidate >= 0; candidate-- {
		row := m.rows[candidate]
		if row.IsProjectHeader && row.ProjectID != "" && !leaving[row.ProjectID] {
			return row.ProjectID, true
		}
	}
	return m.rows[index].ProjectID, true
}

func (m *OutlinerModel) moveCursorToProjectHeader(projectID string) bool {
	for index, row := range m.rows {
		if row.IsProjectHeader && row.ProjectID == projectID {
			m.Cursor = index
			m.clampCursor()
			return true
		}
	}
	return false
}

func (m *OutlinerModel) moveCursorToNearestTask(origin int) {
	if len(m.rows) == 0 {
		m.Cursor = 0
		return
	}
	if origin < 0 {
		origin = 0
	}
	if origin >= len(m.rows) {
		origin = len(m.rows) - 1
	}
	for distance := 0; distance < len(m.rows); distance++ {
		next := origin + distance
		if next < len(m.rows) && m.rows[next].isTaskRow() && m.rows[next].task != nil {
			m.Cursor = next
			return
		}
		previous := origin - distance
		if distance > 0 && previous >= 0 && m.rows[previous].isTaskRow() && m.rows[previous].task != nil {
			m.Cursor = previous
			return
		}
	}
	m.Cursor = origin
	m.clampCursor()
}

// cancelCrossProjectLoad settles an abandoned refresh and invalidates any
// result already in flight. The last accepted snapshot remains renderable.
