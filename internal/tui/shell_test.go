package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func newShellTestApp() *AppModel {
	cfg := newTestConfig()
	app, err := NewAppModel(testRuntimeAt("", cfg), &config.Manager{}, WithPersister(NewMockPersister()))
	if err != nil {
		panic(err)
	}
	return app
}

// TestNavEntryRouteRoundTrip proves that route instances preserve navigation
// history semantics: recording a route as a NavEntry and reconstructing it
// yields the same route for every ViewType and parameter combination.
func TestNavEntryRouteRoundTrip(t *testing.T) {
	for _, vt := range allViewTypes() {
		params := RouteParams{ProjectID: "proj-1", TaskID: "task-9"}
		r := NewRoute(vt, params)

		entry := r.NavEntry()
		if entry.ViewType != vt {
			t.Errorf("%s: NavEntry ViewType = %s", vt, entry.ViewType)
		}

		// Global-nav routes must drop the ProjectID; only task-scoped routes
		// carry a TaskID. This mirrors pushView's recording rules.
		wantProject := "proj-1"
		if isGlobalNavView(vt) {
			wantProject = ""
		}
		if entry.ProjectID != wantProject {
			t.Errorf("%s: ProjectID = %q, want %q", vt, entry.ProjectID, wantProject)
		}
		wantTask := ""
		if isTaskScopedView(vt) {
			wantTask = "task-9"
		}
		if entry.TaskID != wantTask {
			t.Errorf("%s: TaskID = %q, want %q", vt, entry.TaskID, wantTask)
		}

		back := routeFromNavEntry(entry)
		if back.ID() != r.ID() || back.Params != r.Params {
			t.Errorf("%s: round-trip mismatch: %+v -> %+v", vt, r, back)
		}
	}
}

// TestNavHistoryPreservedAcrossPushPop exercises the real push/pop path and
// confirms navigation history is recorded and restorable — the "preserve
// navigation history" outcome of this session.
func TestNavHistoryPreservedAcrossPushPop(t *testing.T) {
	app := newShellTestApp()

	// Dashboard is recorded at construction.
	if got, _ := app.navHistory.Current(); got.ViewType != ViewDashboard {
		t.Fatalf("initial history current = %s, want Dashboard", got.ViewType)
	}

	app.pushView(ViewAllTasks) // tracked, global-nav
	app.pushView(ViewSettings) // tracked, global-nav

	cur, ok := app.navHistory.Current()
	if !ok || cur.ViewType != ViewSettings {
		t.Fatalf("after pushes, current = %s (ok=%v), want Settings", cur.ViewType, ok)
	}
	if !app.navHistory.CanGoBack() {
		t.Fatal("expected to be able to go back after pushes")
	}

	back, _ := app.navHistory.GoBack()
	if back.ViewType != ViewAllTasks {
		t.Errorf("GoBack = %s, want AllTasks", back.ViewType)
	}
	fwd, _ := app.navHistory.GoForward()
	if fwd.ViewType != ViewSettings {
		t.Errorf("GoForward = %s, want Settings", fwd.ViewType)
	}
}

// TestUntrackedViewNotRecorded confirms overlays/pickers stay out of history.
func TestUntrackedViewNotRecorded(t *testing.T) {
	app := newShellTestApp()
	app.pushView(ViewCommandPalette) // untracked overlay
	cur, _ := app.navHistory.Current()
	if cur.ViewType != ViewDashboard {
		t.Errorf("current = %s, want Dashboard (palette must not be recorded)", cur.ViewType)
	}
}

// TestCurrentRouteBinding checks that CurrentRoute binds project/task exactly
// like the navigation-history recording rules.
func TestCurrentRouteBinding(t *testing.T) {
	app := newShellTestApp()
	proj := &models.Project{ID: "p1", Name: "Proj"}
	app.currentProject = proj
	app.lastNavigatedTaskID = "t1"

	// Project-scoped view carries the project, no task.
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree}
	r := app.CurrentRoute()
	if r.ID() != ViewProjectTree || r.Params.ProjectID != "p1" || r.Params.TaskID != "" {
		t.Errorf("ProjectTree route = %+v, want project p1 / no task", r.Params)
	}

	// Task-scoped view carries both.
	app.viewStack = []ViewType{ViewDashboard, ViewTaskDetail}
	r = app.CurrentRoute()
	if r.Params.ProjectID != "p1" || r.Params.TaskID != "t1" {
		t.Errorf("TaskDetail route params = %+v, want p1/t1", r.Params)
	}

	// Global-nav view drops the project.
	app.viewStack = []ViewType{ViewDashboard, ViewAllTasks}
	r = app.CurrentRoute()
	if r.Params.ProjectID != "" {
		t.Errorf("AllTasks route kept ProjectID %q, want empty", r.Params.ProjectID)
	}
}

// TestShellStateFacade exercises the shell-state facade.
func TestShellStateFacade(t *testing.T) {
	app := newShellTestApp()
	s := app.Shell()
	if !s.AtRoot() {
		t.Error("expected AtRoot at construction")
	}
	if s.CurrentView() != ViewDashboard {
		t.Errorf("CurrentView = %s, want Dashboard", s.CurrentView())
	}
	app.pushView(ViewOutliner)
	if s.AtRoot() {
		t.Error("expected not AtRoot after push")
	}
	if s.Depth() != 2 {
		t.Errorf("Depth = %d, want 2", s.Depth())
	}
	if s.CurrentRoute().ID() != ViewOutliner {
		t.Errorf("CurrentRoute = %s, want Outliner", s.CurrentRoute().ID())
	}
}

// TestWorkspaceSessionFacade exercises the workspace-session facade.
func TestWorkspaceSessionFacade(t *testing.T) {
	app := newShellTestApp()
	w := app.Workspace()
	if w.HasCurrentProject() {
		t.Error("expected no current project at construction")
	}
	proj := &models.Project{ID: "p1", Name: "Proj"}
	app.currentProject = proj
	if !w.HasCurrentProject() || w.CurrentProject().ID != "p1" {
		t.Error("expected current project p1")
	}
	ctx := w.ViewContext()
	if ctx.ConfigManager != app.configManager || ctx.Persister != app.persister {
		t.Error("ViewContext did not carry workspace dependencies")
	}
}

// TestOverlayManagerLifecycle checks the centralized overlay lifecycle.
func TestOverlayManagerLifecycle(t *testing.T) {
	app := newShellTestApp()
	o := app.Overlays()
	if o.Active() || o.HasActiveTextInput() {
		t.Fatal("expected no overlays at construction")
	}

	// A text-input overlay drives both Active and HasActiveTextInput.
	app.globalProjectPicker = &GlobalProjectPicker{}
	if !o.Active() || !o.HasActiveTextInput() {
		t.Error("globalProjectPicker should make overlays active and text-input")
	}
	if !app.hasActiveTextInput() {
		t.Error("AppModel.hasActiveTextInput should reflect the overlay")
	}

	// A non-text overlay is active but not text-input.
	o.ClearAll()
	app.priorityQuickPicker = &PriorityQuickPicker{}
	if !o.Active() {
		t.Error("priorityQuickPicker should make overlays active")
	}
	if o.HasActiveTextInput() {
		t.Error("priorityQuickPicker is not a text-input overlay")
	}

	o.ClearAll()
	if o.Active() || app.globalProjectPicker != nil || app.priorityQuickPicker != nil {
		t.Error("ClearAll should dismiss every overlay")
	}
}
