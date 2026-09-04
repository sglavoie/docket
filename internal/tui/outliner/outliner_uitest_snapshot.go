package outliner

// UITestSnapshot exposes the Outliner's mode, rows, and selection as a compact,
// stable map of semantic state — counts, cursor, the selected item's title, and
// the active mode — rather than whole task/project structs. The map is
// JSON-marshaled straight into an observation, so its keys are flat and stable
// across renders.
//
// It satisfies the shell harness's TUITestInspectable assertion (see
// uitest_types.go in the root shell). The harness stays in the shell package
// while this reads the Outliner's unexported rows and mode, and a method cannot
// cross a package boundary, so the seam is one exported method here rather than
// a test file over there.
func (m *OutlinerModel) UITestSnapshot() map[string]any {
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
