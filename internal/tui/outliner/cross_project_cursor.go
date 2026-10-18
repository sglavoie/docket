package outliner

// preserveCrossOutlinerCursorForReload captures the selected task by project and
// task ID before reloadCrossProjects starts replacing the snapshot. If a
// superseding reload starts after the task has already disappeared, keep the
// existing target for the newest accepted load instead of losing the user's
// selection.
func (m *OutlinerModel) preserveCrossOutlinerCursorForReload() {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil || row.ProjectID == "" || row.Task.ID == "" {
		return
	}
	m.Population.captureReloadCursor(row.ProjectID, row.Task.ID)
}

// rebaseReloadTargetAfterRelocation re-aims a read already in flight at the row
// the view is standing on now. It belongs to the acts that replace the whole
// projection rather than move within it — a relationship hop, its return, and
// the reset — because the capture a running read is carrying describes the view
// those acts have just left, and honoring it when the result lands would drag
// the cursor back out of the destination the user asked for.
//
// The read itself is left alone. Its projects are the newest reading there is
// and are still wanted; only where to stand in them has changed. With the cursor
// on no task at all the capture is dropped instead, which lets the read fall
// back to the numeric position the relocation established.
func (m *OutlinerModel) rebaseReloadTargetAfterRelocation() {
	if m == nil || m.Project != nil || !m.Population.Loading {
		return
	}
	identity := m.CurrentTaskIdentity()
	m.Population.rebaseReloadCursor(identity.ProjectID, identity.TaskID)
}

// restoreCrossOutlinerCursorAfterReload consumes the captured cursor identity
// after the accepted load rebuilds rows. If the task disappeared or is hidden,
// focus falls back to the nearest task around the old numeric row, preferring a
// following task at equal distance. When no tasks remain, the ordinary navigable
// row clamp still permits an empty section or project header to receive focus.
func (m *OutlinerModel) RestoreCrossOutlinerCursorAfterReload(fallbackCursor, fallbackScrollOffset int) {
	projectID, taskID, headerProjectID := m.Population.takeReloadTarget()
	m.ViewScrollOffset = fallbackScrollOffset
	if headerProjectID != "" {
		if !m.moveCursorToProjectHeader(headerProjectID) {
			// The successor went away too, so the top of the projection — a section
			// header — is the only landing spot left that still means something.
			m.Cursor = 0
			m.ClampCursor()
		}
		return
	}
	if projectID == "" || taskID == "" {
		m.Cursor = fallbackCursor
		m.ClampCursor()
		return
	}
	if m.MoveCursorToProjectTask(taskID, projectID) {
		return
	}
	m.moveCursorToNearestTask(fallbackCursor)
}

// projectHeaderAnchorAfterRemoval names the project group the cursor should hold
// once the named projects' rows leave: the first surviving group below the
// topmost departing one, else the closest surviving group above it. With no
// survivor it returns a departing project's own ID, which stops resolving after
// the reload and therefore lands the cursor on the owning section header rather
// than an arbitrary row. The second result is false when none of the named
// projects has a group on screen at all.
//
// It walks group identities rather than header rows, so the anchor is the same
// whether the group leads with a header or with its first task.
func (m *OutlinerModel) ProjectHeaderAnchorAfterRemoval(projectIDs ...string) (string, bool) {
	leaving := make(map[string]bool, len(projectIDs))
	for _, projectID := range projectIDs {
		if projectID != "" {
			leaving[projectID] = true
		}
	}
	index := -1
	for candidate, row := range m.Rows {
		if row.Group.ProjectGrouped() && leaving[row.Group.ProjectID] {
			index = candidate
			break
		}
	}
	if index < 0 {
		return "", false
	}
	survivor := func(row OutlinerRow) bool {
		return row.Group.ProjectGrouped() && row.Group.ProjectID != "" && !leaving[row.Group.ProjectID]
	}
	for _, row := range m.Rows[index+1:] {
		if survivor(row) {
			return row.Group.ProjectID, true
		}
	}
	for candidate := index - 1; candidate >= 0; candidate-- {
		if row := m.Rows[candidate]; survivor(row) {
			return row.Group.ProjectID, true
		}
	}
	return m.Rows[index].Group.ProjectID, true
}

// moveCursorToProjectHeader parks the cursor on the named project's group. The
// group's header is emitted above its members and carries the same identity, so
// this lands on that header wherever one is drawn.
func (m *OutlinerModel) moveCursorToProjectHeader(projectID string) bool {
	if projectID == "" {
		return false
	}
	for index, row := range m.Rows {
		if row.Group.ProjectGrouped() && row.Group.ProjectID == projectID {
			m.Cursor = index
			m.ClampCursor()
			return true
		}
	}
	return false
}

func (m *OutlinerModel) moveCursorToNearestTask(origin int) {
	if len(m.Rows) == 0 {
		m.Cursor = 0
		return
	}
	if origin < 0 {
		origin = 0
	}
	if origin >= len(m.Rows) {
		origin = len(m.Rows) - 1
	}
	for distance := 0; distance < len(m.Rows); distance++ {
		next := origin + distance
		if next < len(m.Rows) && m.Rows[next].IsTaskRow() && m.Rows[next].Task != nil {
			m.Cursor = next
			return
		}
		previous := origin - distance
		if distance > 0 && previous >= 0 && m.Rows[previous].IsTaskRow() && m.Rows[previous].Task != nil {
			m.Cursor = previous
			return
		}
	}
	m.Cursor = origin
	m.ClampCursor()
}
