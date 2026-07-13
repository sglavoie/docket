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
// rebuildRows, rebuilds rows, then skips past any non-navigable row (separator
// or project sub-header) the cursor may have landed on after the row list is
// reconstructed.
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
	return m.crossSkipNonNavigableRows(prevCursor, nil)
}

// crossHandlePerspectiveJump handles P0-P9 direct perspective jumps for the
// cross-project outliner. It sets activePerspective, forces a rebuild via
// rebuildRows, then skips past any non-navigable row the cursor may have
// landed on.
func (m *OutlinerModel) crossHandlePerspectiveJump(perspKey int) (tea.Model, tea.Cmd) {
	prevCursor := m.cursor
	m.activePerspective = perspKey
	m.resetViewScrollOffset()
	m.rebuildRows()
	return m.crossSkipNonNavigableRows(prevCursor, nil)
}

// crossSkipNonNavigableRows advances the cursor past any contiguous
// non-navigable rows (separators and project sub-headers) after a cursor
// movement, ensuring the cursor only rests on section headers or task rows.
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

// crossHandleGoDrillDown handles the 'go' (g→o) two-key sequence in cross-project
// mode. When the cursor is on a task row it pushes a fresh temporary OutlinerModel
// for the row's owning project, initialized with OpenAtTask set to the selected
// task, via ShowOutlinerMsg{Temporary: true}. This leaves m.outliner untouched.
// Non-task rows (section headers, project sub-headers, separators) are silently
// ignored.
func (m *OutlinerModel) crossHandleGoDrillDown() (tea.Model, tea.Cmd) {
	if m.project != nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return m, nil
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() {
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
	if row.IsSeparator || row.IsProjectHeader {
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
	ledger := workspaceLedgerFromConfig(m.config)
	scope := storage.GlobalScope()

	selectNewest := ledger.SelectNewestUndo
	if redo {
		selectNewest = ledger.SelectNewestRedo
	}

	entry, ledgerHas, err := selectNewest(scope)
	if err != nil {
		m.setStatus(formatUndoRedoRestoreErrorMessage(crossRestoreLabel(redo), err))
		return
	}
	if !ledgerHas {
		m.setStatus(crossNothingMessage(redo))
		return
	}
	m.crossPerformLedgerRestore(ledger, entry, redo)
}

// crossPerformLedgerRestore reverses (or replays) the selected ledger entry, then
// reloads the touched projects into memory and repairs config task references.
// The ledger already wrote the correct bytes to disk, so the in-memory projects
// are reloaded rather than re-saved.
func (m *OutlinerModel) crossPerformLedgerRestore(ledger *storage.WorkspaceLedger, entry storage.LedgerEntry, redo bool) {
	scope := storage.GlobalScope()
	restore := ledger.Undo
	if redo {
		restore = ledger.Redo
	}
	actionDesc, err := restore(scope)
	if err != nil {
		m.setStatus(formatUndoRedoRestoreErrorMessage(crossRestoreLabel(redo), err))
		m.rebuildRows()
		return
	}
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
	_, err := repairConfigTaskRefsToLoadedProjects(m.configManager, m.config, restored)
	return err
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

func crossNothingMessage(redo bool) string {
	if redo {
		return "Nothing to redo"
	}
	return "Nothing to undo"
}

// workspaceLedgerFromConfig opens the workspace undo ledger the transactional
// commit boundary records to, sized from the configured history depth — the same
// construction newTUIAppRepository uses, so cross-project undo/redo reads exactly
// what the coordinator wrote.
func workspaceLedgerFromConfig(cfg *config.Config) *storage.WorkspaceLedger {
	depth := config.DefaultUndoHistoryDepth
	if cfg != nil && cfg.UndoHistoryDepth != nil {
		depth = *cfg.UndoHistoryDepth
	}
	dataDir := ""
	if cfg != nil {
		dataDir = cfg.DataDir
	}
	return storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), depth, []string{dataDir})
}
