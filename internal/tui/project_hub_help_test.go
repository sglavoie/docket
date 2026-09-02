package tui

import (
	"testing"
)

func TestProjectHubHelpNamesTheTaskFilterOnce(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	found := 0
	for _, item := range hub.GetHelpItems() {
		if item.Key != "Ctrl+T" {
			continue
		}
		found++
		if item.Description != "Cycle every project, only those holding open tasks, only those holding none open" ||
			item.Category != "Discovery" {
			t.Fatalf("Ctrl+T help item = %+v", item)
		}
	}
	if found != 1 {
		t.Fatalf("Ctrl+T help entries = %d, want exactly 1", found)
	}
}

// TestProjectHubHelpKeyRaisesTheHubGrammar holds `?` to the surface it was
// pressed on. The core help descriptor hardcodes the Outliner, so dispatching it
// through the executor would answer a Hub press with the Outliner's grammar; the
// Hub raises its own instead, and the press stays a literal character inside the
// query where a search is being typed.
func TestProjectHubHelpKeyRaisesTheHubGrammar(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)

	_, cmd := hub.Update(syntheticKey('?'))
	if cmd == nil {
		t.Fatal("? raised no help on the Project Hub")
	}
	help, ok := cmd().(ShowHelpMsg)
	if !ok {
		t.Fatalf("? produced %T, want ShowHelpMsg", cmd())
	}
	if help.ViewName != "Projects" {
		t.Fatalf("help header = %q, want \"Projects\"", help.ViewName)
	}

	documented := make(map[string]string, len(help.Items))
	for _, item := range help.Items {
		documented[item.Key] = item.Description
	}
	if documented["Enter"] != "Open project" {
		t.Fatalf("Hub help describes Enter as %q, want the Hub's own grammar: %+v", documented["Enter"], help.Items)
	}
	// `?` documents itself only because the keymap names it: the core descriptor
	// is inapplicable off the Outliner, so the registry half of the merge drops it.
	if documented["?"] != "Help" {
		t.Fatalf("Hub help omits ? itself: %+v", help.Items)
	}
	for _, outlinerOnly := range []string{"tab", "T", "ctrl+p"} {
		if description, listed := documented[outlinerOnly]; listed {
			t.Fatalf("Hub help leaked the Outliner key %q (%q)", outlinerOnly, description)
		}
	}

	searching := openProjectHub(t, projectHubApp(t))
	searching.Update(syntheticKey('/'))
	// The input's own cursor blink can ride along, so the message decides rather
	// than the presence of a command.
	if _, cmd := searching.Update(syntheticKey('?')); cmd != nil {
		if raised, isHelp := cmd().(ShowHelpMsg); isHelp {
			t.Fatalf("? raised %q help from inside the query instead of typing a character", raised.ViewName)
		}
	}
	if got := searching.SearchQuery(); got != "?" {
		t.Fatalf("query after ? = %q, want a literal %q", got, "?")
	}
	for _, item := range searching.GetHelpItems() {
		if item.Key == "?" {
			t.Fatalf("search-editing help still offers ?: %+v", item)
		}
	}
}

func TestShowProjectHubMsgOpensTheHubInBrowse(t *testing.T) {
	app := projectHubApp(t)
	handled, cmd := app.handleProjectViewMessages(ShowProjectHubMsg{})
	if !handled {
		t.Fatal("ShowProjectHubMsg was not handled")
	}
	if cmd != nil {
		cmd()
	}
	hub, ok := app.manageHub.ActiveModuleView().(*ProjectHubModel)
	if app.CurrentRoute().ID() != SecondaryTool || app.currentView() != ViewProjectHub || !ok {
		t.Fatalf("Project Hub landed on %s/%s (%T)", app.CurrentRoute().ID(), app.currentView(), app.manageHub.ActiveModuleView())
	}
	if hub.HasActiveSearch() {
		t.Fatal("the Project Hub opened into search instead of browse")
	}
	if _, ok := hub.selectedProjectHubEntry(); !ok {
		t.Fatal("the Project Hub opened without a live cursor")
	}
}

func TestOutlinerProjectHubShortcutsChooseTheirExplicitModes(t *testing.T) {
	app := projectHubApp(t)
	ctx := app.operationCommandContext()
	ctx.View = ViewCrossProjectOutliner

	for _, test := range []struct {
		name string
		path []string
		mode ProjectHubMode
	}{
		{name: "Ctrl+P pinned", path: []string{"ctrl+p"}, mode: ProjectHubModePinned},
		{name: "uppercase P all", path: []string{"P"}, mode: ProjectHubModeAll},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := executeDirectPath(t, app, ctx, test.path)
			message, ok := result.Message.(ShowProjectHubMsg)
			if !ok || message.Mode != test.mode {
				t.Fatalf("path %v emitted %#v, want ShowProjectHubMsg mode %v", test.path, result.Message, test.mode)
			}
			handled, cmd := app.handleProjectViewMessages(message)
			if !handled {
				t.Fatalf("path %v message was not handled", test.path)
			}
			if cmd != nil {
				cmd()
			}
			params := app.CurrentRoute().Params().(ManageParams)
			hub, mounted := app.manageHub.ActiveModuleView().(*ProjectHubModel)
			mountedMode := ProjectHubMode(255)
			if mounted {
				mountedMode = hub.Mode()
			}
			if params.ProjectHubMode != test.mode || !mounted || mountedMode != test.mode {
				t.Fatalf("path %v route/model mode = %v/%v (%T)", test.path, params.ProjectHubMode, mountedMode, app.manageHub.ActiveModuleView())
			}
			app.navigation.ResetRoot()
		})
	}
}

func TestCtrlPRemainsOwnedByInlineOutlinerControls(t *testing.T) {
	app := projectHubApp(t)
	out := app.crossOutliner
	ctrlP := syntheticCtrlKey('p')

	out.ActivateSearch()
	if handled, cmd := app.handleSimplifiedInputKey(ctrlP); handled || cmd != nil {
		t.Fatalf("active search let Ctrl+P reach the Outliner grammar: handled=%v cmd=%v", handled, cmd)
	}
	if app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatal("Ctrl+P opened the Project Hub while search owned input")
	}
	out.ClearSearch()

	out.confirm.Show("test", "Confirm?")
	if handled, cmd := app.handleSimplifiedInputKey(ctrlP); handled || cmd != nil {
		t.Fatalf("active confirmation let Ctrl+P reach the Outliner grammar: handled=%v cmd=%v", handled, cmd)
	}
	if app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatal("Ctrl+P opened the Project Hub while confirmation owned input")
	}
	out.confirm.Cancel()

	app.operationTextPrompt = newOperationTextPromptModel(
		OperationExecutionRequest{}, "Name", "", "draft", "Name",
	)
	if handled, _ := app.handleGlobalKeyPress(ctrlP); !handled || app.operationTextPrompt == nil {
		t.Fatalf("text prompt did not retain Ctrl+P: handled=%v prompt=%v", handled, app.operationTextPrompt != nil)
	}
	app.operationTextPrompt = nil

	app.operationValuePicker = newSemanticTagPickerForTest(t, []TagInfo{{Name: "urgent", TaskCount: 1}}, true)
	if handled, _ := app.handleGlobalKeyPress(ctrlP); !handled || app.operationValuePicker == nil {
		t.Fatalf("value picker did not retain Ctrl+P: handled=%v picker=%v", handled, app.operationValuePicker != nil)
	}
	app.operationValuePicker = nil

	descriptor := contextActionTestDescriptor("test.ctrl-p-owner", "Owner")
	registry, executor := contextActionTestEngine(t, []OperationDescriptor{descriptor}, nil)
	sheet, err := NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		[]OperationID{descriptor.ID},
		ContextActionSheetConfig{Host: ContextActionHostOutliner},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	app.contextActionSheet = sheet
	if handled, _ := app.handleGlobalKeyPress(ctrlP); !handled || app.contextActionSheet == nil {
		t.Fatalf("action sheet did not retain Ctrl+P: handled=%v sheet=%v", handled, app.contextActionSheet != nil)
	}
	app.contextActionSheet = nil

	app.operationConfirm.Show(operationConfirmAction, "Confirm?")
	if handled, _ := app.handleGlobalKeyPress(ctrlP); !handled || !app.operationConfirm.IsActive() {
		t.Fatalf("operation confirmation did not retain Ctrl+P: handled=%v active=%v", handled, app.operationConfirm.IsActive())
	}
	if app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatal("Ctrl+P opened the Project Hub while an overlay owned input")
	}
}
