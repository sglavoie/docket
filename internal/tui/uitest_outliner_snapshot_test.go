package tui

import "github.com/sglavoie/docket/internal/tui/outliner"

// outlinerSemanticSnapshot keeps test observation in the shell harness. Every
// value comes from the Outliner's public view state after rendering.
func outlinerSemanticSnapshot(m *outliner.OutlinerModel) map[string]any {
	snap := map[string]any{
		"row_count":     len(m.Rows),
		"cursor":        m.Cursor,
		"mode":          string(m.ActiveOutlinerMode()),
		"cross_project": m.Project == nil,
	}
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) && m.Rows[m.Cursor].Task != nil {
		snap["selected_task"] = m.Rows[m.Cursor].Task.Title
	}
	if q := m.SearchQuery(); q != "" {
		snap["search_query"] = q
	}
	return snap
}
