package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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

// crossHandlePerspectiveCycle handles h/l perspective cycling for the
// cross-project outliner. It captures the selected task identity, changes
// activePerspective, rebuilds rows, then restores the cursor onto the same task
// (or the deterministic nearest-row fallback) so a task that stays eligible
// across the differently shaped perspective keeps focus and scroll position.
func (m *OutlinerModel) crossHandlePerspectiveCycle(key string) (tea.Model, tea.Cmd) {
	delta := 0
	switch key {
	case "h":
		delta = -1
	case "l":
		delta = 1
	default:
		return m, nil
	}
	mode, ok := OutlinerModeForPerspective(CyclePerspective(m.activePerspective, delta))
	if !ok {
		return m, nil
	}
	return m.selectOutlinerMode(mode.ID)
}

// crossHandlePerspectiveJump jumps the cross-project outliner directly to a
// perspective-backed mode (from the mode picker or Tab/h/l cycling). It captures
// the selected task identity before rebuilding and restores it afterward, so the
// cursor tracks the task across the reshape rather than a bare numeric row index.
func (m *OutlinerModel) crossHandlePerspectiveJump(perspID int) (tea.Model, tea.Cmd) {
	mode, ok := OutlinerModeForPerspective(perspID)
	if !ok {
		return m, nil
	}
	return m.selectOutlinerMode(mode.ID)
}

// crossSkipNonNavigableRows advances the cursor past any contiguous
// non-navigable rows (separators only; project sub-headers are now navigable)
// after a cursor movement, ensuring the cursor never rests on a separator.
//
// It uses prevCursor to determine the direction of travel:
//   - Moving up (cursor < prevCursor): walks backward until a navigable row or
//     index 0 is reached, handling non-navigable rows at the top.
//   - Moving down (cursor >= prevCursor): walks forward until a navigable row
//     is found, then clamps to the last row if the end is overshot, handling
//     non-navigable rows at the bottom.
//
// Multiple consecutive non-navigable rows are handled because each iteration
// of the loop advances one row, so the loop continues until a navigable row
// is found or a boundary is hit.
//
// cmd is the Cmd returned by handleListNavigation and is passed through
// unchanged so the caller can return it directly.
func (m *OutlinerModel) crossSkipNonNavigableRows(prevCursor int, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if len(m.rows) == 0 {
		return m, cmd
	}

	if m.cursor < prevCursor {
		// Moving upward: skip backward past any non-navigable rows.
		for m.cursor > 0 && !m.rows[m.cursor].isNavigable() {
			m.cursor--
		}
		// If we stopped at index 0 and it is still non-navigable, recover
		// by moving forward to the first navigable row.
		if !m.rows[m.cursor].isNavigable() {
			for m.cursor < len(m.rows)-1 && !m.rows[m.cursor].isNavigable() {
				m.cursor++
			}
		}
	} else {
		// Moving downward (or no change): skip forward past any non-navigable rows.
		for m.cursor < len(m.rows) && !m.rows[m.cursor].isNavigable() {
			m.cursor++
		}
		// Clamp to the last row when non-navigable rows run to the end of the list.
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		// If the clamped row is itself non-navigable, walk backward to the
		// nearest navigable row (mirrors the upward-boundary recovery).
		if !m.rows[m.cursor].isNavigable() {
			for m.cursor > 0 && !m.rows[m.cursor].isNavigable() {
				m.cursor--
			}
		}
	}

	return m, cmd
}

// crossHandleEnterKey handles the Enter key in cross-project mode. For leaf
// task rows it resolves the owning project from row.ProjectID and emits
// ShowTaskDetailMsg to open the task detail via glow. Section headers toggle
// section fold; separators and project sub-headers are silently consumed.
//
// In cross-project Priority mode (m.crossPriorityMode), any task row — leaf
// or parent — opens detail to match single-project Priority View behavior.
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
	if row.IsProjectHeader {
		// Enter on a focusable project header pushes that project's scope,
		// narrowing the global data set to one project in place.
		m.pushProjectScope(row.ProjectID)
		return m, nil, true
	}
	if row.IsSeparator {
		return m, nil, true
	}
	if row.IsSectionHeader {
		m.toggleSectionFoldAtCursor()
		return m, nil, true
	}
	task := row.task
	// Parent tasks: zoom in (same as project-specific outliner Enter behavior).
	// Priority mode skips the zoom branch so parent rows open detail like the
	// single-project Priority View.
	if !m.crossPriorityMode && len(task.Children) > 0 {
		m.crossZoomStack = append(m.crossZoomStack, CrossProjectZoomEntry{
			TaskID:      task.ID,
			ProjectID:   row.ProjectID,
			CursorIndex: m.cursor,
		})
		m.cursor = 0
		m.resetViewScrollOffset()
		return m, nil, true
	}
	// Leaf tasks (and any task row in Priority mode): open detail view.
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

// crossHandleUndo handles the 'u' key in cross-project mode. Undo is
// ledger-only: the newest workspace-ledger transaction is reversed. Every
// cross-project mutation (bulk and single-task alike) now records there through
// the application layer, so there is no legacy per-project fallback.
//
// Returns (model, cmd, true) when in cross-project mode (m.project == nil),
// (nil, nil, false) otherwise.
func (m *OutlinerModel) crossHandleUndo(key string) (tea.Model, tea.Cmd, bool) {
	if m.project != nil || key != "u" {
		return nil, nil, false
	}
	m.crossRestore(false)
	return m, nil, true
}

// crossHandleRedo handles the 'ctrl+r' key in cross-project mode. Like undo it
// restores purely from the workspace ledger.
//
// Returns (model, cmd, true) when in cross-project mode (m.project == nil),
// (nil, nil, false) otherwise.
func (m *OutlinerModel) crossHandleRedo(key string) (tea.Model, tea.Cmd, bool) {
	if m.project != nil || key != "ctrl+r" {
		return nil, nil, false
	}
	m.crossRestore(true)
	return m, nil, true
}

// crossRestore drives the ledger-only restore for both undo (redo=false) and
// redo (redo=true). It rebuilds the rows once the restore settles.
func (m *OutlinerModel) crossRestore(redo bool) {
	ledger := workspaceLedgerFromConfig(m.runtime)
	scope := storage.GlobalScope()
	restore := ledger.UndoWithEntry
	if redo {
		restore = ledger.RedoWithEntry
	}
	entry, actionDesc, err := restore(scope)
	if err != nil {
		m.setStatus(formatUndoRedoRestoreErrorMessage(crossRestoreLabel(redo), err))
		return
	}
	m.crossPerformLedgerRestore(entry, actionDesc, redo)
}

// crossPerformLedgerRestore reloads the touched projects after the ledger has
// atomically restored the entry and moved it between stacks.
func (m *OutlinerModel) crossPerformLedgerRestore(entry storage.LedgerEntry, actionDesc string, redo bool) {
	if reloadErr := m.crossReloadLedgerRestoredProjects(entry.ProjectIDs); reloadErr != nil {
		m.setStatus(fmt.Sprintf("%s completed, refresh failed: %s", crossRestoreVerb(redo), reloadErr))
	} else {
		m.setStatus(fmt.Sprintf("%s: %s", crossRestoreVerb(redo), actionDesc))
	}
	m.rebuildRows()
}

// crossReloadLedgerRestoredProjects reloads every loaded project the restored
// transaction touched and repairs focus-queue / daily-plan / recent-visit refs.
func (m *OutlinerModel) crossReloadLedgerRestoredProjects(projectIDs []string) error {
	restored := make([]*models.Project, 0, len(projectIDs))
	for _, id := range projectIDs {
		proj := m.crossResolveProject(id)
		if proj == nil {
			continue
		}
		if err := storage.LoadProject(proj); err != nil {
			return err
		}
		restored = append(restored, proj)
	}
	return repairRuntimeTaskRefsAfterRestore(m.runtime)(restored)
}

// crossRestoreLabel and crossRestoreVerb produce the short label used by the
// shared error formatter and the capitalized verb used in the status line.
func crossRestoreLabel(redo bool) string {
	if redo {
		return "redo"
	}
	return "undo"
}

func crossRestoreVerb(redo bool) string {
	if redo {
		return "Redo"
	}
	return "Undid"
}

// effectiveUndoHistoryDepth resolves the persisted optional setting to the
// concrete retention policy used by a workspace ledger.
func effectiveUndoHistoryDepth(settings *config.Settings) int {
	if settings != nil && settings.UndoHistoryDepth != nil {
		return *settings.UndoHistoryDepth
	}
	return config.DefaultUndoHistoryDepth
}

// workspaceLedgerFromConfig opens a short-lived workspace undo ledger sized
// from the current runtime settings. Interactive composition owns a shared
// handle instead; this factory remains for startup/test mutation paths and
// restore-only surfaces such as cross-project undo/redo.
func workspaceLedgerFromConfig(runtime *config.RuntimeState) *storage.WorkspaceLedger {
	dataDir := ""
	var settings *config.Settings
	if runtime != nil {
		dataDir = runtime.DataDir()
		settings = runtime.Settings()
	}
	depth := effectiveUndoHistoryDepth(settings)
	return storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), depth, []string{dataDir})
}
