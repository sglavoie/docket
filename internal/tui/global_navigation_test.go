package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func typedTestApp(t *testing.T) *AppModel {
	t.Helper()
	app, _ := newTestAppModelWithProject()
	app.enableTypedNavigation()
	return app
}

func TestGlobalDestinationSwitcherContract(t *testing.T) {
	entries := GlobalDestinationEntries("project-1")
	if len(entries) != 6 {
		t.Fatalf("Ctrl+G destinations = %d, want 6", len(entries))
	}
	wantNames := []string{"Home", "Planner", "Tasks", "Project", "Review", "Manage"}
	wantRoutes := []RouteID{Home, Planner, TaskWorkspace, TaskWorkspace, ReviewHub, ManageHub}
	for i, entry := range entries {
		if entry.Name != wantNames[i] || entry.Route.ID() != wantRoutes[i] {
			t.Errorf("destination[%d] = %s/%s, want %s/%s", i, entry.Name, entry.Route.ID(), wantNames[i], wantRoutes[i])
		}
	}
	tasks := entries[2].Route.Params().(TaskWorkspaceParams)
	project := entries[3].Route.Params().(TaskWorkspaceParams)
	if tasks.Scope != AppEntityScope() || project.Scope != ProjectEntityScope("project-1") {
		t.Fatalf("Tasks/Project scopes = %+v/%+v", tasks.Scope, project.Scope)
	}
}

func TestExactGlobalBindingContract(t *testing.T) {
	items := NewKeybindingRegistry().GetGlobalBindings()
	got := make([]string, len(items))
	for i := range items {
		got[i] = items[i].Key
	}
	want := []string{":", "?", "esc", "ctrl+c", "ctrl+esc", "ctrl+g", "ctrl+n", "ctrl+x", "|", "#"}
	if !slices.Equal(got, want) {
		t.Fatalf("global bindings = %v, want exactly %v", got, want)
	}
}

func TestRemovedGlobalKeysDoNotDispatchInTypedShell(t *testing.T) {
	removed := []tea.KeyPressMsg{
		syntheticKey(';'),
		{Code: '/', Mod: tea.ModCtrl},
		{Code: 'o', Mod: tea.ModAlt},
		{Code: 'i', Mod: tea.ModAlt},
		{Code: '6', Mod: tea.ModCtrl},
		{Code: 'l', Mod: tea.ModAlt},
		{Code: '\\', Mod: tea.ModCtrl},
		{Code: ',', Mod: tea.ModCtrl},
		{Code: 'i', Mod: tea.ModCtrl},
		{Code: 'o', Mod: tea.ModCtrl},
	}
	for _, key := range removed {
		t.Run(key.String(), func(t *testing.T) {
			app := typedTestApp(t)
			before := app.CurrentRoute()
			handled, cmd := app.handleGlobalKeyPress(key)
			if handled || cmd != nil {
				t.Fatalf("removed key %q dispatched handled=%v cmd=%T", key.String(), handled, cmd)
			}
			if !app.CurrentRoute().Equal(before) || app.destinationSwitcher != nil {
				t.Fatalf("removed key %q changed navigation", key.String())
			}
		})
	}
}

func TestCtrlGSwitcherAndCanonicalHomeReset(t *testing.T) {
	app := typedTestApp(t)
	app.pushView(ViewOutliner)

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	if !handled || cmd != nil || app.destinationSwitcher == nil {
		t.Fatalf("Ctrl+G did not open switcher: handled=%v cmd=%T", handled, cmd)
	}
	if app.navigation.Overlays.Depth() != 1 {
		t.Fatalf("overlay depth = %d, want 1", app.navigation.Overlays.Depth())
	}

	cmd = app.handleDestinationSwitcherKey(syntheticKey('1'))
	if cmd != nil || app.CurrentRoute().ID() != Home || app.currentView() != ViewDashboard {
		t.Fatalf("Home selection left route/view %s/%s", app.CurrentRoute().ID(), app.currentView())
	}
	if app.navigation.Overlays.Active() || len(app.viewStack) != 1 {
		t.Fatal("Home reset must clear overlays and the presentation mirror")
	}
}

func TestEscUnwindsModalReferenceDetailAndDestination(t *testing.T) {
	app := typedTestApp(t)
	app.pushView(ViewOutliner)
	app.lastNavigatedTaskID = app.currentProject.Tasks[0].ID
	app.pushView(ViewTaskDetail)
	app.pushView(ViewHelp)
	app.pushView(ViewTaskForm)

	want := []PresentationID{ViewHelp, ViewTaskDetail, ViewOutliner, ViewDashboard}
	for i, view := range want {
		handled, _ := app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeyEsc))
		if !handled || app.currentView() != view {
			t.Fatalf("Esc step %d = handled %v view %s, want %s", i, handled, app.currentView(), view)
		}
	}
}

func TestReviewModuleEscReturnsLandingThenHome(t *testing.T) {
	app := typedTestApp(t)
	app.pushView(ViewDailyReview)
	app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeyEsc))
	params := app.CurrentRoute().Params().(ReviewParams)
	if params.Module != ReviewLanding {
		t.Fatalf("first Esc Review module = %v, want landing", params.Module)
	}
	app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeyEsc))
	if app.CurrentRoute().ID() != Home {
		t.Fatalf("second Esc route = %s, want Home", app.CurrentRoute().ID())
	}
}

func TestEnterAndShiftEnterUseDetailPeekGrammar(t *testing.T) {
	app := typedTestApp(t)
	task := app.currentProject.Tasks[0]
	view := &confirmQueueToggleView{task: task, project: app.currentProject}
	app.views.Register(ViewDailyReview, view)
	app.pushView(ViewDailyReview)

	handled, cmd := app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeyEnter))
	if !handled || cmd == nil {
		t.Fatal("Enter did not dispatch Task Detail")
	}
	if msg, ok := cmd().(ShowTaskDetailMsg); !ok || msg.Task != task {
		t.Fatalf("Enter command = %T, want selected task detail", cmd())
	}

	handled, cmd = app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	if !handled || cmd != nil || !app.peekActive || app.peekTask != task {
		t.Fatalf("Shift+Enter Peek handled=%v cmd=%T active=%v", handled, cmd, app.peekActive)
	}
}

func TestGlobalInputCapturePrecedence(t *testing.T) {
	app := typedTestApp(t)
	view := &ctrlNRecorderTaskRefView{
		task: app.currentProject.Tasks[0], project: app.currentProject, hasTextInput: true,
	}
	app.views.Register(ViewTaskForm, view)
	app.pushView(ViewTaskForm)

	for _, key := range []tea.KeyPressMsg{
		{Code: 'g', Mod: tea.ModCtrl},
		{Code: 'n', Mod: tea.ModCtrl},
		{Code: 'x', Mod: tea.ModCtrl},
		syntheticKey(':'), syntheticKey('?'), syntheticKey('|'), syntheticKey('#'),
	} {
		if handled, cmd := app.handleGlobalKeyPress(key); handled || cmd != nil {
			t.Errorf("text input did not capture %q: handled=%v cmd=%T", key.String(), handled, cmd)
		}
	}

	if handled, _ := app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeyEsc)); !handled || app.currentView() != ViewDashboard {
		t.Fatal("Esc must preempt and close the text-input overlay")
	}
	app.pushView(ViewTaskForm)
	if handled, _ := app.handleGlobalKeyPress(ctrlEscKeyPress()); !handled || app.currentView() != ViewDashboard {
		t.Fatal("Ctrl+Esc must preempt and reset Home")
	}
	app.pushView(ViewTaskForm)
	if handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); !handled || cmd == nil || !app.quitting {
		t.Fatal("Ctrl+C must preempt text input and quit")
	}
}
