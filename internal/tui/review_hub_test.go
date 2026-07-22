package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestReviewHubLandingNavigationEmptyStatesAndNarrowLayouts(t *testing.T) {
	hub := NewReviewHubModel(config.EmptyRuntimeState(t.TempDir()), nil, nil)
	hub.SetSize(60, 18)

	landing := hub.View().Content
	for _, name := range []string{"Daily", "Weekly", "Activity", "Metrics", "Roadmap"} {
		if !strings.Contains(ansi.Strip(landing), name) {
			t.Fatalf("Review landing omitted %q:\n%s", name, ansi.Strip(landing))
		}
	}
	assertBaselineLineWidth(t, "Review landing", landing, 60)
	assertNoFooterContentOverlap(t, "Review landing", landing, 18)

	_, cmd := hub.Update(syntheticKey('3'))
	if cmd == nil {
		t.Fatal("Review landing numeric selection did not emit a module change")
	}
	selected, ok := cmd().(ReviewHubModuleSelectedMsg)
	if !ok || selected.Module != ReviewActivity {
		t.Fatalf("Review landing selection = %#v, want Activity", cmd())
	}

	for _, module := range []ReviewModule{ReviewDaily, ReviewWeekly, ReviewActivity, ReviewMetrics, ReviewRoadmap} {
		t.Run(module.String(), func(t *testing.T) {
			if err := hub.Activate(module, nil); err != nil {
				t.Fatal(err)
			}
			rendered := hub.View().Content
			if strings.TrimSpace(ansi.Strip(rendered)) == "" {
				t.Fatal("empty Review module rendered no empty state")
			}
			assertBaselineLineWidth(t, module.String(), rendered, 60)
			assertNoFooterContentOverlap(t, module.String(), rendered, 18)
		})
	}
}

func TestReviewHubDeepLinksUseOneTypedRouteAndReturnThroughLanding(t *testing.T) {
	tests := []struct {
		name    string
		message any
		module  ReviewModule
	}{
		{name: "daily", message: ShowDailyReviewMsg{}, module: ReviewDaily},
		{name: "weekly", message: ShowWeeklyReviewMsg{}, module: ReviewWeekly},
		{name: "activity", message: ShowRecentActivityMsg{}, module: ReviewActivity},
		{name: "metrics", message: ShowMetricsDashboardMsg{}, module: ReviewMetrics},
		{name: "roadmap", message: ShowRoadmapMsg{}, module: ReviewRoadmap},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := typedTestApp(t)
			handled, _ := app.handleViewMessages(tc.message)
			if !handled {
				t.Fatalf("%T was not handled", tc.message)
			}

			route := app.CurrentRoute()
			params, ok := route.Params().(ReviewParams)
			if route.ID() != ReviewHub || !ok || params.Module != tc.module {
				t.Fatalf("deep link route = %s/%#v, want Review/%s", route.ID(), route.Params(), tc.module)
			}
			if _, legacy := route.Presentation(); legacy {
				t.Fatal("live Review module retained an independent legacy presentation")
			}
			if app.activeView() != app.reviewHub || app.reviewHub.Module() != tc.module {
				t.Fatalf("active Review renderer/module = %T/%s", app.activeView(), app.reviewHub.Module())
			}

			app.handleGlobalKeyPress(syntheticSpecialKey(27))
			landing := app.CurrentRoute().Params().(ReviewParams)
			if landing.Module != ReviewLanding || app.reviewHub.Module() != ReviewLanding {
				t.Fatalf("first Esc = route/hub %s/%s, want Review landing", landing.Module, app.reviewHub.Module())
			}
			app.handleGlobalKeyPress(syntheticSpecialKey(27))
			if app.CurrentRoute().ID() != Home {
				t.Fatalf("second Esc route = %s, want Home", app.CurrentRoute().ID())
			}
		})
	}
}

func TestReviewHubLegacyNamesMapToModulesAndRemainSearchable(t *testing.T) {
	routes := []struct {
		view   PresentationID
		module ReviewModule
	}{
		{ViewDailyReview, ReviewDaily},
		{ViewWeeklyReview, ReviewWeekly},
		{ViewRecentActivity, ReviewActivity},
		{ViewProjectRecentActivity, ReviewActivity},
		{ViewMetricsDashboard, ReviewMetrics},
		{ViewRoadmap, ReviewRoadmap},
	}
	for _, tc := range routes {
		route, ok, err := routeForPresentation(tc.view, "project-1", "")
		if err != nil || !ok {
			t.Fatalf("%s deep-link adapter: ok=%v err=%v", tc.view, ok, err)
		}
		params := route.Params().(ReviewParams)
		if route.ID() != ReviewHub || params.Module != tc.module {
			t.Errorf("%s maps to %s/%s, want Review/%s", tc.view, route.ID(), params.Module, tc.module)
		}
	}

	app := newTestAppModel()
	aliases := []struct {
		command string
		want    []string
	}{
		{"daily review", []string{"DailyReview", "ReviewHub/Daily"}},
		{"weekly review", []string{"WeeklyReview", "ReviewHub/Weekly"}},
		{"latest activity", []string{"RecentActivity", "ProjectRecentActivity", "completed tasks", "ReviewHub/Activity"}},
		{"metrics dashboard", []string{"MetricsDashboard", "ReviewHub/Metrics"}},
		{"roadmap", []string{"review roadmap", "ReviewHub/Roadmap"}},
	}
	commands := app.viewCommands()
	for _, tc := range aliases {
		command := findCommand(commands, tc.command)
		if command == nil {
			t.Fatalf("missing Review command %q", tc.command)
		}
		for _, alias := range tc.want {
			if !slices.Contains(command.Aliases, alias) {
				t.Errorf("%q aliases %v omit retired name %q", tc.command, command.Aliases, alias)
			}
		}
	}
}

func TestReviewTaskModulesShareGrammarAndCompletedBrowsing(t *testing.T) {
	runtime := config.EmptyRuntimeState(t.TempDir())
	daily := NewDailyReviewModel(runtime, nil)
	weekly := NewWeeklyReviewModel(runtime, nil)
	activity := NewRecentActivityModel(runtime, nil)

	for name, keys := range map[string][]string{
		"Daily":    daily.HandledKeys(),
		"Weekly":   weekly.HandledKeys(),
		"Activity": activity.HandledKeys(),
	} {
		for _, shared := range []string{",", "!", "d", "z", "m", "c", "t", "b"} {
			if !slices.Contains(keys, shared) {
				t.Errorf("%s Review grammar omits shared task action %q: %v", name, shared, keys)
			}
		}
	}

	project := &models.Project{ID: "project", Name: "Project"}
	ready := &models.Task{ID: "ready", Title: "Ready", WorkflowState: models.StateReady}
	done := &models.Task{ID: "done", Title: "Done", WorkflowState: models.StateDone}
	activity.tasks = []CrossProjectTask{{Task: ready, Project: project}, {Task: done, Project: project}}
	activity.tabs.Active = recentActivityTabCompleted
	activity.applyFilter()
	if len(activity.filtered) != 1 || activity.filtered[0].Task != done {
		t.Fatalf("Activity/Completed = %+v, want only completed task", activity.filtered)
	}

	app := typedTestApp(t)
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.views.Register(ViewOutliner, app.outliner)
	route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: ProjectEntityScope(project.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if cmd := app.activateTaskWorkspaceRoute(route); cmd != nil {
		t.Fatalf("activate Task Workspace returned a command")
	}
	command := findCommand(app.filterCommands(), "view completed")
	if command == nil {
		t.Fatal("missing view completed command")
	}
	if msg := command.Handler(); msg != nil {
		t.Fatalf("view completed returned %T, want in-place change", msg)
	}
	if app.outliner.smartFocus != nil || app.outliner.activePerspective != PerspectiveCompleted {
		t.Fatalf("view completed focus = %v/%d, want Completed", app.outliner.smartFocus, app.outliner.activePerspective)
	}
}

func TestReviewHubCompletionMessagesPreservePlanningSideEffects(t *testing.T) {
	app, _ := newAlertTestApp(t)

	app.handleViewReviewMessages(MarkDailyReviewDoneMsg{})
	app.handleViewReviewMessages(MarkWeeklyReviewDoneMsg{})
	if app.runtime.Planning().GetLastDailyReviewAt() == nil || app.runtime.Planning().GetLastWeeklyReviewAt() == nil {
		t.Fatal("Review completion did not persist both Planning timestamps")
	}

	app.handleViewReviewMessages(ResetDailyReviewMsg{})
	app.handleViewReviewMessages(ResetWeeklyReviewMsg{})
	if app.runtime.Planning().GetLastDailyReviewAt() != nil || app.runtime.Planning().GetLastWeeklyReviewAt() != nil {
		t.Fatal("Review reset did not clear both Planning timestamps")
	}
}

func TestReviewHubLandingHeightMatchesConfiguredBudget(t *testing.T) {
	hub := NewReviewHubModel(config.EmptyRuntimeState(t.TempDir()), nil, nil)
	hub.SetSize(60, 20)
	if got := lipgloss.Height(hub.View().Content); got != 20 {
		t.Fatalf("Review landing height = %d, want 20", got)
	}
}
