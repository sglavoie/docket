package tui

import (
	"maps"
	"slices"

	"github.com/sglavoie/tt/internal/tui/taskref"
)

// OutlinerToggleState captures section/fold state so it can survive outliner
// recreation within the same TUI session.
type OutlinerToggleState struct {
	CollapsedSections map[string]bool               // Section collapse state for All mode
	ExpandedNodes     map[taskref.TaskIdentity]bool // Project-qualified node fold state (empty = all expanded)
	CursorTaskID      string                        // Task ID at cursor position (session-only)
	FocusStack        []FocusEntry                  // Focus stack (session-only)
}

// SaveToggleState snapshots the current section/fold state.
func (m *OutlinerModel) SaveToggleState() *OutlinerToggleState {
	var cursorTaskID string
	if task := m.getCursorTask(); task != nil {
		cursorTaskID = task.ID
	}

	var focusCopy []FocusEntry
	if len(m.focus.stack) > 0 {
		focusCopy = make([]FocusEntry, len(m.focus.stack))
		copy(focusCopy, m.focus.stack)
	}

	return &OutlinerToggleState{
		CollapsedSections: copyBoolMap(m.folds.collapsedSections),
		ExpandedNodes:     copyBoolMap(m.folds.expandedNodes),
		CursorTaskID:      cursorTaskID,
		FocusStack:        focusCopy,
	}
}

// RestoreToggleState applies a previously saved section/fold state.
func (m *OutlinerModel) RestoreToggleState(state *OutlinerToggleState) {
	if state == nil {
		return
	}

	m.folds.setSections(copyBoolMap(state.CollapsedSections))
	m.folds.setNodes(copyBoolMap(state.ExpandedNodes))
	if len(state.FocusStack) > 0 {
		m.focus.setStack(slices.Clone(state.FocusStack))
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
