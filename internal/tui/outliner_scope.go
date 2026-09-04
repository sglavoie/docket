package tui

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

// captureScopeFrame snapshots the current restorable state into a frame. Every
// mutable map/slice is deep-copied so the frame is fully independent of the
// live model.
func (m *OutlinerModel) captureScopeFrame() outlinerScopeFrame {
	return outlinerScopeFrame{
		scopedProjectID:  m.scope.projectID,
		cursor:           m.Cursor,
		viewScrollOffset: m.ViewScrollOffset,
		primaryMode:      m.primaryMode,
		sort:             m.sort,
		treeSort:         m.treeSort,
		focusStack:       slices.Clone(m.focus.stack),
		crossFocusStack:  slices.Clone(m.focus.crossStack),
		zoom:             m.focus.zoom,
	}
}

// restoreScopeFrame applies a previously captured frame back onto the model,
// deep-copying every mutable map/slice back out so the live model never aliases
// the frame's snapshot.
func (m *OutlinerModel) restoreScopeFrame(f outlinerScopeFrame) {
	m.scope.setProject(f.scopedProjectID)
	m.Cursor = f.cursor
	m.ViewScrollOffset = f.viewScrollOffset
	m.primaryMode = f.primaryMode
	m.clearStaleForRecentMode()
	m.sort = f.sort
	m.treeSort = f.treeSort
	m.focus.restore(slices.Clone(f.focusStack), slices.Clone(f.crossFocusStack), f.zoom)
	m.population.invalidate()
	m.crossProjectRebuildRows()
	m.clampCursor()
}

// scopeProjects returns the projects the outliner should currently range over.
// When scoped to a single project it returns just that project (if still
// loaded); otherwise it returns every loaded project. Callers that build rows,
// counts, and work horizons route through this so scope is honored in one place.
func (m *OutlinerModel) scopeProjects() []*models.Project {
	if !m.scope.scoped() {
		return m.population.projects
	}
	if p := m.scopedProject(); p != nil {
		return []*models.Project{p}
	}
	return nil
}

// scopedProject resolves the currently scoped project from the loaded population, or
// nil when unscoped or the project is no longer loaded.
func (m *OutlinerModel) scopedProject() *models.Project {
	if !m.scope.scoped() {
		return nil
	}
	for _, p := range m.population.projects {
		if p != nil && p.ID == m.scope.projectID {
			return p
		}
	}
	return nil
}

// isProjectScoped reports whether a single-project scope is active. Used to
// drive scope breadcrumbs.
func (m *OutlinerModel) isProjectScoped() bool {
	return m.project == nil && m.scope.scoped()
}

// scopeChainNames returns the active project focus name. The scope model admits
// one base (All or a context) plus at most one project focus, so the chrome can
// never render a nested project chain.
func (m *OutlinerModel) scopeChainNames() []string {
	if m.scope.scoped() {
		return []string{m.scopedProjectChromeLabel(m.scope.projectID)}
	}
	return nil
}

// scopedProjectChromeLabel names a focused project for every piece of scope
// chrome. A parked project says so, because the anchor is what states plainly
// why tasks the user is looking at are absent from the unscoped Outliner.
// Base-workspace projects keep the bare name.
func (m *OutlinerModel) scopedProjectChromeLabel(projectID string) string {
	name := m.projectDisplayNameByID(projectID)
	if m.runtime == nil {
		return name
	}
	project := m.runtime.GetProject(projectID)
	if project == nil || !models.ProjectTagsParked(project.Tags) {
		return name
	}
	return name + scopeChromeSeparator + models.ParkedProjectTag
}

// baseWorkspaceAdmitsProject reports whether the unscoped Outliner would show
// the project on its own, which is what decides if a focus still needs the
// parked exception to keep it in the snapshot.
func (m *OutlinerModel) baseWorkspaceAdmitsProject(projectID string) bool {
	if m == nil || m.runtime == nil || projectID == "" {
		return false
	}
	for _, project := range m.runtime.TaskWorkspaceProjects() {
		if project != nil && project.ID == projectID {
			return true
		}
	}
	return false
}

// settleParkedFocusException records or clears the exception that keeps a
// focused project in a snapshot the base workspace would not admit. Every load
// carrying the focus re-decides it, so a project readmitted to Active mid-focus
// stops being an exception and is handed to the base workspace on the way out
// instead of being shed with its selections.
func (m *OutlinerModel) settleParkedFocusException(projectID string) {
	exception := ""
	if !m.baseWorkspaceAdmitsProject(projectID) {
		exception = projectID
	}
	m.scope.setParkedException(exception)
}

// projectFacetsAdmitProject reports whether the live project facets select the
// project on their own. It is the choice selectCrossScopeProjects makes minus
// the focus exception a load also carries, which is what lets a caller tell a
// project the facets asked for from one only the focus request appended.
func (m *OutlinerModel) projectFacetsAdmitProject(projectID string) bool {
	if m == nil || m.runtime == nil || projectID == "" {
		return false
	}
	if !query.HasActiveProjectFilters(&m.projectFilter) {
		// No facet is active, so the load selected the base task workspace.
		return m.baseWorkspaceAdmitsProject(projectID)
	}
	return query.ProjectFilterAdmits(m.runtime.GetProject(projectID), m.projectFilter)
}

// settleParkedFocusAgainstPopulation retires the parked-focus exception against
// a population the project facets themselves just selected, and returns the
// population that survives it. The exception exists to hold a focused project
// inside a snapshot the base workspace would not admit; a population read under
// the live project filter answers that question itself, so the project stays
// exactly when those facets admit it and is dropped only when the focus request
// alone appended it. Shedding it the way leaving a focus does would delete a
// project the facets explicitly selected and arm a corrective reload, and no
// handler on this route ever claims one.
func (m *OutlinerModel) settleParkedFocusAgainstPopulation(projects []*models.Project) []*models.Project {
	focused := m.scope.takeParkedException()
	// The population in hand was read under the live filter, so it already is
	// whatever an owed reload would have gone back for.
	m.scope.clearParkedReload()
	if focused == "" || m.projectFacetsAdmitProject(focused) {
		return projects
	}
	retained := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project != nil && project.ID == focused {
			continue
		}
		retained = append(retained, project)
	}
	return retained
}

// dropParkedProjectFocus sheds an explicitly focused parked project from the
// live snapshot. Removing it here rather than waiting for the reload is what
// keeps the return to base free of a visible parked task tree, and it lets the
// existing loaded-project prune drop selections the restored base cannot admit.
func (m *OutlinerModel) dropParkedProjectFocus() {
	if m == nil {
		return
	}
	focused := m.scope.takeParkedException()
	if focused == "" {
		return
	}
	m.scope.oweParkedReload()
	retained := make([]*models.Project, 0, len(m.population.projects))
	for _, project := range m.population.projects {
		if project != nil && project.ID == focused {
			continue
		}
		retained = append(retained, project)
	}
	m.setLoadedProjects(retained)
	m.pruneSelectionsOutsideLoadedProjects()
	m.population.invalidate()
	m.crossProjectRebuildRows()
	m.clampCursor()
}

// pushProjectScope enters or replaces the one project focus. The first focus
// captures the base state; later project choices replace the focus in place
// without stacking, so one Esc always returns to that base context.
func (m *OutlinerModel) pushProjectScope(projectID string) {
	if projectID == "" || m.project != nil {
		return
	}
	if projectID == m.scope.projectID {
		return
	}
	if projectID != m.scope.parkedProjectID {
		// Focusing elsewhere ends the exception that admitted the parked project.
		m.dropParkedProjectFocus()
	}
	_ = m.persistActiveRouteState()
	switch {
	case !m.scope.scoped():
		// Starting from a base scope creates the sole restorable frame.
		m.scope.setBase(m.captureScopeFrame())
	case m.scope.retainBase():
		// Replacing a project focus retains only the original base frame.
	default:
		// Defensive: scoped without a frame; repair older nested state without
		// making it visible.
		base := m.captureScopeFrame()
		base.scopedProjectID = ""
		m.scope.setBase(base)
	}
	m.scope.setProject(projectID)
	m.focus.clearCross()
	m.clearSectionZoom()
	m.Cursor = 0
	m.resetViewScrollOffset()
	m.population.invalidate()
	m.crossProjectRebuildRows()
	m.clampCursor()
}

// popProjectScope restores the captured base scope. It reports whether a focus
// was active so the Esc cascade can fall through at the base.
func (m *OutlinerModel) popProjectScope() bool {
	if !m.scope.scoped() {
		return false
	}
	frame, ok := m.scope.takeBase()
	if !ok {
		return false
	}
	_ = m.persistActiveRouteState()
	m.dropParkedProjectFocus()
	m.restoreScopeFrame(frame)
	return true
}

// clearProjectScope drops the entire scope stack and returns to the
// authoritative all-projects scope, restoring the base frame captured at the
// very first push. Reports whether anything changed.
func (m *OutlinerModel) clearProjectScope() bool {
	if !m.scope.scoped() && len(m.scope.stack) == 0 {
		return false
	}
	m.closeMenuBar()
	_ = m.persistActiveRouteState()
	if base, ok := m.scope.takeBase(); ok {
		m.dropParkedProjectFocus()
		m.restoreScopeFrame(base)
		return true
	}
	// Defensive: scoped without a frame (should not happen); reset in place.
	m.dropParkedProjectFocus()
	m.scope.clearProject()
	m.population.invalidate()
	m.crossProjectRebuildRows()
	m.clampCursor()
	return true
}

// resetToGlobalAll returns the cross-project Outliner to a clean, unscoped
// global All root: it drops any project scope, clears every narrowing and ordering
// axis (search, saved criteria, common filters and manual sort), clears
// focus/selection, activates All, and parks the cursor at the top. It is the state
// Ctrl+Esc restores as the single global reset, so no advertised clean reset can
// leave the root silently narrowed by a stale filter, tag, or sort.
func (m *OutlinerModel) resetToGlobalAll() {
	m.Confirm.Cancel()
	m.quickEdit.Exit()
	m.closeMenuBar()
	m.scope.clearStack()
	m.scope.clearProject()
	m.dropParkedProjectFocus()
	m.focus.clear()
	m.focus.clearCross()
	// Restated rather than left to the selectOutlinerMode(ModeAll) below, which
	// also clears it: the reset names every axis it drops, the way it already
	// restates the focus-stack clears.
	m.clearSectionZoom()
	m.folds.resetAll()
	m.replaceFilterClauses(query.FilterQueryTaskFields(), nil)
	m.sort.Reset()
	m.treeSort.Reset()
	if m.MultiSelectMode {
		m.clearMultiSelect()
		m.MultiSelectMode = false
	}
	m.clearSearchProjectionWithoutRestore()
	// The reset is replacing the whole projection, so the lens is dropped rather
	// than restored: restoring its baseline would reinstate the narrowing this
	// key exists to clear.
	m.clearActiveLensWithoutRestore()
	m.Cursor = 0
	m.resetViewScrollOffset()
	m.selectOutlinerMode(chrome.ModeAll)
	m.population.invalidate()
	m.rebuildRows()
	m.clampCursor()
}

// resetToLaunchView returns the view to the state a lens-free launch produces:
// Active Projects, All, no query, no filters, no sort, no focus, and no lens
// applied. Every axis is taken away through the same call its own key uses, the
// way peelQueryReadoutAxis does, so the reset can never reach a state the
// ordinary grammar cannot.
//
// It reports which axes it found set, so the strip can name what it cleared, and
// whether the population owes a reload — the one thing this model cannot do for
// itself. The axes are read before any of them is written, because the unwinding
// calls restore captured frames that reinstate one another's state.
//
// Unlike resetToGlobalAll this is a reset of the view alone: the machine-local
// active-lens pointer is left untouched, so the lens is put down now and opens
// again at the next launch.
func (m *OutlinerModel) resetToLaunchView() ([]string, bool) {
	if m == nil {
		return nil, false
	}
	cleared := make([]string, 0, 8)
	note := func(active bool, axis string) bool {
		if active {
			cleared = append(cleared, axis)
		}
		return active
	}
	note(m.hasActiveLens(), "lens")
	note(m.hasSearchProjection(), "query")
	note(m.focus.active(), "focus")
	note(m.focus.zoom.active(), "zoom")
	note(m.isProjectScoped(), "scope")
	note(query.CommonFilterFacetCount(&m.filter.CommonFilterState) > 0, "filters")
	reload := note(query.ProjectFilterFacetCount(&m.projectFilter) > 0, "project filters")
	note(m.sort.Field != query.SortByDefault || m.treeSort.Field != query.SortByDefault, "sort")
	note(m.primaryMode != chrome.ModeAll, "mode")
	if len(cleared) == 0 {
		return cleared, false
	}

	// The lens goes first and goes without its baseline: restoring it would
	// reinstate the narrowing this key exists to clear.
	m.forgetActiveLensFrame()
	// The query goes next, so no ranked projection is left scored over the
	// population the axes below are about to replace.
	m.ClearSearch()
	// Focus and zoom unwind in the Esc cascade's own order. Focus is a stack and
	// its key pops one level, so it is popped until it is empty.
	for m.popFocusLevel() {
	}
	m.exitSectionZoom()
	// Scope restores the base frame captured at the first push, and that frame
	// carries a mode, a sort, and a zoom of its own, so it lands before the axes
	// below rather than after them.
	m.clearProjectScope()
	// The two filter families are cleared separately for the same reason fx and
	// Fx are two keys: clearing one must never clear the other.
	m.replaceFilterClauses(query.FilterQueryTaskFields(), nil)
	m.replaceFilterClauses(filterOperationFields(operation.OperationFilterProjectClear), nil)
	m.sort.Reset()
	m.treeSort.Reset()
	// Selecting the mode is the last write and owns the rebuild, so the rows are
	// built once, over every axis this reset has already put down.
	m.selectOutlinerMode(chrome.ModeAll)
	return cleared, reload
}

// reconcileProjectScope keeps the one focus valid after a reload or external
// mutation. If it vanished, the captured base is restored. Returns ("", false)
// when no repair was needed.
func (m *OutlinerModel) reconcileProjectScope() (string, bool) {
	if m.project != nil || !m.scope.scoped() {
		return "", false
	}
	if m.scopedProject() != nil {
		return "", false
	}
	// The focus target is gone — removed, archived, or unreadable — so stop
	// re-requesting it on every later reload.
	m.scope.clearParkedException()
	if frame, ok := m.scope.takeBase(); ok {
		m.restoreScopeFrame(frame)
		return "Focused project no longer exists — returned to the base scope", true
	}
	// No captured frame remained; fall back to the active context base in place.
	m.scope.clearProject()
	m.focus.clearCross()
	m.clearSectionZoom()
	m.Cursor = 0
	m.resetViewScrollOffset()
	m.population.invalidate()
	m.crossProjectRebuildRows()
	m.clampCursor()
	return "Focused project no longer exists — returned to the base scope", true
}

// projectIsLoaded reports whether a project ID is present in the loaded population.
func (m *OutlinerModel) projectIsLoaded(projectID string) bool {
	for _, p := range m.population.projects {
		if p != nil && p.ID == projectID {
			return true
		}
	}
	return false
}

// projectDisplayNameByID resolves a friendly project name from the loaded population,
// falling back to the raw ID when the project is not loaded.
func (m *OutlinerModel) projectDisplayNameByID(projectID string) string {
	for _, p := range m.population.projects {
		if p != nil && p.ID == projectID {
			return p.Name
		}
	}
	return projectID
}
