package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// shellOwnedPaletteAction routes the three Task Workspace grammar actions that
// AppModel intercepts before any view dispatcher: Peek (Shift+Enter), Undo (u),
// and Redo (Ctrl+R). In production the direct keys for these actions are handled
// by handleGlobalKeyPress and never reach the Task Workspace or Planner
// dispatchers, so the view dispatchers can only mis-handle them — handleAction
// opens full task detail for Peek and both dispatchers no-op Undo/Redo. The
// palette adapters therefore must emit the identical shell-owned behavior rather
// than delegating these actions to handleAction / handlePlannerAction.
//
// Both the Task Workspace and Planner palette adapters call this before falling
// through to their view dispatcher, so the two adapters share one routing seam
// and cannot drift from the keyboard. It returns (msg, true) when the action is
// shell-owned (the caller must not delegate to its view dispatcher) and
// (nil, false) otherwise. task/project are the palette adapter's resolved cursor
// reference — the same target the direct keys act on.
func (m *AppModel) shellOwnedPaletteAction(action PlannerAction, task *models.Task, project *models.Project) (tea.Msg, bool) {
	switch action {
	case WorkspaceActionOpenPeek:
		if task == nil || project == nil {
			return nil, true
		}
		// Mirror the palette task Peek (peekItem): a PalettePeekRequestMsg keeps
		// the palette on the view stack while the shell activates its in-app Peek
		// overlay, exactly like the direct Shift+Enter key, so a following Esc
		// returns to the palette's search/cursor state rather than discarding it.
		return PalettePeekRequestMsg{Task: task, Project: project}, true
	case WorkspaceActionUndo:
		if cmd := m.performGlobalTaskUndoRedoForProject(project, false); cmd != nil {
			return PaletteCommandResultMsg{Cmd: cmd}, true
		}
		return nil, true
	case WorkspaceActionRedo:
		if cmd := m.performGlobalTaskUndoRedoForProject(project, true); cmd != nil {
			return PaletteCommandResultMsg{Cmd: cmd}, true
		}
		return nil, true
	default:
		return nil, false
	}
}
