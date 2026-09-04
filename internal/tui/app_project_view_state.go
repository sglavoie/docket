package tui

import "github.com/sglavoie/tt/internal/tui/outliner"

// This file hosts the per-project primary-view (Outliner) lifecycle helpers.
// They replace the retired ProjectTree equivalents; the Outliner is now the
// sole per-project primary view, so every former ProjectTree entry point
// (navigation, project cycling, task-link jumps, section/tag filters) routes
// through here.

// saveProjectViewState caches the current outliner's fold state for this session.
func (m *AppModel) saveProjectViewState() {
	m.saveOutlinerFilterState()
}

// refreshProjectView rebuilds the current outliner's rows from the (already
// reloaded) project, preserving the cursor. It never reloads from disk.
func (m *AppModel) refreshProjectView() {
	if m.crossOutliner == nil {
		return
	}
	outliner.RebuildOutlinerPreservingCursor(m.crossOutliner)
}

// refreshProjectViewInMemory is the in-memory-only refresh used as the safe
// fallback after a runtime-domain reload failure.
func (m *AppModel) refreshProjectViewInMemory() {
	m.refreshProjectView()
}
