package tui

// This file implements TUITestInspectable (see uitest_types.go) for the
// high-value TUI views the LLM-driven review harness drives against. Each
// UITestSnapshot returns a compact, stable map of semantic state — counts,
// cursor, the selected item's title, and the active mode/tab/perspective —
// rather than whole task/project structs. The maps are JSON-marshaled straight
// into an observation, so keys are kept flat and stable across renders.
//
// These methods live in package tui so they can read the views' unexported
// state directly. They are the only extra surface session 7 adds to the driver:
// UITestDriver.Observe type-asserts the active view to TUITestInspectable and
// embeds the result as Observation.Snapshot.

// UITestSnapshot exposes the dashboard's project list state.
func (m *DashboardModel) UITestSnapshot() map[string]any {
	snap := map[string]any{
		"project_count": len(m.projects),
		"cursor":        m.cursor,
		"show_archived": m.showArchived,
	}
	if m.cursor >= 0 && m.cursor < len(m.projects) {
		snap["selected_project"] = m.projects[m.cursor].Name
	}
	if m.filterContext != "" {
		snap["filter_context"] = m.filterContext
	}
	if q := m.SearchQuery(); q != "" {
		snap["search_query"] = q
	}
	if m.HasActiveSearch() {
		snap["search_active"] = true
	}
	return snap
}

// UITestSnapshot exposes the focus board (Today) tab and row state.
func (m *FocusBoardModel) UITestSnapshot() map[string]any {
	snap := map[string]any{
		"row_count": len(m.filtered),
		"cursor":    m.cursor,
	}
	if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabs.Names) {
		snap["active_tab"] = m.tabs.Names[m.tabs.Active]
	}
	if row := m.getRowAtCursor(); row != nil && row.Task != nil {
		snap["selected_task"] = row.Task.Title
	}
	if q := m.SearchQuery(); q != "" {
		snap["search_query"] = q
	}
	return snap
}

// UITestSnapshot exposes the outliner's perspective, rows, and selection.
func (m *OutlinerModel) UITestSnapshot() map[string]any {
	snap := map[string]any{
		"row_count":     len(m.rows),
		"cursor":        m.cursor,
		"perspective":   m.currentPerspective().Name,
		"cross_project": m.project == nil,
	}
	if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].task != nil {
		snap["selected_task"] = m.rows[m.cursor].task.Title
	}
	if q := m.SearchQuery(); q != "" {
		snap["search_query"] = q
	}
	return snap
}

// UITestSnapshot exposes the kanban mode, columns, and focused card.
func (m *KanbanModel) UITestSnapshot() map[string]any {
	snap := map[string]any{
		"mode":          KanbanModeNames[m.mode],
		"column_count":  len(m.columns),
		"active_column": m.activeCol,
		"card_count":    len(m.allCards),
		"cross_project": m.crossProject,
	}
	if m.activeCol >= 0 && m.activeCol < len(m.columns) {
		col := m.columns[m.activeCol]
		snap["active_column_label"] = col.Label
		snap["active_column_card_count"] = len(col.Tasks)
		if col.Cursor >= 0 && col.Cursor < len(col.Tasks) && col.Tasks[col.Cursor].Task != nil {
			snap["selected_task"] = col.Tasks[col.Cursor].Task.Title
		}
	}
	return snap
}

// UITestSnapshot exposes the command palette query, results, and selection.
func (m *CommandPaletteModel) UITestSnapshot() map[string]any {
	snap := map[string]any{
		"query":        m.searchInput.Value(),
		"result_count": len(m.filteredItems),
		"cursor":       m.cursor,
		"action_mode":  m.actionMode,
	}
	if m.cursor >= 0 && m.cursor < len(m.filteredItems) {
		item := m.filteredItems[m.cursor].Item
		snap["selected_item"] = item.Name
		snap["selected_category"] = string(item.Category)
	}
	return snap
}
