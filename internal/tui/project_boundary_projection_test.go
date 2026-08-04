package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// The base task workspace admits only non-archived Active-boundary projects.
// These tests walk every route a parked project's tasks could otherwise take
// back into the Outliner — a mode, the Pinned section, a planning reference, a
// context label, slash search, a filter facet, an operation scope, the open-loop
// counter, or a surviving selection — and assert each one is closed.

// parkProject moves projects out of Active through the catalog's own boundary
// operations, so a fixture can only reach states the product can produce.
func parkProject(t *testing.T, runtime *config.RuntimeState, boundaryName string, projectIDs ...string) {
	t.Helper()
	catalog := runtime.Catalog()
	boundary, err := catalog.CreateBoundary(boundaryName)
	if err != nil {
		t.Fatalf("CreateBoundary(%q): %v", boundaryName, err)
	}
	for _, projectID := range projectIDs {
		changed, assignErr := catalog.AssignProjectBoundary(projectID, boundary.ID)
		if assignErr != nil {
			t.Fatalf("AssignProjectBoundary(%q): %v", projectID, assignErr)
		}
		if !changed {
			t.Fatalf("AssignProjectBoundary(%q) reported no change", projectID)
		}
	}
}

// parkedWorkspace builds one Active and one parked project holding tasks with
// matching shapes, plus planning references into both, so every projection has
// something it would show if the boundary were not enforced.
func parkedWorkspace(t *testing.T) (runtime *config.RuntimeState, active, parked *models.Project) {
	t.Helper()
	due := time.Now()

	activeInbox := primaryRowsTask("active-inbox", models.StateInbox)
	activeToday := primaryRowsTask("active-today", models.StateBacklog)
	activeNext := primaryRowsTask("active-next", models.StateBacklog)
	activeDue := primaryRowsTask("active-shared", models.StateBacklog)
	activeDue.DueDate = &due
	activeDue.Tags = []string{"active-tag"}
	active = primaryRowsProject("active", activeInbox, activeToday, activeNext, activeDue)

	parkedInbox := primaryRowsTask("parked-inbox", models.StateInbox)
	parkedToday := primaryRowsTask("parked-today", models.StateBacklog)
	parkedNext := primaryRowsTask("parked-next", models.StateBacklog)
	parkedDue := primaryRowsTask("parked-shared", models.StateBacklog)
	parkedDue.DueDate = &due
	parkedDue.Tags = []string{"parked-tag"}
	parked = primaryRowsProject("parked", parkedInbox, parkedToday, parkedNext, parkedDue)

	runtime = testConfigWithProjects(active, parked)
	parkProject(t, runtime, "On Hold", parked.ID)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: active.ID, TaskID: activeToday.ID},
		{ProjectID: parked.ID, TaskID: parkedToday.ID},
	})
	runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: active.ID, TaskID: activeNext.ID},
		{ProjectID: parked.ID, TaskID: parkedNext.ID},
	})
	return runtime, active, parked
}

// admittedWorkspace runs the real base selection and keeps the fixtures it
// admits. The runtime yields catalog metadata without tasks, exactly as
// production does before the workspace loader parses each admitted project, so
// a test substitutes its own task-bearing projects for the surviving IDs.
func admittedWorkspace(runtime *config.RuntimeState, fixtures ...*models.Project) []*models.Project {
	admitted := make(map[string]bool)
	for _, project := range selectCrossScopeProjects(runtime, CrossScopeOpts{ActiveOnly: true}) {
		admitted[project.ID] = true
	}
	loaded := make([]*models.Project, 0, len(fixtures))
	for _, project := range fixtures {
		if admitted[project.ID] {
			loaded = append(loaded, project)
		}
	}
	return loaded
}

func baseWorkspaceOutliner(runtime *config.RuntimeState, fixtures ...*models.Project) *OutlinerModel {
	outliner := NewOutlinerModel(nil, nil, runtime)
	outliner.loadedProjects = admittedWorkspace(runtime, fixtures...)
	return outliner
}

// TestParkedProjectLeavesEveryOutlinerMode also covers the Today and Next
// stale-reference defense: both planning collections still name a parked task,
// which the projections must refuse to resolve rather than merely not order.
func TestParkedProjectLeavesEveryOutlinerMode(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)
	outliner := baseWorkspaceOutliner(runtime, active, parked)

	for _, mode := range []OutlinerModeID{ModeAll, ModeInbox, ModeToday, ModeNext, ModeBacklog, ModeDue} {
		t.Run(string(mode), func(t *testing.T) {
			outliner.selectOutlinerMode(mode)
			projectIDs := taskRowProjectIDs(outliner)
			if containsProject(projectIDs, parked.ID) {
				t.Fatalf("%s projects the parked project: %v", mode, projectIDs)
			}
			if !containsProject(projectIDs, active.ID) {
				t.Fatalf("%s projects nothing from the Active workspace: %v", mode, projectIDs)
			}
		})
	}
}

func TestParkedProjectLeavesPinnedProjections(t *testing.T) {
	pinnedAt := time.Now().Add(-time.Hour)
	activePin := primaryRowsTask("active-pinned", models.StateBacklog)
	activePin.Pinned, activePin.PinnedAt = true, &pinnedAt
	parkedPin := primaryRowsTask("parked-pinned", models.StateBacklog)
	parkedPin.Pinned, parkedPin.PinnedAt = true, &pinnedAt

	active := primaryRowsProject("active", activePin)
	parked := primaryRowsProject("parked", parkedPin)
	runtime := testConfigWithProjects(active, parked)
	parkProject(t, runtime, "On Hold", parked.ID)

	outliner := baseWorkspaceOutliner(runtime, active, parked)
	outliner.selectOutlinerMode(ModeAll)
	for _, row := range outliner.rows {
		if row.IsPinnedSection && row.isTaskRow() && row.ProjectID == parked.ID {
			t.Fatalf("Pinned section projects the parked task %q", row.task.ID)
		}
	}

	shell := &AppModel{
		runtime:  runtime,
		projects: map[string]*models.Project{active.ID: active, parked.ID: parked},
	}
	pinned := shell.collectPinnedTasks()
	if len(pinned) != 1 || pinned[0].Task.ID != activePin.ID {
		t.Fatalf("collectPinnedTasks = %+v, want only the Active workspace pin", pinned)
	}
}

func TestParkedProjectLeavesContextScopes(t *testing.T) {
	runtime := newTestConfig()
	setTestProjects(runtime, []*models.Project{
		{ID: "p1", Slug: "alpha", Name: "Alpha", Context: "work"},
		{ID: "p2", Slug: "bravo", Name: "Bravo", Context: "work"},
		{ID: "p3", Slug: "charlie", Name: "Charlie", Context: "solo"},
	})
	parkProject(t, runtime, "On Hold", "p2", "p3")

	assertIDs(t, AllContextScope().ProjectIDs(runtime), "p1")
	assertIDs(t, RegularContextScope("work").ProjectIDs(runtime), "p1")
	assertIDs(t, RegularContextScope("solo").ProjectIDs(runtime))

	for _, summary := range ContextScopeSummaries(runtime) {
		if summary.Scope.Name == "solo" {
			t.Fatal("a context carried only by parked projects is still selectable")
		}
	}
}

// A regular context whose last Active member is parked must fall back to the
// base scope, while the protected personal context keeps its documented
// exemption from being removed by emptiness.
func TestRegularContextDegradesWhenItsLastActiveMemberParks(t *testing.T) {
	runtime := newTestConfig()
	setTestProjects(runtime, []*models.Project{
		{ID: "p1", Slug: "alpha", Name: "Alpha", Context: "work"},
	})
	scope := RegularContextScope("work")
	if got := selectableContextScope(runtime, scope); got != scope {
		t.Fatalf("selectable scope = %+v, want the populated work context", got)
	}

	parkProject(t, runtime, "On Hold", "p1")

	if got := selectableContextScope(runtime, scope); !got.IsAll() {
		t.Fatalf("selectable scope = %+v, want the base scope after the last member parked", got)
	}
	personal := false
	for _, summary := range ContextScopeSummaries(runtime) {
		if summary.Scope.Name == runtime.DefaultContext() {
			personal = true
		}
	}
	if !personal {
		t.Fatal("the protected personal context stopped being listed while empty")
	}
}

func TestParkedProjectLeavesSearchProjection(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)
	outliner := baseWorkspaceOutliner(runtime, active, parked)
	outliner.selectOutlinerMode(ModeAll)

	outliner.SetSearchQuery("shared")
	if !outliner.hasSearchProjection() {
		t.Fatal("search did not replace the ordinary projection")
	}
	projectIDs := taskRowProjectIDs(outliner)
	if containsProject(projectIDs, parked.ID) {
		t.Fatalf("search projects the parked project: %v", projectIDs)
	}
	if !containsProject(projectIDs, active.ID) {
		t.Fatalf("search matched nothing in the Active workspace: %v", projectIDs)
	}
}

// The tag facet falls back to the shell population whenever no cross-project
// snapshot has landed, which is the one filter path that does not read
// loadedProjects.
func TestParkedProjectLeavesTagFilterFacet(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)
	shell := &AppModel{
		runtime:  runtime,
		projects: map[string]*models.Project{active.ID: active, parked.ID: parked},
	}

	picker, err := shell.newFilterTagsPicker(OperationExecutionRequest{})
	if err != nil {
		t.Fatalf("newFilterTagsPicker: %v", err)
	}
	var names []string
	for _, item := range picker.surface.Items() {
		names = append(names, item.PrimaryText)
	}
	for _, name := range names {
		if name == "parked-tag" {
			t.Fatalf("tag facet offers a parked project's tag: %v", names)
		}
	}
	if len(names) == 0 {
		t.Fatal("tag facet offered nothing, so the parked exclusion proves nothing")
	}
}

func TestParkedProjectLeavesOperationScopeAndOpenLoops(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)
	shell := &AppModel{
		runtime:  runtime,
		projects: map[string]*models.Project{active.ID: active, parked.ID: parked},
		views:    NewViewRegistry(),
	}
	assertIDs(t, shell.operationScopeProjectIDs(nil), active.ID)

	repo := testutil.NewMockRepository()
	repo.Projects[active.ID] = active
	repo.Projects[parked.ID] = parked
	previous := storage.DefaultRepository
	storage.DefaultRepository = repo
	defer func() { storage.DefaultRepository = previous }()

	loops := ComputeOpenLoops(runtime, map[TaskIdentity]bool{
		taskIdentityFor(parked.Tasks[0], parked): true,
	})
	if loops.Inbox != 1 {
		t.Fatalf("open-loop inbox = %d, want only the Active workspace inbox task", loops.Inbox)
	}
	if loops.TodayTotal != 0 {
		t.Fatalf("open-loop Today total = %d, want a parked Today reference to count nothing", loops.TodayTotal)
	}
}

// Startup hydration reads the base workspace, so a planning reference a project
// kept when it was parked never re-enters the session's Today/Next membership.
func TestParkedProjectPlanningReferencesDoNotHydrate(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)
	persister := NewMockPersister()
	persister.Projects[active.ID] = active
	persister.Projects[parked.ID] = parked
	shell := &AppModel{
		runtime:      runtime,
		persister:    persister,
		projects:     map[string]*models.Project{active.ID: active, parked.ID: parked},
		nextOrder:    []PlanningTask{},
		nextOrderIDs: make(map[TaskIdentity]bool),
		today:        []PlanningTask{},
		todayIDs:     make(map[TaskIdentity]bool),
	}

	workspaceProjects := shell.taskWorkspaceProjects()
	loads := make(map[string]error)
	shell.hydrateNextOrder(workspaceProjects, loads, false)
	shell.hydrateToday(workspaceProjects, loads, false)

	if len(shell.nextOrder) != 1 || shell.nextOrder[0].Project.ID != active.ID {
		t.Fatalf("Next hydration = %+v, want only the Active workspace member", shell.nextOrder)
	}
	if len(shell.today) != 1 || shell.today[0].Project.ID != active.ID {
		t.Fatalf("Today hydration = %+v, want only the Active workspace member", shell.today)
	}
}

// Parking a project while another scope is in view must drop its tasks from the
// session selection, because no later projection can show them again. A hidden
// selection whose project stays admitted is deliberately preserved.
func TestReloadDropsSelectionsOutsideTheAdmittedWorkspace(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)
	outliner := NewOutlinerModel(nil, nil, runtime)
	outliner.loadedProjects = []*models.Project{active, parked}
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()

	activeIdentity := taskIdentityFor(active.Tasks[0], active)
	parkedIdentity := taskIdentityFor(parked.Tasks[0], parked)
	outliner.ReplaceSelection([]TaskIdentity{activeIdentity, parkedIdentity})

	outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   admittedWorkspace(runtime, active, parked),
		generation: outliner.crossLoadGen,
	})

	if outliner.IsTaskIdentitySelected(parkedIdentity) {
		t.Fatal("a parked project's task stayed selected after the reload dropped it")
	}
	if !outliner.IsTaskIdentitySelected(activeIdentity) {
		t.Fatal("the reload dropped a selection the base workspace still admits")
	}
}

// A filter or mode that merely hides a row keeps its selection, which is the
// invariant the boundary prune must not widen into.
func TestReloadKeepsSelectionsHiddenByFilters(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)
	outliner := baseWorkspaceOutliner(runtime, active, parked)
	outliner.selectOutlinerMode(ModeAll)

	hidden := taskIdentityFor(active.Tasks[0], active)
	outliner.ReplaceSelection([]TaskIdentity{hidden})
	outliner.selectOutlinerMode(ModeDue)

	outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   admittedWorkspace(runtime, active, parked),
		generation: outliner.crossLoadGen,
	})

	if !outliner.IsTaskIdentitySelected(hidden) {
		t.Fatal("a selection hidden by the active mode was dropped by the reload")
	}
}
