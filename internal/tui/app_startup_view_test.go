package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestResolveStartupNavigationDefaultsToPlanner(t *testing.T) {
	plan, ok := resolveStartupNavigation(config.NewMachineState(), "")
	if !ok {
		t.Fatal("default startup should navigate to Planner")
	}
	if plan.Route.ID() != Planner {
		t.Fatalf("default route = %s, want Planner", plan.Route.ID())
	}
	params := plan.Route.Params().(PlannerParams)
	if params.Tab != PlannerToday {
		t.Fatalf("default Planner tab = %v, want Today", params.Tab)
	}
	msg, ok := plan.Message.(ShowFocusBoardMsg)
	if !ok || !msg.SetTab || msg.Tab != boardTabToday {
		t.Fatalf("default startup message = %#v, want Today planner", plan.Message)
	}
}

func TestResolveStartupNavigationCanonicalDestinations(t *testing.T) {
	tests := []struct {
		startView string
		projectID string
		wantRoute RouteID
		wantView  PresentationID
		want      bool
	}{
		{config.StartViewHome, "", Home, ViewDashboard, false},
		{config.StartViewPlanner, "", Planner, ViewFocusBoard, true},
		{config.StartViewTasks, "", TaskWorkspace, ViewCrossProjectOutliner, true},
		{config.StartViewProject, "project-1", TaskWorkspace, ViewOutliner, true},
		{config.StartViewReview, "", ReviewHub, ViewDailyReview, true},
		{config.StartViewManage, "", ManageHub, ViewSettings, true},
	}
	for _, tc := range tests {
		t.Run(tc.startView, func(t *testing.T) {
			machine := config.NewMachineState()
			machine.SetStartView(tc.startView)
			plan, ok := resolveStartupNavigation(machine, tc.projectID)
			if ok != tc.want {
				t.Fatalf("resolve ok = %v, want %v", ok, tc.want)
			}
			if !ok {
				return
			}
			if plan.Route.ID() != tc.wantRoute || plan.Presentation != tc.wantView {
				t.Fatalf("plan route/view = %s/%s, want %s/%s", plan.Route.ID(), plan.Presentation, tc.wantRoute, tc.wantView)
			}
		})
	}
}

func TestResolveStartupNavigationPreservesLegacyRouteMeaning(t *testing.T) {
	tests := []struct {
		legacy string
		check  func(*testing.T, startupNavigationPlan)
	}{
		{
			legacy: "backlog",
			check: func(t *testing.T, plan startupNavigationPlan) {
				if got := plan.Route.Params().(PlannerParams).Tab; got != PlannerBacklog {
					t.Fatalf("Planner tab = %v, want Backlog", got)
				}
				msg := plan.Message.(ShowFocusBoardMsg)
				if !msg.SetTab || msg.Tab != boardTabBacklog {
					t.Fatalf("Backlog message = %+v", msg)
				}
			},
		},
		{
			legacy: "priority",
			check: func(t *testing.T, plan startupNavigationPlan) {
				params := plan.Route.Params().(TaskWorkspaceParams)
				if params.Focus != WorkspaceActive || params.Layout != WorkspaceList {
					t.Fatalf("priority route params = %+v", params)
				}
				msg := plan.Message.(ShowAllTasksMsg)
				if !msg.SetSort || msg.Sort != SortByPriority {
					t.Fatalf("priority message = %+v", msg)
				}
			},
		},
		{
			legacy: "due-dates",
			check: func(t *testing.T, plan startupNavigationPlan) {
				msg := plan.Message.(ShowAllTasksMsg)
				if !msg.SetGroup || msg.Group != ATGroupByDueDate {
					t.Fatalf("due-date message = %+v", msg)
				}
			},
		},
		{
			legacy: "daily-review",
			check: func(t *testing.T, plan startupNavigationPlan) {
				if got := plan.Route.Params().(ReviewParams).Module; got != ReviewDaily {
					t.Fatalf("review module = %v, want Daily", got)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.legacy, func(t *testing.T) {
			machine := &config.MachineState{StartView: tc.legacy}
			if _, err := machine.MigrateTUIState(); err != nil {
				t.Fatalf("MigrateTUIState: %v", err)
			}
			plan, ok := resolveStartupNavigation(machine, "")
			if !ok {
				t.Fatal("migrated startup did not navigate")
			}
			tc.check(t, plan)
		})
	}
}

func TestResolveStartupProjectRequiresValidProject(t *testing.T) {
	machine := config.NewMachineState()
	machine.SetStartView(config.StartViewProject)

	missing, ok := resolveStartupNavigation(machine, "")
	if !ok {
		t.Fatal("missing project should open picker")
	}
	if missing.HasRoute {
		t.Fatal("missing project synthesized an invalid project route")
	}
	if _, messageOK := missing.Message.(ShowProjectPickerMsg); !messageOK {
		t.Fatalf("missing-project message = %T, want ShowProjectPickerMsg", missing.Message)
	}

	valid, ok := resolveStartupNavigation(machine, "project-1")
	if !ok || !valid.HasRoute {
		t.Fatal("valid project did not resolve a route")
	}
	params := valid.Route.Params().(TaskWorkspaceParams)
	if params.Scope.Kind != ScopeProject || params.Scope.ProjectID != "project-1" {
		t.Fatalf("project startup scope = %+v", params.Scope)
	}
}

func TestNewAppModelProjectStartupRestoresValidProjectOrOpensPicker(t *testing.T) {
	project := &models.Project{ID: "project-1", Slug: "project-1", Name: "Project 1", Path: t.TempDir()}

	t.Run("valid project", func(t *testing.T) {
		runtime := newTestConfig()
		setTestProjects(runtime, []*models.Project{project})
		runtime.Machine().CurrentProject = project.ID
		runtime.Machine().SetStartView(config.StartViewProject)
		app, err := NewAppModel(runtime, &config.Manager{}, WithPersister(NewMockPersister()))
		if err != nil {
			t.Fatalf("NewAppModel: %v", err)
		}
		if app.currentProject == nil || app.currentProject.ID != project.ID {
			t.Fatalf("restored project = %+v", app.currentProject)
		}
		if _, ok := app.startupMessage.(NavigateToViewMsg); !ok {
			t.Fatalf("startup message = %T, want project navigation", app.startupMessage)
		}
		_, _ = app.Update(app.startupMessage)
		params := app.navigation.Primary.Current().Params().(TaskWorkspaceParams)
		if params.Scope.ProjectID != project.ID {
			t.Fatalf("startup route project = %q, want %q", params.Scope.ProjectID, project.ID)
		}
	})

	t.Run("missing project", func(t *testing.T) {
		runtime := newTestConfig()
		runtime.Machine().CurrentProject = "missing"
		runtime.Machine().SetStartView(config.StartViewProject)
		app, err := NewAppModel(runtime, &config.Manager{}, WithPersister(NewMockPersister()))
		if err != nil {
			t.Fatalf("NewAppModel: %v", err)
		}
		if app.currentProject != nil {
			t.Fatalf("missing current project restored as %+v", app.currentProject)
		}
		if _, ok := app.startupMessage.(ShowProjectPickerMsg); !ok {
			t.Fatalf("startup message = %T, want ShowProjectPickerMsg", app.startupMessage)
		}
	})
}

func TestSettingsStartViewChoicesMatchConfigDefault(t *testing.T) {
	want := []string{
		config.StartViewHome,
		config.StartViewPlanner,
		config.StartViewTasks,
		config.StartViewProject,
		config.StartViewReview,
		config.StartViewManage,
	}
	if len(startViewValues) != len(want) {
		t.Fatalf("start-view choices = %v, want %v", startViewValues, want)
	}
	for i := range want {
		if startViewValues[i] != want[i] {
			t.Fatalf("startViewValues[%d] = %q, want %q", i, startViewValues[i], want[i])
		}
	}
	if got := startViewValues[defaultStartViewCursor]; got != config.DefaultStartView {
		t.Fatalf("default cursor points at %q, want %q", got, config.DefaultStartView)
	}
}
