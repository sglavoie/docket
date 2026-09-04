package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// taskOpsView implementation for OutlinerModel.

func (m *OutlinerModel) GetTaskRef() (*models.Task, *models.Project) {
	if len(m.rows) == 0 || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return nil, nil
	}

	row := m.rows[m.Cursor]
	if !row.isTaskRow() {
		return nil, nil
	}
	if m.project != nil {
		return row.task, m.project
	}

	for _, proj := range m.population.projects {
		if proj.ID == row.ProjectID {
			return row.task, proj
		}
	}
	return nil, nil
}

// cursorProjectHeaderProject returns the project a project sub-header cursor
// names. Naming it is what lets a project operation resolve from a header row;
// task operations still find no task there and refuse with the shared sentence.
// A section header names no project, however it groups its rows.
func (m *OutlinerModel) cursorProjectHeaderProject() *models.Project {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return nil
	}
	row := m.rows[m.Cursor]
	if !row.IsProjectHeader || row.ProjectID == "" {
		return nil
	}
	if m.project != nil && m.project.ID == row.ProjectID {
		return m.project
	}
	return m.crossResolveProject(row.ProjectID)
}

func (m *OutlinerModel) setStatus(msg string) { m.SetStatus(msg) }

func (m *OutlinerModel) taskIdentityForRow(row outlinerRow) taskref.TaskIdentity {
	if row.task == nil {
		return taskref.TaskIdentity{}
	}
	project := m.project
	if project == nil {
		project = m.crossResolveProject(row.ProjectID)
	}
	return taskref.For(row.task, project)
}

func (m *OutlinerModel) isRowSelected(row outlinerRow) bool {
	return m.IsTaskIdentitySelected(m.taskIdentityForRow(row))
}

// toggleCursorSelection flips the cursor task's membership, so a selection can
// name rows that are not adjacent. A live range is frozen first: otherwise the
// next cursor move would recompute the range and discard the toggle.
func (m *OutlinerModel) toggleCursorSelection() {
	if m.Cursor < 0 || m.Cursor >= len(m.rows) || !m.rows[m.Cursor].isTaskRow() {
		return
	}
	m.FreezeRangeSelection()
	m.peekMode = false
	m.MultiSelectMode = true
	m.ToggleTaskIdentity(m.taskIdentityForRow(m.rows[m.Cursor]))
	m.syncChecklistSelectionRows()
	m.updateVisualSelectionStatus()
}

// startCursorRangeSelection starts a fresh inclusive Vim-style range at the
// current task row. Invoking V while a range is active freezes the current
// result; invoking it again starts a new range and replaces the old selection.
func (m *OutlinerModel) startCursorRangeSelection() {
	if m.IsRangeSelectionActive() {
		m.FreezeRangeSelection()
		m.updateVisualSelectionStatus()
		return
	}
	if m.Cursor < 0 || m.Cursor >= len(m.rows) || !m.rows[m.Cursor].isTaskRow() {
		return
	}
	m.peekMode = false
	m.StartRangeSelection(m.taskIdentityForRow(m.rows[m.Cursor]))
	m.syncChecklistSelectionRows()
	m.updateVisualSelectionStatus()
}

func (m *OutlinerModel) togglePeek() {
	if m == nil || m.MultiSelectMode {
		return
	}
	if m.peekMode {
		m.peekMode = false
		return
	}
	if task, _ := m.GetTaskRef(); task == nil {
		return
	}
	// Keep the scroll offset: the viewport clamp then scrolls the minimum needed
	// to fit Peek, instead of snapping the cursor to the last visible row.
	m.peekMode = true
}

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *OutlinerModel) toggleMultiSelectMode() {
	m.MultiSelectMode = !m.MultiSelectMode
	if !m.MultiSelectMode {
		m.ClearSelection()
		m.syncChecklistSelectionRows()
		m.ClearStatus()
		return
	}

	m.peekMode = false
	m.syncChecklistSelectionRows()
	m.updateVisualSelectionStatus()
}

func (m *OutlinerModel) updateVisualSelectionStatus() {
	if !m.MultiSelectMode {
		return
	}
	counts := m.selectionCounts()
	if counts.Total > 0 {
		action := "V freeze · Esc freeze"
		if !m.IsRangeSelectionActive() {
			action = "v toggle · V range · Esc clear"
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
	m.setStatus("Visual selection: v toggle · V range · Ctrl+A all visible · Esc clear")
}

func (m *OutlinerModel) visibleTaskIdentities() []taskref.TaskIdentity {
	visible := make([]taskref.TaskIdentity, 0, len(m.rows))
	seen := make(map[taskref.TaskIdentity]bool, len(m.rows))
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

func (m *OutlinerModel) selectionCounts() listsurface.SelectionCounts {
	return m.ClassifySelection(m.visibleTaskIdentities())
}

// selectAllVisible replaces the selection with all currently visible task rows.
func (m *OutlinerModel) selectAllVisible() {
	m.peekMode = false
	m.ReplaceSelection(m.visibleTaskIdentities())
	m.syncChecklistSelectionRows()
}

// updateRangeSelection recalculates the inclusive anchor-to-cursor range using
// visible task rows only. Headers and separators may lie inside the row span
// but never become selected targets.
func (m *OutlinerModel) updateRangeSelection() {
	if !m.IsRangeSelectionActive() || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return
	}
	anchorIndex := -1
	for index, row := range m.rows {
		if row.isTaskRow() && m.taskIdentityForRow(row) == m.RangeAnchor {
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
	start, end := anchorIndex, m.Cursor
	if start > end {
		start, end = end, start
	}
	identities := make([]taskref.TaskIdentity, 0, end-start+1)
	for _, row := range m.rows[start : end+1] {
		if row.isTaskRow() && row.task != nil {
			identities = append(identities, m.taskIdentityForRow(row))
		}
	}
	m.SetSelection(identities)
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
	admitted := make(map[string]bool, len(m.population.projects))
	for _, project := range m.population.projects {
		if project != nil {
			admitted[project.ID] = true
		}
	}
	for _, identity := range m.SelectedTaskIdentities() {
		if !admitted[identity.ProjectID] {
			m.DeselectTaskIdentity(identity)
		}
	}
	if m.IsRangeSelectionActive() && !admitted[m.RangeAnchor.ProjectID] {
		m.FreezeRangeSelection()
	}
}

// clearMultiSelect exits multi-select mode and clears all selections.
func (m *OutlinerModel) clearMultiSelect() {
	m.ExitMultiSelect()
	m.ClearStatus()
}
