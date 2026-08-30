package tui

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
)

// outlinerScopeFrame captures the outliner state that a project-scope push
// replaces, so popping the scope restores navigation and presentation axes that
// remain scope-owned. Task facets are intentionally absent and remain global
// across mode and scope changes. The sole exception is Stale, which explicit
// mode selection and frame restoration both clear whenever they activate Recent.
//
// Fold state is deliberately NOT snapshotted and remains shared across scopes:
//   - collapsedSections / expandedNodes are keyed by project and section, so a
//     scope change cannot corrupt another project's folds.
//   - computed section fold maps are not per-scope state.
//
// The section zoom is snapshotted precisely because it is not fold state: it is
// a narrowing of the projection the push replaces, like the focus stacks beside
// it, so the pop owes the user the zoom they left.
type outlinerScopeFrame struct {
	scopedProjectID  string
	cursor           int
	viewScrollOffset int

	primaryMode OutlinerModeID

	// Active render ordering plus the durable tree-mode sort.
	sort     SortState
	treeSort SortState

	// Focus stacks (single-project and cross-project).
	focusStack      []FocusEntry
	crossFocusStack []CrossProjectFocusEntry

	// Section zoom the push dropped.
	zoom sectionZoom
}

// captureScopeFrame snapshots the current restorable state into a frame. Every
// mutable map/slice is deep-copied so the frame is fully independent of the
// live model.
func (m *OutlinerModel) captureScopeFrame() outlinerScopeFrame {
	return outlinerScopeFrame{
		scopedProjectID:  m.scopedProjectID,
		cursor:           m.cursor,
		viewScrollOffset: m.viewScrollOffset,
		primaryMode:      m.primaryMode,
		sort:             m.sort,
		treeSort:         m.treeSort,
		focusStack:       slices.Clone(m.focusStack),
		crossFocusStack:  slices.Clone(m.crossFocusStack),
		zoom:             m.zoom,
	}
}

// restoreScopeFrame applies a previously captured frame back onto the model,
// deep-copying every mutable map/slice back out so the live model never aliases
// the frame's snapshot.
func (m *OutlinerModel) restoreScopeFrame(f outlinerScopeFrame) {
	m.scopedProjectID = f.scopedProjectID
	m.cursor = f.cursor
	m.viewScrollOffset = f.viewScrollOffset
	m.primaryMode = f.primaryMode
	m.clearStaleForRecentMode()
	m.sort = f.sort
	m.treeSort = f.treeSort
	m.focusStack = slices.Clone(f.focusStack)
	m.crossFocusStack = slices.Clone(f.crossFocusStack)
	m.zoom = f.zoom
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	m.clampCursor()
}

// scopeProjects returns the projects the outliner should currently range over.
// When scoped to a single project it returns just that project (if still
// loaded); otherwise it returns every loaded project. Callers that build rows,
// counts, and work horizons route through this so scope is honored in one place.
func (m *OutlinerModel) scopeProjects() []*models.Project {
	if m.scopedProjectID == "" {
		return m.loadedProjects
	}
	if p := m.scopedProject(); p != nil {
		return []*models.Project{p}
	}
	return nil
}

// scopedProject resolves the currently scoped project from loadedProjects, or
// nil when unscoped or the project is no longer loaded.
func (m *OutlinerModel) scopedProject() *models.Project {
	if m.scopedProjectID == "" {
		return nil
	}
	for _, p := range m.loadedProjects {
		if p != nil && p.ID == m.scopedProjectID {
			return p
		}
	}
	return nil
}

// isProjectScoped reports whether a single-project scope is active. Used to
// drive scope breadcrumbs.
func (m *OutlinerModel) isProjectScoped() bool {
	return m.project == nil && m.scopedProjectID != ""
}

// scopeChainNames returns the active project focus name. The scope model admits
// one base (All or a context) plus at most one project focus, so the chrome can
// never render a nested project chain.
func (m *OutlinerModel) scopeChainNames() []string {
	if m.scopedProjectID != "" {
		return []string{m.scopedProjectChromeLabel(m.scopedProjectID)}
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
	m.parkedFocusProjectID = ""
	if !m.baseWorkspaceAdmitsProject(projectID) {
		m.parkedFocusProjectID = projectID
	}
}

// projectFacetsAdmitProject reports whether the live project facets select the
// project on their own. It is the choice selectCrossScopeProjects makes minus
// the focus exception a load also carries, which is what lets a caller tell a
// project the facets asked for from one only the focus request appended.
func (m *OutlinerModel) projectFacetsAdmitProject(projectID string) bool {
	if m == nil || m.runtime == nil || projectID == "" {
		return false
	}
	if !hasActiveProjectFilters(&m.projectFilter) {
		// No facet is active, so the load selected the base task workspace.
		return m.baseWorkspaceAdmitsProject(projectID)
	}
	return projectFilterAdmits(m.runtime.GetProject(projectID), m.projectFilter)
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
	focused := m.parkedFocusProjectID
	m.pendingParkedFocusID = ""
	m.parkedFocusProjectID = ""
	// The population in hand was read under the live filter, so it already is
	// whatever an owed reload would have gone back for.
	m.parkedFocusReloadPending = false
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
	focused := m.parkedFocusProjectID
	m.pendingParkedFocusID = ""
	if focused == "" {
		return
	}
	m.parkedFocusProjectID = ""
	m.parkedFocusReloadPending = true
	retained := make([]*models.Project, 0, len(m.loadedProjects))
	for _, project := range m.loadedProjects {
		if project != nil && project.ID == focused {
			continue
		}
		retained = append(retained, project)
	}
	m.loadedProjects = retained
	m.pruneSelectionsOutsideLoadedProjects()
	m.crossForceRebuild = true
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
	if projectID == m.scopedProjectID {
		return
	}
	if projectID != m.parkedFocusProjectID {
		// Focusing elsewhere ends the exception that admitted the parked project.
		m.dropParkedProjectFocus()
	}
	_ = m.persistActiveRouteState()
	if m.scopedProjectID == "" {
		// Starting from a base scope creates the sole restorable frame.
		m.scopeStack = []outlinerScopeFrame{m.captureScopeFrame()}
	} else {
		// Replacing a project focus retains only the original base frame. The
		// defensive branch repairs older nested state without making it visible.
		switch {
		case len(m.scopeStack) > 0:
			m.scopeStack = m.scopeStack[:1]
		default:
			base := m.captureScopeFrame()
			base.scopedProjectID = ""
			m.scopeStack = []outlinerScopeFrame{base}
		}
	}
	m.scopedProjectID = projectID
	m.crossFocusStack = nil
	m.clearSectionZoom()
	m.cursor = 0
	m.resetViewScrollOffset()
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	m.clampCursor()
}

// popProjectScope restores the captured base scope. It reports whether a focus
// was active so the Esc cascade can fall through at the base.
func (m *OutlinerModel) popProjectScope() bool {
	if m.scopedProjectID == "" || len(m.scopeStack) == 0 {
		return false
	}
	_ = m.persistActiveRouteState()
	frame := m.scopeStack[0]
	m.scopeStack = nil
	m.dropParkedProjectFocus()
	m.restoreScopeFrame(frame)
	return true
}

// clearProjectScope drops the entire scope stack and returns to the
// authoritative all-projects scope, restoring the base frame captured at the
// very first push. Reports whether anything changed.
func (m *OutlinerModel) clearProjectScope() bool {
	if m.scopedProjectID == "" && len(m.scopeStack) == 0 {
		return false
	}
	_ = m.persistActiveRouteState()
	if len(m.scopeStack) > 0 {
		base := m.scopeStack[0]
		m.scopeStack = nil
		m.dropParkedProjectFocus()
		m.restoreScopeFrame(base)
		return true
	}
	// Defensive: scoped without a frame (should not happen); reset in place.
	m.dropParkedProjectFocus()
	m.scopedProjectID = ""
	m.crossForceRebuild = true
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
	m.confirm.Cancel()
	m.quickEdit.Exit()
	m.scopeStack = nil
	m.scopedProjectID = ""
	m.dropParkedProjectFocus()
	m.focusStack = nil
	m.crossFocusStack = nil
	// Restated rather than left to the selectOutlinerMode(ModeAll) below, which
	// also clears it: the reset names every axis it drops, the way it already
	// restates the focus-stack nils.
	m.clearSectionZoom()
	m.collapsedSections = make(map[string]bool)
	m.expandedNodes = make(map[TaskIdentity]bool)
	m.expandedChecklists = make(map[TaskIdentity]bool)
	m.sectionToggleRestore = nil
	m.replaceFilterClauses(filterQueryTaskFields(), nil)
	m.sort.Reset()
	m.treeSort.Reset()
	if m.multiSelectMode {
		m.clearMultiSelect()
		m.multiSelectMode = false
	}
	m.clearSearchProjectionWithoutRestore()
	// The reset is replacing the whole projection, so the lens is dropped rather
	// than restored: restoring its baseline would reinstate the narrowing this
	// key exists to clear.
	m.clearActiveLensWithoutRestore()
	m.cursor = 0
	m.resetViewScrollOffset()
	m.selectOutlinerMode(ModeAll)
	m.crossForceRebuild = true
	m.rebuildRows()
	m.clampCursor()
}

// reconcileProjectScope keeps the one focus valid after a reload or external
// mutation. If it vanished, the captured base is restored. Returns ("", false)
// when no repair was needed.
func (m *OutlinerModel) reconcileProjectScope() (string, bool) {
	if m.project != nil || m.scopedProjectID == "" {
		return "", false
	}
	if m.scopedProject() != nil {
		return "", false
	}
	// The focus target is gone — removed, archived, or unreadable — so stop
	// re-requesting it on every later reload.
	m.parkedFocusProjectID = ""
	m.pendingParkedFocusID = ""
	if len(m.scopeStack) > 0 {
		frame := m.scopeStack[0]
		m.scopeStack = nil
		m.restoreScopeFrame(frame)
		return "Focused project no longer exists — returned to the base scope", true
	}
	// No captured frame remained; fall back to the active context base in place.
	m.scopedProjectID = ""
	m.crossFocusStack = nil
	m.clearSectionZoom()
	m.cursor = 0
	m.resetViewScrollOffset()
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	m.clampCursor()
	return "Focused project no longer exists — returned to the base scope", true
}

// projectIsLoaded reports whether a project ID is present in loadedProjects.
func (m *OutlinerModel) projectIsLoaded(projectID string) bool {
	for _, p := range m.loadedProjects {
		if p != nil && p.ID == projectID {
			return true
		}
	}
	return false
}

// projectDisplayNameByID resolves a friendly project name from loadedProjects,
// falling back to the raw ID when the project is not loaded.
func (m *OutlinerModel) projectDisplayNameByID(projectID string) string {
	for _, p := range m.loadedProjects {
		if p != nil && p.ID == projectID {
			return p.Name
		}
	}
	return projectID
}
