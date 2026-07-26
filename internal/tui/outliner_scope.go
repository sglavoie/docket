package tui

import (
	"maps"
	"slices"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// outlinerScopeFrame captures the outliner state that a project-scope push
// replaces, so popping the scope restores every mode discriminator, filter,
// search, saved-filter, sort, pinned-tag, cursor, scroll, and zoom value owned
// per scope exactly. Every mutable map, pointer-backed criteria, and slice is
// snapshotted (deep-copied) at capture time so edits made inside a child scope
// cannot reach back and mutate a parent frame still sitting on the stack.
//
// Fold state is deliberately NOT snapshotted and remains shared across scopes:
//   - collapsedSections / expandedNodes are keyed by project and section, so a
//     scope change cannot corrupt another project's folds.
//   - crossPriorityCollapsedGroups / reviewCollapsedSections are per-computed-mode
//     fold maps reset on mode entry, not per scope.
//
// The active Context is likewise not snapshotted: applying a Context empties the
// scope stack, so no frame can outlive the Context it was pushed under.
type outlinerScopeFrame struct {
	scopedProjectID  string
	cursor           int
	viewScrollOffset int

	// Mode discriminators — the full set consumed by activeOutlinerMode().
	activePerspective int
	smartFocus        *OutlinerFocus
	lens              OutlinerLens
	workNextMode      bool
	workBacklogMode   bool
	crossPriorityMode bool
	reviewMode        reviewModeKind

	// Filter / search / saved-filter / pinned-tag narrowing.
	filter               FilterState
	searchActive         bool
	searchQuery          string
	searchInputValue     string
	activeSavedFilter    string
	activeCriteriaFilter *config.SavedFilter
	pinnedTagTabIndex    int

	// Active render ordering plus the durable tree-mode sort.
	sort     SortState
	treeSort SortState

	// Zoom stacks (single-project and cross-project).
	zoomStack      []ZoomEntry
	crossZoomStack []CrossProjectZoomEntry
}

// cloneFilterState returns a deep copy of a FilterState with all maps allocated,
// so a restored frame never shares (or nil-panics on) Tags/Attrs/Sections.
func cloneFilterState(f FilterState) FilterState {
	dup := FilterState{CommonFilterState: *copyCommonFilterState(&f.CommonFilterState)}
	if dup.Tags == nil {
		dup.Tags = make(map[string]bool)
	}
	if dup.Attrs == nil {
		dup.Attrs = make(map[string]string)
	}
	return dup
}

// cloneSavedFilter deep-copies a pointer-backed saved criteria filter so a child
// scope loading or clearing its own criteria cannot mutate a parent frame's copy.
func cloneSavedFilter(f *config.SavedFilter) *config.SavedFilter {
	if f == nil {
		return nil
	}
	dup := *f
	dup.FilterTags = slices.Clone(f.FilterTags)
	dup.SetFilterStates(f.FilterStates, f.HasStateFilter())
	dup.FilterAttrs = maps.Clone(f.FilterAttrs)
	dup.Criteria = slices.Clone(f.Criteria)
	for i := range dup.Criteria {
		dup.Criteria[i].States = cloneCommonWorkflowStateSet(f.Criteria[i].States)
	}
	return &dup
}

// cloneOutlinerFocus copies the smart-focus discriminator behind a fresh pointer.
func cloneOutlinerFocus(f *OutlinerFocus) *OutlinerFocus {
	if f == nil {
		return nil
	}
	dup := *f
	return &dup
}

// captureScopeFrame snapshots the current restorable state into a frame. Every
// mutable map/pointer/slice is deep-copied so the frame is fully independent of
// the live model.
func (m *OutlinerModel) captureScopeFrame() outlinerScopeFrame {
	return outlinerScopeFrame{
		scopedProjectID:      m.scopedProjectID,
		cursor:               m.cursor,
		viewScrollOffset:     m.viewScrollOffset,
		activePerspective:    m.activePerspective,
		smartFocus:           cloneOutlinerFocus(m.smartFocus),
		lens:                 m.lens,
		workNextMode:         m.workNextMode,
		workBacklogMode:      m.workBacklogMode,
		crossPriorityMode:    m.crossPriorityMode,
		reviewMode:           m.reviewMode,
		filter:               cloneFilterState(m.filter),
		searchActive:         m.HasActiveSearch(),
		searchQuery:          m.SearchQuery(),
		searchInputValue:     m.SearchInputValue(),
		activeSavedFilter:    m.activeSavedFilter,
		activeCriteriaFilter: cloneSavedFilter(m.activeCriteriaFilter),
		pinnedTagTabIndex:    m.pinnedTagTabIndex(),
		sort:                 m.sort,
		treeSort:             m.treeSort,
		zoomStack:            slices.Clone(m.zoomStack),
		crossZoomStack:       slices.Clone(m.crossZoomStack),
	}
}

// restoreScopeFrame applies a previously captured frame back onto the model,
// deep-copying every mutable map/pointer/slice back out so the live model never
// aliases the frame's snapshot.
func (m *OutlinerModel) restoreScopeFrame(f outlinerScopeFrame) {
	m.scopedProjectID = f.scopedProjectID
	m.cursor = f.cursor
	m.viewScrollOffset = f.viewScrollOffset
	m.activePerspective = f.activePerspective
	m.smartFocus = cloneOutlinerFocus(f.smartFocus)
	m.lens = f.lens
	m.workNextMode = f.workNextMode
	m.workBacklogMode = f.workBacklogMode
	m.crossPriorityMode = f.crossPriorityMode
	m.reviewMode = f.reviewMode
	m.filter = cloneFilterState(f.filter)
	m.SetSearchActive(f.searchActive)
	m.SetSearchQuery(f.searchQuery)
	m.SetSearchInputValue(f.searchInputValue)
	m.activeSavedFilter = f.activeSavedFilter
	m.activeCriteriaFilter = cloneSavedFilter(f.activeCriteriaFilter)
	m.setPinnedTagTabIndex(f.pinnedTagTabIndex)
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
// loaded); otherwise it returns the loaded projects admitted by the active
// Context. Callers that build rows, counts, and work horizons route through this
// so scope is honored in one place.
func (m *OutlinerModel) scopeProjects() []*models.Project {
	if m.scopedProjectID == "" {
		return m.contextScopeProjects()
	}
	if p := m.scopedProject(); p != nil {
		return []*models.Project{p}
	}
	return nil
}

// scopedTagFilterMsg builds the tag-filter picker payload for the outliner's
// current scope. Catalog tags and task counts both come from scopeProjects(), so
// an active Context (or drill-in scope) can never offer a tag whose only tasks
// live outside the scoped project set. Keyboard and command-palette entry points
// share this builder so the two can't drift apart.
func (m *OutlinerModel) scopedTagFilterMsg() ShowTagFilterMsg {
	if m.project != nil {
		return ShowTagFilterMsg{
			Tags:     BuildProjectTagList(m.project, false),
			Selected: m.filter.Tags,
			Source:   ViewOutliner,
			Title:    "Filter by Tags",
		}
	}
	return ShowTagFilterMsg{
		Tags:     BuildProjectsTagList(m.scopeProjects(), false),
		Selected: m.filter.Tags,
		Source:   ViewCrossProjectOutliner,
		Title:    "Filter by Tags",
	}
}

// scopeSkipProject reports whether a loaded project must be skipped by the
// row-building loops because a single-project scope is active and this is not
// the scoped project.
func (m *OutlinerModel) scopeSkipProject(projectID string) bool {
	return m.scopedProjectID != "" && projectID != m.scopedProjectID
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

// scopeChainNames returns the ordered list of scoped project display names, from
// the outermost pushed scope to the currently active scope. It is empty at the
// base all-projects scope. Nested scopes yield one entry per level so the scope
// chrome can render the full depth (e.g. Alpha › Beta). Each stacked frame's
// scopedProjectID is the scope that was active before that push; the base frame
// carries an empty scopedProjectID and is naturally excluded.
func (m *OutlinerModel) scopeChainNames() []string {
	var names []string
	for _, frame := range m.scopeStack {
		if frame.scopedProjectID != "" {
			names = append(names, m.projectDisplayNameByID(frame.scopedProjectID))
		}
	}
	if m.scopedProjectID != "" {
		names = append(names, m.projectDisplayNameByID(m.scopedProjectID))
	}
	return names
}

// pushProjectScope enters a single-project scope, saving the previous scope so
// Esc restores it. Fresh scopes reset cursor/scroll and clear any cross-project
// zoom so the scoped project renders from its roots.
func (m *OutlinerModel) pushProjectScope(projectID string) {
	if projectID == "" || m.project != nil {
		return
	}
	if projectID == m.scopedProjectID {
		return
	}
	m.scopeStack = append(m.scopeStack, m.captureScopeFrame())
	m.scopedProjectID = projectID
	m.crossZoomStack = nil
	m.cursor = 0
	m.resetViewScrollOffset()
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	m.clampCursor()
}

// popProjectScope restores the nearest saved scope frame. It reports whether a
// scope was actually popped so the Esc cascade can fall through to the next step
// when the outliner is already at the base all-projects scope.
func (m *OutlinerModel) popProjectScope() bool {
	if len(m.scopeStack) == 0 {
		return false
	}
	frame := m.scopeStack[len(m.scopeStack)-1]
	m.scopeStack = m.scopeStack[:len(m.scopeStack)-1]
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
	if len(m.scopeStack) > 0 {
		base := m.scopeStack[0]
		m.scopeStack = nil
		m.restoreScopeFrame(base)
		return true
	}
	// Defensive: scoped without a frame (should not happen); reset in place.
	m.scopedProjectID = ""
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	m.clampCursor()
	return true
}

// resetToGlobalAll returns the cross-project Outliner to a clean, unscoped
// global All root: it drops any project scope, clears every narrowing and ordering
// axis (active Context, search, saved criteria, common filters,
// section/actionable filters, pinned-tag tab, and manual sort), clears
// zoom/selection, activates All, and parks the cursor at the top. It is the state
// Ctrl+Esc restores as the single global reset, so no advertised clean reset can
// leave the root silently narrowed by a stale context, filter, tag, or sort.
func (m *OutlinerModel) resetToGlobalAll() {
	m.scopeStack = nil
	m.scopedProjectID = ""
	m.clearContextScope()
	m.zoomStack = nil
	m.crossZoomStack = nil
	m.activeSavedFilter = ""
	m.activeCriteriaFilter = nil
	clearCommonFilters(&m.filter.CommonFilterState)
	m.filter.Actionable = false
	m.setPinnedTagTabIndex(0)
	m.sort.Reset()
	m.treeSort.Reset()
	if m.multiSelectMode {
		m.clearMultiSelect()
		m.multiSelectMode = false
	}
	m.ClearSearch()
	m.cursor = 0
	m.resetViewScrollOffset()
	m.selectOutlinerMode(ModeAll)
	m.crossForceRebuild = true
	m.rebuildRows()
	m.clampCursor()
}

// reconcileProjectScope keeps the scope stack valid after a reload or external
// mutation. If the scoped project (or any stacked scope's project) has vanished
// from loadedProjects, it unwinds to the nearest still-valid scope and reports a
// human message describing the fallback. Returns ("", false) when no repair was
// needed.
func (m *OutlinerModel) reconcileProjectScope() (string, bool) {
	if m.project != nil || m.scopedProjectID == "" {
		return "", false
	}
	if m.scopedProject() != nil {
		return "", false
	}
	// The active scope disappeared. Pop frames until we land on a valid scope
	// (or the base all-projects scope, whose scopedProjectID is "").
	for len(m.scopeStack) > 0 {
		frame := m.scopeStack[len(m.scopeStack)-1]
		m.scopeStack = m.scopeStack[:len(m.scopeStack)-1]
		if frame.scopedProjectID == "" || m.projectIsLoaded(frame.scopedProjectID) {
			m.restoreScopeFrame(frame)
			label := "all projects"
			if frame.scopedProjectID != "" {
				label = m.projectDisplayNameByID(frame.scopedProjectID)
			}
			return "Scoped project no longer exists — returned to " + label, true
		}
	}
	// No valid frame remained; fall back to the base all-projects scope.
	m.scopedProjectID = ""
	m.crossZoomStack = nil
	m.cursor = 0
	m.resetViewScrollOffset()
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	m.clampCursor()
	return "Scoped project no longer exists — returned to all projects", true
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
