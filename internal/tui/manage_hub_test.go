package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestManageHubLandingNavigationAndNarrowLayout(t *testing.T) {
	hub := NewManageHubModel(ManageHubDependencies{Runtime: config.EmptyRuntimeState(t.TempDir())})
	hub.SetSize(60, 18)

	landing := hub.View().Content
	for _, name := range []string{"Inbox", "Notes", "Blockers", "Rules", "Templates", "Tags", "Notifications", "Settings"} {
		if !strings.Contains(ansi.Strip(landing), name) {
			t.Fatalf("Manage landing omitted %q:\n%s", name, ansi.Strip(landing))
		}
	}
	assertBaselineLineWidth(t, "Manage landing", landing, 60)
	assertNoFooterContentOverlap(t, "Manage landing", landing, 18)
	if got := lipgloss.Height(landing); got != 18 {
		t.Fatalf("Manage landing height = %d, want 18", got)
	}

	_, cmd := hub.Update(syntheticKey('7'))
	if cmd == nil {
		t.Fatal("Manage landing numeric selection did not emit a module change")
	}
	selected, ok := cmd().(ManageHubModuleSelectedMsg)
	if !ok || selected.Params.Module != ManageNotifications || selected.Params.NotificationMode != NotificationAlerts {
		t.Fatalf("Manage landing selection = %#v, want Notifications/Alerts", cmd())
	}
}

func TestManageHubDeepLinksUseOneTypedRouteAndTwoStageEsc(t *testing.T) {
	tests := []struct {
		name   string
		open   func(*AppModel, *models.Project) (bool, tea.Cmd)
		params ManageParams
	}{
		{name: "project inbox", open: func(app *AppModel, project *models.Project) (bool, tea.Cmd) {
			return app.handleInboxMessages(ShowInboxProcessorMsg{Project: project})
		}, params: ManageParams{Module: ManageInbox, Scope: ProjectEntityScope("project")}},
		{name: "global inbox", open: func(app *AppModel, _ *models.Project) (bool, tea.Cmd) {
			return app.handleInboxMessages(ShowGlobalInboxProcessorMsg{})
		}, params: ManageParams{Module: ManageInbox, Scope: AppEntityScope()}},
		{name: "project notes", open: func(app *AppModel, project *models.Project) (bool, tea.Cmd) {
			return app.handleNotesMessages(ShowNotesMsg{Project: project})
		}, params: ManageParams{Module: ManageNotes, Scope: ProjectEntityScope("project")}},
		{name: "global notes", open: func(app *AppModel, _ *models.Project) (bool, tea.Cmd) {
			return app.handleNotesMessages(ShowGlobalNotesMsg{})
		}, params: ManageParams{Module: ManageNotes, Scope: AppEntityScope()}},
		{name: "project blockers", open: func(app *AppModel, project *models.Project) (bool, tea.Cmd) {
			return app.handleViewSupportMessages(ShowBlockersMsg{Project: project})
		}, params: ManageParams{Module: ManageBlockers, Scope: ProjectEntityScope("project")}},
		{name: "global blockers", open: func(app *AppModel, _ *models.Project) (bool, tea.Cmd) {
			return app.handleViewSupportMessages(ShowGlobalBlockersMsg{})
		}, params: ManageParams{Module: ManageBlockers, Scope: AppEntityScope()}},
		{name: "project tags", open: func(app *AppModel, project *models.Project) (bool, tea.Cmd) {
			return app.handleTagMessages(ShowTagManagerMsg{Project: project})
		}, params: ManageParams{Module: ManageTags, Scope: ProjectEntityScope("project")}},
		{name: "global tags", open: func(app *AppModel, _ *models.Project) (bool, tea.Cmd) {
			return app.handleTagMessages(ShowGlobalTagManagerMsg{})
		}, params: ManageParams{Module: ManageTags, Scope: AppEntityScope()}},
		{name: "alerts", open: func(app *AppModel, _ *models.Project) (bool, tea.Cmd) {
			return app.handleNotificationMessages(ShowNotificationCenterMsg{})
		}, params: ManageParams{Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationAlerts}},
		{name: "message log", open: func(app *AppModel, _ *models.Project) (bool, tea.Cmd) {
			return app.handleViewCoreMessages(ShowMessageLogMsg{})
		}, params: ManageParams{Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationLog}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := typedTestApp(t)
			project := app.currentProject
			want := tc.params
			if want.Scope.Kind == ScopeProject {
				want.Scope.ProjectID = project.ID
			}
			handled, _ := tc.open(app, project)
			if !handled {
				t.Fatal("deep link was not handled")
			}

			route := app.CurrentRoute()
			params, ok := route.Params().(ManageParams)
			if route.ID() != ManageHub || !ok || params != want {
				t.Fatalf("deep link route = %s/%#v, want Manage/%#v", route.ID(), route.Params(), want)
			}
			if _, legacy := route.Presentation(); legacy {
				t.Fatal("live Manage module retained an independent legacy presentation")
			}
			if app.activeView() != app.manageHub || app.manageHub.Params() != want {
				t.Fatalf("active Manage renderer/params = %T/%#v", app.activeView(), app.manageHub.Params())
			}

			app.handleGlobalKeyPress(syntheticSpecialKey(27))
			landing := app.CurrentRoute().Params().(ManageParams)
			if landing.Module != ManageLanding || app.manageHub.Params().Module != ManageLanding {
				t.Fatalf("first Esc = route/hub %s/%s, want Manage landing", landing.Module, app.manageHub.Params().Module)
			}
			app.handleGlobalKeyPress(syntheticSpecialKey(27))
			if app.CurrentRoute().ID() != Home {
				t.Fatalf("second Esc route = %s, want Home", app.CurrentRoute().ID())
			}
		})
	}
}

func TestManageHubScopeAndNotificationModeUseRouteParams(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	app.handleNotesMessages(ShowGlobalNotesMsg{})

	app.manageHub.Update(syntheticKey('v'))
	if app.manageHub.pickerTitle != "Select scope" {
		t.Fatalf("v picker title = %q, want Select scope", app.manageHub.pickerTitle)
	}
	projectIndex := -1
	for i, entry := range app.manageHub.pickerEntries {
		if entry.params.Scope.ProjectID == project.ID {
			projectIndex = i
			break
		}
	}
	if projectIndex < 0 {
		t.Fatalf("scope picker omitted project %q: %#v", project.ID, app.manageHub.pickerEntries)
	}
	app.manageHub.pickerCursor = projectIndex
	_, cmd := app.manageHub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("project scope selection emitted no route change")
	}
	app.handleManageHubMessages(cmd())
	params := app.CurrentRoute().Params().(ManageParams)
	notes, ok := app.manageHub.ActiveModuleView().(*NotesViewModel)
	if params.Scope != ProjectEntityScope(project.ID) || !ok || notes.project != project {
		t.Fatalf("project scope = %#v/%T/%p, want %q/%p", params.Scope, app.manageHub.ActiveModuleView(), notes.project, project.ID, project)
	}

	app.messageLog.Append("background sync finished", MessageLogLevelSuccess)
	app.activateManageHub(ManageParams{Module: ManageNotifications, Scope: ProjectEntityScope(project.ID)})
	params = app.CurrentRoute().Params().(ManageParams)
	if params.Scope != AppEntityScope() {
		t.Fatalf("global-only Notifications retained project scope: %#v", params.Scope)
	}
	app.manageHub.Update(syntheticKey('v'))
	app.manageHub.pickerCursor = 1
	_, cmd = app.manageHub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	app.handleManageHubMessages(cmd())
	params = app.CurrentRoute().Params().(ManageParams)
	logView, ok := app.manageHub.ActiveModuleView().(*MessageLogViewModel)
	if params.NotificationMode != NotificationLog || !ok {
		t.Fatalf("notification mode/view = %s/%T, want Log", params.NotificationMode, app.manageHub.ActiveModuleView())
	}
	if !strings.Contains(ansi.Strip(logView.View().Content), "background sync finished") {
		t.Fatal("Manage Notifications/Log omitted the app message history")
	}
}

func TestManageHubCollectionGrammarAndProjectPickerFallback(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject

	app.handleNotesMessages(ShowNotesMsg{Project: project})
	notes := app.manageHub.ActiveModuleView().(*NotesViewModel)
	notes.notes = []*models.Note{{Filename: "one.md", Title: "One"}}
	app.manageHub.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !notes.multiSelectMode || !notes.selectedNotes["one.md"] {
		t.Fatal("Space did not enter Notes multi-select and select the current row")
	}
	notes.multiSelectMode = false
	notes.selectedNotes = make(map[string]bool)
	_, cmd := app.manageHub.Update(syntheticKey('a'))
	if cmd == nil {
		t.Fatal("Notes a did not dispatch add")
	}
	if _, ok := cmd().(ShowNoteCreatorMsg); !ok {
		t.Fatalf("Notes a emitted %T, want ShowNoteCreatorMsg", cmd())
	}

	app.handleTagMessages(ShowTagManagerMsg{Project: project})
	tags := app.manageHub.ActiveModuleView().(*TagManagerModel)
	tags.tags = []TagInfo{{Name: "alpha", TaskCount: 1}}
	app.manageHub.Update(syntheticKey('e'))
	if !tags.renameMode || tags.renameTarget != "alpha" {
		t.Fatal("Tags e did not enter edit mode")
	}
	app.manageHub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	wantHelp := []string{"j/k", "enter", "a/e/x", "space", "/", "v"}
	help := app.manageHub.GetHelpItems()
	gotHelp := make([]string, len(help))
	for i := range help {
		gotHelp[i] = help[i].Key
	}
	if !slices.Equal(gotHelp, wantHelp) {
		t.Fatalf("Manage collection grammar = %v, want %v", gotHelp, wantHelp)
	}

	app.handleNotesMessages(ShowNotesMsg{})
	if app.CurrentRoute().ID() != ManageHub || app.manageHub.pickerTitle != "Select project" {
		t.Fatalf("missing project fallback = %s/%q, want Manage project picker", app.CurrentRoute().ID(), app.manageHub.pickerTitle)
	}
}

func TestManageHubRemovedProjectFallsBackToAppScope(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	app.handleNotesMessages(ShowNotesMsg{Project: project})
	setTestAppProjects(app)

	handled, _ := app.handleGlobalKeyPress(syntheticKey('j'))
	if !handled {
		t.Fatal("removed Manage project scope was not normalized")
	}
	params := app.CurrentRoute().Params().(ManageParams)
	if app.CurrentRoute().ID() != ManageHub || params.Module != ManageNotes || params.Scope != AppEntityScope() {
		t.Fatalf("removed project fallback = %s/%#v, want Manage Notes app scope", app.CurrentRoute().ID(), params)
	}
	if app.currentProject != nil {
		t.Fatal("removed project pointer remained selected")
	}
}

func TestManageHubProjectDeletionStaysInTheCollectionAtAppScope(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	app.handleNotesMessages(ShowNotesMsg{Project: project})

	handled, _ := app.handleProjectMessages(ProjectDeletedMsg{Project: project})
	if !handled {
		t.Fatal("ProjectDeletedMsg was not handled")
	}
	params := app.CurrentRoute().Params().(ManageParams)
	if app.CurrentRoute().ID() != ManageHub || params.Module != ManageNotes || params.Scope != AppEntityScope() {
		t.Fatalf("delete fallback = %s/%#v, want Manage Notes app scope", app.CurrentRoute().ID(), params)
	}
}

func TestManageLegacyNamesCollapseToScopedModules(t *testing.T) {
	tests := []struct {
		view   PresentationID
		params ManageParams
	}{
		{ViewInboxProcessor, ManageParams{Module: ManageInbox, Scope: ProjectEntityScope("project")}},
		{ViewGlobalInboxProcessor, ManageParams{Module: ManageInbox, Scope: AppEntityScope()}},
		{ViewNotes, ManageParams{Module: ManageNotes, Scope: ProjectEntityScope("project")}},
		{ViewGlobalNotes, ManageParams{Module: ManageNotes, Scope: AppEntityScope()}},
		{ViewBlockers, ManageParams{Module: ManageBlockers, Scope: ProjectEntityScope("project")}},
		{ViewGlobalBlockers, ManageParams{Module: ManageBlockers, Scope: AppEntityScope()}},
		{ViewTagManager, ManageParams{Module: ManageTags, Scope: ProjectEntityScope("project")}},
		{ViewGlobalTagManager, ManageParams{Module: ManageTags, Scope: AppEntityScope()}},
		{ViewNotificationCenter, ManageParams{Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationAlerts}},
		{ViewMessageLog, ManageParams{Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationLog}},
		{ViewWorkflowRules, ManageParams{Module: ManageRules, Scope: AppEntityScope()}},
		{ViewScoreProfileManager, ManageParams{Module: ManageRules, Scope: AppEntityScope()}},
		{ViewTaskTemplates, ManageParams{Module: ManageTemplates, Scope: AppEntityScope()}},
		{ViewSettings, ManageParams{Module: ManageSettings, Scope: AppEntityScope()}},
	}

	for _, tc := range tests {
		route, ok, err := routeForPresentation(tc.view, "project", "")
		if err != nil || !ok {
			t.Fatalf("%s adapter: ok=%v err=%v", tc.view, ok, err)
		}
		if route.ID() != ManageHub || route.Params().(ManageParams) != tc.params {
			t.Errorf("%s maps to %s/%#v, want Manage/%#v", tc.view, route.ID(), route.Params(), tc.params)
		}
	}
}

func TestManageLegacyNavigateMessagesReachTheTypedHub(t *testing.T) {
	tests := []struct {
		view   PresentationID
		module ManageModule
		scope  RouteScope
		mode   NotificationMode
	}{
		{view: ViewInboxProcessor, module: ManageInbox, scope: ScopeProject},
		{view: ViewGlobalInboxProcessor, module: ManageInbox, scope: ScopeApp},
		{view: ViewNotes, module: ManageNotes, scope: ScopeProject},
		{view: ViewGlobalNotes, module: ManageNotes, scope: ScopeApp},
		{view: ViewBlockers, module: ManageBlockers, scope: ScopeProject},
		{view: ViewGlobalBlockers, module: ManageBlockers, scope: ScopeApp},
		{view: ViewTagManager, module: ManageTags, scope: ScopeProject},
		{view: ViewGlobalTagManager, module: ManageTags, scope: ScopeApp},
		{view: ViewNotificationCenter, module: ManageNotifications, scope: ScopeApp, mode: NotificationAlerts},
		{view: ViewMessageLog, module: ManageNotifications, scope: ScopeApp, mode: NotificationLog},
		{view: ViewWorkflowRules, module: ManageRules, scope: ScopeApp},
		{view: ViewScoreProfileManager, module: ManageRules, scope: ScopeApp},
		{view: ViewTaskTemplates, module: ManageTemplates, scope: ScopeApp},
		{view: ViewSettings, module: ManageSettings, scope: ScopeApp},
	}

	for _, tc := range tests {
		t.Run(tc.view.String(), func(t *testing.T) {
			app := typedTestApp(t)
			handled, _ := app.handleNavigateToView(NavigateToViewMsg{PresentationID: tc.view})
			if !handled {
				t.Fatal("NavigateToViewMsg was not handled")
			}
			params := app.CurrentRoute().Params().(ManageParams)
			if app.CurrentRoute().ID() != ManageHub || params.Module != tc.module || params.Scope.Kind != tc.scope || params.NotificationMode != tc.mode {
				t.Fatalf("navigate result = %s/%#v, want Manage/%s/%v/%s", app.CurrentRoute().ID(), params, tc.module, tc.scope, tc.mode)
			}
			if tc.view == ViewWorkflowRules && app.manageHub.RulesMode() != ManageRulesWorkflow {
				t.Fatalf("Workflow deep link selected Rules/%s", app.manageHub.RulesMode())
			}
			if tc.view == ViewScoreProfileManager && app.manageHub.RulesMode() != ManageRulesScoring {
				t.Fatalf("Scoring deep link selected Rules/%s", app.manageHub.RulesMode())
			}
		})
	}
}
