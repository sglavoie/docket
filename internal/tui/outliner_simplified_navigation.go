package tui

import tea "charm.land/bubbletea/v2"

// collapseOrMoveToParent gives h/Left one deterministic tree behavior: collapse
// an expanded header or task first, otherwise move to the visible parent task.
func (m *OutlinerModel) collapseOrMoveToParent() bool {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	if row.IsSectionHeader {
		if m.isSectionRowCollapsed(row) {
			return false
		}
		m.setSectionRowCollapsed(row, true)
		m.rebuildRows()
		return true
	}
	if !row.isTaskRow() || row.task == nil {
		return false
	}

	identity := m.taskIdentityForRow(row)
	if row.hasVisibleChildren && m.isExpanded(identity) {
		m.ensureExpandedNodes()
		m.expandedNodes[identity] = false
		m.rebuildRows()
		m.moveCursorToIdentity(identity)
		return true
	}
	parentID := parentIDOf(row.task)
	if parentID == "" {
		return false
	}
	parent := TaskIdentity{ProjectID: identity.ProjectID, TaskID: parentID}
	return m.moveCursorToIdentity(parent)
}

// expandAtCursor gives l/Right one behavior: expand the current collapsed
// section or task. Indent/outdent remain legacy-only until they move into the
// action sheet.
func (m *OutlinerModel) expandAtCursor() bool {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	if row.IsSectionHeader {
		if !m.isSectionRowCollapsed(row) {
			return false
		}
		m.setSectionRowCollapsed(row, false)
		m.rebuildRows()
		return true
	}
	if !row.isTaskRow() || row.task == nil || !row.hasVisibleChildren {
		return false
	}
	identity := m.taskIdentityForRow(row)
	if m.isExpanded(identity) {
		return false
	}
	m.ensureExpandedNodes()
	m.expandedNodes[identity] = true
	m.rebuildRows()
	m.moveCursorToIdentity(identity)
	return true
}

func (m *OutlinerModel) setSectionRowCollapsed(row outlinerRow, collapsed bool) {
	key := row.effectiveCollapseKey()
	if isReviewCollapseKey(key) {
		if m.reviewCollapsedSections == nil {
			m.reviewCollapsedSections = make(map[string]bool)
		}
		m.reviewCollapsedSections[key] = collapsed
		m.crossForceRebuild = true
		return
	}
	if m.collapsedSections == nil {
		m.collapsedSections = make(map[string]bool)
	}
	m.collapsedSections[key] = collapsed
}

func (m *OutlinerModel) moveCursorToIdentity(identity TaskIdentity) bool {
	for i, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		if m.taskIdentityForRow(row) == identity {
			m.cursor = i
			return true
		}
	}
	m.clampCursor()
	return false
}

func (m *OutlinerModel) crossSkipNonNavigableRows(
	previous int,
	cmd tea.Cmd,
) (tea.Model, tea.Cmd) {
	if len(m.rows) == 0 {
		return m, cmd
	}
	if m.cursor < previous {
		for m.cursor > 0 && !m.rows[m.cursor].isNavigable() {
			m.cursor--
		}
		if !m.rows[m.cursor].isNavigable() {
			for m.cursor < len(m.rows)-1 && !m.rows[m.cursor].isNavigable() {
				m.cursor++
			}
		}
	} else {
		for m.cursor < len(m.rows)-1 && !m.rows[m.cursor].isNavigable() {
			m.cursor++
		}
		if !m.rows[m.cursor].isNavigable() {
			for m.cursor > 0 && !m.rows[m.cursor].isNavigable() {
				m.cursor--
			}
		}
	}
	return m, cmd
}

// simplifiedReorderEligible limits J/K to a project-focused manually ordered
// All/Inbox/Next/Backlog sequence. Base all-project and computed modes cannot
// mutate ordering.
func (m *OutlinerModel) simplifiedReorderEligible() bool {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	if m.multiSelectMode {
		return false
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil {
		return false
	}
	if m.project == nil && !m.isProjectScoped() {
		return false
	}
	mode, ok := OutlinerModeByID(m.activeOutlinerMode())
	return ok && mode.SupportsManualReorder()
}
