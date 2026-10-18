package outliner

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
)

// captureScopeFrame snapshots the current restorable state into a frame. Every
// mutable map/slice is deep-copied so the frame is fully independent of the
// live model.
func (m *OutlinerModel) captureScopeFrame() OutlinerScopeFrame {
	return OutlinerScopeFrame{
		scopedProjectID:  m.Scope.ProjectID,
		cursor:           m.Cursor,
		viewScrollOffset: m.ViewScrollOffset,
		primaryMode:      m.PrimaryMode,
		Sort:             m.Sort,
		treeSort:         m.TreeSort,
		focusStack:       slices.Clone(m.Focus.Stack),
		crossFocusStack:  slices.Clone(m.Focus.CrossStack),
		zoom:             m.Focus.Zoom,
	}
}

// restoreScopeFrame applies a previously captured frame back onto the model,
// deep-copying every mutable map/slice back out so the live model never aliases
// the frame's snapshot.
func (m *OutlinerModel) restoreScopeFrame(f OutlinerScopeFrame) {
	m.Scope.setProject(f.scopedProjectID)
	m.Cursor = f.cursor
	m.ViewScrollOffset = f.viewScrollOffset
	m.PrimaryMode = f.primaryMode
	m.clearStaleForRecentMode()
	m.Sort = f.Sort
	m.TreeSort = f.treeSort
	m.Focus.Restore(slices.Clone(f.focusStack), slices.Clone(f.crossFocusStack), f.zoom)
	m.Population.Invalidate()
	m.CrossProjectRebuildRows()
	m.ClampCursor()
}

// scopeProjects returns the projects the outliner should currently range over.
// When scoped to a single project it returns just that project (if still
// loaded); otherwise it returns every loaded project. Callers that build rows,
// counts, and work horizons route through this so scope is honored in one place.
func (m *OutlinerModel) ScopeProjects() []*models.Project {
	if !m.Scope.scoped() {
		return m.Population.Projects
	}
	if p := m.ScopedProject(); p != nil {
		return []*models.Project{p}
	}
	return nil
}

// scopedProject resolves the currently scoped project from the loaded population, or
// nil when unscoped or the project is no longer loaded.
func (m *OutlinerModel) ScopedProject() *models.Project {
	if !m.Scope.scoped() {
		return nil
	}
	for _, p := range m.Population.Projects {
		if p != nil && p.ID == m.Scope.ProjectID {
			return p
		}
	}
	return nil
}

// isProjectScoped reports whether a single-project scope is active. Used to
// drive scope breadcrumbs.
func (m *OutlinerModel) IsProjectScoped() bool {
	return m.Project == nil && m.Scope.scoped()
}

// focusedProjectHeaderOwner names the project a focused projection's one header
// belongs to, covering both single-project forms: a direct project model, which
// isProjectScoped deliberately excludes, and an explicit project scope. It
// resolves a scope through scopedProject, so an owner the population no longer
// holds resolves to nil and emits no header rather than a fabricated empty
// project. It returns nil in an unscoped workspace, so a mode that happens to
// project one project's tasks still gets ordinary group headers and no focused
// one.
func (m *OutlinerModel) FocusedProjectHeaderOwner() *models.Project {
	if m == nil {
		return nil
	}
	if m.Project != nil {
		return m.Project
	}
	return m.ScopedProject()
}

// scopeChainNames returns the active project focus name. The scope model admits
// one base (All or a context) plus at most one project focus, so the chrome can
// never render a nested project chain.
func (m *OutlinerModel) scopeChainNames() []string {
	if m.Scope.scoped() {
		return []string{m.scopedProjectChromeLabel(m.Scope.ProjectID)}
	}
	return nil
}

// scopedProjectChromeLabel names a focused project for every piece of scope
// chrome. A parked project says so, because the anchor is what states plainly
// why tasks the user is looking at are absent from the unscoped Outliner.
// Base-workspace projects keep the bare name.
func (m *OutlinerModel) scopedProjectChromeLabel(projectID string) string {
	name := m.projectDisplayNameByID(projectID)
	if m.Runtime == nil {
		return name
	}
	project := m.Runtime.GetProject(projectID)
	if project == nil || !models.ProjectTagsParked(project.Tags) {
		return name
	}
	return name + ScopeChromeSeparator + models.ParkedProjectTag
}

// baseWorkspaceAdmitsProject reports whether the unscoped Outliner would show
// the project on its own, which is what decides if a focus still needs the
// parked exception to keep it in the snapshot.
func (m *OutlinerModel) baseWorkspaceAdmitsProject(projectID string) bool {
	if m == nil || m.Runtime == nil || projectID == "" {
		return false
	}
	for _, project := range m.Runtime.TaskWorkspaceProjects() {
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
	m.Scope.setParkedException(exception)
}

// projectFacetsAdmitProject reports whether the live project facets select the
// project on their own. It is the choice selectCrossScopeProjects makes minus
// the focus exception a load also carries, which is what lets a caller tell a
// project the facets asked for from one only the focus request appended.
func (m *OutlinerModel) projectFacetsAdmitProject(projectID string) bool {
	if m == nil || m.Runtime == nil || projectID == "" {
		return false
	}
	if !query.HasActiveProjectFilters(&m.ProjectFilter) {
		// No facet is active, so the load selected the base task workspace.
		return m.baseWorkspaceAdmitsProject(projectID)
	}
	return query.ProjectFilterAdmits(m.Runtime.GetProject(projectID), m.ProjectFilter)
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
	focused := m.Scope.takeParkedException()
	// The population in hand was read under the live filter, so it already is
	// whatever an owed reload would have gone back for.
	m.Scope.clearParkedReload()
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
	focused := m.Scope.takeParkedException()
	if focused == "" {
		return
	}
	m.Scope.oweParkedReload()
	retained := make([]*models.Project, 0, len(m.Population.Projects))
	for _, project := range m.Population.Projects {
		if project != nil && project.ID == focused {
			continue
		}
		retained = append(retained, project)
	}
	m.setLoadedProjects(retained)
	m.pruneSelectionsOutsideLoadedProjects()
	m.Population.Invalidate()
	m.CrossProjectRebuildRows()
	m.ClampCursor()
}

// pushProjectScope enters or replaces the one project focus. The first focus
// captures the base state; later project choices replace the focus in place
// without stacking, so one Esc always returns to that base context.
func (m *OutlinerModel) PushProjectScope(projectID string) {
	m.pushProjectScope(projectID, false)
}

// pushProjectScopeRetainingParkedFocus is the same push for a scope change that
// has already promised a way back to the one it is leaving. Shedding the parked
// exception is what leaving a focus means, and a visit is not leaving: the
// project would have to be reacquired to return to it, and the return the
// promise is made on cannot raise the Cmd a reacquisition needs. Retaining it
// instead leaves the origin in the snapshot the whole time, still re-requested
// by every later read, so the return restores a view over the current reading
// rather than over a project the hop unloaded.
func (m *OutlinerModel) pushProjectScopeRetainingParkedFocus(projectID string) {
	m.pushProjectScope(projectID, true)
}

// pushProjectScope is the one implementation both spellings share. retainParked
// decides only the fate of the parked exception; every other axis moves the same
// way either way.
func (m *OutlinerModel) pushProjectScope(projectID string, retainParked bool) {
	if projectID == "" || m.Project != nil {
		return
	}
	if projectID == m.Scope.ProjectID {
		return
	}
	switch {
	case retainParked:
		// A request still in flight is abandoned even here: it has adopted nothing to
		// retain, and honoring it would push a scope of its own over the destination
		// this push is entering. Only the settled exception survives.
		m.Scope.clearPendingParked()
	case projectID != m.Scope.ParkedProjectID:
		// Focusing elsewhere ends the exception that admitted the parked project.
		m.dropParkedProjectFocus()
	}
	_ = m.PersistActiveRouteState()
	switch {
	case !m.Scope.scoped():
		// Starting from a base scope creates the sole restorable frame.
		m.Scope.setBase(m.captureScopeFrame())
	case m.Scope.retainBase():
		// Replacing a project focus retains only the original base frame.
	default:
		// Defensive: scoped without a frame; repair older nested state without
		// making it visible.
		base := m.captureScopeFrame()
		base.scopedProjectID = ""
		m.Scope.setBase(base)
	}
	m.Scope.setProject(projectID)
	m.Focus.clearCross()
	m.clearSectionZoom()
	m.Cursor = 0
	m.resetViewScrollOffset()
	m.Population.Invalidate()
	m.CrossProjectRebuildRows()
	m.seatCursorAtProjectionTop()
}

// popProjectScope restores the captured base scope. It reports whether a focus
// was active so the Esc cascade can fall through at the base.
func (m *OutlinerModel) PopProjectScope() bool {
	if !m.Scope.scoped() {
		return false
	}
	frame, ok := m.Scope.takeBase()
	if !ok {
		return false
	}
	_ = m.PersistActiveRouteState()
	m.dropParkedProjectFocus()
	m.restoreScopeFrame(frame)
	return true
}

// clearProjectScope drops the entire scope stack and returns to the
// authoritative all-projects scope, restoring the base frame captured at the
// very first push. Reports whether anything changed.
func (m *OutlinerModel) ClearProjectScope() bool {
	if !m.Scope.scoped() && len(m.Scope.Stack) == 0 {
		return false
	}
	m.closeMenuBar()
	_ = m.PersistActiveRouteState()
	if base, ok := m.Scope.takeBase(); ok {
		m.dropParkedProjectFocus()
		m.restoreScopeFrame(base)
		return true
	}
	// Defensive: scoped without a frame (should not happen); reset in place.
	m.dropParkedProjectFocus()
	m.Scope.clearProject()
	m.Population.Invalidate()
	m.CrossProjectRebuildRows()
	m.ClampCursor()
	return true
}

// reconcileProjectScope keeps the one focus valid after a reload or external
// mutation. If it vanished, the captured base is restored. Returns ("", false)
// when no repair was needed.
func (m *OutlinerModel) reconcileProjectScope() (string, bool) {
	if m.Project != nil || !m.Scope.scoped() {
		return "", false
	}
	if m.ScopedProject() != nil {
		return "", false
	}
	// The focus target is gone — removed, archived, or unreadable — so stop
	// re-requesting it on every later reload.
	m.Scope.clearParkedException()
	if frame, ok := m.Scope.takeBase(); ok {
		m.restoreScopeFrame(frame)
		return "Focused project no longer exists — returned to the base scope", true
	}
	// No captured frame remained; fall back to the active context base in place.
	m.Scope.clearProject()
	m.Focus.clearCross()
	m.clearSectionZoom()
	m.Cursor = 0
	m.resetViewScrollOffset()
	m.Population.Invalidate()
	m.CrossProjectRebuildRows()
	m.ClampCursor()
	return "Focused project no longer exists — returned to the base scope", true
}

// projectIsLoaded reports whether a project ID is present in the loaded population.
func (m *OutlinerModel) ProjectIsLoaded(projectID string) bool {
	for _, p := range m.Population.Projects {
		if p != nil && p.ID == projectID {
			return true
		}
	}
	return false
}

// projectDisplayNameByID resolves a friendly project name from the loaded population,
// falling back to the raw ID when the project is not loaded.
func (m *OutlinerModel) projectDisplayNameByID(projectID string) string {
	for _, p := range m.Population.Projects {
		if p != nil && p.ID == projectID {
			return p.Name
		}
	}
	return projectID
}

// ApplyProjectScopeMsg applies a Project Hub or project-header selection to the
// sole Outliner scope stack. It is declared here rather than on the shell
// because every surface that can name a project raises it, and the scope it
// names is this package's.
type ApplyProjectScopeMsg struct {
	ProjectID string
	Clear     bool
}
