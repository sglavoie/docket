package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestReviewToolsRenderDirectlyWithoutLanding(t *testing.T) {
	hub := NewReviewHubModel(config.EmptyRuntimeState(t.TempDir()), nil, nil)
	hub.SetSize(60, 18)

	// Daily Review and Activity are Outliner computed modes, so the review hub
	// only owns the remaining direct tools.
	for _, module := range []ReviewModule{ReviewWeekly, ReviewMetrics, ReviewRoadmap} {
		t.Run(module.String(), func(t *testing.T) {
			if err := hub.Activate(module, nil); err != nil {
				t.Fatal(err)
			}
			rendered := hub.View().Content
			if strings.TrimSpace(ansi.Strip(rendered)) == "" {
				t.Fatal("empty Review tool rendered no empty state")
			}
			assertBaselineLineWidth(t, module.String(), rendered, 60)
			assertNoFooterContentOverlap(t, module.String(), rendered, 18)
		})
	}
}

func TestReviewToolDeepLinksUseSecondaryToolRouteAndReturnDirectly(t *testing.T) {
	tests := []struct {
		name    string
		message any
		module  ReviewModule
	}{
		{name: "weekly", message: ShowWeeklyReviewMsg{}, module: ReviewWeekly},
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
			if route.ID() != SecondaryTool || !ok || params.Module != tc.module {
				t.Fatalf("deep link route = %s/%#v, want SecondaryTool/%s", route.ID(), route.Params(), tc.module)
			}
			if app.activeView() != app.reviewHub || app.reviewHub.Module() != tc.module {
				t.Fatalf("active Review renderer/module = %T/%s", app.activeView(), app.reviewHub.Module())
			}

			// A single Esc dismisses the child tool directly back to its launching
			// destination — the sole Outliner root — with no intermediate landing.
			app.handleGlobalKeyPress(syntheticSpecialKey(27))
			if app.CurrentRoute().ID() != TaskWorkspace {
				t.Fatalf("Esc route = %s, want TaskWorkspace (Outliner root)", app.CurrentRoute().ID())
			}
		})
	}
}

func TestReviewModeLaunchMessagesRedirectToOutliner(t *testing.T) {
	tests := []struct {
		name    string
		message tea.Msg
		mode    OutlinerModeID
	}{
		{name: "daily", message: ShowDailyReviewMsg{}, mode: ModeDailyReview},
		{name: "activity", message: ShowRecentActivityMsg{}, mode: ModeActivity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := typedTestApp(t)
			handled, cmd := app.handleViewMessages(tc.message)
			if !handled || cmd == nil {
				t.Fatalf("%T handled=%v cmd=%v, want redirect command", tc.message, handled, cmd)
			}
			outliner, ok := cmd().(ShowGlobalOutlinerMsg)
			if !ok || !outliner.Options.SetMode || outliner.Options.Mode != tc.mode {
				t.Fatalf("%T redirect = %#v, want Outliner mode %s", tc.message, cmd(), tc.mode)
			}
		})
	}
}

func TestReviewHubLegacyNamesMapToModulesAndRemainSearchable(t *testing.T) {
	// Daily Review and Activity are Outliner computed modes: their legacy
	// presentations resolve to the global Task Workspace.
	outlinerViews := []PresentationID{ViewDailyReview, ViewRecentActivity, ViewProjectRecentActivity}
	for _, view := range outlinerViews {
		route, ok, err := routeForPresentation(view, "project-1", "")
		if err != nil || !ok {
			t.Fatalf("%s deep-link adapter: ok=%v err=%v", view, ok, err)
		}
		if route.ID() != TaskWorkspace {
			t.Errorf("%s maps to %s, want TaskWorkspace", view, route.ID())
		}
	}

	toolRoutes := []struct {
		view   PresentationID
		module ReviewModule
	}{
		{ViewWeeklyReview, ReviewWeekly},
		{ViewMetricsDashboard, ReviewMetrics},
		{ViewRoadmap, ReviewRoadmap},
	}
	for _, tc := range toolRoutes {
		route, ok, err := routeForPresentation(tc.view, "project-1", "")
		if err != nil || !ok {
			t.Fatalf("%s deep-link adapter: ok=%v err=%v", tc.view, ok, err)
		}
		params := route.Params().(ReviewParams)
		if route.ID() != SecondaryTool || params.Module != tc.module {
			t.Errorf("%s maps to %s/%s, want SecondaryTool/%s", tc.view, route.ID(), params.Module, tc.module)
		}
	}

	app := newTestAppModel()
	// The retired Review Hub is fully removed. Daily Review and Activity now
	// derive their searchable names from the canonical Outliner mode catalog;
	// the remaining direct tools stay in viewCommands.
	aliases := []struct {
		command   string
		want      []string
		forbidden []string
		modes     bool
	}{
		{"Daily Review", []string{"DailyReview"}, []string{"ReviewHub/Daily"}, true},
		{"weekly review", []string{"WeeklyReview"}, []string{"ReviewHub/Weekly"}, false},
		{"Activity", []string{"RecentActivity", "latest activity"}, []string{"ReviewHub/Activity", "ProjectRecentActivity", "completed tasks"}, true},
		{"Completed", []string{"completed tasks"}, nil, true},
		{"metrics dashboard", []string{"MetricsDashboard"}, []string{"ReviewHub/Metrics"}, false},
		{"roadmap", []string{"review roadmap"}, []string{"ReviewHub/Roadmap"}, false},
	}
	for _, tc := range aliases {
		commands := app.viewCommands()
		if tc.modes {
			commands = outlinerModeCommands()
		}
		command := findCommand(commands, tc.command)
		if command == nil {
			t.Fatalf("missing Review command %q", tc.command)
		}
		for _, alias := range tc.want {
			if !slices.Contains(command.Aliases, alias) {
				t.Errorf("%q aliases %v omit searchable name %q", tc.command, command.Aliases, alias)
			}
		}
		for _, alias := range tc.forbidden {
			if slices.Contains(command.Aliases, alias) {
				t.Errorf("%q aliases %v still expose retired hub name %q", tc.command, command.Aliases, alias)
			}
		}
	}
}

func TestReviewTaskModulesShareGrammarAndCompletedBrowsing(t *testing.T) {
	runtime := config.EmptyRuntimeState(t.TempDir())
	weekly := NewWeeklyReviewModel(runtime, nil)

	for name, keys := range map[string][]string{
		"Weekly": weekly.HandledKeys(),
	} {
		for _, shared := range []string{",", "!", "d", "z", "m", "c", "t", "b"} {
			if !slices.Contains(keys, shared) {
				t.Errorf("%s Review grammar omits shared task action %q: %v", name, shared, keys)
			}
		}
	}

	project := &models.Project{ID: "project", Name: "Project"}

	app := typedTestApp(t)
	// The single authoritative Outliner is the cross-project instance; project
	// selection is a scope frame on it, not a separate project-scoped root.
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	if cmd := app.activateTaskWorkspaceRoute(outlinerRootRoute()); cmd != nil {
		t.Fatalf("activate Task Workspace returned a command")
	}
	app.crossOutliner.pushProjectScope(project.ID)
	command := findCommand(app.filterCommands(), "view completed")
	if command == nil {
		t.Fatal("missing view completed command")
	}
	if msg := command.Handler(); msg != nil {
		t.Fatalf("view completed returned %T, want in-place change", msg)
	}
	if app.crossOutliner.smartFocus != nil || app.crossOutliner.activePerspective != PerspectiveCompleted {
		t.Fatalf("view completed focus = %v/%d, want Completed", app.crossOutliner.smartFocus, app.crossOutliner.activePerspective)
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
