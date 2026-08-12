package tui

import (
	"maps"
)

// OutlinerToggleState captures section/fold state so it can survive outliner
// recreation within the same TUI session.
type OutlinerToggleState struct {
	CollapsedSections map[string]bool       // Section collapse state for All mode
	ExpandedNodes     map[TaskIdentity]bool // Project-qualified node fold state (empty = all expanded)
	CursorTaskID      string                // Task ID at cursor position (session-only)
	FocusStack        []FocusEntry          // Focus stack (session-only)
}

// SaveToggleState snapshots the current section/fold state.
func (m *OutlinerModel) SaveToggleState() *OutlinerToggleState {
	var cursorTaskID string
	if task := m.getCursorTask(); task != nil {
		cursorTaskID = task.ID
	}

	var focusCopy []FocusEntry
	if len(m.focusStack) > 0 {
		focusCopy = make([]FocusEntry, len(m.focusStack))
		copy(focusCopy, m.focusStack)
	}

	return &OutlinerToggleState{
		CollapsedSections: copyBoolMap(m.collapsedSections),
		ExpandedNodes:     copyBoolMap(m.expandedNodes),
		CursorTaskID:      cursorTaskID,
		FocusStack:        focusCopy,
	}
}

// RestoreToggleState applies a previously saved section/fold state.
func (m *OutlinerModel) RestoreToggleState(state *OutlinerToggleState) {
	if state == nil {
		return
	}

	m.collapsedSections = copyBoolMap(state.CollapsedSections)
	m.expandedNodes = copyBoolMap(state.ExpandedNodes)
	if len(state.FocusStack) > 0 {
		m.focusStack = make([]FocusEntry, len(state.FocusStack))
		copy(m.focusStack, state.FocusStack)
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
