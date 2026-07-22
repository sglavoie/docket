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

// TestCurrentRouteBinding checks that CurrentRoute binds project/task exactly
// like the navigation-history recording rules.
func TestCurrentRouteBinding(t *testing.T) {
	app := newShellTestApp()
	proj := &models.Project{ID: "p1", Name: "Proj"}
	app.currentProject = proj
	app.lastNavigatedTaskID = "t1"

	// Project-scoped view carries the project, no task.
	app.pushView(ViewOutliner)
	r := app.CurrentRoute()
	workspace, ok := r.Params().(TaskWorkspaceParams)
	if r.ID() != TaskWorkspace || !ok || workspace.Scope.ProjectID != "p1" {
		t.Errorf("Outliner route = %s %+v, want project p1", r.ID(), r.Params())
	}

	// Task-scoped view carries both.
	app.pushView(ViewTaskDetail)
	r = app.CurrentRoute()
	detail, ok := r.Params().(TaskDetailParams)
	if r.ID() != TaskDetail || !ok || detail.ProjectID != "p1" || detail.TaskID != "t1" {
		t.Errorf("TaskDetail route params = %+v, want p1/t1", r.Params())
	}

	// Global-nav view drops the project.
	app.jumpToDashboard()
	app.pushView(ViewCrossProjectOutliner)
	r = app.CurrentRoute()
	workspace, ok = r.Params().(TaskWorkspaceParams)
	if !ok || workspace.Scope != AppEntityScope() {
		t.Errorf("cross-project Outliner route scope = %+v, want app", workspace.Scope)
	}

	// Legacy overlays do not replace the typed primary route.
	app.pushView(ViewCommandPalette)
	if got := app.CurrentRoute().ID(); got != TaskWorkspace {
		t.Errorf("CurrentRoute under palette = %s, want TaskWorkspace", got)
	}
}

// TestShellStateFacade exercises the shell-state facade.
func TestShellStateFacade(t *testing.T) {
	app := newShellTestApp()
	app.currentProject = &models.Project{ID: "p1", Name: "Proj"}
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
	if s.CurrentRoute().ID() != TaskWorkspace {
		t.Errorf("CurrentRoute = %s, want TaskWorkspace", s.CurrentRoute().ID())
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
