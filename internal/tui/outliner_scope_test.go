package tui

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// scopeTestProject builds a project with a couple of active root tasks so the
// All-perspective row builder emits a project sub-header followed by task rows.
func scopeTestProject(id, name string, taskIDs ...string) *models.Project {
	var tasks []*models.Task
	for _, tid := range taskIDs {
		t := makeTask(id+"-"+tid, name+" "+tid, "")
		t.WorkflowState = models.StateReady
		tasks = append(tasks, t)
	}
	proj := &models.Project{ID: id, Name: name, Tasks: tasks}
	proj.BuildTaskTree()
	return proj
}

func scopeTestOutliner(projects ...*models.Project) *OutlinerModel {
	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[TaskIdentity]bool),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    projects,
		filterProjects:    make(map[string]bool),
		filter:            NewFilterState(),
	}
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	return m
}

func scopeVisibleProjectIDs(m *OutlinerModel) map[string]bool {
	ids := make(map[string]bool)
	for _, row := range m.rows {
		if row.isTaskRow() && row.task != nil {
			ids[row.ProjectID] = true
		}
	}
	return ids
}

func TestPushProjectScopeNarrowsToOneProject(t *testing.T) {
	projA := scopeTestProject("A", "Alpha", "1", "2")
	projB := scopeTestProject("B", "Bravo", "1")
	m := scopeTestOutliner(projA, projB)

	// Baseline: both projects visible.
	if ids := scopeVisibleProjectIDs(m); !ids["A"] || !ids["B"] {
		t.Fatalf("expected both projects visible before scope, got %v", ids)
	}

	m.pushProjectScope("A")

	if m.scopedProjectID != "A" {
		t.Fatalf("expected scopedProjectID=A, got %q", m.scopedProjectID)
	}
	ids := scopeVisibleProjectIDs(m)
	if !ids["A"] || ids["B"] {
		t.Fatalf("expected only project A visible after scope, got %v", ids)
	}
	if len(m.loadedProjects) != 2 {
		t.Fatalf("scope must not mutate the authoritative loaded set; got %d", len(m.loadedProjects))
	}
	if !m.isProjectScoped() {
		t.Fatal("expected isProjectScoped() true after push")
	}
}

func TestPopProjectScopeRestoresPreviousScope(t *testing.T) {
	projA := scopeTestProject("A", "Alpha", "1")
	projB := scopeTestProject("B", "Bravo", "1")
	m := scopeTestOutliner(projA, projB)
	m.cursor = 2 // some non-zero cursor in the all-projects view

	m.pushProjectScope("A")
	if m.cursor != 0 {
		t.Fatalf("expected cursor reset to 0 on push, got %d", m.cursor)
	}

	if popped := m.popProjectScope(); !popped {
		t.Fatal("expected popProjectScope to report a pop")
	}
	if m.scopedProjectID != "" {
		t.Fatalf("expected base scope after pop, got %q", m.scopedProjectID)
	}
	if m.cursor != 2 {
		t.Fatalf("expected cursor restored to 2 after pop, got %d", m.cursor)
	}
	if ids := scopeVisibleProjectIDs(m); !ids["A"] || !ids["B"] {
		t.Fatalf("expected both projects visible after pop, got %v", ids)
	}
}

func TestPopProjectScopeAtBaseReportsNoPop(t *testing.T) {
	m := scopeTestOutliner(scopeTestProject("A", "Alpha", "1"))
	if m.popProjectScope() {
		t.Fatal("expected no pop at base all-projects scope")
	}
}

func TestPushProjectScopeDisablesMultiProjectFilterAndRestoresOnPop(t *testing.T) {
	projA := scopeTestProject("A", "Alpha", "1")
	projB := scopeTestProject("B", "Bravo", "1")
	m := scopeTestOutliner(projA, projB)
	// Simulate an active multi-project filter (F P).
	m.filterProjects = map[string]bool{"A": true, "B": true}

	m.pushProjectScope("A")
	if len(m.filterProjects) != 0 {
		t.Fatalf("expected multi-project filter disabled inside scope, got %v", m.filterProjects)
	}

	m.popProjectScope()
	if !m.filterProjects["A"] || !m.filterProjects["B"] {
		t.Fatalf("expected multi-project filter restored on pop, got %v", m.filterProjects)
	}
}

func TestClearProjectScopeReturnsToAllProjects(t *testing.T) {
	projA := scopeTestProject("A", "Alpha", "1")
	projB := scopeTestProject("B", "Bravo", "1")
	projC := scopeTestProject("C", "Charlie", "1")
	m := scopeTestOutliner(projA, projB, projC)

	// Nested scopes: A then C.
	m.pushProjectScope("A")
	m.pushProjectScope("C")
	if len(m.scopeStack) != 2 {
		t.Fatalf("expected 2 stacked frames, got %d", len(m.scopeStack))
	}

	if !m.clearProjectScope() {
		t.Fatal("expected clearProjectScope to report a change")
	}
	if m.scopedProjectID != "" || len(m.scopeStack) != 0 {
		t.Fatalf("expected empty scope stack at all-projects, got id=%q stack=%d", m.scopedProjectID, len(m.scopeStack))
	}
	if ids := scopeVisibleProjectIDs(m); !ids["A"] || !ids["B"] || !ids["C"] {
		t.Fatalf("expected all projects visible after clear, got %v", ids)
	}
}

func TestClearProjectScopeNoopAtBase(t *testing.T) {
	m := scopeTestOutliner(scopeTestProject("A", "Alpha", "1"))
	if m.clearProjectScope() {
		t.Fatal("expected clearProjectScope to be a no-op at base scope")
	}
}

func TestProjectHeaderEnterPushesScope(t *testing.T) {
	projA := scopeTestProject("A", "Alpha", "1")
	projB := scopeTestProject("B", "Bravo", "1")
	m := scopeTestOutliner(projA, projB)

	headerIdx := -1
	for i, row := range m.rows {
		if row.IsProjectHeader {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		t.Fatal("expected at least one project sub-header row in All perspective")
	}
	wantProject := m.rows[headerIdx].ProjectID
	m.cursor = headerIdx

	_, _, handled := m.crossHandleEnterKey()
	if !handled {
		t.Fatal("expected Enter handled in cross-project mode")
	}
	if m.scopedProjectID != wantProject {
		t.Fatalf("expected Enter on project header to scope to %q, got %q", wantProject, m.scopedProjectID)
	}
}

func TestReconcileProjectScopeRecoversWhenScopedProjectDisappears(t *testing.T) {
	projA := scopeTestProject("A", "Alpha", "1")
	projB := scopeTestProject("B", "Bravo", "1")
	m := scopeTestOutliner(projA, projB)

	m.pushProjectScope("A")

	// Simulate project A being archived/trashed externally: it vanishes from the
	// reloaded set.
	m.loadedProjects = []*models.Project{projB}

	msg, repaired := m.reconcileProjectScope()
	if !repaired {
		t.Fatal("expected reconcile to repair a vanished scope")
	}
	if m.scopedProjectID != "" {
		t.Fatalf("expected fallback to all-projects, got %q", m.scopedProjectID)
	}
	if msg == "" {
		t.Fatal("expected a human-readable recovery message")
	}
}

func TestReconcileProjectScopeUnwindsToNearestValidScope(t *testing.T) {
	projA := scopeTestProject("A", "Alpha", "1")
	projB := scopeTestProject("B", "Bravo", "1")
	m := scopeTestOutliner(projA, projB)

	m.pushProjectScope("A") // frame0: base (scopedProjectID "")
	m.pushProjectScope("B") // frame1: scope A

	// Project B disappears; nearest valid scope is A.
	m.loadedProjects = []*models.Project{projA}

	_, repaired := m.reconcileProjectScope()
	if !repaired {
		t.Fatal("expected reconcile to repair")
	}
	if m.scopedProjectID != "A" {
		t.Fatalf("expected unwind to nearest valid scope A, got %q", m.scopedProjectID)
	}
}

// seedAllOutlinerNarrowing drives every independent narrowing and ordering axis
// of the cross Outliner into a non-default state so a subsequent clean reset must
// prove it clears all of them, not just the ones the previous reset handled.
func seedAllOutlinerNarrowing(m *OutlinerModel, projectID, taskID string) {
	// Project scope (Ctrl+X) plus a stacked frame.
	m.scopeStack = []outlinerScopeFrame{{}}
	m.scopedProjectID = projectID
	// Multi-project filter (F P).
	m.filterProjects = map[string]bool{projectID: true}
	// Search (/).
	m.SetSearchActive(true)
	m.SetSearchQuery("alpha")
	// Saved filter + active criteria.
	m.activeSavedFilter = "My Filter"
	m.activeCriteriaFilter = &config.SavedFilter{Name: "crit", Mode: "criteria"}
	// Common filters (priority + state).
	m.filter.Priority = 1
	m.filter.State = models.StateReady
	// Section filter (F @).
	m.filter.Sections = map[string]bool{"inbox": true}
	// Actionable filter (F r).
	m.filter.Actionable = true
	// Pinned-tag tab (a non-"All" tab).
	m.setPinnedTagTabIndex(2)
	// Manual sort (1-5).
	m.sort = SortState{Field: SortByPriority, Ascending: false}
	m.treeSort = m.sort
	// Zoom stacks.
	m.zoomStack = []ZoomEntry{{TaskID: taskID, CursorIndex: 1}}
	m.crossZoomStack = []CrossProjectZoomEntry{{TaskID: taskID, ProjectID: projectID, CursorIndex: 1}}
	// A transient review mode.
	m.reviewMode = reviewDaily
	// Multi-select with one member.
	m.multiSelectMode = true
	m.selectedTaskIdentities = map[TaskIdentity]bool{{ProjectID: projectID, TaskID: taskID}: true}
	// A non-Today horizon and a parked cursor.
	m.workTodayMode = false
	m.workBacklogMode = true
	m.cursor = 4
}

// assertGlobalTodayClean asserts every narrowing/ordering axis is back to its
// clean unscoped-global-Today default and the cursor is parked at the top.
func assertGlobalTodayClean(t *testing.T, m *OutlinerModel) {
	t.Helper()
	if m.scopedProjectID != "" || len(m.scopeStack) != 0 {
		t.Fatalf("project scope not cleared: id=%q stack=%d", m.scopedProjectID, len(m.scopeStack))
	}
	if len(m.filterProjects) != 0 {
		t.Fatalf("multi-project filter not cleared: %v", m.filterProjects)
	}
	if m.SearchQuery() != "" {
		t.Fatalf("search not cleared: %q", m.SearchQuery())
	}
	if m.activeSavedFilter != "" || m.activeCriteriaFilter != nil {
		t.Fatalf("saved filter not cleared: saved=%q criteria=%v", m.activeSavedFilter, m.activeCriteriaFilter)
	}
	if hasActiveCommonFilters(&m.filter.CommonFilterState) {
		t.Fatalf("common filters not cleared: %+v", m.filter.CommonFilterState)
	}
	if len(m.filter.Sections) != 0 {
		t.Fatalf("section filter not cleared: %v", m.filter.Sections)
	}
	if m.filter.Actionable {
		t.Fatal("actionable filter not cleared")
	}
	if m.pinnedTagTabIndex() != 0 {
		t.Fatalf("pinned-tag tab not reset: %d", m.pinnedTagTabIndex())
	}
	if m.sort.Field != SortByDefault {
		t.Fatalf("manual sort not reset: %+v", m.sort)
	}
	if m.treeSort.Field != SortByDefault {
		t.Fatalf("stored tree sort not reset: %+v", m.treeSort)
	}
	if len(m.zoomStack) != 0 || len(m.crossZoomStack) != 0 {
		t.Fatalf("zoom not cleared: single=%d cross=%d", len(m.zoomStack), len(m.crossZoomStack))
	}
	if m.reviewMode != reviewNone {
		t.Fatalf("review mode not cleared: %v", m.reviewMode)
	}
	if m.multiSelectMode || m.SelectedCount() != 0 {
		t.Fatalf("selection not cleared: mode=%v count=%d", m.multiSelectMode, m.SelectedCount())
	}
	if !m.workTodayMode {
		t.Fatalf("reset did not land in global Today mode (active=%v)", m.activeOutlinerMode())
	}
	if m.cursor != 0 {
		t.Fatalf("cursor not parked at top: %d", m.cursor)
	}
}

// TestResetToGlobalTodayClearsAllNarrowing locks FR-003: the advertised Ctrl+Esc
// clean reset must drop EVERY narrowing and ordering axis — not just scope,
// search, saved criteria, zoom, and selection, but also the common/section/
// actionable filters, pinned-tag tab, and manual sort that the earlier reset left
// live — and always land at cursor zero in unscoped global Today.
func TestResetToGlobalTodayClearsAllNarrowing(t *testing.T) {
	t.Run("AppModel Ctrl+Esc path clears every seeded narrowing axis", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-reset", "reset", "Alpha", "Bravo")
		app, cross, _ := newPlanningOrderApp(t, project)
		seedAllOutlinerNarrowing(cross, project.ID, tasks[0].ID)

		// Drive the real global reset through the shell, exactly as production does.
		app.Update(ctrlEscKeyPress())

		assertGlobalTodayClean(t, app.crossOutliner)
	})

	t.Run("direct reset lands rows at cursor zero in global Today", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-reset2", "reset2", "Alpha", "Bravo")
		app, cross, _ := newPlanningOrderApp(t, project)
		// Put both tasks in Today so the reset has real rows to render.
		seedDailyPlanOrder(t, app, locatorOf(project, tasks[0]), locatorOf(project, tasks[1]))
		seedAllOutlinerNarrowing(cross, project.ID, tasks[0].ID)

		cross.resetToGlobalToday()

		assertGlobalTodayClean(t, cross)
		// loadedProjects is intact on the direct path, so Today renders the daily
		// plan unscoped across the workspace with the cursor at the top.
		if len(cross.rows) == 0 {
			t.Fatal("expected global Today to render rows after reset")
		}
	})
}

// scopeVariantOutliner builds a runtime-backed cross-project Outliner whose
// loaded projects carry a "backend"-tagged task, so the global pinned-tag tab
// "backend" (index 1) is a valid selection the scope frame can capture/restore.
func scopeVariantOutliner(t *testing.T) (*OutlinerModel, []*models.Task) {
	t.Helper()
	projA := scopeTestProject("A", "Alpha", "1", "2")
	projB := scopeTestProject("B", "Bravo", "1", "2")
	// Pin a shared tag so a non-"All" pinned-tag tab exists to restore.
	projA.Tasks[0].Tags = []string{"backend"}
	projA.BuildTaskTree()

	cfg := newTestConfig()
	cfg.Settings().GlobalPinnedTags = []string{"backend"}

	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(nil, cfg),
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[TaskIdentity]bool),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{projA, projB},
		filterProjects:    make(map[string]bool),
		filter:            NewFilterState(),
		sort:              NewSortState(),
		treeSort:          NewSortState(),
		pinnedTagTab:      make(map[string]int),
	}
	m.InitSearchInput("Search tasks...", 100, 30)
	return m, projA.Tasks
}

// ptrFocus returns a heap pointer to a smart-focus discriminator.
func ptrFocus(f OutlinerFocus) *OutlinerFocus { return &f }

// applyScopeVariant drives every scope-frame-owned field into one of three
// mutually distinct configurations. The variants never overlap on any captured
// field, so a pop that reproduces the parent variant proves exact per-field
// restoration rather than an accidental default match. scopedProjectID is owned
// by push/pop and is deliberately left untouched here.
func applyScopeVariant(m *OutlinerModel, v int, realTaskID string) {
	switch v {
	case 0:
		// Tree/perspective mode with a live cross-project zoom so the zoom
		// stack is exercised in the only mode where it is meaningful.
		m.activePerspective = PerspectiveAll
		m.smartFocus = nil
		m.lens = OutlinerLensTree
		m.workTodayMode, m.workNextMode, m.workBacklogMode = false, false, false
		m.crossPriorityMode = false
		m.reviewMode = reviewNone
		m.filter = NewFilterState()
		m.filterProjects = map[string]bool{"A": true}
		m.SetSearchActive(false)
		m.SetSearchQuery("")
		m.SetSearchInputValue("")
		m.activeSavedFilter = ""
		m.activeCriteriaFilter = nil
		m.setPinnedTagTabIndex(1)
		m.sort = SortState{Field: SortByCreatedAt, Ascending: true}
		m.treeSort = m.sort
		m.zoomStack = []ZoomEntry{{TaskID: "single-zoom", CursorIndex: 3}}
		m.crossZoomStack = []CrossProjectZoomEntry{{TaskID: realTaskID, ProjectID: "A", CursorIndex: 0}}
		m.cursor = 0
		m.viewScrollOffset = 0
	case 1:
		// Daily Review computed mode with a full filter/search/saved-filter load.
		m.activePerspective = PerspectiveNext
		m.smartFocus = ptrFocus(OutlinerHighPriority)
		m.lens = OutlinerLensDueDate
		m.workTodayMode, m.workNextMode, m.workBacklogMode = false, false, true
		m.crossPriorityMode = true
		m.reviewMode = reviewDaily
		m.filter = NewFilterState()
		m.filter.Priority = 1
		m.filter.State = models.StateReady
		m.filter.Actionable = true
		m.filter.Sections = map[string]bool{"inbox": true}
		m.filter.Tags = map[string]bool{"backend": true}
		m.filterProjects = map[string]bool{}
		m.SetSearchActive(true)
		m.SetSearchQuery("alpha")
		m.SetSearchInputValue("alpha")
		m.activeSavedFilter = "Filter One"
		m.activeCriteriaFilter = &config.SavedFilter{ID: "sf-1", Name: "crit1", Mode: "criteria"}
		m.setPinnedTagTabIndex(0)
		m.sort = NewSortState()
		m.treeSort = SortState{Field: SortByPriority, Ascending: false}
		m.zoomStack = nil
		m.crossZoomStack = nil
		m.cursor = 1
		m.viewScrollOffset = 0
	case 2:
		// Next computed mode with a different filter/search/sort load.
		m.activePerspective = PerspectiveActive
		m.smartFocus = ptrFocus(OutlinerBlocked)
		m.lens = OutlinerLensTree
		m.workTodayMode, m.workNextMode, m.workBacklogMode = false, true, false
		m.crossPriorityMode = false
		m.reviewMode = reviewActivity
		m.filter = NewFilterState()
		m.filter.Priority = 2
		m.filter.State = models.StateInProgress
		m.filter.Actionable = false
		m.filter.Sections = map[string]bool{"next": true}
		m.filter.Tags = map[string]bool{}
		m.filterProjects = map[string]bool{"B": true}
		m.SetSearchActive(true)
		m.SetSearchQuery("bravo")
		m.SetSearchInputValue("bravo")
		m.activeSavedFilter = "Filter Two"
		m.activeCriteriaFilter = &config.SavedFilter{ID: "sf-2", Name: "crit2", Mode: "criteria"}
		m.setPinnedTagTabIndex(1)
		m.sort = NewSortState()
		m.treeSort = SortState{Field: SortByDueDate, Ascending: true}
		m.zoomStack = nil
		m.crossZoomStack = nil
		m.cursor = 2
		m.viewScrollOffset = 1
	}
}

// TestProjectScopeRestoresModeFiltersAndSort locks FR-005: a scope frame must
// capture and restore every mode discriminator, filter/search/saved-filter
// value, sort, pinned-tag selection, cursor, scroll, and zoom value owned per
// scope. Nested scopes each drive every captured field into a distinct state
// and prove exact restoration at each pop, including that edits inside a child
// scope never mutate a parent frame's snapshotted maps or pointer criteria.
func TestProjectScopeRestoresModeFiltersAndSort(t *testing.T) {
	m, tasks := scopeVariantOutliner(t)
	realTaskID := tasks[0].ID

	normalize := func() {
		m.crossForceRebuild = true
		m.crossProjectRebuildRows()
		m.clampCursor()
	}

	// Base scope (variant 0): tree mode with a live zoom.
	applyScopeVariant(m, 0, realTaskID)
	normalize()
	want0 := m.captureScopeFrame()

	// Push into project A, load variant 1 (Daily Review + full filter set).
	m.pushProjectScope("A")
	if m.scopedProjectID != "A" {
		t.Fatalf("expected scope A, got %q", m.scopedProjectID)
	}
	applyScopeVariant(m, 1, realTaskID)
	normalize()
	want1 := m.captureScopeFrame()

	// Push into project B, load variant 2 (Next mode + a different filter set).
	m.pushProjectScope("B")
	if m.scopedProjectID != "B" {
		t.Fatalf("expected scope B, got %q", m.scopedProjectID)
	}
	applyScopeVariant(m, 2, realTaskID)
	normalize()

	// The child's edits must not have leaked into the parent frames' snapshots.
	if reflect.DeepEqual(m.captureScopeFrame(), want1) {
		t.Fatal("scope B state unexpectedly equals scope A snapshot; variants must differ")
	}
	assertParentFrameUnmutated(t, m.scopeStack[len(m.scopeStack)-1], want1)

	// Pop B → exact restoration of scope A.
	if !m.popProjectScope() {
		t.Fatal("expected pop from scope B")
	}
	if m.scopedProjectID != "A" {
		t.Fatalf("expected restoration to scope A, got %q", m.scopedProjectID)
	}
	if got := m.captureScopeFrame(); !reflect.DeepEqual(got, want1) {
		t.Fatalf("scope A not restored exactly on pop:\n got=%+v\nwant=%+v", got, want1)
	}

	// Pop A → exact restoration of the base scope, including the zoom stack.
	if !m.popProjectScope() {
		t.Fatal("expected pop from scope A")
	}
	if m.scopedProjectID != "" {
		t.Fatalf("expected restoration to base scope, got %q", m.scopedProjectID)
	}
	if got := m.captureScopeFrame(); !reflect.DeepEqual(got, want0) {
		t.Fatalf("base scope not restored exactly on pop:\n got=%+v\nwant=%+v", got, want0)
	}
}

// assertParentFrameUnmutated proves the deep-copy contract: the frame still on
// the scope stack must equal the snapshot taken before the child mutated live
// state, i.e. the child's map/pointer edits did not reach back into the frame.
func assertParentFrameUnmutated(t *testing.T, frame, want outlinerScopeFrame) {
	t.Helper()
	if !reflect.DeepEqual(frame, want) {
		t.Fatalf("child scope edits mutated the parent frame:\nframe=%+v\n want=%+v", frame, want)
	}
}

// TestProjectNavigationUsesSingleOutlinerScope locks FR12-001: project
// navigation must represent a project as a scope frame on the single
// authoritative Outliner rather than constructing a separate AppModel.outliner
// root behind a project-scoped TaskWorkspace route. The root route stays
// app-scoped, Esc restores the previous scope, and task deep links reveal the
// target inside the scoped Outliner.
func TestProjectNavigationUsesSingleOutlinerScope(t *testing.T) {
	t.Run("ShowOutlinerMsg pushes a scope frame without building m.outliner", func(t *testing.T) {
		project, _ := planningOrderProject(t, t.TempDir(), "p-nav", "nav", "Alpha", "Bravo")
		app, cross, _ := newPlanningOrderApp(t, project)

		handled, _ := app.handleProjectViewMessages(ShowOutlinerMsg{Project: project})
		if !handled {
			t.Fatal("ShowOutlinerMsg should be handled")
		}
		if app.outliner != nil {
			t.Fatal("project navigation must not construct a separate AppModel.outliner root")
		}
		if cross.scopedProjectID != project.ID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, project.ID)
		}
		if len(cross.scopeStack) == 0 {
			t.Fatal("expected a saved scope frame so Esc restores the previous scope")
		}
		if app.baseProjectView() != ViewCrossProjectOutliner {
			t.Fatalf("base view = %v, want the single ViewCrossProjectOutliner root", app.baseProjectView())
		}

		// Esc semantics: the pushed scope pops back to clean global scope.
		if !cross.popProjectScope() {
			t.Fatal("expected the pushed project scope to pop")
		}
		if cross.scopedProjectID != "" {
			t.Fatalf("after pop scopedProjectID = %q, want global (empty)", cross.scopedProjectID)
		}
	})

	t.Run("typed navigation keeps the root TaskWorkspace app-scoped", func(t *testing.T) {
		project, _ := planningOrderProject(t, t.TempDir(), "p-typed", "typed", "Alpha", "Bravo")
		app, cross, _ := newPlanningOrderApp(t, project)
		app.enableTypedNavigation()

		if _, cmd := app.handleProjectViewMessages(ShowOutlinerMsg{Project: project}); cmd != nil {
			cmd()
		}
		if app.outliner != nil {
			t.Fatal("typed project navigation must not construct AppModel.outliner")
		}
		route := app.navigation.Primary.Current()
		if route.ID() != TaskWorkspace {
			t.Fatalf("root route = %v, want TaskWorkspace", route.ID())
		}
		params, ok := route.Params().(TaskWorkspaceParams)
		if !ok || params.Scope.Kind != ScopeApp {
			t.Fatalf("root scope = %v, want app-scoped (project selection is a scope frame)", params.Scope)
		}
		if cross.scopedProjectID != project.ID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, project.ID)
		}
	})

	t.Run("task deep link scopes to the owning project and reveals the task", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-link", "link", "Alpha", "Bravo")
		app, cross, _ := newPlanningOrderApp(t, project)

		app.enterProjectScopeOutliner(project, tasks[1], OutlinerOpenOptions{})

		if app.outliner != nil {
			t.Fatal("deep-link navigation must not construct AppModel.outliner")
		}
		if cross.scopedProjectID != project.ID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, project.ID)
		}
		if cross.cursor < 0 || cross.cursor >= len(cross.rows) {
			t.Fatalf("cursor %d out of range for %d rows", cross.cursor, len(cross.rows))
		}
		row := cross.rows[cross.cursor]
		if row.task == nil || row.task.ID != tasks[1].ID {
			t.Fatalf("cursor did not land on the linked task; got row %+v", row)
		}
	})
}

// scopeIdentityOutliner builds a runtime-backed cross-project Outliner in the
// All perspective over three projects: two task-carrying projects plus one empty
// project so the empty-state render path can be exercised under a scope.
func scopeIdentityOutliner(t *testing.T) *OutlinerModel {
	t.Helper()
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(120, 24)

	projA := scopeTestProject("A", "Alpha", "1", "2")
	projB := scopeTestProject("B", "Bravo", "1")
	empty := &models.Project{ID: "E", Name: "Empty"}
	empty.BuildTaskTree()

	m.loadedProjects = []*models.Project{projA, projB, empty}
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	return m
}

// TestProjectScopeRendersIdentity locks FR14-003: an active project scope must
// identify the scoped project in the title, status bar, and a stable scope
// breadcrumb (in both populated and empty states); the title/status must stop
// reporting "All" or the full loaded-project count while scoped; nested scope
// depth and identity must track push/pop/clear/reconcile; and the added scope
// chrome must be budgeted for height and stay overflow-free at narrow and
// standard widths.
//
//nolint:gocyclo // The assertions intentionally cover the full project-scope rendering matrix.
func TestProjectScopeRendersIdentity(t *testing.T) {
	t.Run("scoped title, status, and breadcrumb identify the project (populated)", func(t *testing.T) {
		m := scopeIdentityOutliner(t)

		// Unscoped baseline reports the loaded-project count and the All context.
		if title := ansi.Strip(m.renderOutlinerTitle(120)); !strings.Contains(title, "(3 projects)") {
			t.Fatalf("unscoped title should report the project count, got %q", title)
		}
		if label := m.crossStatusBarLabel(); !strings.Contains(label, "· All ·") {
			t.Fatalf("unscoped status bar should report the All context, got %q", label)
		}

		m.pushProjectScope("A")
		if len(m.rows) == 0 {
			t.Fatal("expected the scoped project to render task rows")
		}

		title := ansi.Strip(m.renderOutlinerTitle(120))
		if !strings.Contains(title, "Alpha") {
			t.Fatalf("scoped title should name the scoped project, got %q", title)
		}
		if strings.Contains(title, "project)") || strings.Contains(title, "projects)") {
			t.Fatalf("scoped title must not report the loaded-project count, got %q", title)
		}

		label := m.crossStatusBarLabel()
		if !strings.Contains(label, "Alpha") {
			t.Fatalf("scoped status bar should name the scoped project, got %q", label)
		}
		if strings.Contains(label, "· All ·") {
			t.Fatalf("scoped status bar must not report the All context, got %q", label)
		}

		crumb := ansi.Strip(m.renderOutlinerBreadcrumb(120))
		if !strings.Contains(crumb, "[Alpha]") {
			t.Fatalf("scoped breadcrumb should render the scoped project anchor, got %q", crumb)
		}
	})

	t.Run("scope breadcrumb renders in the empty state", func(t *testing.T) {
		m := scopeIdentityOutliner(t)
		m.pushProjectScope("E") // the empty project yields zero rows
		if len(m.rows) != 0 {
			t.Fatalf("expected the empty project to render no rows, got %d", len(m.rows))
		}

		tree := ansi.Strip(m.renderOutlinerTreeContent(120))
		if !strings.Contains(tree, "[Empty]") {
			t.Fatalf("empty-state render should still show the scope breadcrumb, got %q", tree)
		}
		if !strings.Contains(tree, "Empty") || strings.Contains(tree, "projects)") {
			t.Fatalf("empty-state title should name the scoped project, not the count, got %q", tree)
		}
		if crumb := ansi.Strip(m.renderOutlinerBreadcrumb(120)); crumb == "" {
			t.Fatal("scope breadcrumb must be non-empty in the empty state")
		}
	})

	t.Run("nested depth and identity track push, pop, clear, and reconcile", func(t *testing.T) {
		m := scopeIdentityOutliner(t)

		m.pushProjectScope("A")
		m.pushProjectScope("B")
		if got := m.scopeChainNames(); !reflect.DeepEqual(got, []string{"Alpha", "Bravo"}) {
			t.Fatalf("nested scope chain = %v, want [Alpha Bravo]", got)
		}
		crumb := ansi.Strip(m.renderOutlinerBreadcrumb(120))
		if !strings.Contains(crumb, "[Alpha"+scopeChainSeparator+"Bravo]") {
			t.Fatalf("nested breadcrumb should show the full scope chain, got %q", crumb)
		}
		if label := m.crossStatusBarLabel(); !strings.Contains(label, "Alpha"+scopeChainSeparator+"Bravo") {
			t.Fatalf("nested status bar should show the full scope chain, got %q", label)
		}

		// Esc pop: depth 2 → depth 1 (Alpha).
		if !m.popProjectScope() {
			t.Fatal("expected a pop from the nested scope")
		}
		if got := m.scopeChainNames(); !reflect.DeepEqual(got, []string{"Alpha"}) {
			t.Fatalf("after pop scope chain = %v, want [Alpha]", got)
		}
		if crumb := ansi.Strip(m.renderOutlinerBreadcrumb(120)); !strings.Contains(crumb, "[Alpha]") {
			t.Fatalf("after pop breadcrumb should show [Alpha], got %q", crumb)
		}

		// All Projects clear: back to base, no scope chrome.
		if !m.clearProjectScope() {
			t.Fatal("expected clear to report a change")
		}
		if got := m.scopeChainNames(); len(got) != 0 {
			t.Fatalf("after clear scope chain = %v, want empty", got)
		}
		if crumb := m.renderOutlinerBreadcrumb(120); crumb != "" {
			t.Fatalf("after clear breadcrumb should be empty, got %q", crumb)
		}
		if title := ansi.Strip(m.renderOutlinerTitle(120)); !strings.Contains(title, "(3 projects)") {
			t.Fatalf("after clear the title should report the count again, got %q", title)
		}

		// External-scope reconciliation: scope a project that then vanishes.
		m.pushProjectScope("A")
		m.loadedProjects = []*models.Project{m.loadedProjects[1], m.loadedProjects[2]} // drop A
		if _, repaired := m.reconcileProjectScope(); !repaired {
			t.Fatal("expected reconcile to repair the vanished scope")
		}
		if got := m.scopeChainNames(); len(got) != 0 {
			t.Fatalf("after reconcile scope chain = %v, want empty", got)
		}
		if crumb := m.renderOutlinerBreadcrumb(120); crumb != "" {
			t.Fatalf("after reconcile breadcrumb should be empty, got %q", crumb)
		}
	})

	t.Run("scope chrome is budgeted and overflow-free at narrow and standard widths", func(t *testing.T) {
		m := scopeIdentityOutliner(t)
		unscoped := m.contentHeight()

		m.pushProjectScope("A")
		scoped := m.contentHeight()
		if scoped != unscoped-1 {
			t.Fatalf("scope chrome must cost exactly one budgeted line: unscoped=%d scoped=%d", unscoped, scoped)
		}

		// Zoom within the scope: the combined breadcrumb still occupies one line,
		// so the height budget must not double-count.
		m.crossZoomStack = []CrossProjectZoomEntry{{TaskID: "A-1", ProjectID: "A", CursorIndex: 0}}
		if zoomed := m.contentHeight(); zoomed != scoped {
			t.Fatalf("scope+zoom breadcrumb must stay one line: scoped=%d zoomed=%d", scoped, zoomed)
		}
		m.crossZoomStack = nil

		// Overflow-free: a deeply nested scope with long names must never render a
		// breadcrumb wider than the available width, at narrow or standard sizes.
		long := &models.Project{ID: "L", Name: "A Very Long Project Name For Overflow Testing"}
		long.BuildTaskTree()
		m.loadedProjects = append(m.loadedProjects, long)
		m.pushProjectScope("B")
		m.pushProjectScope("L")
		for _, w := range []int{16, 24, 40, 80, 120} {
			crumb := m.renderOutlinerBreadcrumb(w)
			if got := lipgloss.Width(crumb); got > w {
				t.Fatalf("breadcrumb overflowed width %d: rendered width %d (%q)", w, got, ansi.Strip(crumb))
			}
		}
	})
}

func TestScopedWorkModeProjectsHonorsScope(t *testing.T) {
	projA := scopeTestProject("A", "Alpha", "1")
	projB := scopeTestProject("B", "Bravo", "1")
	m := scopeTestOutliner(projA, projB)

	if got := len(m.workModeScopeProjects()); got != 2 {
		t.Fatalf("expected 2 work-mode scope projects unscoped, got %d", got)
	}
	m.pushProjectScope("A")
	scoped := m.workModeScopeProjects()
	if len(scoped) != 1 || scoped[0].ID != "A" {
		t.Fatalf("expected work-mode scope to honor project scope, got %v", scoped)
	}
}
