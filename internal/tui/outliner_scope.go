package tui

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
)

// outlinerScopeFrame captures the outliner state that a project-scope push
// replaces, so popping the scope restores navigation and presentation axes that
// remain scope-owned. Task facets are intentionally absent: they refine the one
// Outliner for the whole run and therefore survive every mode and scope change.
//
// Fold state is deliberately NOT snapshotted and remains shared across scopes:
//   - collapsedSections / expandedNodes are keyed by project and section, so a
//     scope change cannot corrupt another project's folds.
//   - computed section fold maps are not per-scope state.
type outlinerScopeFrame struct {
	scopedProjectID  string
	cursor           int
	viewScrollOffset int

	primaryMode OutlinerModeID

	// Active render ordering plus the durable tree-mode sort.
	sort     SortState
	treeSort SortState

	// Zoom stacks (single-project and cross-project).
	zoomStack      []ZoomEntry
	crossZoomStack []CrossProjectZoomEntry
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
		zoomStack:        slices.Clone(m.zoomStack),
		crossZoomStack:   slices.Clone(m.crossZoomStack),
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
	m.sort = f.sort
	m.treeSort = f.treeSort
	m.zoomStack = slices.Clone(f.zoomStack)
	m.crossZoomStack = slices.Clone(f.crossZoomStack)
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
	m.crossZoomStack = nil
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
// zoom/selection, activates All, and parks the cursor at the top. It is the state
// Ctrl+Esc restores as the single global reset, so no advertised clean reset can
// leave the root silently narrowed by a stale filter, tag, or sort.
func (m *OutlinerModel) resetToGlobalAll() {
	m.confirm.Cancel()
	m.quickEdit.Exit()
	m.crossQuickEditProject = nil
	m.scopeStack = nil
	m.scopedProjectID = ""
	m.dropParkedProjectFocus()
	m.zoomStack = nil
	m.crossZoomStack = nil
	m.collapsedSections = make(map[string]bool)
	m.expandedNodes = make(map[TaskIdentity]bool)
	m.sectionToggleRestore = nil
	clearCommonFilters(&m.filter.CommonFilterState)
	m.sort.Reset()
	m.treeSort.Reset()
	if m.multiSelectMode {
		m.clearMultiSelect()
		m.multiSelectMode = false
	}
	m.clearSearchProjectionWithoutRestore()
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
	m.crossZoomStack = nil
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
