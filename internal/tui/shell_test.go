package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

func newShellTestApp() *AppModel {
	cfg := newTestConfig()
	app, err := NewAppModel(testRuntimeAt("", cfg), &config.Manager{}, withPersister(newMockPersister()))
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

	// The Outliner presentation is the app-scoped TaskWorkspace root; project
	// selection is an Outliner scope frame, not a project-scoped root route.
	app.pushView(presentation.ViewCrossProjectOutliner)
	r := app.CurrentRoute()
	workspace, ok := r.Params().(route.TaskWorkspaceParams)
	if r.ID() != route.TaskWorkspace || !ok || workspace.Scope != route.AppEntityScope() {
		t.Errorf("Outliner route = %s %+v, want app-scoped root", r.ID(), r.Params())
	}

	// Global-nav view drops the project.
	jumpToOutlinerRootForTest(app)
	app.pushView(presentation.ViewCrossProjectOutliner)
	r = app.CurrentRoute()
	workspace, ok = r.Params().(route.TaskWorkspaceParams)
	if !ok || workspace.Scope != route.AppEntityScope() {
		t.Errorf("cross-project Outliner route scope = %+v, want app", workspace.Scope)
	}

	// Overlays do not replace the typed primary route.
	overlay, err := route.NewOverlay(route.OverlayForm, "test:search")
	if err != nil {
		t.Fatalf("NewOverlay: %v", err)
	}
	app.viewNav.navigation.Overlays.Push(overlay, nil)
	if got := app.CurrentRoute().ID(); got != route.TaskWorkspace {
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
	if s.CurrentView() != presentation.ViewCrossProjectOutliner {
		t.Errorf("CurrentView = %s, want CrossProjectOutliner", s.CurrentView())
	}
	if s.CurrentRoute().ID() != route.TaskWorkspace {
		t.Errorf("root route = %s, want TaskWorkspace", s.CurrentRoute().ID())
	}
	if s.Depth() != 1 || len(s.Stack()) != 1 || s.Stack()[0] != presentation.ViewCrossProjectOutliner {
		t.Fatalf("root renderer path = %v depth=%d, want sole Outliner renderer", s.Stack(), s.Depth())
	}
	// The Outliner presentation replaces the single root in place and stays
	// app-scoped; project selection is an Outliner scope frame, not a root route.
	app.pushView(presentation.ViewCrossProjectOutliner)
	if s.CurrentRoute().ID() != route.TaskWorkspace {
		t.Errorf("CurrentRoute = %s, want TaskWorkspace", s.CurrentRoute().ID())
	}
	sp, ok := s.CurrentRoute().Params().(route.TaskWorkspaceParams)
	if !ok || sp.Scope != route.AppEntityScope() {
		t.Errorf("root scope = %+v, want app-scoped root", s.CurrentRoute().Params())
	}
	// A secondary tool stacks above the root.
	child := manageRouteForPresentation(presentation.ViewProjectHub)
	if err := app.viewNav.navigation.Primary.PushChild(child); err != nil {
		t.Fatal(err)
	}
	if s.AtRoot() {
		t.Error("expected not AtRoot after pushing a child")
	}
	if s.CurrentRoute().ID() != route.SecondaryTool {
		t.Errorf("child route = %s, want SecondaryTool", s.CurrentRoute().ID())
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
	_ = w.ViewContext()
}

// TestOverlayManagerLifecycle checks the centralized overlay lifecycle.
func TestOverlayManagerLifecycle(t *testing.T) {
	app := newShellTestApp()
	o := app.Overlays()
	if o.Active() || o.HasActiveTextInput() {
		t.Fatal("expected no overlays at construction")
	}

	// A text-input overlay drives both Active and HasActiveTextInput.
	app.operations.textPrompt = &OperationTextPromptModel{}
	if !o.Active() || !o.HasActiveTextInput() {
		t.Error("text prompt should make overlays active and text-input")
	}
	o.ClearAll()
	if o.Active() || app.operations.textPrompt != nil {
		t.Error("ClearAll should dismiss every overlay")
	}
}
