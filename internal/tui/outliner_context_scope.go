package tui

import "github.com/sglavoie/tt/internal/models"

// applyContextScope makes a Context the global Outliner's active project scope.
// It clears the axes that name individual projects or rows (drill-in scope,
// zoom, multi-select, cursor, scroll) because those cannot survive a project-set
// change, and deliberately preserves task filters, mode, search, sort, columns,
// and the pinned-tag tab so a Context switch re-runs the user's current view
// against a different project set.
func (m *OutlinerModel) applyContextScope(scope ContextScope) {
	m.activeContext = scope
	m.refreshContextScopeProjects()
	m.scopeStack = nil
	m.scopedProjectID = ""
	m.zoomStack = nil
	m.crossZoomStack = nil
	if m.multiSelectMode {
		m.clearMultiSelect()
		m.multiSelectMode = false
	}
	m.cursor = 0
	m.resetViewScrollOffset()
	m.crossForceRebuild = true
	m.rebuildRows()
	m.clampCursor()
}

// clearContextScope returns the Outliner to the All context without touching any
// other axis. Callers that also reset filters (the global reset) do the rest.
func (m *OutlinerModel) clearContextScope() {
	m.activeContext = AllContextScope()
	m.contextScopeProjectIDs = nil
}

// refreshContextScopeProjects re-resolves the active Context against the current
// catalog. It must run after any reload so archiving or deleting a member
// project cannot leave the scope admitting a stale ID.
//
// It is also the single reconciliation boundary for scope selectability. Every
// apply and every cross-project reload passes through here, so a catalog
// mutation made anywhere — the Context Hub, the project editor, an archive, a
// deletion — degrades a regular label that lost its last active member to All
// rather than stranding the Outliner on a name the hub no longer lists.
func (m *OutlinerModel) refreshContextScopeProjects() {
	if m.runtime != nil {
		m.activeContext = selectableContextScope(m.runtime, m.activeContext)
	}
	if m.activeContext.IsAll() || m.runtime == nil {
		m.contextScopeProjectIDs = nil
		return
	}
	ids := m.activeContext.ProjectIDs(m.runtime)
	admitted := make(map[string]bool, len(ids))
	for _, id := range ids {
		admitted[id] = true
	}
	m.contextScopeProjectIDs = admitted
}

// contextScopeSkipsProject reports whether a loaded project falls outside the
// active Context. A project drill-in scope wins: drilling into a project always
// shows it, even when it is not a Context member.
func (m *OutlinerModel) contextScopeSkipsProject(projectID string) bool {
	if m.activeContext.IsAll() || m.scopedProjectID != "" {
		return false
	}
	return !m.contextScopeProjectIDs[projectID]
}

// contextScopeProjects narrows loadedProjects to the active Context, preserving
// load order. Under the All context it returns loadedProjects unchanged.
func (m *OutlinerModel) contextScopeProjects() []*models.Project {
	if m.activeContext.IsAll() {
		return m.loadedProjects
	}
	narrowed := make([]*models.Project, 0, len(m.loadedProjects))
	for _, p := range m.loadedProjects {
		if p != nil && m.contextScopeProjectIDs[p.ID] {
			narrowed = append(narrowed, p)
		}
	}
	return narrowed
}

// contextScopeLabel names the active Context for scope chrome.
func (m *OutlinerModel) contextScopeLabel() string {
	if m.activeContext.IsAll() {
		return "All"
	}
	return m.activeContext.Name
}
