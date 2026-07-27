package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// This file hosts the per-project primary-view (Outliner) lifecycle helpers.
// They replace the retired ProjectTree equivalents; the Outliner is now the
// sole per-project primary view, so every former ProjectTree entry point
// (navigation, project cycling, task-link jumps, section/tag filters) routes
// through here.

// switchToProject switches the current project, loads it from storage, and
// (re)creates the outliner view for it. Pass restoreState=false to skip
// restoring saved filter/sort state (e.g. when applying a custom section or
// tag filter that should not be overwritten by the persisted view state).
func (m *AppModel) switchToProject(project *models.Project, _ bool) {
	if project == nil {
		return
	}
	m.saveProjectViewState()
	m.currentProject = project
	_ = m.persister.Load(project)
	// Navigation owns a single authoritative cross-project
	// Outliner; every project selection becomes a scope frame via
	// enterProjectScopeOutliner, so there is no second single-project Outliner to
	// construct, register, present, or target here.
}

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
	rebuildOutlinerPreservingCursor(m.crossOutliner)
}

// refreshProjectViewInMemory is the in-memory-only refresh used as the safe
// fallback after a runtime-domain reload failure.
func (m *AppModel) refreshProjectViewInMemory() {
	m.refreshProjectView()
}
