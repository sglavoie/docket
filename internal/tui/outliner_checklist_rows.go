package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// checklistCursor names the checklist item the cursor rests on, together with
// the task and project that own it. The owner travels with the index because a
// checklist item has no identity of its own: it is addressed as a position
// inside one task.
type checklistCursor struct {
	Task    *models.Task
	Project *models.Project
	Index   int
}

// cursorChecklistItem reports the checklist row under the cursor. A checklist
// row carries its owner's task pointer, so callers that only ask for a task
// cannot tell the two apart; this is the one predicate that can.
func (m *OutlinerModel) cursorChecklistItem() (checklistCursor, bool) {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return checklistCursor{}, false
	}
	row := m.rows[m.Cursor]
	if !row.IsChecklistItem || row.task == nil {
		return checklistCursor{}, false
	}
	project := m.project
	if project == nil {
		project = m.crossResolveProject(row.ProjectID)
	}
	if project == nil {
		return checklistCursor{}, false
	}
	return checklistCursor{Task: row.task, Project: project, Index: row.ChecklistIndex}, true
}

// isChecklistExpanded reports whether the owner task's checklist items render as
// their own rows. Absent means collapsed, so the default projection is exactly
// the one that existed before checklist rows.
func (m *OutlinerModel) isChecklistExpanded(identity taskref.TaskIdentity) bool {
	return m != nil && m.folds.expandedChecklists[identity]
}

// expandChecklistRows splices each expanded task's checklist items in beneath
// their owner. It runs once over the finished projection rather than inside each
// mode strategy, so every mode gains checklist rows from one kernel.
//
// Visual selection suppresses them entirely: `v`, `V` and `Ctrl+A` already
// cannot select a non-task row, but cursor movement during a live range would
// still land on one.
func (m *OutlinerModel) expandChecklistRows() {
	if m == nil || len(m.folds.expandedChecklists) == 0 || m.MultiSelectMode {
		return
	}
	expanded := make([]outlinerRow, 0, len(m.rows))
	for _, row := range m.rows {
		expanded = append(expanded, row)
		if !row.isTaskRow() || row.task == nil || len(row.task.Checklist) == 0 {
			continue
		}
		identity := m.rowIdentity(row)
		if !m.isChecklistExpanded(identity) {
			continue
		}
		childrenFollow := row.hasVisibleChildren && m.isExpanded(identity)
		for index, item := range row.task.Checklist {
			child := row
			child.IsChecklistItem = true
			child.ChecklistIndex = index
			child.ChecklistDone = item.Done
			child.hasVisibleChildren = false
			child.Breadcrumb = ""
			child.prefix = checklistRowPrefix(
				row.prefix,
				index == len(row.task.Checklist)-1 && !childrenFollow,
			)
			expanded = append(expanded, child)
		}
	}
	m.rows = expanded
}

// checklistRowPrefix continues the owner's tree prefix one level deeper, turning
// the owner's own connector into the continuation glyph its descendants need.
func checklistRowPrefix(ownerPrefix string, last bool) string {
	base := ownerPrefix
	switch {
	case strings.HasSuffix(base, connectorLastItem):
		base = strings.TrimSuffix(base, connectorLastItem) + connectorSpace
	case strings.HasSuffix(base, connectorBranch):
		base = strings.TrimSuffix(base, connectorBranch) + connectorPipe
	}
	if last {
		return base + connectorLastItem
	}
	return base + connectorBranch
}

// syncChecklistSelectionRows re-projects the rows around a visual-selection
// boundary and re-seats the cursor on the owner task, because entering selection
// removes every checklist row the cursor may currently rest on and leaving it
// puts them back.
func (m *OutlinerModel) syncChecklistSelectionRows() {
	if m == nil || len(m.folds.expandedChecklists) == 0 {
		return
	}
	owner := taskref.TaskIdentity{}
	if m.Cursor >= 0 && m.Cursor < len(m.rows) {
		owner = m.taskIdentityForRow(m.rows[m.Cursor])
	}
	m.rebuildRows()
	if owner.Valid() {
		m.moveCursorToIdentity(owner)
	}
}
