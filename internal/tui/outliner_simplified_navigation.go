package tui

import tea "charm.land/bubbletea/v2"

// sectionToggleRestoreAnchor identifies a task temporarily hidden by an
// S-driven all-section collapse. The next S-driven expansion restores focus to
// it when the task remains visible in the active view.
type sectionToggleRestoreAnchor struct {
	task       TaskIdentity
	sectionKey string
}

// collapseOrMoveToParent gives h/Left one deterministic tree behavior: collapse
// an expanded section, project header, checklist, or task first; otherwise move
// to the visible parent task. A checklist row is one rung further down, so it
// returns to its owner and leaves the collapse to the next press. From a direct
// child of a focus root the parent has no row, so the ladder pops that one
// frame to reach it.
func (m *OutlinerModel) collapseOrMoveToParent() bool {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	if row.isHeaderRow() {
		if m.isZoomedSectionRow(row) || m.isSectionRowCollapsed(row) {
			return false
		}
		m.setSectionRowCollapsed(row, true)
		m.rebuildRows()
		return true
	}
	if row.IsChecklistItem {
		return m.moveCursorToIdentity(m.taskIdentityForRow(row))
	}
	if !row.isTaskRow() || row.task == nil {
		return false
	}

	identity := m.taskIdentityForRow(row)
	if m.isChecklistExpanded(identity) {
		m.setChecklistExpanded(identity, false)
		m.rebuildRows()
		m.moveCursorToIdentity(identity)
		return true
	}
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
	if m.moveCursorToIdentity(parent) {
		return true
	}
	// The focus root is the one parent that is deliberately rowless, so h leaves
	// exactly that frame to reach it. A promoted row whose parent is merely
	// filtered out fails the comparison and stays silent.
	if root, focused := m.focusRootIdentity(); focused && root == parent {
		m.popFocusLevel()
		return m.moveCursorToIdentity(parent)
	}
	return false
}

// expandAtCursor gives l/Right one behavior: expand the current collapsed
// section, project header, task, or checklist. Children come before the
// checklist, which is the exact reverse of the order h collapses them in. A
// checklist row has nothing left to expand, so it is a no-op there.
// Indent/outdent remain legacy-only until they move into the action sheet.
func (m *OutlinerModel) expandAtCursor() bool {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	if row.isHeaderRow() {
		if !m.isSectionRowCollapsed(row) {
			return false
		}
		m.setSectionRowCollapsed(row, false)
		m.rebuildRows()
		return true
	}
	if !row.isTaskRow() || row.task == nil {
		return false
	}
	identity := m.taskIdentityForRow(row)
	if row.hasVisibleChildren && !m.isExpanded(identity) {
		m.ensureExpandedNodes()
		m.expandedNodes[identity] = true
		m.rebuildRows()
		m.moveCursorToIdentity(identity)
		return true
	}
	if len(row.task.Checklist) == 0 || m.isChecklistExpanded(identity) {
		return false
	}
	m.setChecklistExpanded(identity, true)
	m.rebuildRows()
	m.moveCursorToIdentity(identity)
	return true
}

// moveCursorToAdjacentHeader steps the cursor to the next (delta 1) or previous
// (delta -1) grouping header, wrapping at either end. It reports false only when
// the projection holds no header at all, which is what lets the caller say so
// instead of leaving the key silent. A projection with exactly one header parks
// the cursor there.
func (m *OutlinerModel) moveCursorToAdjacentHeader(delta int) (tea.Cmd, bool) {
	if m == nil || len(m.rows) == 0 {
		return nil, false
	}
	previous := m.cursor
	for step := 1; step <= len(m.rows); step++ {
		index := ((m.cursor+delta*step)%len(m.rows) + len(m.rows)) % len(m.rows)
		if !m.rows[index].isHeaderRow() {
			continue
		}
		m.cursor = index
		m.updateRangeSelection()
		return flashIfJumped(previous, m.cursor, &m.cursorFlash), true
	}
	return nil, false
}

// toggleVisibleSections expands every visible top-level section when any is
// collapsed; otherwise it collapses them all. Project subheaders and task tree
// folds are deliberately outside this bulk operation. A zoom leaves exactly one
// section visible and overrides its fold, so the bulk toggle refuses there
// instead of writing a fold nothing shows.
func (m *OutlinerModel) toggleVisibleSections() bool {
	if m == nil || m.zoom.active() {
		return false
	}
	headers := m.visibleSectionHeaders()
	if len(headers) == 0 {
		return false
	}

	allExpanded := true
	headerKeys := make(map[string]bool, len(headers))
	for _, header := range headers {
		key := header.effectiveCollapseKey()
		headerKeys[key] = true
		if m.isSectionRowCollapsed(header) {
			allExpanded = false
		}
	}

	cursorAnchor := m.captureModeCursor()
	selectedHeaderKey := ""
	if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].IsSectionHeader {
		selectedHeaderKey = m.rows[m.cursor].effectiveCollapseKey()
	}
	if allExpanded {
		m.sectionToggleRestore = nil
		if header, containsCursor := m.topLevelSectionAtCursor(headerKeys); containsCursor {
			if row := m.rows[m.cursor]; row.isTaskRow() && row.task != nil {
				m.sectionToggleRestore = &sectionToggleRestoreAnchor{
					task:       m.taskIdentityForRow(row),
					sectionKey: header.effectiveCollapseKey(),
				}
			}
			for _, section := range headers {
				m.setSectionRowCollapsed(section, true)
			}
			m.rebuildRows()
			m.moveCursorToSectionHeader(header.effectiveCollapseKey())
			return true
		}
		for _, section := range headers {
			m.setSectionRowCollapsed(section, true)
		}
		m.rebuildRows()
		m.restoreModeCursor(cursorAnchor)
		return true
	}

	for _, section := range headers {
		m.setSectionRowCollapsed(section, false)
	}
	m.rebuildRows()
	if m.restoreSectionToggleFocus() {
		return true
	}
	if selectedHeaderKey != "" {
		m.moveCursorToSectionHeader(selectedHeaderKey)
		return true
	}
	m.restoreModeCursor(cursorAnchor)
	return true
}

func (m *OutlinerModel) visibleSectionHeaders() []outlinerRow {
	headers := make([]outlinerRow, 0)
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers = append(headers, row)
		}
	}
	return headers
}

// topLevelSectionAtCursor returns the visible section that owns the cursor's
// current row. Pinned and Today task rows have explicit membership markers;
// those avoid treating the ungrouped Inbox/Next/Backlog rows after Pinned as
// Pinned content. A checklist row carries its owner's markers, so it answers
// the same membership question its owner does.
func (m *OutlinerModel) topLevelSectionAtCursor(sectionKeys map[string]bool) (outlinerRow, bool) {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return outlinerRow{}, false
	}
	selected := m.rows[m.cursor]
	for i := m.cursor; i >= 0; i-- {
		header := m.rows[i]
		if !header.IsSectionHeader {
			continue
		}
		if !sectionKeys[header.effectiveCollapseKey()] {
			return outlinerRow{}, false
		}
		if selected.isTaskRow() || selected.IsChecklistItem {
			if header.IsPinnedSection && !selected.IsPinnedSection {
				return outlinerRow{}, false
			}
			if header.IsTodaySection && !selected.IsTodaySection {
				return outlinerRow{}, false
			}
			if header.IsNextSection && !selected.IsNextSection {
				return outlinerRow{}, false
			}
			if header.PromotedTag != "" && selected.PromotedTag != header.PromotedTag {
				return outlinerRow{}, false
			}
		}
		return header, true
	}
	return outlinerRow{}, false
}

func (m *OutlinerModel) moveCursorToSectionHeader(key string) {
	for i, row := range m.rows {
		if row.IsSectionHeader && row.effectiveCollapseKey() == key {
			m.cursor = i
			return
		}
	}
	m.clampCursor()
}

func (m *OutlinerModel) restoreSectionToggleFocus() bool {
	anchor := m.sectionToggleRestore
	m.sectionToggleRestore = nil
	if anchor == nil {
		return false
	}
	if m.moveCursorToIdentity(anchor.task) {
		return true
	}
	m.moveCursorToSectionHeader(anchor.sectionKey)
	return true
}

func (m *OutlinerModel) setSectionRowCollapsed(row outlinerRow, collapsed bool) {
	key := row.effectiveCollapseKey()
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

// nearestNavigableInWindow returns the navigable row nearest target, searching
// preferred (1 down, -1 up) first. It never leaves [start, end-1] the way
// crossSkipNonNavigableRows can, because a landing outside the drawn window
// would scroll — which is the one thing a screen-relative jump must not do.
func (m *OutlinerModel) nearestNavigableInWindow(target, start, end, preferred int) (int, bool) {
	for _, step := range []int{preferred, -preferred} {
		for i := target; i >= start && i < end; i += step {
			if m.rows[i].isNavigable() {
				return i, true
			}
		}
	}
	return 0, false
}

// simplifiedReorderEligible dispatches J/K by the selected row's ordering
// context. Pinned uses manual pin order wherever it renders; Due is automatic;
// Today and Next use durable planning order; All, Inbox, and Backlog use
// same-project sibling order.
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
	// A pinned row carries its own manual order, so it answers before the active
	// mode's reorder policy.
	if m.isPinnedReorderRow() {
		return true
	}
	mode, ok := OutlinerModeByID(m.activeOutlinerMode())
	if !ok || mode.ReorderPolicy == ReorderPolicyNone {
		return false
	}
	if m.isPlanningReorderMode() {
		return true
	}
	switch mode.ID {
	case ModeAll, ModeInbox, ModeBacklog:
		return m.resolveProjectForRow(row) != nil
	default:
		return false
	}
}
