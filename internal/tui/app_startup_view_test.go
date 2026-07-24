package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestResolveStartupNavigationAlwaysGlobalToday(t *testing.T) {
	// The Outliner is the sole root and always launches globally in Today,
	// regardless of any persisted state. The root route itself is installed by
	// NewAppModel, so the plan carries only the reload + mode message.
	plan, ok := resolveStartupNavigation(config.NewMachineState(), "project-1")
	if !ok {
		t.Fatal("startup should always resolve")
	}
	if plan.HasRoute {
		t.Fatalf("startup should not synthesize a route; NewAppModel installs the root")
	}
	msg, msgOK := plan.Message.(ShowGlobalOutlinerMsg)
	if !msgOK || !msg.Options.SetMode || msg.Options.Mode != ModeAll {
		t.Fatalf("startup message = %#v, want global All Outliner", plan.Message)
	}
}

func TestNewAppModelStartupOpensGlobalAll(t *testing.T) {
	project := &models.Project{ID: "project-1", Slug: "project-1", Name: "Project 1", Path: t.TempDir()}

	t.Run("valid saved project still opens global All", func(t *testing.T) {
		runtime := newTestConfig()
		setTestProjects(runtime, []*models.Project{project})
		runtime.Machine().CurrentProject = project.ID
		app, err := NewAppModel(runtime, &config.Manager{}, WithPersister(NewMockPersister()))
		if err != nil {
			t.Fatalf("NewAppModel: %v", err)
		}
		// The machine current-project is retained for CLI/contextual defaults.
		if app.currentProject == nil || app.currentProject.ID != project.ID {
			t.Fatalf("restored project = %+v", app.currentProject)
		}
		// The Outliner root is installed at construction.
		if app.navigation.Primary.Current().ID() != TaskWorkspace {
			t.Fatalf("startup root = %s, want TaskWorkspace", app.navigation.Primary.Current().ID())
		}
		msg, ok := app.startupMessage.(ShowGlobalOutlinerMsg)
		if !ok || !msg.Options.SetMode || msg.Options.Mode != ModeAll {
			t.Fatalf("startup message = %#v, want global All Outliner", app.startupMessage)
		}
		_, _ = app.Update(app.startupMessage)
		params := app.navigation.Primary.Current().Params().(TaskWorkspaceParams)
		if params.Scope.Kind != ScopeApp {
			t.Fatalf("startup route scope = %+v, want app (global)", params.Scope)
		}
	})

	t.Run("missing saved project", func(t *testing.T) {
		runtime := newTestConfig()
		runtime.Machine().CurrentProject = "missing"
		app, err := NewAppModel(runtime, &config.Manager{}, WithPersister(NewMockPersister()))
		if err != nil {
			t.Fatalf("NewAppModel: %v", err)
		}
		if app.currentProject != nil {
			t.Fatalf("missing current project restored as %+v", app.currentProject)
		}
		if _, ok := app.startupMessage.(ShowGlobalOutlinerMsg); !ok {
			t.Fatalf("startup message = %T, want ShowGlobalOutlinerMsg", app.startupMessage)
		}
	})
}
