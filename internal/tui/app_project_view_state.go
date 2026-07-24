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
func (m *AppModel) switchToProject(project *models.Project, restoreState bool) {
	if project == nil {
		return
	}
	m.saveProjectViewState()
	m.currentProject = project
	_ = m.persister.Load(project)
	m.runtime.Settings().ApplyGlobalColorsToProject(project)
	// Production navigation (typed) owns a single authoritative cross-project
	// Outliner; every project selection becomes a scope frame via
	// enterProjectScopeOutliner, so there is no second single-project Outliner to
	// construct, register, present, or target here. The legacy single-project
	// ViewOutliner is built only on the non-typed (headless-test) path.
	if m.typedNavigation {
		return
	}
	m.outliner = NewOutlinerModel(project, m.configManager, m.runtime)
	m.outliner.queueIDs = m.focusQueueIDs
	m.outliner.dailyPlanIDs = m.dailyPlanIDs
	m.outliner.SetSize(m.width, m.height-2)
	m.restoreOutlinerToggleState()
	if restoreState {
		m.restoreOutlinerFilterState()
	}
	m.views.Register(ViewOutliner, m.outliner)
}

// saveProjectViewState caches the current outliner's fold state for this session.
func (m *AppModel) saveProjectViewState() {
	if m.outliner == nil {
		return
	}
	m.saveOutlinerToggleState()
	m.saveOutlinerFilterState()
}

// refreshProjectView rebuilds the current outliner's rows from the (already
// reloaded) project, preserving the cursor. It never reloads from disk.
func (m *AppModel) refreshProjectView() {
	if m.currentProject == nil || m.outliner == nil {
		return
	}
	m.currentProject.BuildTaskTree()
	m.outliner.project = m.currentProject
	m.outliner.rebuildRows()
	m.outliner.clampCursor()
}

// refreshProjectViewInMemory is the in-memory-only refresh used as the safe
// fallback after a runtime-domain reload failure.
func (m *AppModel) refreshProjectViewInMemory() {
	m.refreshProjectView()
}
