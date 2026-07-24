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

func TestExactGlobalBindingContract(t *testing.T) {
	items := NewKeybindingRegistry().GetGlobalBindings()
	got := make([]string, len(items))
	for i := range items {
		got[i] = items[i].Key
	}
	// Ctrl+G and the destination switcher are retired: the Outliner is the sole
	// root, so no "go to destination" global binding remains.
	want := []string{":", "?", "esc", "ctrl+c", "ctrl+esc", "ctrl+n", "ctrl+x", "|", "#"}
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
		// Ctrl+G (the retired destination switcher) is now inert.
		{Code: 'g', Mod: tea.ModCtrl},
	}
	for _, key := range removed {
		t.Run(key.String(), func(t *testing.T) {
			app := typedTestApp(t)
			before := app.CurrentRoute()
			handled, cmd := app.handleGlobalKeyPress(key)
			if handled || cmd != nil {
				t.Fatalf("removed key %q dispatched handled=%v cmd=%T", key.String(), handled, cmd)
			}
			if !app.CurrentRoute().Equal(before) {
				t.Fatalf("removed key %q changed navigation", key.String())
			}
		})
	}
}

func TestEscUnwindsModalReferenceDetailToOutlinerRoot(t *testing.T) {
	app := typedTestApp(t)
	app.pushView(ViewOutliner)
	app.lastNavigatedTaskID = app.currentProject.Tasks[0].ID
	app.pushView(ViewTaskDetail)
	app.pushView(ViewHelp)
	app.pushView(ViewTaskForm)

	// Esc unwinds overlays, then the Task Detail child, landing on the Outliner
	// root. The Outliner is the sole root, so the sequence terminates there.
	want := []PresentationID{ViewHelp, ViewTaskDetail, ViewOutliner}
	for i, view := range want {
		handled, _ := app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeyEsc))
		if !handled || app.currentView() != view {
			t.Fatalf("Esc step %d = handled %v view %s, want %s", i, handled, app.currentView(), view)
		}
	}
	// A clean Outliner root Esc is a no-op.
	if handled, _ := app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeyEsc)); handled || app.currentView() != ViewOutliner {
		t.Fatalf("clean root Esc = handled %v view %s, want no-op on Outliner", handled, app.currentView())
	}
}

func TestSecondaryToolEscReturnsToOutlinerRoot(t *testing.T) {
	app := typedTestApp(t)
	// Launch a secondary tool as a child of the Outliner root.
	if cmd := app.activateReviewHub(ReviewWeekly, nil); cmd != nil {
		cmd()
	}
	if app.CurrentRoute().ID() != SecondaryTool {
		t.Fatalf("weekly review route = %s, want SecondaryTool", app.CurrentRoute().ID())
	}
	// A single Esc dismisses the child directly back to the Outliner root with no
	// intermediate landing retreat.
	app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeyEsc))
	if app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatalf("Esc route = %s, want TaskWorkspace (Outliner root)", app.CurrentRoute().ID())
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

	if handled, _ := app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeyEsc)); !handled || app.currentView() != ViewCrossProjectOutliner {
		t.Fatal("Esc must preempt and close the text-input overlay, revealing the Outliner root")
	}
	app.pushView(ViewTaskForm)
	if handled, _ := app.handleGlobalKeyPress(ctrlEscKeyPress()); !handled || app.currentView() != ViewCrossProjectOutliner {
		t.Fatal("Ctrl+Esc must preempt and reset to the Outliner root")
	}
	app.pushView(ViewTaskForm)
	if handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); !handled || cmd == nil || !app.quitting {
		t.Fatal("Ctrl+C must preempt text input and quit")
	}
}
