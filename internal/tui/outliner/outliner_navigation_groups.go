package outliner

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// VisibleSections is the s-side counterpart of VisibleProjectGroups: the
// distinct section collapse keys the current projection holds, in
// first-appearance order. It reads the section half every row carries rather
// than scanning for header rows, so the two bulk folds are symmetric and neither
// depends on a heading being drawn. A projection whose mode emits no sections
// carries an empty section half on every row and returns none, so s stays
// unavailable there rather than toggling nothing.
func (m *OutlinerModel) VisibleSections() []string {
	keys := make([]string, 0)
	seen := make(map[string]bool)
	for _, row := range m.Rows {
		key := row.Group.SectionKey
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

// VisibleProjectGroups is the p-side counterpart to visibleSectionHeaders: the
// distinct project groups the current projection holds, in first-appearance
// order. It reads the structural identity every member row carries rather than
// scanning for header rows, which is what lets one helper serve the five
// project-group namespaces across all seven modes and keeps a group nameable
// whether or not its header is drawn. A projection with no project groups — both
// single-project forms — returns none, so p stays unavailable there rather than
// toggling nothing.
func (m *OutlinerModel) VisibleProjectGroups() []OutlinerRowGroup {
	groups := make([]OutlinerRowGroup, 0)
	seen := make(map[OutlinerRowGroup]bool)
	for _, row := range m.Rows {
		if !row.Group.ProjectGrouped() || seen[row.Group] {
			continue
		}
		seen[row.Group] = true
		groups = append(groups, row.Group)
	}
	return groups
}

// topLevelSectionAtCursor names the visible section that owns the cursor's
// current row. Membership is the group the row carries, so a row after Pinned
// that belongs to the mode below it is never read as Pinned content and no lane
// marker has to be consulted to say so. A checklist row carries its owner's
// group, so it answers the same membership question its owner does.
//
// It returns the key rather than the header row because the key is the whole of
// what a caller needs: the fold to write and the group to seat the cursor in are
// both spelled by it, and looking a header up would make the answer depend on a
// heading this projection may not draw.
func (m *OutlinerModel) topLevelSectionAtCursor(sectionKeys map[string]bool) (string, bool) {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return "", false
	}
	key := m.Rows[m.Cursor].Group.SectionKey
	if key == "" || !sectionKeys[key] {
		return "", false
	}
	return key, true
}

// moveCursorToGroupKey parks the cursor on the first row belonging to the group
// the key names, reporting whether one was found. A group's header is emitted
// above its members and carries the same identity, so this lands on the header
// wherever one is drawn and on the group's first task where none is.
//
// An empty key is refused rather than matched: it is not a namespace, and the
// focused-project header carries no group precisely so no unmatched seek can
// land on it.
func (m *OutlinerModel) moveCursorToGroupKey(key string) bool {
	if m == nil || key == "" {
		return false
	}
	for i, row := range m.Rows {
		if slices.Contains(row.Group.CollapseKeys(), key) {
			m.Cursor = i
			return true
		}
	}
	return false
}

// moveCursorToGroupTask parks the cursor on the group's first task row, which is
// where a group lands a cursor that has no better claim on a row inside it. It
// skips the group's own header because that row is what the cursor is leaving:
// an expansion whose whole point is to reach the work would otherwise stop on the
// heading it just opened.
func (m *OutlinerModel) moveCursorToGroupTask(group OutlinerRowGroup) bool {
	if m == nil || group == (OutlinerRowGroup{}) {
		return false
	}
	for i, row := range m.Rows {
		if row.Group == group && row.IsTaskRow() && row.Task != nil {
			m.Cursor = i
			return true
		}
	}
	return false
}

// restoreGroupCursor seats the cursor inside a group that has just been
// expanded: on the task the fold was covering when it still has a row in that
// same group, otherwise on the group's first task. The remembered identity is
// checked against the group's own rows rather than against the whole projection,
// so a memory left behind by a restored checkpoint can only be ignored, never
// followed somewhere else.
func (m *OutlinerModel) restoreGroupCursor(group OutlinerRowGroup, remembered taskref.TaskIdentity, ok bool) bool {
	if m == nil {
		return false
	}
	if ok {
		for i, row := range m.Rows {
			if row.Group != group || !row.IsTaskRow() || row.Task == nil {
				continue
			}
			if m.TaskIdentityForRow(row) == remembered {
				m.Cursor = i
				return true
			}
		}
	}
	return m.moveCursorToGroupTask(group)
}

// rememberCursorGroupFocus records the task the cursor is standing on against
// its own project group, so a fold that is about to hide it can be undone onto
// the same row. A cursor on a header or outside a project group records nothing,
// which is what makes the expansion fall back to the group's first task.
func (m *OutlinerModel) rememberCursorGroupFocus() {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return
	}
	row := m.Rows[m.Cursor]
	if !row.Group.ProjectGrouped() {
		return
	}
	identity, ok := m.cursorTaskIdentity()
	if !ok {
		return
	}
	m.Folds.rememberGroupFocus(row.Group.ProjectKey, identity)
}

func (m *OutlinerModel) restoreSectionToggleFocus() bool {
	anchor := m.Folds.takeSectionToggleAnchor()
	if anchor == nil {
		return false
	}
	if m.MoveCursorToIdentity(anchor.task) {
		return true
	}
	if !m.moveCursorToGroupKey(anchor.sectionKey) {
		m.ClampCursor()
	}
	return true
}

// clearAllFolds opens everything at once: every task subtree and every section
// and project-group key, across every namespace collapsedSections holds rather
// than only the ones this mode happens to draw. It is deliberately one-way,
// which is what the three toggles cannot be — `s`, `p`, and `t` each reverse, so
// reaching a fully open tree from a mixed state takes three presses that may
// each go the wrong way, and no fourth toggle would fix that.
//
// It clears the whole map rather than the visible headers because a fold under a
// fold is invisible by definition, and "all" that stopped at the first closed
// header would leave a user pressing it twice. The keys are namespaced per
// grouping surface and survive a mode change, so this is also the one act that
// ends a fold another mode wrote; that is the point of it rather than a leak.
//
// Checklists are outside it for the reason they are outside `t`: they default
// collapsed, so opening them is a different promise than undoing a fold.
func (m *OutlinerModel) ClearAllFolds() bool {
	if m == nil {
		return false
	}
	changed := m.setAllTaskFolds(true)
	for _, collapsed := range m.Folds.CollapsedSections {
		if collapsed {
			changed = true
			break
		}
	}
	if !changed {
		return false
	}
	m.Folds.clearSections()
	anchor := m.captureModeCursor()
	m.RebuildRows()
	m.restoreModeCursor(anchor)
	return true
}

// setGroupFold is the single write of a grouping fold, whether a header row or a
// group identity named it, so it is also the single place an empty key is
// refused. An empty key is not a namespace: writing it would make one nameless
// fold that every keyless header and every ungrouped projection would share.
//
// It is also where the group's remembered cursor is dropped, so that memory lasts
// exactly as long as the fold that needs it: opening a group forgets the task its
// summary was covering, and a later close writes the task it is covering then.
func (m *OutlinerModel) setGroupFold(key string, collapsed bool) {
	if m == nil || key == "" {
		return
	}
	m.Folds.setSection(key, collapsed)
	if !collapsed {
		m.Folds.forgetGroupFocus(key)
	}
}

// setSectionRowCollapsed is the header-row spelling of that write, so it is also
// where the one header that owns no fold is refused. The focused-project
// header's EffectiveCollapseKey is empty, and setGroupFold would refuse it
// anyway; refusing the row kind here keeps the reason readable at the call site.
func (m *OutlinerModel) SetSectionRowCollapsed(row OutlinerRow, collapsed bool) {
	if !row.IsCollapsibleHeader() {
		return
	}
	m.setGroupFold(row.EffectiveCollapseKey(), collapsed)
}

// SetProjectGroupFolded writes one project group's fold from its structural
// identity, which is how the p axis reaches a group without holding its header
// row. A group carrying no project half owns no fold and is refused.
func (m *OutlinerModel) SetProjectGroupFolded(group OutlinerRowGroup, collapsed bool) {
	m.setGroupFold(group.ProjectKey, collapsed)
}

// IsProjectGroupFolded reports whether the group's own fold is closed, reading
// the same zoom exemption every other fold read goes through.
func (m *OutlinerModel) IsProjectGroupFolded(group OutlinerRowGroup) bool {
	return m.sectionCollapsed(group.ProjectKey)
}

// rowStartsGroup reports whether the row at index is where a block begins, which
// is what n and N step between. A drawn heading is one: a section header and a
// project header — whether it is an expanded group's heading or a folded group's
// summary — both open the block they name. So is the first row of a project group
// that draws no heading at all, because the block still starts there and a key
// that skipped it would leave a whole group unreachable.
//
// Membership is read off the rows' own groups rather than off the headings above
// them, so the three cases are one rule: index starts a block when it carries a
// project group its predecessor did not. A separator, a note row, and a
// focused-project header all carry the zero group, so none of them turns the row
// after it into a second start of the same block.
func (m *OutlinerModel) rowStartsGroup(index int) bool {
	if m == nil || index < 0 || index >= len(m.Rows) {
		return false
	}
	row := m.Rows[index]
	if row.IsHeaderRow() {
		return true
	}
	if !row.Group.ProjectGrouped() {
		return false
	}
	return index == 0 || m.Rows[index-1].Group != row.Group
}

// moveCursorToAdjacentHeader steps the cursor to the next (delta 1) or previous
// (delta -1) group start, wrapping at either end. It reports false only when the
// projection holds no group at all, which is what lets the caller say so instead
// of leaving the key silent. A projection with exactly one start parks the cursor
// there.
//
// n and N are user cursor navigation like j and k, so they void a pending
// relocation capture on the same terms: a wrap that leaves and returns to the
// settled row is numerically invisible to the reload guard, and parking on the
// only start moves nothing and voids nothing.
func (m *OutlinerModel) MoveCursorToAdjacentHeader(delta int) (tea.Cmd, bool) {
	if m == nil || len(m.Rows) == 0 {
		return nil, false
	}
	previous := m.Cursor
	for step := 1; step <= len(m.Rows); step++ {
		index := ((m.Cursor+delta*step)%len(m.Rows) + len(m.Rows)) % len(m.Rows)
		if !m.rowStartsGroup(index) {
			continue
		}
		m.Cursor = index
		m.UpdateRangeSelection()
		m.AdoptNavigatedCursor(previous)
		return listsurface.FlashIfJumped(previous, m.Cursor, &m.CursorFlash), true
	}
	return nil, false
}

// toggleVisibleSections is s: it expands every visible top-level section when
// any is collapsed; otherwise it collapses them all. Project group headers
// answer to p and task subtrees to t, so this bulk operation stays on the one
// axis its key names. A zoom leaves exactly one section visible and overrides
// its fold, so the bulk toggle refuses there instead of writing a fold nothing
// shows.
func (m *OutlinerModel) ToggleVisibleSections() bool {
	if m == nil || m.Focus.Zoom.Active() {
		return false
	}
	sections := m.VisibleSections()
	if len(sections) == 0 {
		return false
	}

	allExpanded := true
	sectionKeys := make(map[string]bool, len(sections))
	for _, key := range sections {
		sectionKeys[key] = true
		if m.sectionCollapsed(key) {
			allExpanded = false
		}
	}

	cursorAnchor := m.captureModeCursor()
	// A cursor already parked on a section header rides that section's own group
	// through the reshape, because the header is the only thing the press leaves
	// standing there. The key comes off the row's group rather than off the header
	// row, so the same capture serves a projection that draws no header for it.
	selectedSectionKey := ""
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) && m.Rows[m.Cursor].IsSectionHeader {
		selectedSectionKey = m.Rows[m.Cursor].Group.SectionKey
	}
	if allExpanded {
		m.Folds.setSectionToggleAnchor(nil)
		if key, containsCursor := m.topLevelSectionAtCursor(sectionKeys); containsCursor {
			if row := m.Rows[m.Cursor]; row.IsTaskRow() && row.Task != nil {
				m.Folds.setSectionToggleAnchor(&sectionToggleRestoreAnchor{
					task:       m.TaskIdentityForRow(row),
					sectionKey: key,
				})
			}
			for _, section := range sections {
				m.setGroupFold(section, true)
			}
			m.RebuildRows()
			if !m.moveCursorToGroupKey(key) {
				m.ClampCursor()
			}
			return true
		}
		for _, section := range sections {
			m.setGroupFold(section, true)
		}
		m.RebuildRows()
		m.restoreModeCursor(cursorAnchor)
		return true
	}

	for _, section := range sections {
		m.setGroupFold(section, false)
	}
	m.RebuildRows()
	if m.restoreSectionToggleFocus() {
		return true
	}
	if selectedSectionKey != "" {
		if !m.moveCursorToGroupKey(selectedSectionKey) {
			m.ClampCursor()
		}
		return true
	}
	m.restoreModeCursor(cursorAnchor)
	return true
}

// toggleVisibleProjectGroups is p: it expands every visible project group when
// any is collapsed; otherwise it collapses them all. It is the third bulk fold
// beside s and t, and the only one that reaches the groups emitted in all seven
// modes — Today, Next, Inbox, and Backlog carry no section headers at all, so
// project groups are the whole structure there.
//
// It diverges from toggleVisibleSections twice, deliberately:
//
// No zoom refusal. sectionCollapsed exempts only m.focus.zoom.key, which is always a
// section key, and an admitted section still emits its project headers — so
// under a zoom p folds rows that are genuinely on screen. Refusing would be a
// lie.
//
// No restore anchor field. toggleVisibleSections carries
// sectionToggleRestoreAnchor because collapsing a section can hide a mode's worth
// of rows. A project fold hides one group, and the per-group memory every fold
// already writes is what puts that cursor back, so there is no second anchor.
func (m *OutlinerModel) ToggleVisibleProjectGroups() bool {
	if m == nil {
		return false
	}
	groups := m.VisibleProjectGroups()
	if len(groups) == 0 {
		return false
	}

	allExpanded := true
	for _, group := range groups {
		if m.IsProjectGroupFolded(group) {
			allExpanded = false
		}
	}

	cursorAnchor := m.captureModeCursor()
	// Same rule as s, one axis over, and read off the cursor row's own group rather
	// than off a header row: whatever the press leaves standing where the cursor was
	// belongs to that group, so the group is what the cursor rides through the
	// reshape. A collapse remembers the task it is hiding first, so the expansion
	// that follows can return to it rather than to whichever row leads the group.
	cursorGroup := OutlinerRowGroup{}
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) {
		cursorGroup = m.Rows[m.Cursor].Group
	}
	cursorTask, hasCursorTask := m.cursorTaskIdentity()
	if allExpanded {
		m.rememberCursorGroupFocus()
	}
	remembered, hasRemembered := m.Folds.groupFocus(cursorGroup.ProjectKey)
	for _, group := range groups {
		m.SetProjectGroupFolded(group, allExpanded)
	}
	m.RebuildRows()
	if cursorGroup.ProjectGrouped() {
		// An expansion keeps the row the cursor was on whenever that row survived,
		// because nothing about it moved out of view; only a collapse needs the
		// group's own landing.
		if !allExpanded && hasCursorTask && m.MoveCursorToIdentity(cursorTask) {
			return true
		}
		if m.restoreGroupCursor(cursorGroup, remembered, hasRemembered) {
			return true
		}
		if !m.moveCursorToGroupKey(cursorGroup.ProjectKey) {
			m.ClampCursor()
		}
		return true
	}
	m.restoreModeCursor(cursorAnchor)
	return true
}
