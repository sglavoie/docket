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

	// The Outliner presentation is the app-scoped TaskWorkspace root; project
	// selection is an Outliner scope frame, not a project-scoped root route.
	app.pushView(ViewCrossProjectOutliner)
	r := app.CurrentRoute()
	workspace, ok := r.Params().(TaskWorkspaceParams)
	if r.ID() != TaskWorkspace || !ok || workspace.Scope != AppEntityScope() {
		t.Errorf("Outliner route = %s %+v, want app-scoped root", r.ID(), r.Params())
	}

	// Task-scoped view carries both.
	task := &models.Task{ID: "t1", Title: "Task"}
	proj.Tasks = []*models.Task{task}
	proj.BuildTaskTree()
	_ = app.activateTaskDetail(task, proj, TaskOverview)
	r = app.CurrentRoute()
	detail, ok := r.Params().(TaskDetailParams)
	if r.ID() != TaskDetail || !ok || detail.ProjectID != "p1" || detail.TaskID != "t1" {
		t.Errorf("TaskDetail route params = %+v, want p1/t1", r.Params())
	}

	// Global-nav view drops the project.
	app.jumpToOutlinerRoot()
	app.pushView(ViewCrossProjectOutliner)
	r = app.CurrentRoute()
	workspace, ok = r.Params().(TaskWorkspaceParams)
	if !ok || workspace.Scope != AppEntityScope() {
		t.Errorf("cross-project Outliner route scope = %+v, want app", workspace.Scope)
	}

	// Overlays do not replace the typed primary route.
	overlay, err := NewOverlay(OverlaySearch, "test:search")
	if err != nil {
		t.Fatalf("NewOverlay: %v", err)
	}
	app.navigation.Overlays.Push(overlay, FocusTarget{}, nil)
	if got := app.CurrentRoute().ID(); got != TaskWorkspace {
		t.Errorf("CurrentRoute under search = %s, want TaskWorkspace", got)
	}
}

// TestShellStateFacade exercises the shell-state facade.
func TestShellStateFacade(t *testing.T) {
	app := newShellTestApp()
	app.currentProject = &models.Project{ID: "p1", Name: "Proj"}
	s := app.Shell()
	// Construction lands on the sole Outliner root, with no synthetic
	// presentation sentinel beneath it.
	if s.CurrentView() != ViewCrossProjectOutliner {
		t.Errorf("CurrentView = %s, want CrossProjectOutliner", s.CurrentView())
	}
	if s.CurrentRoute().ID() != TaskWorkspace {
		t.Errorf("root route = %s, want TaskWorkspace", s.CurrentRoute().ID())
	}
	if s.Depth() != 1 || len(s.Stack()) != 1 || s.Stack()[0] != ViewCrossProjectOutliner {
		t.Fatalf("root renderer path = %v depth=%d, want sole Outliner renderer", s.Stack(), s.Depth())
	}
	// The Outliner presentation replaces the single root in place and stays
	// app-scoped; project selection is an Outliner scope frame, not a root route.
	app.pushView(ViewCrossProjectOutliner)
	if s.CurrentRoute().ID() != TaskWorkspace {
		t.Errorf("CurrentRoute = %s, want TaskWorkspace", s.CurrentRoute().ID())
	}
	sp, ok := s.CurrentRoute().Params().(TaskWorkspaceParams)
	if !ok || sp.Scope != AppEntityScope() {
		t.Errorf("root scope = %+v, want app-scoped root", s.CurrentRoute().Params())
	}
	// A child (Task Detail) stacks above the root.
	task := &models.Task{ID: "t1", Title: "Task"}
	app.currentProject.Tasks = []*models.Task{task}
	app.currentProject.BuildTaskTree()
	_ = app.activateTaskDetail(task, app.currentProject, TaskOverview)
	if s.AtRoot() {
		t.Error("expected not AtRoot after pushing a child")
	}
	if s.CurrentRoute().ID() != TaskDetail {
		t.Errorf("child route = %s, want TaskDetail", s.CurrentRoute().ID())
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
	app.operationTextPrompt = &OperationTextPromptModel{}
	if !o.Active() || !o.HasActiveTextInput() {
		t.Error("text prompt should make overlays active and text-input")
	}
	o.ClearAll()
	if o.Active() || app.operationTextPrompt != nil {
		t.Error("ClearAll should dismiss every overlay")
	}
}
