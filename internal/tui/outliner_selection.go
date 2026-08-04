package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// taskOpsView implementation for OutlinerModel.

func (m *OutlinerModel) getTaskRef() (*models.Task, *models.Project) {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil, nil
	}

	row := m.rows[m.cursor]
	if !row.isTaskRow() {
		return nil, nil
	}
	if m.project != nil {
		return row.task, m.project
	}

	for _, proj := range m.loadedProjects {
		if proj.ID == row.ProjectID {
			return row.task, proj
		}
	}
	return nil, nil
}

func (m *OutlinerModel) setStatus(msg string) { m.InteractiveViewState.setStatus(msg) }

func (m *OutlinerModel) taskIdentityForRow(row outlinerRow) TaskIdentity {
	if row.task == nil {
		return TaskIdentity{}
	}
	project := m.project
	if project == nil {
		project = m.crossResolveProject(row.ProjectID)
	}
	return taskIdentityFor(row.task, project)
}

func (m *OutlinerModel) isRowSelected(row outlinerRow) bool {
	return m.IsTaskIdentitySelected(m.taskIdentityForRow(row))
}

// toggleCursorSelection starts a fresh inclusive Vim-style range at the
// current task row. Invoking v while a range is active freezes the current
// result; invoking it again starts a new range and replaces the old selection.
func (m *OutlinerModel) toggleCursorSelection() {
	if m.IsRangeSelectionActive() {
		m.FreezeRangeSelection()
		m.updateVisualSelectionStatus()
		return
	}
	if m.cursor < 0 || m.cursor >= len(m.rows) || !m.rows[m.cursor].isTaskRow() {
		return
	}
	m.peekMode = false
	m.StartRangeSelection(m.taskIdentityForRow(m.rows[m.cursor]))
	m.updateVisualSelectionStatus()
}

func (m *OutlinerModel) togglePeek() {
	if m == nil || m.multiSelectMode {
		return
	}
	if m.peekMode {
		m.peekMode = false
		m.resetViewScrollOffset()
		return
	}
	if task, _ := m.getTaskRef(); task == nil {
		return
	}
	m.peekMode = true
	m.resetViewScrollOffset()
}

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *OutlinerModel) toggleMultiSelectMode() {
	m.multiSelectMode = !m.multiSelectMode
	if !m.multiSelectMode {
		m.ClearSelection()
		m.clearStatus()
		return
	}

	m.peekMode = false
	m.updateVisualSelectionStatus()
}

func (m *OutlinerModel) updateVisualSelectionStatus() {
	if !m.multiSelectMode {
		return
	}
	counts := m.selectionCounts()
	if counts.Total > 0 {
		action := "v freeze · Esc freeze"
		if !m.IsRangeSelectionActive() {
			action = "v new range · Esc clear"
		}
		m.setStatus(fmt.Sprintf(
			"Visual selection: %d total · %d visible · %d hidden · %s · Space actions",
			counts.Total,
			counts.Visible,
			counts.Hidden,
			action,
		))
		return
	}
	m.setStatus("Visual selection: v new range · Ctrl+A all visible · Esc clear")
}

func (m *OutlinerModel) visibleTaskIdentities() []TaskIdentity {
	visible := make([]TaskIdentity, 0, len(m.rows))
	seen := make(map[TaskIdentity]bool, len(m.rows))
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		identity := m.taskIdentityForRow(row)
		if !identity.Valid() || seen[identity] {
			continue
		}
		seen[identity] = true
		visible = append(visible, identity)
	}
	return visible
}

func (m *OutlinerModel) selectionCounts() SelectionCounts {
	return m.ClassifySelection(m.visibleTaskIdentities())
}

// selectAllVisible replaces the selection with all currently visible task rows.
func (m *OutlinerModel) selectAllVisible() {
	m.peekMode = false
	m.ReplaceSelection(m.visibleTaskIdentities())
}

// updateRangeSelection recalculates the inclusive anchor-to-cursor range using
// visible task rows only. Headers and separators may lie inside the row span
// but never become selected targets.
func (m *OutlinerModel) updateRangeSelection() {
	if !m.IsRangeSelectionActive() || m.cursor < 0 || m.cursor >= len(m.rows) {
		return
	}
	anchorIndex := -1
	for index, row := range m.rows {
		if row.isTaskRow() && m.taskIdentityForRow(row) == m.rangeAnchor {
			anchorIndex = index
			break
		}
	}
	if anchorIndex < 0 {
		// A projection change can hide the anchor. Preserve the durable selection
		// and stop reshaping it against a different visible task set.
		m.FreezeRangeSelection()
		m.updateVisualSelectionStatus()
		return
	}
	start, end := anchorIndex, m.cursor
	if start > end {
		start, end = end, start
	}
	identities := make([]TaskIdentity, 0, end-start+1)
	for _, row := range m.rows[start : end+1] {
		if row.isTaskRow() && row.task != nil {
			identities = append(identities, m.taskIdentityForRow(row))
		}
	}
	m.replaceSelection(identities)
	m.updateVisualSelectionStatus()
}

// pruneInvisibleSelections intentionally retains valid project-qualified
// selections even when a filter, mode, or successful mutation hides their rows.
// Non-delete bulk actions preserve the frozen selection so a follow-up action
// never narrows silently to what happens to remain visible.
func (m *OutlinerModel) pruneInvisibleSelections() {
}

// pruneSelectionsOutsideLoadedProjects drops selected identities whose project
// the reload no longer admits. A project leaves the admitted set when it is
// parked while some other scope is in view, or when a scope that had admitted it
// is left; either way the Outliner has no route back to those tasks, so keeping
// them selected would let a bulk action reach outside the visible workspace.
// This is narrower than hiding: a filter or mode change still preserves the
// frozen selection, because those keep the owning project admitted.
func (m *OutlinerModel) pruneSelectionsOutsideLoadedProjects() {
	if m.SelectedCount() == 0 {
		return
	}
	admitted := make(map[string]bool, len(m.loadedProjects))
	for _, project := range m.loadedProjects {
		if project != nil {
			admitted[project.ID] = true
		}
	}
	for _, identity := range m.SelectedTaskIdentities() {
		if !admitted[identity.ProjectID] {
			m.DeselectTaskIdentity(identity)
		}
	}
	if m.IsRangeSelectionActive() && !admitted[m.rangeAnchor.ProjectID] {
		m.FreezeRangeSelection()
	}
}

// clearMultiSelect exits multi-select mode and clears all selections.
func (m *OutlinerModel) clearMultiSelect() {
	m.ExitMultiSelect()
	m.clearStatus()
}
