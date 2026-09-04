package tui

// This file implements TUITestInspectable (see uitest_types.go) for the
// high-value TUI views the LLM-driven review harness drives against. Each
// UITestSnapshot returns a compact, stable map of semantic state — counts,
// cursor, the selected item's title, and the active mode/tab —
// rather than whole task/project structs. The maps are JSON-marshaled straight
// into an observation, so keys are kept flat and stable across renders.
//
// These methods live in package tui so they can read the views' unexported
// state directly. They are the only extra surface session 7 adds to the driver:
// UITestDriver.Observe type-asserts the active view to TUITestInspectable and
// embeds the result as Observation.Snapshot.

// UITestSnapshot exposes the Outliner's mode, rows, and selection.
func (m *OutlinerModel) UITestSnapshot() map[string]any {
	snap := map[string]any{
		"row_count":     len(m.rows),
		"cursor":        m.Cursor,
		"mode":          string(m.activeOutlinerMode()),
		"cross_project": m.project == nil,
	}
	if m.Cursor >= 0 && m.Cursor < len(m.rows) && m.rows[m.Cursor].task != nil {
		snap["selected_task"] = m.rows[m.Cursor].task.Title
	}
	if q := m.SearchQuery(); q != "" {
		snap["search_query"] = q
	}
	return snap
}
