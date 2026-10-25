package outliner

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/taskref"
)

const (
	pinnedSectionName                  = "Pinned"
	pinnedSectionCollapseKey           = "_synthetic:pinned"
	pinnedProjectSectionCollapsePrefix = "_pinned_project:"
)

// pinnedSectionBuild carries the transient Pinned project groups and the
// qualified identity set used to suppress those same tasks from the active mode
// below.
type pinnedSectionBuild struct {
	groups    []projectRowGroup
	count     int
	projected map[taskref.TaskIdentity]bool
}

// buildPinnedSection realizes Pinned from the same scoped candidates, reference
// time, and session filters as the active mode. Only modes declaring
// ShowsPinned call it. Projected relatives form a
// section-local tree; roots whose parent is outside Pinned keep their ancestor
// breadcrumb. Project groups get a synthetic header in the unscoped Outliner.
// Rows follow compareSectionTasks with pin order breaking ties, and a root
// ranks by its own position rather than its subtree's earliest one, so J/K
// swapping two adjacent rows moves exactly those rows.
func (m *OutlinerModel) buildPinnedSection(input OutlinerModeInput) pinnedSectionBuild {
	if input.TaskFilter == nil {
		input.TaskFilter = m.workModeTaskFilterAt(input.Now)
	}
	projection := buildPinnedModeProjection(input)
	rows := make([]OutlinerRow, 0, len(projection.rows))
	for _, modeRow := range projection.rows {
		row := m.outlinerRowFromModeRow(modeRow)
		row.HasVisibleChildren = false
		row.IsPinnedSection = true
		rows = append(rows, row)
	}
	m.sortSectionRows(rows)
	return pinnedSectionBuild{
		groups: m.groupSectionRowsByProject(rows, sectionForestOptions{}, func(a, b int) int {
			if c := m.compareSectionTasks(rows[a].Task, rows[b].Task); c != 0 {
				return c
			}
			return comparePinnedAt(rows[a].Task, rows[b].Task)
		}),
		count:     len(projection.rows),
		projected: projection.projected,
	}
}

func pinnedProjectSectionCollapseKey(projectID string) string {
	return pinnedProjectSectionCollapsePrefix + projectID
}

func (m *OutlinerModel) appendPinnedSection(section pinnedSectionBuild) {
	if section.count == 0 || !m.sectionZoomAdmits(pinnedSectionCollapseKey) {
		return
	}
	m.Rows = append(m.Rows, OutlinerRow{
		Group:           OutlinerRowGroup{SectionKey: pinnedSectionCollapseKey},
		IsSectionHeader: true,
		IsPinnedSection: true,
		SectionName:     pinnedSectionName,
		SectionCount:    section.count,
		CollapseKey:     pinnedSectionCollapseKey,
	})
	if m.sectionCollapsed(pinnedSectionCollapseKey) {
		m.hideProjectRowGroups(section.groups)
		return
	}

	m.appendProjectGroupedForests(
		section.groups,
		OutlinerRow{IsPinnedSection: true},
		pinnedSectionCollapseKey,
		pinnedProjectSectionCollapseKey,
	)
}

// isPinnedReorderRow reports whether J/K reorders the pin order rather than
// task hierarchy or a durable planning list. A pinned row answers before the
// active mode's own ordering, so Today's durable plan order does not claim it.
func (m *OutlinerModel) IsPinnedReorderRow() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	if m.MultiSelectMode {
		return false
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil || !row.IsPinnedSection {
		return false
	}
	return m.ResolveProjectForRow(row) != nil
}

// pinnedSectionParentAt returns the section-local parent of the pinned row at
// index, or the zero identity when the row sits at the section's root level.
// It reproduces the edge rule buildSectionRowForestWith applies: a parent link is
// retained only when the parent is itself projected into the same group, which
// the row's own group identity answers without reading a header. The backward
// scan is sufficient because the flattener always emits a parent above its
// children and only descends into expanded parents.
func (m *OutlinerModel) pinnedSectionParentAt(index int) taskref.TaskIdentity {
	if index < 0 || index >= len(m.Rows) {
		return taskref.TaskIdentity{}
	}
	row := m.Rows[index]
	if row.Task == nil || row.Task.ParentID == nil {
		return taskref.TaskIdentity{}
	}
	parent := taskref.TaskIdentity{
		ProjectID: m.TaskIdentityForRow(row).ProjectID,
		TaskID:    *row.Task.ParentID,
	}
	for i := index - 1; i >= 0; i-- {
		candidate := m.Rows[i]
		if !candidate.IsTaskRow() || candidate.Task == nil || candidate.Group != row.Group {
			return taskref.TaskIdentity{}
		}
		if m.TaskIdentityForRow(candidate) == parent {
			return parent
		}
	}
	return taskref.TaskIdentity{}
}

// visiblePinnedSiblings returns every visible pinned row index in the delta
// direction that shares the cursor's section-local parent, nearest first. Rows
// belonging to a sibling's expanded subtree are stepped over; anything that is
// not a task row and any task row outside the cursor's group bounds the search,
// so a move can never leave the cursor's Pinned project group. A sibling the
// active order separates from the cursor bounds it too: pin order only breaks
// ties, so a swap across that boundary would move no row.
func (m *OutlinerModel) visiblePinnedSiblings(delta int) []int {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return nil
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	cursorRow := m.Rows[m.Cursor]
	cursorParent := m.pinnedSectionParentAt(m.Cursor)
	var siblings []int
	for i := m.Cursor + step; i >= 0 && i < len(m.Rows); i += step {
		row := m.Rows[i]
		if !row.IsTaskRow() || row.Task == nil || row.Group != cursorRow.Group {
			break
		}
		if m.pinnedSectionParentAt(i) == cursorParent {
			if m.compareSectionTasks(row.Task, cursorRow.Task) != 0 {
				break
			}
			siblings = append(siblings, i)
		}
	}
	return siblings
}

// adjacentVisiblePinnedSibling returns the index of the nearest visible pinned
// sibling in the delta direction.
func (m *OutlinerModel) adjacentVisiblePinnedSibling(delta int) (int, bool) {
	siblings := m.visiblePinnedSiblings(delta)
	if len(siblings) == 0 {
		return 0, false
	}
	return siblings[0], true
}

// pinnedReorderMove swaps the cursor's pin order with the adjacent visible
// pinned sibling. The identity that must end up displayed first is passed
// first: pinned rows sort PinnedAt-descending and the application layer
// resolves an exact tie in favor of its first argument.
func (m *OutlinerModel) PinnedReorderMove(delta int) tea.Cmd {
	if !m.IsPinnedReorderRow() || !m.taskItem.Wired() {
		return nil
	}
	targetIndex, ok := m.adjacentVisiblePinnedSibling(delta)
	if !ok {
		return nil
	}
	cursor := m.TaskIdentityForRow(m.Rows[m.Cursor])
	target := m.TaskIdentityForRow(m.Rows[targetIndex])
	if !cursor.Valid() || !target.Valid() {
		return nil
	}
	if delta < 0 {
		return m.taskItem.ReorderPinned(cursor, target)
	}
	return m.taskItem.ReorderPinned(target, cursor)
}

// pinnedReorderEdge sends the cursor's pin to the far end of its visible pinned
// siblings in one transaction. The order handed down is display order, and it
// names only the peers the cursor would have swapped past one at a time, so the
// redistributed timestamps stay inside that run.
func (m *OutlinerModel) PinnedReorderEdge(delta int) tea.Cmd {
	if !m.IsPinnedReorderRow() || !m.taskItem.Wired() {
		return nil
	}
	rows := m.visiblePinnedSiblings(delta)
	if len(rows) == 0 {
		return nil
	}
	peers := make([]taskref.TaskIdentity, 0, len(rows))
	for _, index := range rows {
		peers = append(peers, m.TaskIdentityForRow(m.Rows[index]))
	}
	moved := m.TaskIdentityForRow(m.Rows[m.Cursor])
	order := edgeMoveOrder(moved, peers, delta)
	for _, identity := range order {
		if !identity.Valid() {
			return nil
		}
	}
	return m.taskItem.ReorderPinnedOrder(moved, order, reorderEdgeLabel(delta))
}
