package outliner

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
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

// resetToGlobalAll returns the cross-project Outliner to a clean, unscoped
// global All root: it drops any project scope, clears every narrowing and ordering
// axis (search, saved criteria, common filters and manual sort), clears
// focus/selection, activates All, and parks the cursor at the top. It is the state
// Ctrl+Esc restores as the single global reset, so no advertised clean reset can
// leave the root silently narrowed by a stale filter, tag, or sort.
func (m *OutlinerModel) ResetToGlobalAll() {
	m.Confirm.Cancel()
	m.QuickEdit.Exit()
	m.closeMenuBar()
	m.Scope.clearStack()
	m.Scope.clearProject()
	m.dropParkedProjectFocus()
	m.Focus.clear()
	m.Focus.clearCross()
	// Restated rather than left to the selectOutlinerMode(ModeAll) below, which
	// also clears it: the reset names every axis it drops, the way it already
	// restates the focus-stack clears.
	m.clearSectionZoom()
	m.Folds.ResetAll()
	m.ReplaceFilterClauses(query.FilterQueryTaskFields(), nil)
	m.Sort.Reset()
	m.TreeSort.Reset()
	if m.MultiSelectMode {
		m.ExitMultiSelect()
		m.MultiSelectMode = false
	}
	m.clearSearchProjectionWithoutRestore()
	// The reset is replacing the whole projection, so the lens is dropped rather
	// than restored: restoring its baseline would reinstate the narrowing this
	// key exists to clear.
	m.ClearActiveLensWithoutRestore()
	m.Cursor = 0
	m.resetViewScrollOffset()
	m.SelectOutlinerMode(chrome.ModeAll)
	m.Population.Invalidate()
	m.RebuildRows()
	m.ClampCursor()
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
func (m *OutlinerModel) ResetToLaunchView() ([]string, bool) {
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
	// A lens a temporary search suspended is still a lens this key clears: it owns
	// no axis right now, but an Esc would put it back, so a reset that stayed
	// silent about it would report nothing to clear while still having work to do.
	note(m.HasActiveLens() || m.HasSuspendedLens(), "lens")
	note(m.HasSearchProjection(), "query")
	note(m.Focus.Active(), "focus")
	note(m.Focus.Zoom.Active(), "zoom")
	note(m.IsProjectScoped(), "scope")
	note(query.CommonFilterFacetCount(&m.Filter.CommonFilterState) > 0, "filters")
	reload := note(query.ProjectFilterFacetCount(&m.ProjectFilter) > 0, "project filters")
	note(m.Sort.Field != query.SortByDefault || m.TreeSort.Field != query.SortByDefault, "sort")
	note(m.PrimaryMode != chrome.ModeAll, "mode")
	// The relationship trail is an axis of its own for exactly this reason: a hop
	// can land somewhere the launch view already describes, and every note above
	// would then report nothing to clear while an Esc still had a return to make.
	note(m.HasRelationTrail(), "relationship trail")
	if len(cleared) == 0 {
		return cleared, false
	}

	// The trail goes first and is discarded rather than walked: this key restores
	// the launch view, and returning through a hop would restore a view instead.
	m.ClearRelationTrail()
	// The lens goes first and goes without its baseline: restoring it would
	// reinstate the narrowing this key exists to clear.
	m.forgetActiveLensFrame()
	// The query goes next, so no ranked projection is left scored over the
	// population the axes below are about to replace.
	m.ClearSearch()
	// Focus and zoom unwind in the Esc cascade's own order. Focus is a stack and
	// its key pops one level, so it is popped until it is empty.
	for m.PopFocusLevel() {
	}
	m.ExitSectionZoom()
	// Scope restores the base frame captured at the first push, and that frame
	// carries a mode, a sort, and a zoom of its own, so it lands before the axes
	// below rather than after them.
	m.ClearProjectScope()
	// The two filter families are cleared separately for the same reason fx and
	// Fx are two keys: clearing one must never clear the other.
	m.ReplaceFilterClauses(query.FilterQueryTaskFields(), nil)
	m.ReplaceFilterClauses(FilterOperationFields(operation.OperationFilterProjectClear), nil)
	m.Sort.Reset()
	m.TreeSort.Reset()
	// Selecting the mode is the last write and owns the rebuild, so the rows are
	// built once, over every axis this reset has already put down.
	m.SelectOutlinerMode(chrome.ModeAll)
	// A read already in flight is carrying the position this reset has just
	// discarded. It keeps running — its projects are still the newest reading —
	// but it is re-aimed at the launch view, so landing cannot undo the key.
	m.rebaseReloadTargetAfterRelocation()
	return cleared, reload
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

// focusSubtreeAtCursor enters a descendants-only projection rooted at the
// selected task. Each frame retains the exact parent cursor and scroll offset,
// so nested Focus levels unwind without moving the viewport.
func (m *OutlinerModel) FocusSubtreeAtCursor() bool {
	if m == nil || m.MultiSelectMode || m.ShowsRankedList() ||
		m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return false
	}
	identity := m.TaskIdentityForRow(row)
	task, _ := m.TaskAndProjectForIdentity(identity)
	if task == nil || len(task.Children) == 0 {
		return false
	}

	m.pushFocusFrame(identity, m.Cursor, m.ViewScrollOffset)
	m.PeekMode = false
	m.Cursor = 0
	m.ViewScrollOffset = 0
	m.RebuildRows()
	m.seatCursorAtProjectionTop()
	return true
}

// seatCursorAtProjectionTop puts the cursor on the first row of a projection
// that was just rebuilt from scratch, skipping the focused-project header when
// content follows it. It is called only where the cursor was deliberately reset
// to the top — a scope push and a subtree focus — and never on an ordinary
// rebuild, where the header is present on both sides of the pass and every
// captured index already counts it. The header is still where the cursor lands
// when the projection holds nothing else, which is what keeps an empty project
// reachable.
func (m *OutlinerModel) seatCursorAtProjectionTop() {
	m.Cursor = 0
	if len(m.Rows) > 1 && m.Rows[0].IsFocusedProjectHeader {
		m.Cursor = 1
	}
	m.ClampCursor()
}

// focusParentAtCursor widens Focus by exactly one level, keeping the cursor on
// the task it was pressed from so its siblings come into view. It reads the
// cursor only to open Focus; once focused it climbs from the focus root, which
// is the sole rule that terminates. Widening toward the cursor's parent instead
// would push a nested frame back down whenever the cursor sat deeper than a
// direct child, and the next press would climb out of it again forever.
func (m *OutlinerModel) FocusParentAtCursor() bool {
	if m == nil || m.MultiSelectMode || m.ShowsRankedList() ||
		m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return false
	}
	origin := m.TaskIdentityForRow(row)
	if _, focused := m.FocusRootIdentity(); focused {
		m.climbFocusLevel()
	} else {
		task, _ := m.TaskAndProjectForIdentity(origin)
		parentID := parentIDOf(task)
		if parentID == "" {
			return false
		}
		m.pushFocusFrame(
			taskref.TaskIdentity{ProjectID: origin.ProjectID, TaskID: parentID},
			m.Cursor,
			m.ViewScrollOffset,
		)
	}
	m.PeekMode = false
	m.RebuildRows()
	m.MoveCursorToIdentity(origin)
	return true
}

// pushFocusFrame records one Focus frame in whichever stack the current scope
// owns, retaining the cursor and scroll offset Esc restores.
func (m *OutlinerModel) pushFocusFrame(identity taskref.TaskIdentity, cursorIndex, scrollOffset int) {
	if m.Project != nil {
		m.Focus.push(FocusEntry{
			TaskID:       identity.TaskID,
			CursorIndex:  cursorIndex,
			ScrollOffset: scrollOffset,
		})
		return
	}
	m.Focus.pushCross(CrossProjectFocusEntry{
		TaskID:       identity.TaskID,
		ProjectID:    identity.ProjectID,
		CursorIndex:  cursorIndex,
		ScrollOffset: scrollOffset,
	})
	m.Population.Invalidate()
}

// climbFocusLevel replaces the top frame with one rooted at the focus root's own
// parent. Popping alone already lands there when the frame beneath is that
// parent; otherwise the popped frame spanned more than one level and the parent
// is pushed back, which keeps descending from the frame beneath it.
func (m *OutlinerModel) climbFocusLevel() {
	root, focused := m.FocusRootIdentity()
	if !focused {
		return
	}
	rootTask, _ := m.TaskAndProjectForIdentity(root)
	cursorIndex, scrollOffset := m.popFocusFrame()
	m.Cursor, m.ViewScrollOffset = cursorIndex, scrollOffset

	parent := taskref.TaskIdentity{ProjectID: root.ProjectID, TaskID: parentIDOf(rootTask)}
	if parent.TaskID == "" {
		return
	}
	if beneath, stillFocused := m.FocusRootIdentity(); stillFocused && beneath == parent {
		return
	}
	m.pushFocusFrame(parent, cursorIndex, scrollOffset)
}

// popFocusFrame removes the top Focus frame without rebuilding, returning the
// viewport it retained. popFocusLevel is Esc's inverse of Enter and rebuilds;
// the climb needs the frame gone before it decides what to push in its place.
func (m *OutlinerModel) popFocusFrame() (cursorIndex, scrollOffset int) {
	if top, popped := m.Focus.pop(); popped {
		return top.CursorIndex, top.ScrollOffset
	}
	if top, popped := m.Focus.popCross(); popped {
		m.Population.Invalidate()
		return top.CursorIndex, top.ScrollOffset
	}
	return m.Cursor, m.ViewScrollOffset
}

// focusRootIdentity names the task the current Focus projection is rooted at.
// Rebuild-time focus-stack validation keeps the frame live, so a caller still
// resolves the identity against loaded state before acting on it.
func (m *OutlinerModel) FocusRootIdentity() (taskref.TaskIdentity, bool) {
	if m == nil {
		return taskref.TaskIdentity{}, false
	}
	var identity taskref.TaskIdentity
	switch {
	case len(m.Focus.CrossStack) > 0:
		top := m.Focus.CrossStack[len(m.Focus.CrossStack)-1]
		identity = taskref.TaskIdentity{ProjectID: top.ProjectID, TaskID: top.TaskID}
	case m.Project != nil && len(m.Focus.Stack) > 0:
		top := m.Focus.Stack[len(m.Focus.Stack)-1]
		identity = taskref.TaskIdentity{ProjectID: m.Project.ID, TaskID: top.TaskID}
	default:
		return taskref.TaskIdentity{}, false
	}
	return identity, identity.Valid()
}

// popFocusLevel restores one Focus frame. It is deliberately one-level-at-a-
// time so Esc remains a predictable inverse of nested Enter operations.
func (m *OutlinerModel) PopFocusLevel() bool {
	if m == nil {
		return false
	}
	if top, popped := m.Focus.pop(); popped {
		m.Cursor = top.CursorIndex
		m.ViewScrollOffset = top.ScrollOffset
		m.RebuildRows()
		return true
	}
	if top, popped := m.Focus.popCross(); popped {
		m.Cursor = top.CursorIndex
		m.ViewScrollOffset = top.ScrollOffset
		m.Population.Invalidate()
		m.RebuildRows()
		return true
	}
	return false
}

// enterSectionZoom narrows the projection to the one section the header names,
// retaining the viewport Esc restores. Unlike Focus, the world narrows around a
// stationary cursor: the zoomed header is the surviving projection's first row,
// so the cursor keeps naming the row it was pressed on. Re-entering the section
// already zoomed to changes nothing, which keeps the retained viewport the
// pre-zoom one.
func (m *OutlinerModel) EnterSectionZoom(row OutlinerRow) bool {
	if m == nil || m.MultiSelectMode || m.ShowsRankedList() || !row.IsSectionHeader {
		return false
	}
	key := row.EffectiveCollapseKey()
	if m.Focus.Zoom.key == key {
		return true
	}
	m.Focus.enterZoom(key, row.SectionName, m.Cursor, m.ViewScrollOffset)
	m.PeekMode = false
	m.Population.Invalidate()
	m.RebuildRows()
	if !m.moveCursorToGroupKey(key) {
		m.ClampCursor()
	}
	return true
}

// exitSectionZoom widens back to the whole mode, restoring the viewport the zoom
// was entered from. Work completed inside the zoom can have shortened the list
// that cursor was taken against, so the restore clamps.
func (m *OutlinerModel) ExitSectionZoom() bool {
	if m == nil || !m.Focus.Zoom.Active() {
		return false
	}
	cursor, scroll := m.Focus.exitZoom()
	m.Cursor = cursor
	m.ViewScrollOffset = scroll
	m.Population.Invalidate()
	m.RebuildRows()
	m.ClampCursor()
	return true
}

// clearSectionZoom drops the zoom for a transition that reshapes the projection
// on its own terms — a scope push, a mode change, a reveal — so the pre-zoom
// viewport exitSectionZoom would restore is discarded with it. The force flag is
// set here rather than left to the fingerprint so every clear invalidates the
// cross-project memoization guard the same way a scope push does.
func (m *OutlinerModel) clearSectionZoom() {
	if m == nil || !m.Focus.Zoom.Active() {
		return
	}
	m.Focus.clearZoom()
	m.Population.Invalidate()
}

// zoomSectionAtCursor narrows the Outliner to the section the cursor header
// names.
func (m *OutlinerModel) ZoomSectionAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	return m.EnterSectionZoom(m.Rows[m.Cursor])
}

// activateGroupAtCursor scopes the Outliner to the project the cursor sub-header
// names. Enter on a section header is the separate zoom act.
func (m *OutlinerModel) ActivateGroupAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	// The focused-project header names the scope the Outliner is already in, so
	// Enter there has nowhere to go. It refuses here as well as in the
	// descriptor's applicability, so the key is silent rather than reporting a
	// scope push that pushed nothing.
	if !row.IsProjectHeader || row.IsFocusedProjectHeader || row.ProjectID == "" {
		return false
	}
	m.PushProjectScope(row.ProjectID)
	return true
}

// ApplyProjectScopeMsg applies a Project Hub or project-header selection to the
// sole Outliner scope stack. It is declared here rather than on the shell
// because every surface that can name a project raises it, and the scope it
// names is this package's.
type ApplyProjectScopeMsg struct {
	ProjectID string
	Clear     bool
}
