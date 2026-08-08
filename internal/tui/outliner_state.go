package tui

import (
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

// projectCommitOpts constructs the project-scoped options for the
// application-layer commit and drains the pending undo context.
func (m *OutlinerModel) projectCommitOpts() app.ProjectCommitOptions {
	action, context := m.undo.Action, m.undo.Context
	// The pending undo TaskIDs name the tasks this mutation targeted; forward them
	// as the planning-reconcile set so a whole-project commit that changed a task's
	// state (quick edit, section move, paste) folds Today/Next exclusivity
	// into the same transaction. A task whose state did not change reconciles to no
	// planning artifact, so this is harmless for structural-only mutations.
	planningTaskIDs := append([]string(nil), m.undo.TaskIDs...)
	m.undo.Clear()
	return app.ProjectCommitOptions{
		UndoAction:      action,
		UndoContext:     context,
		PlanningTaskIDs: planningTaskIDs,
	}
}

// setUndoContext sets the action context for the next save operation.
func (m *OutlinerModel) setUndoContext(action, context string) {
	m.undo.Set(action, context)
}

// OutlinerToggleState captures section/fold state so it can survive outliner
// recreation within the same TUI session.
type OutlinerToggleState struct {
	CollapsedSections map[string]bool       // Section collapse state for All mode
	ExpandedNodes     map[TaskIdentity]bool // Project-qualified node fold state (empty = all expanded)
	CursorTaskID      string                // Task ID at cursor position (session-only)
	ZoomStack         []ZoomEntry           // Zoom stack (session-only)
}

// SaveToggleState snapshots the current section/fold state.
func (m *OutlinerModel) SaveToggleState() *OutlinerToggleState {
	var cursorTaskID string
	if task := m.getCursorTask(); task != nil {
		cursorTaskID = task.ID
	}

	var zoomCopy []ZoomEntry
	if len(m.zoomStack) > 0 {
		zoomCopy = make([]ZoomEntry, len(m.zoomStack))
		copy(zoomCopy, m.zoomStack)
	}

	return &OutlinerToggleState{
		CollapsedSections: copyBoolMap(m.collapsedSections),
		ExpandedNodes:     copyBoolMap(m.expandedNodes),
		CursorTaskID:      cursorTaskID,
		ZoomStack:         zoomCopy,
	}
}

// RestoreToggleState applies a previously saved section/fold state.
func (m *OutlinerModel) RestoreToggleState(state *OutlinerToggleState) {
	if state == nil {
		return
	}

	m.collapsedSections = copyBoolMap(state.CollapsedSections)
	m.expandedNodes = copyBoolMap(state.ExpandedNodes)
	if len(state.ZoomStack) > 0 {
		m.zoomStack = make([]ZoomEntry, len(state.ZoomStack))
		copy(m.zoomStack, state.ZoomStack)
	}
	m.rebuildRows()
	if state.CursorTaskID != "" {
		m.moveCursorToTask(state.CursorTaskID)
	}
}

// copyBoolMap returns a shallow copy of a bool map.
func copyBoolMap[K comparable](src map[K]bool) map[K]bool {
	if src == nil {
		return nil
	}
	dst := make(map[K]bool, len(src))
	maps.Copy(dst, src)
	return dst
}

// saveProject persists the project's current in-memory state through the
// application layer and grouped ledger undo. The caller has already
// applied its structural mutation (swap/indent/outdent/section move/paste) to
// m.project in memory.
func (m *OutlinerModel) saveProject() tea.Cmd {
	return m.taskItem.CommitProject(m.project, m.projectCommitOpts(), "")
}
