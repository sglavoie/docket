package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The base task workspace admits only non-archived projects that do not carry
// the reserved parked tag. These tests walk every route a parked project's tasks
// could otherwise take back into the Outliner — a mode, the Pinned section, a
// planning reference, a context label, slash search, a filter facet, an
// operation scope, the workspace pulse, or a surviving selection — and assert
// each one is closed.

// parkProject tags cataloged projects parked through the catalog's own upsert,
// so a fixture can only reach states the product can produce.
func parkProject(t *testing.T, runtime *config.RuntimeState, projectIDs ...string) {
	t.Helper()
	catalog := runtime.Catalog()
	for _, projectID := range projectIDs {
		project := runtime.GetProject(projectID)
		if project == nil {
			t.Fatalf("park %q: project is not cataloged", projectID)
		}
		project.Tags = []string{models.ParkedProjectTag}
		if err := catalog.UpsertProject(project); err != nil {
			t.Fatalf("park %q: %v", projectID, err)
		}
	}
}

// parkedWorkspace builds one base-workspace and one parked project holding tasks
// with matching shapes, plus planning references into both, so every projection
// has something it would show if the parked tag were not enforced.
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
	parked.Tags = []string{models.ParkedProjectTag}

	runtime = testConfigWithProjects(active, parked)
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

	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeInbox, chrome.ModeToday, chrome.ModeNext, chrome.ModeBacklog, chrome.ModeDue} {
		t.Run(string(mode), func(t *testing.T) {
			outliner.selectOutlinerMode(mode)
			projectIDs := taskRowProjectIDs(outliner)
			if containsProject(projectIDs, parked.ID) {
				t.Fatalf("%s projects the parked project: %v", mode, projectIDs)
			}
			if !containsProject(projectIDs, active.ID) {
				t.Fatalf("%s projects nothing from the base workspace: %v", mode, projectIDs)
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
	parkProject(t, runtime, parked.ID)

	outliner := baseWorkspaceOutliner(runtime, active, parked)
	outliner.selectOutlinerMode(chrome.ModeAll)
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
		t.Fatalf("collectPinnedTasks = %+v, want only the base workspace pin", pinned)
	}
}

func assertIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("project IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("project IDs = %v, want %v", got, want)
		}
	}
}

func TestParkedProjectLeavesSearchProjection(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)
	outliner := baseWorkspaceOutliner(runtime, active, parked)
	outliner.selectOutlinerMode(chrome.ModeAll)

	outliner.SetSearchQuery("shared")
	if !outliner.hasSearchProjection() {
		t.Fatal("search did not replace the ordinary projection")
	}
	projectIDs := taskRowProjectIDs(outliner)
	if containsProject(projectIDs, parked.ID) {
		t.Fatalf("search projects the parked project: %v", projectIDs)
	}
	if !containsProject(projectIDs, active.ID) {
		t.Fatalf("search matched nothing in the base workspace: %v", projectIDs)
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

	picker, err := shell.newFilterTagsPicker(operation.OperationExecutionRequest{})
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

func TestParkedProjectLeavesOperationScopeAndWorkspacePulse(t *testing.T) {
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

	pulse := computeWorkspacePulse(runtime, map[taskref.TaskIdentity]bool{
		taskref.For(parked.Tasks[0], parked): true,
	}, time.Now())
	if pulse.Inbox != 1 {
		t.Fatalf("workspace pulse inbox = %d, want only the base workspace inbox task", pulse.Inbox)
	}
	if pulse.Today != 0 {
		t.Fatalf("workspace pulse Today = %d, want a parked Today reference to count nothing", pulse.Today)
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
		nextOrderIDs: make(map[taskref.TaskIdentity]bool),
		today:        []PlanningTask{},
		todayIDs:     make(map[taskref.TaskIdentity]bool),
	}

	workspaceProjects := shell.taskWorkspaceProjects()
	loads := make(map[string]error)
	shell.hydrateNextOrder(workspaceProjects, loads, false)
	shell.hydrateToday(workspaceProjects, loads, false)

	if len(shell.nextOrder) != 1 || shell.nextOrder[0].Project.ID != active.ID {
		t.Fatalf("Next hydration = %+v, want only the base workspace member", shell.nextOrder)
	}
	if len(shell.today) != 1 || shell.today[0].Project.ID != active.ID {
		t.Fatalf("Today hydration = %+v, want only the base workspace member", shell.today)
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

	activeIdentity := taskref.For(active.Tasks[0], active)
	parkedIdentity := taskref.For(parked.Tasks[0], parked)
	outliner.ReplaceSelection([]taskref.TaskIdentity{activeIdentity, parkedIdentity})

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
// invariant the parked prune must not widen into.
func TestReloadKeepsSelectionsHiddenByFilters(t *testing.T) {
	runtime, active, parked := parkedWorkspace(t)
	outliner := baseWorkspaceOutliner(runtime, active, parked)
	outliner.selectOutlinerMode(chrome.ModeAll)

	hidden := taskref.For(active.Tasks[0], active)
	outliner.ReplaceSelection([]taskref.TaskIdentity{hidden})
	outliner.selectOutlinerMode(chrome.ModeDue)

	outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   admittedWorkspace(runtime, active, parked),
		generation: outliner.crossLoadGen,
	})

	if !outliner.IsTaskIdentitySelected(hidden) {
		t.Fatal("a selection hidden by the active mode was dropped by the reload")
	}
}
