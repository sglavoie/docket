package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
)

// crossHandleContextPickerKey handles the C key in cross-project mode,
// emitting ShowContextPickerMsg{} to open the context picker. It is called
// early in the key dispatch so that C works even when m.rows is empty (the
// empty-state view rendered when no projects exist in the current context).
//
// Returns (model, cmd, true) in cross-project mode (m.project == nil),
// (nil, nil, false) otherwise.
func (m *OutlinerModel) crossHandleContextPickerKey(key string) (tea.Model, tea.Cmd, bool) {
	if m.project != nil || key != "C" {
		return nil, nil, false
	}
	return m, func() tea.Msg {
		return ShowContextPickerMsg{}
	}, true
}

// crossInterceptCreationKeys intercepts 'a' (normal mode only), 'i', and 'I'
// in the cross-project outliner and shows a status message directing the user
// to use 'go' to open the single-project outliner for task creation.
//
// Returns (model, cmd, true) when the key is consumed, (nil, nil, false)
// otherwise. The caller must place this check after the multi-select block so
// that 'a' in multi-select mode still triggers select-all.
func (m *OutlinerModel) crossInterceptCreationKeys(key string) (tea.Model, tea.Cmd, bool) {
	if m.project != nil {
		return nil, nil, false
	}
	switch key {
	case "a", "i", "I":
		m.setStatus("use go to open project outliner for task creation")
		return m, nil, true
	}
	return nil, nil, false
}

// crossHandlePerspectiveCycle handles h/l perspective cycling for the
// cross-project outliner. It changes activePerspective, sets forceRebuild via
// rebuildRows, rebuilds rows, then skips past any separator the cursor may
// have landed on after the row list is reconstructed.
func (m *OutlinerModel) crossHandlePerspectiveCycle(key string) (tea.Model, tea.Cmd) {
	perspCount := len(PerspectivePresets())
	prevCursor := m.cursor
	switch key {
	case "h":
		m.activePerspective = (m.activePerspective + perspCount - 1) % perspCount
	case "l":
		m.activePerspective = (m.activePerspective + 1) % perspCount
	}
	m.resetViewScrollOffset()
	m.rebuildRows()
	return m.crossSkipSeparators(prevCursor, nil)
}

// crossHandlePerspectiveJump handles P0-P9 direct perspective jumps for the
// cross-project outliner. It sets activePerspective, forces a rebuild via
// rebuildRows, then skips past any separator the cursor may have landed on.
func (m *OutlinerModel) crossHandlePerspectiveJump(perspKey int) (tea.Model, tea.Cmd) {
	prevCursor := m.cursor
	m.activePerspective = perspKey
	m.resetViewScrollOffset()
	m.rebuildRows()
	return m.crossSkipSeparators(prevCursor, nil)
}

// crossSkipSeparators advances the cursor past any contiguous IsSeparator rows
// after a cursor movement, ensuring the cursor never lands on a separator.
//
// It uses prevCursor to determine the direction of travel:
//   - Moving up (cursor < prevCursor): walks backward until a non-separator or
//     index 0 is reached, handling separators at the top.
//   - Moving down (cursor >= prevCursor): walks forward until a non-separator
//     is found, then clamps to the last row if the end is overshot, handling
//     separators at the bottom.
//
// Multiple consecutive separators are handled because each iteration of the
// loop advances one row, so the loop continues until a non-separator is found
// or a boundary is hit.
//
// cmd is the Cmd returned by handleListNavigation and is passed through
// unchanged so the caller can return it directly.
func (m *OutlinerModel) crossSkipSeparators(prevCursor int, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if len(m.rows) == 0 {
		return m, cmd
	}

	if m.cursor < prevCursor {
		// Moving upward: skip backward past any separators.
		for m.cursor > 0 && m.rows[m.cursor].IsSeparator {
			m.cursor--
		}
		// If we stopped at index 0 and it is still a separator, recover
		// by moving forward to the first non-separator row.
		if m.rows[m.cursor].IsSeparator {
			for m.cursor < len(m.rows)-1 && m.rows[m.cursor].IsSeparator {
				m.cursor++
			}
		}
	} else {
		// Moving downward (or no change): skip forward past any separators.
		for m.cursor < len(m.rows) && m.rows[m.cursor].IsSeparator {
			m.cursor++
		}
		// Clamp to the last row when separators run to the end of the list.
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		// If the clamped row is itself a separator, walk backward to the
		// nearest non-separator (mirrors the upward-boundary recovery).
		if m.rows[m.cursor].IsSeparator {
			for m.cursor > 0 && m.rows[m.cursor].IsSeparator {
				m.cursor--
			}
		}
	}

	return m, cmd
}

// crossHandleGoDrillDown handles the 'go' (g→o) two-key sequence in cross-project
// mode. When the cursor is on a task row it pushes a fresh temporary OutlinerModel
// for the row's owning project, initialized with OpenAtTask set to the selected
// task, via ShowOutlinerMsg{Temporary: true}. This leaves m.outliner untouched.
// Non-task rows (section headers, separators) are silently ignored.
func (m *OutlinerModel) crossHandleGoDrillDown() (tea.Model, tea.Cmd) {
	if m.project != nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return m, nil
	}
	row := m.rows[m.cursor]
	if row.IsSeparator || row.IsSectionHeader {
		return m, nil
	}
	for _, p := range m.loadedProjects {
		if p.ID == row.ProjectID {
			proj := p
			task := row.task
			return m, func() tea.Msg {
				return ShowOutlinerMsg{Project: proj, OpenAtTask: task, Temporary: true}
			}
		}
	}
	return m, nil
}

// crossHandleEnterKey handles the Enter key in cross-project mode. For leaf
// task rows it resolves the owning project from row.ProjectID and emits
// ShowTaskDetailMsg to open the task detail via glow. Section headers and
// separators are silently ignored.
//
// Returns (model, cmd, true) when in cross-project mode (m.project == nil),
// (nil, nil, false) otherwise.
func (m *OutlinerModel) crossHandleEnterKey() (tea.Model, tea.Cmd, bool) {
	if m.project != nil {
		return nil, nil, false
	}
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return m, nil, true
	}
	row := m.rows[m.cursor]
	if row.IsSeparator {
		return m, nil, true
	}
	if row.IsSectionHeader {
		m.toggleSectionFoldAtCursor()
		return m, nil, true
	}
	task := row.task
	// Parent tasks: zoom in (same as project-specific outliner Enter behavior).
	if len(task.Children) > 0 {
		m.crossZoomStack = append(m.crossZoomStack, CrossProjectZoomEntry{
			TaskID:      task.ID,
			ProjectID:   row.ProjectID,
			CursorIndex: m.cursor,
		})
		m.cursor = 0
		m.resetViewScrollOffset()
		return m, nil, true
	}
	// Leaf tasks: open detail view.
	for _, p := range m.loadedProjects {
		if p.ID == row.ProjectID {
			proj := p
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: proj}
			}, true
		}
	}
	return m, nil, true
}

// crossHandleUndo handles the 'u' key in cross-project mode. For single-project
// operations it uses snapshot-based undo (storage.PerformUndo) which enables
// redo. For multi-project operations it falls back to backup-based restore.
//
// Returns (model, cmd, true) when in cross-project mode (m.project == nil),
// (nil, nil, false) otherwise.
func (m *OutlinerModel) crossHandleUndo(key string) (tea.Model, tea.Cmd, bool) {
	if m.project != nil || key != "u" {
		return nil, nil, false
	}

	ids := m.crossUndoCtx.ProjectIDs

	// No pending undo context — try snapshot-based undo on any project that
	// has undo history. This handles the case where the user presses u
	// multiple times (crossUndoCtx was already cleared by the first undo).
	if len(ids) == 0 {
		// Check all loaded projects for undo history.
		for _, proj := range m.loadedProjects {
			if storage.UndoHistoryExists(proj) {
				actionDesc, err := storage.PerformUndo(proj)
				if err != nil {
					m.setStatus(fmt.Sprintf("Undo failed: %s", err))
				} else {
					_ = m.projectService.Load(proj)
					m.setStatus(fmt.Sprintf("Undid: %s", actionDesc))
				}
				m.rebuildRows()
				return m, nil, true
			}
		}
		m.setStatus("Nothing to undo")
		return m, nil, true
	}

	// Single-project operation: use snapshot-based undo for redo support.
	if len(ids) == 1 {
		proj := m.crossResolveProject(ids[0])
		if proj == nil {
			m.setStatus("Undo failed: project not found")
			m.crossUndoCtx.Clear()
			m.rebuildRows()
			return m, nil, true
		}
		actionDesc, err := storage.PerformUndo(proj)
		if err != nil {
			// Fallback to backup-based restore.
			if backupErr := m.projectService.RestoreFromBackup(proj); backupErr != nil {
				m.setStatus(fmt.Sprintf("Undo failed: %s", backupErr))
			} else {
				_ = m.projectService.Load(proj)
				m.setStatus(fmt.Sprintf("Undo: restored %s", proj.Name))
			}
		} else {
			_ = m.projectService.Load(proj)
			m.setStatus(fmt.Sprintf("Undid: %s", actionDesc))
		}
		m.crossUndoCtx.Clear()
		m.rebuildRows()
		return m, nil, true
	}

	// Multi-project operation: backup-based restore (no redo support).
	var restored []string
	var failed []string

	for i := len(ids) - 1; i >= 0; i-- {
		proj := m.crossResolveProject(ids[i])
		if proj == nil {
			failed = append(failed, fmt.Sprintf("%s (project not found)", ids[i]))
			continue
		}
		if err := m.projectService.RestoreFromBackup(proj); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%s)", proj.Name, err.Error()))
			continue
		}
		_ = m.projectService.Load(proj)
		restored = append(restored, proj.Name)
	}

	m.crossUndoCtx.Clear()

	var parts []string
	if len(restored) > 0 {
		parts = append(parts, "Undo: restored "+strings.Join(restored, ", "))
	}
	if len(failed) > 0 {
		parts = append(parts, "FAILED: "+strings.Join(failed, ", "))
	}
	if len(parts) > 0 {
		m.setStatus(strings.Join(parts, "; "))
	}

	m.rebuildRows()
	return m, nil, true
}

// crossHandleRedo handles the 'ctrl+r' key in cross-project mode. It performs
// snapshot-based redo on the most recently affected project.
//
// Returns (model, cmd, true) when in cross-project mode (m.project == nil),
// (nil, nil, false) otherwise.
func (m *OutlinerModel) crossHandleRedo(key string) (tea.Model, tea.Cmd, bool) {
	if m.project != nil || key != "ctrl+r" {
		return nil, nil, false
	}

	// Check all loaded projects for redo history.
	for _, proj := range m.loadedProjects {
		if storage.RedoHistoryExists(proj) {
			actionDesc, err := storage.PerformRedo(proj)
			if err != nil {
				m.setStatus(fmt.Sprintf("Redo failed: %s", err))
			} else {
				_ = m.projectService.Load(proj)
				m.setStatus(fmt.Sprintf("Redo: %s", actionDesc))
			}
			m.rebuildRows()
			return m, nil, true
		}
	}

	m.setStatus("Nothing to redo")
	return m, nil, true
}
