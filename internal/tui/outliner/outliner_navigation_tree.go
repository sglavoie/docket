package outliner

import (
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// collapseOrMoveToParent gives h/Left one deterministic tree behavior: collapse
// an expanded section, project header, checklist, or task first; otherwise move
// to the visible parent task. A checklist row is one rung further down, so it
// returns to its owner and leaves the collapse to the next press. From a direct
// child of a focus root the parent has no row, so the ladder pops that one
// frame to reach it.
//
// A task that has run out of hierarchy — no open checklist, no open children, and
// no parent at all — folds its own project group as the last rung, which is what
// keeps one group reachable from inside it when the projection draws no heading
// above it. The rung is the group's, not the section's: a section always draws a
// header of its own, so h on a root task in a single-project form still says
// nothing. A parent that exists but is hidden by narrowing is a hierarchy state
// rather than a group boundary, so it keeps its silence instead of folding.
func (m *OutlinerModel) CollapseOrMoveToParent() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if row.IsHeaderRow() {
		// h says nothing on the focused-project header: it names the projection the
		// cursor is already inside, so the only outline it could fold is the whole
		// one.
		if !row.IsCollapsibleHeader() || m.IsZoomedSectionRow(row) || m.IsSectionRowCollapsed(row) {
			return false
		}
		m.SetSectionRowCollapsed(row, true)
		m.RebuildRows()
		return true
	}
	if row.IsChecklistItem {
		return m.MoveCursorToIdentity(m.TaskIdentityForRow(row))
	}
	if !row.IsTaskRow() || row.Task == nil {
		return false
	}

	identity := m.TaskIdentityForRow(row)
	if m.IsChecklistExpanded(identity) {
		m.Folds.SetChecklist(identity, false)
		m.RebuildRows()
		m.MoveCursorToIdentity(identity)
		return true
	}
	if row.HasVisibleChildren && m.IsExpanded(identity) {
		m.ensureExpandedNodes()
		m.Folds.setNode(identity, false)
		m.RebuildRows()
		m.MoveCursorToIdentity(identity)
		return true
	}
	parentID := parentIDOf(row.Task)
	if parentID == "" {
		return m.foldGroupAtCursor()
	}
	parent := taskref.TaskIdentity{ProjectID: identity.ProjectID, TaskID: parentID}
	if m.MoveCursorToIdentity(parent) {
		return true
	}
	// The focus root is the one parent that is deliberately rowless, so h leaves
	// exactly that frame to reach it. A promoted row whose parent is merely
	// filtered out fails the comparison and stays silent.
	if root, focused := m.FocusRootIdentity(); focused && root == parent {
		m.PopFocusLevel()
		return m.MoveCursorToIdentity(parent)
	}
	return false
}

// foldGroupAtCursor closes the project group the cursor's row belongs to and
// leaves the cursor on the summary that replaces it, remembering the task it was
// standing on so the matching expansion can put it back. It is the one fold a
// task row can write, and it reads the row's own group rather than searching
// upward for a heading, so it answers the same way whether or not one is drawn. A
// row outside a project group owns no such fold and refuses.
func (m *OutlinerModel) foldGroupAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	group := m.Rows[m.Cursor].Group
	if !group.ProjectGrouped() || m.IsProjectGroupFolded(group) {
		return false
	}
	m.rememberCursorGroupFocus()
	m.SetProjectGroupFolded(group, true)
	m.RebuildRows()
	if !m.moveCursorToGroupKey(group.ProjectKey) {
		m.ClampCursor()
	}
	return true
}

// expandAtCursor gives l/Right one behavior: expand the current collapsed
// section, project header, task, or checklist. Children come before the
// checklist, which is the exact reverse of the order h collapses them in. A
// checklist row has nothing left to expand, so it is a no-op there.
// Indent/outdent remain legacy-only until they move into the action sheet.
//
// Expanding a project group is the one expansion that also moves the cursor: the
// summary it opens is a stand-in for the group's rows, so l lands on the task the
// matching fold was covering, or on the group's first task when there is none to
// return to. A section keeps its own header once expanded, so the cursor stays on
// it there.
func (m *OutlinerModel) ExpandAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if row.IsHeaderRow() {
		// l is silent on the focused-project header for the same reason h is: it
		// carries no collapse state either way.
		if !row.IsCollapsibleHeader() || !m.IsSectionRowCollapsed(row) {
			return false
		}
		remembered, hasRemembered := m.Folds.groupFocus(row.Group.ProjectKey)
		m.SetSectionRowCollapsed(row, false)
		m.RebuildRows()
		if row.Group.ProjectGrouped() && !m.restoreGroupCursor(row.Group, remembered, hasRemembered) {
			m.ClampCursor()
		}
		return true
	}
	if !row.IsTaskRow() || row.Task == nil {
		return false
	}
	identity := m.TaskIdentityForRow(row)
	if row.HasVisibleChildren && !m.IsExpanded(identity) {
		m.ensureExpandedNodes()
		m.Folds.setNode(identity, true)
		m.RebuildRows()
		m.MoveCursorToIdentity(identity)
		return true
	}
	if len(row.Task.Checklist) == 0 || m.IsChecklistExpanded(identity) {
		return false
	}
	m.Folds.SetChecklist(identity, true)
	m.RebuildRows()
	m.MoveCursorToIdentity(identity)
	return true
}

// toggleAllSubtrees is t, and takes the same direction rule its two siblings
// do: any fold anywhere means the press unfolds. The two branches keep their own
// bodies rather than merging, because their cursor-restore policies genuinely
// differ — folding must ride the cursor up its ParentID chain and unfolding must
// not — so merging would only move the branch.
func (m *OutlinerModel) ToggleAllSubtrees() bool {
	if m == nil {
		return false
	}
	if m.AnyTaskFoldCollapsed() {
		return m.expandAllSubtrees()
	}
	return m.collapseAllSubtrees()
}

// collapseAllSubtrees folds every task subtree at once, the bulk counterpart to
// the single fold h walks. Sections and checklists are deliberately outside it:
// s owns the section axis, p the project axis, and a checklist already defaults
// collapsed, so one press means one thing here too. A zoom overrides a section's
// fold but leaves task folds alone, which is why this does not refuse there the
// way the section toggle does.
func (m *OutlinerModel) collapseAllSubtrees() bool {
	if m == nil || !m.setAllTaskFolds(false) {
		return false
	}
	anchor, anchored := m.cursorTaskIdentity()
	parents := m.cursorAncestorIdentities()
	m.RebuildRows()
	if !anchored || !m.MoveCursorToIdentity(anchor) {
		// The cursor's own row just went under a fold, so it rides up to the
		// nearest ancestor that still has one.
		m.moveCursorToNearestIdentity(parents)
	}
	return true
}

// expandAllSubtrees unfolds every task subtree. The cursor's row was visible
// before and stays visible, so it only needs putting back after the rebuild.
func (m *OutlinerModel) expandAllSubtrees() bool {
	if m == nil || !m.setAllTaskFolds(true) {
		return false
	}
	anchor, anchored := m.cursorTaskIdentity()
	m.RebuildRows()
	if anchored {
		m.MoveCursorToIdentity(anchor)
	}
	return true
}
