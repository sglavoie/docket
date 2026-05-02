package tui

// KanbanViewCache stores per-session kanban view state (in-memory only).
// Keyed by project ID for per-project kanban, or "cross_project" for the global view.
type KanbanViewCache struct {
	ActiveColID       string         // Column ID of the focused column
	ColCursors        map[string]int // cursor position per column ID
	ColScrollOffsets  map[string]int // scroll offset per column ID
	Mode              KanbanMode     // current axis mode
	DetailPaneVisible bool           // detail pane visibility
}

// kanbanCacheKey returns the map key for a KanbanModel's state cache.
func kanbanCacheKey(kb *KanbanModel) string {
	if kb.crossProject {
		return "cross_project"
	}
	if kb.project != nil {
		return kb.project.ID
	}
	return "unknown"
}

// saveKanbanState snapshots the current kanban view state into the session cache.
func (m *AppModel) saveKanbanState() {
	v := m.views.Get(m.currentView())
	kb, ok := v.(*KanbanModel)
	if !ok {
		return
	}
	key := kanbanCacheKey(kb)

	cursors := make(map[string]int, len(kb.columns))
	scrolls := make(map[string]int, len(kb.columns))
	activeColID := ""
	for i, col := range kb.columns {
		cursors[col.ID] = col.Cursor
		scrolls[col.ID] = col.ScrollOffset
		if i == kb.activeCol {
			activeColID = col.ID
		}
	}

	m.kanbanViewCache[key] = &KanbanViewCache{
		ActiveColID:       activeColID,
		ColCursors:        cursors,
		ColScrollOffsets:  scrolls,
		Mode:              kb.mode,
		DetailPaneVisible: kb.detailPaneVisible,
	}
}

// restoreKanbanState applies saved session state onto a newly created KanbanModel.
func (m *AppModel) restoreKanbanState(kb *KanbanModel) {
	key := kanbanCacheKey(kb)
	cache, ok := m.kanbanViewCache[key]
	if !ok {
		return
	}

	// Restore mode and detail pane.
	kb.mode = cache.Mode
	kb.detailPaneVisible = cache.DetailPaneVisible
	kb.rebuildCols()

	// Restore per-column cursor and scroll, and find the active column.
	activeColIdx := -1
	for i := range kb.columns {
		col := &kb.columns[i]
		if cursor, exists := cache.ColCursors[col.ID]; exists {
			col.Cursor = cursor
			if col.Cursor >= len(col.Tasks) {
				col.Cursor = max(0, len(col.Tasks)-1)
			}
		}
		if scroll, exists := cache.ColScrollOffsets[col.ID]; exists {
			col.ScrollOffset = scroll
		}
		if col.ID == cache.ActiveColID {
			activeColIdx = i
		}
	}
	if activeColIdx >= 0 {
		kb.activeCol = activeColIdx
		kb.adjustHorizontalScroll()
	}
}
